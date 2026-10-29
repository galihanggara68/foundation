# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.3.0] - 2026-10-29

### Added
- **In-Tree Language Server (`foundation lsp`)**: LSP 3.17 implementation over stdio, launched by the Zed extension.
  - Live compiler diagnostics (error/warning/information) on open, edit, and save.
  - Context-aware completions for keywords, scalar types, traits, and pipeline snippets, with correct snippet tab stops.
  - Hover documentation for the DSL catalog (constructs, traits, scalar types) and for symbols declared in the workspace.
  - Go-to-definition for entities, values, enums, events, errors, policies, slots, actions, scenarios, their fields/parameters/methods, and `let`/`for` locals — resolved across files.
- **Symbol Index**: workspace declaration index shared by hover and definition.

### Changed
- **Workspace scanning** prunes dependency and build directories (`node_modules`, `vendor`, `dist`, `build`, `out`, `target`, `tmp`, `coverage`).
- **Caching**: gathered sources and per-document symbol indexes are memoized and invalidated on document changes, so repeated hover/definition requests are sub-millisecond and an edit re-indexes only the changed file.

### Fixed
- Completion items that use LSP snippet syntax now declare `insertTextFormat: Snippet`, so clients insert the template with tab stops instead of literal `${1:...}` placeholders.

---

## [0.2.0] - 2026-10-27

### Added
- **Pluggable Generator Pipelines (`foundation-gen-*`)**:
  - Subprocess wire protocol over standard I/O (JSON-RPC) enabling language-agnostic code generators (ADRs 0011 & 0014).
  - Multi-plugin execution pipelines (`generators: [...]`) supporting `scaffold: builtin` (pre-populating 90% scaffolding) and `scaffold: none`.
  - Reusable `pkg/emitterkit` Go SDK for custom plugin authoring.
  - Three production reference generator plugins:
    - `foundation-gen-rich-ddd`: Emits Go Aggregate Roots with encapsulated business invariant methods.
    - `foundation-gen-otel`: Injects OpenTelemetry distributed tracing middleware.
    - `foundation-gen-python-fastapi`: Emits runnable Python FastAPI HTTP services with automated `pytest` test runner.
- **Categorized Test Architecture & Battle-Testing**:
  - Reorganized test suites into `test/e2e/`, `test/security/`, `test/durability/`, and `test/scale/`.
  - Live HTTP server durability tests under concurrent load, simulated socket drops, and graceful drain.
  - Linear $O(N)$ compiler scaling tests (~733 entities/sec; 500 entities in 0.68s).
  - Parser and JSON-RPC protocol fuzzing suites (125k+ mutations).
- **Developer Automation**:
  - Production `Makefile` with categorized test targets, formatting checks, and file size discipline verification (`make verify`).
  - Automated cross-platform binary release pipeline (`.github/workflows/release.yml`) for Linux, macOS, and Windows.
- **Open-Source Governance & Community**:
  - Official Apache License, Version 2.0 (`LICENSE`).
  - Contributor guidelines (`CONTRIBUTING.md`) and pre-commit verification standards.
  - Responsible disclosure policy (`SECURITY.md`).

### Fixed & Security
- **Security Sandboxing (FDN-SEC-001)**: Prevented external generator plugins from executing directory traversal (zip-slip) escapes via `safeDiskPath` path confinement.
- **Manifest Hygiene (FDN-SEC-002)**: Confined stale file pruning to `outDir` to prevent arbitrary file deletion outside the build directory.
- **Subprocess Isolation**: Enforced execution timeouts, process group signal termination (`SIGKILL` on timeout), and bounded stderr/stdout streams.

---

## [0.1.0] - 2026-10-26

### Added
- **Core Blueprint Compiler**: 100% offline, deterministic `.fdn` compiler with zero third-party dependencies in `pkg/compiler`.
- **Multi-Target Backend Generation**:
  - Go Hexagonal architecture (Chi router, `database/sql`, PostgreSQL migrations).
  - TypeScript Clean architecture (Express, `pg`, transactional repositories).
- **Tactical Durability Traits**:
  - `@transactional`: Atomic database transaction management.
  - `@idempotent(key: ...)`: Idempotency enforcement with lock-storm prevention.
  - `@outbox`: Transactional outbox table generation and background `OutboxRelayWorker` with `FOR UPDATE SKIP LOCKED`.
  - `@version`: Optimistic Concurrency Control (OCC) with conditional SQL updates and HTTP 409 Conflict handling.
  - `@cache` and `@retry`: Declarative caching and fault-tolerant retry pipelines.
- **Query Many & Keyset Pagination**:
  - Declarative cursor-paginated queries (`query many`) with compile-time unique order validation, mandatory limits, and predicate comparison operators.
- **Actor Authentication & Policies**:
  - Built-in actor resolution port and declarative security policies (`actor`, `jwt-oidc`).
- **Declarative BDD Scenarios**:
  - Native scenario testing syntax (`given`, `when`, `expect`) compiling to native Go and TypeScript test suites.
