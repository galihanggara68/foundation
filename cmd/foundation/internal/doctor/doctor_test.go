package doctor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/galihanggara68/foundation/pkg/compiler"
)

// createExecutable creates an executable script file at the given path.
func createExecutable(t *testing.T, path, scriptContent string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("failed to create directory for %s: %v", path, err)
	}
	content := []byte("#!/bin/sh\n" + scriptContent)
	if err := os.WriteFile(path, content, 0755); err != nil {
		t.Fatalf("failed to write script %s: %v", path, err)
	}
}

// TestDoctor_ConformingPlugin confirms that a fully conforming plugin passes all probes.
func TestDoctor_ConformingPlugin(t *testing.T) {
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
    echo '{"protocol_version":1,"files":{"out.txt":"data"},"diagnostics":[]}'
    exit 0
    ;;
esac
`
	createExecutable(t, binPath, script)

	report, err := Run(context.Background(), binPath, DoctorOptions{Verbose: true})
	if err != nil {
		t.Fatalf("unexpected error from Run: %v", err)
	}

	if !report.AllPassed {
		t.Fatalf("expected all probes to pass, but some failed")
	}

	if len(report.Probes) != 5 {
		t.Fatalf("expected 5 probe results, got %d", len(report.Probes))
	}

	for _, p := range report.Probes {
		if !p.Passed {
			t.Errorf("probe %s failed: [%s] %s", p.Name, p.DiagnosticCode, p.Message)
		}
	}
}

// TestDoctor_Probe1_VersionMismatch confirms that probe 1 detects version mismatches.
func TestDoctor_Probe1_VersionMismatch(t *testing.T) {
	tmpDir := t.TempDir()
	binPath := filepath.Join(tmpDir, "version-mismatch-plugin")
	script := `
input=$(cat)
case "$input" in
  *"malformed"*)
    echo '{"protocol_version":1,"diagnostics":[{"code":"FDN0701","message":"malformed input"}]}'
    exit 0
    ;;
  *)
    echo '{"protocol_version":99,"files":{},"diagnostics":[]}'
    exit 0
    ;;
esac
`
	createExecutable(t, binPath, script)

	report, err := Run(context.Background(), binPath, DoctorOptions{})
	if err != nil {
		t.Fatalf("unexpected error from Run: %v", err)
	}

	if report.AllPassed {
		t.Fatalf("expected report.AllPassed to be false")
	}

	probe1 := report.Probes[0]
	if probe1.Passed {
		t.Errorf("expected probe 1 to fail")
	}
	if probe1.DiagnosticCode != "FDN0703" {
		t.Errorf("expected diagnostic FDN0703, got %s", probe1.DiagnosticCode)
	}
}

// TestDoctor_Probe2_NonDeterministic confirms that probe 2 detects non-deterministic output.
func TestDoctor_Probe2_NonDeterministic(t *testing.T) {
	tmpDir := t.TempDir()
	binPath := filepath.Join(tmpDir, "nondet-plugin")
	counterFile := filepath.Join(tmpDir, "counter.txt")
	script := fmt.Sprintf(`
input=$(cat)
case "$input" in
  *"malformed"*)
    echo '{"protocol_version":1,"diagnostics":[{"code":"FDN0701","message":"malformed input"}]}'
    exit 0
    ;;
  *)
    val=0
    if [ -f "%s" ]; then
      val=$(cat "%s")
    fi
    val=$((val + 1))
    echo "$val" > "%s"
    echo "{\"protocol_version\":1,\"files\":{\"run.txt\":\"$val\"},\"diagnostics\":[]}"
    exit 0
    ;;
esac
`, counterFile, counterFile, counterFile)
	createExecutable(t, binPath, script)

	reqBytes, err := compiler.EncodeGenerationRequest(syntheticRequest())
	if err != nil {
		t.Fatalf("failed to encode request: %v", err)
	}

	res := runProbe2(context.Background(), binPath, reqBytes, 5*time.Second)
	if res.Passed {
		t.Errorf("expected probe 2 to fail on non-deterministic output")
	}
	if res.DiagnosticCode != "FDN0704" {
		t.Errorf("expected diagnostic FDN0704, got %s", res.DiagnosticCode)
	}
}

// TestDoctor_Probe3_MalformedErrorHandling confirms that probe 3 detects missing FDN0701.
func TestDoctor_Probe3_MalformedErrorHandling(t *testing.T) {
	tmpDir := t.TempDir()
	binPath := filepath.Join(tmpDir, "bad-malformed-plugin")
	script := `
input=$(cat)
case "$input" in
  *"malformed"*)
    echo '{"protocol_version":1,"diagnostics":[]}'
    exit 0
    ;;
  *)
    echo '{"protocol_version":1,"files":{},"diagnostics":[]}'
    exit 0
    ;;
