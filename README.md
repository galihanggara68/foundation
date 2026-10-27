# Foundation (`.fdn`)

> **Stop writing backend plumbing.** Define your domain entities, authorization policies, and action pipelines in declarative Blueprints (`.fdn`). Foundation compiles them into production-ready Go and TypeScript backends, PostgreSQL schemas, sequential migrations, and automated test suites—with **zero runtime LLM dependency** and **100% offline determinism**.

---

## 💡 Why Foundation?

Modern backend development suffers from two extremes:

1. **The Boilerplate Trap**: Up to 90% of a production backend is repetitive plumbing—HTTP routing, DTO validation, database transaction managers, idempotency stores, transactional outboxes, optimistic concurrency checks, and SQL schema migrations.
2. **The LLM Generation Trap**: Asking AI assistants to generate entire backends leads to silent drift, hallucinated dependencies, architectural erosion, and maintenance nightmares.

**Foundation solves this with the 90/10 Rule** ([ADR 0001](./docs/adr/0001-action-pipelines-and-slot-interfaces.md)):

- **The 90% Generated Core**: Foundation deterministically compiles the repetitive plumbing into idiomatic, typed code following proven architectural patterns (Hexagonal Architecture in Go or Clean Architecture in TypeScript).
- **The 10% Handcrafted Boundary (`Slots`)**: Custom proprietary algorithms, third-party integrations (Stripe, Twilio), and hardware protocols live in strongly-typed **Slot** interfaces. Foundation generates safe starter stubs that are **never overwritten** once customized.

---

## ⚡ Key Features at a Glance

- 🛡️ **Compile-Time Architectural Guardrails**:
  Rejects relational N+1 queries in loops, unbounded `while` loops, recursive actions, and unhandled enum variants at compile time.
- 🔄 **Enterprise Tactical Traits**:
  Annotate action pipelines with `@idempotent(key: ...)`, `@transactional`, `@outbox`, `@cache(ttl: ...)`, and `@retry(attempts: ...)`. Foundation emits the corresponding PostgreSQL DDL and background `OutboxRelayWorker` with `FOR UPDATE SKIP LOCKED` automatically.
- 🔒 **Native Optimistic Concurrency Control (OCC)**:
  Tag entity version fields with `@version` to generate conditional SQL updates (`WHERE id = $1 AND version = $2`), automatic `ETag` / `If-Match` HTTP headers, and deterministic HTTP 409 Conflict handling.
- 🧪 **Declarative BDD Scenarios**:
  Write integration tests directly inside `.fdn` using `given`, `when`, and `expect`. They compile into native `go test` and `npm test` suites with zero mock boilerplate.
- 🎯 **Multi-Target Code Generation**:
  Compile the exact same domain blueprint into a Go Hexagonal service (Chi router + `database/sql`) or a TypeScript Clean service (Express + `pg`).
- 🔌 **Pluggable Generator Pipelines (`foundation-gen-*`)**:
  Compose external generators sequentially (`generators: [...]`) for any language or architecture (Python FastAPI, Rust Axum, Rich DDD Aggregate Roots, OpenTelemetry distributed tracing) with scaffolding pre-population (`scaffold: builtin` vs `none`).

---

## ⚠️ Intentional Boundaries & Limitations

To guarantee architectural integrity, predictability, and safety, Foundation defines clear boundaries:

### What Foundation is NOT:
- **Not a visual Low-Code / No-Code tool**: Foundation is a text-first, git-versioned compiler built by software engineers for software engineers.
- **Not an ORM or Dynamic Query Builder**: Foundation compiles strict, deterministic domain repositories and migrations. It is not designed for runtime ad-hoc reporting or arbitrary SQL construction.
- **Not a proprietary runtime**: Generated backends depend only on standard open-source libraries (`chi`, `express`, `pg`, `lib/pq`). There is no proprietary cloud runtime lock-in.

### Intentional Constraints (By Design):
- **Guaranteed Termination**: Action pipelines are deterministic finite state transitions. Unbounded loops and recursive action calls are rejected at compile time.
- **Transaction Isolation**: Calling third-party APIs (`@external` slots like Stripe or Twilio) inside `@transactional` actions is forbidden at compile time to protect database connection pools.
- **No N+1 Queries**: Database queries and mutations inside `for` loops are strictly rejected.
- **The Escape Hatch**: If logic cannot be expressed cleanly within an action pipeline, declare a `slot`. You implement slots in native Go or TypeScript with complete language freedom.

