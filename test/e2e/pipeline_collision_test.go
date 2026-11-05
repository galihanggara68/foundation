package e2e_test

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

func TestBuild_DiagnosticPrefixCollision_FDN0603(t *testing.T) {
	dir := setupTestWorkspace(t)
	buildYAML := `
project:
  name: shop
  module: example.com/shop

targets:
  - name: custom-backend
    kind: backend
    language: go
    architecture: custom
    out_dir: gen/custom
    generator: ./tools/gen-1
    diagnostic_prefix: ACME
  - name: telemetry-ext
    kind: backend
    language: go
    architecture: custom
    out_dir: gen/telemetry
    generator: ./tools/gen-2
    diagnostic_prefix: ACME
`
	if err := os.WriteFile(filepath.Join(dir, "foundation.build.yaml"), []byte(buildYAML), 0644); err != nil {
		t.Fatalf("failed to write yaml: %v", err)
	}

	_, diags := compiler.Build(dir, compiler.BuildOptions{})
	found := false
	for _, d := range diags {
		if d.Code == "FDN0603" {
			found = true
			if !strings.Contains(d.Message, "ACME") {
				t.Errorf("expected FDN0603 to name the prefix, got: %s", d.Message)
			}
		}
	}
	if !found {
		t.Fatalf("expected FDN0603 prefix collision, got: %+v", diags)
	}
}

func TestBuild_DiagnosticPrefixMismatch_FDN0605(t *testing.T) {
	dir := setupTestWorkspace(t)
	gen := filepath.Join(dir, "tools", "gen-ns")
	createExecutablePlugin(t, gen, `
cat > /dev/null
cat << 'EOF'
{
  "files": { "gen.txt": "ok" },
  "diagnostics": [
    { "code": "OTHER01", "severity": "warning", "message": "outside namespace", "span": { "file": "x", "line": 1, "column": 1 } }
  ]
}
EOF
exit 0
`)

	buildYAML := `
project:
  name: shop
  module: example.com/shop

targets:
  - name: custom
    kind: backend
    language: go
    architecture: custom
    out_dir: gen/custom
    generator: ./tools/gen-ns
    scaffold: none
    diagnostic_prefix: ACME
`
	if err := os.WriteFile(filepath.Join(dir, "foundation.build.yaml"), []byte(buildYAML), 0644); err != nil {
		t.Fatalf("failed to write yaml: %v", err)
	}

	_, diags := compiler.Build(dir, compiler.BuildOptions{})
	found := false
	for _, d := range diags {
		if d.Code == "FDN0605" && d.Severity == compiler.SeverityWarning {
			found = true
			if !strings.Contains(d.Message, "OTHER01") {
				t.Errorf("expected FDN0605 to name the offending code, got: %s", d.Message)
			}
		}
	}
	if !found {
		t.Fatalf("expected FDN0605 prefix mismatch warning, got: %+v", diags)
	}
}

func TestBuild_DiagnosticPrefixMismatchOnError_FDN0605(t *testing.T) {
	dir := setupTestWorkspace(t)
	gen := filepath.Join(dir, "tools", "gen-ns-err")
	createExecutablePlugin(t, gen, `
cat > /dev/null
cat << 'EOF'
{
  "files": { "gen.txt": "ok" },
  "diagnostics": [
    { "code": "OTHER99", "severity": "error", "message": "outside namespace", "span": { "file": "x", "line": 1, "column": 1 } }
  ]
}
EOF
exit 0
`)

	buildYAML := `
project:
  name: shop
  module: example.com/shop

targets:
  - name: custom
    kind: backend
    language: go
    architecture: custom
    out_dir: gen/custom
    generator: ./tools/gen-ns-err
    scaffold: none
    diagnostic_prefix: ACME
`
	if err := os.WriteFile(filepath.Join(dir, "foundation.build.yaml"), []byte(buildYAML), 0644); err != nil {
		t.Fatalf("failed to write yaml: %v", err)
	}

	_, diags := compiler.Build(dir, compiler.BuildOptions{})
	foundWarn := false
	foundErr := false
	for _, d := range diags {
		if d.Code == "FDN0605" && d.Severity == compiler.SeverityWarning {
			foundWarn = true
		}
		if d.Code == "OTHER99" && d.Severity == compiler.SeverityError {
			foundErr = true
		}
	}
	if !foundErr {
		t.Fatalf("expected the plugin error diagnostic to surface, got: %+v", diags)
	}
	if !foundWarn {
		t.Fatalf("expected FDN0605 even when the out-of-prefix code is an error, got: %+v", diags)
	}
}
