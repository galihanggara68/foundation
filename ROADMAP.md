# Foundation Roadmap

Foundation is being built as a **public open-source project**: other engineers adopt it, so trust outranks features — a missing feature loses a user, a lying feature loses the project's reputation. This roadmap resolves every known gap between what the language promises and what the runtime delivers before adding new promises.

**Guiding principle** (ADRs [0006](./docs/adr/0006-actor-resolver-port-with-contrib-adapters.md), [0009](./docs/adr/0009-contrib-runtimes-for-stateful-traits.md), [0010](./docs/adr/0010-optimistic-concurrency-control.md), & [0011](./docs/adr/0011-pluggable-emitter-protocol.md)): *the compiler owns the shape — structure, wiring, ports; published contrib libraries own the behavior — anything with state or a trust boundary.* Foundation already treats chi, express, and pg as imports; its runtime machinery is infrastructure in the same sense.

Phases are sequential; each links its decision records.

---

## Phase 0 — Versioning, Publishing Contract, & Contrib Architecture *(prerequisite for all contrib releases)*

Compiler, authn adapters, and trait runtimes become versioned artifacts. Before the first contrib package is published:

- Contrib libraries are maintained within the monorepo under `contrib/go/runtime` and `contrib/ts/runtime`, published under SemVer.
- Generated code pins contrib library version ranges; the build verifies compiler↔library compatibility and refuses to guess.
- A published contract states what may change when: during 0.x, tracked output may change in any release; **Slots, Scenarios, and the manifest format are stable within a minor**.
- **Hermetic test guarantee**: Trait memory drivers have zero external third-party dependencies, guaranteeing that `foundation test` runs 100% offline without registry access or cold-cache network round-trips.

**Exit:** contract documented and enforced; contrib packages buildable and testable offline. See ADRs [0006](./docs/adr/0006-actor-resolver-port-with-contrib-adapters.md), [0009](./docs/adr/0009-contrib-runtimes-for-stateful-traits.md).

## Phase 1 — Honest Grammar, Event Dispatch, & Runnable Go Server

Kill the traits that lower but never run, and establish runtime parity between Go and TypeScript (ADR [0007](./docs/adr/0007-post-commit-in-process-event-dispatch.md)):

- `@cron` is cut from the grammar — a parse error marked *reserved* — until there is an honest leader-election story; a ticker under two replicas silently double-fires.
- `@event_handler` lowers to a synchronous in-process dispatcher emitted as tracked code: handlers are ordinary Actions running in declaration order **after** the emitting action's transaction commits, composing with their own traits (`@retry`, `@transactional`, `@outbox`).
- Handler Actions carry their own Scenarios.
- **Runnable Go Entrypoint**: Go target emits a manifest-tracked `cmd/server/main.go` with signal handling (`SIGINT`, `SIGTERM`), graceful `http.Server.Shutdown`, and environment-driven port/database configuration, matching TypeScript's `server.ts`.

**Exit:** a Blueprint with `@event_handler` runs handlers post-commit on every target; `@cron` fails fast with a clear message; `go run ./cmd/server` boots a functional server out of the box.

## Phase 2 — Durable Tactical Traits & Concurrency Control

Fix the runtime promises broken under multi-replica deployments — the double-charge (`@idempotent`), the vanished event (`@outbox`), and silent lost updates (ADRs [0009](./docs/adr/0009-contrib-runtimes-for-stateful-traits.md), [0010](./docs/adr/0010-optimistic-concurrency-control.md)):

- One contrib runtime library with swappable drivers: `postgres` persists `idempotency_keys` and outbox rows **inside** the action transaction and replays after crash; `memory` preserves dev/test semantics with documented limits; `@cache` gains a driver seam.
- Emitters stop generating trait plumbing and emit wiring against the library's ports; `@retry` and `@transactional` stay inline (pure control flow).
- **Optimistic Concurrency Control (OCC)**: `@version` field trait on entities; PostgreSQL emits conditional atomic increments (`WHERE id = $1 AND version = $2`); in-memory repositories assert version matches; mutation failures raise built-in `conflict` error (HTTP 409); HTTP handlers emit `ETag` and validate `If-Match`.
- Open design item: DDL ownership for library-owned tables (compiler-emitted vs library-shipped migrations).

**Exit:** double-charge, crash-loss, and concurrent lost-update scenarios are impossible in postgres mode; Scenarios run against the memory driver unchanged.

## Phase 3 — Authentication & Flexible Actor Schema

Close the gap between the Actor/policy language and the runtime that trusts client headers verbatim (ADR [0006](./docs/adr/0006-actor-resolver-port-with-contrib-adapters.md)):

