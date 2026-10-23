package compiler

import (
	"fmt"
	"os/exec"
	"path/filepath"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/arch"
)

// Test executes the native test suites inside every emitted target in foundation.build.yaml.
func Test(workspaceDir string) []Diagnostic {
	bp, diags := arch.LoadBuildProfile(workspaceDir)
	if len(diags) > 0 {
		return diags
	}

	for _, target := range bp.Targets {
		outDir := filepath.Join(workspaceDir, filepath.FromSlash(target.OutDir))
		if target.Language == "go" {
			cmd := exec.Command("go", "test", "./...")
			cmd.Dir = outDir
			out, err := cmd.CombinedOutput()
			if err != nil {
				return []Diagnostic{{
					Code:     "FDN0600",
					Severity: SeverityError,
					Message:  fmt.Sprintf("target %q test execution failed: %v\n%s", target.Name, err, string(out)),
					Span:     Span{File: target.OutDir, Line: 1, Column: 1},
				}}
			}
		}
	}

	return nil
}
