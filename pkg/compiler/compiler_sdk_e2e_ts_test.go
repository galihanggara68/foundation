package compiler_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/galihanggara68/foundation/pkg/compiler"
)

const tsSDKClientTestScript = `import { test } from "node:test";
import assert from "node:assert";
import { Client } from "./client.ts";
import {
  InsufficientStockError,
  UnauthorizedCustomerError,
  UnauthenticatedError,
} from "./errors.ts";

test("TypeScript SDK Client E2E against running backend", async () => {
  const serverURL = process.env.SERVER_URL || "";
  const tokenValid = process.env.TOKEN_VALID || "";
  const tokenUnverified = process.env.TOKEN_UNVERIFIED || "";
  const runID = process.env.RUN_ID || "1";

  const client = new Client({
    baseURL: serverURL,
    authToken: tokenValid,
  });

  // 1. Mutation: CreateProduct
  const prod = await client.createProduct({
    name: "TS Widget",
    sku: "TS-WIDGET-" + runID,
    price: 60,
    stock: 5,
  });
  assert.ok(prod.id, "expected product id");
  assert.strictEqual(prod.name, "TS Widget");

  // 2. Mutation: PlaceOrder 1, 2, 3
  const ord1 = await client.placeOrder({
    product_id: prod.id,
    quantity: 1,
  });
  assert.ok(ord1.id, "expected order id");
  assert.strictEqual(ord1.total, 60);
  assert.strictEqual(ord1.status, "confirmed");

  await client.placeOrder({ product_id: prod.id, quantity: 1 });
  await client.placeOrder({ product_id: prod.id, quantity: 1 });

  // 3. Keyset Pagination: ListOrders
  const page1 = await client.listOrders({
    cust_id: ord1.customer_id,
    cursor: "",
  });
  assert.strictEqual(page1.items.length, 2, "expected 2 items on page 1");
  assert.strictEqual(page1.page_info.has_next, true, "expected page 1 has_next true");
  assert.ok(page1.page_info.next_cursor, "expected page 1 next_cursor");

  const page2 = await client.listOrders({
    cust_id: ord1.customer_id,
    cursor: page1.page_info.next_cursor,
  });
  assert.strictEqual(page2.items.length, 1, "expected 1 item on page 2");
  assert.strictEqual(page2.page_info.has_next, false, "expected page 2 has_next false");

  // 4. Domain Error: InsufficientStock
  await assert.rejects(
    async () => {
      await client.placeOrder({ product_id: prod.id, quantity: 10 });
    },
    (err: any) => {
      assert(err instanceof InsufficientStockError, "expected InsufficientStockError instance");
      assert.strictEqual(err.statusCode, 409);
      assert.strictEqual(err.kind, "conflict");
      assert.strictEqual(err.code, "InsufficientStock");
      return true;
    }
  );

  // 5. Unauthenticated Error: client with no auth token
  const unauthClient = new Client({ baseURL: serverURL });
  await assert.rejects(
    async () => {
      await unauthClient.placeOrder({ product_id: prod.id, quantity: 1 });
    },
    (err: any) => {
      assert(err instanceof UnauthenticatedError, "expected UnauthenticatedError instance");
      assert.strictEqual(err.statusCode, 401);
      return true;
    }
  );

  // 6. Policy / Forbidden Error: client with token having is_verified: false
  const forbiddenClient = new Client({ baseURL: serverURL, authToken: tokenUnverified });
  await assert.rejects(
    async () => {
      await forbiddenClient.placeOrder({ product_id: prod.id, quantity: 1 });
    },
    (err: any) => {
      assert(err instanceof UnauthorizedCustomerError, "expected UnauthorizedCustomerError instance");
      assert.strictEqual(err.statusCode, 403);
      return true;
    }
  );
});
`

func TestE2E_TypeScriptSDKClient_AgainstGoAndTypeScriptServers(t *testing.T) {
	dir := t.TempDir()

	buildYAML := `
project:
  name: e2e-store-ts
  module: example.com/e2estorets

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
  - name: ts-client
    kind: sdk
    language: typescript
    out_dir: clients/ts
`
	if err := os.WriteFile(filepath.Join(dir, "store.fdn"), []byte(e2eSharedBlueprint), 0644); err != nil {
		t.Fatalf("failed to write blueprint: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "foundation.build.yaml"), []byte(buildYAML), 0644); err != nil {
		t.Fatalf("failed to write build yaml: %v", err)
	}

	// 1. Compile project with targets
	_, diags := compiler.Build(dir, compiler.BuildOptions{})
	if len(diags) > 0 {
		t.Fatalf("compiler.Build failed: %+v", diags)
	}

	tsSDKDir := filepath.Join(dir, "clients", "ts")
	if err := os.WriteFile(filepath.Join(tsSDKDir, "client_e2e.test.ts"), []byte(tsSDKClientTestScript), 0644); err != nil {
		t.Fatalf("failed to write ts sdk test file: %v", err)
	}

	tokenValid, tokenUnverified := getTestTokens()

	runTSSDKTest := func(serverURL, runID string) {
		cmd := exec.Command("node", "--experimental-strip-types", "--test", "client_e2e.test.ts")
		cmd.Dir = tsSDKDir
		cmd.Env = append(os.Environ(),
			"SERVER_URL="+serverURL,
			"TOKEN_VALID="+tokenValid,
			"TOKEN_UNVERIFIED="+tokenUnverified,
			"RUN_ID="+runID,
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("TypeScript SDK client test against %s (%s) failed: %v\n%s", serverURL, runID, err, string(out))
		}
	}

	// 2. Test against Go Hexagonal server
	hexURL, stopHex := buildAndStartGoServer(t, filepath.Join(dir, "dist", "hex"))
	defer stopHex()
	runTSSDKTest(hexURL, "hex")

	// 3. Test against Go Layered server
	layeredURL, stopLayered := buildAndStartGoServer(t, filepath.Join(dir, "dist", "layered"))
	defer stopLayered()
	runTSSDKTest(layeredURL, "layered")

	// 4. Test against TypeScript Clean server
	tsURL, stopTS := startTSServer(t, filepath.Join(dir, "dist", "ts-clean"))
	defer stopTS()
	runTSSDKTest(tsURL, "tsclean")
}
