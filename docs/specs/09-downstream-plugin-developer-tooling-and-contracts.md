---
status: ready-for-agent
---

# Specification 09: Downstream Plugin Tooling, Conformance, and SDK Ergonomics

ADR basis: extends [ADR 0011](../adr/0011-pluggable-emitter-protocol.md), [ADR 0014](../adr/0014-multi-generator-plugin-pipelines.md), [ADR 0016](../adr/0016-error-vocabulary-ownership.md), [ADR 0022](../adr/0022-generator-protocol-integrity-and-storage-emission.md), and [ADR 0023](../adr/0023-platform-facts-diagnostic-registry-and-protocol-evolution.md).

## Problem Statement

The implementation of production external generator plugins reveals nine ergonomic, testing, and contract gaps in the Foundation plugin ecosystem:

1. **Absence of Standard Plugin Scaffolding**:
   Developers construct new generator plugins by copying reference implementations manually.
   This manual setup causes boilerplates, Makefile rules, lint configurations, and test layouts to diverge across plugin repositories.

2. **Manual and Fragile Fixture Envelope Creation**:
   Downstream test suites must construct synthetic wire request envelopes (`GenerationRequest`).
   Plugin authors currently write manual shell scripts or `jq` pipelines to construct these payloads.
   Whenever the compiler adds new platform facts or protocol fields, synthetic fixtures silently diverge from real compiler payloads.

3. **Incomplete Plugin Testing Harness**:
   The current testing package `pkg/emitterkit/testing` supports only single-file assertions and basic function determinism.
   It lacks tree golden assertions, smoke test runners for generated code compilation, and full wire request loaders.
   Downstream plugin authors must build private test harnesses to cover these gaps.

4. **Ad-Hoc Model Inspection and Standard Output Contamination**:
   Plugin developers require a dedicated debug seam to inspect intermediate models before template rendering.
   External plugins implement custom `-dump-model` CLI flags independently.
   Poorly disciplined flag implementations print log messages to standard output, which corrupts the wire protocol response document.

5. **Lack of Automated Plugin Conformance Verification**:
   Operators and authors verify external plugin compliance by running an end-to-end compiler build.
   No standalone utility validates whether an external binary adheres to protocol versioning, determinism, diagnostic formats, and standard output purity.

6. **Fragile Code Mutation in Chained Pipelines**:
   Pipeline plugins mutate upstream emitted source files via raw string replacement (for example, wrapping router invocations in `cmd/foundation-gen-otel`).
   Small formatting adjustments in upstream code silently break these string replacements without raising compilation errors.

7. **Unreserved Plugin Diagnostic Namespaces**:
   Plugins create custom diagnostic codes without central prefix coordination.
   Two independent plugins can emit identical diagnostic code prefixes (such as `CUSTOM0101`), which causes ambiguity and collision in aggregated build reports.

8. **Incomplete SemanticIR Schema and Missing Non-Go SDKs**:
   The published JSON Schema marks complex IR declarations as unconstrained objects (`"type": "object"`).
   Code generation tools cannot synthesize strongly typed client models for non-Go environments.
   Python and TypeScript plugin authors must write raw JSON decoders and error formatters by hand.

9. **Unmaterialized Compiler Implications and Unstated Determinism Guarantees**:
   The compiler leaves implied ports (such as event dispatchers) for emitters to synthesize independently.
   Furthermore, dependency version sets (BOM) remain decentralized, and IR collection ordering guarantees remain informal.

---

## Solution

This specification resolves these gaps across four functional areas:

1. **Scaffolding and Capture CLI**:
   - Provide `foundation plugin new <name>` to generate complete, standard plugin projects from embedded templates.
   - Provide `foundation capture --dir <path> [--target <name>]` to export full, authoritative `GenerationRequest` wire envelopes.

2. **Unified Testing Harness and Conformance Doctor**:
   - Expand `pkg/emitterkit/testing` with tree golden assertions, smoke runners (`go build`, `go test`), and wire fixture decoders.
   - Provide `foundation plugin doctor <binary>` to validate protocol compliance, determinism, error handling, and output cleanliness.

3. **Runtime Enhancements and Stable Pipeline Chaining**:
   - Introduce `emitterkit.RunPluginWithFlags` with standard `-dump-model` flag interception and strict standard output isolation.
   - Introduce stable extension comment anchors in built-in emitters and provide anchor manipulation helpers in `pkg/emitterkit/chain`.

