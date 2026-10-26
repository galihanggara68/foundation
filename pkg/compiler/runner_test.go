package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCompilerTest_ExternalCommand_SuccessAndFailure(t *testing.T) {
	tmpDir := t.TempDir()

	// Create output dir
	outDir := filepath.Join(tmpDir, "out")
	if err := os.MkdirAll(outDir, 0755); err != nil {
		t.Fatalf("failed to create out dir: %v", err)
	}

	buildYAML := `schema_version: "0.1.0"
project:
  name: test-project
  module: example.com/test
targets:
  - name: py-backend
    kind: backend
    language: python
    out_dir: out
    test_command: "echo 'hello from python test' && exit 0"
`
	if err := os.WriteFile(filepath.Join(tmpDir, "foundation.build.yaml"), []byte(buildYAML), 0644); err != nil {
		t.Fatalf("failed to write build yaml: %v", err)
	}

	// 1. Success case
	diags := Test(tmpDir)
	if len(diags) != 0 {
		t.Fatalf("expected 0 diagnostics for passing test command, got: %+v", diags)
	}

	// 2. Failure case
	failYAML := `schema_version: "0.1.0"
project:
  name: test-project
  module: example.com/test
targets:
  - name: py-backend
    kind: backend
    language: python
    out_dir: out
    test_command: "echo 'assertion failed: 1 != 2' >&2 && exit 1"
`
	if err := os.WriteFile(filepath.Join(tmpDir, "foundation.build.yaml"), []byte(failYAML), 0644); err != nil {
		t.Fatalf("failed to write build yaml: %v", err)
	}

	failDiags := Test(tmpDir)
	if len(failDiags) != 1 {
		t.Fatalf("expected 1 diagnostic for failing test command, got %d: %+v", len(failDiags), failDiags)
	}
	if failDiags[0].Code != "FDN0600" {
		t.Errorf("expected diagnostic code FDN0600, got: %s", failDiags[0].Code)
	}
	if failDiags[0].Severity != SeverityError {
		t.Errorf("expected SeverityError, got: %s", failDiags[0].Severity)
	}
	if !strings.Contains(failDiags[0].Message, "assertion failed: 1 != 2") {
		t.Errorf("expected diagnostic message to capture stderr, got: %s", failDiags[0].Message)
	}
}

func TestCompilerTest_ExternalCommand_TimeoutTermination_FDN0602(t *testing.T) {
	tmpDir := t.TempDir()

	outDir := filepath.Join(tmpDir, "out")
	if err := os.MkdirAll(outDir, 0755); err != nil {
		t.Fatalf("failed to create out dir: %v", err)
	}

	buildYAML := `schema_version: "0.1.0"
project:
  name: test-project
  module: example.com/test
targets:
  - name: hanging-backend
    kind: backend
    language: python
    out_dir: out
    test_command: "sleep 5"
    test_timeout: "100ms"
`
	if err := os.WriteFile(filepath.Join(tmpDir, "foundation.build.yaml"), []byte(buildYAML), 0644); err != nil {
		t.Fatalf("failed to write build yaml: %v", err)
	}

	start := time.Now()
	diags := Test(tmpDir)
	elapsed := time.Since(start)

	if elapsed > 2*time.Second {
		t.Errorf("expected timeout to terminate within ~100ms, took %s", elapsed)
	}

	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic for timed out test, got %d: %+v", len(diags), diags)
	}
	if diags[0].Code != "FDN0602" {
		t.Errorf("expected diagnostic code FDN0602, got: %s", diags[0].Code)
	}
	if diags[0].Severity != SeverityError {
		t.Errorf("expected SeverityError, got: %s", diags[0].Severity)
	}
	if !strings.Contains(diags[0].Message, "timed out after 100ms") {
		t.Errorf("expected diagnostic message to mention timeout duration, got: %s", diags[0].Message)
	}
}

