package gohex

import (
	"fmt"
	"strings"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/arch"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
)

func (lay goLayout) formatGoStepRepoCall(kind arch.RepoMethodKind, preds []ir.WherePredicate, fallbackField string, fallbackVal *ir.Expr) (string, string) {
	mName := arch.StepRepoMethodName(kind, preds, fallbackField)
	var argExprs []string
	if len(preds) > 0 {
		for _, wp := range preds {
			argExprs = append(argExprs, lay.emitGoExpr(wp.Value, false))
		}
	} else if fallbackVal != nil {
		argExprs = append(argExprs, lay.emitGoExpr(*fallbackVal, false))
	}
	return mName, strings.Join(argExprs, ", ")
}

func (lay goLayout) formatRepoParams(m arch.RepositoryMethodIR) string {
	if len(m.WherePredicates) == 0 {
		if m.WhereField != "" {
			return fmt.Sprintf("%s %s", m.WhereField, lay.goType(m.WhereType, false))
		}
		return ""
	}
	fieldCounts := make(map[string]int)
	for _, wp := range m.WherePredicates {
		fieldCounts[wp.Field]++
	}
	var params []string
	for _, wp := range m.WherePredicates {
		pName := wp.Field
		if fieldCounts[wp.Field] > 1 {
			pName += "_" + strings.ToLower(arch.OpSuffix(wp.Op))
		}
		params = append(params, fmt.Sprintf("%s %s", pName, lay.goType(wp.Type, false)))
	}
	return strings.Join(params, ", ")
}

func (lay goLayout) formatInMemoryCheck(fieldAccess, op, paramVar, typeName string) string {
	if typeName == "timestamp" {
		switch op {
		case "==":
			return fmt.Sprintf("%s.Equal(%s)", fieldAccess, paramVar)
		case "!=":
			return fmt.Sprintf("!%s.Equal(%s)", fieldAccess, paramVar)
		case ">":
			return fmt.Sprintf("%s.After(%s)", fieldAccess, paramVar)
		case "<":
			return fmt.Sprintf("%s.Before(%s)", fieldAccess, paramVar)
		case ">=":
			return fmt.Sprintf("!%s.Before(%s)", fieldAccess, paramVar)
		case "<=":
			return fmt.Sprintf("!%s.After(%s)", fieldAccess, paramVar)
		}
	}
	if typeName == "decimal" {
		switch op {
		case "==":
			return fmt.Sprintf("%s.Equal(%s)", fieldAccess, paramVar)
		case "!=":
			return fmt.Sprintf("!%s.Equal(%s)", fieldAccess, paramVar)
		case ">":
			return fmt.Sprintf("%s.GreaterThan(%s)", fieldAccess, paramVar)
		case "<":
			return fmt.Sprintf("%s.LessThan(%s)", fieldAccess, paramVar)
		case ">=":
			return fmt.Sprintf("%s.GreaterThanOrEqual(%s)", fieldAccess, paramVar)
		case "<=":
			return fmt.Sprintf("%s.LessThanOrEqual(%s)", fieldAccess, paramVar)
		}
	}
	return fmt.Sprintf("%s %s %s", fieldAccess, op, paramVar)
}

func (lay goLayout) emitGoCursorHelpers(pkg string) string {
	return fmt.Sprintf(`package %s

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
)

func encodeCursor(val any, field string) string {
	b, _ := json.Marshal(map[string]any{field: val})
	return base64.RawURLEncoding.EncodeToString(b)
}

func decodeCursor(cursor string, field string) any {
	if cursor == "" {
		return nil
	}
	b, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		b, err = base64.URLEncoding.DecodeString(cursor)
		if err != nil {
			return nil
		}
	}
	var m map[string]any
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	if err := dec.Decode(&m); err != nil {
		return nil
	}
	val := m[field]
	if num, ok := val.(json.Number); ok {
		if i, err := num.Int64(); err == nil {
			return i
		}
		if f, err := num.Float64(); err == nil {
			return f
		}
	}
	return val
}
`, pkg)
}
