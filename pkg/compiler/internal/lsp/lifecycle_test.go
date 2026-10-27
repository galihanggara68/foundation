package lsp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
)

func TestServer_Lifecycle_InitializeAndShutdown(t *testing.T) {
	initReq := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"rootUri":"file:///workspace"}}`
	initAck := `{"jsonrpc":"2.0","method":"initialized","params":{}}`
	shutReq := `{"jsonrpc":"2.0","id":2,"method":"shutdown","params":null}`
	exitNot := `{"jsonrpc":"2.0","method":"exit","params":{}}`

	input := fmt.Sprintf("Content-Length: %d\r\n\r\n%sContent-Length: %d\r\n\r\n%sContent-Length: %d\r\n\r\n%sContent-Length: %d\r\n\r\n%s",
		len(initReq), initReq, len(initAck), initAck, len(shutReq), shutReq, len(exitNot), exitNot)

	var output bytes.Buffer
	server := NewServer(bytes.NewBufferString(input), &output, func(sources []SourceDoc) ([]ir.Diagnostic, error) {
		return nil, nil
	})

	err := server.Run()
	if err != nil {
		t.Fatalf("unexpected server run error: %v", err)
	}

	reader := NewTransport(&output, &bytes.Buffer{})

	// 1. Check initialize response
	initRespPayload, err := reader.ReadMessage()
	if err != nil {
		t.Fatalf("failed reading initialize response: %v", err)
	}
	var initResp struct {
		JSONRPC string           `json:"jsonrpc"`
		ID      int              `json:"id"`
		Result  InitializeResult `json:"result"`
	}
	if err := json.Unmarshal(initRespPayload, &initResp); err != nil {
		t.Fatalf("failed unmarshaling initialize response: %v", err)
	}
	if initResp.ID != 1 {
		t.Fatalf("expected ID 1, got %d", initResp.ID)
	}
	if initResp.Result.Capabilities.TextDocumentSync != 1 {
		t.Fatalf("expected TextDocumentSync 1, got %d", initResp.Result.Capabilities.TextDocumentSync)
	}
	if !initResp.Result.Capabilities.HoverProvider {
		t.Fatalf("expected HoverProvider true")
	}
	if initResp.Result.Capabilities.CompletionProvider == nil {
		t.Fatalf("expected non-nil CompletionProvider")
	}

	// 2. Check shutdown response
	shutRespPayload, err := reader.ReadMessage()
	if err != nil {
		t.Fatalf("failed reading shutdown response: %v", err)
	}
	var shutResp struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      int             `json:"id"`
		Result  json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(shutRespPayload, &shutResp); err != nil {
		t.Fatalf("failed unmarshaling shutdown response: %v", err)
	}
	if shutResp.ID != 2 {
		t.Fatalf("expected ID 2, got %d", shutResp.ID)
	}
	if string(shutResp.Result) != "null" {
		t.Fatalf("expected null result for shutdown, got %s", string(shutResp.Result))
	}
}

func TestServer_UnknownMethod_ReturnsError(t *testing.T) {
	req := `{"jsonrpc":"2.0","id":99,"method":"custom/nonexistent","params":{}}`
	exit := `{"jsonrpc":"2.0","method":"exit","params":{}}`
	input := fmt.Sprintf("Content-Length: %d\r\n\r\n%sContent-Length: %d\r\n\r\n%s", len(req), req, len(exit), exit)

	var output bytes.Buffer
	server := NewServer(bytes.NewBufferString(input), &output, func(sources []SourceDoc) ([]ir.Diagnostic, error) {
		return nil, nil
	})

	_ = server.Run()

	reader := NewTransport(&output, &bytes.Buffer{})
	respPayload, err := reader.ReadMessage()
	if err != nil {
		t.Fatalf("failed reading response: %v", err)
	}

	var resp ResponseMessage
	if err := json.Unmarshal(respPayload, &resp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if resp.Error == nil {
		t.Fatalf("expected error response for unknown method, got nil")
	}
	if resp.Error.Code != -32601 {
		t.Fatalf("expected error code -32601, got %d", resp.Error.Code)
	}
}
