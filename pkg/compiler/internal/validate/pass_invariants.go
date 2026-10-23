package validate

import (
	"fmt"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
)

// InvariantPass enforces compile-time architectural invariants (ADR 0004):
// 1. Forbids calling @external Slot methods inside @transactional Action Pipelines (FDN0213).
// 2. Flags relational query/create/update/delete operations inside bounded 'for' loops (FDN0214 N+1 guardrail).
type InvariantPass struct{}

func (InvariantPass) Validate(sem *ir.SemanticIR, sym *SymbolTable) []ir.Diagnostic {
	var diags []ir.Diagnostic

	for _, a := range sem.Actions {
		isTx := hasTrait(a.Traits, "transactional")
		validateStepsInvariants(a.Name, a.Steps, isTx, false, sym, &diags)
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
				*diags = append(*diags, ir.Diagnostic{
					Code:     "FDN0213",
					Severity: ir.SeverityError,
					Message:  fmt.Sprintf("calling @external slot method %s.%s inside @transactional action %q is forbidden (holds DB transaction locks across network I/O)", s.SlotName, s.SlotMethod, actionName),
					Span:     s.Span,
				})
			}
		}

		if inLoop {
			switch s.Kind {
			case ir.StepQuery, ir.StepCreate, ir.StepUpdate, ir.StepDelete:
				*diags = append(*diags, ir.Diagnostic{
					Code:     "FDN0214",
					Severity: ir.SeverityError,
					Message:  fmt.Sprintf("N+1 query hazard: %q on entity %q inside bounded 'for' loop in action %q is forbidden; use bulk set operations outside the loop", s.Kind, s.EntityName, actionName),
					Span:     s.Span,
				})
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
