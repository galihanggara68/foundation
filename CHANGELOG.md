# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.6.0] - 2026-10-29

### Added
- **TypeScript backends honour error posture** ([ADR 0016](./docs/adr/0016-error-vocabulary-ownership.md)): a `technical` error is logged server-side and answered with a generic body, so an internal diagnostic no longer reaches a caller. Generated `DomainError`s carry the `status` and `posture` the compiler resolved, plus the declared per-locale `messages`, matching the Go backend.
- Generated Go and TypeScript SDKs declare an error type for every standard kind, including the previously absent `UnauthorizedError` and `InternalError`.

### Changed
- **Behaviour change:** a TypeScript backend no longer returns a `technical` error's message — a caller sees `code` and `kind` only. Business errors still return their message and locales, and declared `status:`/`posture:` now reach a TypeScript caller instead of being ignored.
- `tsclean`, `gosdk` and `tssdk` read the compiler's error vocabulary rather than keeping copies, so a status, posture or kind-name change lands in one place.

### Fixed
- A generated SDK could reference an `InternalError` type it never declared, for any domain error of kind `internal`.

---

## [0.5.0] - 2026-10-29

### Added
- **`jwt-oidc` verifies tokens** ([ADR 0017](./docs/adr/0017-jwt-verification-contract.md)): the Bearer token's signature is checked against the key set at `AUTH_JWKS_URL` before any claim is read, in both the Go and TypeScript runtimes. `RS256` and `ES256` are accepted, `exp` is required, `nbf` is honoured and `iss`/`aud` are enforced when configured. Neither runtime gains a dependency.
- `AUTH_JWKS_URL` joins `AUTH_ISSUER`/`AUTH_AUDIENCE` as adapter configuration, so one generated build runs against different identity providers.

### Changed
- **Behaviour change:** a `jwt-oidc` deployment with no `AUTH_JWKS_URL` now answers 401 instead of trusting unverified tokens, and a token with `alg: none`, an HMAC algorithm, a bad signature or an expired claim is refused. Set the variable before deploying.

### Fixed
- A TypeScript backend built with `authn: jwt-oidc` no longer trusts `X-Actor-*` headers: the emitter's Postgres container honours the configured adapter, which it previously ignored.
- `docs/blueprints/build-profile.md` documented a `trusted-headers` adapter the compiler never accepted — the value is `dev-headers` — and now documents the adapter's environment variables.

---

## [0.4.0] - 2026-10-29

### Added
- **Compiler-Owned Standard Error Vocabulary** ([ADR 0016](./docs/adr/0016-error-vocabulary-ownership.md)): each error kind's default HTTP status and posture now live in one compiler table, and every declared error carries the `status`, `posture` and per-locale `messages` the compiler resolved for it.
  - Grammar: `status: <int>`, `posture: business|technical`, and a `messages: { <locale>: "<text>" }` block; `message:` stays the fallback text.
  - A declared `status` or `posture` overrides the kind's default; a status outside 100–599 is `FDN0220`, an unknown posture `FDN0221`, and a duplicate locale `FDN0108`.
  - `FDN0219` warns when a declared error reuses a name the vocabulary owns (`NotFound`, `Conflict`, …), which risks the platform shadowing it.
  - The `ErrorDecl` shape is published in the generation protocol schema, additively: documents written before it still decode and validate.
- **Closed Field-Kind Taxonomy** ([ADR 0015](./docs/adr/0015-closed-field-kind-taxonomy.md)): `FieldDecl` carries a compiler-validated `Kind`, and parameter and slot-return positions classify as well, so an emitter maps types as a total function instead of matching an open type string; a type the compiler cannot classify is `FDN0218`.
- **Platform Section in the Generation Request**: `GenerationRequest.platform` carries compiler-owned policy, starting with pagination defaults, so emitters consume platform decisions instead of inventing them.

### Changed
- Generated Go services answer with the status and posture the compiler resolved for a declared error, falling back to the kind's policy for an error application code builds; a `technical` message is logged and masked rather than returned to a caller.
- `foundation check`, `build`, `ir` and `test` now fail only on errors: a warning is reported and the command still succeeds.
- The error reference in `declarations.md` documents `status:`, `posture:` and `messages:`, and both it and `http-transport.md` now list the full kind set.

---

## [0.3.0] - 2026-10-29

### Added
- **In-Tree Language Server (`foundation lsp`)**: LSP 3.17 implementation over stdio, launched by editor clients.
  - Live compiler diagnostics (error/warning/information) on open, edit, and save.
  - Context-aware completions for keywords, scalar types, traits, and pipeline snippets, with correct snippet tab stops.
  - Hover documentation for the DSL catalog (constructs, traits, scalar types) and for symbols declared in the workspace.
  - Go-to-definition for entities, values, enums, events, errors, policies, slots, actions, scenarios, their fields/parameters/methods, and `let`/`for` locals — resolved across files.
- **Symbol Index**: workspace declaration index shared by hover and definition.
- **Editor Integrations**: companion projects providing syntax support — `tree-sitter-foundation` (grammar for Zed, Neovim, Helix), `foundation-zed`, and `foundation-vscode` (VS Code / Open VSX).

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
