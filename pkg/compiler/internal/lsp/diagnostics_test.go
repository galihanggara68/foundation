package lsp_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/galihanggara68/foundation/pkg/compiler"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/lsp"
)

func realCompilerFn(sources []lsp.SourceDoc) ([]ir.Diagnostic, error) {
	var cSources []compiler.SourceFile
	for _, s := range sources {
		cSources = append(cSources, compiler.SourceFile{
			Path:    s.Path,
			Content: s.Content,
		})
	}
	_, diags := compiler.Compile(cSources)
	return diags, nil
}

func TestServer_Diagnostics_DidOpenAndDidChange(t *testing.T) {
	uri := "file:///workspace/order.fdn"

	validDoc := "entity Order {\n  id: uuid @pk\n  total: decimal\n}\n"
	openReq := fmt.Sprintf(`{"jsonrpc":"2.0","method":"textDocument/didOpen","params":{"textDocument":{"uri":%q,"languageId":"foundation","version":1,"text":%q}}}`,
		uri, validDoc)

	// Introduce syntax error: missing closing brace
	brokenDoc := "entity Order {\n  id: uuid @pk\n  total: decimal\n"
	changeReq := fmt.Sprintf(`{"jsonrpc":"2.0","method":"textDocument/didChange","params":{"textDocument":{"uri":%q,"version":2},"contentChanges":[{"text":%q}]}}`,
		uri, brokenDoc)

	// Fix document: restore valid syntax
	fixedDoc := "entity Order {\n  id: uuid @pk\n  total: decimal\n}\n"
	fixReq := fmt.Sprintf(`{"jsonrpc":"2.0","method":"textDocument/didChange","params":{"textDocument":{"uri":%q,"version":3},"contentChanges":[{"text":%q}]}}`,
		uri, fixedDoc)

	shutReq := `{"jsonrpc":"2.0","id":99,"method":"shutdown","params":null}`
	exitReq := `{"jsonrpc":"2.0","method":"exit","params":{}}`

	input := fmt.Sprintf("Content-Length: %d\r\n\r\n%sContent-Length: %d\r\n\r\n%sContent-Length: %d\r\n\r\n%sContent-Length: %d\r\n\r\n%sContent-Length: %d\r\n\r\n%s",
		len(openReq), openReq, len(changeReq), changeReq, len(fixReq), fixReq, len(shutReq), shutReq, len(exitReq), exitReq)

	var output bytes.Buffer
	server := lsp.NewServer(bytes.NewBufferString(input), &output, realCompilerFn)

	_ = server.Run()

	reader := lsp.NewTransport(&output, &bytes.Buffer{})

	// 1. Check didOpen notification (should have 0 diagnostics)
	p1, err := reader.ReadMessage()
	if err != nil {
		t.Fatalf("failed reading first diagnostics notification: %v", err)
	}
	var notif1 lsp.NotificationMessage
	if err := json.Unmarshal(p1, &notif1); err != nil {
		t.Fatalf("failed unmarshaling notification 1: %v", err)
	}
	if notif1.Method != "textDocument/publishDiagnostics" {
		t.Fatalf("expected publishDiagnostics, got %q", notif1.Method)
	}
	raw1, _ := json.Marshal(notif1.Params)
	var params1 lsp.PublishDiagnosticsParams
	_ = json.Unmarshal(raw1, &params1)
	if len(params1.Diagnostics) != 0 {
		t.Fatalf("expected 0 diagnostics for valid doc, got %d: %+v", len(params1.Diagnostics), params1.Diagnostics)
	}

	// 2. Check didChange notification with syntax error
	p2, err := reader.ReadMessage()
	if err != nil {
		t.Fatalf("failed reading second diagnostics notification: %v", err)
	}
	var notif2 lsp.NotificationMessage
	_ = json.Unmarshal(p2, &notif2)
	raw2, _ := json.Marshal(notif2.Params)
	var params2 lsp.PublishDiagnosticsParams
	_ = json.Unmarshal(raw2, &params2)
	if len(params2.Diagnostics) == 0 {
		t.Fatalf("expected diagnostics for broken doc, got 0")
	}
	if params2.Diagnostics[0].Severity != lsp.SeverityError {
		t.Fatalf("expected SeverityError (1), got %d", params2.Diagnostics[0].Severity)
	}

	// 3. Check didChange notification with fix
	p3, err := reader.ReadMessage()
	if err != nil {
		t.Fatalf("failed reading third diagnostics notification: %v", err)
	}
	var notif3 lsp.NotificationMessage
	_ = json.Unmarshal(p3, &notif3)
	raw3, _ := json.Marshal(notif3.Params)
	var params3 lsp.PublishDiagnosticsParams
	_ = json.Unmarshal(raw3, &params3)
	if len(params3.Diagnostics) != 0 {
		t.Fatalf("expected 0 diagnostics after fix, got %d: %+v", len(params3.Diagnostics), params3.Diagnostics)
	}
}

