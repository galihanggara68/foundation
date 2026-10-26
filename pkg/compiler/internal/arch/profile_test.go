package arch_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/arch"
)

func TestLoadBuildProfile_SemVerCompatibility(t *testing.T) {
	dir := t.TempDir()

	incompatibleYAML := `
project:
  name: test
  module: example.com/test

dependencies:
  runtime: "1.0.0"

targets:
  - name: server
    kind: backend
    language: go
    architecture: hexagonal
`
	if err := os.WriteFile(filepath.Join(dir, "foundation.build.yaml"), []byte(incompatibleYAML), 0644); err != nil {
		t.Fatalf("failed to write yaml: %v", err)
	}

	_, diags := arch.LoadBuildProfile(dir)
	if len(diags) == 0 {
		t.Fatalf("expected diagnostic for incompatible runtime 1.0.0, got 0")
	}
	if diags[0].Code != "FDN0007" {
		t.Fatalf("expected diagnostic code FDN0007, got %s", diags[0].Code)
	}

	// Compatible runtime ~0.1.0 should pass
	compatibleYAML := `
project:
  name: test
  module: example.com/test

dependencies:
  runtime: "^0.1.0"

targets:
  - name: server
    kind: backend
    language: go
    architecture: hexagonal
`
	if err := os.WriteFile(filepath.Join(dir, "foundation.build.yaml"), []byte(compatibleYAML), 0644); err != nil {
		t.Fatalf("failed to write yaml: %v", err)
	}

	bp, diags2 := arch.LoadBuildProfile(dir)
	if len(diags2) > 0 {
		t.Fatalf("expected compatible runtime to pass, got: %v", diags2)
	}
	if bp.Dependencies.Runtime != "^0.1.0" {
		t.Fatalf("expected bp.Dependencies.Runtime == ^0.1.0, got %s", bp.Dependencies.Runtime)
	}
}

func TestLoadBuildProfile_SDKTargetArchitectureDefault(t *testing.T) {
	dir := t.TempDir()

	sdkYAML := `
project:
  name: test
  module: example.com/test

targets:
  - name: go-client
    kind: sdk
    language: go
    out_dir: clients/go
  - name: ts-client
    kind: sdk
    language: typescript
    architecture: client
    out_dir: clients/ts
`
	if err := os.WriteFile(filepath.Join(dir, "foundation.build.yaml"), []byte(sdkYAML), 0644); err != nil {
		t.Fatalf("failed to write yaml: %v", err)
	}

	bp, diags := arch.LoadBuildProfile(dir)
	if len(diags) > 0 {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if len(bp.Targets) != 2 {
		t.Fatalf("expected 2 targets, got %d", len(bp.Targets))
	}
	if bp.Targets[0].Architecture != "sdk" {
		t.Errorf("expected go-client architecture to default to 'sdk', got %q", bp.Targets[0].Architecture)
	}
	if bp.Targets[1].Architecture != "sdk" {
		t.Errorf("expected ts-client architecture 'client' to normalize to 'sdk', got %q", bp.Targets[1].Architecture)
	}
}

func TestLoadBuildProfile_GeneratorPipelinesAndScaffold(t *testing.T) {
	dir := t.TempDir()

	yamlContent := `
project:
  name: shop
  module: example.com/shop

targets:
  - name: backend
    kind: backend
    language: go
    architecture: hexagonal
    http: chi
    out_dir: gen/go
    generators: [ "gen-rich-ddd", "gen-otel" ]
    scaffold: builtin
    test_command: "go test ./..."
    test_timeout: "45s"
    options:
      telemetry: "otel"
      strict_ddd: "true"

  - name: python-api
    kind: backend
    language: python
    architecture: custom
    out_dir: gen/py
    generator: "python"
    scaffold: none
    test_command: "pytest"
`
	if err := os.WriteFile(filepath.Join(dir, "foundation.build.yaml"), []byte(yamlContent), 0644); err != nil {
		t.Fatalf("failed to write yaml: %v", err)
	}

	bp, diags := arch.LoadBuildProfile(dir)
	if len(diags) > 0 {
		t.Fatalf("unexpected diagnostics: %+v", diags)
	}

	if len(bp.Targets) != 2 {
		t.Fatalf("expected 2 targets, got %d", len(bp.Targets))
	}

	t0 := bp.Targets[0]
	if len(t0.Generators) != 2 || t0.Generators[0] != "gen-rich-ddd" || t0.Generators[1] != "gen-otel" {
		t.Errorf("target 0 generators mismatch: %+v", t0.Generators)
	}
	if t0.EffectiveScaffold() != "builtin" {
		t.Errorf("expected scaffold 'builtin', got %q", t0.EffectiveScaffold())
	}
	if t0.TestCommand != "go test ./..." {
		t.Errorf("expected test command 'go test ./...', got %q", t0.TestCommand)
	}
	if t0.TestTimeout != "45s" {
		t.Errorf("expected test timeout '45s', got %q", t0.TestTimeout)
	}
	if t0.Options["telemetry"] != "otel" || t0.Options["strict_ddd"] != "true" {
		t.Errorf("options mismatch: %+v", t0.Options)
	}
	effGens0 := t0.EffectiveGenerators()
	if len(effGens0) != 2 || effGens0[0] != "gen-rich-ddd" {
		t.Errorf("expected 2 effective generators, got: %v", effGens0)
	}

	t1 := bp.Targets[1]
	if t1.Generator != "python" {
		t.Errorf("expected generator 'python', got %q", t1.Generator)
	}
	if t1.EffectiveScaffold() != "none" {
		t.Errorf("expected scaffold 'none', got %q", t1.EffectiveScaffold())
	}
	effGens1 := t1.EffectiveGenerators()
	if len(effGens1) != 1 || effGens1[0] != "python" {
		t.Errorf("expected 1 effective generator 'python', got: %v", effGens1)
	}
}
