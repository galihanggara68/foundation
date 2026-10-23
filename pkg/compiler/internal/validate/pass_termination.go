package validate

import (
	"fmt"

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
					diags = append(diags, ir.Diagnostic{
						Code:     "FDN0210",
						Severity: ir.SeverityError,
						Message:  "unbounded 'while' iteration is forbidden in Action Pipelines; use bounded 'for item in collection'",
						Span:     s.Span,
					})
				}
			},
			func(e ir.Expr) {
				if e.Kind == ir.ExprCall {
					if _, isAction := sym.actions[e.Value]; isAction {
						diags = append(diags, ir.Diagnostic{
							Code:     "FDN0211",
							Severity: ir.SeverityError,
							Message:  fmt.Sprintf("recursive or direct action invocation %q is forbidden in Action Pipelines", e.Value),
							Span:     e.Span,
						})
					}
				}
			},
		)
	}

	return diags
}
