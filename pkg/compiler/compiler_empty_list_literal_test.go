package compiler_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/galihanggara68/foundation/pkg/compiler"
)

func TestBuild_EmptyListLiteral_RendersValidGo(t *testing.T) {
	dir := t.TempDir()

	bp := `value Money {
  amount: decimal
  currency: string
}

action ListSummaries(id: uuid) -> []Money {
  return []
}
`
	if err := os.WriteFile(filepath.Join(dir, "catalog.fdn"), []byte(bp), 0644); err != nil {
		t.Fatalf("write blueprint: %v", err)
	}

	buildProfile := `project:
  name: catalog
  module: example.com/catalog

storage:
  dialect: memory
  mode: in_memory

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

	_, diags := compiler.Build(dir, compiler.BuildOptions{})
	if compiler.HasErrors(diags) {
		t.Fatalf("compiler.Build reported errors: %+v", diags)
	}

	useCasePath := filepath.Join(dir, "gen", "internal", "usecases", "listsummaries_usecase.go")
	useCaseBytes, err := os.ReadFile(useCasePath)
	if err != nil {
		t.Fatalf("read emitted %s: %v", useCasePath, err)
	}
	useCaseSrc := string(useCaseBytes)

	if bad := regexp.MustCompile(`\[\][A-Za-z0-9_.]*\.\{\}`).FindString(useCaseSrc); bad != "" {
		t.Errorf("expected the empty list literal to render as a valid Go empty slice, got malformed composite literal %q:\n%s", bad, useCaseSrc)
	}

	itemsInit := regexp.MustCompile(`Items:\s+([^,]+),`).FindStringSubmatch(useCaseSrc)
	if itemsInit == nil {
		t.Fatalf("emitted use case has no Items initializer:\n%s", useCaseSrc)
	}
	rendered := strings.TrimSpace(itemsInit[1])
	if rendered != "nil" && !regexp.MustCompile(`^\[\]\*?[A-Za-z_][A-Za-z0-9_.]*\{\}$`).MatchString(rendered) {
		t.Errorf("expected the empty list literal to render as a valid Go empty slice, got %q:\n%s", rendered, useCaseSrc)
	}
}
