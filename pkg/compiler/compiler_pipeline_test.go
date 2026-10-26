package compiler_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/galihanggara68/foundation/pkg/compiler"
)

func createExecutablePlugin(t *testing.T, path, scriptContent string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("failed to create dir for %s: %v", path, err)
	}
	content := []byte("#!/bin/sh\n" + scriptContent)
	if err := os.WriteFile(path, content, 0755); err != nil {
		t.Fatalf("failed to write script %s: %v", path, err)
	}
}

func setupTestWorkspace(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	blueprint := `
entity Order {
  id: uuid @pk
  item: string
}

@http(method: POST, path: "/orders")
action CreateOrder(item: string) -> Order {
  let order = create Order {
    item: item
  }
  return order
}
`
	if err := os.WriteFile(filepath.Join(dir, "shop.fdn"), []byte(blueprint), 0644); err != nil {
		t.Fatalf("failed to write blueprint: %v", err)
	}
	return dir
}

func TestBuild_Pipeline_CleanSlateScaffoldNone(t *testing.T) {
	dir := setupTestWorkspace(t)
	genScript := filepath.Join(dir, "tools", "gen-py")
	createExecutablePlugin(t, genScript, `
cat > /dev/null
cat << 'EOF'
{
  "files": {
    "app/main.py": "print('hello from python')",
    "app/models.py": "class Order: pass"
  },
  "test_command": "pytest"
}
EOF
exit 0
`)

	buildYAML := `
project:
  name: shop
  module: example.com/shop

targets:
  - name: python-api
    kind: backend
    language: python
    architecture: custom
    out_dir: gen/py
    generator: ./tools/gen-py
    scaffold: none
`
	if err := os.WriteFile(filepath.Join(dir, "foundation.build.yaml"), []byte(buildYAML), 0644); err != nil {
		t.Fatalf("failed to write yaml: %v", err)
	}

	res, diags := compiler.Build(dir, compiler.BuildOptions{})
	if len(diags) > 0 {
		t.Fatalf("unexpected diagnostics: %+v", diags)
	}
	if res == nil {
		t.Fatal("expected non-nil BuildResult")
	}

	mainPy := filepath.Join(dir, "gen", "py", "app", "main.py")
	modelsPy := filepath.Join(dir, "gen", "py", "app", "models.py")
	manifestPath := filepath.Join(dir, "gen", "py", ".foundation-manifest.json")

	if _, err := os.Stat(mainPy); err != nil {
		t.Fatalf("expected main.py to exist: %v", err)
	}
	if _, err := os.Stat(modelsPy); err != nil {
		t.Fatalf("expected models.py to exist: %v", err)
	}
	if _, err := os.Stat(manifestPath); err != nil {
		t.Fatalf("expected manifest to exist: %v", err)
	}
}

func TestBuild_Pipeline_ScaffoldBuiltin_PartialOverride(t *testing.T) {
	dir := setupTestWorkspace(t)
	genScript := filepath.Join(dir, "tools", "gen-rich-ddd")
	createExecutablePlugin(t, genScript, `
cat > /dev/null
cat << 'EOF'
{
  "files": {
    "domain/order.go": "// rich domain model: encapsulated invariants"
  }
}
EOF
exit 0
`)

	buildYAML := `
project:
  name: shop
  module: example.com/shop

storage:
  dialect: postgres
  mode: hybrid

targets:
  - name: backend
    kind: backend
    language: go
    architecture: hexagonal
    http: chi
    out_dir: gen/go
    generators: [ "./tools/gen-rich-ddd" ]
    scaffold: builtin
`
	if err := os.WriteFile(filepath.Join(dir, "foundation.build.yaml"), []byte(buildYAML), 0644); err != nil {
		t.Fatalf("failed to write yaml: %v", err)
	}

	res, diags := compiler.Build(dir, compiler.BuildOptions{})
	if len(diags) > 0 {
		t.Fatalf("unexpected diagnostics: %+v", diags)
	}
	if res == nil {
		t.Fatal("expected non-nil BuildResult")
	}

	orderGo := filepath.Join(dir, "gen", "go", "domain", "order.go")
	orderContent, err := os.ReadFile(orderGo)
	if err != nil {
		t.Fatalf("expected domain/order.go to exist: %v", err)
	}
	if !strings.Contains(string(orderContent), "rich domain model") {
		t.Errorf("expected rich domain model override, got: %s", string(orderContent))
	}

	// Verify standard plumbing file was preserved from built-in scaffolding
	goMod := filepath.Join(dir, "gen", "go", "go.mod")
	if _, err := os.Stat(goMod); err != nil {
		t.Fatalf("expected go.mod from built-in scaffolding to exist: %v", err)
	}
}

