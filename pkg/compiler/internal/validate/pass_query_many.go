package validate

import (
	"fmt"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/platform"
)

// QueryManyUniqueOrderPass implements validate.Pass to enforce that query many
// specifies an order_by field with a unique trait (@pk or @unique) (FDN0215).
type QueryManyUniqueOrderPass struct{}

func (p QueryManyUniqueOrderPass) Validate(sem *ir.SemanticIR, sym *SymbolTable) []ir.Diagnostic {
	return ValidateQueryManyUniqueOrder(sem, sym)
}

// ValidateQueryManyUniqueOrder checks that every query many step orders by a @pk or @unique field.
func ValidateQueryManyUniqueOrder(doc *ir.SemanticIR, sym *SymbolTable) []ir.Diagnostic {
	var diags []ir.Diagnostic
	var entityMap map[string]ir.EntityDecl
	if sym != nil && sym.entities != nil {
		entityMap = sym.entities
	} else {
		entityMap = make(map[string]ir.EntityDecl)
		for _, ent := range doc.Entities {
			entityMap[ent.Name] = ent
		}
	}

	for _, act := range doc.Actions {
		ir.WalkStepsAndExprs(act.Steps, func(s ir.StepDecl) {
			if s.Kind != ir.StepQueryMany {
				return
			}
			if s.OrderByField == "" {
				diags = append(diags, ir.Diagnostic{
					Code:     "FDN0215",
					Severity: ir.SeverityError,
					Message:  fmt.Sprintf("query many on entity %q requires an 'order_by' clause on a unique field (@pk or @unique)", s.EntityName),
					Span:     s.Span,
				})
				return
			}

			ent, ok := entityMap[s.EntityName]
			if !ok {
				return
			}

			var matchedField *ir.FieldDecl
			for i := range ent.Fields {
				if ent.Fields[i].Name == s.OrderByField {
					matchedField = &ent.Fields[i]
					break
				}
			}

			if matchedField == nil {
				diags = append(diags, ir.Diagnostic{
					Code:     "FDN0215",
					Severity: ir.SeverityError,
					Message:  fmt.Sprintf("order_by field %q does not exist on entity %q", s.OrderByField, s.EntityName),
					Span:     s.Span,
				})
				return
			}

			if !matchedField.HasTrait("pk") && !matchedField.HasTrait("unique") {
				diags = append(diags, ir.Diagnostic{
					Code:     "FDN0215",
					Severity: ir.SeverityError,
					Message:  fmt.Sprintf("order_by field %q on entity %q must have @pk or @unique trait to ensure deterministic keyset ordering", s.OrderByField, s.EntityName),
					Span:     s.Span,
				})
			}
		}, nil)
	}

	return diags
}

// QueryManyLimitCapPass implements validate.Pass to enforce that query many
// specifies a limit that is capped at 100 (FDN0216).
type QueryManyLimitCapPass struct{}

func (p QueryManyLimitCapPass) Validate(sem *ir.SemanticIR, sym *SymbolTable) []ir.Diagnostic {
	return ValidateQueryManyLimitCap(sem)
}

// ValidateQueryManyLimitCap checks that every query many step specifies a limit <= 100 and > 0.
func ValidateQueryManyLimitCap(doc *ir.SemanticIR) []ir.Diagnostic {
	var diags []ir.Diagnostic

	for _, act := range doc.Actions {
		ir.WalkStepsAndExprs(act.Steps, func(s ir.StepDecl) {
			if s.Kind != ir.StepQueryMany {
				return
			}

			if s.Limit == 0 && s.LimitExpr == nil {
				diags = append(diags, ir.Diagnostic{
					Code:     "FDN0216",
					Severity: ir.SeverityError,
					Message:  fmt.Sprintf("query many on entity %q requires a 'limit' clause", s.EntityName),
					Span:     s.Span,
				})
				return
			}

			if s.LimitExpr != nil && s.LimitExpr.Kind != ir.ExprInt {
				diags = append(diags, ir.Diagnostic{
					Code:     "FDN0216",
					Severity: ir.SeverityError,
					Message:  fmt.Sprintf("query many limit on entity %q must be a static integer literal <= %d", s.EntityName, platform.DefaultMaxPageLimit),
					Span:     s.LimitExpr.Span,
				})
				return
			}

			if s.Limit > platform.DefaultMaxPageLimit {
				diags = append(diags, ir.Diagnostic{
					Code:     "FDN0216",
					Severity: ir.SeverityError,
					Message:  fmt.Sprintf("query many limit %d exceeds maximum allowed cap of %d", s.Limit, platform.DefaultMaxPageLimit),
					Span:     s.Span,
				})
			} else if s.Limit <= 0 {
				diags = append(diags, ir.Diagnostic{
					Code:     "FDN0216",
					Severity: ir.SeverityError,
					Message:  fmt.Sprintf("query many limit must be greater than 0, got %d", s.Limit),
					Span:     s.Span,
				})
			}
		}, nil)
	}

	return diags
}
