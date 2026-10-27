package compiler

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/arch"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/emit"
)

// DefaultTestTimeout is the default maximum duration for external test execution.
const DefaultTestTimeout = 60 * time.Second

// testRunner executes the native test suite of one emitted target (Strategy pattern).
// Registering a new target kind or language adds an entry here without modifying existing runners.
type testRunner func(target arch.TargetConfig, outDir string) []Diagnostic

type targetRunnerKey struct {
	Kind     string
	Language string
}

var testRunners = map[targetRunnerKey]testRunner{
	{Kind: "backend", Language: "go"}:         runGoTestSuite,
	{Kind: "backend", Language: "typescript"}: runTypeScriptTestSuite,
	{Kind: "sdk", Language: "go"}:             runNoopTestSuite,
	{Kind: "sdk", Language: "typescript"}:     runNoopTestSuite,
}

func runNoopTestSuite(target arch.TargetConfig, outDir string) []Diagnostic {
	return nil
}

// Test executes the native test suites inside every emitted target in foundation.build.yaml.
func Test(workspaceDir string) []Diagnostic {
	bp, diags := arch.LoadBuildProfile(workspaceDir)
	if len(diags) > 0 {
		return diags
	}

	for _, target := range bp.Targets {
		outDir := filepath.Join(workspaceDir, filepath.FromSlash(target.OutDir))

		// 1. Resolve test command (hybrid resolution: YAML override > Manifest test_command)
		cmdStr := target.TestCommand
		if cmdStr == "" {
			if man, err := emit.ReadManifest(outDir); err == nil && man.TestCommand != "" {
				cmdStr = man.TestCommand
			}
		}

		if cmdStr != "" {
			timeout := parseTestTimeout(target.TestTimeout)
			if runDiags := runExternalTestCommand(target, outDir, cmdStr, timeout); len(runDiags) > 0 {
				return runDiags
			}
			continue
		}

		// 2. Built-in test runner fallback
		key := targetRunnerKey{Kind: target.Kind, Language: target.Language}
		runner, ok := testRunners[key]
		if !ok {
			return []Diagnostic{{
				Code:     "FDN0601",
				Severity: SeverityError,
				Message:  fmt.Sprintf("no test runner registered for target %q language %q", target.Name, target.Language),
				Span:     Span{File: target.OutDir, Line: 1, Column: 1},
			}}
		}
		if runnerDiags := runner(target, outDir); len(runnerDiags) > 0 {
			return runnerDiags
		}
	}

	return nil
}

func parseTestTimeout(raw string) time.Duration {
	if raw == "" {
		return DefaultTestTimeout
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		return DefaultTestTimeout
	}
	return d
}

func runExternalTestCommand(target arch.TargetConfig, outDir, cmdStr string, timeout time.Duration) []Diagnostic {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", cmdStr)
	cmd.Dir = outDir
	configureProcessIsolation(cmd)

	var outBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &outBuf

	if err := cmd.Start(); err != nil {
		return []Diagnostic{testFailureDiag(target, fmt.Sprintf("test start (%s)", cmdStr), err, outBuf.String())}
	}

	err := cmd.Wait()
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return []Diagnostic{{
				Code:     "FDN0602",
				Severity: SeverityError,
				Message:  fmt.Sprintf("target %q test execution timed out after %s (command: %q)", target.Name, timeout, cmdStr),
				Span:     Span{File: target.OutDir, Line: 1, Column: 1},
			}}
		}
		return []Diagnostic{testFailureDiag(target, fmt.Sprintf("test execution (%s)", cmdStr), err, outBuf.String())}
	}

	return nil
}

func runInDir(dir, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func testFailureDiag(target arch.TargetConfig, stage string, err error, out string) Diagnostic {
	return Diagnostic{
		Code:     "FDN0600",
		Severity: SeverityError,
		Message:  fmt.Sprintf("target %q %s failed: %v\n%s", target.Name, stage, err, out),
		Span:     Span{File: target.OutDir, Line: 1, Column: 1},
	}
}

func runGoTestSuite(target arch.TargetConfig, outDir string) []Diagnostic {
	out, err := runInDir(outDir, "go", "test", "./...")
	if err != nil {
		return []Diagnostic{testFailureDiag(target, "test execution (go test ./...)", err, out)}
	}
	return nil
}

func runTypeScriptTestSuite(target arch.TargetConfig, outDir string) []Diagnostic {
	// The emitted package.json declares express/pg dependencies and typescript devDependencies;
	// install them on first test run so tsc type-checks the real library APIs.
	if _, statErr := os.Stat(filepath.Join(outDir, "node_modules")); os.IsNotExist(statErr) {
		out, err := runInDir(outDir, "npm", "install", "--prefer-offline", "--no-audit", "--no-fund", "--silent")
		if err != nil {
			return []Diagnostic{testFailureDiag(target, "dependency installation (npm install)", err, out)}
		}
	}

	// Prefer the project-local TypeScript compiler; fall back to a globally installed tsc.
	tscBin := "tsc"
	if localTsc := filepath.Join(outDir, "node_modules", ".bin", "tsc"); fileExists(localTsc) {
		tscBin = localTsc
	}
	if out, err := runInDir(outDir, tscBin, "--noEmit"); err != nil {
		return []Diagnostic{testFailureDiag(target, "TypeScript type-check (tsc --noEmit)", err, out)}
	}

	if out, err := runInDir(outDir, "node", "--test", "test/contracts.test.ts", "test/scenarios.test.ts"); err != nil {
		return []Diagnostic{testFailureDiag(target, "test execution (node --test)", err, out)}
	}
	return nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
