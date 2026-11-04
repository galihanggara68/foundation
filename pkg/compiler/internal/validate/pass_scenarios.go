package validate

import (
	"fmt"
	"strings"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/diagnostics"
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
				diags = append(diags, diagnostics.NewWithDetail(
					"FDN0203",
					g.Span,
					fmt.Sprintf("unknown entity %q in scenario given", g.EntityName),
				))
				continue
			}
			validateFieldInitializers("entity", ent.Name, g.Fields, nil, sym, &diags)
		}

		for j := range sc.GivenSlots {
			gs := &sc.GivenSlots[j]
			sl, m, ok := sym.findSlotMethod(gs.SlotName, gs.MethodName)
			if !ok {
				diags = append(diags, diagnostics.NewWithDetail(
					"FDN0203",
					gs.Span,
					fmt.Sprintf("unknown slot method %s.%s in scenario given slot", gs.SlotName, gs.MethodName),
				))
				continue
			}
			if len(gs.Args) > 0 {
				validateSlotCallArgs(sl, m, gs.Args, gs.Span, nil, sym, &diags)
			}
			if gs.ReturnExpr != nil {
				sym.resolveEnumLiteral(gs.ReturnExpr, m.ReturnType)
				retType, ok := inferExprType(gs.ReturnExpr, nil, sym, &diags)
				if ok && m.ReturnType != "" && !isAssignableType(m.ReturnType, retType) {
					diags = append(diags, diagnostics.NewWithDetail(
						"FDN0205",
						gs.ReturnExpr.Span,
						fmt.Sprintf("slot %s.%s returns %s, got %s", sl.Name, m.Name, m.ReturnType, retType),
					))
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
					diags = append(diags, diagnostics.NewWithDetail(
						"FDN0206",
						gs.Span,
						fmt.Sprintf("error %q is not declared in slot %s.%s throws contract", gs.ThrowsError, sl.Name, m.Name),
					))
				}
			}
		}

		act, ok := sym.actions[sc.WhenAction]
		if !ok {
			diags = append(diags, diagnostics.New("FDN0207", sc.Span, sc.WhenAction))
			continue
		}

		if len(sc.WhenArgs) != len(act.Params) {
			diags = append(diags, diagnostics.NewWithDetail(
				"FDN0205",
				sc.Span,
				fmt.Sprintf("action %q expects %d arguments, got %d", act.Name, len(act.Params), len(sc.WhenArgs)),
			))
		} else {
			for k := range sc.WhenArgs {
				paramType := act.Params[k].Type
				sym.resolveEnumLiteral(&sc.WhenArgs[k], paramType)
				argType, ok := inferExprType(&sc.WhenArgs[k], nil, sym, &diags)
				if ok && !isAssignableType(paramType, argType) {
					diags = append(diags, diagnostics.NewWithDetail(
						"FDN0205",
						sc.WhenArgs[k].Span,
						fmt.Sprintf("argument %d to %q has type %s, expected %s", k+1, act.Name, argType, paramType),
					))
				} else if ok && paramType == "timestamp" && sc.WhenArgs[k].Kind == ir.ExprString {
					sc.WhenArgs[k].InferredType = "timestamp"
				}
			}
		}

		expectScope := map[string]string{}
		if strings.HasPrefix(act.ReturnType, "[]") {
			expectScope["result"] = act.Name + "Result"
		} else {
			expectScope["result"] = act.ReturnType
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
					diags = append(diags, diagnostics.NewWithDetail(
						"FDN0231",
						ex.Span,
						fmt.Sprintf("undeclared event %q in scenario expect emitted", ex.EmittedEvent),
					))
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
				diags = append(diags, diagnostics.NewWithDetail(
					"FDN0205",
					ex.Span,
					fmt.Sprintf("scenario expect type mismatch: %s vs %s", lt, rt),
				))
			}
		}
	}

	return diags
}
