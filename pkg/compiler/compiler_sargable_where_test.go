package compiler_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/galihanggara68/foundation/pkg/compiler"
)

func TestCheck_DisjunctionInWhere_ReturnsFDN0217(t *testing.T) {
	dir := t.TempDir()

	bp := `entity Order {
  id: uuid @pk
  status: string
  total: decimal
}

action CancelOrders(status: string, min_total: decimal) -> void {
  delete Order where status == status or total > min_total
}
`
	if err := os.WriteFile(filepath.Join(dir, "disjunction.fdn"), []byte(bp), 0644); err != nil {
		t.Fatalf("write blueprint: %v", err)
	}

	semIR, diags := compiler.Check(dir)
	if semIR != nil {
		t.Fatalf("expected nil SemanticIR on disjunction validation error, got %+v", semIR)
	}

	foundFDN0217 := false
	for _, d := range diags {
		if d.Code == "FDN0217" {
			foundFDN0217 = true
			if !strings.Contains(d.Message, "disjunction") && !strings.Contains(d.Message, "sargable") {
				t.Errorf("expected message to mention disjunction or sargable, got: %q", d.Message)
			}
		}
	}
	if !foundFDN0217 {
		t.Fatalf("expected diagnostic FDN0217 for 'or' in where clause, got diagnostics: %+v", diags)
	}
}

func TestCheck_CompoundAndWhere_PassesValidation(t *testing.T) {
	dir := t.TempDir()

	bp := `error NotFound {
  kind: not_found
}

entity Order {
  id: uuid @pk
  customer_id: uuid
  status: string
  created_at: timestamp
  total: decimal
}

action FindRecentOrders(customer_id: uuid, status: string, since: timestamp) -> Order {
  let order = query one Order where customer_id == customer_id and status == status and created_at >= since else NotFound
  return order
}
`
	if err := os.WriteFile(filepath.Join(dir, "compound.fdn"), []byte(bp), 0644); err != nil {
		t.Fatalf("write blueprint: %v", err)
	}

	semIR, diags := compiler.Check(dir)
	if len(diags) > 0 {
		t.Fatalf("expected zero diagnostics for valid compound and query, got: %+v", diags)
	}
	if semIR == nil {
		t.Fatal("expected non-nil SemanticIR")
	}
}

func TestCheck_IncompatibleComparisonTypes_ReturnsFDN0205(t *testing.T) {
	dir := t.TempDir()

	bp := `entity Order {
  id: uuid @pk
  created_at: timestamp
  status: string
}

action BadCompare(status: string) -> bool {
  delete Order where created_at > 123
  return true
}
`
	if err := os.WriteFile(filepath.Join(dir, "bad_compare.fdn"), []byte(bp), 0644); err != nil {
		t.Fatalf("write blueprint: %v", err)
	}

	semIR, diags := compiler.Check(dir)
	if semIR != nil {
		t.Fatalf("expected nil SemanticIR on type error, got %+v", semIR)
	}

	foundFDN0205 := false
	for _, d := range diags {
		if d.Code == "FDN0205" {
			foundFDN0205 = true
		}
	}
	if !foundFDN0205 {
		t.Fatalf("expected diagnostic FDN0205 for timestamp > int, got: %+v", diags)
	}
}

