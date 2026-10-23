package compiler_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/galihanggara68/foundation/pkg/compiler"
)

const sharedMultiTargetBlueprint = `
enum OrderStatus {
  pending
  confirmed
  cancelled
}

value QuoteRequest {
  sku: string
  quantity: int
}

value QuoteResult {
  unit_price: int
  discount: int
}

value PlaceOrderInput {
  request_id: string
  product_id: uuid
  quantity: int
}

entity Product {
  id: uuid @pk
  sku: string @unique
  stock: int = 0
}

entity Order {
  id: uuid @pk
  customer_id: uuid
  total: int
  status: OrderStatus = .pending
}

error InsufficientStock {
  kind: conflict
  message: "Requested quantity exceeds available product stock"
}

error UnauthorizedCustomer {
  kind: forbidden
  message: "Only verified customers may place orders"
}

error ProductNotFound {
  kind: not_found
  message: "Product does not exist"
}

error QuoteUnavailable {
  kind: conflict
  message: "Pricing quote service unavailable"
}

event OrderPlaced {
  order_id: uuid
  customer_id: uuid
  total: int
}

policy CanPlaceOrder(actor: Actor) {
  actor.role == "customer" && actor.is_verified == true
}

slot PricingEngine {
  @external
  CalculateQuote(req: QuoteRequest) -> QuoteResult throws QuoteUnavailable
}

action PlaceOrder(input: PlaceOrderInput) -> Order
  @http(method: POST, path: "/v1/orders")
  @idempotent(key: input.request_id)
  @retry(attempts: 2, backoff: 0)
  @outbox
{
  require CanPlaceOrder(actor) else UnauthorizedCustomer

  let product = query one Product where id == input.product_id else ProductNotFound
  guard product.stock >= input.quantity else InsufficientStock

  let quote = slot PricingEngine.CalculateQuote(QuoteRequest {
    sku: product.sku,
    quantity: input.quantity
  })

  update Product where id == product.id set {
    stock: product.stock - input.quantity
  }

  let total = quote.unit_price * input.quantity - quote.discount
  let order = create Order {
    customer_id: actor.id,
    total: total,
    status: .confirmed
  }

  emit OrderPlaced {
    order_id: order.id,
    customer_id: order.customer_id,
    total: order.total
  }

  return order
}

action CancelOrder(order_id: uuid) -> Order
  @http(method: POST, path: "/v1/orders/{order_id}/cancel")
  @transactional
{
  let order = query one Order where id == order_id else ProductNotFound
  update Order where id == order.id set {
    status: .cancelled
  }
  let updated = query one Order where id == order.id else ProductNotFound
  guard order.status == .confirmed else InsufficientStock
  return updated
}

scenario "PlaceOrder succeeds with PricingEngine quote and emits OrderPlaced" {
  given Product {
    id: "00000000-0000-0000-0000-000000000001",
    sku: "LAPTOP-1",
    stock: 10
  }
  given actor {
    id: "00000000-0000-0000-0000-000000000099",
    role: "customer",
    is_verified: true
  }
  given slot PricingEngine.CalculateQuote(QuoteRequest { sku: "LAPTOP-1", quantity: 2 }) returns QuoteResult {
    unit_price: 500,
    discount: 50
  }
  when PlaceOrder(PlaceOrderInput {
    request_id: "req-layered-1",
    product_id: "00000000-0000-0000-0000-000000000001",
    quantity: 2
  })
  expect result.status == .confirmed
  expect result.total == 950
  expect stored Product where id == "00000000-0000-0000-0000-000000000001" {
    stock: 8
  }
  expect emitted OrderPlaced {
    customer_id: "00000000-0000-0000-0000-000000000099",
    total: 950
  }
}

scenario "PlaceOrder propagates QuoteUnavailable when PricingEngine throws" {
  given Product {
    id: "00000000-0000-0000-0000-000000000001",
    sku: "LAPTOP-1",
    stock: 10
  }
  given actor {
    id: "00000000-0000-0000-0000-000000000099",
    role: "customer",
    is_verified: true
  }
  given slot PricingEngine.CalculateQuote throws QuoteUnavailable
  when PlaceOrder(PlaceOrderInput {
    request_id: "req-layered-2",
    product_id: "00000000-0000-0000-0000-000000000001",
    quantity: 2
  })
  expect error QuoteUnavailable
}

scenario "CancelOrder rolls back status update when transactional guard fails" {
  given Order {
    id: "00000000-0000-0000-0000-000000000055",
    customer_id: "00000000-0000-0000-0000-000000000099",
    total: 950,
    status: .pending
  }
  when CancelOrder("00000000-0000-0000-0000-000000000055")
  expect error InsufficientStock
}
`

