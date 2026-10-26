package plugin_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/plugin"
)

func createExecutable(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("failed to create dir for executable %s: %v", path, err)
	}
	content := []byte("#!/bin/sh\nexit 0\n")
	if err := os.WriteFile(path, content, 0755); err != nil {
		t.Fatalf("failed to write executable %s: %v", path, err)
	}
}

func TestResolve_Tier1_DirectPath(t *testing.T) {
	tmpDir := t.TempDir()

	// Relative direct path
	relPath := filepath.Join(tmpDir, "tools", "my-gen")
	createExecutable(t, relPath)

	resolved, err := plugin.Resolve(tmpDir, "./tools/my-gen")
	if err != nil {
		t.Fatalf("expected successful resolution of direct path, got: %v", err)
	}
	if resolved != relPath {
		t.Errorf("expected %q, got %q", relPath, resolved)
	}

	// Absolute direct path
	resolvedAbs, err := plugin.Resolve(tmpDir, relPath)
	if err != nil {
		t.Fatalf("expected successful resolution of absolute path, got: %v", err)
	}
	if resolvedAbs != relPath {
		t.Errorf("expected %q, got %q", relPath, resolvedAbs)
	}

	// Direct path missing
	if _, err := plugin.Resolve(tmpDir, "./tools/missing"); err == nil {
		t.Fatal("expected error resolving non-existent direct path")
	}

	// Direct path not executable
	nonExec := filepath.Join(tmpDir, "tools", "non-exec")
	if err := os.WriteFile(nonExec, []byte("echo hi"), 0644); err != nil {
		t.Fatalf("failed to create non-executable file: %v", err)
	}
	if _, err := plugin.Resolve(tmpDir, "./tools/non-exec"); err == nil {
		t.Fatal("expected error resolving non-executable file")
	}
}

func TestResolve_Tier2_WorkspaceLocal(t *testing.T) {
	tmpDir := t.TempDir()

	// 1. node_modules/.bin/foundation-gen-<name>
	nodeBinPath := filepath.Join(tmpDir, "node_modules", ".bin", "foundation-gen-ts")
	createExecutable(t, nodeBinPath)

	resolved, err := plugin.Resolve(tmpDir, "ts")
	if err != nil {
		t.Fatalf("expected resolution in node_modules/.bin, got: %v", err)
	}
	if resolved != nodeBinPath {
		t.Errorf("expected %q, got %q", nodeBinPath, resolved)
	}

	// Also resolves if user provides the full foundation-gen- prefix
	resolvedFull, err := plugin.Resolve(tmpDir, "foundation-gen-ts")
	if err != nil {
		t.Fatalf("expected resolution with prefix, got: %v", err)
	}
	if resolvedFull != nodeBinPath {
		t.Errorf("expected %q, got %q", nodeBinPath, resolvedFull)
	}

	// 2. bin/foundation-gen-<name>
	tmpDir2 := t.TempDir()
	localBinPath := filepath.Join(tmpDir2, "bin", "foundation-gen-go")
	createExecutable(t, localBinPath)

	resolved2, err := plugin.Resolve(tmpDir2, "go")
	if err != nil {
		t.Fatalf("expected resolution in ./bin, got: %v", err)
	}
	if resolved2 != localBinPath {
		t.Errorf("expected %q, got %q", localBinPath, resolved2)
	}
}

func TestResolve_Tier3_SystemPath(t *testing.T) {
	tmpDir := t.TempDir()
	binDir := filepath.Join(tmpDir, "sysbin")
	sysGen := filepath.Join(binDir, "foundation-gen-rust")
	createExecutable(t, sysGen)

	// Prepend binDir to PATH
	origPath := os.Getenv("PATH")
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+origPath)

	workspaceDir := t.TempDir()
	resolved, err := plugin.Resolve(workspaceDir, "rust")
	if err != nil {
		t.Fatalf("expected resolution in system PATH, got: %v", err)
	}
	if resolved != sysGen {
		t.Errorf("expected %q, got %q", sysGen, resolved)
	}
}

func TestResolve_NotFound(t *testing.T) {
	workspaceDir := t.TempDir()
	if _, err := plugin.Resolve(workspaceDir, "nonexistent-generator"); err == nil {
		t.Fatal("expected error resolving non-existent generator")
	}

	// Empty name
	if _, err := plugin.Resolve(workspaceDir, "   "); err == nil {
		t.Fatal("expected error resolving empty plugin name")
	}

	// Directory path instead of file
	dirPath := filepath.Join(workspaceDir, "somedir")
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}
	if _, err := plugin.Resolve(workspaceDir, dirPath); err == nil {
		t.Fatal("expected error resolving directory as executable")
	}
}
