package gohex

import (
	"fmt"
	"strings"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/arch"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
)

func emitGoDomainEnums(archIR *arch.ArchitectureIR) string {
	var sb strings.Builder
	sb.WriteString("package domain\n\n")
	for _, en := range archIR.DomainEnums {
		sb.WriteString(fmt.Sprintf("type %s string\n\nconst (\n", en.Name))
		for _, v := range en.Variants {
			sb.WriteString(fmt.Sprintf("\t%s%s %s = %q\n", en.Name, arch.ExportIdent(v), en.Name, v))
		}
		sb.WriteString(")\n\n")
	}
	return sb.String()
}

func emitGoStructDef(name string, fields []ir.FieldDecl) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("type %s struct {\n", name))
	for _, f := range fields {
		sb.WriteString(fmt.Sprintf("\t%s %s `json:%q`\n", arch.ExportIdent(f.Name), goType(f.Type, true), f.Name))
	}
	sb.WriteString("}\n")
	return sb.String()
}

func emitGoFieldInits(sb *strings.Builder, fields []ir.FieldInitDecl, indent string) {
	for _, fi := range fields {
		sb.WriteString(fmt.Sprintf("%s%s: %s,\n", indent, arch.ExportIdent(fi.Name), emitGoExpr(fi.Value, false)))
	}
}

func emitGoDomainValues(archIR *arch.ArchitectureIR) string {
	var sb strings.Builder
	sb.WriteString(`package domain

import "context"

type actorCtxKey struct{}

type Actor struct {
	ID         string ` + "`json:\"id\"`" + `
	Role       string ` + "`json:\"role\"`" + `
	IsVerified bool   ` + "`json:\"is_verified\"`" + `
	TenantID   string ` + "`json:\"tenant_id\"`" + `
}

func WithActor(ctx context.Context, actor Actor) context.Context {
	return context.WithValue(ctx, actorCtxKey{}, actor)
}

func ActorFromContext(ctx context.Context) Actor {
	if v, ok := ctx.Value(actorCtxKey{}).(Actor); ok {
		return v
	}
	return Actor{}
}

`)
	for _, v := range archIR.DomainValues {
		sb.WriteString(emitGoStructDef(v.Name, v.Fields))
		sb.WriteString("\n")
	}
	return sb.String()
}

func emitGoDomainEvents(archIR *arch.ArchitectureIR) string {
	var sb strings.Builder
	sb.WriteString("package domain\n\n")
	for _, ev := range archIR.DomainEvents {
		sb.WriteString(emitGoStructDef(ev.Name, ev.Fields))
		sb.WriteString("\n")
	}
	return sb.String()
}

func emitGoDomainPolicies(archIR *arch.ArchitectureIR) string {
	var sb strings.Builder
	sb.WriteString("package domain\n\n")
	for _, pol := range archIR.DomainPolicies {
		var params []string
		for _, p := range pol.Params {
			params = append(params, fmt.Sprintf("%s %s", p.Name, goType(p.Type, true)))
		}
		sb.WriteString(fmt.Sprintf("func %s(%s) bool {\n\treturn %s\n}\n\n",
			pol.Name, strings.Join(params, ", "), emitGoExpr(pol.Body, true)))
	}
	return sb.String()
}

func emitGoDomainErrors(archIR *arch.ArchitectureIR) string {
	var sb strings.Builder
	sb.WriteString(`package domain

type DomainError struct {
	Code    string ` + "`json:\"code\"`" + `
	Kind    string ` + "`json:\"kind\"`" + `
	Message string ` + "`json:\"message\"`" + `
}

func (e *DomainError) Error() string {
	return e.Message
}

`)
	for _, errDecl := range archIR.DomainErrors {
		sb.WriteString(fmt.Sprintf("var Err%s = &DomainError{\n\tCode: %q,\n\tKind: %q,\n\tMessage: %q,\n}\n\n",
			errDecl.Name, errDecl.Name, errDecl.Kind, errDecl.Message))
	}
	return sb.String()
}

func emitGoSinglePort(module string, p arch.RepositoryPortIR) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`package ports

import (
	"context"
	"%s/internal/domain"
)

`, module))

	sb.WriteString(fmt.Sprintf("type %s interface {\n", p.InterfaceName))
	for _, m := range p.Methods {
		switch m.Kind {
		case arch.RepoMethodCreate:
			sb.WriteString(fmt.Sprintf("\tCreate(ctx context.Context, entity domain.%s) (domain.%s, error)\n", m.EntityName, m.EntityName))
		case arch.RepoMethodUpdate:
			sb.WriteString(fmt.Sprintf("\tUpdate(ctx context.Context, entity domain.%s) error\n", m.EntityName))
		case arch.RepoMethodDelete:
			sb.WriteString(fmt.Sprintf("\t%s(ctx context.Context, %s %s) error\n",
				m.Name, m.WhereField, goType(m.WhereType, false)))
		case arch.RepoMethodFind:
			sb.WriteString(fmt.Sprintf("\t%s(ctx context.Context, %s %s) (*domain.%s, error)\n",
				m.Name, m.WhereField, goType(m.WhereType, false), m.EntityName))
		}
	}
	sb.WriteString("}\n")

	return sb.String()
}

func emitGoEventPublisherPort() string {
	return `package ports

import "context"

type EventPublisher interface {
	Publish(ctx context.Context, eventType string, payload any) error
}
`
}

func emitGoTransactionManagerPort() string {
	return `package ports

import "context"

type TransactionManager interface {
	WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error
}
`
}
