package emitterkit

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/galihanggara68/foundation/pkg/compiler"
)

func TestRunPluginWithFlagsStream_DumpModel_Success(t *testing.T) {
	req := GenerationRequest{
		ProtocolVersion: ProtocolVersion,
		WorkspaceRoot:   "/tmp/work",
		Target: TargetInfo{
			TargetConfig: compiler.TargetConfig{
				Name:     "custom-plugin",
				Kind:     "generator",
				Language: "go",
				OutDir:   "output",
			},
		},
		SemanticIR: &compiler.SemanticIR{},
	}

	reqBytes, err := compiler.EncodeGenerationRequest(req)
	if err != nil {
		t.Fatalf("failed to encode request: %v", err)
	}

	inBuf := bytes.NewBuffer(reqBytes)
	var outBuf bytes.Buffer
	var errBuf bytes.Buffer

	type testModel struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}

	dumpFn := func(r GenerationRequest) (any, error) {
		if r.Target.Name != "custom-plugin" {
			t.Errorf("expected target name 'custom-plugin', got %q", r.Target.Name)
		}
		return testModel{
			Name:  "domain-model",
			Count: 42,
		}, nil
	}

	handlerCalled := false
	handler := func(r GenerationRequest) (GenerationResponse, error) {
		handlerCalled = true
		return GenerationResponse{}, nil
	}

	args := []string{"-dump-model"}
	err = RunPluginWithFlagsStream(args, inBuf, &outBuf, &errBuf, handler, WithDumpModel(dumpFn))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if handlerCalled {
		t.Error("expected handler not to run when -dump-model is active")
	}

	if errBuf.Len() != 0 {
		t.Errorf("expected stderr to remain empty, got %q", errBuf.String())
	}

	expectedJSON := "{\n  \"name\": \"domain-model\",\n  \"count\": 42\n}\n"
	if outBuf.String() != expectedJSON {
		t.Errorf("expected formatted JSON output %q, got %q", expectedJSON, outBuf.String())
	}

	var parsed testModel
	if err := json.Unmarshal(outBuf.Bytes(), &parsed); err != nil {
		t.Fatalf("failed to parse output JSON: %v", err)
	}
	if parsed.Name != "domain-model" || parsed.Count != 42 {
		t.Errorf("parsed model mismatch: %+v", parsed)
	}
}

func TestRunPluginWithFlagsStream_DumpModel_Absent(t *testing.T) {
	req := GenerationRequest{
		ProtocolVersion: ProtocolVersion,
		WorkspaceRoot:   "/tmp/work",
		Target: TargetInfo{
			TargetConfig: compiler.TargetConfig{
				Name:     "custom-plugin",
				Kind:     "generator",
				Language: "go",
				OutDir:   "output",
			},
		},
		SemanticIR: &compiler.SemanticIR{},
	}

	reqBytes, err := compiler.EncodeGenerationRequest(req)
	if err != nil {
		t.Fatalf("failed to encode request: %v", err)
	}

	inBuf := bytes.NewBuffer(reqBytes)
	var outBuf bytes.Buffer
	var errBuf bytes.Buffer

	dumpFnCalled := false
	dumpFn := func(r GenerationRequest) (any, error) {
		dumpFnCalled = true
		return nil, nil
	}

	handlerCalled := false
	handler := func(r GenerationRequest) (GenerationResponse, error) {
		handlerCalled = true
		return GenerationResponse{
			Files: map[string]string{
				"output.go": "package output\n",
			},
		}, nil
	}

	args := []string{}
	err = RunPluginWithFlagsStream(args, inBuf, &outBuf, &errBuf, handler, WithDumpModel(dumpFn))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if dumpFnCalled {
		t.Error("expected dump function not to run when -dump-model is absent")
	}

	if !handlerCalled {
		t.Fatal("expected handler to run when -dump-model is absent")
	}

	resp, err := compiler.DecodeGenerationResponse(&outBuf)
	if err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Files["output.go"] != "package output\n" {
		t.Errorf("expected file content 'package output\\n', got %q", resp.Files["output.go"])
	}
}

