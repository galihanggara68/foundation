package compiler_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/galihanggara68/foundation/pkg/compiler"
)

func TestCheck_RejectsExternalSlotInTransactionAndNPlusOneInLoop_WithExactSpans(t *testing.T) {
	dir := t.TempDir()

	blueprint := `value Address {
  city: string
  country: string
}

value ShippingQuote {
  fee: int
  carrier: string
}

entity OrderItem {
  id: uuid @pk
  sku: string
  quantity: int
}

error ShippingUnavailable {
  kind: conflict
  message: "Shipping carrier is unavailable"
}

error ItemNotFound {
  kind: not_found
  message: "Order item not found"
}

slot ShippingCalculator {
  @external
  Quote(address: Address, subtotal: int) -> ShippingQuote throws ShippingUnavailable
}

action CheckoutOrder(address: Address, skus: []string) -> ShippingQuote
  @transactional
{
  let quote = slot ShippingCalculator.Quote(address, 100)

  for sku in skus {
    let item = query one OrderItem where sku == sku else ItemNotFound
  }

  return quote
}
`
	if err := os.WriteFile(filepath.Join(dir, "invariants.fdn"), []byte(blueprint), 0644); err != nil {
		t.Fatalf("write invariants.fdn: %v", err)
	}

	ir, diags := compiler.Check(dir)
	if ir != nil {
		t.Fatalf("expected nil SemanticIR when architectural invariants fail, got %+v", ir)
	}
	if len(diags) < 2 {
		t.Fatalf("expected at least 2 invariant diagnostics, got %d: %+v", len(diags), diags)
	}

	var foundExternalInTx, foundNPlusOne bool
	for _, d := range diags {
		if d.Span.File != "invariants.fdn" {
			t.Errorf("expected diagnostic file 'invariants.fdn', got %q", d.Span.File)
		}
		if d.Code == "FDN0213" && d.Span.Line == 35 && d.Span.Column == 3 {
			foundExternalInTx = true
		}
		if d.Code == "FDN0214" && d.Span.Line == 38 && d.Span.Column == 5 {
			foundNPlusOne = true
		}
	}

	if !foundExternalInTx {
		t.Errorf("expected FDN0213 (@external slot inside @transactional) at invariants.fdn:35:3, got %+v", diags)
	}
	if !foundNPlusOne {
		t.Errorf("expected FDN0214 (N+1 query inside for loop) at invariants.fdn:38:5, got %+v", diags)
	}
}

