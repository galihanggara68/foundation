---
status: ready-for-agent
---

# Foundation: Deterministic Multi-Target Architecture and Application Compiler

## Problem Statement

Relying continuously on Large Language Models (LLMs) to generate day-to-day application code introduces high variance, hallucinated security flaws, context omission as codebases grow, operational token costs, and cognitive fatigue from constantly reviewing unpredictable boilerplate. Conversely, traditional general-purpose transpilers fail when attempting to translate arbitrary imperative code across languages with incompatible memory and concurrency models (such as Go, TypeScript, and Rust), while simple schema scaffolding tools only generate static data models or CRUD stubs and leave 80%+ of business logic, authorization, architectural layering, and testing to manual maintenance. Additionally, when teams want to evolve their application's structural pattern (for example, migrating from a simple Layered CRUD architecture to Hexagonal or Clean Architecture), they must manually rewrite their entire codebase and unit test suite.

## Solution

Build **Foundation**, a deterministic, rule-based compiler written in Go that translates declarative domain **Blueprints** (`.fdn`) and a project **Build Profile** (`foundation.build.yaml`) into complete, production-ready backend applications, database schemas/migrations, **SDK Clients**, and executable native test suites across multiple target languages and architectural topologies—with zero runtime LLM dependency.

Foundation solves the expressiveness-versus-determinism trade-off by combining:
1. **Declarative Domain Structure**: **Entities**, **Values**, typed **Errors**, domain **Events**, and reusable authorization **Policies**.
2. **Constrained Action Pipelines (The 90% Logic)**: Guaranteed-terminating sequences of domain operations (precondition guards, policy checks, relational queries, mutations, conditional branches, bounded `for` iterations, and event emissions) with static compile-time enforcement of architectural invariants (forbidding N+1 queries in loops and external I/O inside database transactions).
3. **Typed Slots (The 10% Custom Logic Boundary)**: Strongly-typed native interfaces/traits emitted for complex custom algorithms or third-party integrations, paired with create-if-missing **Slot Stubs** and automated **Contract Tests** so the generated codebase compiles and passes tests out-of-the-box while protecting handwritten code across regenerations.
4. **Three-Stage Compilation Pipeline**: Separating domain validation (**Semantic IR**) from structural pattern lowering (**Architecture IR**) and target code formatting (**Emitters**), enabling the exact same **Blueprint** to be compiled into **Go (Hexagonal or Layered + Chi + PostgreSQL)**, **TypeScript (Clean Architecture + Express + PostgreSQL)**, and strongly-typed **SDK Clients** simply by updating the **Build Profile**.

## User Stories

### Blueprint Authoring & Domain Modeling
1. As a domain author, I want to declare persistent **Entities** with scalar types (`uuid`, `string`, `int`, `decimal`, `bool`, `timestamp`), optionality (`?`), default values, primary keys, unique constraints, indexes, and foreign key relationships in a `.fdn` **Blueprint**, so that storage schemas and domain models are derived from a single source of truth.
2. As a domain author, I want to declare immutable **Values** and enums in a **Blueprint**, so that action inputs, outputs, and embedded structures are strongly typed across all target languages.
3. As a domain author, I want to declare typed domain **Errors** with semantic error categories (such as `not_found`, `validation`, `conflict`, `unauthorized`, `forbidden`, `internal`), so that **Emitters** automatically map domain errors to appropriate transport statuses (HTTP status codes) and client error types.
4. As a domain author, I want to declare domain **Events** with typed payloads, so that **Action Pipelines** can emit events and downstream handlers or outbox publishers can consume them deterministically.
5. As a domain author, I want to declare reusable, side-effect-free **Policies** over the caller (`actor`), input, and loaded **Entities**, so that access control rules are centralized and verifiable.
6. As a domain author, I want to split large domains across multiple `.fdn` **Blueprint** files in a workspace, so that complex systems remain modular and navigable.
7. As a domain author, I want to annotate fields or entities with evolution hints (such as `@renamed_from("old_name")`), so that schema diffing generates safe `ALTER TABLE RENAME` migrations instead of destructive drop-and-add operations.

