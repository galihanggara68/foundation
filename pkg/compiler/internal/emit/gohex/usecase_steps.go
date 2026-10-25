package gohex

import (
	"fmt"
	"strings"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/arch"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/emit"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
)

type stepEmitCtx struct {
	arch        *arch.ArchitectureIR
	useCase     arch.UseCaseIR
	useOutbox   bool
	stepCounter *int
}

func (lay goLayout) emitGoSteps(sb *strings.Builder, steps []ir.StepDecl, declaredVars map[string]bool, indent string, ctx stepEmitCtx) {
	for _, s := range steps {
		*ctx.stepCounter++
		idx := *ctx.stepCounter
		switch s.Kind {
		case ir.StepRequire:
			var args []string
			for _, arg := range s.PolicyArgs {
				args = append(args, lay.emitGoExpr(arg, false))
			}
			sb.WriteString(fmt.Sprintf("%sif !%s.%s(%s) {\n%s\treturn zero, %s.Err%s\n%s}\n",
				indent, lay.DomainPkg, s.PolicyName, strings.Join(args, ", "), indent, lay.DomainPkg, s.ElseError, indent))

		case ir.StepGuard:
			if s.Expr != nil {
				sb.WriteString(fmt.Sprintf("%sif !(%s) {\n%s\treturn zero, %s.Err%s\n%s}\n",
					indent, lay.emitGoExpr(*s.Expr, false), indent, lay.DomainPkg, s.ElseError, indent))
			}

		case ir.StepLet:
			if s.Expr != nil {
				assignOp := ":="
				if declaredVars[s.VarName] {
					assignOp = "="
				} else {
					declaredVars[s.VarName] = true
				}
				sb.WriteString(fmt.Sprintf("%s%s %s %s\n%s_ = %s\n", indent, s.VarName, assignOp, lay.emitGoExpr(*s.Expr, false), indent, s.VarName))
			}

		case ir.StepCreate:
			declaredVars[s.VarName] = true
			sb.WriteString(fmt.Sprintf("%s%s, err := uc.%sRepo.Create(ctx, %s.%s{\n", indent, s.VarName, s.EntityName, lay.DomainPkg, s.EntityName))
			lay.emitGoFieldInits(sb, s.Fields, indent+"\t")
			sb.WriteString(fmt.Sprintf("%s})\n", indent))
			sb.WriteString(fmt.Sprintf("%sif err != nil {\n%s\treturn zero, err\n%s}\n%s_ = %s\n", indent, indent, indent, indent, s.VarName))

		case ir.StepQuery:
			declaredVars[s.VarName] = true
			mName, wExpr := lay.formatGoStepRepoCall(arch.RepoMethodFind, s.WherePredicates, s.WhereField, s.WhereValue)
			sb.WriteString(fmt.Sprintf("%s%sPtr, err := uc.%sRepo.%s(ctx, %s)\n", indent, s.VarName, s.EntityName, mName, wExpr))
			sb.WriteString(fmt.Sprintf("%sif err != nil {\n%s\treturn zero, err\n%s}\n", indent, indent, indent))
			if s.ElseError != "" {
				sb.WriteString(fmt.Sprintf("%sif %sPtr == nil {\n%s\treturn zero, %s.Err%s\n%s}\n", indent, s.VarName, indent, lay.DomainPkg, s.ElseError, indent))
			}
			sb.WriteString(fmt.Sprintf("%s%s := *%sPtr\n%s_ = %s\n", indent, s.VarName, s.VarName, indent, s.VarName))

		case ir.StepQueryMany:
			declaredVars[s.VarName] = true
			mName := arch.StepRepoMethodName(arch.RepoMethodFindMany, s.WherePredicates, "")
			var argExprs []string
			for _, wp := range s.WherePredicates {
				argExprs = append(argExprs, lay.emitGoExpr(wp.Value, false))
			}

			hasLimitParam := false
			for _, p := range ctx.useCase.Params {
				if p.Name == "limit" {
					hasLimitParam = true
					break
				}
			}

			limitVal := s.Limit
			if limitVal <= 0 {
				limitVal = 20
			}
			limitExpr := fmt.Sprintf("%d", limitVal)
			if s.LimitExpr != nil && s.LimitExpr.Kind != ir.ExprInt {
				limitExpr = lay.emitGoExpr(*s.LimitExpr, false)
			} else if hasLimitParam {
				limitExpr = fmt.Sprintf("func() int { if limit <= 0 || limit > %d { return %d }; return limit }()", limitVal, limitVal)
			}

			cursorExpr := `""`
			if s.CursorExpr != nil {
				cursorExpr = lay.emitGoExpr(*s.CursorExpr, false)
			} else {
				for _, p := range ctx.useCase.Params {
					if p.Name == "cursor" {
						cursorExpr = "cursor"
						break
					}
				}
			}

			directionExpr := `"next"`
			for _, p := range ctx.useCase.Params {
				if p.Name == "direction" {
					directionExpr = "direction"
					break
				}
			}

			callArgs := []string{"ctx"}
			callArgs = append(callArgs, argExprs...)
			callArgs = append(callArgs, cursorExpr, limitExpr, directionExpr)

			sb.WriteString(fmt.Sprintf("%s%sItems, _pi, err := uc.%sRepo.%s(%s)\n",
				indent, s.VarName, s.EntityName, mName, strings.Join(callArgs, ", ")))
			sb.WriteString(fmt.Sprintf("%sif err != nil {\n%s\treturn zero, err\n%s}\n", indent, indent, indent))
			sb.WriteString(fmt.Sprintf("%s_pageInfo = _pi\n", indent))
			sb.WriteString(fmt.Sprintf("%s%s := %sItems\n%s_ = %s\n",
				indent, s.VarName, s.VarName, indent, s.VarName))

		case ir.StepUpdate:
			mName, wExpr := lay.formatGoStepRepoCall(arch.RepoMethodFind, s.WherePredicates, s.WhereField, s.WhereValue)
			updVar := fmt.Sprintf("updTarget%d", idx)
			sb.WriteString(fmt.Sprintf("%s%s, err := uc.%sRepo.%s(ctx, %s)\n", indent, updVar, s.EntityName, mName, wExpr))
			sb.WriteString(fmt.Sprintf("%sif err != nil {\n%s\treturn zero, err\n%s}\n", indent, indent, indent))
			if s.ElseError != "" {
				sb.WriteString(fmt.Sprintf("%sif %s == nil {\n%s\treturn zero, %s.Err%s\n%s}\n", indent, updVar, indent, lay.DomainPkg, s.ElseError, indent))
			}
			sb.WriteString(fmt.Sprintf("%sif %s != nil {\n", indent, updVar))
			for _, fi := range s.Fields {
				sb.WriteString(fmt.Sprintf("%s\t%s.%s = %s\n", indent, updVar, arch.ExportIdent(fi.Name), lay.emitGoExpr(fi.Value, false)))
			}
			ent := ctx.arch.EntityByName(s.EntityName)
			if _, vExport, hasV := arch.EntityVersionField(ent); hasV && ctx.useCase.ExpectedVersionParam != "" {
				sb.WriteString(fmt.Sprintf("%s\t%s.%s = %s\n", indent, updVar, vExport, ctx.useCase.ExpectedVersionParam))
			}
			sb.WriteString(fmt.Sprintf("%s\tif err := uc.%sRepo.Update(ctx, *%s); err != nil {\n%s\t\treturn zero, err\n%s\t}\n",
				indent, s.EntityName, updVar, indent, indent))
			sb.WriteString(fmt.Sprintf("%s}\n", indent))

		case ir.StepDelete:
			mName, wExpr := lay.formatGoStepRepoCall(arch.RepoMethodDelete, s.WherePredicates, s.WhereField, s.WhereValue)
			sb.WriteString(fmt.Sprintf("%sif err := uc.%sRepo.%s(ctx, %s); err != nil {\n%s\treturn zero, err\n%s}\n",
				indent, s.EntityName, mName, wExpr, indent, indent))

		case ir.StepIf:
			if s.Expr != nil {
				sb.WriteString(fmt.Sprintf("%sif %s {\n", indent, lay.emitGoExpr(*s.Expr, false)))
				lay.emitGoSteps(sb, s.ThenSteps, emit.CloneBoolMap(declaredVars), indent+"\t", ctx)
				if len(s.ElseSteps) > 0 {
					sb.WriteString(fmt.Sprintf("%s} else {\n", indent))
					lay.emitGoSteps(sb, s.ElseSteps, emit.CloneBoolMap(declaredVars), indent+"\t", ctx)
				}
				sb.WriteString(fmt.Sprintf("%s}\n", indent))
			}

		case ir.StepMatch:
			if s.Expr != nil {
				enumType := s.Expr.InferredType
				sb.WriteString(fmt.Sprintf("%sswitch %s {\n", indent, lay.emitGoExpr(*s.Expr, false)))
				for _, arm := range s.MatchArms {
					sb.WriteString(fmt.Sprintf("%scase %s.%s%s:\n", indent, lay.DomainPkg, enumType, arch.ExportIdent(arm.Variant)))
					lay.emitGoSteps(sb, arm.Steps, emit.CloneBoolMap(declaredVars), indent+"\t", ctx)
				}
				sb.WriteString(fmt.Sprintf("%s}\n", indent))
			}

		case ir.StepFor:
			if s.Expr != nil {
				sb.WriteString(fmt.Sprintf("%sfor _, %s := range %s {\n%s\t_ = %s\n",
					indent, s.IterVar, lay.emitGoExpr(*s.Expr, false), indent, s.IterVar))
				loopVars := emit.CloneBoolMap(declaredVars)
				loopVars[s.IterVar] = true
				lay.emitGoSteps(sb, s.ThenSteps, loopVars, indent+"\t", ctx)
				sb.WriteString(fmt.Sprintf("%s}\n", indent))
			}

		case ir.StepEmit:
			if ctx.useOutbox {
				sb.WriteString(fmt.Sprintf("%soutboxEvID%d := fmt.Sprintf(\"%%d\", time.Now().UnixNano())\n", indent, idx))
				sb.WriteString(fmt.Sprintf("%sif uc.OutboxStore != nil {\n", indent))
				sb.WriteString(fmt.Sprintf("%s\tpayloadBytes, _ := json.Marshal(%s.%s{\n", indent, lay.DomainPkg, s.EventName))
				lay.emitGoFieldInits(sb, s.Fields, indent+"\t\t")
				sb.WriteString(fmt.Sprintf("%s\t})\n", indent))
				sb.WriteString(fmt.Sprintf("%s\t_ = uc.OutboxStore.Stage(ctx, runtime.OutboxEvent{\n", indent))
				sb.WriteString(fmt.Sprintf("%s\t\tID: outboxEvID%d,\n", indent, idx))
				sb.WriteString(fmt.Sprintf("%s\t\tTopic: %q,\n", indent, s.EventName))
				sb.WriteString(fmt.Sprintf("%s\t\tEventName: %q,\n", indent, s.EventName))
				sb.WriteString(fmt.Sprintf("%s\t\tPayload: payloadBytes,\n", indent))
				sb.WriteString(fmt.Sprintf("%s\t\tCreatedAt: time.Now(),\n", indent))
				sb.WriteString(fmt.Sprintf("%s\t})\n", indent))
				sb.WriteString(fmt.Sprintf("%s}\n", indent))
			}
			sb.WriteString(fmt.Sprintf("%s*postCommitEvents = append(*postCommitEvents, func() {\n", indent))
			sb.WriteString(fmt.Sprintf("%s\tif uc.Events != nil {\n%s\t\t_ = uc.Events.Publish(ctx, %q, %s.%s{\n", indent, indent, s.EventName, lay.DomainPkg, s.EventName))
			lay.emitGoFieldInits(sb, s.Fields, indent+"\t\t\t")
			sb.WriteString(fmt.Sprintf("%s\t\t})\n", indent))
			if ctx.useOutbox {
				sb.WriteString(fmt.Sprintf("%s\t\tif uc.OutboxStore != nil {\n%s\t\t\t_ = uc.OutboxStore.MarkPublished(ctx, outboxEvID%d)\n%s\t\t}\n", indent, indent, idx, indent))
			}
			sb.WriteString(fmt.Sprintf("%s\t}\n%s})\n", indent, indent))

		case ir.StepSlot:
			var callArgs []string
			callArgs = append(callArgs, "ctx")
			for _, arg := range s.SlotArgs {
				callArgs = append(callArgs, lay.emitGoExpr(arg, false))
			}
			callExpr := fmt.Sprintf("uc.%sSlot.%s(%s)", s.SlotName, s.SlotMethod, strings.Join(callArgs, ", "))
			if s.VarName != "" {
				declaredVars[s.VarName] = true
				sb.WriteString(fmt.Sprintf("%s%s, err := %s\n%s_ = %s\n", indent, s.VarName, callExpr, indent, s.VarName))
			} else {
				sb.WriteString(fmt.Sprintf("%serr := %s\n", indent, callExpr))
			}
			if s.ElseError != "" {
				sb.WriteString(fmt.Sprintf("%sif err != nil {\n%s\treturn zero, %s.Err%s\n%s}\n", indent, indent, lay.DomainPkg, s.ElseError, indent))
			} else {
				sb.WriteString(fmt.Sprintf("%sif err != nil {\n%s\treturn zero, err\n%s}\n", indent, indent, indent))
			}

		case ir.StepReturn:
			if s.Expr != nil {
				retVal := lay.emitGoExpr(*s.Expr, false)
				if ctx.useCase.IsListReturning {
					sb.WriteString(fmt.Sprintf("%svar _finalPI %s.PageInfo\n%sif _pageInfo != nil { _finalPI = *_pageInfo }\n", indent, lay.DomainPkg, indent))
					sb.WriteString(fmt.Sprintf("%sreturn &%s.%sResult{ Items: %s, PageInfo: _finalPI }, nil\n", indent, lay.DomainPkg, ctx.useCase.ActionName, retVal))
				} else {
					sb.WriteString(fmt.Sprintf("%sreturn %s, nil\n", indent, retVal))
				}
			}
		}
	}
}
