package compiler_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/galihanggara68/foundation/pkg/compiler"
)

func TestBuild_ErrorStatus_ResolvedIntoDomainAndHTTPServer(t *testing.T) {
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

	buildProfile := `project:
  name: catalog
  module: example.com/catalog

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

	if _, diags := compiler.Build(dir, compiler.BuildOptions{}); len(diags) > 0 {
		t.Fatalf("compiler.Build failed: %+v", diags)
	}

	errorsGo := readEmittedFile(t, filepath.Join(dir, "gen", "internal", "domain", "errors.go"))
	if !regexp.MustCompile(`Status:\s+404,`).MatchString(errorsGo) {
		t.Errorf("expected the kind default (not_found -> 404) to be emitted as the declared error's status, got:\n%s", errorsGo)
	}
	if !regexp.MustCompile(`Status:\s+409,`).MatchString(errorsGo) {
		t.Errorf("expected the built-in conflict error to take its status from the platform vocabulary, got:\n%s", errorsGo)
	}

	routerGo := readEmittedFile(t, filepath.Join(dir, "gen", "internal", "adapters", "http", "router.go"))
	if !strings.Contains(routerGo, "w.WriteHeader(errorStatus(domErr))") {
		t.Errorf("expected the HTTP error writer to use the resolved status, got:\n%s", routerGo)
	}
	if !regexp.MustCompile(`"forbidden":\s+403`).MatchString(routerGo) {
		t.Errorf("expected the emitted runtime to carry the platform kind->status fallback for errors the compiler did not stamp, got:\n%s", routerGo)
	}
}

func readEmittedFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read emitted %s: %v", path, err)
	}
	return string(content)
}