4. **Ecosystem Governance and Contract Precision**:
   - Implement diagnostic namespace reservation and collision detection (`FDN0603`) in plugin pipeline manifests.
   - Complete the `SemanticIR` JSON Schema definitions and provide reference client packages for Python and TypeScript.
   - Materialize implied capabilities in the compiler, publish platform dependency versions (BOM), and document IR ordering contracts.

---

## Detailed Technical Design

### 1. Plugin Scaffolding: `foundation plugin new`

Add the `plugin new` subcommand to the Foundation CLI in `cmd/foundation/main.go`:

```bash
foundation plugin new <plugin-name> [--dir <output-dir>] [--module <go-module-path>] [--replace <foundation-path>]
```

The `--replace` flag writes a `replace` directive into the generated `go.mod`.
Plugin authors use the flag to test a plugin against a local Foundation checkout.

The command generates the following project directory structure:

```text
<plugin-name>/
├── .gitignore
├── .golangci.yml               # Enforces standard output purity
├── go.mod                      # Pinned to github.com/galihanggara68/foundation
├── main.go                     # Entry point using emitterkit.RunPluginWithFlags
├── dump_model.go               # Standard model dump callback
├── Makefile                    # test-short, smoke, lint, update-goldens
├── testdata/
│   ├── blueprints/             # .fdn sample blueprints
│   ├── requests/               # Authoritative GenerationRequest fixtures
│   ├── models/                 # Expected intermediate model JSON goldens
│   └── golden/                 # Expected emitted file trees
├── determinism_test.go         # AssertDeterministic over wire requests
├── golden_test.go              # Tree golden comparison tests
└── smoke_test.go               # Real build and test execution on emitted files
```

The implementation embeds template files inside `cmd/foundation` using `embed.FS`.
The generator replaces template tokens (`{{.PluginName}}`, `{{.ModulePath}}`, `{{.FoundationVersion}}`) during file creation.

---

### 2. Fixture Envelope Generation: `foundation capture`

Add the `capture` subcommand to `cmd/foundation/main.go`:

```bash
foundation capture --dir <blueprint-dir> [--target <target-name>] [--out <output-file>]
```

#### Workflow:
1. Parse the blueprint directory using `compiler.Check(dir)`.
2. Stop and report errors if parsing or semantic analysis yields diagnostics.
3. Lower the semantic model into architectural representations using `compiler.Lower(sem)`.
4. Construct the standard `GenerationRequest` struct:
   - Populate `ProtocolVersion` with `protocol.ProtocolVersion`.
   - Populate `Semantic` with `sem`.
   - Populate `ArchitectureIR` with the lowered architectural model, including `ArchitectureIR.Storage`.
   - Populate `Platform` with `platform.DefaultInfo()`.
   - Populate `Target` with the selected target configuration.
   - Populate `Files` with the built-in emitter output that a chained pipeline plugin receives.
5. Serialize the request via `protocol.EncodeRequest(req)`.
6. Write deterministic JSON bytes to `--out` or standard output.

---

### 3. Expanded Testing Harness: `pkg/emitterkit/testing`

Expand `pkg/emitterkit/testing/harness.go` with enterprise test assertions:

```go
package testing

import (
    "os/exec"
    "path/filepath"
    stdtesting "testing"

    "github.com/galihanggara68/foundation/pkg/compiler"
    "github.com/galihanggara68/foundation/pkg/emitterkit"
)

type TreeGoldenOptions struct {
    UpdateGoldens bool
    IgnorePaths   []string
}

// LoadWireFixture loads and decodes a complete GenerationRequest JSON fixture.
func (h *Harness) LoadWireFixture(path string) (emitterkit.GenerationRequest, error)

// AssertGoldenTree compares all emitted files against a golden file tree on disk.
func (h *Harness) AssertGoldenTree(resp emitterkit.GenerationResponse, goldenDir string, opts TreeGoldenOptions)

// RunSmokeSuite writes resp.Files to a temporary directory and executes build and test commands.
func (h *Harness) RunSmokeSuite(resp emitterkit.GenerationResponse, command string) SmokeResult

type SmokeResult struct {
    BuildSuccess bool
    TestSuccess  bool
    Output       string
    Error        error
}
```

