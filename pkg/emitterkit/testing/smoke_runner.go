package testing

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/galihanggara68/foundation/pkg/emitterkit"
)

// SmokeResult contains execution outcomes from running smoke commands on emitted files.
type SmokeResult struct {
	// BuildSuccess confirms that compilation succeeded without build errors.
	BuildSuccess bool
	// TestSuccess confirms that all smoke test assertions passed.
	TestSuccess bool
	// Output stores the combined standard output and standard error from execution.
	Output string
	// Error captures any process or execution error.
	Error error
}

// RunSmokeSuite writes emitted files to a temporary directory and runs the test command.
func (h *Harness) RunSmokeSuite(resp emitterkit.GenerationResponse, command string) SmokeResult {
	tmpDir, err := os.MkdirTemp("", "foundation-smoke-*")
	if err != nil {
		return SmokeResult{
			BuildSuccess: false,
			TestSuccess:  false,
			Error:        fmt.Errorf("create temporary smoke directory: %w", err),
		}
	}
	defer os.RemoveAll(tmpDir)

	if err := writeFilesToDir(tmpDir, resp.Stubs); err != nil {
		return SmokeResult{
			BuildSuccess: false,
			TestSuccess:  false,
			Error:        err,
		}
	}
	if err := writeFilesToDir(tmpDir, resp.Files); err != nil {
		return SmokeResult{
			BuildSuccess: false,
			TestSuccess:  false,
			Error:        err,
		}
	}

	cmdStr := command
	if cmdStr == "" {
		cmdStr = resp.TestCommand
	}
	if cmdStr == "" {
		cmdStr = "go test ./..."
	}

	var cmd *exec.Cmd
	if strings.ContainsAny(cmdStr, "|&><;") {
		cmd = exec.Command("sh", "-c", cmdStr)
	} else {
		parts := strings.Fields(cmdStr)
		if len(parts) == 0 {
			return SmokeResult{
				BuildSuccess: false,
				TestSuccess:  false,
				Error:        errors.New("smoke test command is empty"),
			}
		}
		cmd = exec.Command(parts[0], parts[1:]...)
	}

	cmd.Dir = tmpDir
	cmd.Env = os.Environ()

	outBytes, cmdErr := cmd.CombinedOutput()
	output := string(outBytes)

	if cmdErr == nil {
		return SmokeResult{
			BuildSuccess: true,
			TestSuccess:  true,
			Output:       output,
			Error:        nil,
		}
	}

	var exitErr *exec.ExitError
	if !errors.As(cmdErr, &exitErr) {
		return SmokeResult{
			BuildSuccess: false,
			TestSuccess:  false,
			Output:       output,
			Error:        cmdErr,
		}
	}

	buildSucceeded := determineBuildSuccess(tmpDir, cmdStr, output)

	return SmokeResult{
		BuildSuccess: buildSucceeded,
		TestSuccess:  false,
		Output:       output,
		Error:        cmdErr,
	}
}

func writeFilesToDir(rootDir string, files map[string]string) error {
	for relPath, content := range files {
		fullPath, err := emitterkit.SafeRelPath(rootDir, relPath)
		if err != nil {
			return fmt.Errorf("resolve safe path for %q: %w", relPath, err)
		}
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			return fmt.Errorf("create directory for %q: %w", relPath, err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			return fmt.Errorf("write file %q: %w", relPath, err)
		}
	}
	return nil
}

func determineBuildSuccess(dir, cmdStr, output string) bool {
	lowerOut := strings.ToLower(output)
	if strings.Contains(lowerOut, "[build failed]") || strings.Contains(lowerOut, "build failed") {
		return false
	}
	if strings.Contains(cmdStr, "build") && !strings.Contains(cmdStr, "test") {
		return false
	}

	if strings.Contains(cmdStr, "go test") {
		checkCmd := exec.Command("go", "test", "-run", "^$", "./...")
		checkCmd.Dir = dir
		checkCmd.Env = os.Environ()
		if err := checkCmd.Run(); err != nil {
			return false
		}
		return true
	}

	return false
}
