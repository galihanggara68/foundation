package arch

import (
	"fmt"
	"strings"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
)

// forEachCompositeTrait runs the callback for each composite trait matching traitName.
func forEachCompositeTrait(ent ir.EntityDecl, traitName string, fn func(name string, cols []string)) {
	for _, tr := range ent.Traits {
		if tr.Name != traitName {
			continue
		}
		cols := extractCompositeFields(tr)
		if len(cols) == 0 {
			continue
		}
		name := extractTraitNameArg(tr)
		fn(name, cols)
	}
}

// CompositeUniqueConstraintsSQL returns table constraint definitions for composite @unique traits.
func CompositeUniqueConstraintsSQL(ent ir.EntityDecl, tableName string) []string {
	var constraints []string
	forEachCompositeTrait(ent, "unique", func(name string, cols []string) {
		if name == "" {
			name = FormatCompositeUniqueName(tableName, cols)
		}
		constraints = append(constraints, fmt.Sprintf("CONSTRAINT %s UNIQUE (%s)", name, strings.Join(cols, ", ")))
	})
	return constraints
}

// CompositeIndexesSQL returns CREATE INDEX statements for composite @index traits.
func CompositeIndexesSQL(ent ir.EntityDecl, tableName string) []string {
	var indexes []string
	forEachCompositeTrait(ent, "index", func(name string, cols []string) {
		if name == "" {
			name = FormatCompositeIndexName(tableName, cols)
		}
		indexes = append(indexes, fmt.Sprintf("CREATE INDEX %s ON %s (%s);\n", name, tableName, strings.Join(cols, ", ")))
	})
	return indexes
}

// FormatCompositeUniqueName returns the canonical constraint name for composite uniqueness.
func FormatCompositeUniqueName(tableName string, cols []string) string {
	return fmt.Sprintf("uq_%s_%s", tableName, strings.Join(cols, "_"))
}

// FormatCompositeIndexName returns the canonical index name for a composite index.
func FormatCompositeIndexName(tableName string, cols []string) string {
	return fmt.Sprintf("idx_%s_%s", tableName, strings.Join(cols, "_"))
}

// BuildTableSchemas constructs relational TableSchema models for all entities in the blueprint.
func BuildTableSchemas(entities []ir.EntityDecl, enumSet map[string]bool, tableMap map[string]string) []TableSchema {
	var tables []TableSchema
	for _, ent := range entities {
		tableName := EntityTableName(ent)
		var cols []ColumnSchema
		var pks []string
		var fks []ForeignKey
		var idxs []IndexSchema
		var uqs []UniqueConstraint

		for _, f := range ent.Fields {
			isPK := f.HasTrait("pk")
			if isPK {
				pks = append(pks, f.Name)
			}
			if f.HasTrait("unique") {
				uqs = append(uqs, UniqueConstraint{
					Name:    fmt.Sprintf("uq_%s_%s", tableName, f.Name),
					Table:   tableName,
					Columns: []string{f.Name},
				})
			}
			if f.HasTrait("index") {
				idxs = append(idxs, IndexSchema{
					Name:     fmt.Sprintf("idx_%s_%s", tableName, f.Name),
					Table:    tableName,
					Columns:  []string{f.Name},
					IsUnique: false,
				})
			}
			if fkRef, ok := getFieldTraitArg(f, "fk"); ok {
				fks = append(fks, parseForeignKey(f.Name, fkRef, tableMap))
			}
			defVal := ""
			if rawDef, ok := getFieldTraitArg(f, "default"); ok {
				defVal = formatSQLDefault(rawDef, f.Type, enumSet)
			}
			cols = append(cols, ColumnSchema{
				Name:       f.Name,
				Type:       sqlColumnType(f, enumSet),
				IsPK:       isPK,
				IsNullable: f.Optional,
				DefaultVal: defVal,
			})
		}

		forEachCompositeTrait(ent, "unique", func(name string, cols []string) {
			if name == "" {
				name = FormatCompositeUniqueName(tableName, cols)
			}
			uqs = append(uqs, UniqueConstraint{
				Name:    name,
				Table:   tableName,
				Columns: cols,
			})
		})

		forEachCompositeTrait(ent, "index", func(name string, cols []string) {
			if name == "" {
				name = FormatCompositeIndexName(tableName, cols)
			}
			idxs = append(idxs, IndexSchema{
				Name:     name,
				Table:    tableName,
				Columns:  cols,
				IsUnique: false,
			})
		})

		tables = append(tables, TableSchema{
			Name:        tableName,
			Columns:     cols,
			PrimaryKeys: pks,
			ForeignKeys: fks,
			Indexes:     idxs,
			Uniques:     uqs,
		})
	}
	return tables
}

func parseForeignKey(colName, fkRef string, tableMap map[string]string) ForeignKey {
	parts := strings.Split(fkRef, ".")
	targetTable := parts[0]
	targetCol := "id"
	if len(parts) == 2 {
		targetCol = parts[1]
	}
	if mapped, ok := tableMap[parts[0]]; ok {
		targetTable = mapped
	}
	return ForeignKey{
		Column:        colName,
		ForeignTable:  targetTable,
		ForeignColumn: targetCol,
	}
}

// buildTableLinesAndIndexes creates column definitions, table constraints, and index statements.
func buildTableLinesAndIndexes(ent ir.EntityDecl, tableName string, enumSet map[string]bool, tableMap map[string]string) ([]string, []string) {
	var lines []string
	var indexes []string

	for _, f := range ent.Fields {
		colDef := columnDefinitionSQL(f, enumSet, tableMap)
		lines = append(lines, fmt.Sprintf("%s %s", f.Name, colDef))
		if f.HasTrait("index") {
			indexes = append(indexes, fmt.Sprintf("CREATE INDEX idx_%s_%s ON %s (%s);\n", tableName, f.Name, tableName, f.Name))
		}
	}

	lines = append(lines, CompositeUniqueConstraintsSQL(ent, tableName)...)
	indexes = append(indexes, CompositeIndexesSQL(ent, tableName)...)

	return lines, indexes
}

// extractCompositeFields retrieves column names from the trait fields argument.
func extractCompositeFields(tr ir.TraitDecl) []string {
	fields, ok := tr.GetListArg("fields")
	if !ok || len(fields) == 0 {
		fields, ok = tr.GetListArg("_")
	}
	if !ok || len(fields) == 0 {
		return nil
	}
	var clean []string
	for _, f := range fields {
		trimmed := strings.Trim(f, `"' `)
		if trimmed != "" {
			clean = append(clean, trimmed)
		}
	}
	return clean
}

// extractTraitNameArg retrieves the optional name argument from a trait.
func extractTraitNameArg(tr ir.TraitDecl) string {
	if val, ok := tr.GetStringArg("name"); ok {
		trimmed := strings.Trim(val, `"' `)
		if trimmed != "" {
			return trimmed
		}
	}
	if tr.Args != nil {
		if val, ok := tr.Args["name"]; ok {
			trimmed := strings.Trim(val, `"' `)
			if trimmed != "" {
				return trimmed
			}
		}
	}
	return ""
}
