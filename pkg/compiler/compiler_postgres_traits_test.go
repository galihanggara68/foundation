package compiler_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/galihanggara68/foundation/pkg/compiler"
)

func TestBuild_PostgresDeclarativeSchemaAndSchemaDiffingMigrations_HonorsRenamedFromAndRejectsUnhintedDrops(t *testing.T) {
	dir := t.TempDir()

	blueprintV1 := `
enum InvoiceStatus {
  draft
  paid
}

entity Customer {
  id: uuid @pk
  email: string @unique
  full_name: string @index
}

entity Invoice {
  id: uuid @pk
  customer_id: uuid @fk(Customer.id)
  amount: decimal(10, 2)
  status: InvoiceStatus = .draft
  notes: string?
  created_at: timestamp @auto_now_add
}

action CreateCustomer(email: string, full_name: string) -> Customer {
  let c = create Customer {
    email: email,
    full_name: full_name
  }
  return c
}
`

	buildProfile := `
project:
  name: billing
  module: example.com/billing

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

	fdnPath := filepath.Join(dir, "billing.fdn")
	if err := os.WriteFile(fdnPath, []byte(blueprintV1), 0644); err != nil {
		t.Fatalf("write v1 blueprint: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "foundation.build.yaml"), []byte(buildProfile), 0644); err != nil {
		t.Fatalf("write build profile: %v", err)
	}

	// 1. V1 Build: emits schema.sql, 0001_initial.up.sql, 0001_initial.down.sql, and foundation.lock.json
	if _, diags := compiler.Build(dir, compiler.BuildOptions{}); len(diags) != 0 {
		t.Fatalf("v1 build failed: %+v", diags)
	}

	outDir := filepath.Join(dir, "dist", "backend")
	schemaBytes, err := os.ReadFile(filepath.Join(outDir, "schema.sql"))
	if err != nil {
		t.Fatalf("expected schema.sql to be emitted: %v", err)
	}
	schemaSQL := string(schemaBytes)

	expectedDDLFragments := []string{
		"CREATE TABLE customer (",
		"id UUID PRIMARY KEY",
		"email TEXT NOT NULL UNIQUE",
		"full_name TEXT NOT NULL",
		"CREATE INDEX idx_customer_full_name ON customer (full_name);",
		"CREATE TABLE invoice (",
		"customer_id UUID NOT NULL REFERENCES customer(id)",
		"amount NUMERIC(10, 2) NOT NULL",
		"status TEXT NOT NULL DEFAULT 'draft'",
		"notes TEXT",
		"created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()",
	}
	for _, frag := range expectedDDLFragments {
		if !strings.Contains(schemaSQL, frag) {
			t.Errorf("expected schema.sql to contain %q, got:\n%s", frag, schemaSQL)
		}
	}

	mig1UpBytes, err := os.ReadFile(filepath.Join(outDir, "migrations/0001_initial.up.sql"))
	if err != nil {
		t.Fatalf("expected migrations/0001_initial.up.sql: %v", err)
	}
	if !strings.Contains(string(mig1UpBytes), "CREATE TABLE customer") {
		t.Errorf("expected 0001_initial.up.sql to create tables, got:\n%s", string(mig1UpBytes))
	}

	if _, err := os.Stat(filepath.Join(dir, "foundation.lock.json")); err != nil {
		t.Fatalf("expected foundation.lock.json in workspace root: %v", err)
	}

	// 2. V2 Build: Evolve Customer (rename full_name -> display_name with @renamed_from("full_name")) and add phone_number
	blueprintV2 := `
enum InvoiceStatus {
  draft
  paid
}

entity Customer {
  id: uuid @pk
  email: string @unique
  display_name: string @renamed_from("full_name") @index
  phone_number: string?
}

entity Invoice {
  id: uuid @pk
  customer_id: uuid @fk(Customer.id)
  amount: decimal(10, 2)
  status: InvoiceStatus = .draft
  notes: string?
  created_at: timestamp @auto_now_add
}

action CreateCustomer(email: string, display_name: string) -> Customer {
  let c = create Customer {
    email: email,
    display_name: display_name
  }
  return c
}
`
	if err := os.WriteFile(fdnPath, []byte(blueprintV2), 0644); err != nil {
		t.Fatalf("write v2 blueprint: %v", err)
	}

	if _, diags := compiler.Build(dir, compiler.BuildOptions{}); len(diags) != 0 {
		t.Fatalf("v2 build failed: %+v", diags)
	}

	mig2UpBytes, err := os.ReadFile(filepath.Join(outDir, "migrations/0002_migration.up.sql"))
	if err != nil {
		t.Fatalf("expected migrations/0002_migration.up.sql: %v", err)
	}
	mig2Up := string(mig2UpBytes)
	if !strings.Contains(mig2Up, "ALTER TABLE customer RENAME COLUMN full_name TO display_name;") {
		t.Errorf("expected RENAME COLUMN in 0002_migration.up.sql, got:\n%s", mig2Up)
	}
	if !strings.Contains(mig2Up, "ALTER TABLE customer ADD COLUMN phone_number TEXT;") {
		t.Errorf("expected ADD COLUMN phone_number in 0002_migration.up.sql, got:\n%s", mig2Up)
	}

	mig2DownBytes, err := os.ReadFile(filepath.Join(outDir, "migrations/0002_migration.down.sql"))
	if err != nil {
		t.Fatalf("expected migrations/0002_migration.down.sql: %v", err)
	}
	mig2Down := string(mig2DownBytes)
	if !strings.Contains(mig2Down, "ALTER TABLE customer RENAME COLUMN display_name TO full_name;") {
		t.Errorf("expected reverse RENAME COLUMN in 0002_migration.down.sql, got:\n%s", mig2Down)
	}

	// 3. V3 Build: Unhinted destructive drop of `notes` column on Invoice -> must fail with FDN0410 unless Force is set
	blueprintV3Destructive := `
enum InvoiceStatus {
  draft
  paid
}

entity Customer {
  id: uuid @pk
  email: string @unique
  display_name: string @index
  phone_number: string?
}

entity Invoice {
  id: uuid @pk
  customer_id: uuid @fk(Customer.id)
  amount: decimal(10, 2)
  status: InvoiceStatus = .draft
  created_at: timestamp @auto_now_add
}

action CreateCustomer(email: string, display_name: string) -> Customer {
  let c = create Customer {
    email: email,
    display_name: display_name
  }
  return c
}
`
	if err := os.WriteFile(fdnPath, []byte(blueprintV3Destructive), 0644); err != nil {
		t.Fatalf("write v3 blueprint: %v", err)
	}

	_, dropDiags := compiler.Build(dir, compiler.BuildOptions{Force: false})
	if len(dropDiags) == 0 {
		t.Fatal("expected destructive schema change diagnostic when dropping column 'notes' without force, got none")
	}
	if dropDiags[0].Code != "FDN0410" {
		t.Fatalf("expected diagnostic code FDN0410 for unhinted column drop, got: %+v", dropDiags)
	}
}

func TestBuild_PostgresRepositoriesTacticalTraitsAndTransportConventions_CompilesAndPassesScenarios(t *testing.T) {
	dir := t.TempDir()

	blueprint := `
value ChargeInput {
  request_id: string
  account_id: uuid
  amount: int
}

entity Account {
  id: uuid @pk
  balance: int
}

event PaymentProcessed {
  account_id: uuid
  amount: int
}

error AccountNotFound {
  kind: not_found
  message: "Account not found"
}

error InsufficientFunds {
  kind: conflict
  message: "Account balance is insufficient"
}

error GatewayTimeout {
  kind: internal
  message: "Gateway timed out"
}

slot RiskScorer {
  CheckRisk(account_id: uuid, amount: int) -> int throws GatewayTimeout
}

action ChargeAccount(input: ChargeInput) -> Account
  @http(method: POST, path: "/v1/accounts/charge")
  @idempotent(key: input.request_id)
  @transactional
  @retry(attempts: 3, backoff: 10)
  @outbox
{
  let risk = slot RiskScorer.CheckRisk(input.account_id, input.amount)
  let acc = query one Account where id == input.account_id else AccountNotFound
  update Account where id == acc.id set {
    balance: acc.balance - input.amount
  }
  guard acc.balance >= input.amount else InsufficientFunds

  let updated = query one Account where id == acc.id else AccountNotFound
  emit PaymentProcessed {
    account_id: updated.id,
    amount: input.amount
  }
  return updated
}

action GetAccountBalance(id: uuid) -> Account
  @cache(ttl: 60)
{
  let acc = query one Account where id == id else AccountNotFound
  return acc
}

action InternalAudit(id: uuid) -> Account
  @internal
{
  let acc = query one Account where id == id else AccountNotFound
  return acc
}

action OnPaymentProcessed(ev: PaymentProcessed) -> bool
  @event_handler(PaymentProcessed)
{
  return true
}

action ReconcileNightly() -> bool
  @cron("0 0 * * *")
{
  return true
}

scenario "ChargeAccount succeeds with transactional outbox and idempotency" {
  given Account {
    id: "00000000-0000-0000-0000-000000000001",
    balance: 500
  }
  given slot RiskScorer.CheckRisk returns 10
  when ChargeAccount(ChargeInput {
    request_id: "req-100",
    account_id: "00000000-0000-0000-0000-000000000001",
    amount: 200
  })
  expect result.balance == 300
  expect stored Account where id == "00000000-0000-0000-0000-000000000001" {
    balance: 300
  }
  expect emitted PaymentProcessed {
    account_id: "00000000-0000-0000-0000-000000000001",
    amount: 200
  }
}

scenario "ChargeAccount rolls back Account mutation when guard fails inside @transactional" {
  given Account {
    id: "00000000-0000-0000-0000-000000000002",
    balance: 50
  }
  given slot RiskScorer.CheckRisk returns 10
  when ChargeAccount(ChargeInput {
    request_id: "req-200",
    account_id: "00000000-0000-0000-0000-000000000002",
    amount: 200
  })
  expect error InsufficientFunds
  expect stored Account where id == "00000000-0000-0000-0000-000000000002" {
    balance: 50
  }
}
`

	buildProfile := `
project:
  name: payments
  module: example.com/payments

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

	if err := os.WriteFile(filepath.Join(dir, "payments.fdn"), []byte(blueprint), 0644); err != nil {
		t.Fatalf("write payments.fdn: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "foundation.build.yaml"), []byte(buildProfile), 0644); err != nil {
		t.Fatalf("write foundation.build.yaml: %v", err)
	}

	res, diags := compiler.Build(dir, compiler.BuildOptions{})
	if len(diags) != 0 {
		t.Fatalf("expected build to succeed, got: %+v", diags)
	}

	outDir := filepath.Join(dir, "dist", "backend")

	// 1. Verify PostgreSQL repository adapter and transaction manager files are emitted
	pgRepoFile := filepath.Join(outDir, "internal/adapters/postgres/account_repository.go")
	pgTxFile := filepath.Join(outDir, "internal/adapters/postgres/transaction_manager.go")
	for _, f := range []string{pgRepoFile, pgTxFile} {
		if _, err := os.Stat(f); err != nil {
			t.Fatalf("expected PostgreSQL adapter file %s to be emitted: %v", f, err)
		}
	}

	// 2. Verify Transport exposure conventions in lowered ArchitectureIR and router.go
	archIR := res.Architectures[0]
	for _, h := range archIR.HTTPHandlers {
		if h.ActionName == "InternalAudit" || h.ActionName == "OnPaymentProcessed" || h.ActionName == "ReconcileNightly" {
			t.Fatalf("expected @internal, @event_handler, and @cron actions to be excluded from HTTPHandlers, but found %s", h.ActionName)
		}
	}
	if len(archIR.EventHandlers) != 1 || archIR.EventHandlers[0].ActionName != "OnPaymentProcessed" || archIR.EventHandlers[0].EventName != "PaymentProcessed" {
		t.Fatalf("expected 1 EventHandlerIR for OnPaymentProcessed, got %+v", archIR.EventHandlers)
	}
	if len(archIR.CronHandlers) != 1 || archIR.CronHandlers[0].ActionName != "ReconcileNightly" || archIR.CronHandlers[0].Schedule != "0 0 * * *" {
		t.Fatalf("expected 1 CronHandlerIR for ReconcileNightly, got %+v", archIR.CronHandlers)
	}

	// 3. Run generated test suite (verifies Scenarios including @transactional rollback, @idempotent, @cache, @retry, @outbox, and Contract Tests)
	if testDiags := compiler.Test(dir); len(testDiags) != 0 {
		t.Fatalf("expected generated scenarios and tactical trait tests to pass, got: %+v", testDiags)
	}
}
