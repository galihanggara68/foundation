# Plugin Developer Tooling, Conformance, and Chain Contracts

## Context

The Foundation compiler relies on external generator plugins to produce target codebases across diverse architectures and languages.
Production plugin implementations expose nine areas of friction and divergence:

1. **Scaffolding Absence**:
   Authors set up new plugins by hand.
   This practice causes divergence in directory structure, lint configurations, Makefile targets, and testing standards.

2. **Wire Fixture Divergence**:
   Plugins test handlers against hand-crafted synthetic wire envelopes (`GenerationRequest`).
   These hand-crafted fixtures silently drift as the compiler introduces new platform facts, storage plans, and protocol fields.

3. **Incomplete Upstream Test Harness**:
   `pkg/emitterkit/testing` provides basic scalar assertions only.
   It lacks golden directory tree assertions, smoke test runners for real build execution, and full wire request fixture loaders.

4. **Inconsistent Model Debugging and Standard Output Pollution**:
   Plugin developers require an inspection seam for intermediate domain models before rendering templates.
   Ad-hoc implementations of `-dump-model` flags risk printing debugging text to standard output, which corrupts the wire protocol response JSON.

5. **Lack of Conformance Automation**:
   Operators verify plugin binaries through full end-to-end compiler builds.
   No standalone conformance tool validates protocol compliance, determinism, malformed request handling, and output stream purity in isolation.

6. **Fragile Pipeline Chaining**:
   Chained plugins mutate upstream files through raw substring search and replace.
   Unannounced whitespace or formatting changes in upstream emitters break chained generators without compiler feedback.

7. **Diagnostic Namespace Collisions**:
   Plugins invent custom diagnostic codes without prefix reservations.
   Two independent plugins in a multi-generator pipeline can emit identical diagnostic code prefixes, creating collision and confusion in build reports.

8. **Incomplete JSON Schema and Non-Go Ecosystem Gap**:
   `SemanticIR` schema properties remain unconstrained objects.
   Downstream authors writing plugins in Python or TypeScript cannot generate typed data models automatically.

9. **Unmaterialized Contract Commitments**:
   Implied capabilities (such as event dispatcher ports), platform third-party dependency version catalogues (BOM), and collection ordering invariants remain informal.

---

## Decision

1. **Provide Plugin Scaffolding via CLI**:
   Add `foundation plugin new <name>` to the Foundation CLI.
   Use embedded templates (`embed.FS`) to generate complete plugin repositories with standard project layouts, linters, Makefiles, and test suites.

2. **Provide Wire Fixture Capture via CLI**:
   Add `foundation capture --dir <path> [--target <name>]` to the Foundation CLI.
   Compile blueprints and serialize authoritative, complete `GenerationRequest` wire payloads directly into plugin fixture directories.

3. **Expand `pkg/emitterkit/testing`**:
   Add `AssertGoldenTree` to verify entire directory trees against golden fixtures on disk with unified diff output.
   Add `RunSmokeSuite` to run real `go build` and `go test` commands in isolated temporary directory sandboxes.
   Add `LoadWireFixture` to decode full `GenerationRequest` JSON fixtures.

4. **Standardize `-dump-model` Seam in `pkg/emitterkit`**:
   Add `RunPluginWithFlags` and the `WithDumpModel` option decorator in `pkg/emitterkit`.
   Intercept `-dump-model`, invoke the model translation callback, emit formatted JSON to standard output, and exit cleanly with code 0 before code generation begins.

5. **Provide Automated Conformance Checker via CLI**:
   Add `foundation plugin doctor <binary>` to execute five synthetic protocol probes:
   version echo, byte-level determinism, malformed request error reporting (`FDN0701`), standard output purity, and process exit code discipline.

6. **Establish Stable Extension Anchors and Chaining Helpers**:
   Emit standard comment anchors (`// foundation:anchor:<name>`) at key extension points in built-in emitters.
   Provide package `pkg/emitterkit/chain` with `InsertBefore`, `InsertAfter`, and `ReplaceAnchor` functions that return explicit errors when anchors are missing.

7. **Enforce Diagnostic Prefix Reservation**:
   Add `diagnostic_prefix` configuration to targets in `foundation.build.yaml`.
   Reserve prefix `FDN` for compiler diagnostics and emit diagnostic `FDN0603` if multiple plugins claim identical prefixes.

8. **Complete `SemanticIR` Schema and Provide Reference Client SDKs**:
   Complete property definitions in `pkg/compiler/internal/protocol/schema.go`.
   Provide lightweight client libraries in `contrib/emitterkit-py` and `contrib/emitterkit-ts`.

9. **Resolve Open Specification 05 Contracts**:
   Materialize implied ports (`EventDispatcher`) directly in `SemanticIR.Slots` with `Derived: true`.
   Publish canonical third-party language versions in `platform.Info`.
   Document canonical ordering rules for all IR collections.

---

## Consequences

### Positive
- Downstream plugin authors can scaffold production-grade plugins in one command.
- Fixture envelopes remain identical to real compiler wire requests.
- Test suites inherit golden tree assertions and smoke build runners without custom harness code.
- Intermediate model inspection becomes a standard, safe capability that cannot corrupt protocol streams.
- CI pipelines can validate external plugin binaries with the conformance doctor.
- Chained plugins mutate upstream files safely through stable anchors rather than fragile string matches.
- Pipeline diagnostic collisions are caught during validation.

### Negative / Trade-offs
- The compiler binary size increases slightly due to embedded scaffolding templates.
- Built-in emitter templates must preserve extension anchor comments across formatting adjustments.
- Maintaining reference client libraries in Python and TypeScript adds cross-language test maintenance to the repository.
