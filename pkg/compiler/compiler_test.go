package compiler_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/galihanggara68/foundation/pkg/compiler"
)

func TestCheck_ValidMinimalBlueprint_ReturnsSemanticIR(t *testing.T) {
	dir := t.TempDir()

	blueprint := `
value CreateBookInput {
  sku: string
  title: string
  price: int
}

entity Book {
  id: uuid @pk
  sku: string @unique
  title: string
  price: int
}

error BookNotFound {
  kind: not_found
  message: "Requested book was not found"
}

action CreateBook(input: CreateBookInput) -> Book
  @http(method: POST, path: "/v1/books")
{
  let book = create Book {
    sku: input.sku,
    title: input.title,
    price: input.price
  }
  return book
}

action GetBookBySKU(sku: string) -> Book
  @http(method: GET, path: "/v1/books/{sku}")
{
  let book = query one Book where sku == sku else BookNotFound
  return book
}

scenario "Create and query a book" {
  given Book {
    id: "00000000-0000-0000-0000-000000000001",
    sku: "FDN-101",
    title: "Domain Compilers",
    price: 45
  }
  when GetBookBySKU("FDN-101")
  expect result.title == "Domain Compilers"
  expect result.price == 45
}
`
	if err := os.WriteFile(filepath.Join(dir, "catalog.fdn"), []byte(blueprint), 0644); err != nil {
		t.Fatalf("write blueprint: %v", err)
	}

	ir, diags := compiler.Check(dir)
	if len(diags) != 0 {
		t.Fatalf("expected 0 diagnostics, got %+v", diags)
	}
	if ir == nil {
		t.Fatal("expected non-nil SemanticIR")
	}
	if len(ir.Entities) != 1 || ir.Entities[0].Name != "Book" {
		t.Fatalf("expected 1 entity 'Book', got %+v", ir.Entities)
	}
	if len(ir.Values) != 1 || ir.Values[0].Name != "CreateBookInput" {
		t.Fatalf("expected 1 value 'CreateBookInput', got %+v", ir.Values)
	}
	if len(ir.Errors) != 1 || ir.Errors[0].Name != "BookNotFound" {
		t.Fatalf("expected 1 error 'BookNotFound', got %+v", ir.Errors)
	}
	if len(ir.Actions) != 2 {
		t.Fatalf("expected 2 actions, got %d", len(ir.Actions))
	}
	if len(ir.Scenarios) != 1 || ir.Scenarios[0].Name != "Create and query a book" {
		t.Fatalf("expected 1 scenario, got %+v", ir.Scenarios)
	}
}

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

	// Verify exact file, line, and column on the price type mismatch ("not-an-int" is on line 10, col 12)
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

