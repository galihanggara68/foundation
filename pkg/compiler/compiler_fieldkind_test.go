package compiler_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/galihanggara68/foundation/pkg/compiler"
)

func TestCheck_UnclassifiableFieldType_ReturnsFDN0218(t *testing.T) {
	dir := t.TempDir()
	src := `
entity Broken {
  id: uuid @pk
  mystery: strng
}
`
	if err := os.WriteFile(filepath.Join(dir, "broken.fdn"), []byte(src), 0644); err != nil {
		t.Fatalf("write blueprint: %v", err)
	}

	_, diags := compiler.Check(dir)
	found := false
	for _, d := range diags {
		if d.Code == "FDN0218" {
			found = true
		}
		if d.Code == "FDN0202" {
			t.Errorf("unclassifiable field types must report FDN0218, not legacy FDN0202: %+v", d)
		}
	}
	if !found {
		t.Fatalf("expected FDN0218 from compiler.Check, got: %+v", diags)
	}
}

func TestIR_EmitsFieldKind(t *testing.T) {
	dir := t.TempDir()
	src := `
enum OrderStatus {
  open
  closed
}

entity Ticket {
  id: uuid @pk
  title: string
  status: OrderStatus
  tags: []string
}
`
	if err := os.WriteFile(filepath.Join(dir, "ticket.fdn"), []byte(src), 0644); err != nil {
		t.Fatalf("write blueprint: %v", err)
	}

	irJSON, diags := compiler.IR(dir)
	if len(diags) != 0 {
		t.Fatalf("expected compiler.IR to succeed, got: %+v", diags)
	}

	for _, want := range []string{
		`"kind": "uuid"`,
		`"kind": "string"`,
		`"kind": "enum"`,
	} {
		if !strings.Contains(string(irJSON), want) {
			t.Errorf("expected IR JSON to contain %s, got:\n%s", want, irJSON)
		}
	}
}