### Action Pipelines & Tactical Traits
8. As a domain author, I want to write **Action Pipelines** using `require` (policy enforcement), `guard` (input/state validation), `let` (immutable expression binding), `if / else` and `match` (branching), bounded `for item in collection` loops, `query` / `create` / `update` / `delete` (relational operations), `slot` calls, `emit` (event publication), and `return`, so that 90%+ of application behavior is expressed deterministically without target-language boilerplate.
9. As a domain author, I want to use pure collection combinators (`map`, `filter`, `sum`, `count`, `any`, `all`) inside **Action Pipelines**, so that data transformations are concise and compile cleanly into any target language.
10. As a domain author, I want to attach tactical **Traits** to Actions—including `@transactional`, `@idempotent(key: ...)`, `@cache(ttl: ...)`, `@retry(attempts: ..., backoff: ...)`, and `@outbox`—so that operational guarantees are woven into the generated execution pipeline without polluting domain logic.
11. As a domain author, I want public Actions to be automatically exposed via transport conventions defined in the **Build Profile**, while supporting override **Traits** like `@http(method: POST, path: "/v1/orders")`, `@event_handler(OrderPlaced)`, `@cron("0 * * * *")`, or `@internal`, so that routine endpoints require zero transport configuration while custom routing remains flexible.

### Slots, Stubs, and the 10% Custom Code Boundary
12. As a developer, I want to declare a **Slot** with typed method signatures and error contracts in a **Blueprint** (marking methods `@external` when they perform network/third-party I/O), so that custom algorithms or third-party integrations have a strict compile-time boundary.
13. As a developer, I want the compiler to emit a native interface or trait for every **Slot** (`interface` in Go, `interface` in TypeScript) and wire it into the generated dependency injection container, so that generated code calls my custom logic without knowing its implementation details.
14. As a developer, I want the compiler to generate a create-if-missing **Slot Stub** file on the first build (and exclude it from the **Generation Manifest**), so that a freshly compiled project builds and runs immediately without manual wiring and my subsequent handwritten implementation is never overwritten or deleted on re-compile.
15. As a developer, I want the compiler to generate an automated **Contract Test** suite for every **Slot**, so that my handwritten 10% implementation is automatically tested against the **Blueprint**'s input/output invariants and declared error semantics.

### Static Analysis & Compile-Time Guardrails
16. As a developer, I want the compiler to report exact file, line, and column diagnostics for syntax errors, undeclared identifiers, type mismatches, and non-exhaustive `match` expressions in `.fdn` files, so that I can fix specification mistakes immediately.
17. As a developer, I want the compiler to reject unbounded iteration or recursive action calls in **Action Pipelines**, so that every pipeline is guaranteed to terminate.
18. As a developer, I want the compiler's static analyzer to reject calling an `@external` **Slot** inside a `@transactional` pipeline or block, so that generated applications never hold database row locks across external network I/O or suffer unrecoverable partial-commit bugs.
19. As a developer, I want the compiler's static analyzer to flag `query`, `create`, `update`, or `delete` statements placed inside bounded `for` loops when bulk set operations can be used, so that N+1 query performance regressions are caught at compile time.

### Multi-Architecture Lowering & Build Profiles
20. As a system architect, I want to configure the target language, structural architecture pattern, HTTP router, and database strategy in a **Build Profile** (`foundation.build.yaml`) completely outside the `.fdn` **Blueprints**, so that the domain specification remains 100% architecture-agnostic.
21. As a system architect, I want to switch a project's **Build Profile** between `layered` and `hexagonal` (in Go) or `clean` (in TypeScript) and re-run `foundation build`, so that the compiler restructures all layers, ports, adapters, use cases, and wiring deterministically without changing a single line of `.fdn` code.
22. As a developer, I want the compiler to track every file it emits in a **Generation Manifest** (`.foundation-manifest.json`) with SHA-256 content hashes, so that stale generated files (such as renamed entities or removed actions) are deterministically pruned on rebuild while handwritten files and **Slot Stubs** remain untouched.
23. As a developer, I want the compiler to detect if a file tracked in the **Generation Manifest** was manually edited by a human and refuse to overwrite it unless `--force` is passed, so that accidental manual edits in generated files are never silently lost.