- **Actor Resolver** port in every generated backend; Build Profile `authn:` selects the adapter.
- Built-in `unauthenticated` error kind (HTTP 401) — part of the compiler-owned standard error vocabulary (ADR [0016](./docs/adr/0016-error-vocabulary-ownership.md)).
- **Flexible Actor Schema**:
  - *Zero-Config*: APIs without actor policies generate no authn middleware.
  - *Implicit Baseline*: Standard `{ id: string, role: string, verified: bool }` provided if policies use `actor` without an explicit declaration.
  - *Custom Schema*: Optional `actor <Name> { ... }` block in Blueprint for tenant and claim attributes (`tenant_id`, `tier`, etc.), statically type-checked in policies and scenarios.
- Unconfigured profiles keep the **trusted-headers dev adapter** for backward compatibility, with a boot banner *and* a structured warning log line; docs never bless it for production.
- First published authn adapter: `jwt-oidc` (JWT/JWKS verification before policies evaluate).
- Secure-by-default (deny-all) is revisited at the 1.0 gate.

**Exit:** generated backends verify real tokens before policies run; multi-tenant claims type-check in policies; quick-start opts into dev headers with one line.

## Phase 4 — Deterministic Query Many & Domain Types

Make the read surface of a real API expressible (ADR [0008](./docs/adr/0008-deterministic-query-many.md)):

- `query many` step: filtered, ordered, cursor-paginated reads. Determinism is enforced by the grammar: `order_by` on a unique field is a hard validation error when missing; limits are mandatory with a compiler-enforced hard cap; pagination is keyset cursoring — offset is permanently out.
- **Predicate Operators**: `where` clauses expand to support comparison operators (`>`, `<`, `>=`, `<=`, `!=`) and conjunction (`and`), maintaining sargable single-pass query plans.
- **Domain Types**:
  - `timestamp`: Upgraded from plain string to native `time.Time` (Go) and `Date` (TypeScript) with RFC3339 serialization and inequality comparison.
  - `decimal`: Upgraded from floating point to fixed-precision arithmetic across all targets.
- List-returning Actions lower to list HTTP routes across all targets; the validator owns the invariants so no emitter re-derives them.

**Exit:** `GET /v1/orders` with keyset cursoring, status filtering, and exact arithmetic builds in-language on every target.

## Phase 5 — SDK Client Emitters

