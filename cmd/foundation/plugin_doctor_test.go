package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// createDoctorTestScript writes an executable shell script for testing.
func createDoctorTestScript(t *testing.T, path, scriptContent string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("failed to create directory for %s: %v", path, err)
	}
	content := []byte("#!/bin/sh\n" + scriptContent)
	if err := os.WriteFile(path, content, 0755); err != nil {
		t.Fatalf("failed to write script %s: %v", path, err)
	}
}

// TestPluginDoctor_ConformingPlugin confirms that doctor passes for a conforming plugin.
func TestPluginDoctor_ConformingPlugin(t *testing.T) {
	tmpDir := t.TempDir()
	binPath := filepath.Join(tmpDir, "conforming-plugin")
	script := `
input=$(cat)
case "$input" in
  *"malformed"*)
    echo '{"protocol_version":1,"diagnostics":[{"code":"FDN0701","message":"malformed input"}]}'
    exit 0
    ;;
  *)
    echo '{"protocol_version":1,"files":{"test.txt":"ok"},"diagnostics":[]}'
    exit 0
    ;;
esac
`
	createDoctorTestScript(t, binPath, script)

	var stdout, stderr bytes.Buffer
	code := run([]string{"plugin", "doctor", binPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d. stderr: %s", code, stderr.String())
	}

	out := stdout.String()
	expectedProbes := []string{
		"[PASS] Protocol Version Echo",
		"[PASS] Byte-Level Determinism",
		"[PASS] Malformed Payload Error Handling",
		"[PASS] Standard Output Purity",
		"[PASS] Process Exit Code Discipline",
	}

	for _, probe := range expectedProbes {
		if !strings.Contains(out, probe) {
			t.Errorf("expected stdout to contain %q, got:\n%s", probe, out)
		}
	}
}

// TestPluginDoctor_PollutingPlugin confirms that doctor reports FDN0706 on log pollution.
func TestPluginDoctor_PollutingPlugin(t *testing.T) {
	tmpDir := t.TempDir()
	binPath := filepath.Join(tmpDir, "polluting-plugin")
	script := `
input=$(cat)
echo "[INFO] starting generator"
echo '{"protocol_version":1,"files":{},"diagnostics":[]}'
exit 0
`
	createDoctorTestScript(t, binPath, script)

	var stdout, stderr bytes.Buffer
	code := run([]string{"plugin", "doctor", binPath}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}

	out := stdout.String()
	if !strings.Contains(out, "FDN0706") {
		t.Errorf("expected stdout to contain FDN0706, got:\n%s", out)
	}
}

// TestPluginDoctor_VerboseFlag confirms that the verbose flag prints probe detail messages.
func TestPluginDoctor_VerboseFlag(t *testing.T) {
	tmpDir := t.TempDir()
	binPath := filepath.Join(tmpDir, "conforming-plugin")
	script := `
input=$(cat)
case "$input" in
  *"malformed"*)
    echo '{"protocol_version":1,"diagnostics":[{"code":"FDN0701","message":"malformed input"}]}'
    exit 0
    ;;
  *)
    echo '{"protocol_version":1,"files":{"test.txt":"ok"},"diagnostics":[]}'
    exit 0
    ;;
esac
`
	createDoctorTestScript(t, binPath, script)

	var stdout, stderr bytes.Buffer
	code := run([]string{"plugin", "doctor", binPath, "--verbose"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}

	out := stdout.String()
	if !strings.Contains(out, "protocol version 1 echoed successfully") {
		t.Errorf("expected verbose details in stdout, got:\n%s", out)
	}
}

// TestPluginDoctor_MissingBinaryArgument confirms error output when the binary path is missing.
func TestPluginDoctor_MissingBinaryArgument(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"plugin", "doctor"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}

	errOut := stderr.String()
	if !strings.Contains(errOut, "Usage: foundation plugin doctor <binary-path>") {
		t.Errorf("expected usage in stderr, got: %s", errOut)
	}
}

// TestPluginDoctor_UnknownFlag confirms error output when an unknown flag is provided.
func TestPluginDoctor_UnknownFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"plugin", "doctor", "--unknown-flag"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}

	errOut := stderr.String()
	if !strings.Contains(errOut, "unknown flag") {
		t.Errorf("expected unknown flag error in stderr, got: %s", errOut)
	}
}

// TestPluginDoctor_NonExistentBinary confirms error output when the target binary is missing.
func TestPluginDoctor_NonExistentBinary(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"plugin", "doctor", "/path/to/missing/plugin"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}

	errOut := stderr.String()
	if !strings.Contains(errOut, "doctor error") {
		t.Errorf("expected doctor error in stderr, got: %s", errOut)
	}
}