### Current Platform Scope:
- **Storage**: PostgreSQL (declarative schemas, sequential up/down migrations, transactional outbox tables).
- **Languages**: Built-in Go 1.23+ and TypeScript / Node.js 20+; polyglot support via external Generator Plugins (e.g. Python FastAPI reference generator).
- **Transports**: HTTP/REST routes (gRPC and event queue consumer transports are on the roadmap).

### Real-World Boundaries & Seams:
- **Relational Joins & Aggregates**: Foundation compiles 1:1 entity-to-table repositories. Cross-table SQL `JOIN`s, grouping, and complex reporting are handled via CQRS Read-Model **Slots** or multi-step pipeline queries.
- **Secondary Adapter Swapping**: Domain port interfaces (`internal/domain`) decouple your application from default drivers (`database/sql`), allowing custom adapters (`jackc/pgx/v5`, `gorm`, `google/uuid`).
- **Anemic Models vs. Pipelines**: Entities are emitted as pure IR data structures; business logic and state invariants reside in declarative Action Pipelines or handwritten Domain Service Slots.
- **Auth Verification vs. Token Issuance**: `authn: jwt-oidc` provides an Ingress Actor Resolver port for validating tokens; user login and token issuance are handled by external IdPs or crypto Slots.
- **HTTP Ingress Flexibility**: Handlers bind path, query, and body parameters; custom request/response headers are managed via standard HTTP middleware wrapping the generated router.
- **Schema Migrations**: Emits deterministic snapshot DDL (`schema.sql`); incremental migration tracking is handled by dedicated migration tools (`goose`, `atlas`, `golang-migrate`).

👉 **[Read the complete guide to Extension Patterns, Architectural Seams & Platform Boundaries](./docs/blueprints/extension-patterns-and-limitations.md)**.

---

## 🎯 Common Use Cases

- **Fintech & Payment Services**:
  Execute balance transfers with `@transactional`, double-spend prevention via `@version`, deduplicated requests via `@idempotent(key: ...)`, and guaranteed post-commit event publishing via `@outbox`.
- **E-Commerce & Inventory Management**:
  Manage cart calculations with collection combinators (`.filter`, `.map`, `.sum`), enforce coupon expiration guards, and prevent overselling using Optimistic Concurrency Control.
- **Multi-Tenant SaaS Backends**:
  Declare reusable authorization policies (`policy CanAccess(actor: Actor)`), generate clean migration histories, and scaffold decoupled service boundaries.

---

## 🚀 Quickstart in 60 Seconds

### 1. Build or Install the CLI

Requires **Go 1.23+**:

```bash
git clone https://github.com/galihanggara68/foundation.git
cd foundation
go build -o bin/foundation ./cmd/foundation
```

### 2. Define Your Project (`foundation.build.yaml`)

```yaml
project:
  name: wallet-service
  module: example.com/wallet

storage:
  dialect: postgres
  mode: both # Emits schema.sql and sequential migrations/

targets:
  - name: backend
    kind: backend
    language: go
    architecture: hexagonal
    http: chi
    out_dir: ./dist/backend
```

### 3. Write a Blueprint (`wallet.fdn`)

```fdn
entity Wallet {
  id: uuid @pk
  owner_id: uuid @unique
  balance: decimal(12, 2)
  version: int @version
}

event MoneyTransferred {
  from_id: uuid
  to_id: uuid
  amount: decimal(12, 2)
}

error WalletNotFound {
  kind: not_found
  message: "Wallet does not exist"
}

error InsufficientFunds {
  kind: validation
  message: "Sender wallet balance is insufficient"
}

value TransferInput {
  to_wallet_id: uuid
  amount: decimal(12, 2)
  idempotency_key: string
}

action TransferMoney(from_id: uuid, input: TransferInput) -> bool
  @http(method: POST, path: "/wallets/{from_id}/transfer")
  @transactional
  @idempotent(key: input.idempotency_key)
  @outbox
{
  let sender = query one Wallet where id == from_id else WalletNotFound
  let recipient = query one Wallet where id == input.to_wallet_id else WalletNotFound

  guard sender.balance >= input.amount else InsufficientFunds

  update Wallet where id == from_id set {
    balance: sender.balance - input.amount
  }

  update Wallet where id == input.to_wallet_id set {
    balance: recipient.balance + input.amount
  }

  emit MoneyTransferred {
    from_id: sender.id,
    to_id: recipient.id,
    amount: input.amount
  }

  return true
}

scenario "TransferMoney transfers balance and records outbox event" {
  given Wallet {
    id: "00000000-0000-0000-0000-000000000001",
    owner_id: "00000000-0000-0000-0000-000000000099",
    balance: 100.00,
    version: 1
  }
  given Wallet {
    id: "00000000-0000-0000-0000-000000000002",
    owner_id: "00000000-0000-0000-0000-000000000098",
    balance: 50.00,
    version: 1
  }
  when TransferMoney("00000000-0000-0000-0000-000000000001", TransferInput {
    to_wallet_id: "00000000-0000-0000-0000-000000000002",
    amount: 30.00,
    idempotency_key: "tx-1001"
  })
  expect result == true
  expect stored Wallet where id == "00000000-0000-0000-0000-000000000001" {
    balance: 70.00
  }
  expect stored Wallet where id == "00000000-0000-0000-0000-000000000002" {
    balance: 80.00
  }
}
```

