package validate

import (
	"fmt"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
)

// SargableWherePass enforces that where clause expressions are conjunctions of single-field
// comparisons, rejecting disjunctions ('or') with FDN0217 to guarantee index-friendly database queries.
type SargableWherePass struct{}

func (SargableWherePass) Validate(sem *ir.SemanticIR, sym *SymbolTable) []ir.Diagnostic {
	var diags []ir.Diagnostic
	for _, a := range sem.Actions {
		ir.WalkStepsAndExprs(a.Steps, func(s ir.StepDecl) {
			if s.WhereExpr != nil {
				ValidateSargableWhereStructure(*s.WhereExpr, &diags)
			}
		}, nil)
	}
	for _, sc := range sem.Scenarios {
		for _, ex := range sc.Expects {
			if ex.WhereExpr != nil {
				ValidateSargableWhereStructure(*ex.WhereExpr, &diags)
			}
		}
	}
	return diags
}

func isComparisonOp(op string) bool {
	switch op {
	case "==", "!=", ">", "<", ">=", "<=":
		return true
	default:
		return false
	}
}

// ValidateSargableWhereStructure verifies that whereExpr is strictly a conjunction of comparisons.
func ValidateSargableWhereStructure(e ir.Expr, diags *[]ir.Diagnostic) bool {
	if e.Kind == ir.ExprBinary && (e.Op == "or" || e.Op == "||") {
		*diags = append(*diags, ir.Diagnostic{
			Code:     "FDN0217",
			Severity: ir.SeverityError,
			Message:  "disjunction ('or') is not supported in where predicates; only conjunctions ('and') are allowed to enforce sargable query plans",
			Span:     e.Span,
		})
		return false
	}
	if e.Kind == ir.ExprBinary && (e.Op == "and" || e.Op == "&&") {
		lOk := e.Left == nil || ValidateSargableWhereStructure(*e.Left, diags)
		rOk := e.Right == nil || ValidateSargableWhereStructure(*e.Right, diags)
		return lOk && rOk
	}
	if e.Kind == ir.ExprBinary && isComparisonOp(e.Op) {
		return true
	}
	*diags = append(*diags, ir.Diagnostic{
		Code:     "FDN0205",
		Severity: ir.SeverityError,
		Message:  "invalid where predicate: only conjunctions of single-field comparisons are supported",
		Span:     e.Span,
	})
	return false
}

// ValidateSargableWhere checks for disjunctions ('or') in where clauses.
func ValidateSargableWhere(whereExpr *ir.Expr, diags *[]ir.Diagnostic) bool {
	if whereExpr == nil {
		return true
	}
	return ValidateSargableWhereStructure(*whereExpr, diags)
}

func validateStepWhere(opName string, s *ir.StepDecl, scope map[string]string, sym *SymbolTable, diags *[]ir.Diagnostic) (ir.EntityDecl, bool) {
	if s.WhereExpr != nil {
		if !ValidateSargableWhere(s.WhereExpr, diags) {
			ent, _ := sym.entities[s.EntityName]
			return ent, false
		}
	}
	if len(s.WherePredicates) > 0 {
		return validateCompoundWhere(opName, s.EntityName, s.WherePredicates, s.Span, scope, sym, diags)
	}
	return validateEntityWhereClause(opName, s.EntityName, s.WhereField, s.WhereValue, s.Span, scope, sym, diags)
}

func validateExpectWhere(opName string, ex *ir.ExpectDecl, scope map[string]string, sym *SymbolTable, diags *[]ir.Diagnostic) (ir.EntityDecl, bool) {
	if ex.WhereExpr != nil {
		if !ValidateSargableWhere(ex.WhereExpr, diags) {
			ent, _ := sym.entities[ex.StoredEntity]
			return ent, false
		}
	}
	if len(ex.WherePredicates) > 0 {
		return validateCompoundWhere(opName, ex.StoredEntity, ex.WherePredicates, ex.Span, scope, sym, diags)
	}
	return validateEntityWhereClause(opName, ex.StoredEntity, ex.WhereField, ex.WhereValue, ex.Span, scope, sym, diags)
}

func validateCompoundWhere(opName, entityName string, preds []ir.WherePredicate, span ir.Span, scope map[string]string, sym *SymbolTable, diags *[]ir.Diagnostic) (ir.EntityDecl, bool) {
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
	allOk := true
	for i := range preds {
		p := &preds[i]
		if p.Field == "" {
			*diags = append(*diags, ir.Diagnostic{
				Code:     "FDN0205",
				Severity: ir.SeverityError,
				Message:  fmt.Sprintf("invalid where predicate in %s: must compare an entity field", opName),
				Span:     p.Span,
			})
			allOk = false
			continue
		}
		wf, fOk := fields[p.Field]
		if !fOk {
			*diags = append(*diags, ir.Diagnostic{
				Code:     "FDN0204",
				Severity: ir.SeverityError,
				Message:  fmt.Sprintf("entity %q has no field %q", ent.Name, p.Field),
				Span:     p.Span,
			})
			allOk = false
			continue
		}
		sym.resolveEnumLiteral(&p.Value, wf.Type)
		valType, ok := inferExprType(&p.Value, scope, sym, diags)
		if !ok {
			allOk = false
			continue
		}

		if p.Op == "==" || p.Op == "!=" {
			if !isAssignableType(wf.Type, valType) && !isAssignableType(valType, wf.Type) && !(isNumericType(wf.Type) && isNumericType(valType)) {
				*diags = append(*diags, ir.Diagnostic{
					Code:     "FDN0205",
					Severity: ir.SeverityError,
					Message:  fmt.Sprintf("type mismatch in %s where clause: field %q is %s, got %s", opName, p.Field, wf.Type, valType),
					Span:     p.Value.Span,
				})
				allOk = false
			}
		} else {
			bothNumeric := isNumericType(wf.Type) && isNumericType(valType)
			bothTimestamps := wf.Type == "timestamp" && valType == "timestamp"
			if !bothNumeric && !bothTimestamps {
				*diags = append(*diags, ir.Diagnostic{
					Code:     "FDN0205",
					Severity: ir.SeverityError,
					Message:  fmt.Sprintf("type mismatch in %s where clause: comparison operator %s requires numeric or timestamp operands, got %s and %s", opName, p.Op, wf.Type, valType),
					Span:     p.Value.Span,
				})
				allOk = false
			}
		}
	}
	return ent, allOk
}