#### Tree Golden Comparison Algorithm:
1. Walk the `goldenDir` directory and collect all expected relative file paths.
2. Collect all file keys from `resp.Files` and `resp.Stubs`.
3. If `opts.UpdateGoldens` is `true`:
   - Delete files in `goldenDir` absent from the response.
   - Overwrite all golden files with response contents and return.
4. Fail the test if file paths differ between response and disk.
5. Read each golden file and compare contents byte-for-byte with the response content.
6. Report clear unified diffs on content mismatches.

#### Smoke Suite Algorithm:
1. Create an isolated temporary directory via `os.MkdirTemp`.
2. Write all `resp.Files` and `resp.Stubs` using `emitterkit.SafeRelPath`.
3. Locate the build or test command from `resp.TestCommand` or the `command` argument.
4. Execute the command inside the temporary directory with environment variables matching the host Go toolchain.
5. Capture combined output and exit codes.
6. Clean up temporary files after test execution.

---

### 4. Plugin Conformance Checker: `foundation plugin doctor`

Add the `plugin doctor` subcommand to `cmd/foundation/main.go`:

```bash
foundation plugin doctor <path-to-binary> [--verbose]
```

The doctor runs five synthetic test probes against the plugin binary:

| Probe | Input Payload | Verification Rule | Failure Diagnostic |
|---|---|---|---|
| 1. Version Echo | Standard synthetic `GenerationRequest` | `resp.ProtocolVersion == protocol.ProtocolVersion` | `FDN0703`: Protocol version mismatch |
| 2. Byte Determinism | Run identical synthetic request twice | Byte outputs of run 1 and run 2 are identical | `FDN0704`: Non-deterministic generation |
| 3. Malformed Request | Invalid JSON string (`{malformed...`) | Response contains diagnostic `FDN0701` | `FDN0705`: Invalid malformed error handling |
| 4. Stdout Purity | Standard synthetic `GenerationRequest` | Zero bytes outside the single JSON response document | `FDN0706`: Standard output pollution detected |
| 5. Exit Code Discipline | Normal run and handled error | Binary exits with code 0 on handled errors; non-zero only on unhandled crashes | `FDN0707`: Invalid process exit posture |

The doctor reports individual probe status to standard output and exits with code 0 only when all probes succeed.

---

### 5. Standardized `-dump-model` and Plugin Runner

Update `pkg/emitterkit/plugin.go` to support runtime options:

```go
package emitterkit

type PluginOption func(*pluginConfig)

type ModelDumperFunc func(req GenerationRequest) (any, error)

type pluginConfig struct {
    dumpModelFn ModelDumperFunc
}

// WithDumpModel configures an intermediate model extraction callback.
func WithDumpModel(fn ModelDumperFunc) PluginOption {
    return func(c *pluginConfig) {
        c.dumpModelFn = fn
    }
}

// RunPluginWithFlags parses command-line flags and runs the plugin handler.
func RunPluginWithFlags(handler HandlerFunc, opts ...PluginOption) {
    // 1. Initialize configuration.
    // 2. Parse flags: -dump-model.
    // 3. If -dump-model is active:
    //    a. Decode GenerationRequest from os.Stdin.
    //    b. Execute dumpModelFn.
    //    c. Serialize intermediate model as formatted JSON to os.Stdout.
    //    d. Exit with code 0.
    // 4. If -dump-model is inactive, execute standard RunPluginStream.
}
```

This implementation centralizes command-line flag handling in `emitterkit` and guarantees that log statements do not corrupt output streams.

---

### 6. Pipeline Chaining Anchors: `pkg/emitterkit/chain`

Create package `pkg/emitterkit/chain` to manage safe modification of generated source code:

```go
package chain

import "errors"

var (
    ErrAnchorNotFound  = errors.New("pipeline anchor not found")
    ErrDuplicateAnchor = errors.New("duplicate pipeline anchor detected")
)

// Standard anchor comment format:
// // foundation:anchor:<anchor-name>

// InsertBefore finds anchorName in content and inserts snippet before the anchor comment.
func InsertBefore(content, anchorName, snippet string) (string, error)

// InsertAfter finds anchorName in content and inserts snippet after the anchor comment.
func InsertAfter(content, anchorName, snippet string) (string, error)

// ReplaceAnchor replaces the anchor comment and its block with snippet.
func ReplaceAnchor(content, anchorName, snippet string) (string, error)
```