func TestCompilerTest_HybridResolution_ManifestDefaultAndYAMLOverride(t *testing.T) {
	tmpDir := t.TempDir()
	outDir := filepath.Join(tmpDir, "out")
	if err := os.MkdirAll(outDir, 0755); err != nil {
		t.Fatalf("failed to create out dir: %v", err)
	}

	// Write .foundation-manifest.json with plugin's default test_command
	manifestJSON := `{
  "version": 1,
  "test_command": "echo 'plugin-default' > manifest_result.txt",
  "files": {}
}`
	if err := os.WriteFile(filepath.Join(outDir, ".foundation-manifest.json"), []byte(manifestJSON), 0644); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}

	// 1. Without YAML test_command: uses manifest test_command
	buildYAMLDefault := `schema_version: "0.1.0"
project:
  name: test-project
  module: example.com/test
targets:
  - name: plugin-target
    kind: backend
    language: customlang
    out_dir: out
`
	if err := os.WriteFile(filepath.Join(tmpDir, "foundation.build.yaml"), []byte(buildYAMLDefault), 0644); err != nil {
		t.Fatalf("failed to write build yaml: %v", err)
	}

	diags := Test(tmpDir)
	if len(diags) != 0 {
		t.Fatalf("expected 0 diagnostics, got: %+v", diags)
	}

	resultBytes, err := os.ReadFile(filepath.Join(outDir, "manifest_result.txt"))
	if err != nil {
		t.Fatalf("expected manifest_result.txt to exist: %v", err)
	}
	if !strings.Contains(string(resultBytes), "plugin-default") {
		t.Errorf("expected 'plugin-default', got: %s", string(resultBytes))
	}

	// 2. With YAML test_command: YAML overrides manifest test_command
	buildYAMLOverride := `schema_version: "0.1.0"
project:
  name: test-project
  module: example.com/test
targets:
  - name: plugin-target
    kind: backend
    language: customlang
    out_dir: out
    test_command: "echo 'yaml-override' > override_result.txt"
`
	if err := os.WriteFile(filepath.Join(tmpDir, "foundation.build.yaml"), []byte(buildYAMLOverride), 0644); err != nil {
		t.Fatalf("failed to write build yaml: %v", err)
	}

	diagsOverride := Test(tmpDir)
	if len(diagsOverride) != 0 {
		t.Fatalf("expected 0 diagnostics, got: %+v", diagsOverride)
	}

	overrideBytes, err := os.ReadFile(filepath.Join(outDir, "override_result.txt"))
	if err != nil {
		t.Fatalf("expected override_result.txt to exist: %v", err)
	}
	if !strings.Contains(string(overrideBytes), "yaml-override") {
		t.Errorf("expected 'yaml-override', got: %s", string(overrideBytes))
	}
}

func TestCompilerTest_NoRunnerOrCommand_ReturnsFDN0601(t *testing.T) {
	tmpDir := t.TempDir()
	outDir := filepath.Join(tmpDir, "out")
	if err := os.MkdirAll(outDir, 0755); err != nil {
		t.Fatalf("failed to create out dir: %v", err)
	}

	buildYAML := `schema_version: "0.1.0"
project:
  name: test-project
  module: example.com/test
targets:
  - name: custom-unknown
    kind: backend
    language: elixir
    out_dir: out
`
	if err := os.WriteFile(filepath.Join(tmpDir, "foundation.build.yaml"), []byte(buildYAML), 0644); err != nil {
		t.Fatalf("failed to write build yaml: %v", err)
	}

	diags := Test(tmpDir)
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d: %+v", len(diags), diags)
	}
	if diags[0].Code != "FDN0601" {
		t.Errorf("expected FDN0601, got: %s", diags[0].Code)
	}
}

