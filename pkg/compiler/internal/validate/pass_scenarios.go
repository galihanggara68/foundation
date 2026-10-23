package validate

import (
	"fmt"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
)

// ScenarioTypeCheckPass validates Scenario given seeds, when action invocations, and expect assertions.
type ScenarioTypeCheckPass struct{}

func (ScenarioTypeCheckPass) Validate(sem *ir.SemanticIR, sym *SymbolTable) []ir.Diagnostic {
	var diags []ir.Diagnostic

	for i := range sem.Scenarios {
		sc := &sem.Scenarios[i]
		for j := range sc.Givens {
			g := &sc.Givens[j]
			if g.EntityName == "actor" {
				validateFieldInitializers("type", "Actor", g.Fields, nil, sym, &diags)
				continue
			}
			ent, ok := sym.entities[g.EntityName]
			if !ok {
				diags = append(diags, ir.Diagnostic{
					Code:     "FDN0203",
					Severity: ir.SeverityError,
					Message:  fmt.Sprintf("unknown entity %q in scenario given", g.EntityName),
					Span:     g.Span,
				})
				continue
			}
			validateFieldInitializers("entity", ent.Name, g.Fields, nil, sym, &diags)
		}

		act, ok := sym.actions[sc.WhenAction]
		if !ok {
			diags = append(diags, ir.Diagnostic{
				Code:     "FDN0207",
				Severity: ir.SeverityError,
				Message:  fmt.Sprintf("unknown action %q in scenario when", sc.WhenAction),
				Span:     sc.Span,
			})
			continue
		}

		if len(sc.WhenArgs) != len(act.Params) {
			diags = append(diags, ir.Diagnostic{
				Code:     "FDN0205",
				Severity: ir.SeverityError,
				Message:  fmt.Sprintf("action %q expects %d arguments, got %d", act.Name, len(act.Params), len(sc.WhenArgs)),
				Span:     sc.Span,
			})
		} else {
			for k := range sc.WhenArgs {
				paramType := act.Params[k].Type
				sym.resolveEnumLiteral(&sc.WhenArgs[k], paramType)
				argType, ok := inferExprType(&sc.WhenArgs[k], nil, sym, &diags)
				if ok && !isAssignableType(paramType, argType) {
					diags = append(diags, ir.Diagnostic{
						Code:     "FDN0205",
						Severity: ir.SeverityError,
						Message:  fmt.Sprintf("argument %d to %q has type %s, expected %s", k+1, act.Name, argType, paramType),
						Span:     sc.WhenArgs[k].Span,
					})
				}
			}
		}

		expectScope := map[string]string{
			"result": act.ReturnType,
		}
		for k := range sc.Expects {
			ex := &sc.Expects[k]
			if ex.ErrorRaised != "" {
				validateErrorRef(ex.ErrorRaised, "scenario expect", ex.Span, sym, &diags)
				continue
			}
			if ex.EmittedEvent != "" {
				ev, evOk := sym.events[ex.EmittedEvent]
				if !evOk {
					diags = append(diags, ir.Diagnostic{
						Code:     "FDN0203",
						Severity: ir.SeverityError,
						Message:  fmt.Sprintf("undeclared event %q in scenario expect emitted", ex.EmittedEvent),
						Span:     ex.Span,
					})
					continue
				}
				validateFieldInitializers("event", ev.Name, ex.EmittedFields, expectScope, sym, &diags)
				continue
			}
			if ex.StoredEntity != "" {
				ent, sOk := validateEntityWhereClause("scenario expect stored", ex.StoredEntity, ex.WhereField, ex.WhereValue, ex.Span, expectScope, sym, &diags)
				if sOk && len(ex.StoredFields) > 0 {
					validateFieldInitializers("entity", ent.Name, ex.StoredFields, expectScope, sym, &diags)
				}
				continue
			}
			lt, lok := inferExprType(&ex.Left, expectScope, sym, &diags)
			if lok {
				sym.resolveEnumLiteral(&ex.Right, lt)
			}
			rt, rok := inferExprType(&ex.Right, expectScope, sym, &diags)
			if lok && rok && !isAssignableType(lt, rt) {
				diags = append(diags, ir.Diagnostic{
					Code:     "FDN0205",
					Severity: ir.SeverityError,
					Message:  fmt.Sprintf("scenario expect type mismatch: %s vs %s", lt, rt),
					Span:     ex.Span,
				})
			}
		}
	}

	return diags
}