func TestRunPluginWithFlagsStream_DumpModel_InvalidJSON(t *testing.T) {
	inBuf := bytes.NewBufferString("{ invalid json ")
	var outBuf bytes.Buffer
	var errBuf bytes.Buffer

	dumpFn := func(r GenerationRequest) (any, error) {
		return map[string]string{"key": "value"}, nil
	}

	handler := func(r GenerationRequest) (GenerationResponse, error) {
		return GenerationResponse{}, nil
	}

	args := []string{"-dump-model"}
	err := RunPluginWithFlagsStream(args, inBuf, &outBuf, &errBuf, handler, WithDumpModel(dumpFn))
	if err == nil {
		t.Fatal("expected error on invalid JSON input, got nil")
	}

	if !strings.Contains(err.Error(), "failed to decode generation request") {
		t.Errorf("expected decode error message, got: %v", err)
	}
}

func TestRunPluginWithFlagsStream_DumpModel_DumperError(t *testing.T) {
	req := GenerationRequest{
		ProtocolVersion: ProtocolVersion,
		WorkspaceRoot:   "/tmp/work",
		Target: TargetInfo{
			TargetConfig: compiler.TargetConfig{
				Name:     "backend",
				Kind:     "backend",
				Language: "go",
				OutDir:   "output",
			},
		},
		SemanticIR: &compiler.SemanticIR{},
	}

	reqBytes, err := compiler.EncodeGenerationRequest(req)
	if err != nil {
		t.Fatalf("failed to encode request: %v", err)
	}

	inBuf := bytes.NewBuffer(reqBytes)
	var outBuf bytes.Buffer
	var errBuf bytes.Buffer

	dumpFn := func(r GenerationRequest) (any, error) {
		return nil, errors.New("domain model transformation failed")
	}

	handler := func(r GenerationRequest) (GenerationResponse, error) {
		return GenerationResponse{}, nil
	}

	args := []string{"-dump-model"}
	err = RunPluginWithFlagsStream(args, inBuf, &outBuf, &errBuf, handler, WithDumpModel(dumpFn))
	if err == nil {
		t.Fatal("expected error when model dumper fails, got nil")
	}

	if !strings.Contains(err.Error(), "domain model transformation failed") {
		t.Errorf("expected transformation error message, got: %v", err)
	}
}

func TestRunPluginWithFlagsStream_DumpModel_NotConfigured(t *testing.T) {
	req := GenerationRequest{
		ProtocolVersion: ProtocolVersion,
		WorkspaceRoot:   "/tmp/work",
		Target: TargetInfo{
			TargetConfig: compiler.TargetConfig{
				Name:     "backend",
				Kind:     "backend",
				Language: "go",
				OutDir:   "output",
			},
		},
		SemanticIR: &compiler.SemanticIR{},
	}

	reqBytes, err := compiler.EncodeGenerationRequest(req)
	if err != nil {
		t.Fatalf("failed to encode request: %v", err)
	}

	inBuf := bytes.NewBuffer(reqBytes)
	var outBuf bytes.Buffer
	var errBuf bytes.Buffer

	handler := func(r GenerationRequest) (GenerationResponse, error) {
		return GenerationResponse{}, nil
	}

	args := []string{"-dump-model"}
	err = RunPluginWithFlagsStream(args, inBuf, &outBuf, &errBuf, handler)
	if err == nil {
		t.Fatal("expected error when model dumper is not configured, got nil")
	}

	if !strings.Contains(err.Error(), "model dumper is not configured") {
		t.Errorf("expected unconfigured dumper error, got: %v", err)
	}
}

func TestRunPluginWithFlagsStream_FlagParseError(t *testing.T) {
	var inBuf bytes.Buffer
	var outBuf bytes.Buffer
	var errBuf bytes.Buffer

	handler := func(r GenerationRequest) (GenerationResponse, error) {
		return GenerationResponse{}, nil
	}

	args := []string{"-unknown-flag"}
	err := RunPluginWithFlagsStream(args, &inBuf, &outBuf, &errBuf, handler)
	if err == nil {
		t.Fatal("expected error for unknown flag, got nil")
	}
}
