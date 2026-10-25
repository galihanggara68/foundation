package gohex

import (
	"fmt"
	"strings"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/arch"
)

func (lay goLayout) emitGoPostgresTxManager() string {
	return fmt.Sprintf(`package %s

import (
	"context"
	"database/sql"

	"github.com/galihanggara68/foundation/contrib/go/runtime"
)

type DBTX interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type PostgresTransactionManager struct {
	db *sql.DB
}

func NewPostgresTransactionManager(db *sql.DB) *PostgresTransactionManager {
	return &PostgresTransactionManager{db: db}
}

func (tm *PostgresTransactionManager) WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	if tm == nil || tm.db == nil {
		return fn(ctx)
	}
	tx, err := tm.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	txCtx := runtime.ContextWithTx(ctx, tx)
	if err := fn(txCtx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func connFromContext(ctx context.Context, db *sql.DB) DBTX {
	if tx, ok := runtime.TxFromContext(ctx).(*sql.Tx); ok && tx != nil {
		return tx
	}
	return db
}
`, lay.PostgresPkg)
}

func (lay goLayout) emitGoSinglePostgresRepo(archIR *arch.ArchitectureIR, p arch.RepositoryPortIR) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`package %s

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"time"
	"%s/%s"
)

`, lay.PostgresPkg, archIR.Module, lay.DomainDir))

	ent := archIR.EntityByName(p.EntityName)
	tableName := arch.EntityTableName(ent)

	var colNames []string
	var scanTargets []string
	var fieldValues []string
	var placeholders []string
	pkField, pkExport := arch.EntityPKField(ent)
	for i, f := range ent.Fields {
		colNames = append(colNames, f.Name)
		fExp := arch.ExportIdent(f.Name)
		scanTargets = append(scanTargets, "&out."+fExp)
		fieldValues = append(fieldValues, "entity."+fExp)
		placeholders = append(placeholders, fmt.Sprintf("$%d", i+1))
	}

	repoStruct := "Postgres" + p.EntityName + "Repository"
	sb.WriteString(fmt.Sprintf("type %s struct {\n\tdb *sql.DB\n}\n\n", repoStruct))
	sb.WriteString(fmt.Sprintf("func New%s(db *sql.DB) *%s {\n\t_ = errors.Is\n\t_ = time.Now\n\treturn &%s{db: db}\n}\n\n", repoStruct, repoStruct, repoStruct))

	colsCSV := strings.Join(colNames, ", ")

	for _, m := range p.Methods {
		switch m.Kind {
		case arch.RepoMethodCreate:
			query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) RETURNING %s",
				tableName, colsCSV, strings.Join(placeholders, ", "), colsCSV)
			sb.WriteString(fmt.Sprintf("func (r *%s) Create(ctx context.Context, entity %s.%s) (%s.%s, error) {\n",
				repoStruct, lay.DomainPkg, p.EntityName, lay.DomainPkg, p.EntityName))
			hasUUIDPK := false
			for _, f := range ent.Fields {
				if f.HasTrait("pk") && f.Type == "uuid" {
					hasUUIDPK = true
					break
				}
			}
			if hasUUIDPK {
				sb.WriteString(fmt.Sprintf("\tif entity.%s == \"\" {\n\t\tvar b [16]byte\n\t\t_, _ = rand.Read(b[:])\n\t\tb[6] = (b[6] & 0x0f) | 0x40\n\t\tb[8] = (b[8] & 0x3f) | 0x80\n\t\tentity.%s = fmt.Sprintf(\"%%08x-%%04x-%%04x-%%04x-%%012x\", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])\n\t}\n", pkExport, pkExport))
			}
			if _, vExport, hasV := arch.EntityVersionField(ent); hasV {
				sb.WriteString(fmt.Sprintf("\tif entity.%s == 0 {\n\t\tentity.%s = 1\n\t}\n", vExport, vExport))
			}
			for _, f := range ent.Fields {
				if f.HasTrait("auto_now_add") && f.Type == "timestamp" {
					fExp := arch.ExportIdent(f.Name)
					sb.WriteString(fmt.Sprintf("\tif entity.%s.IsZero() {\n\t\tentity.%s = time.Now().UTC()\n\t}\n", fExp, fExp))
				}
			}
			sb.WriteString(fmt.Sprintf("\tvar out %s.%s\n", lay.DomainPkg, p.EntityName))
			sb.WriteString(fmt.Sprintf("\trow := connFromContext(ctx, r.db).QueryRowContext(ctx, %q, %s)\n",
				query, strings.Join(fieldValues, ", ")))
			sb.WriteString(fmt.Sprintf("\tif err := row.Scan(%s); err != nil {\n\t\treturn out, err\n\t}\n\treturn out, nil\n}\n\n",
				strings.Join(scanTargets, ", ")))

		case arch.RepoMethodUpdate:
			vField, vExport, hasV := arch.EntityVersionField(ent)
			if hasV {
				var setClauses []string
				var args []string
				paramIdx := 1
				for _, col := range colNames {
					if col == vField {
						setClauses = append(setClauses, fmt.Sprintf("%s = %s + 1", col, col))
					} else {
						setClauses = append(setClauses, fmt.Sprintf("%s = $%d", col, paramIdx))
						args = append(args, "entity."+arch.ExportIdent(col))
						paramIdx++
					}
				}
				pkParam := fmt.Sprintf("$%d", paramIdx)
				paramIdx++
				verParam := fmt.Sprintf("$%d", paramIdx)
				args = append(args, "entity."+pkExport, "entity."+vExport)

				query := fmt.Sprintf("UPDATE %s SET %s WHERE %s = %s AND %s = %s RETURNING %s",
					tableName, strings.Join(setClauses, ", "), pkField, pkParam, vField, verParam, vField)
				sb.WriteString(fmt.Sprintf("func (r *%s) Update(ctx context.Context, entity %s.%s) error {\n",
					repoStruct, lay.DomainPkg, p.EntityName))
				sb.WriteString(fmt.Sprintf("\tvar newVer int\n\trow := connFromContext(ctx, r.db).QueryRowContext(ctx, %q, %s)\n",
					query, strings.Join(args, ", ")))
				sb.WriteString(fmt.Sprintf("\tif err := row.Scan(&newVer); err != nil {\n\t\tif errors.Is(err, sql.ErrNoRows) {\n\t\t\treturn %s.ErrConflict\n\t\t}\n\t\treturn err\n\t}\n\treturn nil\n}\n\n", lay.DomainPkg))
			} else {
				var setClauses []string
				for i, col := range colNames {
					setClauses = append(setClauses, fmt.Sprintf("%s = $%d", col, i+1))
				}
				whereParam := fmt.Sprintf("$%d", len(colNames)+1)
				query := fmt.Sprintf("UPDATE %s SET %s WHERE %s = %s",
					tableName, strings.Join(setClauses, ", "), pkField, whereParam)
				args := append([]string{}, fieldValues...)
				args = append(args, "entity."+pkExport)
				sb.WriteString(fmt.Sprintf("func (r *%s) Update(ctx context.Context, entity %s.%s) error {\n",
					repoStruct, lay.DomainPkg, p.EntityName))
				sb.WriteString(fmt.Sprintf("\t_, err := connFromContext(ctx, r.db).ExecContext(ctx, %q, %s)\n\treturn err\n}\n\n",
					query, strings.Join(args, ", ")))
			}

		case arch.RepoMethodDelete:
			query := fmt.Sprintf("DELETE FROM %s WHERE %s = $1", tableName, m.WhereField)
			sb.WriteString(fmt.Sprintf("func (r *%s) %s(ctx context.Context, %s %s) error {\n",
				repoStruct, m.Name, m.WhereField, lay.goType(m.WhereType, false)))
			sb.WriteString(fmt.Sprintf("\t_, err := connFromContext(ctx, r.db).ExecContext(ctx, %q, %s)\n\treturn err\n}\n\n",
				query, m.WhereField))

		case arch.RepoMethodFind:
			query := fmt.Sprintf("SELECT %s FROM %s WHERE %s = $1 LIMIT 1", colsCSV, tableName, m.WhereField)
			sb.WriteString(fmt.Sprintf("func (r *%s) %s(ctx context.Context, %s %s) (*%s.%s, error) {\n",
				repoStruct, m.Name, m.WhereField, lay.goType(m.WhereType, false), lay.DomainPkg, p.EntityName))
			sb.WriteString(fmt.Sprintf("\tvar out %s.%s\n", lay.DomainPkg, p.EntityName))
			sb.WriteString(fmt.Sprintf("\trow := connFromContext(ctx, r.db).QueryRowContext(ctx, %q, %s)\n",
				query, m.WhereField))
			sb.WriteString(fmt.Sprintf("\tif err := row.Scan(%s); err != nil {\n\t\tif errors.Is(err, sql.ErrNoRows) {\n\t\t\treturn nil, nil\n\t\t}\n\t\treturn nil, err\n\t}\n\treturn &out, nil\n}\n\n",
				strings.Join(scanTargets, ", ")))
		}
	}

	return sb.String()
}
