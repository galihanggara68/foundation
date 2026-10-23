package gohex

import (
	"fmt"
	"strings"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/arch"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
)

func hasUCTrait(uc arch.UseCaseIR, name string) (ir.TraitDecl, bool) {
	for _, tr := range uc.Traits {
		if tr.Name == name {
			return tr, true
		}
	}
	return ir.TraitDecl{}, false
}

func emitGoSingleUseCase(module string, uc arch.UseCaseIR) string {
	var sb strings.Builder
	_, isTx := hasUCTrait(uc, "transactional")
	idemTrait, isIdem := hasUCTrait(uc, "idempotent")
	cacheTrait, isCache := hasUCTrait(uc, "cache")
	retryTrait, isRetry := hasUCTrait(uc, "retry")
	_, isOutbox := hasUCTrait(uc, "outbox")

	sb.WriteString("package usecases\n\nimport (\n\t\"context\"\n")
	if isIdem || isCache {
		sb.WriteString("\t\"fmt\"\n\t\"sync\"\n")
	}
	if isCache || isRetry {
		sb.WriteString("\t\"time\"\n")
	}
	sb.WriteString(fmt.Sprintf("\t\"%s/internal/domain\"\n", module))
	if len(uc.RepoDeps) > 0 || len(uc.SlotDeps) > 0 || uc.EmitsEvents || isTx {
		sb.WriteString(fmt.Sprintf("\t\"%s/internal/ports\"\n", module))
	}
	sb.WriteString(")\n\n")

	retType := goType(uc.ReturnType, false)

	if isIdem {
		sb.WriteString(fmt.Sprintf("type idem%sEntry struct {\n\tres %s\n\terr error\n}\n\n", uc.Name, retType))
	}
	if isCache {
		sb.WriteString(fmt.Sprintf("type cache%sEntry struct {\n\tval %s\n\texpiresAt time.Time\n}\n\n", uc.Name, retType))
	}

	sb.WriteString(fmt.Sprintf("type %s struct {\n", uc.Name))
	for _, dep := range uc.RepoDeps {
		sb.WriteString(fmt.Sprintf("\t%sRepo ports.%sRepository\n", dep, dep))
	}
	for _, sDep := range uc.SlotDeps {
		sb.WriteString(fmt.Sprintf("\t%sSlot ports.%sSlot\n", sDep, sDep))
	}
	if uc.EmitsEvents {
		sb.WriteString("\tEvents ports.EventPublisher\n")
	}
	if isTx {
		sb.WriteString("\tTxManager ports.TransactionManager\n")
	}
	if isIdem {
		sb.WriteString(fmt.Sprintf("\tidemMu sync.Mutex\n\tidemStore map[string]idem%sEntry\n", uc.Name))
	}
	if isCache {
		sb.WriteString(fmt.Sprintf("\tcacheMu sync.RWMutex\n\tcacheStore map[string]cache%sEntry\n", uc.Name))
	}
	sb.WriteString("}\n\n")

	var ctorParams []string
	for _, dep := range uc.RepoDeps {
		ctorParams = append(ctorParams, fmt.Sprintf("%sRepo ports.%sRepository", strings.ToLower(dep), dep))
	}
	for _, sDep := range uc.SlotDeps {
		ctorParams = append(ctorParams, fmt.Sprintf("%sSlot ports.%sSlot", strings.ToLower(sDep), sDep))
	}
	if uc.EmitsEvents {
		ctorParams = append(ctorParams, "events ports.EventPublisher")
	}
	if isTx {
		ctorParams = append(ctorParams, "txManager ports.TransactionManager")
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
		sb.WriteString(fmt.Sprintf("\t\tidemStore: make(map[string]idem%sEntry),\n", uc.Name))
	}
	if isCache {
		sb.WriteString(fmt.Sprintf("\t\tcacheStore: make(map[string]cache%sEntry),\n", uc.Name))
	}
	sb.WriteString("\t}\n}\n\n")

	var execParams []string
	var callArgNames []string
	execParams = append(execParams, "ctx context.Context")
	callArgNames = append(callArgNames, "ctx")
	for _, p := range uc.Params {
		execParams = append(execParams, fmt.Sprintf("%s %s", p.Name, goType(p.Type, false)))
		callArgNames = append(callArgNames, p.Name)
	}

	sb.WriteString(fmt.Sprintf("func (uc *%s) Execute(%s) (%s, error) {\n", uc.Name, strings.Join(execParams, ", "), retType))
	if isIdem {
		keyExpr := formatIdempotentKeyGoExpr(idemTrait.Args["key"], uc.Params)
		sb.WriteString(fmt.Sprintf("\tidemKey := fmt.Sprintf(\"%%v\", %s)\n", keyExpr))
		sb.WriteString("\tuc.idemMu.Lock()\n\tif entry, ok := uc.idemStore[idemKey]; ok {\n\t\tuc.idemMu.Unlock()\n\t\treturn entry.res, entry.err\n\t}\n\tuc.idemMu.Unlock()\n")
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
		sb.WriteString("\tuc.cacheMu.RLock()\n\tif cached, ok := uc.cacheStore[cacheKey]; ok && time.Now().Before(cached.expiresAt) {\n\t\tuc.cacheMu.RUnlock()\n\t\treturn cached.val, nil\n\t}\n\tuc.cacheMu.RUnlock()\n")
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

	sb.WriteString(fmt.Sprintf("\tvar finalRes %s\n\tvar finalErr error\n", retType))
	sb.WriteString(fmt.Sprintf("\tfor attempt := 0; attempt < %s; attempt++ {\n", attempts))
	if isRetry {
		sb.WriteString(fmt.Sprintf("\t\tif attempt > 0 && %s > 0 {\n\t\t\ttime.Sleep(time.Duration(%s) * time.Millisecond)\n\t\t}\n", backoffMs, backoffMs))
	}
	if isTx {
		txArgs := append([]string{"txCtx"}, callArgNames[1:]...)
		sb.WriteString("\t\tif uc.TxManager != nil {\n")
		sb.WriteString("\t\t\tfinalErr = uc.TxManager.WithinTransaction(ctx, func(txCtx context.Context) error {\n")
		sb.WriteString(fmt.Sprintf("\t\t\t\tvar txErr error\n\t\t\t\tfinalRes, txErr = uc.executePipeline(%s)\n\t\t\t\treturn txErr\n\t\t\t})\n", strings.Join(txArgs, ", ")))
		sb.WriteString("\t\t} else {\n")
		sb.WriteString(fmt.Sprintf("\t\t\tfinalRes, finalErr = uc.executePipeline(%s)\n", strings.Join(callArgNames, ", ")))
		sb.WriteString("\t\t}\n")
	} else {
		sb.WriteString(fmt.Sprintf("\t\tfinalRes, finalErr = uc.executePipeline(%s)\n", strings.Join(callArgNames, ", ")))
	}
	sb.WriteString("\t\tif finalErr == nil {\n\t\t\tbreak\n\t\t}\n\t}\n")

	if isIdem {
		sb.WriteString(fmt.Sprintf("\tif finalErr == nil {\n\t\tuc.idemMu.Lock()\n\t\tuc.idemStore[idemKey] = idem%sEntry{res: finalRes, err: nil}\n\t\tuc.idemMu.Unlock()\n\t}\n", uc.Name))
	}
	if isCache {
		sb.WriteString(fmt.Sprintf("\tif finalErr == nil {\n\t\tuc.cacheMu.Lock()\n\t\tuc.cacheStore[cacheKey] = cache%sEntry{val: finalRes, expiresAt: time.Now().Add(cacheTTL)}\n\t\tuc.cacheMu.Unlock()\n\t}\n", uc.Name))
	}

	sb.WriteString("\treturn finalRes, finalErr\n}\n\n")

	sb.WriteString(fmt.Sprintf("func (uc *%s) executePipeline(%s) (%s, error) {\n", uc.Name, strings.Join(execParams, ", "), retType))
	sb.WriteString(fmt.Sprintf("\tvar zero %s\n\t_ = zero\n\t_ = domain.Actor{}\n", retType))
	if uc.UsesActor {
		sb.WriteString("\tactor := domain.ActorFromContext(ctx)\n\t_ = actor\n")
	}
	if isOutbox && uc.EmitsEvents {
		sb.WriteString("\ttype outboxMsg struct {\n\t\teventType string\n\t\tpayload any\n\t}\n\tvar outboxQueue []outboxMsg\n")
	}

	declaredVars := make(map[string]bool)
	for _, p := range uc.Params {
		declaredVars[p.Name] = true
	}
	stepCounter := 0
	emitGoSteps(&sb, uc.Steps, declaredVars, &stepCounter, isOutbox && uc.EmitsEvents, "\t")

	sb.WriteString("}\n")
	return sb.String()
}

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

func emitGoSteps(sb *strings.Builder, steps []ir.StepDecl, declaredVars map[string]bool, stepCounter *int, useOutbox bool, indent string) {
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
				sb.WriteString(fmt.Sprintf("%s%s %s %s\n%s_ = %s\n", indent, s.VarName, assignOp, emitGoExpr(*s.Expr, false), indent, s.VarName))
			}

		case ir.StepCreate:
			declaredVars[s.VarName] = true
			sb.WriteString(fmt.Sprintf("%s%s, err := uc.%sRepo.Create(ctx, domain.%s{\n", indent, s.VarName, s.EntityName, s.EntityName))
			emitGoFieldInits(sb, s.Fields, indent+"\t")
			sb.WriteString(fmt.Sprintf("%s})\n", indent))
			sb.WriteString(fmt.Sprintf("%sif err != nil {\n%s\treturn zero, err\n%s}\n%s_ = %s\n", indent, indent, indent, indent, s.VarName))

		case ir.StepQuery:
			declaredVars[s.VarName] = true
			mName := "FindBy" + arch.ExportIdent(s.WhereField)
			wExpr := emitGoExpr(*s.WhereValue, false)
			sb.WriteString(fmt.Sprintf("%s%sPtr, err := uc.%sRepo.%s(ctx, %s)\n", indent, s.VarName, s.EntityName, mName, wExpr))
			sb.WriteString(fmt.Sprintf("%sif err != nil {\n%s\treturn zero, err\n%s}\n", indent, indent, indent))
			if s.ElseError != "" {
				sb.WriteString(fmt.Sprintf("%sif %sPtr == nil {\n%s\treturn zero, domain.Err%s\n%s}\n", indent, s.VarName, indent, s.ElseError, indent))
			}
			sb.WriteString(fmt.Sprintf("%s%s := *%sPtr\n%s_ = %s\n", indent, s.VarName, s.VarName, indent, s.VarName))

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
				emitGoSteps(sb, s.ThenSteps, cloneBoolMap(declaredVars), stepCounter, useOutbox, indent+"\t")
				if len(s.ElseSteps) > 0 {
					sb.WriteString(fmt.Sprintf("%s} else {\n", indent))
					emitGoSteps(sb, s.ElseSteps, cloneBoolMap(declaredVars), stepCounter, useOutbox, indent+"\t")
				}
				sb.WriteString(fmt.Sprintf("%s}\n", indent))
			}

		case ir.StepMatch:
			if s.Expr != nil {
				enumType := s.Expr.InferredType
				sb.WriteString(fmt.Sprintf("%sswitch %s {\n", indent, emitGoExpr(*s.Expr, false)))
				for _, arm := range s.MatchArms {
					sb.WriteString(fmt.Sprintf("%scase domain.%s%s:\n", indent, enumType, arch.ExportIdent(arm.Variant)))
					emitGoSteps(sb, arm.Steps, cloneBoolMap(declaredVars), stepCounter, useOutbox, indent+"\t")
				}
				sb.WriteString(fmt.Sprintf("%s}\n", indent))
			}

		case ir.StepFor:
			if s.Expr != nil {
				sb.WriteString(fmt.Sprintf("%sfor _, %s := range %s {\n%s\t_ = %s\n",
					indent, s.IterVar, emitGoExpr(*s.Expr, false), indent, s.IterVar))
				loopVars := cloneBoolMap(declaredVars)
				loopVars[s.IterVar] = true
				emitGoSteps(sb, s.ThenSteps, loopVars, stepCounter, useOutbox, indent+"\t")
				sb.WriteString(fmt.Sprintf("%s}\n", indent))
			}

		case ir.StepEmit:
			if useOutbox {
				sb.WriteString(fmt.Sprintf("%soutboxQueue = append(outboxQueue, outboxMsg{\n%s\teventType: %q,\n%s\tpayload: domain.%s{\n",
					indent, indent, s.EventName, indent, s.EventName))
				emitGoFieldInits(sb, s.Fields, indent+"\t\t")
				sb.WriteString(fmt.Sprintf("%s\t},\n%s})\n", indent, indent))
			} else {
				sb.WriteString(fmt.Sprintf("%sif uc.Events != nil {\n%s\t_ = uc.Events.Publish(ctx, %q, domain.%s{\n", indent, indent, s.EventName, s.EventName))
				emitGoFieldInits(sb, s.Fields, indent+"\t\t")
				sb.WriteString(fmt.Sprintf("%s\t})\n%s}\n", indent, indent))
			}

		case ir.StepSlot:
			var callArgs []string
			callArgs = append(callArgs, "ctx")
			for _, arg := range s.SlotArgs {
				callArgs = append(callArgs, emitGoExpr(arg, false))
			}
			callExpr := fmt.Sprintf("uc.%sSlot.%s(%s)", s.SlotName, s.SlotMethod, strings.Join(callArgs, ", "))
			if s.VarName != "" {
				declaredVars[s.VarName] = true
				sb.WriteString(fmt.Sprintf("%s%s, err := %s\n%s_ = %s\n", indent, s.VarName, callExpr, indent, s.VarName))
			} else {
				sb.WriteString(fmt.Sprintf("%serr := %s\n", indent, callExpr))
			}
			if s.ElseError != "" {
				sb.WriteString(fmt.Sprintf("%sif err != nil {\n%s\treturn zero, domain.Err%s\n%s}\n", indent, indent, s.ElseError, indent))
			} else {
				sb.WriteString(fmt.Sprintf("%sif err != nil {\n%s\treturn zero, err\n%s}\n", indent, indent, indent))
			}

		case ir.StepReturn:
			if useOutbox {
				sb.WriteString(fmt.Sprintf("%sif uc.Events != nil {\n%s\tfor _, msg := range outboxQueue {\n%s\t\t_ = uc.Events.Publish(ctx, msg.eventType, msg.payload)\n%s\t}\n%s}\n",
					indent, indent, indent, indent, indent))
			}
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
