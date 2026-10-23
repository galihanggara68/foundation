# Foundation (`.fdn`)

**Foundation** is a deterministic, rule-based, multi-target architecture and application compiler written in Go. It translates declarative domain **Blueprints** (`.fdn`) and a project **Build Profile** (`foundation.build.yaml`) into complete, production-ready backend applications, PostgreSQL schemas and sequential migrations, **Slot** interfaces and starter stubs, **SDK Clients**, and executable native test suites—with **zero runtime LLM dependency**.

---

## Table of Contents

1. [Why Foundation? (The 90/10 Rule)](#why-foundation-the-9010-rule)
2. [Three-Stage Compiler Pipeline](#three-stage-compiler-pipeline)
3. [Quick Start Guide](#quick-start-guide)
4. [Complete `.fdn` Blueprint Example](#complete-fdn-blueprint-example)
5. [Detailed `.fdn` Blueprint Language Reference](#detailed-fdn-blueprint-language-reference)
   - [Scalar & Collection Types](#1-scalar--collection-types)
   - [Top-Level Declarations (`enum`, `value`, `entity`, `error`, `event`, `policy`, `slot`, `action`, `scenario`)](#2-top-level-declarations)
   - [Action Pipeline Statements & Expressions](#3-action-pipeline-statements--expressions)
   - [Collection Combinators](#4-collection-combinators)
   - [Field, Entity, Slot, & Action Traits (`@`)](#5-field-entity-slot--action-traits-)
   - [Declarative Scenarios & Slot Test Doubles](#6-declarative-scenarios--slot-test-doubles)
6. [Detailed Build Profile Configuration (`foundation.build.yaml`)](#detailed-build-profile-configuration-foundationbuildyaml)
7. [Filesystem Hygiene, Slot Stubs, & Schema Evolution](#filesystem-hygiene-slot-stubs--schema-evolution)
8. [CLI Commands & Diagnostics Reference](#cli-commands--diagnostics-reference)
9. [Architecture Decision Records & Project Standards](#architecture-decision-records--project-standards)

---

## Why Foundation? (The 90/10 Rule)

Relying continuously on LLMs to generate application boilerplate introduces high variance, hallucinated security flaws, context omission, and review fatigue. Conversely, simple CRUD scaffolders leave 80%+ of business rules, authorization policies, architectural wiring, and testing to manual maintenance.

Foundation solves the **expressiveness vs. determinism** trade-off via the **90/10 Rule** ([ADR 0001](./docs/adr/0001-action-pipelines-and-slot-interfaces.md)):

- **The 90% Generated Core (`Action Pipelines`)**: Domain entities, values, authorization policies, precondition guards, relational queries/mutations, conditional branches, bounded `for` loops, collection combinators, tactical operational traits (`@transactional`, `@idempotent`, `@cache`, `@retry`, `@outbox`), and HTTP/event transport adapters are compiled deterministically.
- **The 10% Handwritten Custom Boundary (`Slots`)**: Complex custom algorithms or third-party network integrations are declared as strongly-typed `slot` contracts in `.fdn`. The compiler emits:
  1. A native **Slot Interface** (`internal/ports/<slot>_slot.go`) wired into dependency injection.
  2. A **create-if-missing Slot Stub** (`internal/slots/<slot>_stub.go`) excluded from `.foundation-manifest.json` so the generated project compiles immediately on the first build and your handwritten implementation is **never overwritten** on rebuilds.
  3. An automated **Contract Test** suite (`contracts_test.go`) verifying your handwritten implementation against the Blueprint's declared input/output invariants and `throws` error contracts.
- **Compile-Time Architectural Guardrails ([ADR 0004](./docs/adr/0004-static-architectural-invariants.md))**:
  - Rejects unbounded `while` loops (`FDN0210`) and recursive action calls (`FDN0211`) to guarantee pipeline termination.
  - Enforces exhaustive `match` statements over `enum` variants (`FDN0212`).
  - Forbids calling `@external` **Slot** methods inside `@transactional` actions (`FDN0213`) so database locks are never held across external network I/O.
  - Rejects N+1 `query`, `create`, `update`, or `delete` operations inside bounded `for` loops (`FDN0214`).

---

## Three-Stage Compiler Pipeline

Foundation strictly separates domain semantics from structural architecture patterns and target code generation ([ADR 0002](./docs/adr/0002-three-stage-architecture-lowering.md)):

```
  .fdn Blueprints ──► [Stage 1: Frontend] ──► Validated Semantic IR (.fdn.json)
                            │
                            ▼ (+ foundation.build.yaml & foundation.lock.json)
                      [Stage 2: Middle-End] ──► Architecture IR + Storage Plan
                            │
                            ▼
                      [Stage 3: Backend Emitters & Manifest Reconciler]
                            ├──► Go Backend (Hexagonal / Layered) + Chi + PostgreSQL + Tests
                            ├──► TypeScript Backend (Clean Architecture) + Express + PG + Tests
                            ├──► PostgreSQL DDL (schema.sql & migrations/NNNN_*.sql)
                            └──► Strongly-Typed SDK Clients (Go & TypeScript)
```

---

## Quick Start Guide

### 1. Build & Install the `foundation` CLI

Requires **Go 1.23+**:

```bash
git clone https://github.com/galihanggara68/foundation.git
cd foundation
go build -o bin/foundation ./cmd/foundation
```

### 2. Create a Workspace

Create a new directory for your domain project with a `foundation.build.yaml` **Build Profile** and a `.fdn` **Blueprint**:

```bash
mkdir -p my-commerce-app
cd my-commerce-app
```

Create `foundation.build.yaml`:

```yaml
project:
  name: commerce
  module: example.com/commerce

storage:
  dialect: postgres
  mode: both

targets:
  - name: backend
    kind: backend
    language: go
    architecture: hexagonal
    http: chi
    out_dir: ./dist/backend
```

Create `orders.fdn`:

```fdn
enum OrderStatus {
  pending
  confirmed
  cancelled
}

value PlaceOrderInput {
  request_id: string
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
  product_id: uuid @fk(Product.id)
  customer_id: uuid @index
  total: decimal(10, 2)
  status: OrderStatus = .pending
  created_at: timestamp @auto_now_add
}

error ProductNotFound {
  kind: not_found
  message: "Requested product does not exist"
}

error InsufficientStock {
  kind: conflict
  message: "Requested quantity exceeds available product stock"
}

error UnauthorizedCustomer {
  kind: forbidden
  message: "Only verified customers may place orders"
}

event OrderPlaced {
  order_id: uuid
  customer_id: uuid
  total: decimal(10, 2)
}

policy CanPlaceOrder(actor: Actor) {
  actor.role == "customer" && actor.is_verified == true
}

action PlaceOrder(input: PlaceOrderInput) -> Order
  @http(method: POST, path: "/v1/orders")
  @idempotent(key: input.request_id)
  @transactional
  @outbox
{
  require CanPlaceOrder(actor) else UnauthorizedCustomer

  let product = query one Product where id == input.product_id else ProductNotFound
  guard product.stock >= input.quantity else InsufficientStock

  update Product where id == product.id set {
    stock: product.stock - input.quantity
  }

  let order = create Order {
    product_id: product.id,
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
    request_id: "req-1",
    product_id: "00000000-0000-0000-0000-000000000001",
    quantity: 2
  })
  expect result.status == .confirmed
  expect result.total == 50.00
  expect stored Product where id == "00000000-0000-0000-0000-000000000001" {
    stock: 8
  }
  expect emitted OrderPlaced {
    customer_id: "00000000-0000-0000-0000-000000000099",
    total: 50.00
  }
}
```

### 3. Check, Build, and Test

```bash
# 1. Validate syntax, static types, and architectural invariants
foundation check --dir .

# 2. Compile Blueprints into target architecture, SQL schema/migrations, and tests
foundation build --dir .

# 3. Execute all generated Scenario and Contract Test suites
foundation test --dir .
```

---

## Complete `.fdn` Blueprint Example

You can split your domain across multiple `.fdn` files in the same workspace (e.g., `domain.fdn`, `policies.fdn`, `shipping_slots.fdn`, `actions.fdn`, `scenarios.fdn`). Foundation resolves all symbols across files deterministically.

```fdn
enum FulfillmentTier {
  standard
  express
  priority
}

value Address {
  street: string
  city: string
  country: string
  postal_code: string
}

value LineItem {
  sku: string
  price: int
  quantity: int
  in_stock: bool
}

value ShippingQuote {
  carrier: string
  fee: int
}

value QuoteRequest {
  tier: FulfillmentTier
  address: Address
  items: []LineItem
}

entity ShipmentQuoteLog {
  id: uuid @pk
  carrier: string @index
  quoted_fee: int
  final_total: int
  created_at: timestamp @auto_now_add
}

error EmptyCart {
  kind: validation
  message: "Cart must contain at least one active item"
}

error ShippingUnavailable {
  kind: conflict
  message: "No carrier available for the destination address"
}

slot CarrierGateway {
  @external
  FetchLiveRate(address: Address, subtotal: int) -> ShippingQuote throws ShippingUnavailable
}

action CalculateCheckoutQuote(input: QuoteRequest) -> ShipmentQuoteLog
  @http(method: POST, path: "/v1/quotes")
  @retry(attempts: 3, backoff: 10)
{
  let valid_items = input.items.filter(item => item.quantity > 0)
  guard valid_items.count() > 0 else EmptyCart

  let subtotal = valid_items.sum(item => item.price * item.quantity)

  // Invoke the 10% custom Slot boundary
  let live_quote = slot CarrierGateway.FetchLiveRate(input.address, subtotal)

  let tier_surcharge = 0
  match input.tier {
    .standard => {
      let tier_surcharge = 0
    }
    .express => {
      let tier_surcharge = 10
    }
    .priority => {
      let tier_surcharge = 25
    }
  }

  let bulk_discount = 0
  for item in valid_items {
    if item.quantity >= 10 {
      let bulk_discount = bulk_discount + 5
    } else {
      let bulk_discount = bulk_discount + 0
    }
  }

  let final_total = subtotal + live_quote.fee + tier_surcharge - bulk_discount

  let log = create ShipmentQuoteLog {
    carrier: live_quote.carrier,
    quoted_fee: live_quote.fee,
    final_total: final_total
  }

  return log
}

scenario "CalculateCheckoutQuote combines combinators, match, loop, and stubbed slot" {
  given slot CarrierGateway.FetchLiveRate(Address {
    street: "123 Main St",
    city: "Hanoi",
    country: "VN",
    postal_code: "100000"
  }, 120) returns ShippingQuote {
    carrier: "VNExpress",
    fee: 15
  }
  when CalculateCheckoutQuote(QuoteRequest {
    tier: .express,
    address: Address {
      street: "123 Main St",
      city: "Hanoi",
      country: "VN",
      postal_code: "100000"
    },
    items: [
      LineItem { sku: "ITEM-1", price: 10, quantity: 2, in_stock: true },
      LineItem { sku: "ITEM-2", price: 10, quantity: 10, in_stock: true },
      LineItem { sku: "ITEM-0", price: 99, quantity: 0, in_stock: false }
    ]
  })
  expect result.carrier == "VNExpress"
  expect result.quoted_fee == 15
  expect result.final_total == 140
}

scenario "CalculateCheckoutQuote propagates ShippingUnavailable when slot throws" {
  given slot CarrierGateway.FetchLiveRate throws ShippingUnavailable
  when CalculateCheckoutQuote(QuoteRequest {
    tier: .standard,
    address: Address {
      street: "Remote Base",
      city: "McMurdo",
      country: "AQ",
      postal_code: "00000"
    },
    items: [
      LineItem { sku: "ITEM-1", price: 50, quantity: 1, in_stock: true }
    ]
  })
  expect error ShippingUnavailable
}
```

---

## Detailed `.fdn` Blueprint Language Reference

### 1. Scalar & Collection Types

| `.fdn` Type | Description | Go Mapping | PostgreSQL DDL Mapping |
| :--- | :--- | :--- | :--- |
| `uuid` | Universally unique identifier | `string` | `UUID` |
| `string` | UTF-8 text string | `string` | `TEXT` |
| `int` | Signed integer | `int` | `INTEGER` |
| `decimal` / `decimal(P, S)` | Fixed-precision decimal number | `float64` | `NUMERIC(P, S)` (default `NUMERIC(10, 2)`) |
| `bool` | Boolean (`true` / `false`) | `bool` | `BOOLEAN` |
| `timestamp` | UTC timestamp | `string` | `TIMESTAMPTZ` |
| `<Type>?` | Nullable / optional field |Scalar / pointer | Nullable column (omits `NOT NULL`) |
| `[]<Type>` | Homogeneous list of `<Type>` | `[]<GoType>` | `JSONB` |
| `Actor` | Built-in caller context (`id: uuid`, `role: string`, `is_verified: bool`, `tenant_id: uuid`) | `domain.Actor` | — |

---

### 2. Top-Level Declarations

A `.fdn` Blueprint supports **9 top-level declarations**:

1. **`enum <Name> { <variant>... }`** — Exhaustively checked domain enumerations. Variants are referenced with a leading dot (e.g., `.pending`, `.confirmed`).
2. **`value <Name> { <field>: <Type>... }`** — Immutable, identity-less data structures for inputs, outputs, and embedded payloads.
3. **`entity <Name> { <field>: <Type> [@traits] [= default]... }`** — Persistent domain objects lowered into database tables and repository ports.
4. **`error <Name> { kind: <Category> message: "<Message>" }`** — Strongly-typed domain errors automatically mapped to transport status codes:
   - `not_found` → HTTP `404 Not Found`
   - `validation` → HTTP `400 Bad Request`
   - `conflict` → HTTP `409 Conflict`
   - `unauthorized` → HTTP `401 Unauthorized`
   - `forbidden` → HTTP `403 Forbidden`
   - `internal` → HTTP `500 Internal Server Error`
5. **`event <Name> { <field>: <Type>... }`** — Typed domain event payloads published via `emit`.
6. **`policy <Name>(<param>: <Type>, ...) { <bool_expr> }`** — Side-effect-free boolean authorization rules evaluated via `require`.
7. **`slot <Name> { [@external] <Method>(<param>: <Type>, ...) [-> <ReturnType>] [throws <Error>, ...] }`** — Strongly-typed interface boundary for the 10% handwritten custom code.
8. **`action <Name>(<param>: <Type>, ...) [-> <ReturnType>] [@traits...] { <steps...> }`** — Guaranteed-terminating domain pipeline.
9. **`scenario "<Description>" { given ... when ... expect ... }`** — Declarative domain specification compiled into native unit tests.

---

### 3. Action Pipeline Statements & Expressions

Inside an `action` block, statements execute sequentially:

- **Authorization Policy Check (`require`)**:
  ```fdn
  require CanPlaceOrder(actor) else UnauthorizedCustomer
  ```
- **Precondition Guard (`guard`)**:
  ```fdn
  guard product.stock >= input.quantity else InsufficientStock
  ```
- **Immutable / Rebindable Variable (`let`)**:
  ```fdn
  let total = product.price * input.quantity
  ```
- **Relational Query (`query one`)**:
  ```fdn
  let product = query one Product where id == input.product_id else ProductNotFound
  ```
- **Entity Creation (`create`)**:
  ```fdn
  let order = create Order {
    customer_id: actor.id,
    total: total,
    status: .confirmed
  }
  ```
- **Entity Mutation (`update`)**:
  ```fdn
  update Product where id == product.id set {
    stock: product.stock - input.quantity
  }
  ```
- **Entity Deletion (`delete`)**:
  ```fdn
  delete Coupon where code == input.code
  ```
- **Slot Invocation (`slot`)**:
  ```fdn
  let quote = slot ShippingCalculator.Quote(input.address, subtotal)
  // Or with error remapping:
  let quote = slot ShippingCalculator.Quote(input.address, subtotal) else ShippingFailed
  ```
- **Conditional Branching (`if / else if / else`)**:
  ```fdn
  if item.quantity >= 10 {
    let discount = 5
  } else {
    let discount = 0
  }
  ```
- **Exhaustive Pattern Matching (`match`)**:
  ```fdn
  match order.status {
    .pending => { ... }
    .confirmed => { ... }
    .cancelled => { ... }
  }
  ```
- **Bounded Iteration (`for <var> in <list_expr>`)**:
  ```fdn
  for item in valid_items {
    ...
  }
  ```
  *(Note: Unbounded `while` loops and recursive action calls are rejected at compile time.)*
- **Domain Event Emission (`emit`)**:
  ```fdn
  emit OrderPlaced {
    order_id: order.id,
    customer_id: order.customer_id,
    total: order.total
  }
  ```
- **Return (`return`)**:
  ```fdn
  return order
  ```

---

### 4. Collection Combinators

Pure collection combinators can be chained on any list expression (`[]<Type>`):

| Combinator | Example | Return Type |
| :--- | :--- | :--- |
| `.filter(x => <bool_expr>)` | `input.items.filter(item => item.quantity > 0)` | `[]<ElemType>` |
| `.map(x => <expr>)` | `valid_items.map(item => item.price * item.quantity)` | `[]<MappedType>` |
| `.sum()` / `.sum(x => <num_expr>)` | `valid_items.sum(item => item.price * item.quantity)` | `int` or `decimal` |
| `.count()` / `.count(x => <bool_expr>)` | `valid_items.count()` | `int` |
| `.any(x => <bool_expr>)` | `valid_items.any(item => item.quantity >= 5)` | `bool` |
| `.all(x => <bool_expr>)` | `valid_items.all(item => item.in_stock == true)` | `bool` |

---

### 5. Field, Entity, Slot, & Action Traits (`@`)

#### Entity & Field Storage Traits
| Trait | Target | Effect |
| :--- | :--- | :--- |
| `@pk` | Entity Field | Marks column as `PRIMARY KEY` |
| `@unique` | Entity Field | Adds `UNIQUE` constraint to column |
| `@index` | Entity Field | Emits `CREATE INDEX idx_<table>_<col> ON <table> (<col>);` |
| `@fk(<Entity>.<field>)` | Entity Field | Emits `REFERENCES <table>(<field>)` foreign key constraint |
| `@auto_now_add` | `timestamp` Field | Sets column default to `DEFAULT NOW()` |
| `@renamed_from("<old_name>")` | Entity or Field | Emits safe `ALTER TABLE ... RENAME` migration instead of destructive drop |

#### Slot Method Traits
| Trait | Target | Effect |
| :--- | :--- | :--- |
| `@external` | Slot Method | Marks method as performing external network/third-party I/O. Forbidden inside `@transactional` actions (`FDN0213`). |

#### Tactical Operational Action Traits
| Trait | Example | Runtime Behavior Woven by Compiler |
| :--- | :--- | :--- |
| `@transactional` | `@transactional` | Executes the Action Pipeline inside an atomic transaction (`WithinTransaction`), rolling back all state mutations on error. |
| `@idempotent(key: ...)` | `@idempotent(key: input.request_id)` | Deduplicates repeated executions with the same idempotency key and returns the cached response. |
| `@cache(ttl: <seconds>)` | `@cache(ttl: 60)` | Caches successful Action responses in memory for `<ttl>` seconds keyed by input arguments. |
| `@retry(attempts: <N>, backoff: <ms>)` | `@retry(attempts: 3, backoff: 10)` | Retries failed pipeline executions up to `attempts` times with `backoff` milliseconds between attempts. |
| `@outbox` | `@outbox` | Buffers `emit` events in a transactional outbox queue and flushes them only after the pipeline succeeds. |

#### Transport Exposure Action Traits
By default, public Actions are exposed via `POST /<action_name>`. You can override transport routing with:
- `@http(method: GET|POST|PUT|DELETE, path: "/v1/items/{id}")` — Binds HTTP method, route path, and `{param}` URL path parameters.
- `@internal` — Excludes the Action from public HTTP routers and external SDK Clients.
- `@event_handler(<EventName>)` — Registers the Action as an internal domain event consumer (excluded from HTTP routes).
- `@cron("<schedule>")` — Registers the Action as a scheduled background job (excluded from HTTP routes).

---

### 6. Declarative Scenarios & Slot Test Doubles

Every `scenario` block compiles into a native executable test (`scenarios_test.go`):

```fdn
scenario "Descriptive scenario name" {
  // 1. Seed initial entity state in the in-memory repository
  given Product {
    id: "00000000-0000-0000-0000-000000000001",
    sku: "SKU-1",
    price: 25.00,
    stock: 5
  }

  // 2. Seed caller Actor context
  given actor {
    id: "00000000-0000-0000-0000-000000000099",
    role: "customer",
    is_verified: true
  }

  // 3. Stub Slot method return value (for specific arguments or any arguments)
  given slot ShippingCalculator.Quote(Address { city: "Hanoi", country: "VN" }, 50) returns ShippingQuote {
    fee: 10,
    carrier: "FastShip"
  }
  // Or stub a Slot method to throw a declared error:
  // given slot ShippingCalculator.Quote throws ShippingUnavailable

  // 4. Execute the Action under test
  when PlaceOrder(PlaceOrderInput { ... })

  // 5. Assert on returned result, raised error, emitted events, or persisted state
  expect result.status == .confirmed
  expect emitted OrderPlaced { total: 50.00 }
  expect stored Product where id == "00000000-0000-0000-0000-000000000001" { stock: 3 }
  expect not_stored Coupon where code == "ONE-TIME"
  // Or expect a domain error:
  // expect error InsufficientStock
}
```

---

## Detailed Build Profile Configuration (`foundation.build.yaml`)

The **Build Profile** (`foundation.build.yaml`) lives at the root of your workspace and controls how `.fdn` Blueprints are lowered and emitted—keeping `.fdn` files 100% agnostic of target languages and folder structures.

```yaml
# 1. Project Metadata
project:
  name: ordering                   # Logical project identifier
  module: example.com/ordering     # Go module path / package namespace

# 2. Database Schema & Migration Strategy
storage:
  dialect: postgres                # Target SQL dialect (postgres)
  mode: both                       # "declarative" | "migrations" | "both"

# 3. Compilation Targets
targets:
  - name: backend
    kind: backend                  # "backend" | "sdk"
    language: go                   # "go" | "typescript"
    architecture: hexagonal        # "hexagonal" | "layered" (Go) | "clean" (TypeScript)
    http: chi                      # "chi" (Go) | "express" (TypeScript)
    out_dir: ./dist/backend        # Output root directory for this target
```

### Storage Modes (`storage.mode`)
- `declarative`: Emits a desired-state `schema.sql` file containing all `CREATE TABLE` and `CREATE INDEX` statements, plus `foundation.lock.json`.
- `migrations`: Diffs the current **Semantic IR** against `foundation.lock.json` and emits sequential `migrations/NNNN_*.up.sql` and `migrations/NNNN_*.down.sql` files.
- `both`: Emits both `schema.sql` and `migrations/NNNN_*.up.sql` / `.down.sql`.

### Go Hexagonal Target Layout (`architecture: hexagonal`)
When compiling a Go `hexagonal` target, Foundation emits:
```
dist/backend/
├── .foundation-manifest.json                 # SHA-256 registry of generated files
├── go.mod / go.sum
├── schema.sql                                # Declarative PostgreSQL DDL
├── migrations/
│   ├── 0001_initial.up.sql
│   └── 0001_initial.down.sql
├── internal/
│   ├── domain/                               # Zero-dependency Entities, Values, Enums, Errors, Events, Policies
│   ├── ports/                                # Repository interfaces, Slot interfaces, EventPublisher, TransactionManager
│   ├── usecases/                             # Action Pipeline interactors + tactical Trait decorators
│   ├── slots/                                # Create-if-missing Slot Stubs (excluded from manifest; safe to edit!)
│   ├── adapters/
│   │   ├── http/router.go                    # Chi HTTP handlers, Actor extraction, & domain error status mapping
│   │   ├── memory/                           # Concurrency-safe in-memory repositories, tx snapshots, & Slot fakes
│   │   └── postgres/                         # Production database/sql PostgreSQL repositories & TransactionManager
│   └── bootstrap/container.go                # NewTestContainer() & NewPostgresContainer(*sql.DB) DI wiring
├── scenarios_test.go                         # Compiled Scenario test suite
└── contracts_test.go                         # Automated Slot Contract Test suite
```

---

## Filesystem Hygiene, Slot Stubs, & Schema Evolution

1. **Manifest-Tracked Root Scaffolding (`.foundation-manifest.json`)**:
   - Every compiler-owned file is recorded with its SHA-256 hash in `.foundation-manifest.json` ([ADR 0005](./docs/adr/0005-manifest-tracked-root-scaffolding.md)).
   - **Stale File Pruning**: If you remove or rename an `entity` or `action` in `.fdn`, `foundation build` automatically deletes its stale generated files on the next build.
   - **Conflict Protection**: If a human accidentally modifies a manifest-tracked file, `foundation build` aborts with `FDN0402` instead of silently overwriting the edit (pass `--force` to overwrite).
2. **Create-If-Missing Slot Stubs (`internal/slots/*_stub.go`)**:
   - Emitted only if the file does not yet exist on disk, and **never** added to `.foundation-manifest.json`.
   - You write your 10% custom logic directly in `internal/slots/<slot>_stub.go` and rebuild as many times as you want without conflicts or data loss.
3. **Safe Schema Diffing & Evolution (`foundation.lock.json`)**:
   - Adding `@renamed_from("old_col")` to a field emits `ALTER TABLE <table> RENAME COLUMN old_col TO new_col;`.
   - Removing an entity or column without a `@renamed_from` hint is blocked with error diagnostic `FDN0410` unless `--force` is explicitly provided.

---

## CLI Commands & Diagnostics Reference

```
Usage: foundation <check|build|ir|test> [--dir <path>] [--force]
```

| Command | Description |
| :--- | :--- |
| `foundation check [--dir <path>]` | Parses `.fdn` and `.fdn.json` files, runs symbol resolution, type checking, termination checks, and static invariant analysis. Exits non-zero on error diagnostics. |
| `foundation build [--dir <path>] [--force]` | Runs `check`, lowers the **Semantic IR** into **Architecture IR** and **Storage Plan**, reconciles `.foundation-manifest.json`, writes `foundation.lock.json`, and emits all target files. |
| `foundation ir [--dir <path>]` | Outputs the canonical, deterministic JSON representation of the validated **Semantic IR** (`.fdn.json` compatible). |
| `foundation test [--dir <path>]` | Executes the native test suites (`scenarios_test.go`, `contracts_test.go`) across all configured targets. |

### Compiler Diagnostic Codes

| Code Range | Stage | Description |
| :--- | :--- | :--- |
| `FDN0001`–`FDN0006` | Workspace & Profile | Missing `.fdn` files, unreadable `foundation.build.yaml`, or malformed `.fdn.json`. |
| `FDN0100`–`FDN0107` | Lexer & Parser | Invalid characters, unexpected tokens, or malformed declarations/steps/scenarios. |
| `FDN0200`–`FDN0209` | Type & Symbol Checker | Duplicate declarations, unknown types/entities/fields/events/policies/slots, or type mismatches. |
| `FDN0210`–`FDN0212` | Termination & Exhaustiveness | Unbounded `while` loop (`FDN0210`), recursive action call (`FDN0211`), or non-exhaustive `match` (`FDN0212`). |
| `FDN0213`–`FDN0214` | Architectural Invariants | `@external` slot call inside `@transactional` action (`FDN0213`) or N+1 query/mutation inside `for` loop (`FDN0214`). |
| `FDN0300` | Middle-End Lowering | Unsupported target language/architecture combination in `foundation.build.yaml`. |
| `FDN0400`–`FDN0402` | Manifest Reconciler | Filesystem write failure, corrupt `.foundation-manifest.json`, or manual edit conflict (`FDN0402`). |
| `FDN0410`–`FDN0411` | Schema Evolution | Unhinted destructive table/column drop (`FDN0410`) or `foundation.lock.json` write error. |
| `FDN0500` | Emitter | Target source formatting error. |
| `FDN0600` | Test Runner | Generated target test suite failure. |

---

## Architecture Decision Records & Project Standards

- [GLOSSARY.md](./GLOSSARY.md) — Canonical ubiquitous domain language.
- [SPEC.md](./SPEC.md) — Formal technical specification and testing boundaries.
- [CODING_STANDARDS.md](./CODING_STANDARDS.md) — Deep Module architecture, SOLID principles, and stage-aligned package layout (`pkg/compiler/internal/{ir,frontend,validate,arch,emit}`).
- [GRILL-SUMMARY.md](./GRILL-SUMMARY.md) — Complete design trade-offs and architectural interview summary.
- **Architectural Decision Records (`docs/adr/`)**:
  - [ADR 0001: Constrained Action Pipelines with Generated Slot Interfaces](./docs/adr/0001-action-pipelines-and-slot-interfaces.md)
  - [ADR 0002: Three-Stage Compilation with Architecture Lowering](./docs/adr/0002-three-stage-architecture-lowering.md)
  - [ADR 0003: Purpose-Built `.fdn` Grammar over YAML/JSON](./docs/adr/0003-purpose-built-fdn-grammar.md)
  - [ADR 0004: Static Architectural Invariant Enforcement](./docs/adr/0004-static-architectural-invariants.md)
  - [ADR 0005: Manifest-Tracked Root Scaffolding over Isolated `gen/` Subtrees](./docs/adr/0005-manifest-tracked-root-scaffolding.md)
