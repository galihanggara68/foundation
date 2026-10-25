package tsclean

import (
	"fmt"
	"strings"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/arch"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/emit"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
)

func emitTSPorts(archIR *arch.ArchitectureIR) string {
	var sb strings.Builder
	sb.WriteString(`import * as domain from "../entities/domain.ts";

export interface EventPublisher {
  publish(eventType: string, payload: unknown): Promise<void>;
  subscribe(eventType: string, handler: (payload: unknown) => Promise<void>): void;
}

export interface TransactionManager {
  withinTransaction<T>(fn: () => Promise<T>): Promise<T>;
}

export interface ActorResolverGateway {
  resolveActor(headers: Record<string, string | undefined>): Promise<Record<string, unknown>>;
}

`)

	for _, p := range archIR.Ports {
		sb.WriteString(fmt.Sprintf("export interface %s {\n", p.InterfaceName))
		for _, m := range p.Methods {
			switch m.Kind {
			case arch.RepoMethodCreate:
				sb.WriteString(fmt.Sprintf("  create(entity: Partial<domain.%s>): Promise<domain.%s>;\n", m.EntityName, m.EntityName))
			case arch.RepoMethodUpdate:
				sb.WriteString(fmt.Sprintf("  update(entity: domain.%s): Promise<void>;\n", m.EntityName))
			case arch.RepoMethodDelete:
				sb.WriteString(fmt.Sprintf("  %s(%s: %s): Promise<void>;\n",
					toCamelCase(m.Name), m.WhereField, tsType(m.WhereType, false)))
			case arch.RepoMethodFind:
				sb.WriteString(fmt.Sprintf("  %s(%s: %s): Promise<domain.%s | null>;\n",
					toCamelCase(m.Name), m.WhereField, tsType(m.WhereType, false), m.EntityName))
			}
		}
		sb.WriteString("}\n\n")
	}

	for _, sl := range archIR.Slots {
		sb.WriteString(fmt.Sprintf("export interface %sSlot {\n", sl.Name))
		for _, m := range sl.Methods {
			var params []string
			for _, p := range m.Params {
				params = append(params, fmt.Sprintf("%s: %s", p.Name, tsType(p.Type, false)))
			}
			retTS := tsType(m.ReturnType, false)
			sb.WriteString(fmt.Sprintf("  %s(%s): Promise<%s>;\n", toCamelCase(m.Name), strings.Join(params, ", "), retTS))
		}
		sb.WriteString("}\n\n")
	}

	return sb.String()
}

