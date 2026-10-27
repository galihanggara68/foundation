package e2e_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/galihanggara68/foundation/pkg/compiler"
)

const goSDKClientTestScript = `package client

import (
	"context"
	"errors"
	"os"
	"testing"
)

func TestGoSDKClientE2E(t *testing.T) {
	serverURL := os.Getenv("SERVER_URL")
	tokenValid := os.Getenv("TOKEN_VALID")
	tokenUnverified := os.Getenv("TOKEN_UNVERIFIED")

	ctx := context.Background()
	c := NewClient(serverURL, WithBearerToken(tokenValid))

	// 1. Create product (mutation)
	prod, err := c.CreateProduct(ctx, CreateProductInput{
		Name:  "Test Widget",
		SKU:   "WIDGET-" + os.Getenv("RUN_ID"),
		Price: 50,
		Stock: 5,
	})
	if err != nil {
		t.Fatalf("CreateProduct failed: %v", err)
	}
	if prod.ID == "" || prod.Name != "Test Widget" {
		t.Fatalf("unexpected product: %+v", prod)
	}

	// 2. Place Order 1 (success)
	ord1, err := c.PlaceOrder(ctx, PlaceOrderInput{
		ProductID: prod.ID,
		Quantity:  1,
	})
	if err != nil {
		t.Fatalf("PlaceOrder 1 failed: %v", err)
	}
	if ord1.ID == "" || ord1.Total != 50 || ord1.Status != OrderStatusConfirmed {
		t.Fatalf("unexpected order 1: %+v", ord1)
	}

	// Place Order 2 and Order 3 to test keyset pagination limit 2
	ord2, err := c.PlaceOrder(ctx, PlaceOrderInput{
		ProductID: prod.ID,
		Quantity:  1,
	})
	if err != nil {
		t.Fatalf("PlaceOrder 2 failed: %v", err)
	}
	ord3, err := c.PlaceOrder(ctx, PlaceOrderInput{
		ProductID: prod.ID,
		Quantity:  1,
	})
	if err != nil {
		t.Fatalf("PlaceOrder 3 failed: %v", err)
	}
	_ = ord2
	_ = ord3

	// 3. Keyset Pagination (ListOrders)
	custID := ord1.CustomerID
	page1, err := c.ListOrders(ctx, ListOrdersInput{
		CustID: custID,
		Cursor: "",
	})
	if err != nil {
		t.Fatalf("ListOrders page 1 failed: %v", err)
	}
	if len(page1.Items) != 2 {
		t.Fatalf("expected 2 items on page 1, got %d", len(page1.Items))
	}
	if !page1.PageInfo.HasNext {
		t.Fatalf("expected page 1 HasNext == true")
	}
	if page1.PageInfo.NextCursor == "" {
		t.Fatalf("expected page 1 NextCursor to be non-empty")
	}

	// Page 2: navigate using NextCursor
	page2, err := c.ListOrders(ctx, ListOrdersInput{
		CustID: custID,
		Cursor: page1.PageInfo.NextCursor,
	})
	if err != nil {
		t.Fatalf("ListOrders page 2 failed: %v", err)
	}
	if len(page2.Items) != 1 {
		t.Fatalf("expected 1 item on page 2, got %d", len(page2.Items))
	}
	if page2.PageInfo.HasNext {
		t.Fatalf("expected page 2 HasNext == false")
	}

	// 4. Domain Error Deserialization: InsufficientStock
	_, err = c.PlaceOrder(ctx, PlaceOrderInput{
		ProductID: prod.ID,
		Quantity:  10,
	})
	if err == nil {
		t.Fatalf("expected InsufficientStock error, got nil")
	}
	var stockErr *InsufficientStockError
	if !errors.As(err, &stockErr) {
		t.Fatalf("expected error to unwrap to *InsufficientStockError, got: %T %v", err, err)
	}
	if stockErr.StatusCode != 409 || stockErr.Kind != "conflict" {
		t.Fatalf("expected status 409 and kind conflict, got: %d %s", stockErr.StatusCode, stockErr.Kind)
	}

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected error to unwrap to *APIError")
	}

	// 5. Unauthenticated Error: client with no token
	unauthClient := NewClient(serverURL)
	_, err = unauthClient.PlaceOrder(ctx, PlaceOrderInput{
		ProductID: prod.ID,
		Quantity:  1,
	})
	if err == nil {
		t.Fatalf("expected unauthenticated error, got nil")
	}
	var unauthErr *UnauthenticatedError
	if !errors.As(err, &unauthErr) {
		t.Fatalf("expected error to unwrap to *UnauthenticatedError, got: %T %v", err, err)
	}
	if unauthErr.StatusCode != 401 {
		t.Fatalf("expected status 401, got %d", unauthErr.StatusCode)
	}

	// 6. Policy / Forbidden Error: token with is_verified: false
	forbiddenClient := NewClient(serverURL, WithBearerToken(tokenUnverified))
	_, err = forbiddenClient.PlaceOrder(ctx, PlaceOrderInput{
		ProductID: prod.ID,
		Quantity:  1,
	})
	if err == nil {
		t.Fatalf("expected forbidden error, got nil")
	}
	var forbiddenErr *UnauthorizedCustomerError
	if !errors.As(err, &forbiddenErr) {
		t.Fatalf("expected error to unwrap to *UnauthorizedCustomerError, got: %T %v", err, err)
	}
	if forbiddenErr.StatusCode != 403 {
		t.Fatalf("expected status 403, got %d", forbiddenErr.StatusCode)
	}
}
`