esac
`
	createExecutable(t, binPath, script)

	res := runProbe3(context.Background(), binPath, 5*time.Second)
	if res.Passed {
		t.Errorf("expected probe 3 to fail when FDN0701 is missing")
	}
	if res.DiagnosticCode != "FDN0705" {
		t.Errorf("expected diagnostic FDN0705, got %s", res.DiagnosticCode)
	}
}

// TestDoctor_Probe4_StdoutPollution confirms that probe 4 detects stdout pollution.
func TestDoctor_Probe4_StdoutPollution(t *testing.T) {
	tmpDir := t.TempDir()
	binPath := filepath.Join(tmpDir, "polluting-plugin")
	script := `
input=$(cat)
echo "[LOG] unexpected log line before JSON"
echo '{"protocol_version":1,"files":{},"diagnostics":[]}'
exit 0
`
	createExecutable(t, binPath, script)

	reqBytes, err := compiler.EncodeGenerationRequest(syntheticRequest())
	if err != nil {
		t.Fatalf("failed to encode request: %v", err)
	}

	res := runProbe4(context.Background(), binPath, reqBytes, 5*time.Second)
	if res.Passed {
		t.Errorf("expected probe 4 to fail on polluted output")
	}
	if res.DiagnosticCode != "FDN0706" {
		t.Errorf("expected diagnostic FDN0706, got %s", res.DiagnosticCode)
	}
}

// TestDoctor_Probe5_ExitCodeDiscipline confirms that probe 5 detects non-zero exit codes.
func TestDoctor_Probe5_ExitCodeDiscipline(t *testing.T) {
	tmpDir := t.TempDir()
	binPath := filepath.Join(tmpDir, "exit-code-plugin")
	script := `
input=$(cat)
case "$input" in
  *"malformed"*)
    echo '{"protocol_version":1,"diagnostics":[{"code":"FDN0701","message":"malformed input"}]}'
    exit 1
    ;;
  *)
    echo '{"protocol_version":1,"files":{},"diagnostics":[]}'
    exit 0
    ;;
esac
`
	createExecutable(t, binPath, script)

	reqBytes, err := compiler.EncodeGenerationRequest(syntheticRequest())
	if err != nil {
		t.Fatalf("failed to encode request: %v", err)
	}

	res := runProbe5(context.Background(), binPath, reqBytes, 5*time.Second)
	if res.Passed {
		t.Errorf("expected probe 5 to fail on non-zero exit code")
	}
	if res.DiagnosticCode != "FDN0707" {
		t.Errorf("expected diagnostic FDN0707, got %s", res.DiagnosticCode)
	}
}

// TestDoctor_BinaryNotFound confirms that Run returns an error if the binary does not exist.
func TestDoctor_BinaryNotFound(t *testing.T) {
	report, err := Run(context.Background(), "/non/existent/plugin/path", DoctorOptions{})
	if err == nil {
		t.Fatalf("expected error for missing binary, got nil")
	}
	if report.AllPassed {
		t.Errorf("expected report.AllPassed to be false")
	}
}

// TestDoctor_CrashingPlugin confirms that a crash on a valid request reports run failures
// instead of version-mismatch or standard output pollution codes.
func TestDoctor_CrashingPlugin(t *testing.T) {
	tmpDir := t.TempDir()
	binPath := filepath.Join(tmpDir, "crashing-plugin")
	script := `
input=$(cat)
echo "[FATAL] generator crashed" >&2
exit 1
`
	createExecutable(t, binPath, script)

	reqBytes, err := compiler.EncodeGenerationRequest(syntheticRequest())
	if err != nil {
		t.Fatalf("failed to encode request: %v", err)
	}

	probe1 := runProbe1(context.Background(), binPath, reqBytes, 5*time.Second)
	if probe1.Passed || probe1.DiagnosticCode != "FDN0701" {
		t.Errorf("expected probe 1 to fail with FDN0701, got passed=%v code=%s", probe1.Passed, probe1.DiagnosticCode)
	}

	probe4 := runProbe4(context.Background(), binPath, reqBytes, 5*time.Second)
	if probe4.Passed || probe4.DiagnosticCode != "FDN0701" {
		t.Errorf("expected probe 4 to fail with FDN0701, got passed=%v code=%s", probe4.Passed, probe4.DiagnosticCode)
	}

	probe5 := runProbe5(context.Background(), binPath, reqBytes, 5*time.Second)
	if probe5.Passed || probe5.DiagnosticCode != "FDN0707" {
		t.Errorf("expected probe 5 to fail with FDN0707, got passed=%v code=%s", probe5.Passed, probe5.DiagnosticCode)
	}
}