func TestServer_Diagnostics_SemanticInvariantViolation(t *testing.T) {
	uri := "file:///workspace/semantic.fdn"
	validDoc := "entity Item {\n  id: uuid @pk\n  name: string\n}\n"
	openReq := fmt.Sprintf(`{"jsonrpc":"2.0","method":"textDocument/didOpen","params":{"textDocument":{"uri":%q,"languageId":"foundation","version":1,"text":%q}}}`,
		uri, validDoc)

	// didChange introducing semantic error: unknown scalar type 'FooBar'
	brokenDoc := "entity Item {\n  id: uuid @pk\n  invalid: FooBar\n}\n"
	changeReq := fmt.Sprintf(`{"jsonrpc":"2.0","method":"textDocument/didChange","params":{"textDocument":{"uri":%q,"version":2},"contentChanges":[{"text":%q}]}}`,
		uri, brokenDoc)

	shutReq := `{"jsonrpc":"2.0","id":99,"method":"shutdown","params":null}`
	exitReq := `{"jsonrpc":"2.0","method":"exit","params":{}}`

	input := fmt.Sprintf("Content-Length: %d\r\n\r\n%sContent-Length: %d\r\n\r\n%sContent-Length: %d\r\n\r\n%sContent-Length: %d\r\n\r\n%s",
		len(openReq), openReq, len(changeReq), changeReq, len(shutReq), shutReq, len(exitReq), exitReq)

	var output bytes.Buffer
	server := lsp.NewServer(bytes.NewBufferString(input), &output, realCompilerFn)

	_ = server.Run()

	reader := lsp.NewTransport(&output, &bytes.Buffer{})

	// Discard first notification (didOpen valid)
	_, _ = reader.ReadMessage()

	// Read second notification (didChange with semantic error)
	p, err := reader.ReadMessage()
	if err != nil {
		t.Fatalf("failed reading semantic error notification: %v", err)
	}

	var notif lsp.NotificationMessage
	_ = json.Unmarshal(p, &notif)
	raw, _ := json.Marshal(notif.Params)
	var params lsp.PublishDiagnosticsParams
	_ = json.Unmarshal(raw, &params)

	if len(params.Diagnostics) == 0 {
		t.Fatalf("expected semantic diagnostic for unknown type FooBar, got 0")
	}
	if params.Diagnostics[0].Code != "FDN0202" {
		t.Fatalf("expected diagnostic code 'FDN0202', got %q", params.Diagnostics[0].Code)
	}
}

func TestServer_Diagnostics_MultiFileCrossReference(t *testing.T) {
	// Setup workspace on disk with users.fdn
	workspaceDir := t.TempDir()
	userFile := filepath.Join(workspaceDir, "users.fdn")
	userContent := "entity User {\n  id: uuid @pk\n  email: string\n}\n"
	if err := os.WriteFile(userFile, []byte(userContent), 0644); err != nil {
		t.Fatalf("failed writing users.fdn: %v", err)
	}

	// Open in-memory orders.fdn in the same workspace referencing User
	orderFile := filepath.Join(workspaceDir, "orders.fdn")
	orderURI := lsp.PathToURI(orderFile)
	orderContent := "entity Order {\n  id: uuid @pk\n  userId: uuid\n}\n"

	initReq := fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"rootUri":%q}}`, lsp.PathToURI(workspaceDir))
	openReq := fmt.Sprintf(`{"jsonrpc":"2.0","method":"textDocument/didOpen","params":{"textDocument":{"uri":%q,"languageId":"foundation","version":1,"text":%q}}}`,
		orderURI, orderContent)
	shutReq := `{"jsonrpc":"2.0","id":2,"method":"shutdown","params":null}`
	exitReq := `{"jsonrpc":"2.0","method":"exit","params":{}}`

	input := fmt.Sprintf("Content-Length: %d\r\n\r\n%sContent-Length: %d\r\n\r\n%sContent-Length: %d\r\n\r\n%sContent-Length: %d\r\n\r\n%s",
		len(initReq), initReq, len(openReq), openReq, len(shutReq), shutReq, len(exitReq), exitReq)

	var output bytes.Buffer
	server := lsp.NewServer(bytes.NewBufferString(input), &output, realCompilerFn)

	_ = server.Run()

	reader := lsp.NewTransport(&output, &bytes.Buffer{})

	// 1. Read initialize response
	_, _ = reader.ReadMessage()

	// 2. Read diagnostics notification for orders.fdn
	p, err := reader.ReadMessage()
	if err != nil {
		t.Fatalf("failed reading diagnostics notification: %v", err)
	}

	var notif lsp.NotificationMessage
	_ = json.Unmarshal(p, &notif)
	raw, _ := json.Marshal(notif.Params)
	var params lsp.PublishDiagnosticsParams
	_ = json.Unmarshal(raw, &params)

	if len(params.Diagnostics) != 0 {
		t.Fatalf("expected 0 diagnostics in multi-file workspace, got %d: %+v", len(params.Diagnostics), params.Diagnostics)
	}
}
