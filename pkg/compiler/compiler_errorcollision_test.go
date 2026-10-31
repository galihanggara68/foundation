package compiler_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/galihanggara68/foundation/pkg/compiler"
)

func findCollision(diags []compiler.Diagnostic) *compiler.Diagnostic {
	for i := range diags {
		if diags[i].Code == "FDN0219" {
			return &diags[i]
		}
	}
	return nil
}

// Reusing a name the vocabulary owns is worth telling the author about, but it is not
// something to stop for: the blueprint still compiles and still builds. This is the trap
// the diagnostic carries — a warning treated as a failure makes the build fatal.
func TestCompileAndBuild_ReservedErrorName_WarnsWithoutFailing(t *testing.T) {
	blueprint := `error NotFound {
  kind: not_found
  message: "no such item"
}

entity Item {
  id: uuid @pk
  name: string
}

action GetItem(id: uuid) -> Item {
  let item = query one Item where id == id else NotFound
  return item
}
`

	sem, diags := compiler.Compile([]compiler.SourceFile{{Path: "shop.fdn", Content: blueprint}})
	if sem == nil {
		t.Fatalf("a warning must not abort compilation, got %+v", diags)
	}
	if compiler.HasErrors(diags) {
		t.Fatalf("expected no errors, got %+v", diags)
	}
	if findCollision(diags) == nil {
		t.Fatalf("expected the reserved name to be reported, got %+v", diags)
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "shop.fdn"), []byte(blueprint), 0644); err != nil {
		t.Fatalf("write blueprint: %v", err)
	}
	buildProfile := `project:
  name: shop
  module: example.com/shop

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

	res, buildDiags := compiler.Build(dir, compiler.BuildOptions{})
	if res == nil {
		t.Fatalf("a warning must not abort the build, got %+v", buildDiags)
	}
	if compiler.HasErrors(buildDiags) {
		t.Fatalf("expected no build errors, got %+v", buildDiags)
	}
	if findCollision(buildDiags) == nil {
		t.Errorf("expected the warning to travel with the build result, got %+v", buildDiags)
	}
}
