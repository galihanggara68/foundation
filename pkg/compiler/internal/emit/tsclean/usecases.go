package tsclean

import (
	"fmt"
	"strings"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/arch"
)

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
	if uc.IsListReturning {
		retTS = fmt.Sprintf("{ items: %s; page_info: domain.PageInfo }", tsType(uc.ReturnType, false))
	}

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
	sb.WriteString("    let _pageInfo: domain.PageInfo | undefined;\n")
	emitTSSteps(&sb, uc.Steps, declaredVars, "    ", ctx)
	if uc.IsListReturning {
		sb.WriteString("    return { items: [], page_info: _pageInfo ?? { next_cursor: \"\", prev_cursor: \"\", has_next: false, has_prev: false } };\n  }\n}\n")
	} else {
		sb.WriteString(fmt.Sprintf("    return %s;\n  }\n}\n", tsZeroVal(uc.ReturnType, archIR, false)))
	}

	return sb.String()
}
