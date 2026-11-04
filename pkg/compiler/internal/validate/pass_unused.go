package validate

import (
	"fmt"
	"strings"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/diagnostics"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
)

// UnusedVariablePass detects unused action parameters and variables.
type UnusedVariablePass struct{}

// Validate runs the unused identifier analysis on actions in the semantic IR.
func (UnusedVariablePass) Validate(sem *ir.SemanticIR, _ *SymbolTable) []ir.Diagnostic {
	var diags []ir.Diagnostic

	for _, a := range sem.Actions {
		diags = append(diags, validateActionUnused(a)...)
	}

	return diags
}

type varBinding struct {
	name         string
	span         ir.Span
	stepKind     string
	isParam      bool
	isSideEffect bool
	reassigned   bool
	used         bool
}

type unusedScope struct {
	parent   *unusedScope
	bindings map[string]*varBinding
}

func (s *unusedScope) lookup(name string) *varBinding {
	for cur := s; cur != nil; cur = cur.parent {
		if b, ok := cur.bindings[name]; ok {
			return b
		}
	}
	return nil
}

func (s *unusedScope) markUsed(name string) {
	if b := s.lookup(name); b != nil {
		b.used = true
	}
}

func (s *unusedScope) checkExpr(e ir.Expr) {
	ir.WalkExpr(e, func(node ir.Expr) {
		switch node.Kind {
		case ir.ExprIdent:
			if node.Value != "" {
				s.markUsed(node.Value)
			}
		case ir.ExprField:
			if root := rootTarget(node); root != "" {
				s.markUsed(root)
			}
		}
	})
}

func (s *unusedScope) checkStepExpressions(step ir.StepDecl) {
	if step.Expr != nil {
		s.checkExpr(*step.Expr)
	}
	// For where clauses, only walk value expressions, NOT entity field names.
	if step.WhereValue != nil {
		s.checkExpr(*step.WhereValue)
	}
	for _, p := range step.WherePredicates {
		s.checkExpr(p.Value)
	}
	if len(step.WherePredicates) == 0 && step.WhereValue == nil && step.WhereExpr != nil {
		walkWhereValueExprs(step.WhereExpr, s.checkExpr)
	}
	if step.LimitExpr != nil {
		s.checkExpr(*step.LimitExpr)
	}
	if step.CursorExpr != nil {
		s.checkExpr(*step.CursorExpr)
	}
	for _, a := range step.PolicyArgs {
		s.checkExpr(a)
	}
	for _, a := range step.SlotArgs {
		s.checkExpr(a)
	}
	for _, f := range step.Fields {
		s.checkExpr(f.Value)
	}
}

func walkWhereValueExprs(e *ir.Expr, onExpr func(ir.Expr)) {
	if e == nil {
		return
	}
	if e.Op == "and" || e.Op == "or" {
		walkWhereValueExprs(e.Left, onExpr)
		walkWhereValueExprs(e.Right, onExpr)
		return
	}
	if e.Right != nil {
		onExpr(*e.Right)
	}
}

func rootTarget(e ir.Expr) string {
	cur := &e
	for cur != nil {
		if cur.Target != "" {
			return cur.Target
		}
		if cur.Kind == ir.ExprIdent && cur.Value != "" {
			return cur.Value
		}
		cur = cur.Left
	}
	return ""
}

func validateActionUnused(a ir.ActionDecl) []ir.Diagnostic {
	var allBindings []*varBinding

	rootScope := &unusedScope{
		bindings: make(map[string]*varBinding),
	}

	for _, p := range a.Params {
		b := &varBinding{
			name:    p.Name,
			span:    p.Span,
			isParam: true,
			used:    isParameterBoundByContract(p, a),
		}
		allBindings = append(allBindings, b)
		rootScope.bindings[p.Name] = b
	}

	processStepList(a.Steps, rootScope, &allBindings)

	var diags []ir.Diagnostic
	for _, b := range allBindings {
		if b.used {
			continue
		}
		if strings.HasPrefix(b.name, "_") {
			continue
		}

		if b.isParam {
			diags = append(diags, diagnostics.NewWithDetail(
				"FDN0228",
				b.span,
				fmt.Sprintf("action parameter %q in action %q is never read; prefix with '_' if intentionally unused", b.name, a.Name),
			))
		} else if b.isSideEffect {
			diags = append(diags, diagnostics.NewWithDetail(
				"FDN0228",
				b.span,
				fmt.Sprintf("variable %q declared in action %q is never read, but its initializer performs a side effect (%s); discard the binding with '_' or remove the let prefix", b.name, a.Name, b.stepKind),
			))
		} else {
			diags = append(diags, diagnostics.NewWithDetail(
				"FDN0228",
				b.span,
				fmt.Sprintf("variable %q declared in action %q is never read; remove the declaration or prefix with '_' to ignore", b.name, a.Name),
			))
		}
	}

	return diags
}

