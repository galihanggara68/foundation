package gohex

import (
	"fmt"
	"strings"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/arch"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
)

func (lay goLayout) emitGoSingleUseCase(archIR *arch.ArchitectureIR, uc arch.UseCaseIR) string {
	var sb strings.Builder
	module := archIR.Module
	_, isTx := uc.HasTrait("transactional")
	idemTrait, isIdem := uc.HasTrait("idempotent")
	cacheTrait, isCache := uc.HasTrait("cache")
	retryTrait, isRetry := uc.HasTrait("retry")
	_, isOutbox := uc.HasTrait("outbox")

	sb.WriteString(fmt.Sprintf("package %s\n\nimport (\n\t\"context\"\n", lay.UseCasesPkg))
	if isIdem || isCache {
		sb.WriteString("\t\"encoding/json\"\n\t\"fmt\"\n")
	}
	hasTime := false
	for _, p := range uc.Params {
		if p.Kind == ir.KindTimestamp {
			hasTime = true
		}
	}
	if uc.ReturnType == "timestamp" {
		hasTime = true
	}
	if isIdem || isCache || isRetry || hasTime {
		sb.WriteString("\t\"time\"\n")
	}
	if isIdem || isCache || isOutbox {
		sb.WriteString("\t\"github.com/galihanggara68/foundation/contrib/go/runtime\"\n")
	}
	sb.WriteString(fmt.Sprintf("\t\"%s/%s\"\n", module, lay.DomainDir))
	if len(uc.RepoDeps) > 0 || len(uc.SlotDeps) > 0 || uc.EmitsEvents || isTx {
		sb.WriteString(fmt.Sprintf("\t\"%s/%s\"\n", module, lay.PortsDir))
	}
	sb.WriteString(")\n\n")

	retType := lay.goType(uc.ReturnType, false)
	if uc.IsListReturning {
		retType = fmt.Sprintf("*%s.%sResult", lay.DomainPkg, uc.ActionName)
	}

	sb.WriteString(fmt.Sprintf("type %s struct {\n", uc.Name))
	for _, dep := range uc.RepoDeps {
		sb.WriteString(fmt.Sprintf("\t%sRepo %s.%sRepository\n", dep, lay.PortsPkg, dep))
	}
	for _, sDep := range uc.SlotDeps {
		sb.WriteString(fmt.Sprintf("\t%sSlot %s.%sSlot\n", sDep, lay.PortsPkg, sDep))
	}
	if uc.EmitsEvents {
		sb.WriteString(fmt.Sprintf("\tEvents %s.EventPublisher\n", lay.PortsPkg))
	}
	if isTx {
		sb.WriteString(fmt.Sprintf("\tTxManager %s.TransactionManager\n", lay.PortsPkg))
	}
	if isIdem {
		sb.WriteString("\tIdemStore runtime.IdempotencyStore\n")
	}
	if isCache {
		sb.WriteString("\tCacheStore runtime.CacheStore\n")
	}
	if isOutbox {
		sb.WriteString("\tOutboxStore runtime.OutboxStore\n")
	}
	sb.WriteString("}\n\n")

	var ctorParams []string
	for _, dep := range uc.RepoDeps {
		ctorParams = append(ctorParams, fmt.Sprintf("%sRepo %s.%sRepository", strings.ToLower(dep), lay.PortsPkg, dep))
	}
	for _, sDep := range uc.SlotDeps {
		ctorParams = append(ctorParams, fmt.Sprintf("%sSlot %s.%sSlot", strings.ToLower(sDep), lay.PortsPkg, sDep))
	}
	if uc.EmitsEvents {
		ctorParams = append(ctorParams, fmt.Sprintf("events %s.EventPublisher", lay.PortsPkg))
	}
	if isTx {
		ctorParams = append(ctorParams, fmt.Sprintf("txManager %s.TransactionManager", lay.PortsPkg))
	}
	if isIdem {
		ctorParams = append(ctorParams, "idemStore runtime.IdempotencyStore")
	}
	if isCache {
		ctorParams = append(ctorParams, "cacheStore runtime.CacheStore")
	}
	if isOutbox {
		ctorParams = append(ctorParams, "outboxStore runtime.OutboxStore")
	}
	sb.WriteString(fmt.Sprintf("func New%s(%s) *%s {\n\treturn &%s{\n", uc.Name, strings.Join(ctorParams, ", "), uc.Name, uc.Name))
	for _, dep := range uc.RepoDeps {
		sb.WriteString(fmt.Sprintf("\t\t%sRepo: %sRepo,\n", dep, strings.ToLower(dep)))
	}
	for _, sDep := range uc.SlotDeps {
		sb.WriteString(fmt.Sprintf("\t\t%sSlot: %sSlot,\n", sDep, strings.ToLower(sDep)))
	}
	if uc.EmitsEvents {
		sb.WriteString("\t\tEvents: events,\n")
	}
	if isTx {
		sb.WriteString("\t\tTxManager: txManager,\n")
	}
	if isIdem {
		sb.WriteString("\t\tIdemStore: idemStore,\n")
	}
	if isCache {
		sb.WriteString("\t\tCacheStore: cacheStore,\n")
	}
	if isOutbox {
		sb.WriteString("\t\tOutboxStore: outboxStore,\n")
	}
	sb.WriteString("\t}\n}\n\n")

	var execParams []string
	var callArgNames []string
	execParams = append(execParams, "ctx context.Context")
	callArgNames = append(callArgNames, "ctx")
	for _, p := range uc.Params {
		execParams = append(execParams, fmt.Sprintf("%s %s", p.Name, lay.goKindType(p.Kind, p.Type, false)))
		callArgNames = append(callArgNames, p.Name)
	}

	sb.WriteString(fmt.Sprintf("func (uc *%s) Execute(%s) (%s, error) {\n", uc.Name, strings.Join(execParams, ", "), retType))
	sb.WriteString("\tvar postCommitEvents []func()\n")
	if isIdem {
		keyExpr := formatIdempotentKeyGoExpr(idemTrait.Args["key"], uc.Params)
		sb.WriteString(fmt.Sprintf("\tidemKey := fmt.Sprintf(\"%%v\", %s)\n", keyExpr))
		sb.WriteString("\tstatus, cachedPayload, unlock, idemErr := uc.IdemStore.GetOrLock(ctx, idemKey, 5*time.Minute)\n")
		sb.WriteString("\tif idemErr != nil {\n\t\tvar zero " + retType + "\n\t\treturn zero, idemErr\n\t}\n")
		sb.WriteString(fmt.Sprintf("\tif status == runtime.IdempotencyStatusCompleted && len(cachedPayload) > 0 {\n\t\tvar cachedRes %s\n\t\tif jsonErr := json.Unmarshal(cachedPayload, &cachedRes); jsonErr == nil {\n\t\t\treturn cachedRes, nil\n\t\t}\n\t}\n", retType))
	}
	if isCache {
		ttlSecs := "60"
		if tVal, ok := cacheTrait.Args["ttl"]; ok && tVal != "" {
			ttlSecs = tVal
		}
		var keyParts []string
		for _, p := range uc.Params {
			keyParts = append(keyParts, p.Name)
		}
		if len(keyParts) > 0 {
			sb.WriteString(fmt.Sprintf("\tcacheKey := fmt.Sprintf(\"%%v\", %s)\n", strings.Join(keyParts, ", ")))
		} else {
			sb.WriteString("\tcacheKey := \"default\"\n")
		}
		sb.WriteString(fmt.Sprintf("\tif cachedRaw, found, _ := uc.CacheStore.Get(ctx, cacheKey); found {\n\t\tvar cachedVal %s\n\t\tif jsonErr := json.Unmarshal(cachedRaw, &cachedVal); jsonErr == nil {\n\t\t\treturn cachedVal, nil\n\t\t}\n\t}\n", retType))
		sb.WriteString(fmt.Sprintf("\tcacheTTL := time.Duration(%s) * time.Second\n", ttlSecs))
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

	sb.WriteString(fmt.Sprintf("\tvar finalRes %s\n\tvar finalErr error\n\tvar idemRecorded bool\n\t_ = idemRecorded\n", retType))
	sb.WriteString(fmt.Sprintf("\tfor attempt := 0; attempt < %s; attempt++ {\n", attempts))
	sb.WriteString("\t\tpostCommitEvents = nil\n")
	if isRetry {
		sb.WriteString(fmt.Sprintf("\t\tif attempt > 0 && %s > 0 {\n\t\t\ttime.Sleep(time.Duration(%s) * time.Millisecond)\n\t\t}\n", backoffMs, backoffMs))
	}

	txArgs := append([]string{"txCtx"}, callArgNames[1:]...)
	txArgs = append(txArgs, "&postCommitEvents")
	regArgs := append(append([]string(nil), callArgNames...), "&postCommitEvents")

	if isTx {
		sb.WriteString("\t\tif uc.TxManager != nil {\n")
		sb.WriteString("\t\t\tfinalErr = uc.TxManager.WithinTransaction(ctx, func(txCtx context.Context) error {\n")
		sb.WriteString(fmt.Sprintf("\t\t\t\tvar txErr error\n\t\t\t\tfinalRes, txErr = uc.executePipeline(%s)\n", strings.Join(txArgs, ", ")))
		if isIdem {
			sb.WriteString("\t\t\t\tif txErr == nil && unlock != nil {\n\t\t\t\t\trespBytes, _ := json.Marshal(finalRes)\n\t\t\t\t\t_ = unlock(txCtx, true, respBytes)\n\t\t\t\t\tidemRecorded = true\n\t\t\t\t}\n")
		}
		sb.WriteString("\t\t\t\treturn txErr\n\t\t\t})\n")
		sb.WriteString("\t\t} else {\n")
		sb.WriteString(fmt.Sprintf("\t\t\tfinalRes, finalErr = uc.executePipeline(%s)\n", strings.Join(regArgs, ", ")))
		sb.WriteString("\t\t}\n")
	} else {
		sb.WriteString(fmt.Sprintf("\t\tfinalRes, finalErr = uc.executePipeline(%s)\n", strings.Join(regArgs, ", ")))
	}
	sb.WriteString("\t\tif finalErr == nil {\n\t\t\tbreak\n\t\t}\n\t}\n")

	if isIdem {
		sb.WriteString("\tif unlock != nil {\n\t\tif finalErr == nil && !idemRecorded {\n\t\t\trespBytes, _ := json.Marshal(finalRes)\n\t\t\t_ = unlock(ctx, true, respBytes)\n\t\t} else if finalErr != nil {\n\t\t\t_ = unlock(ctx, false, nil)\n\t\t}\n\t}\n")
	}
	if isCache {
		sb.WriteString("\tif finalErr == nil {\n\t\tcacheBytes, _ := json.Marshal(finalRes)\n\t\t_ = uc.CacheStore.Set(ctx, cacheKey, cacheBytes, cacheTTL)\n\t}\n")
	}
	sb.WriteString("\tif finalErr == nil {\n\t\tfor _, fn := range postCommitEvents {\n\t\t\tfn()\n\t\t}\n\t}\n")
	sb.WriteString("\treturn finalRes, finalErr\n}\n\n")

	pipeExecParams := append(append([]string(nil), execParams...), "postCommitEvents *[]func()")
	sb.WriteString(fmt.Sprintf("func (uc *%s) executePipeline(%s) (%s, error) {\n", uc.Name, strings.Join(pipeExecParams, ", "), retType))
	sb.WriteString(fmt.Sprintf("\tvar zero %s\n\t_ = zero\n\t_ = %s.Actor{}\n\tvar _pageInfo *%s.PageInfo\n\t_ = _pageInfo\n", retType, lay.DomainPkg, lay.DomainPkg))
	if uc.IsListReturning {
		sb.WriteString(fmt.Sprintf("\tvar _finalPI %s.PageInfo\n\t_ = _finalPI\n", lay.DomainPkg))
	}
	if uc.UsesActor {
		sb.WriteString(fmt.Sprintf("\tactor := %s.ActorFromContext(ctx)\n\t_ = actor\n", lay.DomainPkg))
	}
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
	lay.emitGoSteps(&sb, uc.Steps, declaredVars, "\t", ctx)

	if uc.ReturnType != "" {
		if uc.IsListReturning {
			sb.WriteString("\tif _pageInfo != nil { _finalPI = *_pageInfo }\n")
			sb.WriteString(fmt.Sprintf("\treturn &%s.%sResult{ PageInfo: _finalPI }, nil\n", lay.DomainPkg, uc.ActionName))
		} else {
			sb.WriteString("\treturn zero, nil\n")
		}
	} else {
		sb.WriteString("\treturn nil\n")
	}
	sb.WriteString("}\n")
	return sb.String()
}
