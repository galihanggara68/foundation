package compiler_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/galihanggara68/foundation/pkg/compiler"
)

// A configured authn adapter has to reach every container the emitter generates. The Postgres
// container used to hardcode DevHeadersResolver, so a TypeScript backend built with
// `authn: jwt-oidc` silently ran the adapter the operator did not choose.
func TestBuild_TypeScriptAuthnAdapter_ReachesEveryContainer(t *testing.T) {
	dir := t.TempDir()

	profile := `project:
  name: authn-ts
  module: example.com/authnts

authn: jwt-oidc

storage:
  dialect: memory
  mode: in_memory

targets:
  - name: backend-ts
    kind: backend
    language: typescript
    architecture: clean
    http: express
    out_dir: gen
`
	if err := os.WriteFile(filepath.Join(dir, "foundation.build.yaml"), []byte(profile), 0644); err != nil {
		t.Fatalf("write build profile: %v", err)
	}

	blueprint := `
entity Doc {
  id: uuid @pk
  title: string
}

error DocNotFound {
  kind: not_found
  message: "no such doc"
}

error DocForbidden {
  kind: forbidden
  message: "not a reader"
}

policy CanRead(actor: Actor) {
  actor.role == "reader"
}

action ReadDoc(doc_id: uuid) -> Doc {
  require CanRead(actor) else DocForbidden
  let doc = query one Doc where id == doc_id else DocNotFound
  return doc
}
`
	if err := os.WriteFile(filepath.Join(dir, "docs.fdn"), []byte(blueprint), 0644); err != nil {
		t.Fatalf("write blueprint: %v", err)
	}

	if _, diags := compiler.Build(dir, compiler.BuildOptions{}); compiler.HasErrors(diags) {
		t.Fatalf("compiler.Build failed: %+v", diags)
	}

	emitted, err := os.ReadFile(filepath.Join(dir, "gen", "src", "main", "container.ts"))
	if err != nil {
		t.Fatalf("read emitted container: %v", err)
	}
	container := string(emitted)

	if got := strings.Count(container, "new JWTOIDCResolver("); got != 2 {
		t.Errorf("expected both containers to construct the configured jwt-oidc resolver, found %d:\n%s", got, container)
	}
	if strings.Contains(container, "new DevHeadersResolver()") {
		t.Errorf("a jwt-oidc build must not construct the dev-headers resolver:\n%s", container)
	}
}
