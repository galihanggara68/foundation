package compiler_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/galihanggara68/foundation/pkg/compiler"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/arch"
)

// The emitted pin, the emitted checksum and the build profile's compatibility check all have to
// name the same contrib runtime release, or a generated application depends on a version the
// compiler would then refuse through dependencies.runtime. They once disagreed: the emitter wrote
// v0.2.0 while the check still required ~0.1.0.
func TestBuild_ContribRuntimePinMatchesTheCompatibilityCheck(t *testing.T) {
	dir := t.TempDir()

	profile := `project:
  name: pin
  module: example.com/pin

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
	if err := os.WriteFile(filepath.Join(dir, "foundation.build.yaml"), []byte(profile), 0644); err != nil {
		t.Fatalf("write build profile: %v", err)
	}

	blueprint := `
entity Doc {
  id: uuid @pk
  title: string
}

action CreateDoc(title: string) -> Doc
  @http(method: POST, path: "/v1/docs")
{
  let doc = create Doc { title: title }
  return doc
}
`
	if err := os.WriteFile(filepath.Join(dir, "docs.fdn"), []byte(blueprint), 0644); err != nil {
		t.Fatalf("write blueprint: %v", err)
	}

	if _, diags := compiler.Build(dir, compiler.BuildOptions{}); compiler.HasErrors(diags) {
		t.Fatalf("compiler.Build failed: %+v", diags)
	}

	want := "contrib/go/runtime v" + arch.ContribRuntimeVersion
	for _, name := range []string{"go.mod", "go.sum"} {
		emitted, err := os.ReadFile(filepath.Join(dir, "gen", name))
		if err != nil {
			t.Fatalf("read emitted %s: %v", name, err)
		}
		if !strings.Contains(string(emitted), want) {
			t.Errorf("emitted %s does not pin %q:\n%s", name, want, emitted)
		}
	}
}