### 4. Check, Build, and Test

```bash
# Verify syntax, type correctness, and architectural invariants
foundation check

# Compile into Go backend, Chi router, SQL migrations, and outbox tables
foundation build

# Run hermetic offline scenario tests
foundation test
```

---

## CLI Command Reference

```bash
Usage: foundation <check|build|test|ir> [--dir <path>] [--force]
```

| Command | Description |
| :--- | :--- |
| `foundation check` | Parses blueprints, resolves symbol tables, type-checks actions, and verifies architectural rules. |
| `foundation build` | Compiles IR into production code, executes generator pipelines, and reconciles `.foundation-manifest.json`. |
| `foundation test` | Compiles and runs native target scenario tests (`go test ./...`, `npm test`, or `test_command` like `pytest`) 100% offline. |
| `foundation ir` | Emits canonical Semantic IR JSON to stdout for tooling and inspection. |

---

## 🛠️ Development & Testing

Foundation includes a root `Makefile` for local development workflows and categorized test execution:

```bash
# Build
make build             # Compile Foundation CLI and reference generator plugins into bin/
make clean             # Remove build outputs and binaries

# Testing by Category
make test-unit         # Run fast, hermetic unit tests (cmd/ and pkg/)
make test-e2e          # Run end-to-end integration tests (test/e2e/)
make test-security     # Run sandboxing & adversarial security suite (test/security/)
make test-durability   # Run live HTTP durability & crash recovery suite (test/durability/)
make test-scale        # Run heavy compiler scaling & stress suite (test/scale/)
make test-contrib      # Run Go and TypeScript runtime test suites
make test-all          # Run all test suites across the repository

# Quality & Code Discipline
make fmt               # Format all Go source files with gofmt
make fmt-check         # Verify all files are formatted
make line-check        # Enforce file size discipline (<= 450 lines per file)
make vet               # Run go vet across all packages
make verify            # Full pre-commit gate (fmt-check, line-check, vet, test-unit)
```

---

## 📚 Blueprint Guides & Documentation

Explore in-depth documentation and reference examples in [`docs/blueprints/`](./docs/blueprints/README.md):

- [**Declarations & Syntax**](./docs/blueprints/declarations.md): Complete guide to `entity`, `value`, `enum`, `error`, `event`, `policy`, `slot`, `action`, and combinators.
- [**HTTP Transport & Routing**](./docs/blueprints/http-transport.md): Path binding (`/items/{id}`), request body mapping, headers, and error status codes.
- [**Tactical Traits & Durability**](./docs/blueprints/tactical-traits.md): Enterprise durability with `@transactional`, `@idempotent`, `@outbox`, `@cache`, and PostgreSQL relay workers.
- [**Optimistic Concurrency Control**](./docs/blueprints/concurrency-control.md): Field versioning with `@version`, conditional SQL updates, and HTTP 409 Conflict handling.
- [**Slots & Custom Boundaries**](./docs/blueprints/slots-and-contracts.md): Handwritten code seams, starter stubs, manifest exclusion, and automated contract tests.
- [**BDD Scenarios & Testing**](./docs/blueprints/scenarios-and-testing.md): BDD testing syntax (`given`, `when`, `expect`) and hermetic execution.
- [**Build Profile Configuration**](./docs/blueprints/build-profile.md): Reference for `foundation.build.yaml` options, targets, and storage modes.
- [**Generator Plugins & Emitters**](./docs/blueprints/generator-plugins.md): Subprocess plugin wire protocol, multi-generator pipelines, scaffolding pre-population, and authoring custom generators with `pkg/emitterkit`.
- [**Extension Patterns & Platform Boundaries**](./docs/blueprints/extension-patterns-and-limitations.md): Cookbook guide for real-world backend needs: SQL `JOIN`s, swapping DB drivers (`pgx`), custom headers, DDD aggregate invariants, and JWT token issuance.

For architecture decisions and compiler specifications, see [Architectural Decision Records (`docs/adr/`)](./docs/adr/) and [Technical Specifications (`docs/specs/`)](./docs/specs/).
