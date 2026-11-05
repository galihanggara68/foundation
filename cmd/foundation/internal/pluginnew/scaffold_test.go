package pluginnew_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/galihanggara68/foundation/cmd/foundation/internal/pluginnew"
)

func TestScaffold_MissingPluginName_ReturnsError(t *testing.T) {
	err := pluginnew.Scaffold(pluginnew.ScaffoldOptions{
		PluginName: "",
	})
	if err == nil {
		t.Fatal("expected error when plugin name is empty")
	}
}

func TestScaffold_DefaultOptions_CreatesExpectedFiles(t *testing.T) {
	tmpDir := t.TempDir()
	targetDir := filepath.Join(tmpDir, "sample-plugin")

	opts := pluginnew.ScaffoldOptions{
		PluginName: "sample-plugin",
		TargetDir:  targetDir,
	}

	if err := pluginnew.Scaffold(opts); err != nil {
		t.Fatalf("scaffold failed: %v", err)
	}

	expectedFiles := []string{
		"go.mod",
		"main.go",
		"dump_model.go",
		".golangci.yml",
		".gitignore",
		"Makefile",
		"golden_test.go",
		"determinism_test.go",
		"smoke_test.go",
		"testdata/blueprints/sample.fdn",
		"testdata/requests/sample.json",
		"testdata/models/sample.json",
		"testdata/golden/go.mod",
		"testdata/golden/sample.go",
		"testdata/golden/sample_test.go",
	}

	for _, rel := range expectedFiles {
		fullPath := filepath.Join(targetDir, rel)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			t.Errorf("expected file %s does not exist", rel)
		}
	}

	goModBytes, err := os.ReadFile(filepath.Join(targetDir, "go.mod"))
	if err != nil {
		t.Fatalf("read go.mod: %v", err)
	}
	goModContent := string(goModBytes)
	if !strings.Contains(goModContent, "module sample-plugin") {
		t.Errorf("expected module declaration in go.mod, got: %s", goModContent)
	}
	if !strings.Contains(goModContent, "require github.com/galihanggara68/foundation v0.1.0") {
		t.Errorf("expected foundation dependency in go.mod, got: %s", goModContent)
	}

	mainBytes, err := os.ReadFile(filepath.Join(targetDir, "main.go"))
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}
	if !strings.Contains(string(mainBytes), "emitterkit.RunPluginWithFlags") {
		t.Errorf("expected emitterkit.RunPluginWithFlags in main.go")
	}

	lintBytes, err := os.ReadFile(filepath.Join(targetDir, ".golangci.yml"))
	if err != nil {
		t.Fatalf("read .golangci.yml: %v", err)
	}
	if !strings.Contains(string(lintBytes), "forbidigo") {
		t.Errorf("expected forbidigo rule in .golangci.yml")
	}
}

func TestScaffold_WithCustomModuleAndReplace(t *testing.T) {
	tmpDir := t.TempDir()
	targetDir := filepath.Join(tmpDir, "custom-plugin")

	opts := pluginnew.ScaffoldOptions{
		PluginName:        "custom-plugin",
		TargetDir:         targetDir,
		ModulePath:        "example.com/custom-plugin",
		FoundationVersion: "v0.2.0",
		ReplacePath:       "/local/foundation",
	}

	if err := pluginnew.Scaffold(opts); err != nil {
		t.Fatalf("scaffold failed: %v", err)
	}

	goModBytes, err := os.ReadFile(filepath.Join(targetDir, "go.mod"))
	if err != nil {
		t.Fatalf("read go.mod: %v", err)
	}
	goModContent := string(goModBytes)
	if !strings.Contains(goModContent, "module example.com/custom-plugin") {
		t.Errorf("expected custom module path in go.mod, got: %s", goModContent)
	}
	if !strings.Contains(goModContent, "require github.com/galihanggara68/foundation v0.2.0") {
		t.Errorf("expected foundation version v0.2.0, got: %s", goModContent)
	}
	if !strings.Contains(goModContent, "replace github.com/galihanggara68/foundation => /local/foundation") {
		t.Errorf("expected replace directive in go.mod, got: %s", goModContent)
	}
}