func TestBuild_SlotsStubsScenarioMocksAndContractTests_CompilesPreservesStubsAndPassesTests(t *testing.T) {
	dir := t.TempDir()

	blueprint := `
value Address {
  city: string
  country: string
}

value ShippingQuote {
  fee: int
  carrier: string
}

value CheckoutInput {
  order_id: uuid
  subtotal: int
  address: Address
}

entity Shipment {
  id: uuid @pk
  order_id: uuid
  carrier: string
  fee: int
}

error ShippingUnavailable {
  kind: conflict
  message: "Shipping carrier is unavailable"
}

slot ShippingCalculator {
  @external
  Quote(address: Address, subtotal: int) -> ShippingQuote throws ShippingUnavailable
}

action CreateShipment(input: CheckoutInput) -> Shipment
  @http(method: POST, path: "/v1/shipments")
{
  let quote = slot ShippingCalculator.Quote(input.address, input.subtotal)
  let shipment = create Shipment {
    order_id: input.order_id,
    carrier: quote.carrier,
    fee: quote.fee
  }
  return shipment
}

scenario "CreateShipment uses stubbed ShippingCalculator.Quote response" {
  given slot ShippingCalculator.Quote(Address { city: "Hanoi", country: "VN" }, 100) returns ShippingQuote {
    fee: 15,
    carrier: "FastExpress"
  }
  when CreateShipment(CheckoutInput {
    order_id: "00000000-0000-0000-0000-000000000001",
    subtotal: 100,
    address: Address { city: "Hanoi", country: "VN" }
  })
  expect result.carrier == "FastExpress"
  expect result.fee == 15
  expect stored Shipment where order_id == "00000000-0000-0000-0000-000000000001" {
    carrier: "FastExpress",
    fee: 15
  }
}

scenario "CreateShipment propagates ShippingUnavailable when slot throws" {
  given slot ShippingCalculator.Quote throws ShippingUnavailable
  when CreateShipment(CheckoutInput {
    order_id: "00000000-0000-0000-0000-000000000002",
    subtotal: 250,
    address: Address { city: "Remote", country: "AQ" }
  })
  expect error ShippingUnavailable
}
`

	buildProfile := `
project:
  name: shipping
  module: example.com/shipping

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

	if err := os.WriteFile(filepath.Join(dir, "shipping.fdn"), []byte(blueprint), 0644); err != nil {
		t.Fatalf("write shipping.fdn: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "foundation.build.yaml"), []byte(buildProfile), 0644); err != nil {
		t.Fatalf("write foundation.build.yaml: %v", err)
	}

	// 1. Initial Build
	if _, diags := compiler.Build(dir, compiler.BuildOptions{}); len(diags) != 0 {
		t.Fatalf("expected build with slots to succeed, got: %+v", diags)
	}

	outDir := filepath.Join(dir, "dist", "backend")
	slotPortPath := filepath.Join(outDir, "internal/ports/shippingcalculator_slot.go")
	slotStubPath := filepath.Join(outDir, "internal/slots/shippingcalculator_stub.go")
	contractTestPath := filepath.Join(outDir, "contracts_test.go")
	manifestPath := filepath.Join(outDir, ".foundation-manifest.json")

	for _, p := range []string{slotPortPath, slotStubPath, contractTestPath} {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("expected file %s to be emitted: %v", p, err)
		}
	}

	// Verify stub is NOT tracked in .foundation-manifest.json
	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	if strings.Contains(string(manifestBytes), "internal/slots/shippingcalculator_stub.go") {
		t.Fatalf("expected create-if-missing slot stub to be excluded from .foundation-manifest.json, got:\n%s", string(manifestBytes))
	}

	// 2. Verify freshly generated project compiles and passes both scenarios_test.go and contracts_test.go immediately
	if testDiags := compiler.Test(dir); len(testDiags) != 0 {
		t.Fatalf("expected generated scenarios and contract tests to pass on first build, got: %+v", testDiags)
	}

	// 3. Simulate developer editing the 10% handwritten Slot Stub
	customStub := `package slots

import (
	"context"
	"example.com/shipping/internal/domain"
)

// Handwritten custom implementation by developer
type ShippingCalculatorStub struct{}

func NewShippingCalculatorSlot() *ShippingCalculatorStub {
	return &ShippingCalculatorStub{}
}

func (s *ShippingCalculatorStub) Quote(_ context.Context, address domain.Address, subtotal int) (domain.ShippingQuote, error) {
	if address.Country == "INVALID" {
		return domain.ShippingQuote{}, domain.ErrShippingUnavailable
	}
	return domain.ShippingQuote{
		Fee:     subtotal / 10,
		Carrier: "CustomHandwrittenCarrier",
	}, nil
}
`
	if err := os.WriteFile(slotStubPath, []byte(customStub), 0644); err != nil {
		t.Fatalf("write custom slot stub: %v", err)
	}

	// 4. Rebuild without --force: must succeed without manifest conflict AND must preserve handwritten stub
	if _, diags := compiler.Build(dir, compiler.BuildOptions{Force: false}); len(diags) != 0 {
		t.Fatalf("expected rebuild after editing slot stub to succeed without conflict, got: %+v", diags)
	}

	afterBytes, err := os.ReadFile(slotStubPath)
	if err != nil {
		t.Fatalf("read slot stub after rebuild: %v", err)
	}
	if string(afterBytes) != customStub {
		t.Fatalf("expected rebuild to preserve handwritten slot stub untouched, got:\n%s", string(afterBytes))
	}

	// 5. Re-run compiler.Test to verify handwritten stub passes the Contract Test suite
	if testDiags := compiler.Test(dir); len(testDiags) != 0 {
		t.Fatalf("expected handwritten slot stub to pass contract and scenario tests, got: %+v", testDiags)
	}
}
