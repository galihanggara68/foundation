# Foundation (`.fdn`)

> Stop writing backend boilerplate code. Define domain entities, authorization policies, and action pipelines in declarative Blueprint files (`.fdn`). Foundation compiles blueprints into Go and TypeScript backends, PostgreSQL schemas, sequential migrations, and automated test suites. Foundation operates with zero runtime LLM dependency and 100 percent offline determinism.

---

## Why Foundation?

Modern backend development has two common problems:

1. **Repetitive Boilerplate Code**: Up to 90 percent of a backend consists of plumbing code. This code includes HTTP routing, input validation, database transactions, idempotency stores, outbox tables, and schema migrations.
2. **Unreliable AI Code Generation**: AI assistants produce hallucinated dependencies, architectural drift, and maintenance problems when they generate full backends.

Foundation solves these problems with the 90/10 Rule ([ADR 0001](./docs/adr/0001-action-pipelines-and-slot-interfaces.md)):

- **The 90 Percent Generated Core**: Foundation compiles repetitive plumbing code into typed, idiomatic code. Foundation supports Hexagonal Architecture in Go and Clean Architecture in TypeScript.
- **The 10 Percent Handcrafted Boundary (`Slots`)**: Custom algorithms, hardware protocols, and third-party integrations live in strongly typed Slot interfaces. Foundation generates safe starter stubs. Foundation never overwrites customized slot files.

---

## Key Features at a Glance

- **Compile-Time Architectural Guardrails**:
  The compiler rejects relational N+1 queries in loops, unbounded loops, recursive actions, and unhandled enum variants at compile time.
- **Enterprise Tactical Traits**:
  Annotate action pipelines with `@idempotent(key: ...)`, `@transactional`, `@outbox`, `@cache(ttl: ...)`, and `@retry(attempts: ...)`. Foundation emits the corresponding PostgreSQL DDL and a background outbox worker automatically.
- **Native Optimistic Concurrency Control (OCC)**:
  Tag entity version fields with `@version`. Foundation generates conditional SQL updates, automatic `ETag` and `If-Match` HTTP headers, and deterministic HTTP 409 conflict handling.
- **Declarative BDD Scenarios**:
  Write integration tests inside `.fdn` files with `given`, `when`, and `expect` blocks. They compile into native test suites without mock boilerplate.
- **Multi-Target Code Generation**:
  Compile one domain blueprint into a Go Hexagonal service or a TypeScript Clean service.
- **Pluggable Generator Pipelines**:
  Compose external generators in sequence (`generators: [...]`) for any target language or architecture. Foundation supports scaffolding pre-population.
