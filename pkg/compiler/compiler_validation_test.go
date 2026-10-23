package compiler_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/galihanggara68/foundation/pkg/compiler"
)

func TestCheck_SyntaxAndTypeErrors_ReturnsExactSpans(t *testing.T) {
	dir := t.TempDir()

	badBlueprint := `entity Book {
  id: uuid @pk
  sku: string
  price: int
}

action CreateBook(sku: string) -> Book {
  let book = create Book {
    sku: sku,
    price: "not-an-int",
    missing_field: 10
  }
  let found = query one UnknownEntity where id == sku else MissingError
  return undeclared_var
}
`
	if err := os.WriteFile(filepath.Join(dir, "bad.fdn"), []byte(badBlueprint), 0644); err != nil {
		t.Fatalf("write blueprint: %v", err)
	}

	ir, diags := compiler.Check(dir)
	if ir != nil {
		t.Fatalf("expected nil SemanticIR on validation errors, got %+v", ir)
	}
	if len(diags) < 4 {
		t.Fatalf("expected at least 4 diagnostics, got %d: %+v", len(diags), diags)
	}

	foundPriceMismatch := false
	foundMissingField := false
	foundUnknownEntity := false
	foundUndeclaredVar := false

	for _, d := range diags {
		if d.Span.File != "bad.fdn" {
			t.Errorf("expected diagnostic file 'bad.fdn', got %q", d.Span.File)
		}
		if d.Code == "FDN0205" && d.Span.Line == 10 && d.Span.Column == 12 {
			foundPriceMismatch = true
		}
		if d.Code == "FDN0204" && d.Span.Line == 11 && d.Span.Column == 5 {
			foundMissingField = true
		}
		if d.Code == "FDN0203" && d.Span.Line == 13 && d.Span.Column == 3 {
			foundUnknownEntity = true
		}
		if d.Code == "FDN0208" && d.Span.Line == 14 && d.Span.Column == 10 {
			foundUndeclaredVar = true
		}
	}

	if !foundPriceMismatch {
		t.Errorf("expected FDN0205 price type mismatch at bad.fdn:10:12, got %+v", diags)
	}
	if !foundMissingField {
		t.Errorf("expected FDN0204 unknown field at bad.fdn:11:5, got %+v", diags)
	}
	if !foundUnknownEntity {
		t.Errorf("expected FDN0203 unknown entity at bad.fdn:13:3, got %+v", diags)
	}
	if !foundUndeclaredVar {
		t.Errorf("expected FDN0208 undeclared var at bad.fdn:14:10, got %+v", diags)
	}
}

