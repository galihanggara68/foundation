package compiler_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/galihanggara68/foundation/pkg/compiler"
)

func TestCompiler_SDKRuntimeExecution(t *testing.T) {
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

	_, diags := compiler.Build(dir, compiler.BuildOptions{})
	if len(diags) > 0 {
		t.Fatalf("unexpected build diagnostics: %v", diags)
	}

	// Spin up mock HTTP backend
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-jwt-token" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"code":    "Unauthorized",
				"kind":    "unauthenticated",
				"message": "missing or invalid token",
			})
			return
		}

		if r.Method == "POST" && r.URL.Path == "/v1/orders" {
			var body map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["item_id"] == "conflict-item" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusConflict)
				_ = json.NewEncoder(w).Encode(map[string]string{
					"code":    "InsufficientStock",
					"kind":    "conflict",
					"message": "insufficient stock for requested item",
				})
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"id":          "018f3a21-72a3-7b2c-9a4f-5231a498b8c1",
				"customer_id": "018f3a21-72a3-7b2c-9a4f-5231a498b8c2",
				"status":      "pending",
				"total": map[string]string{
					"amount":   "99.50",
					"currency": "USD",
				},
				"created_at": "2026-10-26T08:00:00Z",
			})
			return
		}

		if r.Method == "GET" && r.URL.Path == "/v1/orders" {
			custID := r.URL.Query().Get("customer_id")
			if custID == "not-found" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusNotFound)
				_ = json.NewEncoder(w).Encode(map[string]string{
					"code":    "OrderNotFound",
					"kind":    "not_found",
					"message": "order not found",
				})
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"items": []map[string]interface{}{
					{
						"id":          "018f3a21-72a3-7b2c-9a4f-5231a498b8c1",
						"customer_id": custID,
						"status":      "confirmed",
						"total": map[string]string{
							"amount":   "50.00",
							"currency": "USD",
						},
						"created_at": "2026-10-26T08:00:00Z",
					},
				},
				"page_info": map[string]interface{}{
					"next_cursor": "eyJpZCI6IjAxOGYzYTIxIn0=",
					"has_next":    true,
					"has_prev":    false,
				},
			})
			return
		}

		http.NotFound(w, r)
	}))
	defer srv.Close()

	// 1. Run Go SDK Client Test
	goSDKDir := filepath.Join(dir, "clients", "go")
	goTestContent := fmt.Sprintf(`package client

import (
	"context"
	"errors"
	"testing"
)

func TestGoSDKRuntime(t *testing.T) {
	c := NewClient(%q, WithBearerToken("test-jwt-token"))

	ctx := context.Background()

	// 1. Test Success Mutation (PlaceOrder)
	ord, err := c.PlaceOrder(ctx, PlaceOrderInput{
		ItemID:   "item-123",
		Quantity: 2,
	})
	if err != nil {
		t.Fatalf("unexpected error calling PlaceOrder: %%v", err)
	}
	if ord.ID != "018f3a21-72a3-7b2c-9a4f-5231a498b8c1" || ord.Status != OrderStatusPending {
		t.Errorf("unexpected order response: %%+v", ord)
	}

	// 2. Test Success List Query with Keyset Pagination
	listRes, err := c.ListOrders(ctx, ListOrdersInput{
		CustomerID: "cust-999",
		Limit:      20,
		Cursor:     "cur-start",
	})
	if err != nil {
		t.Fatalf("unexpected error calling ListOrders: %%v", err)
	}
	if len(listRes.Items) != 1 || !listRes.PageInfo.HasNext {
		t.Errorf("unexpected list response: %%+v", listRes)
	}

	// 3. Test Domain Error Matching with errors.As
	_, err = c.PlaceOrder(ctx, PlaceOrderInput{
		ItemID: "conflict-item",
	})
	if err == nil {
		t.Fatalf("expected InsufficientStock error, got nil")
	}
	var stockErr *InsufficientStockError
	if !errors.As(err, &stockErr) {
		t.Errorf("expected error to match *InsufficientStockError, got: %%T %%v", err, err)
	}
	if stockErr.StatusCode != 409 {
		t.Errorf("expected status code 409, got %%d", stockErr.StatusCode)
	}

	// 4. Test Built-in Error Matching with errors.As
	_, err = c.ListOrders(ctx, ListOrdersInput{
		CustomerID: "not-found",
	})
	if err == nil {
		t.Fatalf("expected NotFoundError, got nil")
	}
	var notFoundErr *NotFoundError
	if !errors.As(err, &notFoundErr) {
		t.Errorf("expected error to match *NotFoundError, got: %%T %%v", err, err)
	}

	// 5. Test Unauthorized Error
	unauthClient := NewClient(%q)
	_, err = unauthClient.PlaceOrder(ctx, PlaceOrderInput{ItemID: "item-123"})
	if err == nil {
		t.Fatalf("expected UnauthenticatedError, got nil")
	}
	var unauthErr *UnauthenticatedError
	if !errors.As(err, &unauthErr) {
		t.Errorf("expected error to match *UnauthenticatedError, got: %%T %%v", err, err)
	}
}
`, srv.URL, srv.URL)

	if err := os.WriteFile(filepath.Join(goSDKDir, "client_runtime_test.go"), []byte(goTestContent), 0644); err != nil {
		t.Fatalf("failed to write go test file: %v", err)
	}

	cmdGo := exec.Command("go", "test", "-v", "./...")
	cmdGo.Dir = goSDKDir
	outGo, errGo := cmdGo.CombinedOutput()
	if errGo != nil {
		t.Fatalf("go test failed in Go SDK package: %v\n%s", errGo, string(outGo))
	}

	// 2. Run TypeScript SDK Client Test with Node 24 native type stripping
	tsSDKDir := filepath.Join(dir, "clients", "ts")
	tsTestContent := fmt.Sprintf(`import { test } from "node:test";
import assert from "node:assert";
import { Client } from "./client.ts";
import { InsufficientStockError, NotFoundError, UnauthenticatedError } from "./errors.ts";

test("TypeScript SDK Client Runtime Execution", async () => {
  const client = new Client({
    baseURL: %q,
    authToken: "test-jwt-token",
  });

  // 1. Mutation
  const order = await client.placeOrder({
    item_id: "item-123",
    quantity: 2,
  });
  assert.strictEqual(order.id, "018f3a21-72a3-7b2c-9a4f-5231a498b8c1");
  assert.strictEqual(order.status, "pending");

  // 2. List Query Many
  const listRes = await client.listOrders({
    customer_id: "cust-999",
    limit: 20,
    cursor: "cur-start",
  });
  assert.strictEqual(listRes.items.length, 1);
  assert.strictEqual(listRes.page_info.has_next, true);

  // 3. Domain Error Matching
  await assert.rejects(
    async () => {
      await client.placeOrder({ item_id: "conflict-item", quantity: 1 });
    },
    (err: any) => {
      assert(err instanceof InsufficientStockError, "expected InsufficientStockError instance");
      assert.strictEqual(err.statusCode, 409);
      return true;
    }
  );

  // 4. Built-in Error Matching
  await assert.rejects(
    async () => {
      await client.listOrders({ customer_id: "not-found", limit: 20, cursor: "" });
    },
    (err: any) => {
      assert(err instanceof NotFoundError, "expected NotFoundError instance");
      assert.strictEqual(err.statusCode, 404);
      return true;
    }
  );

  // 5. Unauthenticated Error
  const unauthClient = new Client({ baseURL: %q });
  await assert.rejects(
    async () => {
      await unauthClient.placeOrder({ item_id: "item-123", quantity: 1 });
    },
    (err: any) => {
      assert(err instanceof UnauthenticatedError, "expected UnauthenticatedError instance");
      assert.strictEqual(err.statusCode, 401);
      return true;
    }
  );
});
`, srv.URL, srv.URL)

	if err := os.WriteFile(filepath.Join(tsSDKDir, "client_runtime.test.ts"), []byte(tsTestContent), 0644); err != nil {
		t.Fatalf("failed to write ts test file: %v", err)
	}

	cmdTS := exec.Command("node", "--experimental-strip-types", "--test", "client_runtime.test.ts")
	cmdTS.Dir = tsSDKDir
	outTS, errTS := cmdTS.CombinedOutput()
	if errTS != nil {
		t.Fatalf("node --test failed in TS SDK package: %v\n%s", errTS, string(outTS))
	}
}