#### Built-in Emitter Anchor Locations:
Update built-in emitters in `pkg/compiler/internal/emit/` to publish stable anchor comments:
- `// foundation:anchor:http-router-middleware`: Insertion point for HTTP middlewares.
- `// foundation:anchor:http-router-routes`: Insertion point for extra route registrations.
- `// foundation:anchor:domain-imports`: Insertion point for additional package imports.

The OpenTelemetry plugin `cmd/foundation-gen-otel` migrates from fragile string replacement to `chain.InsertBefore("http-router-middleware", ...)`.

---

### 7. Diagnostic Namespace Reservation

Extend `foundation.build.yaml` target configuration to declare diagnostic code prefixes:

```yaml
targets:
  - name: custom-backend
    generator: ./bin/foundation-gen-custom
    diagnostic_prefix: ACME
```

#### Collision Detection Rules:
1. The compiler reserves prefix `FDN` exclusively for compiler and core tool diagnostics.
2. Target generators cannot declare prefix `FDN`.
3. If two targets in the pipeline declare identical non-empty `diagnostic_prefix` values, compiler validation raises diagnostic `FDN0603`:
   - `[FDN0603] diagnostic prefix "ACME" is claimed by multiple plugins: custom-backend, telemetry-ext`.
4. The pipeline runner checks emitted diagnostic codes in `GenerationResponse.Diagnostics`.
5. If a plugin emits a diagnostic code that does not start with its declared prefix, the pipeline logs a warning diagnostic.

---

### 8. SemanticIR JSON Schema and Multi-Language SDK Contracts

#### JSON Schema Completion:
Update `pkg/compiler/internal/protocol/schema.go` to provide explicit property definitions for:
- `ir.SemanticIR`: Full property schemas for `actors`, `entities`, `values`, `actions`, `events`, `enums`, `policies`, and `slots`.
- `ir.EntityDecl`: Fields, types, kinds, traits, and physical table names.
- `ir.ActionDecl`: Steps, parameters, return types, and route bindings.
- `ir.StepDecl`: Complete discriminated union schemas for all step kinds.

#### Reference Non-Go Packages:
Provide lightweight client packages in `contrib/`:
- `contrib/emitterkit-py`: Python implementation containing `run_plugin`, `decode_request`, `encode_response`, and dataclass definitions.
- `contrib/emitterkit-ts`: TypeScript implementation containing `runPlugin`, `decodeRequest`, `encodeResponse`, and TypeScript interfaces.

---

### 9. Specification 05 Contract Resolutions

1. **Implied Capability Materialization**:
   - When a blueprint declares events, the semantic lowering stage automatically inserts an `EventDispatcher` port into `SemanticIR.Slots`.
   - The inserted slot receives flag `Derived: true`.
   - Downstream generators consume standard port definitions without custom synthesis.

2. **Platform Bill of Materials (BOM)**:
   - Expand `platform.Info` to publish canonical third-party dependency versions:
     ```go
     type DependencyVersion struct {
         Language string `json:"language"`
         Package  string `json:"package"`
         Version  string `json:"version"`
     }
     ```
   - Provide standard Go versions (for example: `pgx/v5`, `gorm`, `opentelemetry`).
   - Downstream Go generators use these versions to construct consistent `go.mod` files.

3. **Collection Ordering Guarantees**:
   - Document canonical ordering invariants across all IR collections:
     - All top-level declaration slices (`Entities`, `Actions`, `Slots`, `Enums`) remain sorted alphabetically by name.
     - `EntityDecl.Fields` maintains declaration order from the source `.fdn` blueprint.
     - `ActionDecl.Steps` maintains strict execution sequence order.
     - `StoragePlan.Tables` remains sorted alphabetically by physical table name.

---

## User Stories

1. As a plugin developer, I want to run `foundation plugin new my-plugin`, so that I receive an operational, linted plugin project with complete test harnesses immediately.
2. As a plugin author, I want to run `foundation capture --dir ./blueprints`, so that I generate real wire request fixtures without writing manual JSON scripts.
3. As a plugin test author, I want `emitterkit/testing` to compare directory golden trees, so that I can detect regressions across all emitted files in one assertion.
4. As a plugin test author, I want `emitterkit/testing` to run smoke builds and tests on generated code, so that I know emitted code compiles cleanly.
5. As a plugin author, I want `emitterkit.RunPluginWithFlags` to handle `-dump-model`, so that I can inspect intermediate models without corrupting standard output.
6. As a CI engineer, I want to run `foundation plugin doctor ./my-plugin`, so that I can verify binary protocol compliance and determinism automatically.
7. As a pipeline plugin author, I want stable anchor comments in generated code, so that my plugin can inject middleware without fragile string replacements.
8. As a platform architect, I want plugins to declare diagnostic prefixes, so that diagnostic codes from multiple generators never collide.
9. As a Python plugin developer, I want an authoritative `emitterkit-py` package, so that I can write plugins without parsing raw JSON protocols manually.