func TestBuild_MultiFileEnumsPoliciesGuardsEvents_CompilesAndPassesScenarios(t *testing.T) {
	dir := t.TempDir()

	ordersFDN := `
action PlaceOrder(input: PlaceOrderInput) -> Order
  @http(method: POST, path: "/v1/orders")
{
  require CanPlaceOrder(actor) else UnauthorizedCustomer

  let product = query one Product where id == input.product_id else ProductNotFound
  guard product.stock >= input.quantity else InsufficientStock

  let order = create Order {
    customer_id: actor.id,
    total: product.price * input.quantity,
    status: .confirmed
  }

  emit OrderPlaced {
    order_id: order.id,
    customer_id: order.customer_id,
    total: order.total
  }

  return order
}

scenario "PlaceOrder succeeds when stock is sufficient and actor is verified" {
  given Product {
    id: "00000000-0000-0000-0000-000000000001",
    sku: "BOOK-1",
    price: 25.00,
    stock: 10
  }
  given actor {
    id: "00000000-0000-0000-0000-000000000099",
    role: "customer",
    is_verified: true
  }
  when PlaceOrder(PlaceOrderInput {
    product_id: "00000000-0000-0000-0000-000000000001",
    quantity: 2
  })
  expect result.status == .confirmed
  expect result.total == 50.00
  expect emitted OrderPlaced {
    customer_id: "00000000-0000-0000-0000-000000000099",
    total: 50.00
  }
}

scenario "PlaceOrder rejects unverified actor via policy" {
  given Product {
    id: "00000000-0000-0000-0000-000000000001",
    sku: "BOOK-1",
    price: 25.00,
    stock: 10
  }
  given actor {
    id: "00000000-0000-0000-0000-000000000099",
    role: "customer",
    is_verified: false
  }
  when PlaceOrder(PlaceOrderInput {
    product_id: "00000000-0000-0000-0000-000000000001",
    quantity: 2
  })
  expect error UnauthorizedCustomer
}

scenario "PlaceOrder rejects when stock is insufficient via guard" {
  given Product {
    id: "00000000-0000-0000-0000-000000000001",
    sku: "BOOK-1",
    price: 25.00,
    stock: 1
  }
  given actor {
    id: "00000000-0000-0000-0000-000000000099",
    role: "customer",
    is_verified: true
  }
  when PlaceOrder(PlaceOrderInput {
    product_id: "00000000-0000-0000-0000-000000000001",
    quantity: 5
  })
  expect error InsufficientStock
}
`

	policiesFDN := `
policy CanPlaceOrder(actor: Actor) {
  actor.role == "customer" && actor.is_verified == true
}

event OrderPlaced {
  order_id: uuid
  customer_id: uuid
  total: decimal(10, 2)
}

error ProductNotFound {
  kind: not_found
  message: "Product was not found"
}

error InsufficientStock {
  kind: conflict
  message: "Requested quantity exceeds available product stock"
}

error UnauthorizedCustomer {
  kind: forbidden
  message: "Only verified customers may place orders"
}
`

	domainFDN := `
enum OrderStatus {
  pending
  confirmed
  fulfilled
  cancelled
}

value PlaceOrderInput {
  product_id: uuid
  quantity: int
}

entity Product {
  id: uuid @pk
  sku: string @unique
  price: decimal(10, 2)
  stock: int = 0
}

entity Order {
  id: uuid @pk
  customer_id: uuid
  total: decimal(10, 2)
  status: OrderStatus = .pending
}
`

	buildProfile := `
project:
  name: ordering
  module: example.com/ordering

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

	if err := os.WriteFile(filepath.Join(dir, "a_orders.fdn"), []byte(ordersFDN), 0644); err != nil {
		t.Fatalf("write a_orders.fdn: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "m_policies.fdn"), []byte(policiesFDN), 0644); err != nil {
		t.Fatalf("write m_policies.fdn: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "z_domain.fdn"), []byte(domainFDN), 0644); err != nil {
		t.Fatalf("write z_domain.fdn: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "foundation.build.yaml"), []byte(buildProfile), 0644); err != nil {
		t.Fatalf("write build profile: %v", err)
	}

	if _, diags := compiler.Build(dir, compiler.BuildOptions{}); len(diags) != 0 {
		t.Fatalf("expected multi-file build to succeed, got diagnostics: %+v", diags)
	}

	if testDiags := compiler.Test(dir); len(testDiags) != 0 {
		t.Fatalf("expected generated scenario tests to pass, got: %+v", testDiags)
	}
}

func TestBuild_MutationsBranchingLoopsAndCombinators_CompilesAndPassesScenarios(t *testing.T) {
	dir := t.TempDir()

	blueprint := `
enum FulfillmentTier {
  standard
  express
  priority
}

value LineItem {
  sku: string
  price: int
  quantity: int
  in_stock: bool
}

value CartSummaryInput {
  tier: FulfillmentTier
  discount_code: string
  items: []LineItem
}

value CartSummary {
  active_count: int
  subtotal: int
  shipping_fee: int
  final_total: int
  all_available: bool
  has_bulk_item: bool
  line_totals: []int
}

entity Coupon {
  id: uuid @pk
  code: string @unique
  discount_amount: int
  uses_remaining: int
}

error CouponNotFound {
  kind: not_found
  message: "Coupon code does not exist"
}

error EmptyCart {
  kind: validation
  message: "Cart must have at least one active item"
}

action SummarizeCart(input: CartSummaryInput) -> CartSummary
  @http(method: POST, path: "/v1/cart/summarize")
{
  let valid_items = input.items.filter(item => item.quantity > 0)
  let active_count = valid_items.count()
  guard active_count > 0 else EmptyCart

  let line_totals = valid_items.map(item => item.price * item.quantity)
  let subtotal = valid_items.sum(item => item.price * item.quantity)
  let all_available = valid_items.all(item => item.in_stock == true)
  let has_bulk_item = valid_items.any(item => item.quantity >= 5)

  let shipping_fee = 0
  match input.tier {
    .standard => {
      let shipping_fee = 5
    }
    .express => {
      let shipping_fee = 15
    }
    .priority => {
      let shipping_fee = 30
    }
  }

  let bonus_discount = 0
  for item in valid_items {
    if item.quantity >= 10 {
      let bonus_discount = bonus_discount + 5
    } else {
      let bonus_discount = bonus_discount + 0
    }
  }

  let final_total = subtotal + shipping_fee - bonus_discount

  return CartSummary {
    active_count: active_count,
    subtotal: subtotal,
    shipping_fee: shipping_fee,
    final_total: final_total,
    all_available: all_available,
    has_bulk_item: has_bulk_item,
    line_totals: line_totals
  }
}

action RedeemCoupon(code: string) -> Coupon
  @http(method: POST, path: "/v1/coupons/{code}/redeem")
{
  let coupon = query one Coupon where code == code else CouponNotFound
  update Coupon where code == code set {
    uses_remaining: coupon.uses_remaining - 1
  }
  let updated = query one Coupon where code == code else CouponNotFound
  return updated
}

action RevokeCoupon(code: string) -> bool
  @http(method: DELETE, path: "/v1/coupons/{code}")
{
  let coupon = query one Coupon where code == code else CouponNotFound
  delete Coupon where code == coupon.code
  return true
}

scenario "SummarizeCart computes combinators, match, if/else, and bounded loop" {
  when SummarizeCart(CartSummaryInput {
    tier: .express,
    discount_code: "NONE",
    items: [
      LineItem { sku: "A", price: 10, quantity: 2, in_stock: true },
      LineItem { sku: "B", price: 5, quantity: 10, in_stock: true },
      LineItem { sku: "C", price: 99, quantity: 0, in_stock: false }
    ]
  })
  expect result.active_count == 2
  expect result.subtotal == 70
  expect result.shipping_fee == 15
  expect result.final_total == 80
  expect result.all_available == true
  expect result.has_bulk_item == true
}

scenario "RedeemCoupon updates uses_remaining in repository" {
  given Coupon {
    id: "00000000-0000-0000-0000-000000000010",
    code: "SAVE20",
    discount_amount: 20,
    uses_remaining: 3
  }
  when RedeemCoupon("SAVE20")
  expect result.uses_remaining == 2
  expect stored Coupon where code == "SAVE20" {
    uses_remaining: 2
  }
}

scenario "RevokeCoupon deletes coupon from repository" {
  given Coupon {
    id: "00000000-0000-0000-0000-000000000011",
    code: "EXPIRED",
    discount_amount: 10,
    uses_remaining: 1
  }
  when RevokeCoupon("EXPIRED")
  expect result == true
  expect not_stored Coupon where code == "EXPIRED"
}
`

	buildProfile := `
project:
  name: cart
  module: example.com/cart

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

	if err := os.WriteFile(filepath.Join(dir, "cart.fdn"), []byte(blueprint), 0644); err != nil {
		t.Fatalf("write cart.fdn: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "foundation.build.yaml"), []byte(buildProfile), 0644); err != nil {
		t.Fatalf("write build profile: %v", err)
	}

	if _, diags := compiler.Build(dir, compiler.BuildOptions{}); len(diags) != 0 {
		t.Fatalf("expected build to succeed, got diagnostics: %+v", diags)
	}

	if testDiags := compiler.Test(dir); len(testDiags) != 0 {
		t.Fatalf("expected generated scenario tests to pass, got: %+v", testDiags)
	}
}

func TestCheck_RejectsUnboundedWhileRecursionAndNonExhaustiveMatch_WithExactSpans(t *testing.T) {
	dir := t.TempDir()

	badGuardsFDN := `enum OrderStatus {
  pending
  confirmed
  cancelled
}

action BadControlFlow(status: OrderStatus, n: int) -> int {
  while n > 0 {
    let n = n - 1
  }

  match status {
    .pending => {
      let x = 1
    }
  }

  let recurse = BadControlFlow(status, n - 1)
  return recurse
}
`
	if err := os.WriteFile(filepath.Join(dir, "guards.fdn"), []byte(badGuardsFDN), 0644); err != nil {
		t.Fatalf("write guards.fdn: %v", err)
	}

	ir, diags := compiler.Check(dir)
	if ir != nil {
		t.Fatalf("expected nil SemanticIR when static guardrails fail, got %+v", ir)
	}

	var foundWhile, foundNonExhaustive, foundRecursion bool
	for _, d := range diags {
		if d.Span.File != "guards.fdn" {
			t.Errorf("expected diagnostic file 'guards.fdn', got %q", d.Span.File)
		}
		if d.Code == "FDN0210" && d.Span.Line == 8 && d.Span.Column == 3 {
			foundWhile = true
		}
		if d.Code == "FDN0212" && d.Span.Line == 12 && d.Span.Column == 3 {
			foundNonExhaustive = true
		}
		if d.Code == "FDN0211" && d.Span.Line == 18 && d.Span.Column == 17 {
			foundRecursion = true
		}
	}

	if !foundWhile {
		t.Errorf("expected FDN0210 unbounded while diagnostic at guards.fdn:8:3, got %+v", diags)
	}
	if !foundNonExhaustive {
		t.Errorf("expected FDN0212 non-exhaustive match diagnostic at guards.fdn:12:3, got %+v", diags)
	}
	if !foundRecursion {
		t.Errorf("expected FDN0211 recursive action call diagnostic at guards.fdn:18:17, got %+v", diags)
	}
}
