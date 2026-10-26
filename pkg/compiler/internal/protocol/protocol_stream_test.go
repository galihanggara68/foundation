package protocol_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/arch"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/protocol"
)

func TestStreams(t *testing.T) {
	req := protocol.GenerationRequest{
		ProtocolVersion: protocol.ProtocolVersion,
		WorkspaceRoot:   "/workspace",
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

	var reqBuf bytes.Buffer
	if err := protocol.WriteRequest(&reqBuf, req); err != nil {
		t.Fatalf("WriteRequest failed: %v", err)
	}

	readReq, err := protocol.ReadRequest(&reqBuf)
	if err != nil {
		t.Fatalf("ReadRequest failed: %v", err)
	}
	if readReq.Target.Name != req.Target.Name {
		t.Errorf("expected target name %q, got %q", req.Target.Name, readReq.Target.Name)
	}

	resp := protocol.GenerationResponse{
		Files:       map[string]string{"main.go": "package main"},
		TestCommand: "go test",
	}

	var respBuf bytes.Buffer
	if err := protocol.WriteResponse(&respBuf, resp); err != nil {
		t.Fatalf("WriteResponse failed: %v", err)
	}

	readResp, err := protocol.ReadResponse(&respBuf)
	if err != nil {
		t.Fatalf("ReadResponse failed: %v", err)
	}
	if readResp.Files["main.go"] != "package main" {
		t.Errorf("expected main.go content, got %v", readResp.Files)
	}
}

func TestTargetInfoFromConfig(t *testing.T) {
	cfg := arch.TargetConfig{
		Name:         "svc",
		Kind:         "backend",
		Language:     "go",
		Architecture: "hexagonal",
		HTTP:         "chi",
		OutDir:       "services/svc",
	}
	opts := map[string]string{"foo": "bar"}

	info := protocol.TargetInfoFromConfig(cfg, opts)
	if info.Name != cfg.Name || info.Kind != cfg.Kind || info.Language != cfg.Language ||
		info.Architecture != cfg.Architecture || info.HTTP != cfg.HTTP || info.OutDir != cfg.OutDir {
		t.Errorf("TargetInfoFromConfig field mismatch: %+v", info)
	}
	if info.Options["foo"] != "bar" {
		t.Errorf("TargetInfoFromConfig Options mismatch: %+v", info.Options)
	}

	infoNilOpts := protocol.TargetInfoFromConfig(cfg, nil)
	if infoNilOpts.Options == nil {
		t.Error("expected non-nil Options map when passed nil")
	}
}

func TestJSONSchemas(t *testing.T) {
	reqSchema := protocol.RequestJSONSchema()
	respSchema := protocol.ResponseJSONSchema()

	if strings.TrimSpace(reqSchema) == "" {
		t.Fatal("expected non-empty RequestJSONSchema")
	}
	if strings.TrimSpace(respSchema) == "" {
		t.Fatal("expected non-empty ResponseJSONSchema")
	}

	var parsedReq map[string]any
	if err := json.Unmarshal([]byte(reqSchema), &parsedReq); err != nil {
		t.Fatalf("RequestJSONSchema is invalid JSON: %v", err)
	}
	if parsedReq["$schema"] == nil {
		t.Errorf("RequestJSONSchema missing $schema: %v", parsedReq)
	}

	var parsedResp map[string]any
	if err := json.Unmarshal([]byte(respSchema), &parsedResp); err != nil {
		t.Fatalf("ResponseJSONSchema is invalid JSON: %v", err)
	}
	if parsedResp["$schema"] == nil {
		t.Errorf("ResponseJSONSchema missing $schema: %v", parsedResp)
	}
}

func TestDecodeErrors(t *testing.T) {
	if _, err := protocol.DecodeRequest([]byte(`{invalid-json`)); err == nil {
		t.Error("expected error decoding invalid JSON request")
	}
	if _, err := protocol.DecodeResponse([]byte(`{invalid-json`)); err == nil {
		t.Error("expected error decoding invalid JSON response")
	}

	// Broken writer and reader
	errWriter := &failingWriter{}
	if err := protocol.WriteRequest(errWriter, protocol.GenerationRequest{}); err == nil {
		t.Error("expected error writing request to failing writer")
	}
	if err := protocol.WriteResponse(errWriter, protocol.GenerationResponse{}); err == nil {
		t.Error("expected error writing response to failing writer")
	}

	errReader := &failingReader{}
	if _, err := protocol.ReadRequest(errReader); err == nil {
		t.Error("expected error reading request from failing reader")
	}
	if _, err := protocol.ReadResponse(errReader); err == nil {
		t.Error("expected error reading response from failing reader")
	}

	// EncodeRequest defaults ProtocolVersion if zero
	reqZero := protocol.GenerationRequest{}
	encoded, err := protocol.EncodeRequest(reqZero)
	if err != nil {
		t.Fatalf("EncodeRequest failed: %v", err)
	}
	if !strings.Contains(string(encoded), `"protocol_version": 1`) {
		t.Errorf("expected protocol_version: 1 in encoded output: %s", string(encoded))
	}

	// Validate with future protocol version
	reqFuture := protocol.GenerationRequest{
		ProtocolVersion: 99,
		WorkspaceRoot:   "/workspace",
		Target: protocol.TargetInfo{
			TargetConfig: arch.TargetConfig{
				Name:     "backend",
				Kind:     "backend",
				Language: "go",
				OutDir:   "gen",
			},
		},
		SemanticIR:      &ir.SemanticIR{},
	}
	if err := reqFuture.Validate(); err == nil {
		t.Error("expected error validating request with future protocol version")
	}
}

func TestGenerationResponse_Helpers(t *testing.T) {
	var resp protocol.GenerationResponse
	if resp.HasErrors() {
		t.Error("expected false for HasErrors on empty response")
	}

	resp.SetFile("file.go", "content")
	resp.AddStub("stub.go", "stub-content")
	resp.AddDiagnostic(ir.Diagnostic{Code: "FDN0001", Severity: ir.SeverityWarning, Message: "warn"})

	if resp.HasErrors() {
		t.Error("expected false for HasErrors with only warning diagnostic")
	}
	if resp.Files["file.go"] != "content" {
		t.Errorf("expected file.go content, got %q", resp.Files["file.go"])
	}
	if resp.Stubs["stub.go"] != "stub-content" {
		t.Errorf("expected stub.go content, got %q", resp.Stubs["stub.go"])
	}

	resp.AddDiagnostic(ir.Diagnostic{Code: "FDN0002", Severity: ir.SeverityError, Message: "err"})
	if !resp.HasErrors() {
		t.Error("expected true for HasErrors after adding error diagnostic")
	}
}

type failingWriter struct{}

func (f *failingWriter) Write(p []byte) (n int, err error) {
	return 0, bytes.ErrTooLarge
}

type failingReader struct{}

func (f *failingReader) Read(p []byte) (n int, err error) {
	return 0, bytes.ErrTooLarge
}
