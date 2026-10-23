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
}

export interface TransactionManager {
  withinTransaction<T>(fn: () => Promise<T>): Promise<T>;
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

	sb.WriteString(`import * as domain from "../entities/domain.ts";
import * as ports from "./ports.ts";

`)

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
		sb.WriteString(fmt.Sprintf("  private readonly idemStore = new Map<string, %s>();\n", retTS))
	}
	if isCache {
		sb.WriteString(fmt.Sprintf("  private readonly cacheStore = new Map<string, { val: %s; expiresAt: number }>();\n", retTS))
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
	sb.WriteString("  }\n\n")

	var execParams []string
	var callArgs []string
	execParams = append(execParams, "actor: domain.Actor")
	callArgs = append(callArgs, "actor")
	for _, p := range uc.Params {
		execParams = append(execParams, fmt.Sprintf("%s: %s", p.Name, tsType(p.Type, false)))
		callArgs = append(callArgs, p.Name)
	}

	// Tactical traits compose as decorator methods around executePipeline:
	// execute -> withIdempotency -> withCache -> withRetry -> withTransaction -> executePipeline.
	paramsSig := strings.Join(execParams, ", ")
	argsSig := strings.Join(callArgs, ", ")

	outermost := "this.executePipeline"
	if isTx {
		outermost = "this.withTransaction"
	}
	if isRetry {
		outermost = "this.withRetry"
	}
	if isCache {
		outermost = "this.withCache"
	}
	if isIdem {
		outermost = "this.withIdempotency"
	}
	sb.WriteString(fmt.Sprintf("  async execute(%s): Promise<%s> {\n    return %s(%s);\n  }\n\n", paramsSig, retTS, outermost, argsSig))

	innerCall := "this.executePipeline"
	if isTx {
		sb.WriteString("  // withTransaction applies the @transactional tactical trait decorator.\n")
		sb.WriteString(fmt.Sprintf("  private async withTransaction(%s): Promise<%s> {\n", paramsSig, retTS))
		sb.WriteString("    if (this.txManager) {\n")
		sb.WriteString(fmt.Sprintf("      return this.txManager.withinTransaction(() => %s(%s));\n", innerCall, argsSig))
		sb.WriteString("    }\n")
		sb.WriteString(fmt.Sprintf("    return %s(%s);\n  }\n\n", innerCall, argsSig))
		innerCall = "this.withTransaction"
	}

	if isRetry {
		attempts := "1"
		if a, ok := retryTrait.Args["attempts"]; ok && a != "" {
			attempts = a
		}
		sb.WriteString("  // withRetry applies the @retry tactical trait decorator.\n")
		sb.WriteString(fmt.Sprintf("  private async withRetry(%s): Promise<%s> {\n", paramsSig, retTS))
		sb.WriteString("    let lastErr: unknown;\n")
		sb.WriteString(fmt.Sprintf("    for (let attempt = 0; attempt < %s; attempt++) {\n      try {\n        return await %s(%s);\n      } catch (err) {\n        lastErr = err;\n      }\n    }\n    throw lastErr;\n  }\n\n", attempts, innerCall, argsSig))
		innerCall = "this.withRetry"
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
		keyStr := `"default"`
		if len(keyParts) > 0 {
			keyStr = "JSON.stringify([" + strings.Join(keyParts, ", ") + "])"
		}
		sb.WriteString("  // withCache applies the @cache tactical trait decorator.\n")
		sb.WriteString(fmt.Sprintf("  private async withCache(%s): Promise<%s> {\n", paramsSig, retTS))
		sb.WriteString(fmt.Sprintf("    const cacheKey = %s;\n", keyStr))
		sb.WriteString("    const cached = this.cacheStore.get(cacheKey);\n    if (cached && Date.now() < cached.expiresAt) {\n      return cached.val;\n    }\n")
		sb.WriteString(fmt.Sprintf("    const cacheTTLMs = %s * 1000;\n", ttlSecs))
		sb.WriteString(fmt.Sprintf("    const res = await %s(%s);\n", innerCall, argsSig))
		sb.WriteString("    this.cacheStore.set(cacheKey, { val: res, expiresAt: Date.now() + cacheTTLMs });\n    return res;\n  }\n\n")
		innerCall = "this.withCache"
	}

	if isIdem {
		keyExpr := idemTrait.Args["key"]
		if keyExpr == "" && len(uc.Params) > 0 {
			keyExpr = uc.Params[0].Name
		} else if keyExpr == "" {
			keyExpr = `"default"`
		}
		sb.WriteString("  // withIdempotency applies the @idempotent tactical trait decorator.\n")
		sb.WriteString(fmt.Sprintf("  private async withIdempotency(%s): Promise<%s> {\n", paramsSig, retTS))
		sb.WriteString(fmt.Sprintf("    const idemKey = String(%s);\n", keyExpr))
		sb.WriteString("    if (this.idemStore.has(idemKey)) {\n      return this.idemStore.get(idemKey)!;\n    }\n")
		sb.WriteString(fmt.Sprintf("    const res = await %s(%s);\n", innerCall, argsSig))
		sb.WriteString("    this.idemStore.set(idemKey, res);\n    return res;\n  }\n\n")
	}

	sb.WriteString(fmt.Sprintf("  private async executePipeline(%s): Promise<%s> {\n    void actor;\n",
		strings.Join(execParams, ", "), retTS))
	if isOutbox && uc.EmitsEvents {
		sb.WriteString("    const outboxQueue: Array<{ eventType: string; payload: unknown }> = [];\n")
	}

	declaredVars := make(map[string]bool)
	for _, p := range uc.Params {
		declaredVars[p.Name] = true
	}
	stepCounter := 0
	emitTSSteps(&sb, uc.Steps, declaredVars, &stepCounter, isOutbox && uc.EmitsEvents, "    ")
	sb.WriteString(fmt.Sprintf("    return %s;\n  }\n}\n", tsZeroVal(uc.ReturnType, archIR, false)))

	return sb.String()
}

