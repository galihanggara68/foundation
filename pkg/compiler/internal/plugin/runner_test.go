package plugin_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/arch"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/plugin"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/protocol"
)

func createScript(t *testing.T, path, scriptContent string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("failed to create dir for %s: %v", path, err)
	}
	content := []byte("#!/bin/sh\n" + scriptContent)
	if err := os.WriteFile(path, content, 0755); err != nil {
		t.Fatalf("failed to write script %s: %v", path, err)
	}
}

func TestFormatOptionsAsArgs(t *testing.T) {
	opts := map[string]string{
		"zebra": "stripes",
		"apple": "red",
		"mode":  "fast",
		"flag":  "",
	}

	args := plugin.FormatOptionsAsArgs(opts)
	expected := []string{"--apple=red", "--flag=", "--mode=fast", "--zebra=stripes"}

	if len(args) != len(expected) {
		t.Fatalf("expected %d args, got %d: %v", len(expected), len(args), args)
	}
	for i, exp := range expected {
		if args[i] != exp {
			t.Errorf("arg[%d] = %q, expected %q", i, args[i], exp)
		}
	}

	if emptyArgs := plugin.FormatOptionsAsArgs(nil); emptyArgs != nil {
		t.Errorf("expected nil for empty options, got: %v", emptyArgs)
	}
}

func TestMirrorOptions(t *testing.T) {
	req := protocol.GenerationRequest{}
	opts := map[string]string{"foo": "bar", "telemetry": "otel"}

	plugin.MirrorOptions(&req, opts)
	if len(req.Target.Options) != 2 {
		t.Fatalf("expected 2 options mirrored, got %d", len(req.Target.Options))
	}
	if req.Target.Options["foo"] != "bar" || req.Target.Options["telemetry"] != "otel" {
		t.Errorf("options mismatch: %+v", req.Target.Options)
	}
}

func TestNewRunner_DefaultTimeout(t *testing.T) {
	r := plugin.NewRunner(plugin.RunnerConfig{})
	if r == nil {
		t.Fatal("expected non-nil runner")
	}
}

func TestRunner_StartFailure_FDN0701(t *testing.T) {
	runner := plugin.NewRunner(plugin.RunnerConfig{})
	req := protocol.GenerationRequest{
		ProtocolVersion: protocol.ProtocolVersion,
		WorkspaceRoot:   "/workspace",
		Target: protocol.TargetInfo{
			TargetConfig: arch.TargetConfig{Name: "test", Kind: "backend", Language: "go", OutDir: "gen"},
		},
		SemanticIR: &ir.SemanticIR{},
	}

	_, diags := runner.Execute(context.Background(), "/non/existent/path", req)
	if len(diags) != 1 || diags[0].Code != "FDN0701" {
		t.Fatalf("expected FDN0701 for start failure, got: %+v", diags)
	}
}