func isParameterBoundByContract(p ir.ParamDecl, a ir.ActionDecl) bool {
	for _, tr := range a.Traits {
		for _, v := range tr.Args {
			if v == p.Name || strings.HasPrefix(v, p.Name+".") {
				return true
			}
		}
	}
	if hasTrait(a.Traits, "event_handler") {
		return true
	}
	if p.Name == "direction" && hasStepKind(a.Steps, ir.StepQueryMany) {
		return true
	}
	if (p.Name == "expected_version" || p.Name == "version") && hasStepKind(a.Steps, ir.StepUpdate) {
		return true
	}
	return false
}

func processStepList(steps []ir.StepDecl, curScope *unusedScope, allBindings *[]*varBinding) {
	for _, step := range steps {
		processStep(step, curScope, allBindings)
	}
}

func processStep(step ir.StepDecl, curScope *unusedScope, allBindings *[]*varBinding) {
	curScope.checkStepExpressions(step)

	switch step.Kind {
	case ir.StepIf:
		if len(step.ThenSteps) > 0 {
			thenScope := &unusedScope{parent: curScope, bindings: make(map[string]*varBinding)}
			processStepList(step.ThenSteps, thenScope, allBindings)
		}
		if len(step.ElseSteps) > 0 {
			elseScope := &unusedScope{parent: curScope, bindings: make(map[string]*varBinding)}
			processStepList(step.ElseSteps, elseScope, allBindings)
		}
	case ir.StepWhile:
		if len(step.ThenSteps) > 0 {
			bodyScope := &unusedScope{parent: curScope, bindings: make(map[string]*varBinding)}
			processStepList(step.ThenSteps, bodyScope, allBindings)
		}
	case ir.StepFor:
		bodyScope := &unusedScope{parent: curScope, bindings: make(map[string]*varBinding)}
		if step.IterVar != "" {
			b := &varBinding{
				name:         step.IterVar,
				span:         step.Span,
				stepKind:     "for",
				isParam:      false,
				isSideEffect: false,
			}
			*allBindings = append(*allBindings, b)
			bodyScope.bindings[step.IterVar] = b
		}
		processStepList(step.ThenSteps, bodyScope, allBindings)
	case ir.StepMatch:
		for _, arm := range step.MatchArms {
			armScope := &unusedScope{parent: curScope, bindings: make(map[string]*varBinding)}
			processStepList(arm.Steps, armScope, allBindings)
		}
	}

	if step.VarName != "" {
		isSideEffect := step.Kind == ir.StepCreate || step.Kind == ir.StepSlot
		if curScope.parent != nil {
			existing := curScope.parent.lookup(step.VarName)
			if existing != nil && !existing.isParam {
				existing.reassigned = true
				curScope.bindings[step.VarName] = existing
				return
			}
		}

		b := &varBinding{
			name:         step.VarName,
			span:         step.Span,
			stepKind:     string(step.Kind),
			isParam:      false,
			isSideEffect: isSideEffect,
		}
		*allBindings = append(*allBindings, b)
		curScope.bindings[step.VarName] = b
	}
}

func hasStepKind(steps []ir.StepDecl, target ir.StepKind) bool {
	found := false
	ir.WalkStepsAndExprs(steps, func(s ir.StepDecl) {
		if s.Kind == target {
			found = true
		}
	}, nil)
	return found
}
