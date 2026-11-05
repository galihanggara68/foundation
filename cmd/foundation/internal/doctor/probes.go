package doctor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/galihanggara68/foundation/pkg/compiler"
)

// runFailureCode is the diagnostic the doctor reports when a plugin process does not run cleanly.
const runFailureCode = "FDN0701"

// syntheticRequest builds a minimal valid generation request that the probes send.
func syntheticRequest() compiler.GenerationRequest {
	return compiler.GenerationRequest{
		ProtocolVersion: compiler.ProtocolVersion,
		WorkspaceRoot:   ".",
		Target: compiler.TargetInfo{
			TargetConfig: compiler.TargetConfig{
				Name:     "doctor-probe",
				Kind:     "backend",
				Language: "go",
				OutDir:   "gen",
			},
			Options: map[string]string{},
		},
		SemanticIR: &compiler.SemanticIR{
			Entities: []compiler.EntityDecl{},
		},
		Files: make(map[string]string),
	}
}

// passedResult builds a passing probe result.
func passedResult(name, message string) ProbeResult {
	return ProbeResult{Name: name, Passed: true, Message: message}
}

// failedResult builds a failing probe result with a diagnostic code.
func failedResult(name, code, message string) ProbeResult {
	return ProbeResult{Name: name, Passed: false, DiagnosticCode: code, Message: message}
}

// runFailure reports a probe failure when the plugin process exits non-zero or fails to start.
// The label prefixes the message, for example "first" in a two-run probe.
func runFailure(name, code, label string, res execResult) (ProbeResult, bool) {
	if res.err == nil && res.exitCode == 0 {
		return ProbeResult{}, false
	}
	return failedResult(name, code, fmt.Sprintf("%splugin run failed with exit code %d: %v", label, res.exitCode, res.err)), true
}

// runProbe1 validates that the plugin echoes the correct protocol version.
func runProbe1(ctx context.Context, binaryPath string, reqBytes []byte, timeout time.Duration) ProbeResult {
	const name = "Protocol Version Echo"
	res := execPlugin(ctx, binaryPath, reqBytes, timeout)
	if failure, failed := runFailure(name, runFailureCode, "", res); failed {
		return failure
	}

	var resp compiler.GenerationResponse
	if err := json.Unmarshal(res.stdout, &resp); err != nil {
		return failedResult(name, runFailureCode, fmt.Sprintf("failed to decode response JSON: %v", err))
	}

	if resp.ProtocolVersion != compiler.ProtocolVersion {
		return failedResult(name, "FDN0703", fmt.Sprintf("protocol version mismatch: expected %d, got %d", compiler.ProtocolVersion, resp.ProtocolVersion))
	}

	return passedResult(name, fmt.Sprintf("protocol version %d echoed successfully", resp.ProtocolVersion))
}

// runProbe2 validates byte-level determinism across two identical runs.
func runProbe2(ctx context.Context, binaryPath string, reqBytes []byte, timeout time.Duration) ProbeResult {
	const name = "Byte-Level Determinism"
	res1 := execPlugin(ctx, binaryPath, reqBytes, timeout)
	if failure, failed := runFailure(name, runFailureCode, "first ", res1); failed {
		return failure
	}

	res2 := execPlugin(ctx, binaryPath, reqBytes, timeout)
	if failure, failed := runFailure(name, runFailureCode, "second ", res2); failed {
		return failure
	}

	if !bytes.Equal(res1.stdout, res2.stdout) {
		return failedResult(name, "FDN0704", "output bytes differ between identical runs")
	}

	return passedResult(name, "output bytes match across identical runs")
}

// runProbe3 validates error reporting on malformed payload input.
func runProbe3(ctx context.Context, binaryPath string, timeout time.Duration) ProbeResult {
	const name = "Malformed Payload Error Handling"
	malformed := []byte("{malformed json")
	res := execPlugin(ctx, binaryPath, malformed, timeout)

	var resp compiler.GenerationResponse
	if err := json.Unmarshal(res.stdout, &resp); err != nil {
		return failedResult(name, "FDN0705", fmt.Sprintf("plugin returned invalid response JSON: %v", err))
	}

	for _, d := range resp.Diagnostics {
		if d.Code == "FDN0701" {
			return passedResult(name, "response contains diagnostic FDN0701")
		}
	}

	return failedResult(name, "FDN0705", "response does not contain required diagnostic FDN0701")
}

// runProbe4 validates that no extra bytes exist outside the response JSON.
func runProbe4(ctx context.Context, binaryPath string, reqBytes []byte, timeout time.Duration) ProbeResult {
	const name = "Standard Output Purity"
	res := execPlugin(ctx, binaryPath, reqBytes, timeout)
	if failure, failed := runFailure(name, runFailureCode, "", res); failed {
		return failure
	}

	if len(bytes.TrimSpace(res.stdout)) == 0 {
		return failedResult(name, "FDN0706", "standard output is empty")
	}

	var resp compiler.GenerationResponse
	if err := json.Unmarshal(res.stdout, &resp); err != nil {
		return failedResult(name, "FDN0706", fmt.Sprintf("standard output contains extra bytes or invalid JSON: %v", err))
	}

	dec := json.NewDecoder(bytes.NewReader(res.stdout))
	var raw json.RawMessage
	if err := dec.Decode(&raw); err != nil {
		return failedResult(name, "FDN0706", fmt.Sprintf("failed to decode JSON response: %v", err))
	}

	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return failedResult(name, "FDN0706", "standard output contains extra data after the JSON document")
	}

	return passedResult(name, "standard output contains only the JSON response document")
}

// runProbe5 validates process exit codes on normal and error runs.
func runProbe5(ctx context.Context, binaryPath string, reqBytes []byte, timeout time.Duration) ProbeResult {
	const name = "Process Exit Code Discipline"
	resNormal := execPlugin(ctx, binaryPath, reqBytes, timeout)
	if resNormal.exitCode != 0 {
		return failedResult(name, "FDN0707", fmt.Sprintf("process exited with code %d during a normal run", resNormal.exitCode))
	}

	malformed := []byte("{malformed json")
	resError := execPlugin(ctx, binaryPath, malformed, timeout)
	if resError.exitCode != 0 {
		return failedResult(name, "FDN0707", fmt.Sprintf("process exited with code %d during a malformed input run", resError.exitCode))
	}

	return passedResult(name, "process exits with code 0 on handled runs and handled errors")
}