func TestRunner_ContextCancellation_FDN0700(t *testing.T) {
	tmpDir := t.TempDir()
	genScript := filepath.Join(tmpDir, "bin", "mock-sleep")
	createScript(t, genScript, `
sleep 5
exit 0
`)

	runner := plugin.NewRunner(plugin.RunnerConfig{
		Timeout: 5 * time.Second,
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	req := protocol.GenerationRequest{
		ProtocolVersion: protocol.ProtocolVersion,
		WorkspaceRoot:   tmpDir,
		Target: protocol.TargetInfo{
			TargetConfig: arch.TargetConfig{Name: "test", Kind: "backend", Language: "go", OutDir: "gen"},
		},
		SemanticIR: &ir.SemanticIR{},
	}

	_, diags := runner.Execute(ctx, genScript, req)
	if len(diags) != 1 || diags[0].Code != "FDN0700" {
		t.Fatalf("expected FDN0700 for context cancellation, got: %+v", diags)
	}
	if !strings.Contains(diags[0].Message, "canceled") {
		t.Errorf("expected message to mention canceled, got: %s", diags[0].Message)
	}
}

func TestRunner_SuccessfulExecution(t *testing.T) {
	tmpDir := t.TempDir()
	genScript := filepath.Join(tmpDir, "bin", "mock-gen")
	createScript(t, genScript, `
cat > /dev/null
cat << 'EOF'
{
  "files": {
    "src/sample.txt": "hello from plugin"
  },
  "test_command": "echo test pass"
}
EOF
exit 0
`)

	runner := plugin.NewRunner(plugin.RunnerConfig{
		Timeout: 5 * time.Second,
	})

	req := protocol.GenerationRequest{
		ProtocolVersion: protocol.ProtocolVersion,
		WorkspaceRoot:   tmpDir,
		Target: protocol.TargetInfo{
			TargetConfig: arch.TargetConfig{
				Name:     "backend",
				Kind:     "backend",
				Language: "go",
				OutDir:   "gen",
			},
			Options: map[string]string{"foo": "bar"},
		},
		SemanticIR: &ir.SemanticIR{},
	}

	resp, diags := runner.Execute(context.Background(), genScript, req)
	if len(diags) > 0 {
		t.Fatalf("unexpected diagnostics: %+v", diags)
	}
	if resp.Files["src/sample.txt"] != "hello from plugin" {
		t.Errorf("expected file content, got: %v", resp.Files)
	}
	if resp.TestCommand != "echo test pass" {
		t.Errorf("expected test command 'echo test pass', got: %q", resp.TestCommand)
	}
}

func TestRunner_TimeoutTerminatesProcessGroup_FDN0700(t *testing.T) {
	tmpDir := t.TempDir()
	genScript := filepath.Join(tmpDir, "bin", "mock-hang")
	createScript(t, genScript, `
sleep 5
exit 0
`)

	runner := plugin.NewRunner(plugin.RunnerConfig{
		Timeout: 100 * time.Millisecond,
	})

	req := protocol.GenerationRequest{
		ProtocolVersion: protocol.ProtocolVersion,
		WorkspaceRoot:   tmpDir,
		Target: protocol.TargetInfo{
			TargetConfig: arch.TargetConfig{
				Name:     "backend",
				Kind:     "backend",
				Language: "go",
				OutDir:   "gen",
			},
		},
		SemanticIR: &ir.SemanticIR{},
	}

	start := time.Now()
	_, diags := runner.Execute(context.Background(), genScript, req)
	elapsed := time.Since(start)

	if elapsed > 2*time.Second {
		t.Errorf("runner took %s, expected timeout termination within ~100ms", elapsed)
	}

	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d: %+v", len(diags), diags)
	}
	if diags[0].Code != "FDN0700" {
		t.Errorf("expected diagnostic code FDN0700, got: %s", diags[0].Code)
	}
	if diags[0].Severity != ir.SeverityError {
		t.Errorf("expected SeverityError, got: %s", diags[0].Severity)
	}
	if !strings.Contains(diags[0].Message, "timed out") {
		t.Errorf("expected message to mention 'timed out', got: %s", diags[0].Message)
	}
}

func TestRunner_NonZeroExitCapturesStderr_FDN0701(t *testing.T) {
	tmpDir := t.TempDir()
	genScript := filepath.Join(tmpDir, "bin", "mock-fail")
	createScript(t, genScript, `
echo "fatal: compilation failed at line 42" >&2
exit 2
`)

	runner := plugin.NewRunner(plugin.RunnerConfig{
		Timeout: 5 * time.Second,
	})

	req := protocol.GenerationRequest{
		ProtocolVersion: protocol.ProtocolVersion,
		WorkspaceRoot:   tmpDir,
		Target: protocol.TargetInfo{
			TargetConfig: arch.TargetConfig{
				Name:     "backend",
				Kind:     "backend",
				Language: "go",
				OutDir:   "gen",
			},
		},
		SemanticIR: &ir.SemanticIR{},
	}

	_, diags := runner.Execute(context.Background(), genScript, req)
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d: %+v", len(diags), diags)
	}
	if diags[0].Code != "FDN0701" {
		t.Errorf("expected diagnostic code FDN0701, got: %s", diags[0].Code)
	}
	if diags[0].Severity != ir.SeverityError {
		t.Errorf("expected SeverityError, got: %s", diags[0].Severity)
	}
	if !strings.Contains(diags[0].Message, "fatal: compilation failed at line 42") {
		t.Errorf("expected message to contain captured stderr, got: %s", diags[0].Message)
	}
	if !strings.Contains(diags[0].Message, "code 2") {
		t.Errorf("expected message to mention exit code 2, got: %s", diags[0].Message)
	}
}

func TestRunner_InvalidJSONResponse_FDN0701(t *testing.T) {
	tmpDir := t.TempDir()
	genScript := filepath.Join(tmpDir, "bin", "mock-badjson")
	createScript(t, genScript, `
echo "invalid json output"
exit 0
`)

	runner := plugin.NewRunner(plugin.RunnerConfig{
		Timeout: 5 * time.Second,
	})

	req := protocol.GenerationRequest{
		ProtocolVersion: protocol.ProtocolVersion,
		WorkspaceRoot:   tmpDir,
		Target: protocol.TargetInfo{
			TargetConfig: arch.TargetConfig{
				Name:     "backend",
				Kind:     "backend",
				Language: "go",
				OutDir:   "gen",
			},
		},
		SemanticIR: &ir.SemanticIR{},
	}

	_, diags := runner.Execute(context.Background(), genScript, req)
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d: %+v", len(diags), diags)
	}
	if diags[0].Code != "FDN0701" {
		t.Errorf("expected diagnostic code FDN0701, got: %s", diags[0].Code)
	}
	if !strings.Contains(diags[0].Message, "invalid response JSON") {
		t.Errorf("expected message to mention invalid response JSON, got: %s", diags[0].Message)
	}
}

func TestRunner_CLIArgsPassedThrough(t *testing.T) {
	tmpDir := t.TempDir()
	genScript := filepath.Join(tmpDir, "bin", "mock-args")
	createScript(t, genScript, `
if [ "$1" != "--env=prod" ] || [ "$2" != "--telemetry=otel" ]; then
  echo "unexpected args: $@" >&2
  exit 1
fi
echo '{"files": {"ok.txt": "yes"}}'
exit 0
`)

	runner := plugin.NewRunner(plugin.RunnerConfig{
		Timeout: 5 * time.Second,
	})

	req := protocol.GenerationRequest{
		ProtocolVersion: protocol.ProtocolVersion,
		WorkspaceRoot:   tmpDir,
		Target: protocol.TargetInfo{
			TargetConfig: arch.TargetConfig{
				Name:     "backend",
				Kind:     "backend",
				Language: "go",
				OutDir:   "gen",
			},
			Options: map[string]string{
				"telemetry": "otel",
				"env":       "prod",
			},
		},
		SemanticIR: &ir.SemanticIR{},
	}

	resp, diags := runner.Execute(context.Background(), genScript, req)
	if len(diags) > 0 {
		t.Fatalf("unexpected diagnostics: %+v", diags)
	}
	if resp.Files["ok.txt"] != "yes" {
		t.Errorf("expected ok.txt content, got: %v", resp.Files)
	}
}