func TestBuild_Pipeline_ChainedGenerators(t *testing.T) {
	dir := setupTestWorkspace(t)
	gen1 := filepath.Join(dir, "tools", "gen-1")
	createExecutablePlugin(t, gen1, `
cat > /dev/null
cat << 'EOF'
{
  "files": {
    "gen1_output.txt": "created by gen1"
  }
}
EOF
exit 0
`)

	gen2 := filepath.Join(dir, "tools", "gen-2")
	createExecutablePlugin(t, gen2, `
# Inspect stdin to ensure gen1_output.txt was chained into files
INPUT=$(cat)
if ! echo "$INPUT" | grep -q "created by gen1"; then
  echo "gen1_output.txt not found in chained files" >&2
  exit 1
fi
cat << 'EOF'
{
  "files": {
    "gen2_output.txt": "created by gen2"
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
  - name: chained
    kind: backend
    language: go
    architecture: custom
    out_dir: gen/chained
    generators:
      - ./tools/gen-1
      - ./tools/gen-2
    scaffold: none
`
	if err := os.WriteFile(filepath.Join(dir, "foundation.build.yaml"), []byte(buildYAML), 0644); err != nil {
		t.Fatalf("failed to write yaml: %v", err)
	}

	_, diags := compiler.Build(dir, compiler.BuildOptions{})
	if len(diags) > 0 {
		t.Fatalf("unexpected diagnostics: %+v", diags)
	}

	f1 := filepath.Join(dir, "gen", "chained", "gen1_output.txt")
	f2 := filepath.Join(dir, "gen", "chained", "gen2_output.txt")
	if _, err := os.Stat(f1); err != nil {
		t.Fatalf("expected gen1_output.txt to exist: %v", err)
	}
	if _, err := os.Stat(f2); err != nil {
		t.Fatalf("expected gen2_output.txt to exist: %v", err)
	}
}

func TestBuild_Pipeline_StubProtection(t *testing.T) {
	dir := setupTestWorkspace(t)
	genScript := filepath.Join(dir, "tools", "gen-stubs")
	createExecutablePlugin(t, genScript, `
cat > /dev/null
cat << 'EOF'
{
  "files": {
    "main.go": "package main"
  },
  "stubs": {
    "slots/payment.go": "// initial generated stub"
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
  - name: stub-test
    kind: backend
    language: go
    architecture: custom
    out_dir: gen/stubs
    generator: ./tools/gen-stubs
    scaffold: none
`
	if err := os.WriteFile(filepath.Join(dir, "foundation.build.yaml"), []byte(buildYAML), 0644); err != nil {
		t.Fatalf("failed to write yaml: %v", err)
	}

	// 1. Initial build writes stub
	_, diags1 := compiler.Build(dir, compiler.BuildOptions{})
	if len(diags1) > 0 {
		t.Fatalf("build 1 failed: %+v", diags1)
	}

	stubFile := filepath.Join(dir, "gen", "stubs", "slots", "payment.go")
	if _, err := os.Stat(stubFile); err != nil {
		t.Fatalf("stub file not created: %v", err)
	}

	// 2. User edits stub file
	customContent := "// handwritten custom payment logic"
	if err := os.WriteFile(stubFile, []byte(customContent), 0644); err != nil {
		t.Fatalf("failed to write custom content: %v", err)
	}

	// 3. Second build: stub must NOT be overwritten
	_, diags2 := compiler.Build(dir, compiler.BuildOptions{})
	if len(diags2) > 0 {
		t.Fatalf("build 2 failed: %+v", diags2)
	}

	afterBuild, err := os.ReadFile(stubFile)
	if err != nil {
		t.Fatalf("failed to read stub after build: %v", err)
	}
	if string(afterBuild) != customContent {
		t.Fatalf("stub file was overwritten!\nExpected: %s\nGot: %s", customContent, string(afterBuild))
	}
}

func TestBuild_Pipeline_PluginDiagnosticsSurfaced(t *testing.T) {
	dir := setupTestWorkspace(t)
	genScript := filepath.Join(dir, "tools", "gen-diag")
	createExecutablePlugin(t, genScript, `
cat > /dev/null
cat << 'EOF'
{
  "diagnostics": [
    {
      "code": "CUSTOM01",
      "severity": "error",
      "message": "custom invariant violated",
      "span": {
        "file": "shop.fdn",
        "line": 10,
        "column": 5
      }
    }
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
  - name: diag-test
    kind: backend
    language: go
    architecture: custom
    out_dir: gen/diag
    generator: ./tools/gen-diag
    scaffold: none
`
	if err := os.WriteFile(filepath.Join(dir, "foundation.build.yaml"), []byte(buildYAML), 0644); err != nil {
		t.Fatalf("failed to write yaml: %v", err)
	}

	_, diags := compiler.Build(dir, compiler.BuildOptions{})
	found := false
	for _, d := range diags {
		if d.Code == "CUSTOM01" && d.Severity == compiler.SeverityError {
			found = true
			if d.Span.Line != 10 || d.Span.Column != 5 {
				t.Errorf("diagnostic span mismatch: %+v", d.Span)
			}
		}
	}
	if !found {
		t.Fatalf("expected CUSTOM01 diagnostic surfaced by compiler: %+v", diags)
	}
}