---

## Implementation Plan

### Stage 1: Testing Harness & Runtime Ergonomics
- **Ticket 01: Expanded Testing Harness (`pkg/emitterkit/testing`)**
  - Implement `AssertGoldenTree` with unified diff reporting and update flags.
  - Implement `RunSmokeSuite` with temporary directory sandbox and process execution.
  - Implement `LoadWireFixture` to decode full `GenerationRequest` JSON files.
- **Ticket 02: Model Dump Flag & Standard Plugin Runner (`pkg/emitterkit`)**
  - Implement `RunPluginWithFlags` with `-dump-model` flag interception.
  - Implement `WithDumpModel` option.
  - Add unit tests validating standard output purity during model dump.
- **Ticket 03: Pipeline Anchors & Modification Helpers (`pkg/emitterkit/chain`)**
  - Add package `pkg/emitterkit/chain` with `InsertBefore`, `InsertAfter`, and `ReplaceAnchor`.
  - Add stable anchor comments to Go hexagonal built-in emitters.
  - Refactor `cmd/foundation-gen-otel` to use anchor helpers.

### Stage 2: Developer Tooling & Conformance Verification
- **Ticket 04: Fixture Capture Command (`foundation capture`)**
  - Add `capture` subcommand to `cmd/foundation`.
  - Wire semantic compilation and architectural lowering to request envelope serialization.
  - Add tests validating captured request structure.
- **Ticket 05: Plugin Scaffolding Generator (`foundation plugin new`)**
  - Create embedded template project structure in `cmd/foundation/internal/pluginnew/`.
  - Add `plugin new` subcommand to `cmd/foundation`.
  - Add integration tests verifying generated projects compile and pass short tests.
- **Ticket 06: Plugin Conformance Doctor (`foundation plugin doctor`)**
  - Add `plugin doctor` subcommand to `cmd/foundation`.
  - Implement the five protocol probes (version echo, determinism, malformed payload, stdout purity, exit code).
  - Add unit tests running the doctor against compliant and non-compliant test binaries.

### Stage 3: Contracts, Governance, and Multi-Language
- **Ticket 07: Diagnostic Prefix Reservation & Collision Detection**
  - Add `DiagnosticPrefix` field to target definitions in `foundation.build.yaml`.
  - Add validation pass raising `FDN0603` on prefix collisions.
  - Add response validation checking declared prefix compliance.
- **Ticket 08: SemanticIR JSON Schema & Language SDKs**
  - Complete JSON schema definitions in `pkg/compiler/internal/protocol/schema.go`.
  - Implement lightweight Python client package in `contrib/emitterkit-py`.
  - Implement lightweight TypeScript client package in `contrib/emitterkit-ts`.
- **Ticket 09: Implied Capabilities, Platform BOM, and Determinism Contracts**
  - Materialize `EventDispatcher` slot in `pkg/compiler/internal/lowering/`.
  - Add `BOM` version catalogue to `platform.Info`.
  - Add explicit collection ordering documentation in generator guides.

---

## Testing Strategy and Acceptance Scenarios

### Scenario 1: Scaffolded Plugin Compilation and Test Execution
- **Given** a clean workspace
- **When** running `foundation plugin new my-emitter --module example.com/my-emitter`
- **Then** the command creates the complete plugin directory structure
- **And** running `go test ./...` inside the generated directory succeeds with zero failures.

### Scenario 2: Wire Fixture Capture Accuracy
- **Given** a valid blueprint directory with `foundation.build.yaml`
- **When** running `foundation capture --dir ./testdata/fixtures/ecommerce`
- **Then** the command outputs valid JSON adhering to `GenerationRequest`
- **And** `req.ProtocolVersion` matches `protocol.ProtocolVersion`
- **And** `req.ArchitectureIR.Storage` contains lowered relational tables.

