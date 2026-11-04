package validate

import (
	"fmt"
	"strings"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/diagnostics"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
)

// ActionTypeCheckPass validates Action parameters, return types, and Action Pipeline step types.
type ActionTypeCheckPass struct{}

func (ActionTypeCheckPass) Validate(sem *ir.SemanticIR, sym *SymbolTable) []ir.Diagnostic {
	var diags []ir.Diagnostic

	for i := range sem.Actions {
		a := &sem.Actions[i]
		scope := map[string]string{
			"actor": "Actor",
		}
		for pi := range a.Params {
			p := &a.Params[pi]
			kind, ok := sym.classifyType(p.Type)
			if !ok {
				diags = append(diags, diagnostics.NewWithDetail(
					"FDN0202",
					p.Span,
					fmt.Sprintf("unknown parameter type %q", p.Type),
				))
			} else {
				p.Kind = kind
			}
			scope[p.Name] = p.Type
		}
		if a.ReturnType != "" && !sym.isKnownType(a.ReturnType) {
			diags = append(diags, diagnostics.NewWithDetail(
				"FDN0202",
				a.Span,
				fmt.Sprintf("unknown return type %q", a.ReturnType),
			))
		}

		for j := range a.Steps {
			validateActionStep(*a, &a.Steps[j], scope, sym, &diags)
		}
		sym.actions[a.Name] = *a
	}

	return diags
}

func validateActionStep(a ir.ActionDecl, s *ir.StepDecl, scope map[string]string, sym *SymbolTable, diags *[]ir.Diagnostic) {
	switch s.Kind {
	case ir.StepRequire:
		validateRequireStep(s, scope, sym, diags)

	case ir.StepGuard:
		if s.Expr != nil {
			condType, ok := inferExprType(s.Expr, scope, sym, diags)
			if ok && condType != "bool" {
				*diags = append(*diags, diagnostics.NewWithDetail(
					"FDN0205",
					s.Expr.Span,
					fmt.Sprintf("guard condition must be bool, got %s", condType),
				))
			}
		}
		validateErrorRef(s.ElseError, "guard else", s.Span, sym, diags)

	case ir.StepEmit:
		ev, ok := sym.events[s.EventName]
		if !ok {
			*diags = append(*diags, diagnostics.NewWithDetail(
				"FDN0203",
				s.Span,
				fmt.Sprintf("undeclared event %q in emit", s.EventName),
			))
			return
		}
		validateFieldInitializers("event", ev.Name, s.Fields, scope, sym, diags)

	case ir.StepQuery:
		ent, ok := validateStepWhere("query", s, scope, sym, diags)
		if !ok {
			return
		}
		if s.ElseError == "" {
			*diags = append(*diags, diagnostics.NewWithDetail(
				"FDN0206",
				s.Span,
				"query one requires an 'else <Error>' clause",
			))
		} else {
			validateErrorRef(s.ElseError, "query else", s.Span, sym, diags)
		}
		scope[s.VarName] = ent.Name

	case ir.StepQueryMany:
		ent, ok := sym.entities[s.EntityName]
		if !ok {
			*diags = append(*diags, diagnostics.NewWithDetail(
				"FDN0203",
				s.Span,
				fmt.Sprintf("unknown entity %q in query many", s.EntityName),
			))
			return
		}
		if s.WhereExpr != nil || len(s.WherePredicates) > 0 {
			validateStepWhere("query many", s, scope, sym, diags)
		}
		if s.CursorExpr != nil {
			inferExprType(s.CursorExpr, scope, sym, diags)
		}
		if s.LimitExpr != nil {
			inferExprType(s.LimitExpr, scope, sym, diags)
		}
		scope[s.VarName] = "[]" + ent.Name

	case ir.StepCreate:
		ent, ok := sym.entities[s.EntityName]
		if !ok {
			*diags = append(*diags, diagnostics.NewWithDetail(
				"FDN0203",
				s.Span,
				fmt.Sprintf("unknown entity %q in create", s.EntityName),
			))
			return
		}
		validateFieldInitializers("entity", ent.Name, s.Fields, scope, sym, diags)
		scope[s.VarName] = ent.Name

	case ir.StepUpdate:
		ent, ok := validateStepWhere("update", s, scope, sym, diags)
		if !ok {
			return
		}
		validateFieldInitializers("entity", ent.Name, s.Fields, scope, sym, diags)

	case ir.StepDelete:
		validateStepWhere("delete", s, scope, sym, diags)

	case ir.StepIf:
		if s.Expr != nil {
			condType, ok := inferExprType(s.Expr, scope, sym, diags)
			if ok && condType != "bool" {
				*diags = append(*diags, diagnostics.NewWithDetail(
					"FDN0205",
					s.Expr.Span,
					fmt.Sprintf("if condition must be bool, got %s", condType),
				))
			}
		}
		thenScope := cloneScope(scope)
		for i := range s.ThenSteps {
			validateActionStep(a, &s.ThenSteps[i], thenScope, sym, diags)
		}
		elseScope := cloneScope(scope)
		for i := range s.ElseSteps {
			validateActionStep(a, &s.ElseSteps[i], elseScope, sym, diags)
		}

	case ir.StepMatch:
		validateMatchStep(a, s, scope, sym, diags)

	case ir.StepFor:
		var elemType string
		if s.Expr != nil {
			collType, ok := inferExprType(s.Expr, scope, sym, diags)
			if ok {
				if !strings.HasPrefix(collType, "[]") {
					*diags = append(*diags, diagnostics.NewWithDetail(
						"FDN0205",
						s.Expr.Span,
						fmt.Sprintf("for loop collection must be a list type, got %s", collType),
					))
				} else {
					elemType = strings.TrimPrefix(collType, "[]")
				}
			}
		}
		bodyScope := cloneScope(scope)
		if elemType != "" {
			bodyScope[s.IterVar] = elemType
		}
		for i := range s.ThenSteps {
			validateActionStep(a, &s.ThenSteps[i], bodyScope, sym, diags)
		}

	case ir.StepLet:
		if s.Expr != nil {
			if t, ok := inferExprType(s.Expr, scope, sym, diags); ok {
				scope[s.VarName] = t
			}
		}

	case ir.StepSlot:
		validateSlotStep(s, scope, sym, diags)

	case ir.StepReturn:
		if s.Expr != nil {
			sym.resolveEnumLiteral(s.Expr, a.ReturnType)
			t, ok := inferExprType(s.Expr, scope, sym, diags)
			if ok && a.ReturnType != "" && !isAssignableType(a.ReturnType, t) {
				*diags = append(*diags, diagnostics.NewWithDetail(
					"FDN0205",
					s.Expr.Span,
					fmt.Sprintf("return type mismatch: action %q returns %s, got %s", a.Name, a.ReturnType, t),
				))
			}
		}
	}
}

