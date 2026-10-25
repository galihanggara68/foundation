package frontend

import "github.com/galihanggara68/foundation/pkg/compiler/internal/ir"

func isComparisonOp(op string) bool {
	switch op {
	case "==", "!=", ">", "<", ">=", "<=":
		return true
	default:
		return false
	}
}

func invertComparisonOp(op string) string {
	switch op {
	case ">":
		return "<"
	case "<":
		return ">"
	case ">=":
		return "<="
	case "<=":
		return ">="
	default:
		return op
	}
}

func extractWherePredicates(e ir.Expr) []ir.WherePredicate {
	if e.Kind == ir.ExprBinary && (e.Op == "and" || e.Op == "&&") {
		var preds []ir.WherePredicate
		if e.Left != nil {
			preds = append(preds, extractWherePredicates(*e.Left)...)
		}
		if e.Right != nil {
			preds = append(preds, extractWherePredicates(*e.Right)...)
		}
		return preds
	}

	if e.Kind == ir.ExprBinary && isComparisonOp(e.Op) {
		if e.Left != nil && e.Left.Kind == ir.ExprIdent && e.Right != nil {
			return []ir.WherePredicate{
				{
					Field: e.Left.Value,
					Op:    e.Op,
					Value: *e.Right,
					Span:  e.Span,
				},
			}
		}
		if e.Right != nil && e.Right.Kind == ir.ExprIdent && e.Left != nil {
			return []ir.WherePredicate{
				{
					Field: e.Right.Value,
					Op:    invertComparisonOp(e.Op),
					Value: *e.Left,
					Span:  e.Span,
				},
			}
		}
		if e.Right != nil {
			return []ir.WherePredicate{
				{
					Field: "",
					Op:    e.Op,
					Value: *e.Right,
					Span:  e.Span,
				},
			}
		}
	}

	return nil
}

func populateStepWhere(step *ir.StepDecl, whereExpr ir.Expr) {
	step.WhereExpr = &whereExpr
	preds := extractWherePredicates(whereExpr)
	step.WherePredicates = preds
	if len(preds) > 0 {
		step.WhereField = preds[0].Field
		step.WhereValue = &preds[0].Value
	}
}

func populateExpectWhere(expect *ir.ExpectDecl, whereExpr ir.Expr) {
	expect.WhereExpr = &whereExpr
	preds := extractWherePredicates(whereExpr)
	expect.WherePredicates = preds
	if len(preds) > 0 {
		expect.WhereField = preds[0].Field
		expect.WhereValue = &preds[0].Value
	}
}