- **Editor Support with Language Server**:
  The `foundation lsp` command speaks LSP 3.17 over standard I/O. The language server gives live diagnostics, hover documentation, code completion, and go-to-definition. Official editor extensions include [`tree-sitter-foundation`](https://github.com/galihanggara68/tree-sitter-foundation), [`foundation-zed`](https://github.com/galihanggara68/foundation-zed), and [`foundation-vscode`](https://github.com/galihanggara68/foundation-vscode).

---

## Intentional Boundaries and Limitations

Foundation maintains clear boundaries to protect architectural safety and predictability:

### What Foundation is Not:
- **Not a Low-Code Tool**: Foundation is a text-first, version-controlled compiler for software engineers.
- **Not an ORM or Dynamic Query Builder**: Foundation compiles strict domain repositories and migrations. It does not construct arbitrary runtime SQL queries.
- **Not a Proprietary Runtime**: Generated backends use standard open-source libraries (`chi`, `express`, `pg`, `lib/pq`). Foundation causes no proprietary vendor lock-in.

### Intentional Constraints:
- **Guaranteed Termination**: Action pipelines execute deterministic finite state transitions. The compiler rejects unbounded loops and recursive action calls.
- **Transaction Isolation**: The compiler forbids calls to external slots inside `@transactional` actions to protect database connections.
- **No N+1 Queries**: The compiler rejects database queries and mutations inside `for` loops.
- **Custom Extension Seams**: When logic cannot fit into an action pipeline, declare a `slot`. Write slot implementations in Go or TypeScript with full language freedom.

### Platform Scope:
- **Storage**: PostgreSQL (declarative schemas, sequential migrations, transactional outbox tables).
- **Languages**: Go 1.23+ and TypeScript on Node.js 20+; external generator plugins support other languages.
- **Transports**: HTTP/REST routes.

### Real-World Boundaries and Seams:
- **Relational Joins and Aggregates**: Foundation compiles 1:1 entity-to-table repositories. Handle cross-table joins, grouping, and reports with CQRS read-model slots or multi-step pipeline queries.
- **Secondary Adapter Swapping**: Domain port interfaces in `internal/domain` decouple your application from default database drivers. You can supply custom adapters.
- **Data Models and Pipelines**: Foundation emits entities as plain data structures. Business logic and state invariants live in action pipelines or handwritten domain slots.
- **Authentication and Identity**: The `authn: jwt-oidc` setting provides an actor resolver port to validate tokens. External identity providers or custom slots handle user login and token issuance.
- **HTTP Ingress Control**: Handlers bind path, query, and body parameters. Standard HTTP middleware manages custom request and response headers.
- **Schema Migrations**: Foundation emits snapshot DDL (`schema.sql`). Dedicated migration tools (`goose`, `atlas`, `golang-migrate`) manage incremental migration tracking.

Read the [complete guide to extension patterns, architectural seams, and boundaries](./docs/blueprints/extension-patterns-and-limitations.md).

---

## Common Use Cases

- **Fintech and Payment Services**:
  Run balance transfers with `@transactional`. Prevent double spending with `@version`. Deduplicate requests with `@idempotent(key: ...)`. Publish reliable post-commit events with `@outbox`.
- **E-Commerce and Inventory Management**:
  Calculate cart totals with collection combinators (`.filter`, `.map`, `.sum`). Enforce coupon validation rules. Prevent overselling with optimistic concurrency control.
- **Multi-Tenant SaaS Backends**:
  Declare reusable authorization policies (`policy CanAccess(actor: Actor)`). Generate clean migration files. Create decoupled service boundaries.

---

## Quickstart in 60 Seconds

### 1. Install the CLI

Choose one installation method:

#### Option A: Install via `curl` (Prebuilt Binaries)

Download the prebuilt binary for your platform (`Linux`, `macOS`, or `Windows` on `amd64` or `arm64`):

```bash
curl -fsSL https://raw.githubusercontent.com/galihanggara68/foundation/main/install.sh | sh
```

You can set a custom installation directory or version:

```bash
FOUNDATION_INSTALL_DIR=$HOME/.local/bin FOUNDATION_VERSION=v0.9.0 curl -fsSL https://raw.githubusercontent.com/galihanggara68/foundation/main/install.sh | sh
```

#### Option B: Install via `go install` (Requires Go 1.23+)

```bash
go install github.com/galihanggara68/foundation/cmd/foundation@latest
# Or install a specific version:
go install github.com/galihanggara68/foundation/cmd/foundation@v0.9.0
```

#### Option C: Build from Source

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
# Check syntax, types, and architectural rules
foundation check

# Compile into Go backend, Chi router, SQL migrations, and outbox tables
foundation build

# Run scenario tests offline
foundation test
```

---

## CLI Command Reference

```bash
Usage: foundation <check|build|ir|test|lsp|version> [--dir <path>] [--force]
```

| Command | Description |
| :--- | :--- |
| `foundation check` | Parses blueprints, resolves symbol tables, type-checks actions, and verifies architectural rules. |
| `foundation build` | Compiles IR into production code, runs generator pipelines, and reconciles the manifest. |
| `foundation test` | Compiles and runs scenario tests offline with native runners (`go test`, `npm test`, or `pytest`). |
| `foundation ir` | Emits canonical Semantic IR JSON to standard output. |
| `foundation lsp` | Runs the Foundation language server over standard I/O for editor integration. |
| `foundation version` | Prints compiler version, commit hash, and build timestamp. |

---

## Development and Testing

Foundation includes a root `Makefile` for local development workflows and test runs:

```bash
# Build
make build             # Compile Foundation CLI and reference generator plugins into bin/
make clean             # Remove build outputs and binaries

# Testing by Category
make test-unit         # Run unit tests in cmd/ and pkg/
make test-e2e          # Run end-to-end integration tests in test/e2e/
make test-security     # Run security tests in test/security/
make test-durability   # Run durability and crash recovery tests in test/durability/
make test-scale        # Run compiler scaling tests in test/scale/
make test-contrib      # Run Go and TypeScript runtime test suites
make test-all          # Run all test suites across the repository

# Quality and Code Discipline
make fmt               # Format all Go source files with gofmt
make fmt-check         # Verify code formatting
make line-check        # Verify file size limits (<= 450 lines per file)
make vet               # Run go vet across all packages
make verify            # Run pre-commit gate (fmt-check, line-check, vet, test-unit)

# Release and Versioning
make version           # Print version of current tree
make changelog-draft   # Print commit subjects since last tag for CHANGELOG
make release-check     # Verify clean tree, semver tag, CHANGELOG entry, and verify gate
make release VERSION=x.y.z  # Validate, tag, and push release
make snapshot          # Cross-compile release archives into dist/ without tagging
```

The build process injects the version from `git describe`. The command `make build` produces a binary that reports its exact version through `foundation version`.

> [!NOTE]
> The TypeScript target suites require `tsc` on your `PATH`.
> `make test-unit`, `make test-e2e`, and `make verify` build TypeScript targets and check them with `tsc`.
> The emitted `package.json` declares `tsc` as a devDependency.
> A shell with `NODE_ENV=production` causes npm to skip development dependencies.
> That environment causes the suites to fail with `exec: "tsc": executable file not found in $PATH` (`FDN0600`).
> Run the gate with `env -u NODE_ENV make verify`.
> If an earlier run created `node_modules` without devDependencies, delete that directory first.

---

## Blueprint Guides and Documentation

Read in-depth documentation and reference examples in [`docs/blueprints/`](./docs/blueprints/README.md):

- [**Declarations and Syntax**](./docs/blueprints/declarations.md): Guide to `entity`, `value`, `enum`, `error`, `event`, `policy`, `slot`, `action`, and combinators.
- [**HTTP Transport and Routing**](./docs/blueprints/http-transport.md): Path binding, request body mapping, headers, and error status codes.
- [**Tactical Traits and Durability**](./docs/blueprints/tactical-traits.md): Enterprise durability with `@transactional`, `@idempotent`, `@outbox`, `@cache`, and PostgreSQL relay workers.
- [**Optimistic Concurrency Control**](./docs/blueprints/concurrency-control.md): Field versioning with `@version`, conditional SQL updates, and HTTP 409 conflict handling.
- [**Slots and Custom Boundaries**](./docs/blueprints/slots-and-contracts.md): Handwritten code seams, starter stubs, manifest exclusion, and automated contract tests.
- [**BDD Scenarios and Testing**](./docs/blueprints/scenarios-and-testing.md): BDD testing syntax (`given`, `when`, `expect`) and offline test execution.
- [**Build Profile Configuration**](./docs/blueprints/build-profile.md): Reference for `foundation.build.yaml` options, targets, and storage modes.
- [**Generator Plugins and Emitters**](./docs/blueprints/generator-plugins.md): Subprocess plugin wire protocol, multi-generator pipelines, scaffolding pre-population, and custom generators with `pkg/emitterkit`.
- [**Editor Support and Language Server**](./docs/blueprints/editor-support.md): The in-tree LSP server (`foundation lsp`), live diagnostics, hover, completions, member completion, and go-to-definition.
- [**Extension Patterns and Platform Boundaries**](./docs/blueprints/extension-patterns-and-limitations.md): Cookbook guide for SQL joins, database drivers, custom headers, aggregate invariants, and JWT authentication.

For architecture decisions and compiler specifications, see [Architectural Decision Records (`docs/adr/`)](./docs/adr/) and [Technical Specifications (`docs/specs/`)](./docs/specs/).

---

## Community and Contributing

We welcome community contributions, bug reports, and generator plugin ideas.

- **[Contributing Guide](./CONTRIBUTING.md)**: Development setup, `make verify` pre-commit gate, line discipline, and pull request workflow.
- **[Security Policy](./SECURITY.md)**: Vulnerability disclosure and defensive sandboxing guarantees.
- **[Coding Standards](./CODING_STANDARDS.md)**: Code formatting, architectural principles, and line count boundaries.
- **[Roadmap](./ROADMAP.md)**: Phased platform evolution and planned features.

---

## License

Foundation is licensed under the [Apache License, Version 2.0](./LICENSE).
Copyright 2026 Foundation Authors.
