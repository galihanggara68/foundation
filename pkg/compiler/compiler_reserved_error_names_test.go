package compiler_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/galihanggara68/foundation/pkg/compiler"
)

// A declaration that reuses a name the platform ships is legal and merely warned about: the
// shipped error keeps the name, so the author's declaration is shadowed and the build
// succeeds. This is the end-to-end contract — the emitted domain error must still be the
// built-in one. A pre-seeded name being treated as a duplicate declaration would fail the
// build here, which is exactly the inconsistency this test pins down.
func TestBuild_ReservedBuiltinErrorName_ShadowedButBuilds(t *testing.T) {
	dir := t.TempDir()

	bp := `error Conflict {
  kind: conflict
  message: "order was modified concurrently"
}

entity Order {
  id: uuid @pk
  status: string
}

action GetOrder(id: uuid) -> Order {
  let order = query one Order where id == id else Conflict
  return order
}
`
	if err := os.WriteFile(filepath.Join(dir, "orders.fdn"), []byte(bp), 0644); err != nil {
		t.Fatalf("write blueprint: %v", err)
	}

	buildProfile := `project:
  name: orders
  module: example.com/orders

storage:
  dialect: postgres
  mode: stateful

targets:
  - name: backend
    kind: backend
    language: go
    architecture: hexagonal
    http: chi
    out_dir: gen
`
	if err := os.WriteFile(filepath.Join(dir, "foundation.build.yaml"), []byte(buildProfile), 0644); err != nil {
		t.Fatalf("write foundation.build.yaml: %v", err)
	}

	res, diags := compiler.Build(dir, compiler.BuildOptions{})
	if res == nil {
		t.Fatalf("a reserved built-in name must not abort the build, got %+v", diags)
	}
	if compiler.HasErrors(diags) {
		t.Fatalf("a collision with a shipped error is a warning, not an error; got %+v", diags)
	}
	if findCollision(diags) == nil {
		t.Errorf("expected the collision to be reported as FDN0219, got %+v", diags)
	}

	errorsGo := readEmittedFile(t, filepath.Join(dir, "gen", "internal", "domain", "errors.go"))

	// The built-in wins: the author's declaration is shadowed, so the variable is defined
	// exactly once and it carries the platform's message rather than the author's.
	defs := regexp.MustCompile(`(?m)^\s*(?:var\s+)?ErrConflict\s*=`).FindAllString(errorsGo, -1)
	if len(defs) != 1 {
		t.Errorf("expected exactly one ErrConflict definition — the declaration is shadowed by the built-in — got %d:\n%s", len(defs), errorsGo)
	}
	if !strings.Contains(errorsGo, "optimistic concurrency conflict") {
		t.Errorf("expected the shipped conflict message to be emitted, got:\n%s", errorsGo)
	}
	if strings.Contains(errorsGo, "order was modified concurrently") {
		t.Errorf("expected the author's shadowed message not to be emitted, got:\n%s", errorsGo)
	}
}
