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

		for j := range sc.GivenSlots {
			gs := &sc.GivenSlots[j]
			sl, m, ok := sym.findSlotMethod(gs.SlotName, gs.MethodName)
			if !ok {
				diags = append(diags, ir.Diagnostic{
					Code:     "FDN0203",
					Severity: ir.SeverityError,
					Message:  fmt.Sprintf("unknown slot method %s.%s in scenario given slot", gs.SlotName, gs.MethodName),
					Span:     gs.Span,
				})
				continue
			}
			if len(gs.Args) > 0 {
				validateSlotCallArgs(sl, m, gs.Args, gs.Span, nil, sym, &diags)
			}
			if gs.ReturnExpr != nil {
				sym.resolveEnumLiteral(gs.ReturnExpr, m.ReturnType)
				retType, ok := inferExprType(gs.ReturnExpr, nil, sym, &diags)
				if ok && m.ReturnType != "" && !isAssignableType(m.ReturnType, retType) {
					diags = append(diags, ir.Diagnostic{
						Code:     "FDN0205",
						Severity: ir.SeverityError,
						Message:  fmt.Sprintf("slot %s.%s returns %s, got %s", sl.Name, m.Name, m.ReturnType, retType),
						Span:     gs.ReturnExpr.Span,
					})
				}
			}
			if gs.ThrowsError != "" {
				validateErrorRef(gs.ThrowsError, fmt.Sprintf("scenario given slot %s.%s throws", sl.Name, m.Name), gs.Span, sym, &diags)
				inContract := false
				for _, th := range m.Throws {
					if th == gs.ThrowsError {
						inContract = true
						break
					}
				}
				if !inContract {
					diags = append(diags, ir.Diagnostic{
						Code:     "FDN0206",
						Severity: ir.SeverityError,
						Message:  fmt.Sprintf("error %q is not declared in slot %s.%s throws contract", gs.ThrowsError, sl.Name, m.Name),
						Span:     gs.Span,
					})
				}
			}
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
				} else if ok && paramType == "timestamp" && sc.WhenArgs[k].Kind == ir.ExprString {
					sc.WhenArgs[k].InferredType = "timestamp"
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
				ent, sOk := validateExpectWhere("scenario expect stored", ex, expectScope, sym, &diags)
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