func TestE2E_GoSDKClient_AgainstGoHexagonalAndLayeredServers(t *testing.T) {
	dir := t.TempDir()

	buildYAML := `
project:
  name: e2e-store
  module: example.com/e2estore

authn: jwt-oidc

storage:
  dialect: memory
  mode: in_memory

targets:
  - name: hex-backend
    kind: backend
    language: go
    architecture: hexagonal
    out_dir: dist/hex
  - name: layered-backend
    kind: backend
    language: go
    architecture: layered
    out_dir: dist/layered
  - name: ts-backend
    kind: backend
    language: typescript
    architecture: clean
    out_dir: dist/ts-clean
  - name: go-client
    kind: sdk
    language: go
    out_dir: clients/go
`
	if err := os.WriteFile(filepath.Join(dir, "store.fdn"), []byte(e2eSharedBlueprint), 0644); err != nil {
		t.Fatalf("failed to write blueprint: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "foundation.build.yaml"), []byte(buildYAML), 0644); err != nil {
		t.Fatalf("failed to write build yaml: %v", err)
	}

	// 1. Compile project with all targets
	_, diags := compiler.Build(dir, compiler.BuildOptions{})
	if len(diags) > 0 {
		t.Fatalf("compiler.Build failed: %+v", diags)
	}

	goSDKDir := filepath.Join(dir, "clients", "go")
	if err := os.WriteFile(filepath.Join(goSDKDir, "client_e2e_test.go"), []byte(goSDKClientTestScript), 0644); err != nil {
		t.Fatalf("failed to write go sdk test file: %v", err)
	}

	tokenValid, tokenUnverified := getTestTokens()

	runGoSDKTest := func(serverURL, runID string) {
		cmd := exec.Command("go", "test", "-v", "-run", "TestGoSDKClientE2E", "./...")
		cmd.Dir = goSDKDir
		cmd.Env = append(os.Environ(),
			"GOPROXY=off",
			"SERVER_URL="+serverURL,
			"TOKEN_VALID="+tokenValid,
			"TOKEN_UNVERIFIED="+tokenUnverified,
			"RUN_ID="+runID,
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Go SDK client test against %s (%s) failed: %v\n%s", serverURL, runID, err, string(out))
		}
	}

	// 2. Test against Go Hexagonal server
	hexURL, stopHex := buildAndStartGoServer(t, filepath.Join(dir, "dist", "hex"))
	defer stopHex()
	runGoSDKTest(hexURL, "hex")

	// 3. Test against Go Layered server
	layeredURL, stopLayered := buildAndStartGoServer(t, filepath.Join(dir, "dist", "layered"))
	defer stopLayered()
	runGoSDKTest(layeredURL, "layered")

	// 4. Test against TypeScript Clean server
	tsURL, stopTS := startTSServer(t, filepath.Join(dir, "dist", "ts-clean"))
	defer stopTS()
	runGoSDKTest(tsURL, "tsclean")
}
