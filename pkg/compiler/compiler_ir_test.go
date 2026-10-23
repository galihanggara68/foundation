package compiler_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/galihanggara68/foundation/pkg/compiler"
)

func TestIR_CanonicalJSONRoundTrip_IngestsAndBuildsIdenticallyToFDN(t *testing.T) {
	srcDir := t.TempDir()

	blueprint := `
enum TicketStatus {
  open
  resolved
}

value OpenTicketInput {
  subject: string
}

entity SupportTicket {
  id: uuid @pk
  subject: string
  status: TicketStatus = .open
}

event TicketOpened {
  ticket_id: uuid
  subject: string
}

action OpenTicket(input: OpenTicketInput) -> SupportTicket
  @http(method: POST, path: "/v1/tickets")
{
  let ticket = create SupportTicket {
    subject: input.subject,
    status: .open
  }
  emit TicketOpened {
    ticket_id: ticket.id,
    subject: ticket.subject
  }
  return ticket
}

scenario "OpenTicket creates open ticket and emits event" {
  when OpenTicket(OpenTicketInput {
    subject: "Login issue"
  })
  expect result.subject == "Login issue"
  expect result.status == .open
  expect emitted TicketOpened {
    subject: "Login issue"
  }
}
`
	if err := os.WriteFile(filepath.Join(srcDir, "support.fdn"), []byte(blueprint), 0644); err != nil {
		t.Fatalf("write support.fdn: %v", err)
	}

	irBytes1, diags := compiler.IR(srcDir)
	if len(diags) != 0 {
		t.Fatalf("expected compiler.IR to succeed, got: %+v", diags)
	}

	jsonDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(jsonDir, "support.fdn.json"), irBytes1, 0644); err != nil {
		t.Fatalf("write support.fdn.json: %v", err)
	}

	irBytes2, diags := compiler.IR(jsonDir)
	if len(diags) != 0 {
		t.Fatalf("expected compiler.IR on .fdn.json workspace to succeed, got: %+v", diags)
	}
	if string(irBytes1) != string(irBytes2) {
		t.Fatalf("expected round-trip SemanticIR JSON to be byte-for-byte identical:\ngot:\n%s\nwant:\n%s", string(irBytes2), string(irBytes1))
	}

	buildProfile := `
project:
  name: supportdesk
  module: example.com/supportdesk

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
	if err := os.WriteFile(filepath.Join(jsonDir, "foundation.build.yaml"), []byte(buildProfile), 0644); err != nil {
		t.Fatalf("write foundation.build.yaml: %v", err)
	}

	if _, buildDiags := compiler.Build(jsonDir, compiler.BuildOptions{}); len(buildDiags) != 0 {
		t.Fatalf("expected compiler.Build on .fdn.json workspace to succeed, got: %+v", buildDiags)
	}
	if testDiags := compiler.Test(jsonDir); len(testDiags) != 0 {
		t.Fatalf("expected generated scenarios from .fdn.json build to pass, got: %+v", testDiags)
	}
}
