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
