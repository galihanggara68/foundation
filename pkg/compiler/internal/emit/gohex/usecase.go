package gohex

import (
	"fmt"
	"strings"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/arch"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/emit"
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
	if isIdem || isCache || isRetry {
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
		execParams = append(execParams, fmt.Sprintf("%s %s", p.Name, lay.goType(p.Type, false)))
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
	sb.WriteString(fmt.Sprintf("\tvar zero %s\n\t_ = zero\n\t_ = %s.Actor{}\n", retType, lay.DomainPkg))
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

	sb.WriteString("}\n")
	return sb.String()
}

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
			mName := "FindBy" + arch.ExportIdent(s.WhereField)
			wExpr := lay.emitGoExpr(*s.WhereValue, false)
			sb.WriteString(fmt.Sprintf("%s%sPtr, err := uc.%sRepo.%s(ctx, %s)\n", indent, s.VarName, s.EntityName, mName, wExpr))
			sb.WriteString(fmt.Sprintf("%sif err != nil {\n%s\treturn zero, err\n%s}\n", indent, indent, indent))
			if s.ElseError != "" {
				sb.WriteString(fmt.Sprintf("%sif %sPtr == nil {\n%s\treturn zero, %s.Err%s\n%s}\n", indent, s.VarName, indent, lay.DomainPkg, s.ElseError, indent))
			}
			sb.WriteString(fmt.Sprintf("%s%s := *%sPtr\n%s_ = %s\n", indent, s.VarName, s.VarName, indent, s.VarName))

		case ir.StepUpdate:
			mName := "FindBy" + arch.ExportIdent(s.WhereField)
			wExpr := lay.emitGoExpr(*s.WhereValue, false)
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
			mName := "DeleteBy" + arch.ExportIdent(s.WhereField)
			wExpr := lay.emitGoExpr(*s.WhereValue, false)
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
				sb.WriteString(fmt.Sprintf("%sreturn %s, nil\n", indent, lay.emitGoExpr(*s.Expr, false)))
			}
		}
	}
}