### Scenario 3: Tree Golden Assertion Regression Detection
- **Given** a response with emitted files and a golden directory on disk
- **When** an emitted file contains modified content and `UpdateGoldens == false`
- **Then** `harness.AssertGoldenTree` fails the test and prints a unified diff
- **When** `UpdateGoldens == true`
- **Then** the harness updates the golden file on disk and the test passes.

### Scenario 4: Plugin Doctor Conformance Verification
- **Given** a compliant generator plugin binary
- **When** running `foundation plugin doctor ./bin/compliant-plugin`
- **Then** all five probes pass and the process exits with code 0
- **When** running `foundation plugin doctor ./bin/polluting-plugin` (which prints logs to standard output)
- **Then** probe 4 fails with diagnostic `FDN0706` and the process exits with code 1.

### Scenario 5: Pipeline Anchor Modification and Missing Anchor Failure
- **Given** generated source code containing `// foundation:anchor:http-router-middleware`
- **When** calling `chain.InsertBefore(content, "http-router-middleware", "Use(OtelMiddleware())")`
- **Then** the returned source code contains the injected middleware before the anchor comment
- **When** calling `chain.InsertBefore(content, "non-existent-anchor", "snippet")`
- **Then** the function returns `chain.ErrAnchorNotFound`.

### Scenario 6: Diagnostic Prefix Collision Detection
- **Given** a `foundation.build.yaml` with two targets declaring `diagnostic_prefix: "ACME"`
- **When** running `foundation check`
- **Then** compilation fails with diagnostic `FDN0603`
- **And** the diagnostic message reports the colliding target names.

---

## File Breakdown

### Existing Files Modified
- `cmd/foundation/main.go`: Add `plugin new`, `plugin doctor`, and `capture` subcommands.
- `pkg/compiler/compiler_protocol.go`: Export capture helpers and new testing types.
- `pkg/emitterkit/plugin.go`: Implement `RunPluginWithFlags`, `WithDumpModel`, and option structures.
- `pkg/emitterkit/testing/harness.go`: Add `AssertGoldenTree`, `RunSmokeSuite`, and `LoadWireFixture`.
- `pkg/compiler/internal/emit/go_hexagonal.go`: Insert stable anchor comments at extension seams.
- `cmd/foundation-gen-otel/main.go`: Refactor to use `pkg/emitterkit/chain`.
- `pkg/compiler/internal/protocol/schema.go`: Complete `SemanticIR` JSON schema.
- `pkg/compiler/internal/platform/platform.go`: Add platform BOM dependency versions.
- `docs/blueprints/generator-plugins.md`: Document scaffolding, capture, doctor, and testing patterns.

### New Files Created
- `docs/specs/09-downstream-plugin-developer-tooling-and-contracts.md`: This specification.
- `docs/adr/0024-plugin-developer-tooling-and-chain-contracts.md`: Architectural Decision Record.
- `pkg/emitterkit/chain/chain.go`: Anchor lookup and code manipulation package.
- `pkg/emitterkit/chain/chain_test.go`: Tests for anchor manipulation.
- `pkg/emitterkit/testing/tree_golden.go`: Tree golden comparison implementation.
- `pkg/emitterkit/testing/smoke_runner.go`: Smoke test process runner.
- `cmd/foundation/capture.go`: Capture subcommand and request envelope export.
- `cmd/foundation/plugin.go`: Plugin subcommand router.
- `cmd/foundation/plugin_new.go`: `plugin new` subcommand.
- `cmd/foundation/plugin_doctor.go`: `plugin doctor` subcommand.
- `cmd/foundation/internal/pluginnew/scaffold.go`: Embedded template scaffolding generator.
- `cmd/foundation/internal/pluginnew/templates/`: Embedded template files for plugin generation.
- `cmd/foundation/internal/doctor/probes.go`: Probe implementations for plugin doctor.
- `test/e2e/plugin_tooling_e2e_test.go`: Black-box integration tests for plugin tooling.
- `contrib/emitterkit-py/`: Reference Python plugin client SDK.
- `contrib/emitterkit-ts/`: Reference TypeScript plugin client SDK.

---

## Non-Goals

- Replacing standard language toolchains (`go test`, `cargo test`, `pytest`) with custom build systems.
- Introducing binary IPC protocols (such as gRPC or shared memory) in place of standard input/output JSON streams.
- Allowing arbitrary code generation hooks that bypass the deterministic `GenerationResponse` file map contract.
- Generating full end-to-end user application templates outside generator plugin scaffolding.