func TestCompilerTest_InvalidTestTimeout_ReturnsFDN0004(t *testing.T) {
	tmpDir := t.TempDir()

	buildYAML := `schema_version: "0.1.0"
project:
  name: test-project
  module: example.com/test
targets:
  - name: bad-timeout
    kind: backend
    language: go
    out_dir: out
    test_timeout: "not-a-valid-duration"
`
	if err := os.WriteFile(filepath.Join(tmpDir, "foundation.build.yaml"), []byte(buildYAML), 0644); err != nil {
		t.Fatalf("failed to write build yaml: %v", err)
	}

	diags := Test(tmpDir)
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic for invalid test_timeout, got %d: %+v", len(diags), diags)
	}
	if diags[0].Code != "FDN0004" {
		t.Errorf("expected FDN0004, got: %s", diags[0].Code)
	}
	if !strings.Contains(diags[0].Message, "invalid test_timeout") {
		t.Errorf("expected diagnostic message to mention invalid test_timeout, got: %s", diags[0].Message)
	}

	// Non-positive duration
	buildYAMLZero := `schema_version: "0.1.0"
project:
  name: test-project
  module: example.com/test
targets:
  - name: zero-timeout
    kind: backend
    language: go
    out_dir: out
    test_timeout: "0s"
`
	if err := os.WriteFile(filepath.Join(tmpDir, "foundation.build.yaml"), []byte(buildYAMLZero), 0644); err != nil {
		t.Fatalf("failed to write build yaml: %v", err)
	}

	diagsZero := Test(tmpDir)
	if len(diagsZero) != 1 || diagsZero[0].Code != "FDN0004" {
		t.Fatalf("expected FDN0004 for 0s timeout, got: %+v", diagsZero)
	}
}

func TestBuildAndTest_PluginTestCommandE2E(t *testing.T) {
	tmpDir := t.TempDir()

	binDir := filepath.Join(tmpDir, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatalf("failed to create bin dir: %v", err)
	}

	mockPlugin := filepath.Join(binDir, "foundation-gen-py")
	pluginScript := `#!/bin/sh
cat > /dev/null
cat << 'EOF'
{
  "protocol_version": 1,
  "files": {
    "app.py": "print('hello from app')\n"
  },
  "test_command": "python3 -c \"print('mock tests pass')\""
}
EOF
`
	if err := os.WriteFile(mockPlugin, []byte(pluginScript), 0755); err != nil {
		t.Fatalf("failed to write mock plugin: %v", err)
	}

	fdnSrc := `entity Invoice {
    id: string
}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "app.fdn"), []byte(fdnSrc), 0644); err != nil {
		t.Fatalf("failed to write fdn source: %v", err)
	}

	buildYAML := `schema_version: "0.1.0"
project:
  name: py-app
  module: example.com/pyapp
targets:
  - name: py-backend
    kind: backend
    language: python
    generator: ./bin/foundation-gen-py
    scaffold: none
    out_dir: dist/backend
`
	if err := os.WriteFile(filepath.Join(tmpDir, "foundation.build.yaml"), []byte(buildYAML), 0644); err != nil {
		t.Fatalf("failed to write build yaml: %v", err)
	}

	// 1. Build project
	res, buildDiags := Build(tmpDir, BuildOptions{})
	if len(buildDiags) > 0 {
		t.Fatalf("build failed: %+v", buildDiags)
	}
	if res == nil {
		t.Fatal("expected non-nil build result")
	}

	// Verify .foundation-manifest.json recorded test_command
	manifestBytes, err := os.ReadFile(filepath.Join(tmpDir, "dist", "backend", ".foundation-manifest.json"))
	if err != nil {
		t.Fatalf("failed to read manifest: %v", err)
	}
	if !strings.Contains(string(manifestBytes), `"test_command": "python3 -c \"print('mock tests pass')\""`) {
		t.Errorf("manifest missing test_command, got: %s", string(manifestBytes))
	}

	// 2. Test project
	testDiags := Test(tmpDir)
	if len(testDiags) > 0 {
		t.Fatalf("test execution failed: %+v", testDiags)
	}
}
