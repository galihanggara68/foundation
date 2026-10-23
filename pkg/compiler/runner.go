package compiler

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/arch"
)

// testRunner executes the native test suite of one emitted backend target (Strategy pattern).
// Registering a new target language adds an entry here without modifying existing runners.
type testRunner func(target arch.TargetConfig, outDir string) []Diagnostic

var testRunners = map[string]testRunner{
	"go":         runGoTestSuite,
	"typescript": runTypeScriptTestSuite,
}

// Test executes the native test suites inside every emitted target in foundation.build.yaml.
func Test(workspaceDir string) []Diagnostic {
	bp, diags := arch.LoadBuildProfile(workspaceDir)
	if len(diags) > 0 {
		return diags
	}

	for _, target := range bp.Targets {
		runner, ok := testRunners[target.Language]
		if !ok {
			return []Diagnostic{{
				Code:     "FDN0601",
				Severity: SeverityError,
				Message:  fmt.Sprintf("no test runner registered for target %q language %q", target.Name, target.Language),
				Span:     Span{File: target.OutDir, Line: 1, Column: 1},
			}}
		}
		outDir := filepath.Join(workspaceDir, filepath.FromSlash(target.OutDir))
		if runnerDiags := runner(target, outDir); len(runnerDiags) > 0 {
			return runnerDiags
		}
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
