package gohex

import (
	"fmt"
	"strings"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/arch"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
)

func emitGoSingleUseCase(module string, uc arch.UseCaseIR) string {
	var sb strings.Builder
	sb.WriteString("package usecases\n\nimport (\n\t\"context\"\n")
	sb.WriteString(fmt.Sprintf("\t\"%s/internal/domain\"\n", module))
	if len(uc.RepoDeps) > 0 || uc.EmitsEvents {
		sb.WriteString(fmt.Sprintf("\t\"%s/internal/ports\"\n", module))
	}
	sb.WriteString(")\n\n")

	sb.WriteString(fmt.Sprintf("type %s struct {\n", uc.Name))
	for _, dep := range uc.RepoDeps {
		sb.WriteString(fmt.Sprintf("\t%sRepo ports.%sRepository\n", dep, dep))
	}
	if uc.EmitsEvents {
		sb.WriteString("\tEvents ports.EventPublisher\n")
	}
	sb.WriteString("}\n\n")

	var ctorParams []string
	for _, dep := range uc.RepoDeps {
		ctorParams = append(ctorParams, fmt.Sprintf("%sRepo ports.%sRepository", strings.ToLower(dep), dep))
	}
	if uc.EmitsEvents {
		ctorParams = append(ctorParams, "events ports.EventPublisher")
	}
	sb.WriteString(fmt.Sprintf("func New%s(%s) *%s {\n\treturn &%s{\n", uc.Name, strings.Join(ctorParams, ", "), uc.Name, uc.Name))
	for _, dep := range uc.RepoDeps {
		sb.WriteString(fmt.Sprintf("\t\t%sRepo: %sRepo,\n", dep, strings.ToLower(dep)))
	}
	if uc.EmitsEvents {
		sb.WriteString("\t\tEvents: events,\n")
	}
	sb.WriteString("\t}\n}\n\n")

	var execParams []string
	execParams = append(execParams, "ctx context.Context")
	for _, p := range uc.Params {
		execParams = append(execParams, fmt.Sprintf("%s %s", p.Name, goType(p.Type, false)))
	}
	retType := goType(uc.ReturnType, false)
	sb.WriteString(fmt.Sprintf("func (uc *%s) Execute(%s) (%s, error) {\n", uc.Name, strings.Join(execParams, ", "), retType))
	sb.WriteString(fmt.Sprintf("\tvar zero %s\n\t_ = zero\n\t_ = domain.Actor{}\n", retType))
	if uc.UsesActor {
		sb.WriteString("\tactor := domain.ActorFromContext(ctx)\n\t_ = actor\n")
	}

	declaredVars := make(map[string]bool)
	for _, p := range uc.Params {
		declaredVars[p.Name] = true
	}
	stepCounter := 0
	emitGoSteps(&sb, uc.Steps, declaredVars, &stepCounter, "\t")

	sb.WriteString("}\n")
	return sb.String()
}

