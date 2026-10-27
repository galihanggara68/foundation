package e2e_test

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestE2E_FoundationLSP_LifecycleAndFeatures(t *testing.T) {
	// 1. Build foundation CLI binary
	tmpDir := t.TempDir()
	binPath := filepath.Join(tmpDir, "foundation")
	repoRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("failed to resolve repo root: %v", err)
	}

	buildCmd := exec.Command("go", "build", "-o", binPath, "./cmd/foundation")
	buildCmd.Dir = repoRoot
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build foundation CLI: %v\noutput: %s", err, string(out))
	}

	// 2. Spawn 'foundation lsp' process
	cmd := exec.Command(binPath, "lsp", "--debug")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("failed to open stdin pipe: %v", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("failed to open stdout pipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start foundation lsp: %v", err)
	}
	defer func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	}()

	reader := bufio.NewReader(stdout)

	sendMsg := func(body string) {
		frame := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(body), body)
		if _, wErr := stdin.Write([]byte(frame)); wErr != nil {
			t.Fatalf("failed writing message to lsp stdin: %v", wErr)
		}
	}

	readMsg := func() string {
		type res struct {
			payload string
			err     error
		}
		ch := make(chan res, 1)
		go func() {
			contentLength := -1
			for {
				line, rErr := reader.ReadString('\n')
				if rErr != nil {
					ch <- res{err: rErr}
					return
				}
				trimmed := strings.TrimRight(line, "\r\n")
				if trimmed == "" {
					break
				}
				parts := strings.SplitN(trimmed, ":", 2)
				if len(parts) == 2 && strings.EqualFold(strings.TrimSpace(parts[0]), "Content-Length") {
					contentLength, _ = strconv.Atoi(strings.TrimSpace(parts[1]))
				}
			}
			if contentLength <= 0 {
				ch <- res{err: fmt.Errorf("invalid or missing Content-Length: %d", contentLength)}
				return
			}
			buf := make([]byte, contentLength)
			if _, rErr := io.ReadFull(reader, buf); rErr != nil {
				ch <- res{err: rErr}
				return
			}
			ch <- res{payload: string(buf)}
		}()

		select {
		case r := <-ch:
			if r.err != nil {
				t.Fatalf("readMsg failed: %v", r.err)
			}
			return r.payload
		case <-time.After(5 * time.Second):
			t.Fatalf("timed out waiting for LSP message from server")
			return ""
		}
	}

	// 3. Test 'initialize' request
	sendMsg(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"rootUri":"file:///workspace"}}`)
	initRespRaw := readMsg()
	var initResp struct {
		ID     int `json:"id"`
		Result struct {
			Capabilities struct {
				TextDocumentSync   int  `json:"textDocumentSync"`
				HoverProvider      bool `json:"hoverProvider"`
				CompletionProvider any  `json:"completionProvider"`
			} `json:"capabilities"`
			ServerInfo struct {
				Name string `json:"name"`
			} `json:"serverInfo"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(initRespRaw), &initResp); err != nil {
		t.Fatalf("failed parsing initialize response: %v\nraw: %s", err, initRespRaw)
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
		t.Fatalf("expected CompletionProvider to be non-nil")
	}

	// 4. Send 'initialized' notification
	sendMsg(`{"jsonrpc":"2.0","method":"initialized","params":{}}`)

	// 5. Test 'textDocument/didOpen' with valid Blueprint
	fileURI := "file:///workspace/order.fdn"
	docText := "entity Order @table(name = \"orders\") {\n  id: uuid @pk\n  version: int @version\n}\n"
	didOpenPayload := fmt.Sprintf(`{"jsonrpc":"2.0","method":"textDocument/didOpen","params":{"textDocument":{"uri":%q,"languageId":"foundation","version":1,"text":%q}}}`,
		fileURI, docText)
	sendMsg(didOpenPayload)

	// Expect publishDiagnostics with 0 errors
	diagRespRaw := readMsg()
	var diagNotif struct {
		Method string `json:"method"`
		Params struct {
			URI         string `json:"uri"`
			Diagnostics []any  `json:"diagnostics"`
		} `json:"params"`
	}
	if err := json.Unmarshal([]byte(diagRespRaw), &diagNotif); err != nil {
		t.Fatalf("failed parsing didOpen diagnostics: %v", err)
	}
	if diagNotif.Method != "textDocument/publishDiagnostics" {
		t.Fatalf("expected publishDiagnostics, got %q", diagNotif.Method)
	}
	if len(diagNotif.Params.Diagnostics) != 0 {
		t.Fatalf("expected 0 diagnostics for valid schema, got %d", len(diagNotif.Params.Diagnostics))
	}

	// 6. Test 'textDocument/hover' on '@table' (Line 0, Col 15)
	hoverReq := fmt.Sprintf(`{"jsonrpc":"2.0","id":2,"method":"textDocument/hover","params":{"textDocument":{"uri":%q},"position":{"line":0,"character":15}}}`, fileURI)
	sendMsg(hoverReq)
	hoverRespRaw := readMsg()
	var hoverResp struct {
		ID     int `json:"id"`
		Result struct {
			Contents struct {
				Kind  string `json:"kind"`
				Value string `json:"value"`
			} `json:"contents"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(hoverRespRaw), &hoverResp); err != nil {
		t.Fatalf("failed parsing hover response: %v", err)
	}
	if hoverResp.ID != 2 {
		t.Fatalf("expected ID 2, got %d", hoverResp.ID)
	}
	if !strings.Contains(hoverResp.Result.Contents.Value, "@table") {
		t.Fatalf("expected hover markdown to document @table, got: %s", hoverResp.Result.Contents.Value)
	}

	// 7. Test 'textDocument/completion' with '@' prefix
	compReq := fmt.Sprintf(`{"jsonrpc":"2.0","id":3,"method":"textDocument/completion","params":{"textDocument":{"uri":%q},"position":{"line":0,"character":14}}}`, fileURI)
	sendMsg(compReq)
	compRespRaw := readMsg()
	var compResp struct {
		ID     int `json:"id"`
		Result struct {
			Items []struct {
				Label string `json:"label"`
			} `json:"items"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(compRespRaw), &compResp); err != nil {
		t.Fatalf("failed parsing completion response: %v", err)
	}
	if compResp.ID != 3 {
		t.Fatalf("expected ID 3, got %d", compResp.ID)
	}
	hasTableCompletion := false
	for _, item := range compResp.Result.Items {
		if item.Label == "@table" {
			hasTableCompletion = true
			break
		}
	}
	if !hasTableCompletion {
		t.Fatalf("expected completion items to include '@table'")
	}

	// 8. Test 'textDocument/didChange' introducing syntax error
	brokenText := "entity Order {\n  id: uuid\n"
	didChangePayload := fmt.Sprintf(`{"jsonrpc":"2.0","method":"textDocument/didChange","params":{"textDocument":{"uri":%q,"version":2},"contentChanges":[{"text":%q}]}}`,
		fileURI, brokenText)
	sendMsg(didChangePayload)

	changeDiagRaw := readMsg()
	var changeNotif struct {
		Method string `json:"method"`
		Params struct {
			Diagnostics []struct {
				Severity int `json:"severity"`
			} `json:"diagnostics"`
		} `json:"params"`
	}
	if err := json.Unmarshal([]byte(changeDiagRaw), &changeNotif); err != nil {
		t.Fatalf("failed parsing didChange diagnostics: %v", err)
	}
	if len(changeNotif.Params.Diagnostics) == 0 {
		t.Fatalf("expected diagnostic errors for incomplete document, got 0")
	}

	// 9. Test 'shutdown' request
	sendMsg(`{"jsonrpc":"2.0","id":4,"method":"shutdown","params":null}`)
	shutRespRaw := readMsg()
	var shutResp struct {
		ID     int             `json:"id"`
		Result json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal([]byte(shutRespRaw), &shutResp); err != nil {
		t.Fatalf("failed parsing shutdown response: %v", err)
	}
	if shutResp.ID != 4 || string(shutResp.Result) != "null" {
		t.Fatalf("expected shutdown ID 4 and null result, got ID=%d result=%s", shutResp.ID, string(shutResp.Result))
	}

	// 10. Test 'exit' notification
	sendMsg(`{"jsonrpc":"2.0","method":"exit","params":{}}`)

	// Wait for process to exit cleanly
	waitErr := cmd.Wait()
	if waitErr != nil {
		t.Fatalf("expected clean process exit, got: %v", waitErr)
	}
}
