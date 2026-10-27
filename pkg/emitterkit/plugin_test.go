package emitterkit

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/galihanggara68/foundation/pkg/compiler"
)

func TestRunPluginStream_Success(t *testing.T) {
	req := GenerationRequest{
		ProtocolVersion: ProtocolVersion,
		WorkspaceRoot:   "/tmp/test",
		Target: TargetInfo{
			TargetConfig: compiler.TargetConfig{
				Name:     "backend",
				Kind:     "backend",
				Language: "go",
				OutDir:   "gen",
			},
		},
		SemanticIR: &compiler.SemanticIR{},
	}

	reqBytes, err := compiler.EncodeGenerationRequest(req)
	if err != nil {
		t.Fatalf("failed to encode request: %v", err)
	}

	var inBuf bytes.Buffer
	inBuf.Write(reqBytes)
	var outBuf bytes.Buffer

	handler := func(r GenerationRequest) (GenerationResponse, error) {
		if r.Target.Name != "backend" {
			t.Errorf("expected target name 'backend', got %q", r.Target.Name)
		}
		return GenerationResponse{
			Files: map[string]string{
				"main.go": "package main\n",
			},
			TestCommand: "go test ./...",
		}, nil
	}

	err = RunPluginStream(&inBuf, &outBuf, handler)
	if err != nil {
		t.Fatalf("unexpected stream error: %v", err)
	}

	resp, err := compiler.DecodeGenerationResponse(&outBuf)
	if err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Files["main.go"] != "package main\n" {
		t.Errorf("expected main.go content, got %q", resp.Files["main.go"])
	}
	if resp.TestCommand != "go test ./..." {
		t.Errorf("expected test_command 'go test ./...', got %q", resp.TestCommand)
	}
	if len(resp.Diagnostics) != 0 {
		t.Errorf("expected 0 diagnostics, got: %+v", resp.Diagnostics)
	}
}

func TestRunPluginStream_InvalidJSON(t *testing.T) {
	inBuf := bytes.NewBufferString("{ invalid json ")
	var outBuf bytes.Buffer

	err := RunPluginStream(inBuf, &outBuf, func(r GenerationRequest) (GenerationResponse, error) {
		return GenerationResponse{}, nil
	})
	if err != nil {
		t.Fatalf("unexpected stream error: %v", err)
	}

	resp, err := compiler.DecodeGenerationResponse(&outBuf)
	if err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if len(resp.Diagnostics) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d: %+v", len(resp.Diagnostics), resp.Diagnostics)
	}
	if resp.Diagnostics[0].Code != "FDN0701" {
		t.Errorf("expected diagnostic FDN0701, got: %s", resp.Diagnostics[0].Code)
	}
	if !strings.Contains(resp.Diagnostics[0].Message, "failed to decode generation request") {
		t.Errorf("expected message to mention decode failure, got: %s", resp.Diagnostics[0].Message)
	}
}

func TestRunPluginStream_HandlerError(t *testing.T) {
	req := GenerationRequest{
		ProtocolVersion: ProtocolVersion,
		WorkspaceRoot:   "/tmp/test",
		Target: TargetInfo{
			TargetConfig: compiler.TargetConfig{
				Name:     "backend",
				Kind:     "backend",
				Language: "go",
				OutDir:   "gen",
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

	handler := func(r GenerationRequest) (GenerationResponse, error) {
		return GenerationResponse{}, errors.New("custom domain transformation failed")
	}

	err = RunPluginStream(inBuf, &outBuf, handler)
	if err != nil {
		t.Fatalf("unexpected stream error: %v", err)
	}

	resp, err := compiler.DecodeGenerationResponse(&outBuf)
	if err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp.Diagnostics) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d: %+v", len(resp.Diagnostics), resp.Diagnostics)
	}
	if resp.Diagnostics[0].Code != "FDN0701" {
		t.Errorf("expected FDN0701, got: %s", resp.Diagnostics[0].Code)
	}
	if !strings.Contains(resp.Diagnostics[0].Message, "custom domain transformation failed") {
		t.Errorf("expected message to contain handler error, got: %s", resp.Diagnostics[0].Message)
	}
}
