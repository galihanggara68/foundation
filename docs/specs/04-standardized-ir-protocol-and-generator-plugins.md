---
status: ready-for-agent
---

# Specification 04: Standardized IR Protocol & Pluggable Generator Plugins (Phase 6)

## Problem Statement

Foundation currently hardcodes its code generation targets (Go Hexagonal, Go Layered, TypeScript Clean, Go SDK, TypeScript SDK) directly inside the compiler binary:
1. **Language Barrier**: Supporting new programming languages (e.g. Rust, Python, Java, C#) requires submitting pull requests directly to Foundation's Go compiler codebase and expanding internal dependencies.
2. **Architectural Rigidity**: Teams cannot customize specific architectural concerns (e.g. Rich DDD Aggregate Roots with private fields, custom HTTP router middleware, company-wide OpenTelemetry tracers, or custom ID generators like ULID/UUIDv7) without forking the compiler or rewriting hundreds of lines of boilerplate.
3. **Monolithic Emitter Coupling**: All emitter improvements risk introducing regressions across unrelated targets because there is no formal, decoupled protocol boundary separating the validated Semantic IR / Architecture IR from the emission layer.
4. **Toolchain Friction**: External language targets cannot integrate with `foundation test` because test execution logic is hardcoded to `go test` and `node --test`.

## Solution

Deliver Phase 6 of the Foundation Roadmap by implementing a **Standardized Wire Protocol & Subprocess Generator Plugin Runner** ([ADR 0011](../adr/0011-pluggable-emitter-protocol.md), [ADR 0014](../adr/0014-multi-generator-plugin-pipelines.md)):
1. **JSON Schema Wire Protocol**:
   - Publish a versioned JSON Schema defining `GenerationRequest` (containing `protocol_version`, `workspace_root`, `target`, `semantic_ir`, optional `architecture_ir`, and `files`) and `GenerationResponse` (containing `files`, `stubs`, `diagnostics`, and `test_command`).
2. **Hybrid Plugin Discovery & Execution**:
   - The compiler resolves plugin executables via a prioritized 3-tier lookup:
     1. Direct path (`./bin/my-gen` or `/opt/bin/my-gen`).
     2. Workspace local (`./node_modules/.bin/foundation-gen-<name>` or `./bin/foundation-gen-<name>`).
     3. System `$PATH` (`foundation-gen-<name>`).
   - Plugins run in process isolation with configurable timeouts (default 30s) and structured stderr diagnostics (`FDN0700`, `FDN0701`).
   - Target configuration options are passed as CLI flags (`--key=value`) and mirrored in the request JSON payload.
3. **Sequential Generator Pipelines & Scaffolding Pre-population**:
   - Support multiple chained plugins (`generators: [ "gen-rich-ddd", "acme-gen-telemetry" ]`) where each generator transforms or appends to in-memory virtual files.
   - Support `scaffold: builtin` (pre-populating the standard 90% scaffolding before invoking plugins) and `scaffold: none` (clean slate for new languages).
   - Detect and warn on file overwrites in generator chains (`FDN0702`).
4. **Manifest Engine Authority & Stub Protection**:
   - External plugins never write to disk directly; all emissions pass through Foundation's Manifest Engine.
   - Separate `files` (tracked and pruned via `.foundation-manifest.json`) and `stubs` (create-if-missing, protected from overwrites and manifest pruning).
5. **Pluggable Testing Seam (`foundation test`)**:
   - Execute native test commands (`test_command`) in target output directories with configurable timeouts and structured failure reporting (`FDN0600`, `FDN0602`).
6. **Go `emitterkit` SDK**:
   - Expose reusable Go library packages for building plugins, providing standard scaffolding emitters and statement lowerers.

## User Stories

### Configuration & Discovery
1. As a platform engineer, I want to add `generator: rust` or `generator: ./tools/gen-ddd` in `foundation.build.yaml`, so that external generators can be invoked during `foundation build`.
2. As a developer, I want target options passed as CLI flags and JSON fields, so that I can easily debug plugins from the terminal.
3. As a CI engineer, I want hanging or crashed plugins terminated within a bounded deadline (30s) with clear diagnostics (`FDN0700`, `FDN0701`), so that builds never hang indefinitely.

### Architecture Customization & Pipelines
4. As a DDD practitioner, I want to use `generators: [ foundation-gen-rich-ddd ]` with `scaffold: builtin`, so that I get Rich DDD Aggregate Roots without having to rewrite HTTP routing or SQL repositories.
5. As an enterprise architect, I want to chain a custom telemetry generator after a rich domain generator, so that company-wide tracing headers are injected automatically.
6. As a developer, I want a warning (`FDN0702`) if a downstream generator overwrites a file emitted by an upstream generator, so that accidental collisions are caught early.

### Filesystem & Testing Hygiene
7. As a developer, I want my handwritten Slot implementations protected from plugin overwrites, so that subsequent builds never clobber custom logic.
8. As a polyglot developer, I want `foundation test` to invoke `cargo test` or `pytest` when configured, so that multi-language projects test seamlessly with a single command.

## Implementation Plan

### Ticket 01: Standardized Wire Protocol & JSON Schema Definitions
- Define `GenerationRequest` and `GenerationResponse` Go structs in `pkg/compiler/internal/protocol/`.
- Export serialized JSON schemas and version tags (`protocol_version: 1`).
- Support `files` (tracked) and `stubs` (create-if-missing) in `GenerationResponse`.

### Ticket 02: Hybrid Plugin Discovery, Invocation & Subprocess Runner
- Implement plugin resolver in `pkg/compiler/internal/plugin/`:
  - Direct path check (`./`, `../`, `/`).
  - Workspace-local binary check (`node_modules/.bin`, `./bin`).
  - System `$PATH` lookup (`foundation-gen-<name>`).
- Subprocess runner executing plugins over standard I/O with JSON streaming.
- Timeout enforcement (default 30s) with process group termination (`FDN0700`).
- Stderr capture and non-zero exit code mapping (`FDN0701`).

### Ticket 03: Sequential Generator Pipelines & Pre-population
- Support `generators:` list and `generator:` scalar in `BuildProfile`.
- Implement `scaffold: builtin` and `scaffold: none` semantics.
- Virtual file chaining: pass previous plugin's emitted files into `GenerationRequest.files`.
- File collision detection emitting warning `FDN0702`.
- Normalization of plugin diagnostics into native compiler diagnostics.

### Ticket 04: Pluggable Testing Seam (`foundation test`)
- Extend `runner.go` to support custom `test_command` execution.
- Hybrid resolution: use plugin `GenerationResponse.test_command` by default, allow override in `foundation.build.yaml`.
- Enforce test execution timeout (default 60s) with `FDN0602`.

### Ticket 05: Go `emitterkit` SDK & Rich DDD Example Plugin
- Publish `pkg/emitterkit/` providing helper functions for running plugins and calling built-in scaffolding emitters.
- Build an end-to-end reference plugin (`foundation-gen-rich-ddd` or `foundation-gen-custom-middleware`) verifying partial overrides.
- Integration tests verifying full wire compatibility, manifest tracking, conflict detection, and `foundation test` execution.

## Testing Decisions & Agreed Seams
- **Seam 1 (Protocol Seam)**: Unit tests verify byte-for-byte JSON serialization and deserialization of `GenerationRequest` and `GenerationResponse`.
- **Seam 2 (Runner Seam)**: Mock subprocess scripts testing timeout enforcement, non-zero exit codes, stderr capture, and CLI argument passing.
- **Seam 3 (Pipeline Seam)**: Multi-plugin chaining tests asserting virtual file passing, collision warnings (`FDN0702`), and manifest reconciliation.
- **Seam 4 (End-to-End Seam)**: Running a real standalone plugin binary in a test project, verifying disk outputs, manifest hashing, and `foundation test` execution.

## Out of Scope
- Direct runtime LLM code synthesis in plugins.
- Dynamic network socket or gRPC plugin daemons (standard I/O subprocess model is strictly maintained per ADR 0011).
