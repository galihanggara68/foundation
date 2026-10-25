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
				sb.WriteString(fmt.Sprintf("  async update(entity: domain.%s): Promise<void> {\n", p.EntityName))
				sb.WriteString(fmt.Sprintf("    const idx = this.entities.findIndex((e) => e.%s === entity.%s);\n", pkField, pkField))
				sb.WriteString("    if (idx >= 0) {\n      this.entities[idx] = { ...entity };\n    }\n  }\n\n")

			case arch.RepoMethodDelete:
				sb.WriteString(fmt.Sprintf("  async %s(%s: %s): Promise<void> {\n",
					toCamelCase(m.Name), m.WhereField, tsType(m.WhereType, false)))
				sb.WriteString(fmt.Sprintf("    this.entities = this.entities.filter((e) => e.%s !== %s);\n  }\n\n",
					m.WhereField, m.WhereField))

			case arch.RepoMethodFind:
				sb.WriteString(fmt.Sprintf("  async %s(%s: %s): Promise<domain.%s | null> {\n",
					toCamelCase(m.Name), m.WhereField, tsType(m.WhereType, false), p.EntityName))
				sb.WriteString(fmt.Sprintf("    const found = this.entities.find((e) => e.%s === %s);\n", m.WhereField, m.WhereField))
				sb.WriteString("    return found ? { ...found } : null;\n  }\n\n")
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

func emitTSPostgresGateways(archIR *arch.ArchitectureIR) string {
	var sb strings.Builder
	sb.WriteString(`import * as domain from "../entities/domain.ts";
import * as ports from "../usecases/ports.ts";

export interface PgQueryable {
  query(text: string, values?: unknown[]): Promise<{ rows: Record<string, unknown>[] }>;
}

export class PostgresTransactionManager implements ports.TransactionManager {
  private readonly db: PgQueryable;

  constructor(db: PgQueryable) {
    this.db = db;
  }

  async withinTransaction<T>(fn: () => Promise<T>): Promise<T> {
    await this.db.query("BEGIN");
    try {
      const res = await fn();
      await this.db.query("COMMIT");
      return res;
    } catch (err) {
      await this.db.query("ROLLBACK");
      throw err;
    }
  }
}

`)

	for _, p := range archIR.Ports {
		ent := archIR.EntityByName(p.EntityName)
		tableName := arch.ToSnakeCase(p.EntityName)
		var colNames []string
		var placeholders []string
		var fieldVals []string
		pkField, _ := arch.EntityPKField(ent)
		for i, f := range ent.Fields {
			colNames = append(colNames, f.Name)
			placeholders = append(placeholders, fmt.Sprintf("$%d", i+1))
			fieldVals = append(fieldVals, "entity."+f.Name)
		}
		colsCSV := strings.Join(colNames, ", ")
		repoClass := "Postgres" + p.EntityName + "Repository"
		sb.WriteString(fmt.Sprintf("export class %s implements ports.%s {\n  private readonly db: PgQueryable;\n\n  constructor(db: PgQueryable) {\n    this.db = db;\n  }\n\n",
			repoClass, p.InterfaceName))

		for _, m := range p.Methods {
			switch m.Kind {
			case arch.RepoMethodCreate:
				query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) RETURNING %s",
					tableName, colsCSV, strings.Join(placeholders, ", "), colsCSV)
				sb.WriteString(fmt.Sprintf("  async create(entity: Partial<domain.%s>): Promise<domain.%s> {\n", p.EntityName, p.EntityName))
				sb.WriteString(fmt.Sprintf("    const res = await this.db.query(%q, [%s]);\n    return res.rows[0] as unknown as domain.%s;\n  }\n\n",
					query, strings.Join(fieldVals, ", "), p.EntityName))

			case arch.RepoMethodUpdate:
				var sets []string
				for i, col := range colNames {
					sets = append(sets, fmt.Sprintf("%s = $%d", col, i+1))
				}
				query := fmt.Sprintf("UPDATE %s SET %s WHERE %s = $%d",
					tableName, strings.Join(sets, ", "), pkField, len(colNames)+1)
				vals := append([]string{}, fieldVals...)
				vals = append(vals, "entity."+pkField)
				sb.WriteString(fmt.Sprintf("  async update(entity: domain.%s): Promise<void> {\n    await this.db.query(%q, [%s]);\n  }\n\n",
					p.EntityName, query, strings.Join(vals, ", ")))

			case arch.RepoMethodDelete:
				query := fmt.Sprintf("DELETE FROM %s WHERE %s = $1", tableName, m.WhereField)
				sb.WriteString(fmt.Sprintf("  async %s(%s: %s): Promise<void> {\n    await this.db.query(%q, [%s]);\n  }\n\n",
					toCamelCase(m.Name), m.WhereField, tsType(m.WhereType, false), query, m.WhereField))

			case arch.RepoMethodFind:
				query := fmt.Sprintf("SELECT %s FROM %s WHERE %s = $1 LIMIT 1", colsCSV, tableName, m.WhereField)
				sb.WriteString(fmt.Sprintf("  async %s(%s: %s): Promise<domain.%s | null> {\n    const res = await this.db.query(%q, [%s]);\n    return (res.rows[0] as unknown as domain.%s) ?? null;\n  }\n\n",
					toCamelCase(m.Name), m.WhereField, tsType(m.WhereType, false), p.EntityName, query, m.WhereField, p.EntityName))
			}
		}
		sb.WriteString("}\n\n")
	}

	return sb.String()
}
