package compiler_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/galihanggara68/foundation/pkg/compiler"
)

func TestBuild_CronTrait_FailsWithFDN0115(t *testing.T) {
	dir := t.TempDir()

	blueprint := `
action RunPeriodicCleanup() -> int
  @cron("0 0 * * *")
{
  return 0
}
`
	if err := os.WriteFile(filepath.Join(dir, "app.fdn"), []byte(blueprint), 0644); err != nil {
		t.Fatalf("failed to write app.fdn: %v", err)
	}

	buildProfile := `
project:
  name: cron-test
  module: example.com/crontest

storage:
  dialect: memory
  mode: in_memory

targets:
  - name: server
    kind: backend
    language: go
    architecture: hexagonal
`
	if err := os.WriteFile(filepath.Join(dir, "foundation.build.yaml"), []byte(buildProfile), 0644); err != nil {
		t.Fatalf("failed to write build profile: %v", err)
	}

	_, diags := compiler.Build(dir, compiler.BuildOptions{})
	if len(diags) == 0 {
		t.Fatalf("expected build failure for @cron, got 0 diagnostics")
	}

	found := false
	for _, d := range diags {
		if d.Code == "FDN0115" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected diagnostic FDN0115, got: %+v", diags)
	}
}

func TestBuild_EventHandler_EndToEndSynchronousDispatch(t *testing.T) {
	dir := t.TempDir()

	blueprint := `
entity Order {
  id: uuid @pk
  item: string
  total: int
}

entity AuditLog {
  id: uuid @pk
  action_name: string
  detail: string
}

event OrderPlaced {
  order_id: uuid
  item: string
}

action PlaceOrder(item: string, total: int) -> Order
  @transactional
{
  let o = create Order {
    item: item,
    total: total
  }
  emit OrderPlaced {
    order_id: o.id,
    item: o.item
  }
  return o
}

action RecordOrderAudit(ev: OrderPlaced) -> AuditLog
  @event_handler(OrderPlaced)
{
  let log = create AuditLog {
    action_name: "order_placed",
    detail: ev.item
  }
  return log
}

scenario "PlaceOrder triggers RecordOrderAudit event handler post-commit" {
  when PlaceOrder("Mechanical Keyboard", 150)
  expect result.item == "Mechanical Keyboard"
  expect stored AuditLog where action_name == "order_placed"
}
`
	if err := os.WriteFile(filepath.Join(dir, "app.fdn"), []byte(blueprint), 0644); err != nil {
		t.Fatalf("failed to write app.fdn: %v", err)
	}

	buildProfile := `
project:
  name: order-service
  module: example.com/orderservice

storage:
  dialect: memory
  mode: in_memory

targets:
  - name: server
    kind: backend
    language: go
    architecture: hexagonal
`
	if err := os.WriteFile(filepath.Join(dir, "foundation.build.yaml"), []byte(buildProfile), 0644); err != nil {
		t.Fatalf("failed to write build profile: %v", err)
	}

	res, diags := compiler.Build(dir, compiler.BuildOptions{})
	if len(diags) > 0 {
		t.Fatalf("unexpected build diagnostics: %+v", diags)
	}
	if res == nil {
		t.Fatal("expected non-nil build result")
	}

	testDiags := compiler.Test(dir)
	if len(testDiags) > 0 {
		t.Fatalf("expected Scenario test to pass via event dispatch, got: %+v", testDiags)
	}
}

func TestBuild_EventHandler_EndToEndTypeScriptClean(t *testing.T) {
	dir := t.TempDir()

	blueprint := `
entity Order {
  id: uuid @pk
  item: string
  total: int
}

entity AuditLog {
  id: uuid @pk
  action_name: string
  detail: string
}

event OrderPlaced {
  order_id: uuid
  item: string
}

action PlaceOrder(item: string, total: int) -> Order
  @transactional
  @idempotent(key: item)
{
  let o = create Order {
    item: item,
    total: total
  }
  emit OrderPlaced {
    order_id: o.id,
    item: o.item
  }
  return o
}

action RecordOrderAudit(ev: OrderPlaced) -> AuditLog
  @event_handler(OrderPlaced)
{
  let log = create AuditLog {
    action_name: "order_placed",
    detail: ev.item
  }
  return log
}

scenario "PlaceOrder triggers RecordOrderAudit event handler post-commit in TypeScript" {
  when PlaceOrder("Ergonomic Mouse", 85)
  expect result.item == "Ergonomic Mouse"
  expect stored AuditLog where action_name == "order_placed"
}

scenario "PlaceOrder with same item returns cached result via contrib runtime" {
  when PlaceOrder("Ergonomic Mouse", 85)
  expect result.item == "Ergonomic Mouse"
}
`
	if err := os.WriteFile(filepath.Join(dir, "app.fdn"), []byte(blueprint), 0644); err != nil {
		t.Fatalf("failed to write app.fdn: %v", err)
	}

	buildProfile := `
project:
  name: order-service-ts
  module: example.com/orderservicets

storage:
  dialect: memory
  mode: in_memory

targets:
  - name: backend-ts
    kind: backend
    language: typescript
    architecture: clean
`
	if err := os.WriteFile(filepath.Join(dir, "foundation.build.yaml"), []byte(buildProfile), 0644); err != nil {
		t.Fatalf("failed to write build profile: %v", err)
	}

	res, diags := compiler.Build(dir, compiler.BuildOptions{})
	if len(diags) > 0 {
		t.Fatalf("unexpected build diagnostics: %+v", diags)
	}
	if res == nil {
		t.Fatal("expected non-nil build result")
	}

	testDiags := compiler.Test(dir)
	if len(testDiags) > 0 {
		t.Fatalf("expected Scenario test to pass via event dispatch in TS, got: %+v", testDiags)
	}
}
