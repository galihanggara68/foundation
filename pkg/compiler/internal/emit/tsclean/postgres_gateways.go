package tsclean

import (
	"fmt"
	"strings"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/arch"
)

func emitTSPostgresGateways(archIR *arch.ArchitectureIR) string {
	var sb strings.Builder
	sb.WriteString(`import * as domain from "../entities/domain.ts";
import * as ports from "../usecases/ports.ts";

export interface PgQueryable {
  query(text: string, values?: unknown[]): Promise<{ rows: Record<string, unknown>[]; rowCount?: number }>;
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
		tableName := arch.EntityTableName(ent)
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
				vField, _, hasV := arch.EntityVersionField(ent)
				if hasV {
					var sets []string
					var vals []string
					paramIdx := 1
					for _, col := range colNames {
						if col == vField {
							sets = append(sets, fmt.Sprintf("%s = %s + 1", col, col))
						} else {
							sets = append(sets, fmt.Sprintf("%s = $%d", col, paramIdx))
							vals = append(vals, "entity."+col)
							paramIdx++
						}
					}
					pkParam := fmt.Sprintf("$%d", paramIdx)
					paramIdx++
					verParam := fmt.Sprintf("$%d", paramIdx)
					vals = append(vals, "entity."+pkField, "entity."+vField)
					query := fmt.Sprintf("UPDATE %s SET %s WHERE %s = %s AND %s = %s RETURNING %s",
						tableName, strings.Join(sets, ", "), pkField, pkParam, vField, verParam, vField)
					sb.WriteString(fmt.Sprintf("  async update(entity: domain.%s): Promise<void> {\n    const res = await this.db.query(%q, [%s]);\n    if ((res.rowCount ?? 0) === 0) {\n      throw domain.ErrConflict;\n    }\n  }\n\n",
						p.EntityName, query, strings.Join(vals, ", ")))
				} else {
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
				}

			case arch.RepoMethodDelete:
				var conds []string
				var argVars []string
				fieldCounts := make(map[string]int)
				for _, wp := range m.WherePredicates {
					fieldCounts[wp.Field]++
				}
				if len(m.WherePredicates) > 0 {
					for i, wp := range m.WherePredicates {
						conds = append(conds, fmt.Sprintf("%s %s $%d", wp.Field, arch.SQLComparisonOp(wp.Op), i+1))
						pName := toCamelCase(wp.Field)
						if fieldCounts[wp.Field] > 1 {
							pName += arch.OpSuffix(wp.Op)
						}
						argVars = append(argVars, pName)
					}
				} else {
					conds = append(conds, fmt.Sprintf("%s = $1", m.WhereField))
					argVars = append(argVars, m.WhereField)
				}
				query := fmt.Sprintf("DELETE FROM %s WHERE %s", tableName, strings.Join(conds, " AND "))
				sb.WriteString(fmt.Sprintf("  async %s(%s): Promise<void> {\n    await this.db.query(%q, [%s]);\n  }\n\n",
					toCamelCase(m.Name), formatTSRepoParams(m), query, strings.Join(argVars, ", ")))

			case arch.RepoMethodFind:
				var conds []string
				var argVars []string
				fieldCounts := make(map[string]int)
				for _, wp := range m.WherePredicates {
					fieldCounts[wp.Field]++
				}
				if len(m.WherePredicates) > 0 {
					for i, wp := range m.WherePredicates {
						conds = append(conds, fmt.Sprintf("%s %s $%d", wp.Field, arch.SQLComparisonOp(wp.Op), i+1))
						pName := toCamelCase(wp.Field)
						if fieldCounts[wp.Field] > 1 {
							pName += arch.OpSuffix(wp.Op)
						}
						argVars = append(argVars, pName)
					}
				} else {
					conds = append(conds, fmt.Sprintf("%s = $1", m.WhereField))
					argVars = append(argVars, m.WhereField)
				}
				query := fmt.Sprintf("SELECT %s FROM %s WHERE %s LIMIT 1", colsCSV, tableName, strings.Join(conds, " AND "))
				sb.WriteString(fmt.Sprintf("  async %s(%s): Promise<domain.%s | null> {\n    const res = await this.db.query(%q, [%s]);\n    return (res.rows[0] as unknown as domain.%s) ?? null;\n  }\n\n",
					toCamelCase(m.Name), formatTSRepoParams(m), p.EntityName, query, strings.Join(argVars, ", "), p.EntityName))

			case arch.RepoMethodFindMany:
				whereParams := formatTSRepoParams(m)
				callParams := ""
				if whereParams != "" {
					callParams = whereParams + ", "
				}
				callParams += "cursor: string, limit: number, direction: string"

				orderCol := m.OrderByField
				if orderCol == "" {
					orderCol = pkField
				}
				orderDesc := m.OrderByDesc

				sb.WriteString(fmt.Sprintf("  async %s(%s): Promise<{ items: domain.%s[]; page_info: domain.PageInfo }> {\n",
					toCamelCase(m.Name), callParams, p.EntityName))
				sb.WriteString(fmt.Sprintf("    const curVal = decodeCursor(cursor, %q);\n", orderCol))
				sb.WriteString("    const conds: string[] = [];\n    const args: unknown[] = [];\n")

				fieldCounts := make(map[string]int)
				for _, wp := range m.WherePredicates {
					fieldCounts[wp.Field]++
				}
				for _, wp := range m.WherePredicates {
					pName := toCamelCase(wp.Field)
					if fieldCounts[wp.Field] > 1 {
						pName += arch.OpSuffix(wp.Op)
					}
					sb.WriteString(fmt.Sprintf("    args.push(%s);\n    conds.push(`${%q}${args.length}`);\n", pName, fmt.Sprintf("%s %s $", wp.Field, arch.SQLComparisonOp(wp.Op))))
				}

				sb.WriteString("    let orderClause = \"\";\n")
				sb.WriteString("    if (direction === \"prev\" && curVal !== undefined && cursor !== \"\") {\n")
				if orderDesc {
					sb.WriteString(fmt.Sprintf("      args.push(curVal);\n      conds.push(`${%q}${args.length}`);\n      orderClause = %q;\n", fmt.Sprintf("%s > $", orderCol), fmt.Sprintf("ORDER BY %s ASC", orderCol)))
				} else {
					sb.WriteString(fmt.Sprintf("      args.push(curVal);\n      conds.push(`${%q}${args.length}`);\n      orderClause = %q;\n", fmt.Sprintf("%s < $", orderCol), fmt.Sprintf("ORDER BY %s DESC", orderCol)))
				}
				sb.WriteString("    } else if (curVal !== undefined && cursor !== \"\") {\n")
				if orderDesc {
					sb.WriteString(fmt.Sprintf("      args.push(curVal);\n      conds.push(`${%q}${args.length}`);\n      orderClause = %q;\n", fmt.Sprintf("%s < $", orderCol), fmt.Sprintf("ORDER BY %s DESC", orderCol)))
				} else {
					sb.WriteString(fmt.Sprintf("      args.push(curVal);\n      conds.push(`${%q}${args.length}`);\n      orderClause = %q;\n", fmt.Sprintf("%s > $", orderCol), fmt.Sprintf("ORDER BY %s ASC", orderCol)))
				}
				sb.WriteString("    } else {\n")
				if orderDesc {
					sb.WriteString(fmt.Sprintf("      orderClause = %q;\n", fmt.Sprintf("ORDER BY %s DESC", orderCol)))
				} else {
					sb.WriteString(fmt.Sprintf("      orderClause = %q;\n", fmt.Sprintf("ORDER BY %s ASC", orderCol)))
				}
				sb.WriteString("    }\n")

				sb.WriteString("    const whereSQL = conds.length > 0 ? ` WHERE ${conds.join(\" AND \")}` : \"\";\n")
				sb.WriteString("    args.push(limit + 1);\n")
				sb.WriteString(fmt.Sprintf("    const query = `SELECT %s FROM %s${whereSQL} ${orderClause} LIMIT $${args.length}`;\n", colsCSV, tableName))
				sb.WriteString("    const res = await this.db.query(query, args);\n")
				sb.WriteString(fmt.Sprintf("    let items = res.rows as unknown as domain.%s[];\n", p.EntityName))
				sb.WriteString("    let hasNext = false;\n    let hasPrev = false;\n")
				sb.WriteString("    if (direction === \"prev\" && curVal !== undefined && cursor !== \"\") {\n")
				sb.WriteString("      hasPrev = items.length > limit;\n      if (hasPrev) items = items.slice(0, limit);\n      hasNext = true;\n      items.reverse();\n")
				sb.WriteString("    } else {\n")
				sb.WriteString("      hasNext = items.length > limit;\n      if (hasNext) items = items.slice(0, limit);\n      if (cursor !== \"\") hasPrev = true;\n    }\n")
				sb.WriteString("    let nextCursor = \"\";\n    let prevCursor = \"\";\n")
				sb.WriteString("    if (items.length > 0) {\n")
				sb.WriteString(fmt.Sprintf("      if (hasNext) nextCursor = encodeCursor(items[items.length - 1].%s, %q);\n", toCamelCase(orderCol), orderCol))
				sb.WriteString(fmt.Sprintf("      if (hasPrev) prevCursor = encodeCursor(items[0].%s, %q);\n", toCamelCase(orderCol), orderCol))
				sb.WriteString("    }\n")
				sb.WriteString("    return { items, page_info: { next_cursor: nextCursor, prev_cursor: prevCursor, has_next: hasNext, has_prev: hasPrev } };\n  }\n\n")
			}
		}
		sb.WriteString("}\n\n")
	}

	return sb.String()
}
