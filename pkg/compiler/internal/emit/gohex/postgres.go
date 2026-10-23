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
)

type txCtxKey struct{}

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
	txCtx := context.WithValue(ctx, txCtxKey{}, tx)
	if err := fn(txCtx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func connFromContext(ctx context.Context, db *sql.DB) DBTX {
	if tx, ok := ctx.Value(txCtxKey{}).(*sql.Tx); ok && tx != nil {
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
	"database/sql"
	"errors"
	"%s/%s"
)

`, lay.PostgresPkg, archIR.Module, lay.DomainDir))

	ent := archIR.EntityByName(p.EntityName)
	tableName := arch.ToSnakeCase(p.EntityName)

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
	sb.WriteString(fmt.Sprintf("func New%s(db *sql.DB) *%s {\n\t_ = errors.Is\n\treturn &%s{db: db}\n}\n\n", repoStruct, repoStruct, repoStruct))

	colsCSV := strings.Join(colNames, ", ")

	for _, m := range p.Methods {
		switch m.Kind {
		case arch.RepoMethodCreate:
			query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) RETURNING %s",
				tableName, colsCSV, strings.Join(placeholders, ", "), colsCSV)
			sb.WriteString(fmt.Sprintf("func (r *%s) Create(ctx context.Context, entity %s.%s) (%s.%s, error) {\n",
				repoStruct, lay.DomainPkg, p.EntityName, lay.DomainPkg, p.EntityName))
			sb.WriteString(fmt.Sprintf("\tvar out %s.%s\n", lay.DomainPkg, p.EntityName))
			sb.WriteString(fmt.Sprintf("\trow := connFromContext(ctx, r.db).QueryRowContext(ctx, %q, %s)\n",
				query, strings.Join(fieldValues, ", ")))
			sb.WriteString(fmt.Sprintf("\tif err := row.Scan(%s); err != nil {\n\t\treturn out, err\n\t}\n\treturn out, nil\n}\n\n",
				strings.Join(scanTargets, ", ")))

		case arch.RepoMethodUpdate:
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