func emitGoSteps(sb *strings.Builder, steps []ir.StepDecl, declaredVars map[string]bool, stepCounter *int, indent string) {
	for _, s := range steps {
		*stepCounter++
		idx := *stepCounter
		switch s.Kind {
		case ir.StepRequire:
			var args []string
			for _, arg := range s.PolicyArgs {
				args = append(args, emitGoExpr(arg, false))
			}
			sb.WriteString(fmt.Sprintf("%sif !domain.%s(%s) {\n%s\treturn zero, domain.Err%s\n%s}\n",
				indent, s.PolicyName, strings.Join(args, ", "), indent, s.ElseError, indent))

		case ir.StepGuard:
			if s.Expr != nil {
				sb.WriteString(fmt.Sprintf("%sif !(%s) {\n%s\treturn zero, domain.Err%s\n%s}\n",
					indent, emitGoExpr(*s.Expr, false), indent, s.ElseError, indent))
			}

		case ir.StepLet:
			if s.Expr != nil {
				assignOp := ":="
				if declaredVars[s.VarName] {
					assignOp = "="
				} else {
					declaredVars[s.VarName] = true
				}
				sb.WriteString(fmt.Sprintf("%s%s %s %s\n", indent, s.VarName, assignOp, emitGoExpr(*s.Expr, false)))
			}

		case ir.StepCreate:
			declaredVars[s.VarName] = true
			sb.WriteString(fmt.Sprintf("%s%s, err := uc.%sRepo.Create(ctx, domain.%s{\n", indent, s.VarName, s.EntityName, s.EntityName))
			emitGoFieldInits(sb, s.Fields, indent+"\t")
			sb.WriteString(fmt.Sprintf("%s})\n", indent))
			sb.WriteString(fmt.Sprintf("%sif err != nil {\n%s\treturn zero, err\n%s}\n", indent, indent, indent))

		case ir.StepQuery:
			declaredVars[s.VarName] = true
			mName := "FindBy" + arch.ExportIdent(s.WhereField)
			wExpr := emitGoExpr(*s.WhereValue, false)
			sb.WriteString(fmt.Sprintf("%s%sPtr, err := uc.%sRepo.%s(ctx, %s)\n", indent, s.VarName, s.EntityName, mName, wExpr))
			sb.WriteString(fmt.Sprintf("%sif err != nil {\n%s\treturn zero, err\n%s}\n", indent, indent, indent))
			if s.ElseError != "" {
				sb.WriteString(fmt.Sprintf("%sif %sPtr == nil {\n%s\treturn zero, domain.Err%s\n%s}\n", indent, s.VarName, indent, s.ElseError, indent))
			}
			sb.WriteString(fmt.Sprintf("%s%s := *%sPtr\n", indent, s.VarName, s.VarName))

		case ir.StepUpdate:
			mName := "FindBy" + arch.ExportIdent(s.WhereField)
			wExpr := emitGoExpr(*s.WhereValue, false)
			updVar := fmt.Sprintf("updTarget%d", idx)
			sb.WriteString(fmt.Sprintf("%s%s, err := uc.%sRepo.%s(ctx, %s)\n", indent, updVar, s.EntityName, mName, wExpr))
			sb.WriteString(fmt.Sprintf("%sif err != nil {\n%s\treturn zero, err\n%s}\n", indent, indent, indent))
			sb.WriteString(fmt.Sprintf("%sif %s != nil {\n", indent, updVar))
			for _, fi := range s.Fields {
				sb.WriteString(fmt.Sprintf("%s\t%s.%s = %s\n", indent, updVar, arch.ExportIdent(fi.Name), emitGoExpr(fi.Value, false)))
			}
			sb.WriteString(fmt.Sprintf("%s\tif err := uc.%sRepo.Update(ctx, *%s); err != nil {\n%s\t\treturn zero, err\n%s\t}\n",
				indent, s.EntityName, updVar, indent, indent))
			sb.WriteString(fmt.Sprintf("%s}\n", indent))

		case ir.StepDelete:
			mName := "DeleteBy" + arch.ExportIdent(s.WhereField)
			wExpr := emitGoExpr(*s.WhereValue, false)
			sb.WriteString(fmt.Sprintf("%sif err := uc.%sRepo.%s(ctx, %s); err != nil {\n%s\treturn zero, err\n%s}\n",
				indent, s.EntityName, mName, wExpr, indent, indent))

		case ir.StepIf:
			if s.Expr != nil {
				sb.WriteString(fmt.Sprintf("%sif %s {\n", indent, emitGoExpr(*s.Expr, false)))
				emitGoSteps(sb, s.ThenSteps, cloneBoolMap(declaredVars), stepCounter, indent+"\t")
				if len(s.ElseSteps) > 0 {
					sb.WriteString(fmt.Sprintf("%s} else {\n", indent))
					emitGoSteps(sb, s.ElseSteps, cloneBoolMap(declaredVars), stepCounter, indent+"\t")
				}
				sb.WriteString(fmt.Sprintf("%s}\n", indent))
			}

		case ir.StepMatch:
			if s.Expr != nil {
				enumType := s.Expr.InferredType
				sb.WriteString(fmt.Sprintf("%sswitch %s {\n", indent, emitGoExpr(*s.Expr, false)))
				for _, arm := range s.MatchArms {
					sb.WriteString(fmt.Sprintf("%scase domain.%s%s:\n", indent, enumType, arch.ExportIdent(arm.Variant)))
					emitGoSteps(sb, arm.Steps, cloneBoolMap(declaredVars), stepCounter, indent+"\t")
				}
				sb.WriteString(fmt.Sprintf("%s}\n", indent))
			}

		case ir.StepFor:
			if s.Expr != nil {
				sb.WriteString(fmt.Sprintf("%sfor _, %s := range %s {\n%s\t_ = %s\n",
					indent, s.IterVar, emitGoExpr(*s.Expr, false), indent, s.IterVar))
				loopVars := cloneBoolMap(declaredVars)
				loopVars[s.IterVar] = true
				emitGoSteps(sb, s.ThenSteps, loopVars, stepCounter, indent+"\t")
				sb.WriteString(fmt.Sprintf("%s}\n", indent))
			}

		case ir.StepEmit:
			sb.WriteString(fmt.Sprintf("%sif uc.Events != nil {\n%s\t_ = uc.Events.Publish(ctx, %q, domain.%s{\n", indent, indent, s.EventName, s.EventName))
			emitGoFieldInits(sb, s.Fields, indent+"\t\t")
			sb.WriteString(fmt.Sprintf("%s\t})\n%s}\n", indent, indent))

		case ir.StepReturn:
			if s.Expr != nil {
				sb.WriteString(fmt.Sprintf("%sreturn %s, nil\n", indent, emitGoExpr(*s.Expr, false)))
			}
		}
	}
}

