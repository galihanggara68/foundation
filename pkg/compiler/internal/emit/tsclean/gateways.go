package tsclean

import (
	"fmt"
	"strings"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/arch"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
)

func emitTSSlotStub(archIR *arch.ArchitectureIR, sl ir.SlotDecl) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`import type { %sSlot } from "../usecases/ports.ts";
import * as domain from "../entities/domain.ts";

void domain;

// %sStub is a create-if-missing starter implementation of %sSlot.
// Developers may freely edit this file; Foundation never overwrites existing Slot Stubs.
export class %sStub implements %sSlot {
`, sl.Name, sl.Name, sl.Name, sl.Name, sl.Name))

	for _, m := range sl.Methods {
		var params []string
		for _, p := range m.Params {
			params = append(params, fmt.Sprintf("_%s: %s", p.Name, tsType(p.Type, false)))
		}
		retTS := tsType(m.ReturnType, false)
		sb.WriteString(fmt.Sprintf("  async %s(%s): Promise<%s> {\n", toCamelCase(m.Name), strings.Join(params, ", "), retTS))
		if m.ReturnType != "" {
			sb.WriteString(fmt.Sprintf("    return %s;\n", tsZeroVal(m.ReturnType, archIR, false)))
		}
		sb.WriteString("  }\n")
	}

	sb.WriteString(fmt.Sprintf(`}

export function create%sSlot(): %sSlot {
  return new %sStub();
}
`, sl.Name, sl.Name, sl.Name))
	return sb.String()
}

