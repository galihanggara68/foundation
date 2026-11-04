package validate

import (
	"fmt"
	"strings"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/diagnostics"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
)

// InvariantPass checks compile-time architectural invariants (ADR 0004, ADR 0021):
// 1. Forbids calling @external slot methods inside @transactional actions (FDN0213).
// 2. Flags relational operations inside bounded loops (FDN0214).
// 3. Flags actions with multiple writes on an execution path without @transactional (FDN0227).
type InvariantPass struct{}

func (InvariantPass) Validate(sem *ir.SemanticIR, sym *SymbolTable) []ir.Diagnostic {
	var diags []ir.Diagnostic

	for _, a := range sem.Actions {
		isTx := hasTrait(a.Traits, "transactional")
		validateStepsInvariants(a.Name, a.Steps, isTx, false, sym, &diags)
		if !isTx {
			if diag := checkMultiWrite(a); diag != nil {
				diags = append(diags, *diag)
			}
		}
	}

	return diags
}

func hasTrait(traits []ir.TraitDecl, name string) bool {
	for _, tr := range traits {
		if tr.Name == name {
			return true
		}
	}
	return false
}

func validateStepsInvariants(actionName string, steps []ir.StepDecl, inTx bool, inLoop bool, sym *SymbolTable, diags *[]ir.Diagnostic) {
	for _, s := range steps {
		if inTx && s.Kind == ir.StepSlot {
			_, m, ok := sym.findSlotMethod(s.SlotName, s.SlotMethod)
			if ok && hasTrait(m.Traits, "external") {
				*diags = append(*diags, diagnostics.NewWithDetail(
					"FDN0213",
					s.Span,
					fmt.Sprintf("calling @external slot method %s.%s inside @transactional action %q is forbidden (holds DB transaction locks across network I/O)", s.SlotName, s.SlotMethod, actionName),
				))
			}
		}

		if inLoop {
			switch s.Kind {
			case ir.StepQuery, ir.StepCreate, ir.StepUpdate, ir.StepDelete:
				*diags = append(*diags, diagnostics.NewWithDetail(
					"FDN0214",
					s.Span,
					fmt.Sprintf("N+1 query hazard: %q on entity %q inside bounded 'for' loop in action %q is forbidden; use bulk set operations outside the loop", s.Kind, s.EntityName, actionName),
				))
			}
		}

		nextLoop := inLoop || s.Kind == ir.StepFor || s.Kind == ir.StepWhile
		validateStepsInvariants(actionName, s.ThenSteps, inTx, nextLoop, sym, diags)
		validateStepsInvariants(actionName, s.ElseSteps, inTx, nextLoop, sym, diags)
		for _, arm := range s.MatchArms {
			validateStepsInvariants(actionName, arm.Steps, inTx, nextLoop, sym, diags)
		}
	}
}

type pathState struct {
	writes     []string
	inLoop     bool
	terminated bool
}

// checkMultiWrite evaluates execution paths of an action for multiple write steps.
// It returns a diagnostic when a path contains two or more write steps without @transactional.
func checkMultiWrite(a ir.ActionDecl) *ir.Diagnostic {
	allPaths := evaluateStepPaths(a.Steps, pathState{})
	for _, st := range allPaths {
		if len(st.writes) >= 2 || (st.inLoop && len(st.writes) >= 1) {
			diag := diagnostics.NewWithDetail(
				"FDN0227",
				a.Span,
				fmt.Sprintf("action %q performs multiple write operations (%s) along an execution path without @transactional; wrap the action in @transactional to guarantee atomicity", a.Name, strings.Join(st.writes, ", ")),
			)
			return &diag
		}
	}
	return nil
}

// evaluateStepPaths traces write operations across all execution paths in a sequence of steps.
func evaluateStepPaths(steps []ir.StepDecl, initial pathState) []pathState {
	states := []pathState{initial}
	for _, s := range steps {
		var nextStates []pathState
		for _, st := range states {
			if st.terminated {
				nextStates = append(nextStates, st)
				continue
			}

			switch s.Kind {
			case ir.StepCreate, ir.StepUpdate, ir.StepDelete:
				w := make([]string, len(st.writes)+1)
				copy(w, st.writes)
				w[len(st.writes)] = string(s.Kind)
				nextStates = append(nextStates, pathState{writes: w, inLoop: st.inLoop})

			case ir.StepReturn:
				nextStates = append(nextStates, pathState{writes: st.writes, inLoop: st.inLoop, terminated: true})

			case ir.StepIf:
				thenStates := evaluateStepPaths(s.ThenSteps, st)
				nextStates = append(nextStates, thenStates...)
				if len(s.ElseSteps) > 0 {
					elseStates := evaluateStepPaths(s.ElseSteps, st)
					nextStates = append(nextStates, elseStates...)
				} else {
					nextStates = append(nextStates, st)
				}

			case ir.StepMatch:
				if len(s.MatchArms) == 0 {
					nextStates = append(nextStates, st)
				} else {
					for _, arm := range s.MatchArms {
						armStates := evaluateStepPaths(arm.Steps, st)
						nextStates = append(nextStates, armStates...)
					}
				}

			case ir.StepFor, ir.StepWhile:
				bodyStates := evaluateStepPaths(s.ThenSteps, pathState{})
				// A loop might execute 0 iterations; preserve the zero-write path:
				nextStates = append(nextStates, st)
				for _, bs := range bodyStates {
					if len(bs.writes) > 0 {
						w := append(append([]string{}, st.writes...), bs.writes...)
						nextStates = append(nextStates, pathState{
							writes:     w,
							inLoop:     true,
							terminated: bs.terminated,
						})
					}
				}

			default:
				// StepLet, StepQuery, StepQueryMany, StepGuard, StepRequire, StepEmit, StepSlot
				nextStates = append(nextStates, st)
			}
		}
		states = nextStates
	}
	return states
}