func cloneBoolMap(in map[string]bool) map[string]bool {
	out := make(map[string]bool, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func emitGoExpr(e ir.Expr, inDomainPkg bool) string {
	switch e.Kind {
	case ir.ExprString:
		return fmt.Sprintf("%q", e.Value)
	case ir.ExprInt, ir.ExprFloat, ir.ExprBool:
		return e.Value
	case ir.ExprEnumLit:
		prefix := "domain."
		if inDomainPkg {
			prefix = ""
		}
		if e.InferredType != "" {
			return fmt.Sprintf("%s%s%s", prefix, e.InferredType, arch.ExportIdent(e.Value))
		}
		return fmt.Sprintf("%q", e.Value)
	case ir.ExprList:
		sliceGoType := goType(e.InferredType, inDomainPkg)
		var items []string
		for _, it := range e.Items {
			items = append(items, emitGoExpr(it, inDomainPkg))
		}
		return fmt.Sprintf("%s{%s}", sliceGoType, strings.Join(items, ", "))
	case ir.ExprIdent:
		return e.Value
	case ir.ExprField:
		if e.Left != nil {
			return fmt.Sprintf("%s.%s", emitGoExpr(*e.Left, inDomainPkg), arch.ExportIdent(e.Field))
		}
		return fmt.Sprintf("%s.%s", e.Target, arch.ExportIdent(e.Field))
	case ir.ExprCombinator:
		return emitGoCombinatorExpr(e, inDomainPkg)
	case ir.ExprBinary:
		leftStr := emitGoExpr(*e.Left, inDomainPkg)
		rightStr := emitGoExpr(*e.Right, inDomainPkg)
		if (e.Left.InferredType == "decimal" && e.Right.InferredType == "int") ||
			(e.Left.InferredType == "int" && e.Right.InferredType == "decimal") {
			if e.Left.InferredType == "int" {
				leftStr = fmt.Sprintf("float64(%s)", leftStr)
			}
			if e.Right.InferredType == "int" {
				rightStr = fmt.Sprintf("float64(%s)", rightStr)
			}
		}
		return fmt.Sprintf("(%s %s %s)", leftStr, e.Op, rightStr)
	case ir.ExprConstruct:
		prefix := "domain."
		if inDomainPkg {
			prefix = ""
		}
		var parts []string
		for _, f := range e.Fields {
			parts = append(parts, fmt.Sprintf("%s: %s", arch.ExportIdent(f.Name), emitGoExpr(f.Value, inDomainPkg)))
		}
		return fmt.Sprintf("%s%s{%s}", prefix, e.TypeName, strings.Join(parts, ", "))
	default:
		return e.Value
	}
}

func emitGoCombinatorExpr(e ir.Expr, inDomainPkg bool) string {
	recvStr := emitGoExpr(*e.Left, inDomainPkg)
	retGoType := goType(e.InferredType, inDomainPkg)
	param := e.Target
	if param == "" {
		param = "_item"
	}
	switch e.Op {
	case "filter":
		bodyStr := emitGoExpr(*e.Right, inDomainPkg)
		return fmt.Sprintf("func() %s { var _out %s; for _, %s := range %s { if %s { _out = append(_out, %s) } }; return _out }()",
			retGoType, retGoType, param, recvStr, bodyStr, param)
	case "map":
		bodyStr := emitGoExpr(*e.Right, inDomainPkg)
		return fmt.Sprintf("func() %s { var _out %s; for _, %s := range %s { _out = append(_out, %s) }; return _out }()",
			retGoType, retGoType, param, recvStr, bodyStr)
	case "sum":
		bodyStr := param
		if e.Right != nil {
			bodyStr = emitGoExpr(*e.Right, inDomainPkg)
		}
		return fmt.Sprintf("func() %s { var _sum %s; for _, %s := range %s { _sum += %s }; return _sum }()",
			retGoType, retGoType, param, recvStr, bodyStr)
	case "count":
		if e.Right == nil {
			return fmt.Sprintf("len(%s)", recvStr)
		}
		bodyStr := emitGoExpr(*e.Right, inDomainPkg)
		return fmt.Sprintf("func() int { var _cnt int; for _, %s := range %s { if %s { _cnt++ } }; return _cnt }()",
			param, recvStr, bodyStr)
	case "any":
		bodyStr := emitGoExpr(*e.Right, inDomainPkg)
		return fmt.Sprintf("func() bool { for _, %s := range %s { if %s { return true } }; return false }()",
			param, recvStr, bodyStr)
	case "all":
		bodyStr := emitGoExpr(*e.Right, inDomainPkg)
		return fmt.Sprintf("func() bool { for _, %s := range %s { if !(%s) { return false } }; return true }()",
			param, recvStr, bodyStr)
	}
	return recvStr
}