func emitTSMemoryGateways(archIR *arch.ArchitectureIR) string {
	var sb strings.Builder
	sb.WriteString(`import * as domain from "../entities/domain.ts";
import * as ports from "../usecases/ports.ts";

export interface Snapshotter {
  snapshot(): () => void;
}

export class InMemoryEventPublisher implements ports.EventPublisher {
  private events: Array<{ eventType: string; payload: unknown }> = [];
  private handlers = new Map<string, Array<(payload: unknown) => Promise<void>>>();

  subscribe(eventType: string, handler: (payload: unknown) => Promise<void>): void {
    const list = this.handlers.get(eventType) ?? [];
    list.push(handler);
    this.handlers.set(eventType, list);
  }

  async publish(eventType: string, payload: unknown): Promise<void> {
    this.events.push({ eventType, payload });
    const list = this.handlers.get(eventType) ?? [];
    for (const h of list) {
      try {
        await h(payload);
      } catch (err) {
        // Handlers run independently; errors do not disrupt publisher or other handlers
      }
    }
  }

  byType<T = unknown>(eventType: string): T[] {
    return this.events.filter((e) => e.eventType === eventType).map((e) => e.payload as T);
  }
}

export class InMemoryTransactionManager implements ports.TransactionManager {
  private readonly repos: Snapshotter[];

  constructor(...repos: Snapshotter[]) {
    this.repos = repos;
  }

  async withinTransaction<T>(fn: () => Promise<T>): Promise<T> {
    const restores = this.repos.map((r) => r.snapshot());
    try {
      return await fn();
    } catch (err) {
      for (const restore of restores) {
        restore();
      }
      throw err;
    }
  }
}

function encodeCursor(val: unknown, field: string): string {
  return Buffer.from(JSON.stringify({ [field]: val })).toString("base64url");
}

function decodeCursor(cursor: string, field: string): unknown {
  if (!cursor) return undefined;
  try {
    return JSON.parse(Buffer.from(cursor, "base64url").toString("utf-8"))[field];
  } catch {
    return undefined;
  }
}

`)

	for _, p := range archIR.Ports {
		ent := archIR.EntityByName(p.EntityName)
		repoClass := "InMemory" + p.EntityName + "Repository"
		sb.WriteString(fmt.Sprintf("export class %s implements ports.%s, Snapshotter {\n", repoClass, p.InterfaceName))
		sb.WriteString(fmt.Sprintf("  private entities: domain.%s[] = [];\n  private seq = 0;\n\n", p.EntityName))
		sb.WriteString(fmt.Sprintf("  seed(entity: domain.%s): void {\n    this.entities.push({ ...entity });\n  }\n\n", p.EntityName))
		sb.WriteString("  snapshot(): () => void {\n    const saved = this.entities.map((e) => ({ ...e }));\n    const savedSeq = this.seq;\n    return () => {\n      this.entities = saved;\n      this.seq = savedSeq;\n    };\n  }\n\n")

		for _, m := range p.Methods {
			switch m.Kind {
			case arch.RepoMethodCreate:
				sb.WriteString(fmt.Sprintf("  async create(input: Partial<domain.%s>): Promise<domain.%s> {\n    this.seq++;\n", p.EntityName, p.EntityName))
				sb.WriteString(fmt.Sprintf("    const entity: domain.%s = {\n", p.EntityName))
				for _, f := range ent.Fields {
					if f.Name == "id" && f.Type == "uuid" {
						sb.WriteString(`      id: input.id ?? "00000000-0000-0000-0000-" + String(this.seq).padStart(12, "0"),` + "\n")
					} else if f.HasTrait("version") {
						sb.WriteString(fmt.Sprintf("      %s: input.%s ?? 1,\n", f.Name, f.Name))
					} else if strings.HasPrefix(f.DefaultVal, ".") {
						sb.WriteString(fmt.Sprintf("      %s: input.%s ?? %q,\n", f.Name, f.Name, strings.TrimPrefix(f.DefaultVal, ".")))
					} else if f.DefaultVal != "" {
						sb.WriteString(fmt.Sprintf("      %s: input.%s ?? %s,\n", f.Name, f.Name, f.DefaultVal))
					} else {
						sb.WriteString(fmt.Sprintf("      %s: input.%s ?? %s,\n", f.Name, f.Name, tsZeroVal(f.Type, archIR, false)))
					}
				}
				sb.WriteString("    };\n    this.entities.push({ ...entity });\n    return { ...entity };\n  }\n\n")

			case arch.RepoMethodUpdate:
				pkField, _ := arch.EntityPKField(ent)
				vField, _, hasV := arch.EntityVersionField(ent)
				sb.WriteString(fmt.Sprintf("  async update(entity: domain.%s): Promise<void> {\n", p.EntityName))
				sb.WriteString(fmt.Sprintf("    const idx = this.entities.findIndex((e) => e.%s === entity.%s);\n", pkField, pkField))
				if hasV {
					sb.WriteString(fmt.Sprintf("    if (idx < 0 || this.entities[idx].%s !== entity.%s) {\n      throw domain.ErrConflict;\n    }\n", vField, vField))
					sb.WriteString(fmt.Sprintf("    this.entities[idx] = { ...entity, %s: entity.%s + 1 };\n  }\n\n", vField, vField))
				} else {
					sb.WriteString("    if (idx >= 0) {\n      this.entities[idx] = { ...entity };\n    }\n  }\n\n")
				}

			case arch.RepoMethodDelete:
				sb.WriteString(fmt.Sprintf("  async %s(%s): Promise<void> {\n",
					toCamelCase(m.Name), formatTSRepoParams(m)))
				fieldCounts := make(map[string]int)
				for _, wp := range m.WherePredicates {
					fieldCounts[wp.Field]++
				}
				var checks []string
				if len(m.WherePredicates) > 0 {
					for _, wp := range m.WherePredicates {
						pName := toCamelCase(wp.Field)
						if fieldCounts[wp.Field] > 1 {
							pName += arch.OpSuffix(wp.Op)
						}
						checks = append(checks, formatTSInMemoryCheck("e."+toCamelCase(wp.Field), wp.Op, pName, wp.Type))
					}
				} else {
					checks = append(checks, fmt.Sprintf("e.%s === %s", m.WhereField, m.WhereField))
				}
				sb.WriteString(fmt.Sprintf("    this.entities = this.entities.filter((e) => !(%s));\n  }\n\n",
					strings.Join(checks, " && ")))

			case arch.RepoMethodFind:
				sb.WriteString(fmt.Sprintf("  async %s(%s): Promise<domain.%s | null> {\n",
					toCamelCase(m.Name), formatTSRepoParams(m), p.EntityName))
				fieldCounts := make(map[string]int)
				for _, wp := range m.WherePredicates {
					fieldCounts[wp.Field]++
				}
				var checks []string
				if len(m.WherePredicates) > 0 {
					for _, wp := range m.WherePredicates {
						pName := toCamelCase(wp.Field)
						if fieldCounts[wp.Field] > 1 {
							pName += arch.OpSuffix(wp.Op)
						}
						checks = append(checks, formatTSInMemoryCheck("e."+toCamelCase(wp.Field), wp.Op, pName, wp.Type))
					}
				} else {
					checks = append(checks, fmt.Sprintf("e.%s === %s", m.WhereField, m.WhereField))
				}
				sb.WriteString(fmt.Sprintf("    const found = this.entities.find((e) => %s);\n", strings.Join(checks, " && ")))
				sb.WriteString("    return found ? { ...found } : null;\n  }\n\n")

			case arch.RepoMethodFindMany:
				whereParams := formatTSRepoParams(m)
				callParams := ""
				if whereParams != "" {
					callParams = whereParams + ", "
				}
				callParams += "cursor: string, limit: number, direction: string"

				pkField, _ := arch.EntityPKField(ent)
				orderCol := m.OrderByField
				if orderCol == "" {
					orderCol = pkField
				}
				orderDesc := m.OrderByDesc

				sb.WriteString(fmt.Sprintf("  async %s(%s): Promise<{ items: domain.%s[]; page_info: domain.PageInfo }> {\n",
					toCamelCase(m.Name), callParams, p.EntityName))
				sb.WriteString("    let filtered = [...this.entities];\n")
				if len(m.WherePredicates) > 0 {
					var checks []string
					fieldCounts := make(map[string]int)
					for _, wp := range m.WherePredicates {
						fieldCounts[wp.Field]++
					}
					for _, wp := range m.WherePredicates {
						pName := toCamelCase(wp.Field)
						if fieldCounts[wp.Field] > 1 {
							pName += arch.OpSuffix(wp.Op)
						}
						checks = append(checks, formatTSInMemoryCheck("e."+toCamelCase(wp.Field), wp.Op, pName, wp.Type))
					}
					sb.WriteString(fmt.Sprintf("    filtered = filtered.filter((e) => %s);\n", strings.Join(checks, " && ")))
				}
				if m.OrderByType == "int" {
					if orderDesc {
						sb.WriteString(fmt.Sprintf("    filtered.sort((a, b) => b.%s - a.%s);\n", toCamelCase(orderCol), toCamelCase(orderCol)))
					} else {
						sb.WriteString(fmt.Sprintf("    filtered.sort((a, b) => a.%s - b.%s);\n", toCamelCase(orderCol), toCamelCase(orderCol)))
					}
				} else {
					if orderDesc {
						sb.WriteString(fmt.Sprintf("    filtered.sort((a, b) => String(b.%s).localeCompare(String(a.%s)));\n", toCamelCase(orderCol), toCamelCase(orderCol)))
					} else {
						sb.WriteString(fmt.Sprintf("    filtered.sort((a, b) => String(a.%s).localeCompare(String(b.%s)));\n", toCamelCase(orderCol), toCamelCase(orderCol)))
					}
				}

				sb.WriteString(fmt.Sprintf("    const curVal = decodeCursor(cursor, %q);\n", orderCol))
				sb.WriteString(fmt.Sprintf("    let seeked: domain.%s[] = [];\n", p.EntityName))
				sb.WriteString("    if (curVal !== undefined && cursor !== \"\") {\n")
				if m.OrderByType == "int" {
					sb.WriteString("      const targetVal = Number(curVal);\n")
					sb.WriteString("      if (direction === \"prev\") {\n")
					sb.WriteString("        for (let i = filtered.length - 1; i >= 0; i--) {\n")
					sb.WriteString(fmt.Sprintf("          const itemVal = Number(filtered[i].%s);\n", toCamelCase(orderCol)))
					if orderDesc {
						sb.WriteString("          if (itemVal > targetVal) seeked.push(filtered[i]);\n")
					} else {
						sb.WriteString("          if (itemVal < targetVal) seeked.push(filtered[i]);\n")
					}
					sb.WriteString("        }\n      } else {\n")
					sb.WriteString("        for (const item of filtered) {\n")
					sb.WriteString(fmt.Sprintf("          const itemVal = Number(item.%s);\n", toCamelCase(orderCol)))
					if orderDesc {
						sb.WriteString("          if (itemVal < targetVal) seeked.push(item);\n")
					} else {
						sb.WriteString("          if (itemVal > targetVal) seeked.push(item);\n")
					}
					sb.WriteString("        }\n      }\n    } else {\n      seeked = filtered;\n    }\n")
				} else {
					sb.WriteString("      const targetVal = String(curVal);\n")
					sb.WriteString("      if (direction === \"prev\") {\n")
					sb.WriteString("        for (let i = filtered.length - 1; i >= 0; i--) {\n")
					sb.WriteString(fmt.Sprintf("          const itemVal = String(filtered[i].%s);\n", toCamelCase(orderCol)))
					if orderDesc {
						sb.WriteString("          if (itemVal > targetVal) seeked.push(filtered[i]);\n")
					} else {
						sb.WriteString("          if (itemVal < targetVal) seeked.push(filtered[i]);\n")
					}
					sb.WriteString("        }\n      } else {\n")
					sb.WriteString("        for (const item of filtered) {\n")
					sb.WriteString(fmt.Sprintf("          const itemVal = String(item.%s);\n", toCamelCase(orderCol)))
					if orderDesc {
						sb.WriteString("          if (itemVal < targetVal) seeked.push(item);\n")
					} else {
						sb.WriteString("          if (itemVal > targetVal) seeked.push(item);\n")
					}
					sb.WriteString("        }\n      }\n    } else {\n      seeked = filtered;\n    }\n")
				}

				sb.WriteString("    let hasNext = false;\n    let hasPrev = false;\n")
				sb.WriteString("    if (curVal !== undefined && cursor !== \"\" && direction === \"prev\") {\n")
				sb.WriteString("      hasPrev = seeked.length > limit;\n      if (hasPrev) seeked = seeked.slice(0, limit);\n      hasNext = true;\n      seeked.reverse();\n")
				sb.WriteString("    } else {\n")
				sb.WriteString("      hasNext = seeked.length > limit;\n      if (hasNext) seeked = seeked.slice(0, limit);\n      if (cursor !== \"\") hasPrev = true;\n    }\n")

				sb.WriteString("    let nextCursor = \"\";\n    let prevCursor = \"\";\n")
				sb.WriteString("    if (seeked.length > 0) {\n")
				sb.WriteString(fmt.Sprintf("      if (hasNext) nextCursor = encodeCursor(seeked[seeked.length - 1].%s, %q);\n", toCamelCase(orderCol), orderCol))
				sb.WriteString(fmt.Sprintf("      if (hasPrev) prevCursor = encodeCursor(seeked[0].%s, %q);\n", toCamelCase(orderCol), orderCol))
				sb.WriteString("    }\n")
				sb.WriteString("    return { items: seeked, page_info: { next_cursor: nextCursor, prev_cursor: prevCursor, has_next: hasNext, has_prev: hasPrev } };\n  }\n\n")
			}
		}
		sb.WriteString("}\n\n")
	}

	for _, sl := range archIR.Slots {
		fakeClass := "Fake" + sl.Name + "Slot"
		sb.WriteString(fmt.Sprintf("export class %s implements ports.%sSlot {\n", fakeClass, sl.Name))
		sb.WriteString(fmt.Sprintf("  private readonly fallback?: ports.%sSlot;\n", sl.Name))
		for _, m := range sl.Methods {
			retTS := tsType(m.ReturnType, false)
			sb.WriteString(fmt.Sprintf("  private rules%s: Array<{ hasArgs: boolean; argsJson: string; ret?: %s; err?: unknown }> = [];\n",
				m.Name, retTS))
		}
		sb.WriteString(fmt.Sprintf("\n  constructor(fallback?: ports.%sSlot) {\n    this.fallback = fallback;\n  }\n\n", sl.Name))

		for _, m := range sl.Methods {
			var paramDecls []string
			var argNames []string
			for _, p := range m.Params {
				paramDecls = append(paramDecls, fmt.Sprintf("%s: %s", p.Name, tsType(p.Type, false)))
				argNames = append(argNames, p.Name)
			}
			retTS := tsType(m.ReturnType, false)

			if m.ReturnType != "" {
				stubWithArgs := append([]string{}, paramDecls...)
				stubWithArgs = append(stubWithArgs, fmt.Sprintf("ret: %s", retTS))
				sb.WriteString(fmt.Sprintf("  stub%sReturn(%s): void {\n    this.rules%s.push({ hasArgs: true, argsJson: JSON.stringify([%s]), ret });\n  }\n\n",
					m.Name, strings.Join(stubWithArgs, ", "), m.Name, strings.Join(argNames, ", ")))
				sb.WriteString(fmt.Sprintf("  stub%sReturnAny(ret: %s): void {\n    this.rules%s.push({ hasArgs: false, argsJson: \"\", ret });\n  }\n\n",
					m.Name, retTS, m.Name))
			}

			if len(paramDecls) > 0 {
				errWithArgs := append([]string{}, paramDecls...)
				errWithArgs = append(errWithArgs, "err: unknown")
				sb.WriteString(fmt.Sprintf("  stub%sError(%s): void {\n    this.rules%s.push({ hasArgs: true, argsJson: JSON.stringify([%s]), err });\n  }\n\n",
					m.Name, strings.Join(errWithArgs, ", "), m.Name, strings.Join(argNames, ", ")))
			}
			sb.WriteString(fmt.Sprintf("  stub%sErrorAny(err: unknown): void {\n    this.rules%s.push({ hasArgs: false, argsJson: \"\", err });\n  }\n\n",
				m.Name, m.Name))

			sb.WriteString(fmt.Sprintf("  async %s(%s): Promise<%s> {\n", toCamelCase(m.Name), strings.Join(paramDecls, ", "), retTS))
			sb.WriteString(fmt.Sprintf("    const callJson = JSON.stringify([%s]);\n", strings.Join(argNames, ", ")))
			sb.WriteString(fmt.Sprintf("    for (let i = this.rules%s.length - 1; i >= 0; i--) {\n      const r = this.rules%s[i];\n      if (!r.hasArgs || r.argsJson === callJson) {\n        if (r.err !== undefined) throw r.err;\n        return r.ret!;\n      }\n    }\n",
				m.Name, m.Name))
			sb.WriteString(fmt.Sprintf("    if (this.fallback) {\n      return this.fallback.%s(%s);\n    }\n",
				toCamelCase(m.Name), strings.Join(argNames, ", ")))
			if m.ReturnType != "" {
				sb.WriteString(fmt.Sprintf("    return %s;\n", tsZeroVal(m.ReturnType, archIR, false)))
			}
			sb.WriteString("  }\n")
		}
		sb.WriteString("}\n\n")
	}

	return sb.String()
}