func TestBuild_CompoundWhere_EmitsPostgresAndInMemoryRepos(t *testing.T) {
	dir := t.TempDir()

	bp := `error NotFound {
  kind: not_found
}

entity Order {
  id: uuid @pk
  customer_id: uuid
  status: string
  created_at: timestamp
  total: decimal
}

action FindRecentOrder(customer_id: uuid, status: string, since: timestamp) -> Order {
  let order = query one Order where customer_id == customer_id and status == status and created_at >= since else NotFound
  return order
}

action DeleteOldOrders(customer_id: uuid, status: string) -> bool {
  delete Order where customer_id == customer_id and status == status
  return true
}
`
	if err := os.WriteFile(filepath.Join(dir, "orders.fdn"), []byte(bp), 0644); err != nil {
		t.Fatalf("write blueprint: %v", err)
	}

	buildProfile := `project:
  name: test-orders
  module: example.com/orders

storage:
  dialect: postgres
  mode: stateful

targets:
  - name: backend
    kind: backend
    language: go
    architecture: hexagonal
    out_dir: gen
`
	if err := os.WriteFile(filepath.Join(dir, "foundation.build.yaml"), []byte(buildProfile), 0644); err != nil {
		t.Fatalf("write foundation.build.yaml: %v", err)
	}

	_, buildDiags := compiler.Build(dir, compiler.BuildOptions{})
	if len(buildDiags) > 0 {
		t.Fatalf("compiler.Build failed: %+v", buildDiags)
	}

	// 1. Verify PostgreSQL repository emits parameterized compound SQL
	pgPath := filepath.Join(dir, "gen", "internal", "adapters", "postgres", "order_repository.go")
	pgBytes, err := os.ReadFile(pgPath)
	if err != nil {
		t.Fatalf("read postgres/order_repository.go: %v", err)
	}
	pgSrc := string(pgBytes)
	expectedSelect := `WHERE customer_id = $1 AND status = $2 AND created_at >= $3 LIMIT 1`
	if !strings.Contains(pgSrc, expectedSelect) {
		t.Errorf("postgres order_repository.go missing expected compound WHERE clause: %q\ngot:\n%s", expectedSelect, pgSrc)
	}
	expectedDelete := `DELETE FROM orders WHERE customer_id = $1 AND status = $2`
	if !strings.Contains(pgSrc, expectedDelete) {
		t.Errorf("postgres order_repository.go missing expected delete SQL: %q\ngot:\n%s", expectedDelete, pgSrc)
	}

	// 2. Verify in-memory repository emits compound matching
	memPath := filepath.Join(dir, "gen", "internal", "adapters", "memory", "order_repository.go")
	memBytes, err := os.ReadFile(memPath)
	if err != nil {
		t.Fatalf("read memory/order_repository.go: %v", err)
	}
	memSrc := string(memBytes)
	if !strings.Contains(memSrc, "item.CustomerID == customer_id") || !strings.Contains(memSrc, "item.Status == status") {
		t.Errorf("in_memory_repository.go missing expected checks, got:\n%s", memSrc)
	}
}

func TestTest_CompoundWhere_HermeticScenarios(t *testing.T) {
	dir := t.TempDir()

	bp := `error NotFound {
  kind: not_found
}

entity Order {
  id: uuid @pk
  customer_id: uuid
  status: string
  created_at: timestamp
  total: decimal
}

action CreateOrder(customer_id: uuid, status: string, created_at: timestamp, total: decimal) -> Order {
  let ord = create Order {
    customer_id: customer_id,
    status: status,
    created_at: created_at,
    total: total
  }
  return ord
}

action FindRecentOrder(customer_id: uuid, status: string, since: timestamp) -> Order {
  let order = query one Order where customer_id == customer_id and status == status and created_at >= since else NotFound
  return order
}

scenario "FindRecentOrder retrieves order matching compound and criteria" {
  given Order with {
    id: "00000000-0000-0000-0000-000000000001",
    customer_id: "00000000-0000-0000-0000-000000000002",
    status: "active",
    created_at: "2026-10-25T12:00:00Z",
    total: 99.50
  }
  when FindRecentOrder("00000000-0000-0000-0000-000000000002", "active", "2026-10-25T10:00:00Z")
  expect stored Order where customer_id == "00000000-0000-0000-0000-000000000002" and status == "active" {
    total: 99.50
  }
}
`
	if err := os.WriteFile(filepath.Join(dir, "scenario.fdn"), []byte(bp), 0644); err != nil {
		t.Fatalf("write blueprint: %v", err)
	}

	buildProfile := `target:
  name: test-scenario
  kind: backend
  language: go
  architecture: hexagonal
`
	if err := os.WriteFile(filepath.Join(dir, "foundation.build.yaml"), []byte(buildProfile), 0644); err != nil {
		t.Fatalf("write foundation.build.yaml: %v", err)
	}

	_, buildDiags := compiler.Build(dir, compiler.BuildOptions{})
	if len(buildDiags) > 0 {
		t.Fatalf("compiler.Build failed: %+v", buildDiags)
	}

	testDiags := compiler.Test(dir)
	if len(testDiags) > 0 {
		t.Fatalf("compiler.Test failed on compound where scenario: %+v", testDiags)
	}
}

