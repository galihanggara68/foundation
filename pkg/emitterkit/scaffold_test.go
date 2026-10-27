package emitterkit

import (
	"testing"

	"github.com/galihanggara68/foundation/pkg/compiler"
)

func TestEmitGoHexagonalScaffolding_NilArchIR(t *testing.T) {
	emission, diags := EmitGoHexagonalScaffolding(nil)
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d: %+v", len(diags), diags)
	}
	if diags[0].Code != "FDN0300" {
		t.Errorf("expected diagnostic FDN0300, got: %s", diags[0].Code)
	}
	if len(emission.TrackedFiles) != 0 {
		t.Errorf("expected 0 files emitted, got %d", len(emission.TrackedFiles))
	}
}

func TestEmitGoHexagonalScaffolding_ValidArchIR(t *testing.T) {
	archIR := &compiler.ArchitectureIR{
		TargetName: "backend",
		Kind:       "backend",
		Language:   "go",
		Pattern:    "hexagonal",
		Module:     "example.com/acme",
		OutDir:     "dist/backend",
	}

	emission, diags := EmitGoHexagonalScaffolding(archIR)
	if len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %+v", diags)
	}

	files, stubs := ExtractFiles(emission)
	if len(files) == 0 {
		t.Fatal("expected non-empty files emitted")
	}

	// Verify go.mod was emitted
	goMod, ok := files["go.mod"]
	if !ok {
		t.Fatal("expected go.mod in emitted files")
	}
	if len(goMod) == 0 {
		t.Error("expected non-empty go.mod")
	}

	_ = stubs
}

func TestExtractFiles(t *testing.T) {
	emission := compiler.Emission{
		TrackedFiles: map[string][]byte{
			"a.txt": []byte("hello"),
		},
		StubFiles: map[string][]byte{
			"b.txt": []byte("starter"),
		},
	}

	files, stubs := ExtractFiles(emission)
	if files["a.txt"] != "hello" {
		t.Errorf("expected 'hello', got %q", files["a.txt"])
	}
	if stubs["b.txt"] != "starter" {
		t.Errorf("expected 'starter', got %q", stubs["b.txt"])
	}
}

func TestEmitGoDomain(t *testing.T) {
	archIR := &compiler.ArchitectureIR{
		TargetName: "backend",
		Kind:       "backend",
		Language:   "go",
		Pattern:    "hexagonal",
		Module:     "example.com/acme",
		OutDir:     "dist/backend",
	}

	domainFiles, diags := EmitGoDomain(archIR)
	if len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %+v", diags)
	}
	for path := range domainFiles {
		if len(path) < 16 || path[:16] != "internal/domain/" {
			t.Errorf("expected file in internal/domain/, got %s", path)
		}
	}
}
