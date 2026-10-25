package gohex

import (
	"fmt"
	"strings"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/arch"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
)

func formatIdempotentKeyGoExpr(rawKey string, params []ir.ParamDecl) string {
	if rawKey == "" {
		if len(params) > 0 {
			return params[0].Name
		}
		return `"default"`
	}
	if strings.Contains(rawKey, ".") {
		parts := strings.SplitN(rawKey, ".", 2)
		return fmt.Sprintf("%s.%s", parts[0], arch.ExportIdent(parts[1]))
	}
	return rawKey
}

func (lay goLayout) emitGoExpr(e ir.Expr, inDomainPkg bool) string {
	switch e.Kind {
	case ir.ExprString:
		if e.InferredType == "timestamp" {
			return fmt.Sprintf("func() time.Time { t, _ := time.Parse(time.RFC3339, %q); return t }()", e.Value)
		}
		return fmt.Sprintf("%q", e.Value)
	case ir.ExprInt, ir.ExprBool:
		return e.Value
	case ir.ExprFloat:
		prefix := lay.DomainPkg + "."
		if inDomainPkg {
			prefix = ""
		}
		return fmt.Sprintf("%sMustNewDecimal(%q)", prefix, e.Value)
	case ir.ExprEnumLit:
		prefix := lay.DomainPkg + "."
		if inDomainPkg {
			prefix = ""
		}
		if e.InferredType != "" {
			return fmt.Sprintf("%s%s%s", prefix, e.InferredType, arch.ExportIdent(e.Value))
		}
		return fmt.Sprintf("%q", e.Value)
	case ir.ExprList:
		sliceGoType := lay.goType(e.InferredType, inDomainPkg)
		var items []string
		for _, it := range e.Items {
			items = append(items, lay.emitGoExpr(it, inDomainPkg))
		}
		return fmt.Sprintf("%s{%s}", sliceGoType, strings.Join(items, ", "))
	case ir.ExprIdent:
		return e.Value
	case ir.ExprField:
		if e.Left != nil {
			return fmt.Sprintf("%s.%s", lay.emitGoExpr(*e.Left, inDomainPkg), arch.ExportIdent(e.Field))
		}
		return fmt.Sprintf("%s.%s", e.Target, arch.ExportIdent(e.Field))
	case ir.ExprCombinator:
		return lay.emitGoCombinatorExpr(e, inDomainPkg)
	case ir.ExprBinary:
		leftStr := lay.emitGoExpr(*e.Left, inDomainPkg)
		rightStr := lay.emitGoExpr(*e.Right, inDomainPkg)

		if e.Left.InferredType == "timestamp" && e.Right.InferredType == "timestamp" {
			switch e.Op {
			case "<":
				return fmt.Sprintf("%s.Before(%s)", leftStr, rightStr)
			case ">":
				return fmt.Sprintf("%s.After(%s)", leftStr, rightStr)
			case "<=":
				return fmt.Sprintf("!%s.After(%s)", leftStr, rightStr)
			case ">=":
				return fmt.Sprintf("!%s.Before(%s)", leftStr, rightStr)
			case "==":
				return fmt.Sprintf("%s.Equal(%s)", leftStr, rightStr)
			case "!=":
				return fmt.Sprintf("!%s.Equal(%s)", leftStr, rightStr)
			}
		}

		if e.Left.InferredType == "decimal" || e.Right.InferredType == "decimal" {
			decPrefix := lay.DomainPkg + "."
			if inDomainPkg {
				decPrefix = ""
			}
			if e.Left.InferredType == "int" {
				leftStr = fmt.Sprintf("%sNewDecimalFromInt(int64(%s))", decPrefix, leftStr)
			}
			if e.Right.InferredType == "int" {
				rightStr = fmt.Sprintf("%sNewDecimalFromInt(int64(%s))", decPrefix, rightStr)
			}
			switch e.Op {
			case "+":
				return fmt.Sprintf("%s.Add(%s)", leftStr, rightStr)
			case "-":
				return fmt.Sprintf("%s.Sub(%s)", leftStr, rightStr)
			case "*":
				return fmt.Sprintf("%s.Mul(%s)", leftStr, rightStr)
			case "/":
				return fmt.Sprintf("%s.Div(%s)", leftStr, rightStr)
			case "==":
				return fmt.Sprintf("%s.Equal(%s)", leftStr, rightStr)
			case "!=":
				return fmt.Sprintf("!%s.Equal(%s)", leftStr, rightStr)
			case ">":
				return fmt.Sprintf("%s.GreaterThan(%s)", leftStr, rightStr)
			case ">=":
				return fmt.Sprintf("%s.GreaterThanOrEqual(%s)", leftStr, rightStr)
			case "<":
				return fmt.Sprintf("%s.LessThan(%s)", leftStr, rightStr)
			case "<=":
				return fmt.Sprintf("%s.LessThanOrEqual(%s)", leftStr, rightStr)
			}
		}
		goOp := e.Op
		switch goOp {
		case "and":
			goOp = "&&"
		case "or":
			goOp = "||"
		}
		return fmt.Sprintf("(%s %s %s)", leftStr, goOp, rightStr)
	case ir.ExprConstruct:
		prefix := lay.DomainPkg + "."
		if inDomainPkg {
			prefix = ""
		}
		var parts []string
		for _, f := range e.Fields {
			parts = append(parts, fmt.Sprintf("%s: %s", arch.ExportIdent(f.Name), lay.emitGoExpr(f.Value, inDomainPkg)))
		}
		return fmt.Sprintf("%s%s{%s}", prefix, e.TypeName, strings.Join(parts, ", "))
	default:
		return e.Value
	}
}

