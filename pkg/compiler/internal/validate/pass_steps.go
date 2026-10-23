package validate

import (
	"fmt"
	"strings"

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
		for _, p := range a.Params {
			if !sym.isKnownType(p.Type) {
				diags = append(diags, ir.Diagnostic{
					Code:     "FDN0202",
					Severity: ir.SeverityError,
					Message:  fmt.Sprintf("unknown parameter type %q", p.Type),
					Span:     p.Span,
				})
			}
			scope[p.Name] = p.Type
		}
		if a.ReturnType != "" && !sym.isKnownType(a.ReturnType) {
			diags = append(diags, ir.Diagnostic{
				Code:     "FDN0202",
				Severity: ir.SeverityError,
				Message:  fmt.Sprintf("unknown return type %q", a.ReturnType),
				Span:     a.Span,
			})
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
				*diags = append(*diags, ir.Diagnostic{
					Code:     "FDN0205",
					Severity: ir.SeverityError,
					Message:  fmt.Sprintf("guard condition must be bool, got %s", condType),
					Span:     s.Expr.Span,
				})
			}
		}
		validateErrorRef(s.ElseError, "guard else", s.Span, sym, diags)

	case ir.StepEmit:
		ev, ok := sym.events[s.EventName]
		if !ok {
			*diags = append(*diags, ir.Diagnostic{
				Code:     "FDN0203",
				Severity: ir.SeverityError,
				Message:  fmt.Sprintf("undeclared event %q in emit", s.EventName),
				Span:     s.Span,
			})
			return
		}
		validateFieldInitializers("event", ev.Name, s.Fields, scope, sym, diags)

	case ir.StepQuery:
		ent, ok := validateEntityWhereClause("query", s.EntityName, s.WhereField, s.WhereValue, s.Span, scope, sym, diags)
		if !ok {
			return
		}
		if s.ElseError == "" {
			*diags = append(*diags, ir.Diagnostic{
				Code:     "FDN0206",
				Severity: ir.SeverityError,
				Message:  "query one requires an 'else <Error>' clause",
				Span:     s.Span,
			})
		} else {
			validateErrorRef(s.ElseError, "query else", s.Span, sym, diags)
		}
		scope[s.VarName] = ent.Name

	case ir.StepCreate:
		ent, ok := sym.entities[s.EntityName]
		if !ok {
			*diags = append(*diags, ir.Diagnostic{
				Code:     "FDN0203",
				Severity: ir.SeverityError,
				Message:  fmt.Sprintf("unknown entity %q in create", s.EntityName),
				Span:     s.Span,
			})
			return
		}
		validateFieldInitializers("entity", ent.Name, s.Fields, scope, sym, diags)
		scope[s.VarName] = ent.Name

	case ir.StepUpdate:
		ent, ok := validateEntityWhereClause("update", s.EntityName, s.WhereField, s.WhereValue, s.Span, scope, sym, diags)
		if !ok {
			return
		}
		validateFieldInitializers("entity", ent.Name, s.Fields, scope, sym, diags)

	case ir.StepDelete:
		validateEntityWhereClause("delete", s.EntityName, s.WhereField, s.WhereValue, s.Span, scope, sym, diags)

	case ir.StepIf:
		if s.Expr != nil {
			condType, ok := inferExprType(s.Expr, scope, sym, diags)
			if ok && condType != "bool" {
				*diags = append(*diags, ir.Diagnostic{
					Code:     "FDN0205",
					Severity: ir.SeverityError,
					Message:  fmt.Sprintf("if condition must be bool, got %s", condType),
					Span:     s.Expr.Span,
				})
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
					*diags = append(*diags, ir.Diagnostic{
						Code:     "FDN0205",
						Severity: ir.SeverityError,
						Message:  fmt.Sprintf("for loop collection must be a list type, got %s", collType),
						Span:     s.Expr.Span,
					})
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
				*diags = append(*diags, ir.Diagnostic{
					Code:     "FDN0205",
					Severity: ir.SeverityError,
					Message:  fmt.Sprintf("return type mismatch: action %q returns %s, got %s", a.Name, a.ReturnType, t),
					Span:     s.Expr.Span,
				})
			}
		}
	}
}

func validateErrorRef(errName, contextLabel string, span ir.Span, sym *SymbolTable, diags *[]ir.Diagnostic) {
	if _, ok := sym.errors[errName]; !ok {
		*diags = append(*diags, ir.Diagnostic{
			Code:     "FDN0206",
			Severity: ir.SeverityError,
			Message:  fmt.Sprintf("undeclared error %q in %s", errName, contextLabel),
			Span:     span,
		})
	}
}

