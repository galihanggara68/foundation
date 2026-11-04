# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.8.0] - 2026-11-04

### Added
- **Composite entity constraints and trait list arguments** ([ADR 0020](./docs/adr/0020-composite-entity-traits-and-list-arguments.md)):
  - Grammar: Bracketed string list syntax `["col1", "col2"]` allows multi-value arguments in trait declarations.
  - Platform registry permits `TraitOnEntity` for `@index` and `@unique`.
  - The `@index` trait on entities accepts a `fields` list and an optional `name` argument for custom database index names.
  - Entities permit multiple composite `@index` and `@unique` annotations with distinct column sets.
  - `FDN0203`: The compiler returns an error when a composite trait references a field that does not exist on the target entity.
  - `FDN0226`: The compiler warns when a composite entity trait supplies fewer than two fields. The compiler also warns when field-level `@index` or `@unique` traits declare arguments.
- **Multi-write transaction invariant analysis** ([ADR 0021](./docs/adr/0021-path-sensitive-mutation-invariants-and-dead-code-detection.md)):
  - `FDN0227`: Path-sensitive static analysis warns when an action performs multiple mutation steps (`create`, `update`, `delete`) without the `@transactional` trait.
- **Dead code and unused variable detection** ([ADR 0021](./docs/adr/0021-path-sensitive-mutation-invariants-and-dead-code-detection.md)):
  - `FDN0228`: An AST analysis pass warns when an action declares a `let` variable or parameter that is never referenced in subsequent expressions. Authors prefix unused identifiers with an underscore (`_`) to suppress the warning.

---

## [0.7.0] - 2026-11-02

### Added
- **Trait namespaces and trait validation** ([ADR 0019](./docs/adr/0019-trait-namespaces-and-validation.md)):
  - Grammar: `@ns:name` syntax allows downstream code generators and third-party tooling to embed metadata annotations with the ignore guarantee (the compiler never judges non-reserved namespaced traits).
  - `FDN0224`: an un-namespaced trait outside the compiler's standard vocabulary produces a compile warning.
  - `FDN0225`: the `foundation` namespace is reserved for compiler-owned features; writing `@foundation:...` is a compile error.
  - `FDN0226`: an argument problem on a standard trait (unknown argument, missing required argument, or non-numeric value for numeric arguments) produces a compile warning.
  - Central platform traits registry (`platform/traits.go`) with automated drift prevention for LSP hover and completions.
- `FDN0222`: an `@http` route path segment with no matching action parameter is a compile error. Previously such a blueprint validated and the emitted server routed the segment while never reading it.
- `FDN0223`: a trait written ahead of a `slot` declaration is an error. `SlotDecl` has no field for it, so it was silently discarded.

### Changed
- **Behaviour change:** a trait binds to the declaration that follows it ([ADR 0018](./docs/adr/0018-trait-attachment.md)). The slot parser's post-signature loop could not tell a trailing trait from the next method's leading trait, so each slot method absorbed its neighbour's annotations and the last kept none. A trailing trait is no longer accepted.
- **Behaviour change:** a reserved error name is legal for all seven kinds. `error Conflict` and `error Unauthenticated` failed with `FDN0200` while the other five names only warned; the shipped error now shadows the declaration for all seven, as [ADR 0016](./docs/adr/0016-error-vocabulary-ownership.md) always said.
- A list of Values, enums or scalars now emits a value slice. The result field was hard-coded to a pointer slice, so `-> []Money` emitted Go that did not compile.

### Fixed
- Slot method traits were associated with the wrong method, which made `FDN0213` (calling an `@external` method inside `@transactional`) fire on a safe call and stay silent on the genuinely external one.
- An empty list literal emitted `[]domain.{}` and failed the gofmt step with `FDN0500`. It now renders as `nil`.
- The reference pages taught `slot PaymentGateway @external {`, a form the parser has never accepted.

---

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