func validateErrorRef(errName, contextLabel string, span ir.Span, sym *SymbolTable, diags *[]ir.Diagnostic) {
	if _, ok := sym.errors[errName]; !ok {
		*diags = append(*diags, diagnostics.NewWithDetail(
			"FDN0206",
			span,
			fmt.Sprintf("undeclared error %q in %s", errName, contextLabel),
		))
	}
}

func validateEntityWhereClause(opName, entityName, whereField string, whereVal *ir.Expr, span ir.Span, scope map[string]string, sym *SymbolTable, diags *[]ir.Diagnostic) (ir.EntityDecl, bool) {
	ent, ok := sym.entities[entityName]
	if !ok {
		*diags = append(*diags, diagnostics.NewWithDetail(
			"FDN0203",
			span,
			fmt.Sprintf("unknown entity %q in %s", entityName, opName),
		))
		return ir.EntityDecl{}, false
	}
	fields := sym.fieldsOf(ent.Name)
	wf, fOk := fields[whereField]
	if !fOk {
		*diags = append(*diags, diagnostics.New("FDN0204", span, "entity", ent.Name, whereField))
	} else if whereVal != nil {
		sym.resolveEnumLiteral(whereVal, wf.Type)
		valType, ok := inferExprType(whereVal, scope, sym, diags)
		if ok && !isAssignableType(wf.Type, valType) {
			*diags = append(*diags, diagnostics.NewWithDetail(
				"FDN0205",
				whereVal.Span,
				fmt.Sprintf("type mismatch in %s where clause: field %q is %s, got %s", opName, whereField, wf.Type, valType),
			))
		}
	}
	return ent, true
}

func validateSlotStep(s *ir.StepDecl, scope map[string]string, sym *SymbolTable, diags *[]ir.Diagnostic) {
	sl, m, ok := sym.findSlotMethod(s.SlotName, s.SlotMethod)
	if !ok {
		if _, slotExists := sym.slots[s.SlotName]; !slotExists {
			*diags = append(*diags, diagnostics.NewWithDetail(
				"FDN0203",
				s.Span,
				fmt.Sprintf("undeclared slot %q", s.SlotName),
			))
		} else {
			*diags = append(*diags, diagnostics.NewWithDetail(
				"FDN0204",
				s.Span,
				fmt.Sprintf("slot %q has no method %q", sl.Name, s.SlotMethod),
			))
		}
		return
	}
	validateSlotCallArgs(sl, m, s.SlotArgs, s.Span, scope, sym, diags)
	if s.ElseError != "" {
		validateErrorRef(s.ElseError, fmt.Sprintf("slot %s.%s else", sl.Name, m.Name), s.Span, sym, diags)
	}
	if s.VarName != "" && m.ReturnType != "" {
		scope[s.VarName] = m.ReturnType
	}
}

