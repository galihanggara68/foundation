package compiler_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/galihanggara68/foundation/pkg/compiler"
)

const errorDeclTSBuildProfile = `project:
  name: catalog
  module: example.com/catalog

storage:
  dialect: postgres
  mode: stateful

targets:
  - name: backend
    kind: backend
    language: typescript
    architecture: clean
    http: express
    out_dir: gen
`

func TestBuild_ErrorStatus_ResolvedIntoTSDomainAndController(t *testing.T) {
	dir := t.TempDir()

	bp := `error ProductNotFound {
  kind: not_found
  message: "requested product does not exist"
}

entity Product {
  id: uuid @pk
  name: string
}

action GetProduct(id: uuid) -> Product {
  let product = query one Product where id == id else ProductNotFound
  return product
}
`
	if err := os.WriteFile(filepath.Join(dir, "catalog.fdn"), []byte(bp), 0644); err != nil {
		t.Fatalf("write blueprint: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "foundation.build.yaml"), []byte(errorDeclTSBuildProfile), 0644); err != nil {
		t.Fatalf("write foundation.build.yaml: %v", err)
	}

	if _, diags := compiler.Build(dir, compiler.BuildOptions{}); len(diags) > 0 {
		t.Fatalf("compiler.Build failed: %+v", diags)
	}

	domainTS := readEmittedFile(t, filepath.Join(dir, "gen", "src", "entities", "domain.ts"))
	if !regexp.MustCompile(`ErrProductNotFound[\s\S]*?404,\s*"business"\)`).MatchString(domainTS) {
		t.Errorf("expected the kind default (not_found -> 404, business) to be stamped onto the declared error constant, got:\n%s", domainTS)
	}
	if !regexp.MustCompile(`ErrConflict[\s\S]*?409,\s*"business"\)`).MatchString(domainTS) {
		t.Errorf("expected the built-in conflict error to take its status and posture from the platform vocabulary, got:\n%s", domainTS)
	}

	routerTS := readEmittedFile(t, filepath.Join(dir, "gen", "src", "controllers", "express_router.ts"))
	if !strings.Contains(routerTS, "res.status(policy.status)") {
		t.Errorf("expected the TypeScript error writer to use the resolved status, got:\n%s", routerTS)
	}
	if !strings.Contains(routerTS, "standardErrorPolicy") {
		t.Errorf("expected the emitted runtime to carry a generated policy table, got:\n%s", routerTS)
	}
	if !regexp.MustCompile(`"forbidden":\s*\{status:\s*403,\s*posture:\s*"business"\}`).MatchString(routerTS) {
		t.Errorf("expected the emitted runtime to carry the platform kind->policy fallback for errors the compiler did not stamp, got:\n%s", routerTS)
	}
}

func TestBuild_ErrorPostureAndMessages_ReachEmittedTSDomainAndController(t *testing.T) {
	dir := t.TempDir()

	bp := `error ProductNotFound {
  kind: not_found
  message: "requested product does not exist"
  messages: {
    en: "requested product does not exist"
    id: "produk yang diminta tidak ditemukan"
  }
}

error Boom {
  kind: internal
  message: "database connection reset"
}

entity Product {
  id: uuid @pk
  name: string
}

action GetProduct(id: uuid) -> Product {
  let product = query one Product where id == id else ProductNotFound
  return product
}
`
	if err := os.WriteFile(filepath.Join(dir, "catalog.fdn"), []byte(bp), 0644); err != nil {
		t.Fatalf("write blueprint: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "foundation.build.yaml"), []byte(errorDeclTSBuildProfile), 0644); err != nil {
		t.Fatalf("write foundation.build.yaml: %v", err)
	}

	if _, diags := compiler.Build(dir, compiler.BuildOptions{}); len(diags) > 0 {
		t.Fatalf("compiler.Build failed: %+v", diags)
	}

	domainTS := readEmittedFile(t, filepath.Join(dir, "gen", "src", "entities", "domain.ts"))
	if !regexp.MustCompile(`ErrProductNotFound[\s\S]*?"business"`).MatchString(domainTS) {
		t.Errorf("expected not_found to resolve to the business posture, got:\n%s", domainTS)
	}
	if !regexp.MustCompile(`ErrBoom[\s\S]*?500,\s*"technical"\)`).MatchString(domainTS) {
		t.Errorf("expected internal to resolve to the technical posture at 500, got:\n%s", domainTS)
	}
	if !strings.Contains(domainTS, `"id": "produk yang diminta tidak ditemukan"`) {
		t.Errorf("expected the declared locale to be emitted, got:\n%s", domainTS)
	}
	if !regexp.MustCompile(`"en":[\s\S]*?"id":`).MatchString(domainTS) {
		t.Errorf("expected the declared locale keys to be sorted, got:\n%s", domainTS)
	}

	routerTS := readEmittedFile(t, filepath.Join(dir, "gen", "src", "controllers", "express_router.ts"))
	if !strings.Contains(routerTS, `policy.posture === "technical"`) {
		t.Errorf("expected a technical error's message to be withheld from the caller, got:\n%s", routerTS)
	}
	if !strings.Contains(routerTS, "console.error") {
		t.Errorf("expected the withheld diagnostic to be logged rather than dropped, got:\n%s", routerTS)
	}
	if !regexp.MustCompile(`"forbidden":\s*\{status:\s*403,\s*posture:\s*"business"\}`).MatchString(routerTS) {
		t.Errorf("expected the fallback table to carry each kind's standard posture as well as its status, got:\n%s", routerTS)
	}
}
