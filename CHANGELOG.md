# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0] - 2026-10-27

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
- **Pluggable Generator Protocol**:
  - JSON-RPC over stdin/stdout wire protocol supporting any external programming language.
  - Multi-generator sequential pipelines (`generators: [...]`) supporting `scaffold: builtin` and `scaffold: none`.
  - Reference plugins for Rich DDD Aggregate Roots, OpenTelemetry distributed tracing, and Python FastAPI.
- **Query Many & Keyset Pagination**:
  - Declarative cursor-paginated queries (`query many`) with compile-time unique order validation, mandatory limits, and predicate comparison operators.
- **Actor Authentication & Policies**:
  - Built-in actor resolution port and declarative security policies (`actor`, `jwt-oidc`).
- **Declarative BDD Scenarios**:
  - Native scenario testing syntax (`given`, `when`, `expect`) compiling to native Go and TypeScript test suites.
- **Developer & CI Infrastructure**:
  - Categorized test architecture: `test/e2e/`, `test/security/`, `test/durability/`, and `test/scale/`.
  - Automated `Makefile` with `make verify` pre-commit gate (line-check, fmt-check, vet, test-unit).
  - Automated cross-platform binary releases for Linux, macOS, and Windows.
  - Official Apache 2.0 License, `CONTRIBUTING.md`, and `SECURITY.md`.

### Security
- Host filesystem path confinement (`safeDiskPath`) preventing zip-slip path traversal and arbitrary file deletion.
- Subprocess plugin sandboxing with execution timeouts, process group signal termination, and output bomb rejection.
