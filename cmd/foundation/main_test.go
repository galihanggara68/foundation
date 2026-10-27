package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLI_CheckBuildTestAndIR(t *testing.T) {
	dir := t.TempDir()

	blueprint := `
entity Item {
  id: uuid @pk
  code: string
}

error ItemNotFound {
  kind: not_found
  message: "Item not found"
}

action CreateItem(code: string) -> Item
  @http(method: POST, path: "/v1/items")
{
  let item = create Item { code: code }
  return item
}

scenario "CreateItem succeeds" {
  when CreateItem("ITM-1")
  expect result.code == "ITM-1"
}
`
	buildProfile := `
project:
  name: inventory
  module: example.com/inventory

storage:
  dialect: postgres
  mode: declarative

targets:
  - name: backend
    kind: backend
    language: go
    architecture: hexagonal
    http: chi
    out_dir: ./dist/backend
`
	if err := os.WriteFile(filepath.Join(dir, "inventory.fdn"), []byte(blueprint), 0644); err != nil {
		t.Fatalf("write blueprint: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "foundation.build.yaml"), []byte(buildProfile), 0644); err != nil {
		t.Fatalf("write build profile: %v", err)
	}

	var stdout, stderr bytes.Buffer

	if code := run([]string{"check", "--dir", dir}, &stdout, &stderr); code != 0 {
		t.Fatalf("check failed (code %d): %s", code, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"ir", "--dir", dir}, &stdout, &stderr); code != 0 {
		t.Fatalf("ir failed (code %d): %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"CreateItem"`) {
		t.Fatalf("expected IR JSON output to contain CreateItem, got: %s", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"build", "--dir", dir}, &stdout, &stderr); code != 0 {
		t.Fatalf("build failed (code %d): %s", code, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"test", "--dir", dir}, &stdout, &stderr); code != 0 {
		t.Fatalf("test failed (code %d): %s", code, stderr.String())
	}
}

func TestCLI_LSPCommand(t *testing.T) {
	shutReq := `{"jsonrpc":"2.0","id":1,"method":"shutdown","params":null}`
	exitNotif := `{"jsonrpc":"2.0","method":"exit","params":{}}`
	frame := fmt.Sprintf("Content-Length: %d\r\n\r\n%sContent-Length: %d\r\n\r\n%s",
		len(shutReq), shutReq, len(exitNotif), exitNotif)

	stdin := strings.NewReader(frame)
	var stdout, stderr bytes.Buffer

	code := runWithStdin([]string{"lsp", "--debug"}, stdin, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("lsp command failed with code %d: %s", code, stderr.String())
	}
}