func validateSlotCallArgs(sl ir.SlotDecl, m ir.SlotMethodDecl, args []ir.Expr, span ir.Span, scope map[string]string, sym *SymbolTable, diags *[]ir.Diagnostic) {
	if len(args) != len(m.Params) {
		*diags = append(*diags, diagnostics.NewWithDetail(
			"FDN0205",
			span,
			fmt.Sprintf("slot %s.%s expects %d arguments, got %d", sl.Name, m.Name, len(m.Params), len(args)),
		))
		return
	}
	for idx := range args {
		expectedType := m.Params[idx].Type
		sym.resolveEnumLiteral(&args[idx], expectedType)
		argType, ok := inferExprType(&args[idx], scope, sym, diags)
		if ok && !isAssignableType(expectedType, argType) {
			*diags = append(*diags, diagnostics.NewWithDetail(
				"FDN0205",
				args[idx].Span,
				fmt.Sprintf("argument %d to slot %s.%s has type %s, expected %s", idx+1, sl.Name, m.Name, argType, expectedType),
			))
		}
	}
}

func validateRequireStep(s *ir.StepDecl, scope map[string]string, sym *SymbolTable, diags *[]ir.Diagnostic) {
	pol, ok := sym.policies[s.PolicyName]
	if !ok {
		*diags = append(*diags, diagnostics.New("FDN0209", s.Span, s.PolicyName))
	} else if len(s.PolicyArgs) != len(pol.Params) {
		*diags = append(*diags, diagnostics.NewWithDetail(
			"FDN0205",
			s.Span,
			fmt.Sprintf("policy %q expects %d arguments, got %d", pol.Name, len(pol.Params), len(s.PolicyArgs)),
		))
	} else {
		for idx := range s.PolicyArgs {
			expectedType := pol.Params[idx].Type
			sym.resolveEnumLiteral(&s.PolicyArgs[idx], expectedType)
			argType, ok := inferExprType(&s.PolicyArgs[idx], scope, sym, diags)
			if ok && !isAssignableType(expectedType, argType) {
				*diags = append(*diags, diagnostics.NewWithDetail(
					"FDN0205",
					s.PolicyArgs[idx].Span,
					fmt.Sprintf("argument %d to policy %q has type %s, expected %s", idx+1, pol.Name, argType, expectedType),
				))
			}
		}
	}
	validateErrorRef(s.ElseError, "require else", s.Span, sym, diags)
}

func validateMatchStep(a ir.ActionDecl, s *ir.StepDecl, scope map[string]string, sym *SymbolTable, diags *[]ir.Diagnostic) {
	if s.Expr == nil {
		return
	}
	targetType, ok := inferExprType(s.Expr, scope, sym, diags)
	if !ok {
		return
	}
	en, isEnum := sym.enums[targetType]
	if !isEnum {
		*diags = append(*diags, diagnostics.NewWithDetail(
			"FDN0205",
			s.Expr.Span,
			fmt.Sprintf("match target must be an enum type, got %s", targetType),
		))
		return
	}
	validVariants := make(map[string]bool, len(en.Variants))
	for _, v := range en.Variants {
		validVariants[v] = true
	}
	covered := make(map[string]bool, len(s.MatchArms))
	for i := range s.MatchArms {
		arm := &s.MatchArms[i]
		if !validVariants[arm.Variant] {
			*diags = append(*diags, diagnostics.NewWithDetail(
				"FDN0208",
				arm.Span,
				fmt.Sprintf("enum %q has no variant .%s", en.Name, arm.Variant),
			))
		}
		covered[arm.Variant] = true
		armScope := cloneScope(scope)
		for j := range arm.Steps {
			validateActionStep(a, &arm.Steps[j], armScope, sym, diags)
		}
	}
	var missing []string
	for _, v := range en.Variants {
		if !covered[v] {
			missing = append(missing, "."+v)
		}
	}
	if len(missing) > 0 {
		*diags = append(*diags, diagnostics.New("FDN0212", s.Span, en.Name, strings.Join(missing, ", ")))
	}
}