func emitTSUseCase(archIR *arch.ArchitectureIR, uc arch.UseCaseIR) string {
	var sb strings.Builder
	_, isTx := uc.HasTrait("transactional")
	idemTrait, isIdem := uc.HasTrait("idempotent")
	cacheTrait, isCache := uc.HasTrait("cache")
	retryTrait, isRetry := uc.HasTrait("retry")
	_, isOutbox := uc.HasTrait("outbox")

	sb.WriteString("import * as domain from \"../entities/domain.ts\";\n")
	sb.WriteString("import * as ports from \"./ports.ts\";\n")
	if isIdem || isCache || isOutbox {
		sb.WriteString("import type { IdempotencyStore, CacheStore, OutboxStore } from \"@foundation-contrib/runtime\";\n")
	}
	sb.WriteString("\n")

	retTS := tsType(uc.ReturnType, false)

	sb.WriteString(fmt.Sprintf("export class %s {\n", uc.Name))
	for _, dep := range uc.RepoDeps {
		sb.WriteString(fmt.Sprintf("  private readonly %sRepo: ports.%sRepository;\n", toCamelCase(dep), dep))
	}
	for _, sDep := range uc.SlotDeps {
		sb.WriteString(fmt.Sprintf("  private readonly %sSlot: ports.%sSlot;\n", toCamelCase(sDep), sDep))
	}
	if uc.EmitsEvents {
		sb.WriteString("  private readonly events?: ports.EventPublisher;\n")
	}
	if isTx {
		sb.WriteString("  private readonly txManager?: ports.TransactionManager;\n")
	}
	if isIdem {
		sb.WriteString("  private readonly idemStore?: IdempotencyStore;\n")
	}
	if isCache {
		sb.WriteString("  private readonly cacheStore?: CacheStore;\n")
	}
	if isOutbox {
		sb.WriteString("  private readonly outboxStore?: OutboxStore;\n")
	}
	sb.WriteString("\n  constructor(\n")
	var ctorParams []string
	for _, dep := range uc.RepoDeps {
		ctorParams = append(ctorParams, fmt.Sprintf("    %sRepo: ports.%sRepository", toCamelCase(dep), dep))
	}
	for _, sDep := range uc.SlotDeps {
		ctorParams = append(ctorParams, fmt.Sprintf("    %sSlot: ports.%sSlot", toCamelCase(sDep), sDep))
	}
	if uc.EmitsEvents {
		ctorParams = append(ctorParams, "    events?: ports.EventPublisher")
	}
	if isTx {
		ctorParams = append(ctorParams, "    txManager?: ports.TransactionManager")
	}
	if isIdem {
		ctorParams = append(ctorParams, "    idemStore?: IdempotencyStore")
	}
	if isCache {
		ctorParams = append(ctorParams, "    cacheStore?: CacheStore")
	}
	if isOutbox {
		ctorParams = append(ctorParams, "    outboxStore?: OutboxStore")
	}
	sb.WriteString(strings.Join(ctorParams, ",\n") + "\n  ) {\n")
	for _, dep := range uc.RepoDeps {
		c := toCamelCase(dep)
		sb.WriteString(fmt.Sprintf("    this.%sRepo = %sRepo;\n", c, c))
	}
	for _, sDep := range uc.SlotDeps {
		c := toCamelCase(sDep)
		sb.WriteString(fmt.Sprintf("    this.%sSlot = %sSlot;\n", c, c))
	}
	if uc.EmitsEvents {
		sb.WriteString("    this.events = events;\n")
	}
	if isTx {
		sb.WriteString("    this.txManager = txManager;\n")
	}
	if isIdem {
		sb.WriteString("    this.idemStore = idemStore;\n")
	}
	if isCache {
		sb.WriteString("    this.cacheStore = cacheStore;\n")
	}
	if isOutbox {
		sb.WriteString("    this.outboxStore = outboxStore;\n")
	}
	sb.WriteString("  }\n\n")

	var execParams []string
	var callArgs []string
	execParams = append(execParams, "actor: domain.Actor")
	callArgs = append(callArgs, "actor")
	for _, p := range uc.Params {
		execParams = append(execParams, fmt.Sprintf("%s: %s", p.Name, tsType(p.Type, false)))
		callArgs = append(callArgs, p.Name)
	}

	paramsSig := strings.Join(execParams, ", ")
	pipelineCallArgs := append(callArgs, "postCommitEvents")
	pipelineArgsSig := strings.Join(pipelineCallArgs, ", ")

	sb.WriteString(fmt.Sprintf("  async execute(%s): Promise<%s> {\n", paramsSig, retTS))
	sb.WriteString("    const postCommitEvents: Array<() => Promise<void>> = [];\n")

	if isIdem {
		keyExpr := idemTrait.Args["key"]
		if keyExpr == "" && len(uc.Params) > 0 {
			keyExpr = uc.Params[0].Name
		} else if keyExpr == "" {
			keyExpr = `"default"`
		}
		sb.WriteString(fmt.Sprintf("    const idemKey = String(%s);\n", keyExpr))
		sb.WriteString("    let unlock: ((completed: boolean, responsePayload?: Uint8Array) => Promise<void>) | undefined;\n")
		sb.WriteString("    if (this.idemStore) {\n")
		sb.WriteString("      const idemRes = await this.idemStore.getOrLock(idemKey, 5 * 60 * 1000);\n")
		sb.WriteString(fmt.Sprintf("      if (idemRes.status === \"completed\" && idemRes.payload && idemRes.payload.length > 0) {\n        return JSON.parse(new TextDecoder().decode(idemRes.payload)) as %s;\n      }\n", retTS))
		sb.WriteString("      unlock = idemRes.unlock;\n")
		sb.WriteString("    }\n")
	}

	ttlSecs := "60"
	if isCache {
		if tVal, ok := cacheTrait.Args["ttl"]; ok && tVal != "" {
			ttlSecs = tVal
		}
		var keyParts []string
		for _, p := range uc.Params {
			keyParts = append(keyParts, p.Name)
		}
		cacheKeyStr := `"default"`
		if len(keyParts) > 0 {
			cacheKeyStr = "JSON.stringify([" + strings.Join(keyParts, ", ") + "])"
		}
		sb.WriteString(fmt.Sprintf("    const cacheKey = %s;\n", cacheKeyStr))
		sb.WriteString("    if (this.cacheStore) {\n")
		sb.WriteString("      const cached = await this.cacheStore.get(cacheKey);\n")
		sb.WriteString(fmt.Sprintf("      if (cached.found && cached.value) {\n        return JSON.parse(new TextDecoder().decode(cached.value)) as %s;\n      }\n", retTS))
		sb.WriteString("    }\n")
	}

	attempts := "1"
	backoffMs := "0"
	if isRetry {
		if a, ok := retryTrait.Args["attempts"]; ok && a != "" {
			attempts = a
		}
		if b, ok := retryTrait.Args["backoff"]; ok && b != "" {
			backoffMs = b
		}
	}

	sb.WriteString(fmt.Sprintf("    let finalRes: %s;\n", retTS))
	sb.WriteString("    let finalErr: unknown;\n")
	sb.WriteString("    let idemRecorded = false;\n")
	sb.WriteString(fmt.Sprintf("    for (let attempt = 0; attempt < %s; attempt++) {\n", attempts))
	sb.WriteString("      postCommitEvents.length = 0;\n")
	if isRetry && backoffMs != "0" {
		sb.WriteString(fmt.Sprintf("      if (attempt > 0) { await new Promise((r) => setTimeout(r, %s)); }\n", backoffMs))
	}
	sb.WriteString("      try {\n")
	if isTx {
		sb.WriteString("        if (this.txManager) {\n")
		if isIdem {
			sb.WriteString(fmt.Sprintf("          finalRes = await this.txManager.withinTransaction(async () => {\n            const r = await this.executePipeline(%s);\n            if (unlock) {\n              const respBytes = new TextEncoder().encode(JSON.stringify(r!));\n              await unlock(true, respBytes);\n              idemRecorded = true;\n            }\n            return r;\n          });\n", pipelineArgsSig))
		} else {
			sb.WriteString(fmt.Sprintf("          finalRes = await this.txManager.withinTransaction(() => this.executePipeline(%s));\n", pipelineArgsSig))
		}
		sb.WriteString("        } else {\n")
		sb.WriteString(fmt.Sprintf("          finalRes = await this.executePipeline(%s);\n", pipelineArgsSig))
		sb.WriteString("        }\n")
	} else {
		sb.WriteString(fmt.Sprintf("        finalRes = await this.executePipeline(%s);\n", pipelineArgsSig))
	}
	sb.WriteString("        finalErr = undefined;\n        break;\n")
	sb.WriteString("      } catch (err) {\n        finalErr = err;\n      }\n    }\n")

	if isIdem {
		sb.WriteString("    if (unlock) {\n      if (!finalErr && !idemRecorded) {\n        const respBytes = new TextEncoder().encode(JSON.stringify(finalRes!));\n        await unlock(true, respBytes);\n      } else if (finalErr) {\n        await unlock(false);\n      }\n    }\n")
	}
	sb.WriteString("    if (finalErr) {\n      throw finalErr;\n    }\n")
	if isCache {
		sb.WriteString(fmt.Sprintf("    if (this.cacheStore) {\n      const respBytes = new TextEncoder().encode(JSON.stringify(finalRes!));\n      await this.cacheStore.set(cacheKey, respBytes, %s * 1000);\n    }\n", ttlSecs))
	}
	sb.WriteString("    for (const fn of postCommitEvents) {\n      await fn();\n    }\n")
	sb.WriteString("    return finalRes!;\n  }\n\n")

	pipelineParams := append(execParams, "postCommitEvents: Array<() => Promise<void>>")
	sb.WriteString(fmt.Sprintf("  private async executePipeline(%s): Promise<%s> {\n    void actor;\n    void postCommitEvents;\n",
		strings.Join(pipelineParams, ", "), retTS))

	declaredVars := make(map[string]bool)
	for _, p := range uc.Params {
		declaredVars[p.Name] = true
	}
	stepCounter := 0
	ctx := stepEmitCtx{
		arch:        archIR,
		useCase:     uc,
		useOutbox:   isOutbox && uc.EmitsEvents,
		stepCounter: &stepCounter,
	}
	emitTSSteps(&sb, uc.Steps, declaredVars, "    ", ctx)
	sb.WriteString(fmt.Sprintf("    return %s;\n  }\n}\n", tsZeroVal(uc.ReturnType, archIR, false)))

	return sb.String()
}

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
			mName := "findBy" + arch.ExportIdent(s.WhereField)
			wExpr := emitTSExpr(*s.WhereValue, false)
			sb.WriteString(fmt.Sprintf("%sconst %sOrNull = await this.%sRepo.%s(%s);\n",
				indent, s.VarName, toCamelCase(s.EntityName), mName, wExpr))
			if s.ElseError != "" {
				sb.WriteString(fmt.Sprintf("%sif (!%sOrNull) {\n%s  throw domain.Err%s;\n%s}\n",
					indent, s.VarName, indent, s.ElseError, indent))
			}
			sb.WriteString(fmt.Sprintf("%sconst %s = %sOrNull!;\n", indent, s.VarName, s.VarName))

		case ir.StepUpdate:
			mName := "findBy" + arch.ExportIdent(s.WhereField)
			wExpr := emitTSExpr(*s.WhereValue, false)
			updVar := fmt.Sprintf("updTarget%d", idx)
			sb.WriteString(fmt.Sprintf("%sconst %s = await this.%sRepo.%s(%s);\n",
				indent, updVar, toCamelCase(s.EntityName), mName, wExpr))
			if s.ElseError != "" {
				sb.WriteString(fmt.Sprintf("%sif (!%s) {\n%s  throw domain.Err%s;\n%s}\n",
					indent, updVar, indent, s.ElseError, indent))
			}
			sb.WriteString(fmt.Sprintf("%sif (%s) {\n", indent, updVar))
			for _, fi := range s.Fields {
				sb.WriteString(fmt.Sprintf("%s  %s.%s = %s;\n", indent, updVar, fi.Name, emitTSExpr(fi.Value, false)))
			}
			ent := ctx.arch.EntityByName(s.EntityName)
			if vField, _, hasV := arch.EntityVersionField(ent); hasV && ctx.useCase.ExpectedVersionParam != "" {
				sb.WriteString(fmt.Sprintf("%s  %s.%s = %s;\n", indent, updVar, vField, ctx.useCase.ExpectedVersionParam))
			}
			sb.WriteString(fmt.Sprintf("%s  await this.%sRepo.update(%s);\n%s}\n",
				indent, toCamelCase(s.EntityName), updVar, indent))

		case ir.StepDelete:
			mName := "deleteBy" + arch.ExportIdent(s.WhereField)
			wExpr := emitTSExpr(*s.WhereValue, false)
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
				sb.WriteString(fmt.Sprintf("%sreturn %s;\n", indent, emitTSExpr(*s.Expr, false)))
			}
		}
	}
}