func TestBuild_CompoundWhere_TypeScriptClean(t *testing.T) {
	dir := t.TempDir()

	bp := `error NotFound {
  kind: not_found
}

entity Order {
  id: uuid @pk
  customer_id: uuid
  status: string
  created_at: timestamp
  total: decimal
}

action FindRecentOrder(customer_id: uuid, status: string, since: timestamp) -> Order {
  let order = query one Order where customer_id == customer_id and status == status and created_at >= since else NotFound
  return order
}
`
	if err := os.WriteFile(filepath.Join(dir, "orders.fdn"), []byte(bp), 0644); err != nil {
		t.Fatalf("write blueprint: %v", err)
	}

	buildProfile := `project:
  name: test-orders-ts
  module: "@orders/backend"

storage:
  dialect: postgres
  mode: stateful

targets:
  - name: backend
    kind: backend
    language: typescript
    architecture: clean
    out_dir: dist
`
	if err := os.WriteFile(filepath.Join(dir, "foundation.build.yaml"), []byte(buildProfile), 0644); err != nil {
		t.Fatalf("write foundation.build.yaml: %v", err)
	}

	_, buildDiags := compiler.Build(dir, compiler.BuildOptions{})
	if len(buildDiags) > 0 {
		t.Fatalf("compiler.Build failed: %+v", buildDiags)
	}

	// 1. Verify TypeScript ports interface
	portsPath := filepath.Join(dir, "dist", "src", "usecases", "ports.ts")
	portsBytes, err := os.ReadFile(portsPath)
	if err != nil {
		t.Fatalf("read ports.ts: %v", err)
	}
	portsSrc := string(portsBytes)
	expectedPortMethod := "findByCustomerIDAndStatusAndCreatedAt(customer_id: string, status: string, created_at: Date): Promise<domain.Order | null>;"
	if !strings.Contains(portsSrc, expectedPortMethod) {
		t.Errorf("ports.ts missing expected method: %q\ngot:\n%s", expectedPortMethod, portsSrc)
	}

	// 2. Verify TypeScript postgres repository
	pgPath := filepath.Join(dir, "dist", "src", "gateways", "postgres_repositories.ts")
	pgBytes, err := os.ReadFile(pgPath)
	if err != nil {
		t.Fatalf("read postgres_repositories.ts: %v", err)
	}
	pgSrc := string(pgBytes)
	expectedSelect := `WHERE customer_id = $1 AND status = $2 AND created_at >= $3 LIMIT 1`
	if !strings.Contains(pgSrc, expectedSelect) {
		t.Errorf("postgres_repositories.ts missing expected compound SQL: %q\ngot:\n%s", expectedSelect, pgSrc)
	}

	// 3. Verify TypeScript in-memory repository
	memPath := filepath.Join(dir, "dist", "src", "gateways", "memory_repositories.ts")
	memBytes, err := os.ReadFile(memPath)
	if err != nil {
		t.Fatalf("read memory_repositories.ts: %v", err)
	}
	memSrc := string(memBytes)
	if !strings.Contains(memSrc, "e.customer_id === customer_id") || !strings.Contains(memSrc, "e.status === status") {
		t.Errorf("memory_repositories.ts missing expected checks, got:\n%s", memSrc)
	}
}