func (lay goLayout) emitGoCombinatorExpr(e ir.Expr, inDomainPkg bool) string {
	recvStr := lay.emitGoExpr(*e.Left, inDomainPkg)
	retGoType := lay.goType(e.InferredType, inDomainPkg)
	param := e.Target
	if param == "" {
		param = "_item"
	}
	switch e.Op {
	case "filter":
		bodyStr := lay.emitGoExpr(*e.Right, inDomainPkg)
		return fmt.Sprintf("func() %s { var _out %s; for _, %s := range %s { if %s { _out = append(_out, %s) } }; return _out }()",
			retGoType, retGoType, param, recvStr, bodyStr, param)
	case "map":
		bodyStr := lay.emitGoExpr(*e.Right, inDomainPkg)
		return fmt.Sprintf("func() %s { var _out %s; for _, %s := range %s { _out = append(_out, %s) }; return _out }()",
			retGoType, retGoType, param, recvStr, bodyStr)
	case "sum":
		bodyStr := param
		if e.Right != nil {
			bodyStr = lay.emitGoExpr(*e.Right, inDomainPkg)
		}
		return fmt.Sprintf("func() %s { var _sum %s; for _, %s := range %s { _sum += %s }; return _sum }()",
			retGoType, retGoType, param, recvStr, bodyStr)
	case "count":
		if e.Right == nil {
			return fmt.Sprintf("len(%s)", recvStr)
		}
		bodyStr := lay.emitGoExpr(*e.Right, inDomainPkg)
		return fmt.Sprintf("func() int { var _cnt int; for _, %s := range %s { if %s { _cnt++ } }; return _cnt }()",
			param, recvStr, bodyStr)
	case "any":
		bodyStr := lay.emitGoExpr(*e.Right, inDomainPkg)
		return fmt.Sprintf("func() bool { for _, %s := range %s { if %s { return true } }; return false }()",
			param, recvStr, bodyStr)
	case "all":
		bodyStr := lay.emitGoExpr(*e.Right, inDomainPkg)
		return fmt.Sprintf("func() bool { for _, %s := range %s { if !(%s) { return false } }; return true }()",
			param, recvStr, bodyStr)
	}
	return recvStr
}
