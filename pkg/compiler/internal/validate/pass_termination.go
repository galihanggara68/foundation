package validate

import (
	"github.com/galihanggara68/foundation/pkg/compiler/internal/diagnostics"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
)

// TerminationPass enforces Action Pipeline termination invariants (no unbounded while loops or recursive action calls).
type TerminationPass struct{}

func (TerminationPass) Validate(sem *ir.SemanticIR, sym *SymbolTable) []ir.Diagnostic {
	var diags []ir.Diagnostic

	for _, a := range sem.Actions {
		ir.WalkStepsAndExprs(a.Steps,
			func(s ir.StepDecl) {
				if s.Kind == ir.StepWhile {
					diags = append(diags, diagnostics.NewWithDetail(
						"FDN0210",
						s.Span,
						"unbounded 'while' iteration is forbidden in Action Pipelines; use bounded 'for item in collection'",
					))
				}
			},
			func(e ir.Expr) {
				if e.Kind == ir.ExprCall {
					if _, isAction := sym.actions[e.Value]; isAction {
						diags = append(diags, diagnostics.New("FDN0211", e.Span, e.Value))
					}
				}
			},
		)
	}

	return diags
}