### Multi-Language Emitters & SDK Clients
24. As a backend engineer targeting Go, I want an **Emitter** that outputs idiomatic, `gofmt`-formatted Go code (using `chi` for HTTP routing, `pgx`/SQL repositories for PostgreSQL, structured domain error mapping, and dependency injection bootstrap wiring) in either `hexagonal` or `layered` architecture.
25. As a backend engineer targeting TypeScript, I want an **Emitter** that outputs idiomatic, strictly-typed TypeScript code (using `express` for HTTP routing, `pg` parameterized queries for PostgreSQL, and Clean Architecture layers: `domain`, `usecases`, `controllers`, `gateways`).
26. As a frontend or service consumer, I want an **Emitter** that outputs a strongly-typed **SDK Client** (in Go and TypeScript) containing all exposed Action methods, request/response **Value** types, and typed error discriminators, so that callers never write manual `fetch`/`http.Client` boilerplate or drift out of sync with the backend.
27. As a database administrator or developer, I want the PostgreSQL **Emitter** to output either declarative desired-state DDL (`schema.sql`) or sequential diff-based up/down SQL migrations (`migrations/NNNN_*.sql`) paired with a deterministic `foundation.lock.json` snapshot, configured via the **Build Profile**.
28. As a tooling integrator, I want the compiler to export and ingest the validated **Semantic IR** as canonical JSON, so that external visualization tools, linters, or future generators can inspect the domain model without re-parsing `.fdn` syntax.

### Scenarios & Deterministic Testing
29. As a domain author, I want to write declarative **Scenarios** (`scenario "name" { given ... when ... expect ... }`) in `.fdn` files that seed entity state, stub **Slot** responses, invoke an **Action**, and assert returned outputs, raised **Errors**, persisted state changes, and emitted **Events**, so that domain behavior is verified independent of target language or architecture pattern.
30. As a developer, I want the compiler to emit native executable test files (`*_test.go` in Go, `*.test.ts` in TypeScript) together with generated in-memory repository fakes and **Slot** test doubles from Blueprint **Scenarios**, so that running `go test ./...` or `npm test` verifies both the 90% generated pipelines and the 10% handwritten **Slots** in milliseconds without requiring a live database.

## Implementation Decisions

### 1. Compiler Pipeline Architecture (Three-Stage Design)
The compiler is structured as a pure, deterministic pipeline divided into three isolated stages:
- **Stage 1: Frontend (Lexer, Parser, Type Checker, & Invariant Analyzer)**
  - Ingests `.fdn` **Blueprint** source files from a workspace filesystem and produces a validated, immutable **Semantic IR**.
  - Performs lexical scanning, recursive-descent parsing with exact source span tracking (`file`, `line`, `column`), symbol resolution across files, static type checking of expressions and pipeline bindings, exhaustiveness checking, and architectural invariant verification (bounded termination, no `@external` **Slot** inside `@transactional`, and N+1 loop query detection).
  - Supports deterministic serialization of the **Semantic IR** to and from canonical JSON.
- **Stage 2: Middle-End (Architecture Lowering & Schema Diffing)**
  - Consumes the **Semantic IR** and the **Build Profile** (`foundation.build.yaml`) to produce an **Architecture IR** and a **Storage Plan**.
  - **Pattern Lowering Transformer**: Maps pure domain constructs into concrete architectural components (layers, modules/packages, ports/interfaces, use-case interactors, controllers/handlers, repository implementations, and DI bootstrap containers) according to the chosen pattern (`hexagonal`, `clean`, or `layered`).
  - **Schema Evolution Engine**: Compares the current **Semantic IR** entities against `foundation.lock.json` (when present) to produce both a full declarative SQL schema and sequential up/down migration steps, honoring `@renamed_from` traits and flagging unhinted destructive changes.
- **Stage 3: Backend (Target Emitters & Manifest Reconciler)**
  - **Emitters** translate the **Architecture IR** into formatted virtual file trees (`path -> bytes`) for each configured target in the **Build Profile**:
    - `go-backend` (`hexagonal` or `layered` topology, `chi` HTTP router, PostgreSQL repository adapters, in-memory fakes, native `testing` suites for **Scenarios** and **Contract Tests**).
    - `ts-backend` (`clean` architecture topology, `express` HTTP router, `pg` repository adapters, in-memory fakes, native test suites for **Scenarios** and **Contract Tests**).
    - `go-sdk` and `ts-sdk` (**SDK Clients** with typed request/response models and error discriminators).
    - `postgres-sql` (declarative `schema.sql` and/or numbered up/down SQL migrations).
  - **Manifest Reconciler**: Compares the emitted virtual file tree against `.foundation-manifest.json` on disk:
    - Verifies that previously generated files on disk still match their recorded SHA-256 hash (aborting with a conflict diagnostic if modified by hand unless forced).
    - Writes updated and newly generated files deterministically.
    - Deletes stale files present in the previous manifest but absent from the new emission.
    - Writes **Slot Stub** files (`create-if-missing` only) and never records them in `.foundation-manifest.json`.

