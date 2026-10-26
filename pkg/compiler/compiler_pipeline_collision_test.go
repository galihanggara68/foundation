package compiler_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/galihanggara68/foundation/pkg/compiler"
)

func TestBuild_Pipeline_CollisionWarning_FDN0702(t *testing.T) {
	dir := setupTestWorkspace(t)
	gen1 := filepath.Join(dir, "tools", "gen-1")
	createExecutablePlugin(t, gen1, `
cat > /dev/null
cat << 'EOF'
{
  "files": {
    "shared.txt": "v1 from gen1"
  }
}
EOF
exit 0
`)

	gen2 := filepath.Join(dir, "tools", "gen-2")
	createExecutablePlugin(t, gen2, `
cat > /dev/null
cat << 'EOF'
{
  "files": {
    "shared.txt": "v2 from gen2"
  }
}
EOF
exit 0
`)

	buildYAML := `
project:
  name: shop
  module: example.com/shop

targets:
  - name: collision-test
    kind: backend
    language: go
    architecture: custom
    out_dir: gen/collision
    generators:
      - ./tools/gen-1
      - ./tools/gen-2
    scaffold: none
`
	if err := os.WriteFile(filepath.Join(dir, "foundation.build.yaml"), []byte(buildYAML), 0644); err != nil {
		t.Fatalf("failed to write yaml: %v", err)
	}

	_, diags := compiler.Build(dir, compiler.BuildOptions{})

	// Should produce warning diagnostic FDN0702
	foundCollision := false
	for _, d := range diags {
		if d.Code == "FDN0702" && d.Severity == compiler.SeverityWarning {
			foundCollision = true
			if !strings.Contains(d.Message, "shared.txt") {
				t.Errorf("expected diagnostic message to cite shared.txt: %s", d.Message)
			}
		}
	}
	if !foundCollision {
		t.Fatalf("expected collision warning FDN0702, got: %+v", diags)
	}
}

func TestBuild_Pipeline_InvalidScaffoldMode(t *testing.T) {
	dir := setupTestWorkspace(t)
	buildYAML := `
project:
  name: shop
  module: example.com/shop

targets:
  - name: test
    kind: backend
    language: go
    architecture: hexagonal
    out_dir: gen/test
    generator: ./tools/gen-dummy
    scaffold: invalid_mode
`
	if err := os.WriteFile(filepath.Join(dir, "foundation.build.yaml"), []byte(buildYAML), 0644); err != nil {
		t.Fatalf("failed to write yaml: %v", err)
	}

	_, diags := compiler.Build(dir, compiler.BuildOptions{})
	found := false
	for _, d := range diags {
		if d.Code == "FDN0004" && strings.Contains(d.Message, "invalid scaffold mode") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected FDN0004 for invalid scaffold mode, got: %+v", diags)
	}
}
