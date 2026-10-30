package gohex

import (
	"fmt"
	"strings"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/arch"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
)

func (lay goLayout) emitGoDomainEnums(archIR *arch.ArchitectureIR) string {
	var sb strings.Builder
	sb.WriteString("package " + lay.DomainPkg + "\n\n")
	for _, en := range archIR.DomainEnums {
		sb.WriteString(fmt.Sprintf("type %s string\n\nconst (\n", en.Name))
		for _, v := range en.Variants {
			sb.WriteString(fmt.Sprintf("\t%s%s %s = %q\n", en.Name, arch.ExportIdent(v), en.Name, v))
		}
		sb.WriteString(")\n\n")
	}
	return sb.String()
}

func (lay goLayout) emitGoEntityFile(e ir.EntityDecl) string {
	var sb strings.Builder
	sb.WriteString("package " + lay.DomainPkg + "\n\n")
	hasTime := false
	for _, f := range e.Fields {
		if f.Kind == ir.KindTimestamp {
			hasTime = true
			break
		}
	}
	if hasTime {
		sb.WriteString("import \"time\"\n\n")
	}
	sb.WriteString(lay.emitGoStructDef(e.Name, e.Fields))
	return sb.String()
}

func (lay goLayout) emitGoStructDef(name string, fields []ir.FieldDecl) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("type %s struct {\n", name))
	for _, f := range fields {
		sb.WriteString(fmt.Sprintf("\t%s %s `json:%q`\n", arch.ExportIdent(f.Name), lay.goKindType(f.Kind, f.Type, true), f.Name))
	}
	sb.WriteString("}\n")
	return sb.String()
}

func (lay goLayout) emitGoFieldInits(sb *strings.Builder, fields []ir.FieldInitDecl, indent string) {
	for _, fi := range fields {
		sb.WriteString(fmt.Sprintf("%s%s: %s,\n", indent, arch.ExportIdent(fi.Name), lay.emitGoExpr(fi.Value, false)))
	}
}

func (lay goLayout) emitGoDomainValues(archIR *arch.ArchitectureIR) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`package %s

import "context"

type actorCtxKey struct{}

`, lay.DomainPkg))

	if archIR.DomainActor != nil {
		sb.WriteString(lay.emitGoStructDef("Actor", archIR.DomainActor.Fields))
	} else {
		sb.WriteString(`type Actor struct {
	ID         string ` + "`json:\"id\"`" + `
	Role       string ` + "`json:\"role\"`" + `
	Verified   bool   ` + "`json:\"verified\"`" + `
	IsVerified bool   ` + "`json:\"is_verified\"`" + `
	TenantID   string ` + "`json:\"tenant_id\"`" + `
}
`)
	}

	sb.WriteString(fmt.Sprintf(`
func WithActor(ctx context.Context, actor Actor) context.Context {
	return context.WithValue(ctx, actorCtxKey{}, actor)
}

func ActorFromContext(ctx context.Context) Actor {
	if v, ok := ctx.Value(actorCtxKey{}).(Actor); ok {
		return v
	}
	return Actor{}
}

`))
	sb.WriteString(`type PageInfo struct {
	NextCursor string ` + "`json:\"next_cursor\"`" + `
	PrevCursor string ` + "`json:\"prev_cursor\"`" + `
	HasNext    bool   ` + "`json:\"has_next\"`" + `
	HasPrev    bool   ` + "`json:\"has_prev\"`" + `
}

`)

	for _, uc := range archIR.UseCases {
		if uc.IsListReturning {
			entName := strings.TrimPrefix(uc.ReturnType, "[]")
			sb.WriteString(fmt.Sprintf(`type %sResult struct {
	Items    []*%s   `+"`json:\"items\"`"+`
	PageInfo PageInfo `+"`json:\"page_info\"`"+`
}

func (r *%sResult) Count() int {
	if r == nil {
		return 0
	}
	return len(r.Items)
}

`, uc.ActionName, entName, uc.ActionName))
		}
	}

	for _, v := range archIR.DomainValues {
		sb.WriteString(lay.emitGoStructDef(v.Name, v.Fields))
		sb.WriteString("\n")
	}
	return sb.String()
}

func (lay goLayout) emitGoDomainEvents(archIR *arch.ArchitectureIR) string {
	var sb strings.Builder
	sb.WriteString("package " + lay.DomainPkg + "\n\n")
	for _, ev := range archIR.DomainEvents {
		sb.WriteString(lay.emitGoStructDef(ev.Name, ev.Fields))
		sb.WriteString("\n")
	}
	return sb.String()
}

