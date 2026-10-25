package compiler_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/galihanggara68/foundation/pkg/compiler"
)

func TestBuild_CustomActorAndAuthnAdapters_CompilesAndPassesTests(t *testing.T) {
	dir := t.TempDir()

	bp := `project:
  name: tenancy
  module: example.com/tenancy

authn: dev-headers

storage:
  dialect: postgres
  mode: stateful

targets:
  - name: backend
    kind: backend
    language: go
    architecture: hexagonal
    http: stdlib
    out_dir: gen
`
	if err := os.WriteFile(filepath.Join(dir, "foundation.build.yaml"), []byte(bp), 0644); err != nil {
		t.Fatalf("write build profile: %v", err)
	}

	fdn := `
actor TenantActor {
  id: uuid
  role: string
  tenant_id: uuid
  tier: string
}

error UnauthorizedTenant {
  kind: unauthenticated
  message: "tenant credentials required"
}

error ForbiddenTier {
  kind: forbidden
  message: "enterprise tier required"
}

policy CanAccessEnterprise(actor: Actor) {
  actor.tier == "enterprise"
}

entity Organization {
  id: uuid @pk
  name: string
  tier: string
}

action AccessEnterprise(org_name: string) -> Organization {
  require CanAccessEnterprise(actor) else ForbiddenTier
  let org = create Organization {
    name: org_name,
    tier: actor.tier
  }
  return org
}

scenario "Enterprise access succeeds for enterprise actor" {
  given actor with {
    id: "00000000-0000-0000-0000-000000000001",
    role: "admin",
    tenant_id: "00000000-0000-0000-0000-000000000002",
    tier: "enterprise"
  }
  when AccessEnterprise("Acme Corp")
  expect stored Organization where name == "Acme Corp" {
    name: "Acme Corp"
  }
}

scenario "Enterprise access fails for free tier actor" {
  given actor with {
    id: "00000000-0000-0000-0000-000000000001",
    role: "member",
    tenant_id: "00000000-0000-0000-0000-000000000002",
    tier: "free"
  }
  when AccessEnterprise("Acme Corp")
  expect error ForbiddenTier
}
`
	if err := os.WriteFile(filepath.Join(dir, "tenancy.fdn"), []byte(fdn), 0644); err != nil {
		t.Fatalf("write blueprint: %v", err)
	}

	res, diags := compiler.Build(dir, compiler.BuildOptions{})
	if len(diags) != 0 {
		t.Fatalf("build failed: %+v", diags)
	}
	_ = res

	// Verify ActorResolver port and authn adapter files are generated
	actorPortFile := filepath.Join(dir, "gen/internal/ports/actor_resolver.go")
	if _, err := os.Stat(actorPortFile); err != nil {
		t.Fatalf("expected actor_resolver.go port to be emitted, err: %v", err)
	}
	authnAdapterFile := filepath.Join(dir, "gen/internal/adapters/authn/authn_adapter.go")
	if _, err := os.Stat(authnAdapterFile); err != nil {
		t.Fatalf("expected authn_adapter.go to be emitted, err: %v", err)
	}

	// Verify domain Actor struct declared with custom fields
	domainActorBytes, err := os.ReadFile(filepath.Join(dir, "gen/internal/domain/values.go"))
	if err != nil {
		t.Fatalf("read values.go: %v", err)
	}
	if !strings.Contains(string(domainActorBytes), "TenantID") || !strings.Contains(string(domainActorBytes), "Tier") {
		t.Fatalf("expected custom Actor struct fields in values.go, got:\n%s", string(domainActorBytes))
	}

	// Run tests in the generated directory
	if testDiags := compiler.Test(dir); len(testDiags) != 0 {
		t.Fatalf("generated test suite failed: %+v", testDiags)
	}
}

func TestBuild_OmitActorResolverPortWhenNoActorUsed(t *testing.T) {
	dir := t.TempDir()

	bp := `project:
  name: store
  module: example.com/store

storage:
  dialect: postgres
  mode: stateful

targets:
  - name: backend
    kind: backend
    language: go
    architecture: hexagonal
    http: stdlib
    out_dir: gen
`
	if err := os.WriteFile(filepath.Join(dir, "foundation.build.yaml"), []byte(bp), 0644); err != nil {
		t.Fatalf("write build profile: %v", err)
	}

	fdn := `
entity Product {
  id: uuid @pk
  name: string
}

action CreateProduct(name: string) -> Product {
  let p = create Product {
    name: name
  }
  return p
}
`
	if err := os.WriteFile(filepath.Join(dir, "store.fdn"), []byte(fdn), 0644); err != nil {
		t.Fatalf("write blueprint: %v", err)
	}

	_, diags := compiler.Build(dir, compiler.BuildOptions{})
	if len(diags) != 0 {
		t.Fatalf("build failed: %+v", diags)
	}

	// Verify ActorResolver port was NOT emitted
	actorPortFile := filepath.Join(dir, "gen/internal/ports/actor_resolver.go")
	if _, err := os.Stat(actorPortFile); !os.IsNotExist(err) {
		t.Fatalf("expected actor_resolver.go to be omitted when no actor used, but it exists")
	}
}
