package diagnostics_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/diagnostics"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
)

// TestRegistry_AllCodesUnique confirms that all registered diagnostic codes are unique.
func TestRegistry_AllCodesUnique(t *testing.T) {
	seen := make(map[string]bool)
	for key, def := range diagnostics.Registry {
		if key != def.Code {
			t.Errorf("registry key %q does not match definition code %q", key, def.Code)
		}
		if seen[def.Code] {
			t.Errorf("duplicate diagnostic code registered: %s", def.Code)
		}
		seen[def.Code] = true
	}
}

// TestRegistry_CodeFormat confirms that all diagnostic codes match the required pattern.
func TestRegistry_CodeFormat(t *testing.T) {
	formatRe := regexp.MustCompile(`^FDN[0-9]{4}$`)
	for key, def := range diagnostics.Registry {
		if !formatRe.MatchString(key) {
			t.Errorf("registry key %q does not match format ^FDN[0-9]{4}$", key)
		}
		if !formatRe.MatchString(def.Code) {
			t.Errorf("definition code %q does not match format ^FDN[0-9]{4}$", def.Code)
		}
		if def.Severity != ir.SeverityError && def.Severity != ir.SeverityWarning && def.Severity != ir.SeverityInfo {
			t.Errorf("code %s has invalid severity: %v", key, def.Severity)
		}
		if def.Category == "" {
			t.Errorf("code %s has empty category", key)
		}
		if def.Description == "" {
			t.Errorf("code %s has empty description", key)
		}
		if def.DocAnchor == "" {
			t.Errorf("code %s has empty doc anchor", key)
		}
	}
}

// TestRegistry_CodebaseDrift confirms that all FDN codes in the codebase exist in the registry.
func TestRegistry_CodebaseDrift(t *testing.T) {
	root := findRepoRoot(t)
	pkgDir := filepath.Join(root, "pkg")
	codeRe := regexp.MustCompile(`\bFDN[0-9]{4}\b`)

	err := filepath.Walk(pkgDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		matches := codeRe.FindAllString(string(data), -1)
		for _, code := range matches {
			if _, ok := diagnostics.Registry[code]; !ok {
				t.Errorf("file %s references unregistered diagnostic code %s", path, code)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("failed to walk pkg directory: %v", err)
	}
}

func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current working directory: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find repository root containing go.mod")
		}
		dir = parent
	}
}

// TestLookup confirms that Lookup returns definitions for registered codes.
func TestLookup(t *testing.T) {
	def, ok := diagnostics.Lookup("FDN0001")
	if !ok {
		t.Fatal("expected FDN0001 to exist in registry")
	}
	if def.Code != "FDN0001" {
		t.Fatalf("expected code FDN0001, got: %s", def.Code)
	}

	missingCode := "FDN" + "9999"
	_, missing := diagnostics.Lookup(missingCode)
	if missing {
		t.Fatalf("expected %s not to exist in registry", missingCode)
	}
}

// TestNew confirms that New constructs diagnostics with templates and severities.
func TestNew(t *testing.T) {
	span := ir.Span{File: "test.fdn", Line: 10, Column: 5}
	diag := diagnostics.New("FDN0001", span, "permission denied")
	if diag.Code != "FDN0001" {
		t.Fatalf("expected code FDN0001, got: %s", diag.Code)
	}
	if diag.Severity != ir.SeverityError {
		t.Fatalf("expected severity error, got: %s", diag.Severity)
	}
	if !strings.Contains(diag.Message, "permission denied") {
		t.Fatalf("expected message to contain argument, got: %s", diag.Message)
	}

	unregistered := "FDN" + "9999"
	fallback := diagnostics.New(unregistered, span, "custom error")
	if fallback.Code != unregistered || fallback.Severity != ir.SeverityError {
		t.Fatalf("unexpected fallback diagnostic: %+v", fallback)
	}
	if fallback.Message != "custom error" {
		t.Fatalf("expected formatted fallback message, got: %s", fallback.Message)
	}
}