### 2. `.fdn` Blueprint Grammar & Language Semantics
The `.fdn` grammar supports seven top-level declarations (`enum`, `value`, `entity`, `error`, `event`, `policy`, `slot`, `action`, and `scenario`):

```fdn
enum OrderStatus {
  pending
  confirmed
  fulfilled
  cancelled
}

value Address {
  street: string
  city: string
  country: string
  postal_code: string
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
  shipping_quote_id: string?
  created_at: timestamp @auto_now_add
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

slot ShippingCalculator {
  @external
  Quote(address: Address, subtotal: decimal(10, 2)) -> ShippingQuote throws ShippingUnavailable
}

action PlaceOrder(input: PlaceOrderInput) -> Order
  @http(method: POST, path: "/v1/orders")
  @idempotent(key: input.request_id)
  @transactional
{
  require CanPlaceOrder(actor) else UnauthorizedCustomer

  let product = query one Product where id == input.product_id else ProductNotFound
  guard product.stock >= input.quantity else InsufficientStock

  update Product where id == product.id set {
    stock: product.stock - input.quantity
  }

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

scenario "PlaceOrder succeeds when stock is sufficient" {
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
  expect emitted OrderPlaced {
    customer_id: "00000000-0000-0000-0000-000000000099",
    total: 50.00
  }
}
```

### 3. Build Profile Contract (`foundation.build.yaml`)
The **Build Profile** configures how Blueprints are lowered and emitted without modifying `.fdn` files:
- `project`: Name, module/package namespace, and blueprint glob paths.
- `storage`: Database dialect (`postgres`) and schema emission mode (`declarative`, `migrations`, or `both`).
- `targets`: List of compilation targets, each specifying:
  - `name`: Target identifier.
  - `kind`: `backend` or `sdk`.
  - `language`: `go` or `typescript`.
  - `architecture` (for `backend`): `hexagonal`, `layered` (Go), or `clean` (TypeScript).
  - `http` (for `backend`): `chi` (Go) or `express` (TypeScript).
  - `out_dir`: Relative output directory for this target's generated root and `.foundation-manifest.json`.

### 4. Architectural Topology Mappings in the Middle-End
- **Go `hexagonal` Topology**:
  - Domain Layer: Pure **Entity** structs, **Value** structs, **Error** definitions, **Event** payloads, and **Policy** evaluators with zero infrastructure imports.
  - Ports Layer: Driven repository interfaces (`OrderRepository`), event publisher interfaces, transaction manager interfaces, and **Slot** interfaces.
  - Application/UseCase Layer: Interactor structs executing the lowered **Action Pipelines** against Port interfaces and tactical **Trait** decorators (idempotency, retry, transaction boundaries).
  - Adapters Layer: Driving `chi` HTTP handlers (decoding requests, extracting `Actor`, invoking UseCases, mapping domain errors to HTTP status codes) and driven PostgreSQL/in-memory repository implementations.
  - Slots Layer: Create-if-missing **Slot Stub** implementations and generated **Contract Tests**.
  - Bootstrap Layer: Deterministic dependency injection container wiring ports, adapters, use cases, and slots.
- **Go `layered` Topology**:
  - Flattens Ports and UseCases into direct `handlers -> services -> repositories -> models` packages while preserving **Slot** interfaces and **Scenario** testability.
- **TypeScript `clean` Topology**:
  - Emits `entities` (pure domain models and policies), `usecases` (application business rules and port interfaces including **Slots**), `controllers` (HTTP presentation mapping), `gateways` (PostgreSQL and in-memory persistence adapters), `slots` (create-if-missing **Slot Stubs** and **Contract Tests**), and `main` (`express` router and dependency wiring).

### 5. CLI Interface & Compiler Driver API
The `foundation` binary exposes four deterministic commands backed by a single Go compiler driver interface:
- `foundation check`: Parses Blueprints, runs type checking and static invariant analysis, and exits non-zero if any error diagnostics exist.
- `foundation build`: Runs `check`, lowers the **Semantic IR** into **Architecture IR** for every target in `foundation.build.yaml`, reconciles `.foundation-manifest.json`, writes `foundation.lock.json`, and emits all source, SQL, and test files.
- `foundation ir`: Dumps the canonical JSON representation of the validated **Semantic IR**.
- `foundation test`: Compiles and executes generated native test suites (**Scenarios** and **Contract Tests**) across configured targets.

## Testing Decisions

