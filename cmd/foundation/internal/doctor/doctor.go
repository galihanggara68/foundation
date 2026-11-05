package doctor

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/galihanggara68/foundation/pkg/compiler"
)

// DefaultTimeout specifies the default run limit for a single probe.
const DefaultTimeout = 10 * time.Second

// probeFunc runs one conformance probe against a plugin binary.
type probeFunc func(context.Context, string, []byte, time.Duration) ProbeResult

// DoctorOptions defines configuration settings for the doctor runner.
type DoctorOptions struct {
	Verbose bool
	Timeout time.Duration
}

// ProbeResult stores the outcome of a single conformance probe.
type ProbeResult struct {
	Name           string
	Passed         bool
	DiagnosticCode string
	Message        string
}

// Report collects all probe results from a doctor test run.
type Report struct {
	Probes    []ProbeResult
	AllPassed bool
}

// execResult stores the process output and exit status.
type execResult struct {
	stdout   []byte
	stderr   []byte
	exitCode int
	err      error
}

// Run tests a plugin binary with the five synthetic protocol probes.
func Run(ctx context.Context, binaryPath string, opts DoctorOptions) (Report, error) {
	resolvedPath, err := exec.LookPath(binaryPath)
	if err != nil && !errors.Is(err, exec.ErrDot) {
		if _, statErr := os.Stat(binaryPath); statErr != nil {
			return Report{}, fmt.Errorf("plugin binary not found: %w", err)
		}
		resolvedPath = binaryPath
	}

	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}

	reqBytes, err := compiler.EncodeGenerationRequest(syntheticRequest())
	if err != nil {
		return Report{}, fmt.Errorf("failed to encode synthetic request: %w", err)
	}

	probeRuns := []probeFunc{
		runProbe1,
		runProbe2,
		func(ctx context.Context, binaryPath string, _ []byte, timeout time.Duration) ProbeResult {
			return runProbe3(ctx, binaryPath, timeout)
		},
		runProbe4,
		runProbe5,
	}

	probes := make([]ProbeResult, 0, len(probeRuns))
	allPassed := true
	for _, runProbe := range probeRuns {
		result := runProbe(ctx, resolvedPath, reqBytes, timeout)
		probes = append(probes, result)
		if !result.Passed {
			allPassed = false
		}
		if ctx.Err() != nil {
			return Report{Probes: probes, AllPassed: false}, ctx.Err()
		}
	}

	return Report{Probes: probes, AllPassed: allPassed}, nil
}

// execPlugin runs the target binary with the provided input.
func execPlugin(ctx context.Context, binaryPath string, input []byte, timeout time.Duration) execResult {
	ctxWithTimeout, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctxWithTimeout, binaryPath)
	cmd.Stdin = bytes.NewReader(input)
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}

	return execResult{
		stdout:   stdoutBuf.Bytes(),
		stderr:   stderrBuf.Bytes(),
		exitCode: exitCode,
		err:      err,
	}
}
