package compiler_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/galihanggara68/foundation/pkg/compiler"
)

func TestCompiler_SDKTargetsBuild(t *testing.T) {
	dir := t.TempDir()

	blueprint := `
enum OrderStatus {
  pending
  confirmed
  cancelled
}

value Money {
  amount: decimal
  currency: string
}

entity Order {
  id: uuid @pk
  customer_id: uuid
  status: OrderStatus
  total: Money
  created_at: timestamp
}

error InsufficientStock {
  kind: conflict
  message: "insufficient stock for requested item"
}

error OrderNotFound {
  kind: not_found
  message: "order not found"
}

action PlaceOrder(item_id: uuid, quantity: int) -> Order
  @http(method: "POST", path: "/v1/orders")
{
  let order = query one Order where id == item_id else OrderNotFound
  return order
}

action ListOrders(customer_id: uuid, limit: int, cursor: string) -> []Order
  @http(method: "GET", path: "/v1/orders")
{
  let orders = query many Order
    where customer_id == customer_id
    order_by id asc
    limit 20
    cursor cursor
  return orders
}

action InternalJob(id: uuid)
  @internal
{
  let order = query one Order where id == id else OrderNotFound
}
`
	if err := os.WriteFile(filepath.Join(dir, "shop.fdn"), []byte(blueprint), 0644); err != nil {
		t.Fatalf("failed to write blueprint: %v", err)
	}

	buildYAML := `
project:
  name: shop
  module: example.com/shop

targets:
  - name: go-client
    kind: sdk
    language: go
    out_dir: clients/go
  - name: ts-client
    kind: sdk
    language: typescript
    out_dir: clients/ts
`
	if err := os.WriteFile(filepath.Join(dir, "foundation.build.yaml"), []byte(buildYAML), 0644); err != nil {
		t.Fatalf("failed to write build yaml: %v", err)
	}

	res, diags := compiler.Build(dir, compiler.BuildOptions{})
	if len(diags) > 0 {
		t.Fatalf("unexpected build diagnostics: %v", diags)
	}
	if len(res.Architectures) != 2 {
		t.Fatalf("expected 2 architectures, got %d", len(res.Architectures))
	}

	// 1. Verify Go SDK emitted files
	goSDKDir := filepath.Join(dir, "clients", "go")
	expectedGoFiles := []string{
		"go.mod",
		"client.go",
		"types.go",
		"actions.go",
		"errors.go",
		".foundation-manifest.json",
	}
	for _, ef := range expectedGoFiles {
		p := filepath.Join(goSDKDir, ef)
		if _, err := os.Stat(p); os.IsNotExist(err) {
			t.Errorf("expected Go SDK file %q to exist in %s", ef, goSDKDir)
		}
	}

	// Verify Go SDK content
	clientGoBytes, err := os.ReadFile(filepath.Join(goSDKDir, "client.go"))
	if err != nil {
		t.Fatalf("failed to read client.go: %v", err)
	}
	clientGo := string(clientGoBytes)
	if !strings.Contains(clientGo, "type Client struct") || !strings.Contains(clientGo, "func NewClient") {
		t.Errorf("client.go missing Client struct or NewClient: %s", clientGo)
	}
	if !strings.Contains(clientGo, "WithBearerToken") || !strings.Contains(clientGo, "WithHTTPClient") {
		t.Errorf("client.go missing functional options")
	}

	actionsGoBytes, err := os.ReadFile(filepath.Join(goSDKDir, "actions.go"))
	if err != nil {
		t.Fatalf("failed to read actions.go: %v", err)
	}
	actionsGo := string(actionsGoBytes)
	if !strings.Contains(actionsGo, "func (c *Client) PlaceOrder") {
		t.Errorf("actions.go missing PlaceOrder method")
	}
	if !strings.Contains(actionsGo, "func (c *Client) ListOrders") {
		t.Errorf("actions.go missing ListOrders method")
	}
	if strings.Contains(actionsGo, "InternalJob") {
		t.Errorf("actions.go should not expose @internal action InternalJob")
	}

	errorsGoBytes, err := os.ReadFile(filepath.Join(goSDKDir, "errors.go"))
	if err != nil {
		t.Fatalf("failed to read errors.go: %v", err)
	}
	errorsGo := string(errorsGoBytes)
	if !strings.Contains(errorsGo, "type InsufficientStockError struct") {
		t.Errorf("errors.go missing InsufficientStockError")
	}
	if !strings.Contains(errorsGo, "type NotFoundError struct") {
		t.Errorf("errors.go missing NotFoundError")
	}

	// 2. Verify TypeScript SDK emitted files
	tsSDKDir := filepath.Join(dir, "clients", "ts")
	expectedTSFiles := []string{
		"package.json",
		"index.ts",
		"client.ts",
		"types.ts",
		"errors.ts",
		".foundation-manifest.json",
	}
	for _, ef := range expectedTSFiles {
		p := filepath.Join(tsSDKDir, ef)
		if _, err := os.Stat(p); os.IsNotExist(err) {
			t.Errorf("expected TypeScript SDK file %q to exist in %s", ef, tsSDKDir)
		}
	}

	clientTSBytes, err := os.ReadFile(filepath.Join(tsSDKDir, "client.ts"))
	if err != nil {
		t.Fatalf("failed to read client.ts: %v", err)
	}
	clientTS := string(clientTSBytes)
	if !strings.Contains(clientTS, "export class Client") {
		t.Errorf("client.ts missing export class Client")
	}
	if !strings.Contains(clientTS, "async placeOrder") || !strings.Contains(clientTS, "async listOrders") {
		t.Errorf("client.ts missing typed action methods")
	}
	if strings.Contains(clientTS, "internalJob") {
		t.Errorf("client.ts should not expose @internal action")
	}

	errorsTSBytes, err := os.ReadFile(filepath.Join(tsSDKDir, "errors.ts"))
	if err != nil {
		t.Fatalf("failed to read errors.ts: %v", err)
	}
	errorsTS := string(errorsTSBytes)
	if !strings.Contains(errorsTS, "export class InsufficientStockError extends") {
		t.Errorf("errors.ts missing InsufficientStockError")
	}
	if !strings.Contains(errorsTS, "export class UnauthenticatedError extends") {
		t.Errorf("errors.ts missing UnauthenticatedError")
	}
}
