package arch

import (
	"strings"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
)

// SQLComparisonOp converts Foundation binary comparison operators to SQL equivalents.
func SQLComparisonOp(op string) string {
	switch op {
	case "==":
		return "="
	case "!=":
		return "!="
	default:
		return op
	}
}

// OpSuffix returns a PascalCase suffix representing the operator for disambiguation.
func OpSuffix(op string) string {
	switch op {
	case ">":
		return "Gt"
	case ">=":
		return "Gte"
	case "<":
		return "Lt"
	case "<=":
		return "Lte"
	case "!=":
		return "Neq"
	default:
		return ""
	}
}

// StepRepoMethodName returns the Go method name for a repository operation on given predicates.
func StepRepoMethodName(kind RepoMethodKind, preds []ir.WherePredicate, fallbackField string) string {
	prefix := "FindBy"
	if kind == RepoMethodDelete {
		prefix = "DeleteBy"
	} else if kind == RepoMethodFindMany {
		prefix = "FindManyBy"
	}
	if len(preds) == 0 {
		if kind == RepoMethodFindMany {
			return "FindMany"
		}
		if fallbackField != "" {
			return prefix + ExportIdent(fallbackField)
		}
		return prefix
	}
	fieldCounts := make(map[string]int)
	for _, p := range preds {
		fieldCounts[p.Field]++
	}
	var parts []string
	for _, p := range preds {
		part := ExportIdent(p.Field)
		if fieldCounts[p.Field] > 1 {
			part += OpSuffix(p.Op)
		}
		parts = append(parts, part)
	}
	return prefix + strings.Join(parts, "And")
}

// BuildRepoMethodIR constructs a normalized RepositoryMethodIR from where predicates.
func BuildRepoMethodIR(kind RepoMethodKind, entityName string, preds []ir.WherePredicate, fallbackField, fallbackType string, entityFields map[string]string) RepositoryMethodIR {
	var repoPreds []RepoWherePredicateIR
	for _, p := range preds {
		fType := ""
		if entityFields != nil {
			fType = entityFields[p.Field]
		}
		repoPreds = append(repoPreds, RepoWherePredicateIR{
			Field: p.Field,
			Op:    p.Op,
			Type:  fType,
		})
	}

	whereField := fallbackField
	whereType := fallbackType
	if len(repoPreds) > 0 {
		whereField = repoPreds[0].Field
		whereType = repoPreds[0].Type
	} else if fallbackField != "" {
		repoPreds = []RepoWherePredicateIR{
			{
				Field: fallbackField,
				Op:    "==",
				Type:  fallbackType,
			},
		}
	}

	methodName := StepRepoMethodName(kind, preds, fallbackField)

	return RepositoryMethodIR{
		Kind:            kind,
		Name:            methodName,
		EntityName:      entityName,
		WhereField:      whereField,
		WhereType:       whereType,
		WherePredicates: repoPreds,
	}
}

// BuildFindManyRepoMethodIR constructs a normalized RepositoryMethodIR for a keyset-paginated read.
func BuildFindManyRepoMethodIR(entityName string, preds []ir.WherePredicate, orderByField string, orderByDesc bool, limit int, entityFields map[string]string) RepositoryMethodIR {
	var repoPreds []RepoWherePredicateIR
	for _, p := range preds {
		fType := ""
		if entityFields != nil {
			fType = entityFields[p.Field]
		}
		repoPreds = append(repoPreds, RepoWherePredicateIR{
			Field: p.Field,
			Op:    p.Op,
			Type:  fType,
		})
	}

	orderType := ""
	if entityFields != nil {
		orderType = entityFields[orderByField]
	}

	methodName := StepRepoMethodName(RepoMethodFindMany, preds, "")

	return RepositoryMethodIR{
		Kind:            RepoMethodFindMany,
		Name:            methodName,
		EntityName:      entityName,
		WherePredicates: repoPreds,
		OrderByField:    orderByField,
		OrderByDesc:     orderByDesc,
		OrderByType:     orderType,
		Limit:           limit,
	}
}
