package tsclean

import (
	"fmt"
	"strings"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/arch"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
)

func formatTSRepoParams(m arch.RepositoryMethodIR) string {
	if len(m.WherePredicates) == 0 {
		if m.WhereField != "" {
			return fmt.Sprintf("%s: %s", toCamelCase(m.WhereField), tsType(m.WhereType, false))
		}
		return ""
	}
	fieldCounts := make(map[string]int)
	for _, wp := range m.WherePredicates {
		fieldCounts[wp.Field]++
	}
	var params []string
	for _, wp := range m.WherePredicates {
		pName := toCamelCase(wp.Field)
		if fieldCounts[wp.Field] > 1 {
			pName += arch.OpSuffix(wp.Op)
		}
		params = append(params, fmt.Sprintf("%s: %s", pName, tsType(wp.Type, false)))
	}
	return strings.Join(params, ", ")
}

func formatTSInMemoryCheck(fieldAccess, op, paramVar, typeName string) string {
	if typeName == "timestamp" {
		switch op {
		case "==":
			return fmt.Sprintf("(%s.getTime() === %s.getTime())", fieldAccess, paramVar)
		case "!=":
			return fmt.Sprintf("(%s.getTime() !== %s.getTime())", fieldAccess, paramVar)
		case ">":
			return fmt.Sprintf("(%s.getTime() > %s.getTime())", fieldAccess, paramVar)
		case "<":
			return fmt.Sprintf("(%s.getTime() < %s.getTime())", fieldAccess, paramVar)
		case ">=":
			return fmt.Sprintf("(%s.getTime() >= %s.getTime())", fieldAccess, paramVar)
		case "<=":
			return fmt.Sprintf("(%s.getTime() <= %s.getTime())", fieldAccess, paramVar)
		}
	}
	if typeName == "decimal" {
		switch op {
		case "==":
			return fmt.Sprintf("%s.eq(%s)", fieldAccess, paramVar)
		case "!=":
			return fmt.Sprintf("!%s.eq(%s)", fieldAccess, paramVar)
		case ">":
			return fmt.Sprintf("%s.gt(%s)", fieldAccess, paramVar)
		case "<":
			return fmt.Sprintf("%s.lt(%s)", fieldAccess, paramVar)
		case ">=":
			return fmt.Sprintf("%s.gte(%s)", fieldAccess, paramVar)
		case "<=":
			return fmt.Sprintf("%s.lte(%s)", fieldAccess, paramVar)
		}
	}
	tsOp := op
	if tsOp == "==" {
		tsOp = "==="
	} else if tsOp == "!=" {
		tsOp = "!=="
	}
	return fmt.Sprintf("%s %s %s", fieldAccess, tsOp, paramVar)
}

func formatTSStepRepoCall(kind arch.RepoMethodKind, s ir.StepDecl) (string, string) {
	mName := toCamelCase(arch.StepRepoMethodName(kind, s.WherePredicates, s.WhereField))
	var argExprs []string
	if len(s.WherePredicates) > 0 {
		for _, wp := range s.WherePredicates {
			argExprs = append(argExprs, emitTSExpr(wp.Value, false))
		}
	} else if s.WhereValue != nil {
		argExprs = append(argExprs, emitTSExpr(*s.WhereValue, false))
	}
	return mName, strings.Join(argExprs, ", ")
}