The existing [ticket #07](./.scratch/00-baseline-compiler-and-architecture/issues/07-go-and-typescript-sdk-client-emitters.md), deferred so it incorporates the full trustworthy runtime contract in a single move:

- Strongly-typed, zero-boilerplate client packages emitted from a Blueprint's Actions, Values, and Error contracts.
- First-class support for single actions, keyset-paginated `query many` reads, and all built-in error kinds (`unauthenticated`, `forbidden`, `not_found`, `conflict`).
- Pluggable authn token interceptors matching target idioms.

**Exit:** ticket #07 acceptance criteria, end-to-end verified against running Go and TypeScript HTTP servers.

## Phase 6 — Standardized IR Protocol & Pluggable Generator Plugins *(Implemented & Verified)*

Enable external language adoption and company-specific architectures without hardcoding them into the compiler binary (ADRs [0011](./docs/adr/0011-pluggable-emitter-protocol.md), [0014](./docs/adr/0014-multi-generator-plugin-pipelines.md)):

- **Standardized Wire Protocol**: Formally published JSON Schema for `GenerationRequest` (containing `SemanticIR`, optional `ArchitectureIR`, and target config) and `GenerationResponse` (target file tree, create-if-missing stubs, and diagnostics).
- **Subprocess Plugin Runner**: The compiler invokes `foundation-gen-<target>` executables over standard I/O; the compiler's Manifest Engine maintains exclusive control of disk writes, content hashing, conflict detection, and stale-file pruning.
- **Generator Pipelines & Chaining**: Multi-plugin pipelines (`generators: [...]`) supporting `scaffold: builtin` (pre-populating the standard 90% scaffolding for partial overrides) and `scaffold: none` (clean slate for new languages), with collision detection (`FDN0702`).
- **Composable `emitterkit` SDK**: Reusable Go libraries for SQL generation, domain types, and Action Pipeline statement lowering so plugin authors do not rewrite generators from scratch.
- **Testing Seam**: Execution of tests emitted by third-party plugins is delegated to native language toolchains (`pytest`, `cargo test`, etc.) with configurable timeouts.

**Exit:** two working reference generators running via `generators:` / `generator:` with full Generation Manifest hygiene:
1. **Rich DDD + OpenTelemetry Trace Plugin Pipeline**: Emits Go Aggregate Roots with encapsulated business invariant methods alongside injected OpenTelemetry tracing middleware. Verified via `go test ./...`.
2. **Python FastAPI Generator Plugin**: Emits a simple, runnable Python FastAPI HTTP backend from `.fdn` with automated test execution via `pytest`. Verified end-to-end.

## Phase 7 — Editor Tooling & Language Server *(Implemented & Verified)*

Bring `.fdn` Blueprints into the editor using the same compiler that powers `foundation check`, so diagnostics in the editor never drift from the CLI (see the [Editor Support guide](./docs/blueprints/editor-support.md)):

- **In-Tree Language Server (`foundation lsp`)**: LSP 3.17 over stdio, launched as a CLI subcommand with no separate daemon or network listener.
- **Live Diagnostics**: compiler error/warning/information diagnostics published on open, edit, and save, mirroring `foundation check` codes, severities, and spans.
- **Hover, Completion & Definition**: DSL catalog documentation and workspace-declared symbols with inferred types; context-aware keyword/type/trait completions; member completion after `value.`; and cross-file go-to-definition for declarations and `let`/`for` locals.
- **Workspace Symbol Index**: a memoized declaration index that prunes dependency and build directories and invalidates only the edited document.
- **Editor Integrations**: companion projects — a [tree-sitter grammar](https://github.com/galihanggara68/tree-sitter-foundation), a [Zed extension](https://github.com/galihanggara68/foundation-zed), and a [VS Code / Open VSX extension](https://github.com/galihanggara68/foundation-vscode) — providing syntax highlighting and launching `foundation lsp`.

**Exit:** accurate, offline `.fdn` diagnostics, hover, completion, and navigation in any LSP-capable editor, with official syntax integrations for Zed, Neovim/Helix (tree-sitter), and VS Code.

## Phase 8 — High-Contention State Machines & Real-Time Streaming

Enable real-time, high-contention financial and event-driven workloads (e.g., collector auctions, seat reservations, inventory allocation) without dropping to handwritten scaffolding:

- **Pessimistic Row Locking (`@lock(exclusive)`)**:
  - Allow Action Pipelines to declare exclusive row locks on queries (`query single ... @lock(exclusive)`).
  - Emits `SELECT ... FOR UPDATE` in PostgreSQL (and fine-grained mutex serialization in memory).
  - Prevents OCC `409 Conflict` abort storms on high-contention bottlenecks by serializing mutations through the pipeline (e.g., chained soft-close bids).
- **Database-Backed Background Workers (`@worker`)**:
  - First-class polling worker construct in `.fdn` executing on an interval with database row locks (`SELECT ... FOR UPDATE SKIP LOCKED`).
  - Provides an honest, multi-replica batch processing and expiration mechanism (e.g., 500ms auction settlement worker) without requiring complex distributed consensus or Raft leader election.
- **Real-Time Streaming Emitters (`@stream` / SSE)**:
  - Streaming query actions emitting `text/event-stream` (Server-Sent Events) HTTP handlers.
  - Generates live subscription feeds (e.g., `GET /auctions/{id}/events` and personal user feeds) with `Last-Event-ID` offset resumption.
- **Contrib Message Broker Adapters (`contrib/go/broker/nats`, `kafka`)**:
  - Published contrib adapters extending the Phase 2 Outbox relay with at-least-once streaming publishers (NATS JetStream, Kafka, RabbitMQ).
- **Deep Modular Monolith Architecture Target**:
  - Support multi-module Blueprint declarations (`module Trading`, `module Catalog`, `module Identity`) compiling into distinct internal domain packages with isolated schemas/roles inside a single high-performance binary.

**Exit:** an end-to-end real-time collector auction system (`auction-go`) with serialized proxy bidding, 500ms settlement workers, SSE price streams, and JetStream outbox relay compiles and runs entirely through Foundation.

---

## Explicitly deferred

- **CQRS** — until a real adopter need; the glossary now names the patterns that actually exist (Hexagonal, Layered CRUD, Clean).
- **`@cron`** — until an honest HA scheduler story (leader election), not an emitted ticker; transactional background tasks use `@worker` with database row locks (`SKIP LOCKED`).
- **Offset pagination** — never; keyset cursors are the only emitted pagination (ADR [0008](./docs/adr/0008-deterministic-query-many.md)).
- **Broker event delivery** — the outbox table plus in-process replay is the contract; broker adapters arrive in Phase 8 as contrib libraries.
- **Secure-by-default flip** — decided at the 1.0 gate (ADR [0006](./docs/adr/0006-actor-resolver-port-with-contrib-adapters.md)).

**1.0** = Phases 0–5 landed, the versioning contract proven through at least one breaking-change cycle, and the secure-by-default decision made. Phases 6–8 open the pluggable generator ecosystem, editor tooling, broker adapters, and the real-time high-contention engine post-1.0.
