package e2e_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// buildFoundationCLI compiles the foundation CLI into a temporary directory and returns its path.
func buildFoundationCLI(t *testing.T) string {
	t.Helper()

	repoRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}

	binPath := filepath.Join(t.TempDir(), "foundation")
	buildCmd := exec.Command("go", "build", "-o", binPath, "./cmd/foundation")
	buildCmd.Dir = repoRoot
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build foundation CLI: %v\noutput: %s", err, string(out))
	}
	return binPath
}

// TestE2E_PluginNew_GeneratedProjectBuildsAndPassesTests runs the plugin new journey end to end.
func TestE2E_PluginNew_GeneratedProjectBuildsAndPassesTests(t *testing.T) {
	repoRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}

	binPath := buildFoundationCLI(t)
	outDir := t.TempDir()

	cmd := exec.Command(
		binPath, "plugin", "new", "e2e-emitter",
		"--dir", outDir,
		"--module", "example.com/e2e-emitter",
		"--replace", repoRoot,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("plugin new failed: %v\noutput: %s", err, string(out))
	}

	pluginDir := filepath.Join(outDir, "e2e-emitter")
	if _, err := os.Stat(filepath.Join(pluginDir, "main.go")); err != nil {
		t.Fatalf("expected generated main.go: %v", err)
	}

	tidyCmd := exec.Command("go", "mod", "tidy")
	tidyCmd.Dir = pluginDir
	_ = tidyCmd.Run()

	testCmd := exec.Command("go", "test", "./...")
	testCmd.Dir = pluginDir
	if out, err := testCmd.CombinedOutput(); err != nil {
		t.Fatalf("generated plugin tests failed: %v\noutput: %s", err, string(out))
	}

	shortCmd := exec.Command("go", "test", "-short", "./...")
	shortCmd.Dir = pluginDir
	if out, err := shortCmd.CombinedOutput(); err != nil {
		t.Fatalf("generated plugin short tests failed: %v\noutput: %s", err, string(out))
	}
}