func TestBuild_GoHexagonal_LowersAndEmitsTestableWorkspace(t *testing.T) {
	dir := t.TempDir()

	blueprint := `
value CreateBookInput {
  sku: string
  title: string
  price: int
}

entity Book {
  id: uuid @pk
  sku: string @unique
  title: string
  price: int
}

error BookNotFound {
  kind: not_found
  message: "Requested book was not found"
}

action CreateBook(input: CreateBookInput) -> Book
  @http(method: POST, path: "/v1/books")
{
  let book = create Book {
    sku: input.sku,
    title: input.title,
    price: input.price
  }
  return book
}

action GetBookBySKU(sku: string) -> Book
  @http(method: GET, path: "/v1/books/{sku}")
{
  let book = query one Book where sku == sku else BookNotFound
  return book
}

scenario "CreateBook persists and returns book" {
  when CreateBook(CreateBookInput {
    sku: "FDN-200",
    title: "Hexagonal Go",
    price: 60
  })
  expect result.sku == "FDN-200"
  expect result.title == "Hexagonal Go"
  expect result.price == 60
}

scenario "GetBookBySKU returns existing seeded book" {
  given Book {
    id: "00000000-0000-0000-0000-000000000001",
    sku: "FDN-101",
    title: "Domain Compilers",
    price: 45
  }
  when GetBookBySKU("FDN-101")
  expect result.title == "Domain Compilers"
  expect result.price == 45
}

scenario "GetBookBySKU raises BookNotFound when missing" {
  when GetBookBySKU("MISSING-SKU")
  expect error BookNotFound
}
`
	buildProfile := `
project:
  name: bookstore
  module: example.com/bookstore

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

	if err := os.WriteFile(filepath.Join(dir, "catalog.fdn"), []byte(blueprint), 0644); err != nil {
		t.Fatalf("write blueprint: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "foundation.build.yaml"), []byte(buildProfile), 0644); err != nil {
		t.Fatalf("write build profile: %v", err)
	}

	res, diags := compiler.Build(dir, compiler.BuildOptions{})
	if len(diags) != 0 {
		t.Fatalf("expected 0 diagnostics, got %+v", diags)
	}
	if res == nil || len(res.Architectures) != 1 {
		t.Fatalf("expected 1 lowered ArchitectureIR, got %+v", res)
	}
	arch := res.Architectures[0]
	if arch.Pattern != "hexagonal" || arch.Language != "go" {
		t.Fatalf("expected go hexagonal ArchitectureIR, got %+v", arch)
	}

	outDir := filepath.Join(dir, "dist", "backend")
	expectedFiles := []string{
		".foundation-manifest.json",
		"go.mod",
		"internal/domain/book_entity.go",
		"internal/domain/values.go",
		"internal/domain/errors.go",
		"internal/ports/book_repository.go",
		"internal/usecases/createbook_usecase.go",
		"internal/usecases/getbookbysku_usecase.go",
		"internal/adapters/http/router.go",
		"internal/adapters/memory/book_repository.go",
		"internal/bootstrap/container.go",
		"scenarios_test.go",
	}
	for _, rel := range expectedFiles {
		full := filepath.Join(outDir, rel)
		if _, err := os.Stat(full); err != nil {
			t.Errorf("expected emitted file %s to exist: %v", rel, err)
		}
	}

	// Execute compiler.Test(dir) which runs `go test ./...` inside emitted Go targets
	testDiags := compiler.Test(dir)
	if len(testDiags) != 0 {
		t.Fatalf("expected emitted Go hexagonal workspace `go test ./...` to pass, got diagnostics: %+v", testDiags)
	}
}

func TestBuild_GenerationManifest_PrunesStaleAndDetectsManualConflicts(t *testing.T) {
	dir := t.TempDir()

	blueprintV1 := `
entity Book {
  id: uuid @pk
  sku: string
}

entity Author {
  id: uuid @pk
  name: string
}

action CreateBook(sku: string) -> Book {
  let b = create Book { sku: sku }
  return b
}

action CreateAuthor(name: string) -> Author {
  let a = create Author { name: name }
  return a
}
`
	buildProfile := `
project:
  name: library
  module: example.com/library

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
	fdnPath := filepath.Join(dir, "library.fdn")
	if err := os.WriteFile(fdnPath, []byte(blueprintV1), 0644); err != nil {
		t.Fatalf("write blueprint v1: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "foundation.build.yaml"), []byte(buildProfile), 0644); err != nil {
		t.Fatalf("write build profile: %v", err)
	}

	if _, diags := compiler.Build(dir, compiler.BuildOptions{}); len(diags) != 0 {
		t.Fatalf("v1 build failed: %+v", diags)
	}

	outDir := filepath.Join(dir, "dist", "backend")
	authorEntityFile := filepath.Join(outDir, "internal/domain/author_entity.go")
	authorUseCaseFile := filepath.Join(outDir, "internal/usecases/createauthor_usecase.go")
	if _, err := os.Stat(authorEntityFile); err != nil {
		t.Fatalf("expected author_entity.go in v1 build: %v", err)
	}
	if _, err := os.Stat(authorUseCaseFile); err != nil {
		t.Fatalf("expected createauthor_usecase.go in v1 build: %v", err)
	}

	// 1. Remove Author entity and CreateAuthor action -> rebuild should prune stale files
	blueprintV2 := `
entity Book {
  id: uuid @pk
  sku: string
}

action CreateBook(sku: string) -> Book {
  let b = create Book { sku: sku }
  return b
}
`
	if err := os.WriteFile(fdnPath, []byte(blueprintV2), 0644); err != nil {
		t.Fatalf("write blueprint v2: %v", err)
	}

	if _, diags := compiler.Build(dir, compiler.BuildOptions{}); len(diags) != 0 {
		t.Fatalf("v2 build failed: %+v", diags)
	}

	if _, err := os.Stat(authorEntityFile); !os.IsNotExist(err) {
		t.Errorf("expected stale file author_entity.go to be pruned, got err=%v", err)
	}
	if _, err := os.Stat(authorUseCaseFile); !os.IsNotExist(err) {
		t.Errorf("expected stale file createauthor_usecase.go to be pruned, got err=%v", err)
	}

	// 2. Tamper manually with a tracked file -> Build without Force must abort with FDN0402 conflict diagnostic
	bookEntityFile := filepath.Join(outDir, "internal/domain/book_entity.go")
	if err := os.WriteFile(bookEntityFile, []byte("// manual human edit\npackage domain\n"), 0644); err != nil {
		t.Fatalf("tamper with book_entity.go: %v", err)
	}

	_, conflictDiags := compiler.Build(dir, compiler.BuildOptions{Force: false})
	if len(conflictDiags) == 0 {
		t.Fatal("expected conflict diagnostic when tracked file was manually edited, got none")
	}
	if conflictDiags[0].Code != "FDN0402" {
		t.Fatalf("expected diagnostic code FDN0402, got %+v", conflictDiags)
	}

	// 3. Rebuild with Force: true -> succeeds and restores deterministic file content
	if _, forceDiags := compiler.Build(dir, compiler.BuildOptions{Force: true}); len(forceDiags) != 0 {
		t.Fatalf("expected forced build to succeed, got %+v", forceDiags)
	}
}

func TestBuild_MultiFileEnumsPoliciesGuardsEvents_CompilesAndPassesScenarios(t *testing.T) {
	dir := t.TempDir()

	// File 1: z_orders.fdn (intentionally named with 'z_' so it is parsed AFTER a_types.fdn or vice versa to test order independence)
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

	// File 2: m_policies.fdn (references entities/enums in z_domain.fdn)
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

	// File 3: z_domain.fdn (alphabetically last, defines types used by a_orders.fdn and m_policies.fdn)
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
		// while n > 0 is on line 8, col 3
		if d.Code == "FDN0210" && d.Span.Line == 8 && d.Span.Column == 3 {
			foundWhile = true
		}
		// match status is on line 12, col 3
		if d.Code == "FDN0212" && d.Span.Line == 12 && d.Span.Column == 3 {
			foundNonExhaustive = true
		}
		// BadControlFlow(status, n - 1) is on line 18, col 17
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

func TestIR_CanonicalJSONRoundTrip_IngestsAndBuildsIdenticallyToFDN(t *testing.T) {
	srcDir := t.TempDir()

	blueprint := `
enum TicketStatus {
  open
  resolved
}

value OpenTicketInput {
  subject: string
}

entity SupportTicket {
  id: uuid @pk
  subject: string
  status: TicketStatus = .open
}

event TicketOpened {
  ticket_id: uuid
  subject: string
}

action OpenTicket(input: OpenTicketInput) -> SupportTicket
  @http(method: POST, path: "/v1/tickets")
{
  let ticket = create SupportTicket {
    subject: input.subject,
    status: .open
  }
  emit TicketOpened {
    ticket_id: ticket.id,
    subject: ticket.subject
  }
  return ticket
}

scenario "OpenTicket creates open ticket and emits event" {
  when OpenTicket(OpenTicketInput {
    subject: "Login issue"
  })
  expect result.subject == "Login issue"
  expect result.status == .open
  expect emitted TicketOpened {
    subject: "Login issue"
  }
}
`
	if err := os.WriteFile(filepath.Join(srcDir, "support.fdn"), []byte(blueprint), 0644); err != nil {
		t.Fatalf("write support.fdn: %v", err)
	}

	// 1. Export canonical SemanticIR JSON from .fdn workspace
	irBytes1, diags := compiler.IR(srcDir)
	if len(diags) != 0 {
		t.Fatalf("expected compiler.IR to succeed, got: %+v", diags)
	}

	// 2. Write the exported JSON into a new workspace as support.fdn.json (no .fdn source files)
	jsonDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(jsonDir, "support.fdn.json"), irBytes1, 0644); err != nil {
		t.Fatalf("write support.fdn.json: %v", err)
	}

	// 3. Re-export IR from the JSON-ingested workspace and verify byte-for-byte determinism
	irBytes2, diags := compiler.IR(jsonDir)
	if len(diags) != 0 {
		t.Fatalf("expected compiler.IR on .fdn.json workspace to succeed, got: %+v", diags)
	}
	if string(irBytes1) != string(irBytes2) {
		t.Fatalf("expected round-trip SemanticIR JSON to be byte-for-byte identical:\ngot:\n%s\nwant:\n%s", string(irBytes2), string(irBytes1))
	}

	// 4. Build and run generated tests directly from the ingested SemanticIR JSON workspace
	buildProfile := `
project:
  name: supportdesk
  module: example.com/supportdesk

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
	if err := os.WriteFile(filepath.Join(jsonDir, "foundation.build.yaml"), []byte(buildProfile), 0644); err != nil {
		t.Fatalf("write foundation.build.yaml: %v", err)
	}

	if _, buildDiags := compiler.Build(jsonDir, compiler.BuildOptions{}); len(buildDiags) != 0 {
		t.Fatalf("expected compiler.Build on .fdn.json workspace to succeed, got: %+v", buildDiags)
	}
	if testDiags := compiler.Test(jsonDir); len(testDiags) != 0 {
		t.Fatalf("expected generated scenarios from .fdn.json build to pass, got: %+v", testDiags)
	}
}




