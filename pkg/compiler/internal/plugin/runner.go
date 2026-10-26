package plugin

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/protocol"
)

// DefaultTimeout is the default maximum execution duration for an external generator plugin.
const DefaultTimeout = 30 * time.Second

// RunnerConfig configures execution parameters for the plugin subprocess runner.
type RunnerConfig struct {
	Timeout time.Duration
}

// Runner executes generator plugin subprocesses in process isolation.
type Runner struct {
	timeout time.Duration
}

// NewRunner creates a new Runner with the specified configuration.
func NewRunner(cfg RunnerConfig) *Runner {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	return &Runner{
		timeout: timeout,
	}
}

// FormatOptionsAsArgs converts a key-value options map into deterministic CLI flags (--key=value).
func FormatOptionsAsArgs(options map[string]string) []string {
	if len(options) == 0 {
		return nil
	}
	keys := make([]string, 0, len(options))
	for k := range options {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	args := make([]string, 0, len(keys))
	for _, k := range keys {
		v := options[k]
		args = append(args, fmt.Sprintf("--%s=%s", k, v))
	}
	return args
}

// MirrorOptions copies target options into the GenerationRequest Target.Options map.
func MirrorOptions(req *protocol.GenerationRequest, options map[string]string) {
	if req.Target.Options == nil {
		req.Target.Options = make(map[string]string, len(options))
	}
	for k, v := range options {
		req.Target.Options[k] = v
	}
}

// Execute runs the resolved plugin executable, streaming the GenerationRequest over stdin,
// capturing GenerationResponse from stdout, and terminating process groups on timeout.
func (r *Runner) Execute(ctx context.Context, pluginPath string, req protocol.GenerationRequest) (protocol.GenerationResponse, []ir.Diagnostic) {
	pluginName := filepath.Base(pluginPath)

	// Ensure Target.Options contains all options and format CLI flags
	args := FormatOptionsAsArgs(req.Target.Options)

	reqBytes, err := protocol.EncodeRequest(req)
	if err != nil {
		return protocol.GenerationResponse{}, pluginErrorDiag(
			"FDN0701",
			fmt.Sprintf("failed to encode generation request for plugin %s: %v", pluginName, err),
		)
	}

	ctxWithTimeout, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctxWithTimeout, pluginPath, args...)
	cmd.Dir = req.WorkspaceRoot
	if cmd.Dir == "" {
		cmd.Dir = "."
	}
	cmd.Stdin = bytes.NewReader(reqBytes)

	// Process group isolation: kill entire process tree on cancellation or timeout
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process != nil && cmd.Process.Pid > 0 {
			return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
		return nil
	}

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	if err := cmd.Start(); err != nil {
		if ctx.Err() != nil {
			return protocol.GenerationResponse{}, cancellationDiag(pluginName, ctx.Err())
		}
		return protocol.GenerationResponse{}, pluginErrorDiag(
			"FDN0701",
			fmt.Sprintf("failed to start generator plugin %s: %v", pluginName, err),
		)
	}

	err = cmd.Wait()
	if err != nil {
		if errors.Is(ctxWithTimeout.Err(), context.DeadlineExceeded) {
			return protocol.GenerationResponse{}, pluginErrorDiag(
				"FDN0700",
				fmt.Sprintf("generator plugin %s timed out after %s", pluginName, r.timeout),
			)
		}
		if ctx.Err() != nil {
			return protocol.GenerationResponse{}, cancellationDiag(pluginName, ctx.Err())
		}

		exitCode := -1
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		}

		stderrStr := strings.TrimSpace(stderrBuf.String())
		msg := fmt.Sprintf("generator plugin %s exited with code %d", pluginName, exitCode)
		if stderrStr != "" {
			msg = fmt.Sprintf("%s: %s", msg, stderrStr)
		}

		return protocol.GenerationResponse{}, pluginErrorDiag("FDN0701", msg)
	}

	resp, decodeErr := protocol.DecodeResponse(stdoutBuf.Bytes())
	if decodeErr != nil {
		return protocol.GenerationResponse{}, pluginErrorDiag(
			"FDN0701",
			fmt.Sprintf("generator plugin %s emitted invalid response JSON: %v", pluginName, decodeErr),
		)
	}

	return resp, nil
}

func buildConfigSpan() ir.Span {
	return ir.Span{File: "foundation.build.yaml", Line: 1, Column: 1}
}

func pluginErrorDiag(code, msg string) []ir.Diagnostic {
	return []ir.Diagnostic{{
		Code:     code,
		Severity: ir.SeverityError,
		Message:  msg,
		Span:     buildConfigSpan(),
	}}
}

func cancellationDiag(pluginName string, err error) []ir.Diagnostic {
	return pluginErrorDiag("FDN0700", fmt.Sprintf("generator plugin %s execution canceled: %v", pluginName, err))
}