func (lay goLayout) emitGoDomainPolicies(archIR *arch.ArchitectureIR) string {
	var sb strings.Builder
	sb.WriteString("package " + lay.DomainPkg + "\n\n")
	for _, pol := range archIR.DomainPolicies {
		var params []string
		for _, p := range pol.Params {
			params = append(params, fmt.Sprintf("%s %s", p.Name, lay.goKindType(p.Kind, p.Type, true)))
		}
		sb.WriteString(fmt.Sprintf("func %s(%s) bool {\n\treturn %s\n}\n\n",
			pol.Name, strings.Join(params, ", "), lay.emitGoExpr(pol.Body, true)))
	}
	return sb.String()
}

func (lay goLayout) emitGoDomainErrors(archIR *arch.ArchitectureIR) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`package %s

type DomainError struct {
	Code    string `+"`json:\"code\"`"+`
	Kind    string `+"`json:\"kind\"`"+`
	Message string `+"`json:\"message\"`"+`
}

func (e *DomainError) Error() string {
	return e.Message
}

var ErrConflict = &DomainError{
	Code:    "Conflict",
	Kind:    "conflict",
	Message: "optimistic concurrency conflict",
}

var ErrUnauthenticated = &DomainError{
	Code:    "Unauthenticated",
	Kind:    "unauthenticated",
	Message: "unauthenticated actor",
}

`, lay.DomainPkg))
	for _, errDecl := range archIR.DomainErrors {
		if errDecl.Name == "Conflict" || errDecl.Name == "Unauthenticated" {
			continue
		}
		sb.WriteString(fmt.Sprintf("var Err%s = &DomainError{\n\tCode: %q,\n\tKind: %q,\n\tMessage: %q,\n}\n\n",
			errDecl.Name, errDecl.Name, errDecl.Kind, errDecl.Message))
	}
	return sb.String()
}

func (lay goLayout) emitGoSinglePort(module string, p arch.RepositoryPortIR) string {
	var sb strings.Builder
	hasTime := false
	for _, m := range p.Methods {
		for _, wp := range m.WherePredicates {
			if wp.Type == "timestamp" {
				hasTime = true
			}
		}
		if m.WhereType == "timestamp" {
			hasTime = true
		}
	}
	timeImport := ""
	if hasTime {
		timeImport = "\t\"time\"\n"
	}
	sb.WriteString(fmt.Sprintf(`package %s

import (
	"context"
%s	"%s/%s"
)

`, lay.PortsPkg, timeImport, module, lay.DomainDir))

	sb.WriteString(fmt.Sprintf("type %s interface {\n", p.InterfaceName))
	for _, m := range p.Methods {
		switch m.Kind {
		case arch.RepoMethodCreate:
			sb.WriteString(fmt.Sprintf("\tCreate(ctx context.Context, entity %s.%s) (%s.%s, error)\n",
				lay.DomainPkg, m.EntityName, lay.DomainPkg, m.EntityName))
		case arch.RepoMethodUpdate:
			sb.WriteString(fmt.Sprintf("\tUpdate(ctx context.Context, entity %s.%s) error\n", lay.DomainPkg, m.EntityName))
		case arch.RepoMethodDelete:
			sb.WriteString(fmt.Sprintf("\t%s(ctx context.Context, %s) error\n",
				m.Name, lay.formatRepoParams(m)))
		case arch.RepoMethodFind:
			sb.WriteString(fmt.Sprintf("\t%s(ctx context.Context, %s) (*%s.%s, error)\n",
				m.Name, lay.formatRepoParams(m), lay.DomainPkg, m.EntityName))
		case arch.RepoMethodFindMany:
			whereParams := lay.formatRepoParams(m)
			callParams := "ctx context.Context"
			if whereParams != "" {
				callParams += ", " + whereParams
			}
			callParams += ", cursor string, limit int, direction string"
			sb.WriteString(fmt.Sprintf("\t%s(%s) ([]*%s.%s, *%s.PageInfo, error)\n",
				m.Name, callParams, lay.DomainPkg, m.EntityName, lay.DomainPkg))
		}
	}
	sb.WriteString("}\n")

	return sb.String()
}

func (lay goLayout) emitGoEventPublisherPort() string {
	return fmt.Sprintf(`package %s

import "context"

type EventPublisher interface {
	Publish(ctx context.Context, eventType string, payload any) error
	Subscribe(eventType string, handler func(ctx context.Context, payload any) error)
}
`, lay.PortsPkg)
}

func (lay goLayout) emitGoTransactionManagerPort() string {
	return fmt.Sprintf(`package %s

import "context"

type TransactionManager interface {
	WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error
}
`, lay.PortsPkg)
}
