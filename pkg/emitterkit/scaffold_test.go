package emitterkit

import (
	"path/filepath"
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

func TestSafeRelPath(t *testing.T) {
	rootDir := "/workspace/project"

	tests := []struct {
		name        string
		rel         string
		expected    string
		expectError bool
	}{
		{
			name:        "simple filename",
			rel:         "file.txt",
			expected:    filepath.Join(rootDir, "file.txt"),
			expectError: false,
		},
		{
			name:        "nested path with forward slashes",
			rel:         "domain/order.go",
			expected:    filepath.Join(rootDir, "domain", "order.go"),
			expectError: false,
		},
		{
			name:        "deeply nested path",
			rel:         "internal/app/service/order/handler.go",
			expected:    filepath.Join(rootDir, "internal", "app", "service", "order", "handler.go"),
			expectError: false,
		},
		{
			name:        "nested path with leading dot slash",
			rel:         "./domain/order.go",
			expected:    filepath.Join(rootDir, "domain", "order.go"),
			expectError: false,
		},
		{
			name:        "windows style separators",
			rel:         "domain\\order.go",
			expected:    filepath.Join(rootDir, "domain", "order.go"),
			expectError: false,
		},
		{
			name:        "windows deeply nested path",
			rel:         "internal\\domain\\sub\\file.go",
			expected:    filepath.Join(rootDir, "internal", "domain", "sub", "file.go"),
			expectError: false,
		},
		{
			name:        "empty string",
			rel:         "",
			expectError: true,
		},
		{
			name:        "whitespace only",
			rel:         "   ",
			expectError: true,
		},
		{
			name:        "dot root",
			rel:         ".",
			expectError: true,
		},
		{
			name:        "dot slash root",
			rel:         "./",
			expectError: true,
		},
		{
			name:        "posix root path",
			rel:         "/root",
			expectError: true,
		},
		{
			name:        "posix absolute etc passwd",
			rel:         "/etc/passwd",
			expectError: true,
		},
		{
			name:        "windows drive with backslash",
			rel:         "C:\\root",
			expectError: true,
		},
		{
			name:        "windows drive with forward slash",
			rel:         "C:/root",
			expectError: true,
		},
		{
			name:        "windows lowercase drive",
			rel:         "d:\\nested\\file.go",
			expectError: true,
		},
		{
			name:        "windows unc path",
			rel:         "\\\\server\\share\\file.txt",
			expectError: true,
		},
		{
			name:        "backslash root",
			rel:         "\\root",
			expectError: true,
		},
		{
			name:        "simple parent traversal",
			rel:         "..",
			expectError: true,
		},
		{
			name:        "parent traversal with etc passwd",
			rel:         "../../etc/passwd",
			expectError: true,
		},
		{
			name:        "nested traversal escaping root",
			rel:         "sub/../../escaped",
			expectError: true,
		},
		{
			name:        "windows traversal escaping root",
			rel:         "sub\\..\\..\\escaped",
			expectError: true,
		},
		{
			name:        "deep traversal escaping root",
			rel:         "a/b/c/../../../../escaped",
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := SafeRelPath(rootDir, tc.rel)
			if tc.expectError {
				if err == nil {
					t.Fatalf("expected error for rel path %q, but got nil (path: %s)", tc.rel, got)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error for rel path %q: %v", tc.rel, err)
				}
				if got != tc.expected {
					t.Fatalf("expected %q, got %q", tc.expected, got)
				}
			}
		})
	}
}

func TestSafeRelPath_RelativeRootDir(t *testing.T) {
	rootDir := "dist/backend"
	got, err := SafeRelPath(rootDir, "internal/domain/entity.go")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := filepath.Join("dist/backend", "internal", "domain", "entity.go")
	if got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}