### Philosophy: What Makes a Good Test
Tests must verify observable behavior at the highest boundary—the **Compiler Driver Seam**—rather than coupling to internal parser states, AST helper methods, or intermediate visitors. Every test provides `.fdn` **Blueprint** source strings and a **Build Profile** configuration as inputs, and asserts on:
1. Structured compiler diagnostics (error code, severity, file, line, column).
2. The emitted virtual or on-disk workspace files, `.foundation-manifest.json`, and `foundation.lock.json`.
3. Byte-for-byte determinism across repeated builds and target toolchain execution (`go test` / TypeScript compilation and test execution) of the generated code.

### Modules & Seams Under Test
1. **Frontend & Static Invariant Verification (via Compiler Driver `Check`)**:
   - Validating complete `.fdn` Blueprints with Entities, Values, Enums, Errors, Events, Policies, Slots, Action Pipelines, and Scenarios.
   - Rejecting syntax errors, undefined symbols, type mismatches in queries/expressions, and invalid Trait arguments with accurate source spans.
   - Rejecting `@external` **Slot** invocations inside `@transactional` actions or blocks ([ADR 0004](./docs/adr/0004-static-architectural-invariants.md)).
   - Detecting and rejecting/warning on N+1 `query`/`update`/`delete` operations inside bounded `for` loops ([ADR 0004](./docs/adr/0004-static-architectural-invariants.md)).
   - Verifying round-trip deterministic JSON serialization of the **Semantic IR** ([ADR 0003](./docs/adr/0003-purpose-built-fdn-grammar.md)).
2. **Middle-End Lowering & Schema Evolution (via Compiler Driver `Build`)**:
   - Compiling the identical `.fdn` **Blueprint** under `go` (`hexagonal`), `go` (`layered`), and `typescript` (`clean`) **Build Profiles** ([ADR 0002](./docs/adr/0002-three-stage-architecture-lowering.md)) and verifying the resulting architectural boundaries and dependency rules.
   - Evolving an **Entity** across two sequential builds (adding fields, renaming fields with `@renamed_from`, and detecting unhinted destructive drops) and verifying the emitted `schema.sql`, numbered migration files, and `foundation.lock.json`.
3. **Manifest Reconciliation & Slot Lifecycle (via Compiler Driver `Build`)**:
   - Verifying that initial build creates `.foundation-manifest.json` and emits create-if-missing **Slot Stubs** excluded from the manifest ([ADR 0001](./docs/adr/0001-action-pipelines-and-slot-interfaces.md), [ADR 0005](./docs/adr/0005-manifest-tracked-root-scaffolding.md)).
   - Verifying that modifying a **Slot Stub** with custom logic and re-running `Build` preserves the user's handwritten code untouched.
   - Verifying that removing an **Action** or **Entity** from a **Blueprint** prunes its previously emitted files on the next `Build`.
   - Verifying that manually tampering with a manifest-tracked file causes `Build` to abort with a conflict error unless forced.
4. **End-to-End Emitted Code & Scenario Execution**:
   - Executing `go test ./...` inside the generated Go (`hexagonal` and `layered`) workspaces and Go **SDK Client** to verify that all generated **Scenarios** and **Slot Contract Tests** compile and pass against generated in-memory fakes.
   - Executing TypeScript type-checking and test execution inside the generated TypeScript (`clean`) workspace and TypeScript **SDK Client** to verify cross-language parity.

## Out of Scope

- **Turing-Complete General-Purpose Language Constructs**: Unbounded `while` loops, arbitrary recursion, raw pointers, manual thread/goroutine spawning, and inline target-language code blocks inside `.fdn` files ([ADR 0001](./docs/adr/0001-action-pipelines-and-slot-interfaces.md)).
- **Frontend UI Component Generation**: Generating React/Vue/HTML visual components or CSS layouts (only backend applications, database schemas/migrations, and strongly-typed **SDK Clients** are in scope).
- **Runtime LLM Calls**: Neither the `foundation` compiler nor any emitted application code makes runtime calls to any LLM API.
- **Additional Target Languages Beyond Go, TypeScript, and PostgreSQL**: Rust, Python, Java, and non-SQL databases are future **Emitter** extensions enabled by the **Architecture IR**, not part of this initial specification.
- **Visual/GUI Blueprint Editors**: Only the `.fdn` text grammar and canonical **Semantic IR** JSON export/import are included.

## Further Notes

- All domain terminology in this specification follows [GLOSSARY.md](./GLOSSARY.md).
- All architectural constraints respect [ADR 0001](./docs/adr/0001-action-pipelines-and-slot-interfaces.md) through [ADR 0005](./docs/adr/0005-manifest-tracked-root-scaffolding.md).