func TestBuildAndTest_GoLayeredArchitecture_AndSwitchBetweenHexagonalAndLayered(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "commerce.fdn"), []byte(sharedMultiTargetBlueprint), 0644); err != nil {
		t.Fatalf("write commerce.fdn: %v", err)
	}

	writeProfile := func(arch string) {
		profile := `project:
  name: commerce
  module: example.com/commerce

storage:
  dialect: postgres
  mode: declarative

targets:
  - name: backend
    kind: backend
    language: go
    architecture: ` + arch + `
    http: chi
    out_dir: ./dist/backend
`
		if err := os.WriteFile(filepath.Join(dir, "foundation.build.yaml"), []byte(profile), 0644); err != nil {
			t.Fatalf("write foundation.build.yaml: %v", err)
		}
	}

	outDir := filepath.Join(dir, "dist", "backend")

	// 1. Build as hexagonal first
	writeProfile("hexagonal")
	if _, diags := compiler.Build(dir, compiler.BuildOptions{}); len(diags) != 0 {
		t.Fatalf("initial hexagonal build failed: %+v", diags)
	}

	// Customize the create-if-missing Slot Stub
	stubPath := filepath.Join(outDir, "internal/slots/pricingengine_stub.go")
	origStubBytes, err := os.ReadFile(stubPath)
	if err != nil {
		t.Fatalf("read pricingengine_stub.go: %v", err)
	}
	customStub := string(origStubBytes) + "\n// Custom developer comment preserved across architecture switches\n"
	if err := os.WriteFile(stubPath, []byte(customStub), 0644); err != nil {
		t.Fatalf("write custom stub: %v", err)
	}

	// 2. Switch Build Profile from hexagonal -> layered
	writeProfile("layered")
	res, diags := compiler.Build(dir, compiler.BuildOptions{})
	if len(diags) != 0 {
		t.Fatalf("expected layered build to succeed, got diagnostics: %+v", diags)
	}
	if len(res.Architectures) != 1 || res.Architectures[0].Pattern != "layered" {
		t.Fatalf("expected lowered ArchitectureIR with pattern 'layered', got: %+v", res.Architectures)
	}

	// Verify layered files exist
	expectedLayeredFiles := []string{
		"internal/models/order_entity.go",
		"internal/models/values.go",
		"internal/models/errors.go",
		"internal/repositories/order_repository.go",
		"internal/repositories/order_memory.go",
		"internal/repositories/order_postgres.go",
		"internal/services/placeorder_service.go",
		"internal/handlers/router.go",
		"internal/bootstrap/container.go",
		"contracts_test.go",
		"scenarios_test.go",
	}
	for _, rel := range expectedLayeredFiles {
		if _, err := os.Stat(filepath.Join(outDir, rel)); err != nil {
			t.Errorf("expected layered file %s to exist: %v", rel, err)
		}
	}

	// Verify old hexagonal files were pruned via .foundation-manifest.json
	prunedHexFiles := []string{
		"internal/domain/order_entity.go",
		"internal/ports/order_repository.go",
		"internal/usecases/placeorder_usecase.go",
		"internal/adapters/http/router.go",
		"internal/adapters/memory/order_repository.go",
		"internal/adapters/postgres/order_repository.go",
	}
	for _, rel := range prunedHexFiles {
		if _, err := os.Stat(filepath.Join(outDir, rel)); !os.IsNotExist(err) {
			t.Errorf("expected hexagonal file %s to be pruned after switching to layered, err=%v", rel, err)
		}
	}

	// Verify the pruning also removed the hexagonal layout's now-empty directories
	for _, dir := range []string{"internal/adapters/http", "internal/adapters/memory", "internal/adapters/postgres", "internal/adapters"} {
		if _, err := os.Stat(filepath.Join(outDir, dir)); !os.IsNotExist(err) {
			t.Errorf("expected empty hexagonal directory %s to be pruned after switching to layered, err=%v", dir, err)
		}
	}

	// Verify handwritten Slot Stub was preserved untouched
	afterStubBytes, err := os.ReadFile(stubPath)
	if err != nil {
		t.Fatalf("read pricingengine_stub.go after switch: %v", err)
	}
	if !strings.Contains(string(afterStubBytes), "Custom developer comment preserved across architecture switches") {
		t.Fatalf("expected handwritten Slot Stub to be preserved across architecture switch, got:\n%s", string(afterStubBytes))
	}

	// 3. Execute go test ./... on the layered codebase
	if testDiags := compiler.Test(dir); len(testDiags) != 0 {
		t.Fatalf("expected go test ./... to pass on layered architecture, got: %+v", testDiags)
	}

	// 4. Switch back from layered -> hexagonal and verify pruning + tests pass
	writeProfile("hexagonal")
	if _, diags := compiler.Build(dir, compiler.BuildOptions{}); len(diags) != 0 {
		t.Fatalf("expected switch back to hexagonal to succeed, got: %+v", diags)
	}
	if _, err := os.Stat(filepath.Join(outDir, "internal/services/placeorder_service.go")); !os.IsNotExist(err) {
		t.Fatalf("expected layered service file to be pruned when switching back to hexagonal")
	}
	if testDiags := compiler.Test(dir); len(testDiags) != 0 {
		t.Fatalf("expected go test ./... to pass after switching back to hexagonal, got: %+v", testDiags)
	}
}
