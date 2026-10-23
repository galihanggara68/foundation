package tsclean

import (
	"fmt"
	"strings"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/arch"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
)

func emitTSDomain(archIR *arch.ArchitectureIR) string {
	var sb strings.Builder
	sb.WriteString(`export interface Actor {
  id: string;
  role: string;
  is_verified: boolean;
  tenant_id?: string;
}

export class DomainError extends Error {
  readonly code: string;
  readonly kind: string;

  constructor(code: string, kind: string, message: string) {
    super(message);
    this.code = code;
    this.kind = kind;
  }
}

`)

	for _, errDecl := range archIR.DomainErrors {
		sb.WriteString(fmt.Sprintf("export const Err%s = new DomainError(%q, %q, %q);\n",
			errDecl.Name, errDecl.Name, errDecl.Kind, errDecl.Message))
	}
	if len(archIR.DomainErrors) > 0 {
		sb.WriteString("\n")
	}

	for _, en := range archIR.DomainEnums {
		var quoted []string
		for _, v := range en.Variants {
			quoted = append(quoted, fmt.Sprintf("%q", v))
		}
		sb.WriteString(fmt.Sprintf("export type %s = %s;\n\n", en.Name, strings.Join(quoted, " | ")))
	}

	for _, v := range archIR.DomainValues {
		emitTSInterface(&sb, v.Name, v.Fields)
	}

	for _, e := range archIR.DomainEntities {
		emitTSInterface(&sb, e.Name, e.Fields)
	}

	for _, ev := range archIR.DomainEvents {
		emitTSInterface(&sb, ev.Name, ev.Fields)
	}

	for _, pol := range archIR.DomainPolicies {
		var params []string
		for _, p := range pol.Params {
			params = append(params, fmt.Sprintf("%s: %s", p.Name, tsType(p.Type, true)))
		}
		sb.WriteString(fmt.Sprintf("export function %s(%s): boolean {\n  return %s;\n}\n\n",
			pol.Name, strings.Join(params, ", "), emitTSExpr(pol.Body, true)))
	}

	return sb.String()
}

func emitTSInterface(sb *strings.Builder, name string, fields []ir.FieldDecl) {
	sb.WriteString(fmt.Sprintf("export interface %s {\n", name))
	for _, f := range fields {
		opt := ""
		if f.Optional {
			opt = "?"
		}
		sb.WriteString(fmt.Sprintf("  %s%s: %s;\n", f.Name, opt, tsType(f.Type, true)))
	}
	sb.WriteString("}\n\n")
}

func emitTSFieldInits(sb *strings.Builder, fields []ir.FieldInitDecl, indent string) {
	for _, fi := range fields {
		sb.WriteString(fmt.Sprintf("%s%s: %s,\n", indent, fi.Name, emitTSExpr(fi.Value, false)))
	}
}

func emitTSExpr(e ir.Expr, inDomainFile bool) string {
	switch e.Kind {
	case ir.ExprString:
		return fmt.Sprintf("%q", e.Value)
	case ir.ExprInt, ir.ExprFloat, ir.ExprBool:
		return e.Value
	case ir.ExprEnumLit:
		return fmt.Sprintf("%q", e.Value)
	case ir.ExprList:
		var items []string
		for _, it := range e.Items {
			items = append(items, emitTSExpr(it, inDomainFile))
		}
		return fmt.Sprintf("[%s]", strings.Join(items, ", "))
	case ir.ExprIdent:
		return e.Value
	case ir.ExprField:
		if e.Left != nil {
			return fmt.Sprintf("%s.%s", emitTSExpr(*e.Left, inDomainFile), e.Field)
		}
		return fmt.Sprintf("%s.%s", e.Target, e.Field)
	case ir.ExprCombinator:
		return emitTSCombinatorExpr(e, inDomainFile)
	case ir.ExprBinary:
		op := e.Op
		switch op {
		case "==":
			op = "==="
		case "!=":
			op = "!=="
		}
		return fmt.Sprintf("(%s %s %s)", emitTSExpr(*e.Left, inDomainFile), op, emitTSExpr(*e.Right, inDomainFile))
	case ir.ExprConstruct:
		var parts []string
		for _, f := range e.Fields {
			parts = append(parts, fmt.Sprintf("%s: %s", f.Name, emitTSExpr(f.Value, inDomainFile)))
		}
		return fmt.Sprintf("{ %s }", strings.Join(parts, ", "))
	default:
		return e.Value
	}
}

func emitTSCombinatorExpr(e ir.Expr, inDomainFile bool) string {
	recvStr := emitTSExpr(*e.Left, inDomainFile)
	param := e.Target
	if param == "" {
		param = "_item"
	}
	switch e.Op {
	case "filter":
		return fmt.Sprintf("%s.filter((%s) => %s)", recvStr, param, emitTSExpr(*e.Right, inDomainFile))
	case "map":
		return fmt.Sprintf("%s.map((%s) => %s)", recvStr, param, emitTSExpr(*e.Right, inDomainFile))
	case "sum":
		body := param
		if e.Right != nil {
			body = emitTSExpr(*e.Right, inDomainFile)
		}
		return fmt.Sprintf("%s.reduce((_acc, %s) => _acc + (%s), 0)", recvStr, param, body)
	case "count":
		if e.Right == nil {
			return fmt.Sprintf("%s.length", recvStr)
		}
		return fmt.Sprintf("%s.filter((%s) => %s).length", recvStr, param, emitTSExpr(*e.Right, inDomainFile))
	case "any":
		return fmt.Sprintf("%s.some((%s) => %s)", recvStr, param, emitTSExpr(*e.Right, inDomainFile))
	case "all":
		return fmt.Sprintf("%s.every((%s) => %s)", recvStr, param, emitTSExpr(*e.Right, inDomainFile))
	}
	return recvStr
}
