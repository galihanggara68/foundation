# Foundation (`.fdn`)

**Foundation** is a deterministic, rule-based, multi-target architecture and application compiler written in Go. It translates declarative domain **Blueprints** (`.fdn`) and a project **Build Profile** (`foundation.build.yaml`) into production-ready backend services, PostgreSQL schemas, sequential migrations, **Slot** interfaces with starter stubs, and hermetic test suites—with **zero runtime LLM dependency**.

---

## The 90/10 Rule

Foundation balances developer velocity, architectural integrity, and determinism via the **90/10 Rule** ([ADR 0001](./docs/adr/0001-action-pipelines-and-slot-interfaces.md)):

- **The 90% Generated Core (`Action Pipelines`)**: Domain entities, value objects, authorization policies, validation guards, transactional persistence, tactical traits (`@transactional`, `@idempotent`, `@outbox`, `@cache`, `@retry`), and HTTP transport routes are compiled deterministically.
- **The 10% Handcrafted Boundary (`Slots`)**: Proprietary algorithms, third-party APIs (Stripe, Twilio), and hardware protocols are declared as strongly-typed `slot` contracts. The compiler emits safe starter stubs that are **never overwritten** once customized.
- **Compile-Time Architectural Guardrails**: Enforces termination (no unbounded loops or recursive actions), exhaustive `match` handling, relational N+1 query prevention, and isolation of `@external` slots from tight database transactions.

---

## Three-Stage Compiler Architecture

Foundation cleanly separates business semantics from structural code generation:

```text
  .fdn Blueprints ──► [Stage 1: Frontend] ──► Validated Semantic IR (.fdn.json)
                            │
                            ▼ (+ foundation.build.yaml & foundation.lock.json)
                      [Stage 2: Middle-End] ──► Architecture IR + Storage Plan
                            │
                            ▼
                      [Stage 3: Backend Emitters & Manifest Reconciler]
                            ├──► Go Backend (Hexagonal) + Chi + PostgreSQL + Tests + Server Entrypoint
                            ├──► TypeScript Backend (Clean) + Express + PG + Tests + Server Entrypoint
                            ├──► Storage Artifacts (schema.sql & migrations/NNNN_*.sql)
                            └──► Manifest Tracker (.foundation-manifest.json)
```

---

## Quick Start

### 1. Build the CLI
Requires **Go 1.23+**:
```bash
git clone https://github.com/galihanggara68/foundation.git
cd foundation
go build -o bin/foundation ./cmd/foundation
```

### 2. Define a Project
Create `foundation.build.yaml`:
```yaml
version: "1"
name: "commerce"

target:
  language: "go"
  architecture: "hexagonal"
  module: "example.com/commerce"

storage:
  dialect: "postgres"
  mode: "both"
```

Create `orders.fdn`:
```fdn
entity Order {
  id: string @pk
  customer_id: string @index
  total: int
  version: int @version
}

error OrderNotFound {
  kind: not_found
  message: "Order does not exist"
}

action GetOrder(id: string) -> Order
  @http(method: GET, path: "/orders/{id}")
{
  return get Order where id == id or fail OrderNotFound
}
```

### 3. Check, Build, and Test
```bash
# Validate syntax, type safety, and architectural invariants
foundation check

# Compile blueprints into target backend code, SQL migrations, and stubs
foundation build

# Run hermetic offline scenario tests
foundation test
```

---

## Detailed Blueprint Documentation (`docs/blueprints/`)

Exhaustive guides and code examples for each component of the Foundation Blueprint language:

