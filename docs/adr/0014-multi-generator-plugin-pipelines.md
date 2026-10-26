# Multi-Generator Plugin Pipelines and Sequential Filter Chains

Foundation extends the external generator plugin protocol (ADR 0011) to support **multi-generator plugin pipelines (sequential filter chains)** and **selective scaffolding pre-population**, enabling developers to compose specialized generators (such as Rich DDD aggregates, custom HTTP middleware stacks, and telemetry injectors) without rebuilding common scaffolding from scratch.

## Context

Phase 6 enables polyglot targets (e.g. Rust, Python) and custom architectures via external generator executables communicating over standard I/O via a versioned JSON protocol. However, developers frequently want to customize only a specific architectural slice—such as generating Rich DDD Aggregate Roots with private fields and business methods—while preserving Foundation's standard Chi/Express HTTP routing, PostgreSQL migrations, transactional outbox relays, and client SDKs. Requiring every custom generator to author the full 100% stack from scratch leads to massive duplication, drift, and maintenance overhead.

## Decision

1. **Sequential Filter Chain**:
   A target in `foundation.build.yaml` may declare a list of generators (`generators: [ "gen-rich-ddd", "acme-gen-telemetry" ]`) or a single `generator: "name"`.
   The compiler executes plugins sequentially in process isolation over standard I/O (`stdin`/`stdout`).

2. **Scaffolding Pre-population (`scaffold: builtin` vs `scaffold: none`)**:
   - When `scaffold: builtin` (default when `architecture` and `language` match a built-in target), the Foundation compiler executes the built-in target emitter in memory first, pre-populating `GenerationRequest.files` with the 90% production plumbing.
   - When `scaffold: none`, `GenerationRequest.files` starts empty, providing a clean slate for brand new language targets (Rust, Python).

3. **Virtual File Transmission & Collision Detection**:
   Each plugin receives the current virtual file map in `GenerationRequest.files`, transforms or appends files, and returns `GenerationResponse.files`.
   If a downstream generator mutates a file previously emitted or modified by an upstream generator, the compiler emits a non-fatal warning diagnostic `FDN0702` to alert developers to potential unintended overwrites.

4. **Dual Lifecycle Output (`files` vs `stubs`)**:
   `GenerationResponse` distinguishes between:
   - `files`: Tracked files deterministically managed by `.foundation-manifest.json` (overwritten on build, pruned when actions/values are removed).
   - `stubs`: Create-if-missing files (e.g., Slot implementations, config starters) written once to disk and permanently protected from overwrites and pruning.

5. **External Toolchain Testing Seam**:
   `foundation test` delegates test execution for custom targets to a native test command (`test_command`), defaulting to the plugin's `GenerationResponse.test_command` (e.g. `cargo test`, `pytest`) and overridable in `foundation.build.yaml`.

## Consequences

- Teams can mix and match specialized open-source and corporate plugins (e.g. Rich DDD + OpenTelemetry + Datadog) without rewriting database or routing layers.
- The Generation Manifest engine retains exclusive control of the disk; plugins never touch the filesystem directly.
- Overwrite conflicts in plugin chains are detected and reported before files reach the disk.