func emitTSSteps(sb *strings.Builder, steps []ir.StepDecl, declaredVars map[string]bool, stepCounter *int, useOutbox bool, indent string) {
	for _, s := range steps {
		*stepCounter++
		idx := *stepCounter
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
			sb.WriteString(fmt.Sprintf("%sif (%s) {\n", indent, updVar))
			for _, fi := range s.Fields {
				sb.WriteString(fmt.Sprintf("%s  %s.%s = %s;\n", indent, updVar, fi.Name, emitTSExpr(fi.Value, false)))
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
				emitTSSteps(sb, s.ThenSteps, emit.CloneBoolMap(declaredVars), stepCounter, useOutbox, indent+"  ")
				if len(s.ElseSteps) > 0 {
					sb.WriteString(fmt.Sprintf("%s} else {\n", indent))
					emitTSSteps(sb, s.ElseSteps, emit.CloneBoolMap(declaredVars), stepCounter, useOutbox, indent+"  ")
				}
				sb.WriteString(fmt.Sprintf("%s}\n", indent))
			}

		case ir.StepMatch:
			if s.Expr != nil {
				sb.WriteString(fmt.Sprintf("%sswitch (%s) {\n", indent, emitTSExpr(*s.Expr, false)))
				for _, arm := range s.MatchArms {
					sb.WriteString(fmt.Sprintf("%s  case %q: {\n", indent, arm.Variant))
					emitTSSteps(sb, arm.Steps, emit.CloneBoolMap(declaredVars), stepCounter, useOutbox, indent+"    ")
					sb.WriteString(fmt.Sprintf("%s    break;\n%s  }\n", indent, indent))
				}
				sb.WriteString(fmt.Sprintf("%s}\n", indent))
			}

		case ir.StepFor:
			if s.Expr != nil {
				sb.WriteString(fmt.Sprintf("%sfor (const %s of %s) {\n", indent, s.IterVar, emitTSExpr(*s.Expr, false)))
				loopVars := emit.CloneBoolMap(declaredVars)
				loopVars[s.IterVar] = true
				emitTSSteps(sb, s.ThenSteps, loopVars, stepCounter, useOutbox, indent+"  ")
				sb.WriteString(fmt.Sprintf("%s}\n", indent))
			}

		case ir.StepEmit:
			if useOutbox {
				sb.WriteString(fmt.Sprintf("%soutboxQueue.push({\n%s  eventType: %q,\n%s  payload: {\n", indent, indent, s.EventName, indent))
				emitTSFieldInits(sb, s.Fields, indent+"    ")
				sb.WriteString(fmt.Sprintf("%s  } satisfies domain.%s,\n%s});\n", indent, s.EventName, indent))
			} else {
				sb.WriteString(fmt.Sprintf("%sif (this.events) {\n%s  await this.events.publish(%q, {\n", indent, indent, s.EventName))
				emitTSFieldInits(sb, s.Fields, indent+"    ")
				sb.WriteString(fmt.Sprintf("%s  } satisfies domain.%s);\n%s}\n", indent, s.EventName, indent))
			}

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
			if useOutbox {
				sb.WriteString(fmt.Sprintf("%sif (this.events) {\n%s  for (const msg of outboxQueue) {\n%s    await this.events.publish(msg.eventType, msg.payload);\n%s  }\n%s}\n",
					indent, indent, indent, indent, indent))
			}
			if s.Expr != nil {
				sb.WriteString(fmt.Sprintf("%sreturn %s;\n", indent, emitTSExpr(*s.Expr, false)))
			}
		}
	}
}
