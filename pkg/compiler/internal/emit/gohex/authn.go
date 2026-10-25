package gohex

import (
	"fmt"
	"strings"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/arch"
)

func (lay goLayout) emitGoActorResolverPort(module string) string {
	return fmt.Sprintf(`package %s

import (
	"context"
	"net/http"
	"%s/%s"
)

// ActorResolver intercepts incoming HTTP requests to authenticate and resolve the verified Actor.
type ActorResolver interface {
	ResolveActor(ctx context.Context, r *http.Request) (%s.Actor, error)
}
`, lay.PortsPkg, module, lay.DomainDir, lay.DomainPkg)
}

func (lay goLayout) emitGoAuthnAdapters(archIR *arch.ArchitectureIR) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`package %s

import (
	"context"
	"net/http"
	"os"

	"github.com/galihanggara68/foundation/contrib/go/runtime"
	"%s/%s"
	"%s/%s"
)

type DevHeadersActorResolver struct {
	inner *runtime.DevHeadersResolver
}

func NewDevHeadersActorResolver() %s.ActorResolver {
	return &DevHeadersActorResolver{inner: runtime.NewDevHeadersResolver()}
}

func (a *DevHeadersActorResolver) ResolveActor(ctx context.Context, r *http.Request) (%s.Actor, error) {
	claims, err := a.inner.ResolveActor(ctx, r)
	if err != nil {
		return %s.Actor{}, %s.ErrUnauthenticated
	}
	var act %s.Actor
`, lay.AuthnPkg, archIR.Module, lay.DomainDir, archIR.Module, lay.PortsDir, lay.PortsPkg, lay.DomainPkg, lay.DomainPkg, lay.DomainPkg, lay.DomainPkg))

	emitActorPopulation(&sb, archIR)

	sb.WriteString(fmt.Sprintf(`	return act, nil
}

type JWTOIDCActorResolver struct {
	inner *runtime.JWTOIDCResolver
}

func NewJWTOIDCActorResolver(issuer, audience string) %s.ActorResolver {
	if issuer == "" {
		issuer = os.Getenv("AUTH_ISSUER")
	}
	if audience == "" {
		audience = os.Getenv("AUTH_AUDIENCE")
	}
	return &JWTOIDCActorResolver{inner: runtime.NewJWTOIDCResolver(issuer, audience)}
}

func (a *JWTOIDCActorResolver) ResolveActor(ctx context.Context, r *http.Request) (%s.Actor, error) {
	claims, err := a.inner.ResolveActor(ctx, r)
	if err != nil {
		return %s.Actor{}, %s.ErrUnauthenticated
	}
	var act %s.Actor
`, lay.PortsPkg, lay.DomainPkg, lay.DomainPkg, lay.DomainPkg, lay.DomainPkg))

	emitActorPopulation(&sb, archIR)

	sb.WriteString("\treturn act, nil\n}\n")
	return sb.String()
}

func emitActorPopulation(sb *strings.Builder, archIR *arch.ArchitectureIR) {
	if archIR.DomainActor == nil {
		sb.WriteString(`	if id, ok := claims["id"].(string); ok {
		act.ID = id
	}
	if role, ok := claims["role"].(string); ok {
		act.Role = role
	}
	if v, ok := claims["verified"].(bool); ok {
		act.Verified = v
	}
	if v, ok := claims["is_verified"].(bool); ok {
		act.IsVerified = v
	}
	if tid, ok := claims["tenant_id"].(string); ok {
		act.TenantID = tid
	}
`)
		return
	}
	for _, f := range archIR.DomainActor.Fields {
		fExp := arch.ExportIdent(f.Name)
		switch f.Type {
		case "bool":
			sb.WriteString(fmt.Sprintf("\tif v, ok := claims[%q].(bool); ok {\n\t\tact.%s = v\n\t}\n", f.Name, fExp))
		default:
			sb.WriteString(fmt.Sprintf("\tif v, ok := claims[%q].(string); ok {\n\t\tact.%s = v\n\t}\n", f.Name, fExp))
		}
	}
}
