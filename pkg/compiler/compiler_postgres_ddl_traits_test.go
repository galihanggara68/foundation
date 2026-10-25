package compiler_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/galihanggara68/foundation/pkg/compiler"
)

func TestBuild_PostgresDeclarativeDDLAndMigrationsForTacticalTraits(t *testing.T) {
	dir := t.TempDir()

	bpWithTraits := `
entity Account {
  id: uuid @pk
  balance: int
}

event AccountDebited {
  account_id: uuid
  amount: int
}

action DebitAccount(id: uuid, amount: int) -> Account
  @idempotent(key: id)
  @transactional
  @outbox
{
  let acc = query one Account where id == id else AccountNotFound
  update Account where id == acc.id set {
    balance: acc.balance - amount
  }
  emit AccountDebited {
    account_id: acc.id,
    amount: amount
  }
  return acc
}

error AccountNotFound {
  kind: not_found
  message: "account not found"
}
`

	buildProfile := `
project:
  name: banking
  module: example.com/banking

storage:
  dialect: postgres
  mode: both

targets:
  - name: backend
    kind: backend
    language: go
    architecture: hexagonal
    http: chi
    out_dir: ./dist/backend
`

	if err := os.WriteFile(filepath.Join(dir, "banking.fdn"), []byte(bpWithTraits), 0644); err != nil {
		t.Fatalf("write banking.fdn: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "foundation.build.yaml"), []byte(buildProfile), 0644); err != nil {
		t.Fatalf("write foundation.build.yaml: %v", err)
	}

	_, diags := compiler.Build(dir, compiler.BuildOptions{})
	if len(diags) > 0 {
		t.Fatalf("build failed with diagnostics: %+v", diags)
	}

	outDir := filepath.Join(dir, "dist", "backend")
	schemaBytes, err := os.ReadFile(filepath.Join(outDir, "schema.sql"))
	if err != nil {
		t.Fatalf("failed to read schema.sql: %v", err)
	}
	schemaSQL := string(schemaBytes)

	// Verify idempotency_keys and outbox_events tables in declarative schema
	expectedTables := []string{
		"CREATE TABLE idempotency_keys (",
		"key TEXT PRIMARY KEY",
		"status TEXT NOT NULL",
		"response_payload BYTEA",
		"expires_at TIMESTAMPTZ NOT NULL",
		"CREATE TABLE outbox_events (",
		"id TEXT PRIMARY KEY",
		"topic TEXT NOT NULL",
		"event_name TEXT NOT NULL",
		"payload BYTEA NOT NULL",
		"published BOOLEAN NOT NULL DEFAULT FALSE",
		"CREATE INDEX idx_outbox_events_published ON outbox_events (published, created_at);",
	}
	for _, exp := range expectedTables {
		if !strings.Contains(schemaSQL, exp) {
			t.Errorf("expected schema.sql to contain %q, but got:\n%s", exp, schemaSQL)
		}
	}

	// Verify 0001_initial migration up and down
	migUpBytes, err := os.ReadFile(filepath.Join(outDir, "migrations", "0001_initial.up.sql"))
	if err != nil {
		t.Fatalf("failed to read initial up migration: %v", err)
	}
	if !strings.Contains(string(migUpBytes), "CREATE TABLE idempotency_keys") || !strings.Contains(string(migUpBytes), "CREATE TABLE outbox_events") {
		t.Errorf("expected initial up migration to contain tactical tables, got:\n%s", string(migUpBytes))
	}

	migDownBytes, err := os.ReadFile(filepath.Join(outDir, "migrations", "0001_initial.down.sql"))
	if err != nil {
		t.Fatalf("failed to read initial down migration: %v", err)
	}
	migDown := string(migDownBytes)
	if !strings.Contains(migDown, "DROP TABLE IF EXISTS outbox_events CASCADE;") || !strings.Contains(migDown, "DROP TABLE IF EXISTS idempotency_keys CASCADE;") {
		t.Errorf("expected initial down migration to drop tactical tables, got:\n%s", migDown)
	}
}
