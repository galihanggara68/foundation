package compiler_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/galihanggara68/foundation/pkg/compiler"
)

func TestBuild_TimestampAndDecimalTypes_CompilesAndPassesTests(t *testing.T) {
	dir := t.TempDir()

	bp := `project:
  name: accounting
  module: example.com/accounting

storage:
  dialect: postgres
  mode: both

targets:
  - name: backend
    kind: backend
    language: go
    architecture: hexagonal
    http: stdlib
    out_dir: gen
`
	if err := os.WriteFile(filepath.Join(dir, "foundation.build.yaml"), []byte(bp), 0644); err != nil {
		t.Fatalf("write build profile: %v", err)
	}

	fdn := `
entity LedgerEntry {
  id: uuid @pk
  amount: decimal
  posted_at: timestamp @auto_now_add
}

action PostEntry(val: decimal) -> LedgerEntry {
  let e = create LedgerEntry {
    amount: val
  }
  return e
}

scenario "Decimal operations and timestamp validations run precisely" {
  when PostEntry(100.2500)
  expect result.amount == 100.2500
}
`
	if err := os.WriteFile(filepath.Join(dir, "accounting.fdn"), []byte(fdn), 0644); err != nil {
		t.Fatalf("write blueprint: %v", err)
	}

	res, diags := compiler.Build(dir, compiler.BuildOptions{})
	if len(diags) != 0 {
		t.Fatalf("build failed: %+v", diags)
	}
	_ = res

	// Verify schema.sql uses NUMERIC(18, 4) and TIMESTAMPTZ
	schemaBytes, err := os.ReadFile(filepath.Join(dir, "gen/schema.sql"))
	if err != nil {
		t.Fatalf("read schema.sql: %v", err)
	}
	schema := string(schemaBytes)
	if !strings.Contains(schema, "NUMERIC(18, 4)") {
		t.Fatalf("expected NUMERIC(18, 4) in schema.sql, got:\n%s", schema)
	}
	if !strings.Contains(schema, "TIMESTAMPTZ") {
		t.Fatalf("expected TIMESTAMPTZ in schema.sql, got:\n%s", schema)
	}

	// Verify entity definition uses Decimal and time.Time
	entityBytes, err := os.ReadFile(filepath.Join(dir, "gen/internal/domain/ledgerentry_entity.go"))
	if err != nil {
		t.Fatalf("read ledgerentry_entity.go: %v", err)
	}
	entityStr := string(entityBytes)
	if !strings.Contains(entityStr, "Decimal") {
		t.Fatalf("expected Decimal type in entity, got:\n%s", entityStr)
	}
	if !strings.Contains(entityStr, "time.Time") {
		t.Fatalf("expected time.Time in entity, got:\n%s", entityStr)
	}

	// Run generated tests
	if testDiags := compiler.Test(dir); len(testDiags) != 0 {
		t.Fatalf("generated test suite failed: %+v", testDiags)
	}
}