func validateEntityWhereClause(opName, entityName, whereField string, whereVal *ir.Expr, span ir.Span, scope map[string]string, sym *SymbolTable, diags *[]ir.Diagnostic) (ir.EntityDecl, bool) {
	ent, ok := sym.entities[entityName]
	if !ok {
		*diags = append(*diags, ir.Diagnostic{
			Code:     "FDN0203",
			Severity: ir.SeverityError,
			Message:  fmt.Sprintf("unknown entity %q in %s", entityName, opName),
			Span:     span,
		})
		return ir.EntityDecl{}, false
	}
	fields := sym.fieldsOf(ent.Name)
	wf, fOk := fields[whereField]
	if !fOk {
		*diags = append(*diags, ir.Diagnostic{
			Code:     "FDN0204",
			Severity: ir.SeverityError,
			Message:  fmt.Sprintf("entity %q has no field %q", ent.Name, whereField),
			Span:     span,
		})
	} else if whereVal != nil {
		sym.resolveEnumLiteral(whereVal, wf.Type)
		valType, ok := inferExprType(whereVal, scope, sym, diags)
		if ok && !isAssignableType(wf.Type, valType) {
			*diags = append(*diags, ir.Diagnostic{
				Code:     "FDN0205",
				Severity: ir.SeverityError,
				Message:  fmt.Sprintf("type mismatch in %s where clause: field %q is %s, got %s", opName, whereField, wf.Type, valType),
				Span:     whereVal.Span,
			})
		}
	}
	return ent, true
}

func validateSlotStep(s *ir.StepDecl, scope map[string]string, sym *SymbolTable, diags *[]ir.Diagnostic) {
	sl, m, ok := sym.findSlotMethod(s.SlotName, s.SlotMethod)
	if !ok {
		if _, slotExists := sym.slots[s.SlotName]; !slotExists {
			*diags = append(*diags, ir.Diagnostic{
				Code:     "FDN0203",
				Severity: ir.SeverityError,
				Message:  fmt.Sprintf("undeclared slot %q", s.SlotName),
				Span:     s.Span,
			})
		} else {
			*diags = append(*diags, ir.Diagnostic{
				Code:     "FDN0204",
				Severity: ir.SeverityError,
				Message:  fmt.Sprintf("slot %q has no method %q", sl.Name, s.SlotMethod),
				Span:     s.Span,
			})
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
		*diags = append(*diags, ir.Diagnostic{
			Code:     "FDN0205",
			Severity: ir.SeverityError,
			Message:  fmt.Sprintf("slot %s.%s expects %d arguments, got %d", sl.Name, m.Name, len(m.Params), len(args)),
			Span:     span,
		})
		return
	}
	for idx := range args {
		expectedType := m.Params[idx].Type
		sym.resolveEnumLiteral(&args[idx], expectedType)
		argType, ok := inferExprType(&args[idx], scope, sym, diags)
		if ok && !isAssignableType(expectedType, argType) {
			*diags = append(*diags, ir.Diagnostic{
				Code:     "FDN0205",
				Severity: ir.SeverityError,
				Message:  fmt.Sprintf("argument %d to slot %s.%s has type %s, expected %s", idx+1, sl.Name, m.Name, argType, expectedType),
				Span:     args[idx].Span,
			})
		}
	}
}

func validateRequireStep(s *ir.StepDecl, scope map[string]string, sym *SymbolTable, diags *[]ir.Diagnostic) {
	pol, ok := sym.policies[s.PolicyName]
	if !ok {
		*diags = append(*diags, ir.Diagnostic{
			Code:     "FDN0209",
			Severity: ir.SeverityError,
			Message:  fmt.Sprintf("undeclared policy %q in require", s.PolicyName),
			Span:     s.Span,
		})
	} else if len(s.PolicyArgs) != len(pol.Params) {
		*diags = append(*diags, ir.Diagnostic{
			Code:     "FDN0205",
			Severity: ir.SeverityError,
			Message:  fmt.Sprintf("policy %q expects %d arguments, got %d", pol.Name, len(pol.Params), len(s.PolicyArgs)),
			Span:     s.Span,
		})
	} else {
		for idx := range s.PolicyArgs {
			expectedType := pol.Params[idx].Type
			sym.resolveEnumLiteral(&s.PolicyArgs[idx], expectedType)
			argType, ok := inferExprType(&s.PolicyArgs[idx], scope, sym, diags)
			if ok && !isAssignableType(expectedType, argType) {
				*diags = append(*diags, ir.Diagnostic{
					Code:     "FDN0205",
					Severity: ir.SeverityError,
					Message:  fmt.Sprintf("argument %d to policy %q has type %s, expected %s", idx+1, pol.Name, argType, expectedType),
					Span:     s.PolicyArgs[idx].Span,
				})
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
		*diags = append(*diags, ir.Diagnostic{
			Code:     "FDN0205",
			Severity: ir.SeverityError,
			Message:  fmt.Sprintf("match target must be an enum type, got %s", targetType),
			Span:     s.Expr.Span,
		})
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
			*diags = append(*diags, ir.Diagnostic{
				Code:     "FDN0208",
				Severity: ir.SeverityError,
				Message:  fmt.Sprintf("enum %q has no variant .%s", en.Name, arm.Variant),
				Span:     arm.Span,
			})
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
		*diags = append(*diags, ir.Diagnostic{
			Code:     "FDN0212",
			Severity: ir.SeverityError,
			Message:  fmt.Sprintf("non-exhaustive match over enum %q: missing variants %s", en.Name, strings.Join(missing, ", ")),
			Span:     s.Span,
		})
	}
}