- [**Blueprint README & Sitemap**](./docs/blueprints/README.md)
- [**Top-Level Declarations & Syntax**](./docs/blueprints/declarations.md): Guide to `entity`, `value`, `enum`, `error`, `event`, `policy`, `slot`, `action`, `scenario`, optionals, and collection combinators (`.filter`, `.map`, `.sum`).
- [**HTTP Transport & Parameter Binding**](./docs/blueprints/http-transport.md): Route definition, path parameters (`/items/{id}`), request body unmarshaling, actor authentication headers, `ETag` / `If-Match`, and HTTP error mappings.
- [**Tactical Traits & Durability**](./docs/blueprints/tactical-traits.md): Enterprise resilience with `@transactional`, `@idempotent(key: ...)`, `@outbox`, `@cache`, `@retry`, DDL table generation, and background `OutboxRelayWorker`.
- [**Optimistic Concurrency Control (OCC)**](./docs/blueprints/concurrency-control.md): Field versioning with `@version`, conditional SQL execution (`version = version + 1`), HTTP `409 Conflict`, and client retry patterns.
- [**Slots & Handwritten Boundaries**](./docs/blueprints/slots-and-contracts.md): Handcrafted custom code boundaries, `@external` transaction protection, manifest exclusion, and automated contract tests.
- [**Declarative Scenarios & Testing**](./docs/blueprints/scenarios-and-testing.md): BDD-style unit tests with `given`, `when`, `expect`, slot mocking, and 100% offline hermetic execution.
- [**Build Profile & Configuration**](./docs/blueprints/build-profile.md): Reference for `foundation.build.yaml`, storage modes (`declarative`, `migrations`, `both`), and CLI commands.

---

## CLI Command Reference

```bash
Usage: foundation <check|build|test|ir> [--dir <path>] [--force]
```

| Command | Description |
|---|---|
| `foundation check` | Parses blueprints, resolves symbols, checks type safety, and verifies architectural rules. |
| `foundation build` | Compiles IR into production source code, SQL migrations, stubs, and updates `.foundation.manifest.json`. |
| `foundation test` | Compiles and executes native scenario tests (`go test ./...` or `npm test`) offline. |
| `foundation ir` | Emits canonical compiler Intermediate Representation (IR) JSON to stdout or file. |

---

## Architectural Guardrails & Invariants

| Diagnostic Code | Rule | Explanation |
|---|---|---|
| `FDN0210` | Termination | Unbounded `while` loops are rejected. |
| `FDN0211` | Termination | Recursive action calls are rejected. |
| `FDN0212` | Exhaustiveness | `match` expressions must exhaustively cover all enum variants. |
| `FDN0213` | Isolation | `@external` slot methods cannot be invoked inside `@transactional` actions. |
| `FDN0214` | Performance | Relational database operations (`query`, `save`, `delete`) are forbidden inside `for` loops. |

---

## Specifications & Decision Records

- [Ubiquitous Language Glossary](./GLOSSARY.md)
- [Public Trust-First Roadmap](./ROADMAP.md)
- [Coding & Architectural Standards](./CODING_STANDARDS.md)
- **Technical Specifications**:
  - [Spec 00: Baseline Compiler & Architecture](./docs/specs/00-baseline-compiler-and-architecture.md)
  - [Spec 01: Runtime Integrity & Durability](./docs/specs/01-runtime-integrity-and-durability.md)
  - [Spec 02: Production API Surface & Security](./docs/specs/02-production-api-surface-and-security.md)
- **Architectural Decision Records**:
  - [ADR 0001: Constrained Action Pipelines & Slots](./docs/adr/0001-action-pipelines-and-slot-interfaces.md)
  - [ADR 0002: Three-Stage Lowering](./docs/adr/0002-three-stage-architecture-lowering.md)
  - [ADR 0003: Purpose-Built Grammar](./docs/adr/0003-purpose-built-fdn-grammar.md)
  - [ADR 0004: Static Invariants](./docs/adr/0004-static-architectural-invariants.md)
  - [ADR 0005: Manifest-Tracked Scaffolding](./docs/adr/0005-manifest-tracked-root-scaffolding.md)
  - [ADR 0006: Actor Resolver Port](./docs/adr/0006-actor-resolver-port-with-contrib-adapters.md)
  - [ADR 0007: Post-Commit Event Dispatch](./docs/adr/0007-post-commit-in-process-event-dispatch.md)
  - [ADR 0008: Deterministic Query-Many](./docs/adr/0008-deterministic-query-many.md)
  - [ADR 0009: Contrib Runtime Libraries](./docs/adr/0009-contrib-runtimes-for-stateful-traits.md)
  - [ADR 0010: Optimistic Concurrency Control](./docs/adr/0010-optimistic-concurrency-control.md)
  - [ADR 0011: Standardized IR Protocol](./docs/adr/0011-pluggable-emitter-protocol.md)
