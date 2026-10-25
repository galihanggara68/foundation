package tsclean

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

func emitTSSteps(sb *strings.Builder, steps []ir.StepDecl, declaredVars map[string]bool, indent string, ctx stepEmitCtx) {
	for _, s := range steps {
		*ctx.stepCounter++
		idx := *ctx.stepCounter
		switch s.Kind {
		case ir.StepRequire:
			var args []string
			for _, arg := range s.PolicyArgs {
				args = append(args, emitTSExpr(arg, false))
			}
			sb.WriteString(fmt.Sprintf("%sif (!domain.%s(%s)) {\n%s  throw domain.Err%s;\n%s}\n",
				indent, s.PolicyName, strings.Join(args, ", "), indent, s.ElseError, indent))

		case ir.StepGuard:
			if s.Expr != nil {
				sb.WriteString(fmt.Sprintf("%sif (!(%s)) {\n%s  throw domain.Err%s;\n%s}\n",
					indent, emitTSExpr(*s.Expr, false), indent, s.ElseError, indent))
			}

		case ir.StepLet:
			if s.Expr != nil {
				prefix := "let "
				if declaredVars[s.VarName] {
					prefix = ""
				} else {
					declaredVars[s.VarName] = true
				}
				sb.WriteString(fmt.Sprintf("%s%s%s = %s;\n", indent, prefix, s.VarName, emitTSExpr(*s.Expr, false)))
			}

		case ir.StepCreate:
			declaredVars[s.VarName] = true
			sb.WriteString(fmt.Sprintf("%sconst %s = await this.%sRepo.create({\n", indent, s.VarName, toCamelCase(s.EntityName)))
			emitTSFieldInits(sb, s.Fields, indent+"  ")
			sb.WriteString(fmt.Sprintf("%s});\n", indent))

		case ir.StepQuery:
			declaredVars[s.VarName] = true
			mName, wExpr := formatTSStepRepoCall(arch.RepoMethodFind, s)
			sb.WriteString(fmt.Sprintf("%sconst %sOrNull = await this.%sRepo.%s(%s);\n",
				indent, s.VarName, toCamelCase(s.EntityName), mName, wExpr))
			if s.ElseError != "" {
				sb.WriteString(fmt.Sprintf("%sif (!%sOrNull) {\n%s  throw domain.Err%s;\n%s}\n",
					indent, s.VarName, indent, s.ElseError, indent))
			}
			sb.WriteString(fmt.Sprintf("%sconst %s = %sOrNull!;\n", indent, s.VarName, s.VarName))

		case ir.StepQueryMany:
			declaredVars[s.VarName] = true
			mName := toCamelCase(arch.StepRepoMethodName(arch.RepoMethodFindMany, s.WherePredicates, ""))
			var argExprs []string
			for _, wp := range s.WherePredicates {
				argExprs = append(argExprs, emitTSExpr(wp.Value, false))
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
				limitExpr = emitTSExpr(*s.LimitExpr, false)
			} else if hasLimitParam {
				limitExpr = fmt.Sprintf("Math.min(Math.max(limit || %d, 1), %d)", limitVal, limitVal)
			}
			cursorExpr := `""`
			if s.CursorExpr != nil {
				cursorExpr = emitTSExpr(*s.CursorExpr, false)
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
			callArgs := append([]string{}, argExprs...)
			callArgs = append(callArgs, cursorExpr, limitExpr, directionExpr)
			sb.WriteString(fmt.Sprintf("%sconst { items: %sItems, page_info: %sPI } = await this.%sRepo.%s(%s);\n",
				indent, s.VarName, s.VarName, toCamelCase(s.EntityName), mName, strings.Join(callArgs, ", ")))
			sb.WriteString(fmt.Sprintf("%s_pageInfo = %sPI;\n", indent, s.VarName))
			sb.WriteString(fmt.Sprintf("%sconst %s = %sItems;\n", indent, s.VarName, s.VarName))

		case ir.StepUpdate:
			mName, wExpr := formatTSStepRepoCall(arch.RepoMethodFind, s)
			updVar := fmt.Sprintf("updTarget%d", idx)
			sb.WriteString(fmt.Sprintf("%sconst %s = await this.%sRepo.%s(%s);\n",
				indent, updVar, toCamelCase(s.EntityName), mName, wExpr))
			if s.ElseError != "" {
				sb.WriteString(fmt.Sprintf("%sif (!%s) {\n%s  throw domain.Err%s;\n%s}\n",
					indent, updVar, indent, s.ElseError, indent))
			}
			sb.WriteString(fmt.Sprintf("%sif (%s) {\n", indent, updVar))
			for _, fi := range s.Fields {
				sb.WriteString(fmt.Sprintf("%s  %s.%s = %s;\n", indent, updVar, toCamelCase(fi.Name), emitTSExpr(fi.Value, false)))
			}
			ent := ctx.arch.EntityByName(s.EntityName)
			if _, vExport, hasV := arch.EntityVersionField(ent); hasV && ctx.useCase.ExpectedVersionParam != "" {
				sb.WriteString(fmt.Sprintf("%s  %s.%s = %s;\n", indent, updVar, toCamelCase(vExport), toCamelCase(ctx.useCase.ExpectedVersionParam)))
			}
			sb.WriteString(fmt.Sprintf("%s  await this.%sRepo.update(%s);\n", indent, toCamelCase(s.EntityName), updVar))
			sb.WriteString(fmt.Sprintf("%s}\n", indent))

		case ir.StepDelete:
			mName, wExpr := formatTSStepRepoCall(arch.RepoMethodDelete, s)
			sb.WriteString(fmt.Sprintf("%sawait this.%sRepo.%s(%s);\n",
				indent, toCamelCase(s.EntityName), mName, wExpr))

		case ir.StepIf:
			if s.Expr != nil {
				sb.WriteString(fmt.Sprintf("%sif (%s) {\n", indent, emitTSExpr(*s.Expr, false)))
				emitTSSteps(sb, s.ThenSteps, emit.CloneBoolMap(declaredVars), indent+"  ", ctx)
				if len(s.ElseSteps) > 0 {
					sb.WriteString(fmt.Sprintf("%s} else {\n", indent))
					emitTSSteps(sb, s.ElseSteps, emit.CloneBoolMap(declaredVars), indent+"  ", ctx)
				}
				sb.WriteString(fmt.Sprintf("%s}\n", indent))
			}

		case ir.StepMatch:
			if s.Expr != nil {
				sb.WriteString(fmt.Sprintf("%sswitch (%s) {\n", indent, emitTSExpr(*s.Expr, false)))
				for _, arm := range s.MatchArms {
					sb.WriteString(fmt.Sprintf("%s  case %q: {\n", indent, arm.Variant))
					emitTSSteps(sb, arm.Steps, emit.CloneBoolMap(declaredVars), indent+"    ", ctx)
					sb.WriteString(fmt.Sprintf("%s    break;\n%s  }\n", indent, indent))
				}
				sb.WriteString(fmt.Sprintf("%s}\n", indent))
			}

		case ir.StepFor:
			if s.Expr != nil {
				sb.WriteString(fmt.Sprintf("%sfor (const %s of %s) {\n", indent, s.IterVar, emitTSExpr(*s.Expr, false)))
				loopVars := emit.CloneBoolMap(declaredVars)
				loopVars[s.IterVar] = true
				emitTSSteps(sb, s.ThenSteps, loopVars, indent+"  ", ctx)
				sb.WriteString(fmt.Sprintf("%s}\n", indent))
			}

		case ir.StepEmit:
			if ctx.useOutbox {
				sb.WriteString(fmt.Sprintf("%sif (this.outboxStore) {\n", indent))
				sb.WriteString(fmt.Sprintf("%s  const payload = {\n", indent))
				emitTSFieldInits(sb, s.Fields, indent+"    ")
				sb.WriteString(fmt.Sprintf("%s  } satisfies domain.%s;\n", indent, s.EventName))
				sb.WriteString(fmt.Sprintf("%s  await this.outboxStore.stage({\n", indent))
				sb.WriteString(fmt.Sprintf("%s    id: String(Date.now()),\n", indent))
				sb.WriteString(fmt.Sprintf("%s    topic: %q,\n", indent, s.EventName))
				sb.WriteString(fmt.Sprintf("%s    eventName: %q,\n", indent, s.EventName))
				sb.WriteString(fmt.Sprintf("%s    payload: new TextEncoder().encode(JSON.stringify(payload)),\n", indent))
				sb.WriteString(fmt.Sprintf("%s    createdAt: new Date(),\n", indent))
				sb.WriteString(fmt.Sprintf("%s  });\n%s}\n", indent, indent))
			}
			sb.WriteString(fmt.Sprintf("%spostCommitEvents.push(async () => {\n", indent))
			sb.WriteString(fmt.Sprintf("%s  if (this.events) {\n%s    await this.events.publish(%q, {\n", indent, indent, s.EventName))
			emitTSFieldInits(sb, s.Fields, indent+"      ")
			sb.WriteString(fmt.Sprintf("%s    } satisfies domain.%s);\n%s  }\n%s});\n", indent, s.EventName, indent, indent))

		case ir.StepSlot:
			var callArgs []string
			for _, arg := range s.SlotArgs {
				callArgs = append(callArgs, emitTSExpr(arg, false))
			}
			callExpr := fmt.Sprintf("await this.%sSlot.%s(%s)", toCamelCase(s.SlotName), toCamelCase(s.SlotMethod), strings.Join(callArgs, ", "))
			if s.ElseError != "" {
				if s.VarName != "" {
					declaredVars[s.VarName] = true
					sb.WriteString(fmt.Sprintf("%slet %s: Awaited<ReturnType<ports.%sSlot[%q]>>;\n",
						indent, s.VarName, s.SlotName, toCamelCase(s.SlotMethod)))
					sb.WriteString(fmt.Sprintf("%stry {\n%s  %s = %s;\n%s} catch {\n%s  throw domain.Err%s;\n%s}\n",
						indent, indent, s.VarName, callExpr, indent, indent, s.ElseError, indent))
				} else {
					sb.WriteString(fmt.Sprintf("%stry {\n%s  %s;\n%s} catch {\n%s  throw domain.Err%s;\n%s}\n",
						indent, indent, callExpr, indent, indent, s.ElseError, indent))
				}
			} else if s.VarName != "" {
				declaredVars[s.VarName] = true
				sb.WriteString(fmt.Sprintf("%sconst %s = %s;\n", indent, s.VarName, callExpr))
			} else {
				sb.WriteString(fmt.Sprintf("%s%s;\n", indent, callExpr))
			}

		case ir.StepReturn:
			if s.Expr != nil {
				retVal := emitTSExpr(*s.Expr, false)
				if ctx.useCase.IsListReturning {
					sb.WriteString(fmt.Sprintf("%sreturn { items: %s, page_info: _pageInfo || { next_cursor: \"\", prev_cursor: \"\", has_next: false, has_prev: false } };\n", indent, retVal))
				} else {
					sb.WriteString(fmt.Sprintf("%sreturn %s;\n", indent, retVal))
				}
			}
		}
	}
}
