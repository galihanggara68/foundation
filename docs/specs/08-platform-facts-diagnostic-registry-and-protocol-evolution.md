---
status: ready-for-agent
---

# Specification 08: Platform Facts, Diagnostic Registry, and Protocol Evolution

ADR basis: extends [ADR 0011](../adr/0011-pluggable-emitter-protocol.md), [ADR 0014](../adr/0014-multi-generator-plugin-pipelines.md), [ADR 0016](../adr/0016-error-vocabulary-ownership.md), [ADR 0022](../adr/0022-generator-protocol-integrity-and-storage-emission.md), and [ADR 0023](../adr/0023-platform-facts-diagnostic-registry-and-protocol-evolution.md).

## Problem Statement

The interaction between the Foundation compiler and downstream generator plugins reveals six points of contract drift, duplicated logic, and protocol evolution friction:

1. **Platform Facts Divergence**:
   The compiler defines pagination defaults (`DefaultPageLimit = 20`, `DefaultMaxPageLimit = 100`) and standard error mappings.
   However, `platform.Info` publishes only pagination limits, while downstream plugins hardcode default page size to 10 and construct private error-kind-to-HTTP-status mappings.
   This divergence causes generated documentation to contradict compiler runtime expectations.

2. **Duplicated Path Security Sanitization**:
   The compiler implements directory traversal and zip-slip protection via `safeDiskPath` in `pkg/compiler/internal/emit/manifest.go`.
   Downstream plugins duplicate this logic in private functions (`safeRelPath`), creating duplicate code and possible security divergence.

3. **Duplicated Naming and Route Extraction Heuristics**:
   Downstream plugins maintain over 500 lines of heuristic code to derive physical table names and parse route paths.
   The compiler already resolves table names during lowering and validates route parameters against action parameter lists (`FDN0222`).
   However, the compiler does not expose `TableName` on `EntityDecl` or `InPath` flags on `ParamDecl`.

4. **Missing Test Fixtures, Golden IR Tests, and Determinism Suite**:
   The compiler lacks a standard blueprint fixture corpus in `testdata/fixtures/`, golden semantic IR tests, and byte-determinism verification suites.
   Downstream plugins had to create their own determinism harness (`determinism_test.go: byte-identical twice`).
   In addition, `pkg/emitterkit/testing` was planned in Specification 05 Ticket 06 but was never implemented.

5. **Diagnostic Registry Absence and Overloaded Codes**:
   The codebase contains 60 `FDNxxxx` codes as bare string literals across more than 500 call sites.
   The code `FDN0203` is overloaded across 7 distinct validation passes (unknown entity, unknown composite field, undeclared event, unknown scenario step, etc.).
   No central catalog exists, and no automated test prevents code collisions or undocumented codes.

6. **Unversioned Generator Responses**:
   The wire structure `GenerationResponse` lacks a `protocol_version` field.
   The decoder `DecodeResponse` does not check version compatibility.
   Without bidirectional version confirmation, future protocol updates cannot detect incompatible plugin responses safely.

---

## Solution

Implement Phase 3 (Platform Policies and Downstream Ergonomics) and Phase 4 (Quality, Stability & Protocol Evolution):

1. **Publish Authoritative Platform Facts**:
   Expand `platform.Info` to publish `PaginationDefaults` (default 20, max 100) and a structured `ReservedErrors` list detailing standard platform errors (Name, Kind, Status, Posture).
   Re-export these types in `pkg/compiler` and `pkg/emitterkit`.

2. **Export Canonical Path Safety Rule**:
   Export `SafeRelPath(rootDir, relPath string) (string, error)` from `pkg/emitterkit`.
   Share this audited path sanitizer between the compiler and all generator plugins.

3. **Publish Resolved Table Names and Route Parameter Markers**:
   Add `TableName string` to `ir.EntityDecl` and populate it during semantic lowering.
   Add `InPath bool` to `ir.ParamDecl` and set it to `true` when a parameter matches an action route token.

4. **Provide Fixture Corpus, Determinism Tests, and Plugin Test Harness**:
   Create a standard `.fdn` fixture corpus in `testdata/fixtures/`.
   Add golden IR assertion tests and a byte-determinism test suite verifying byte-identical outputs across compilation passes.
   Implement `pkg/emitterkit/testing` with reusable test harnesses (`Harness`, `AssertDeterministic`).

5. **Establish Centralized Diagnostic Registry**:
   Create a centralized diagnostic catalog in `pkg/compiler/internal/diagnostics/registry.go`.
   Disambiguate overloaded codes (assign dedicated codes for composite trait unknown fields and undeclared event handlers).
   Add automated tests for code uniqueness, catalog completeness, and documentation anchor alignment.

6. **Implement Bidirectional Protocol Versioning**:
   Add `ProtocolVersion int` to `protocol.GenerationResponse`.
   Validate protocol version compatibility in `protocol.DecodeResponse` with backward-compatible defaults.
   Document additive-only schema evolution rules in generator plugin guides.

---

## Detailed Technical Design

### 1. Platform Facts Publication

Update `pkg/compiler/internal/platform/platform.go` to publish authoritative platform decisions:

```go
type Info struct {
    Pagination     PaginationDefaults `json:"pagination"`
    ReservedErrors []ReservedError    `json:"reserved_errors"`
}

type ReservedError struct {
    Name    string       `json:"name"`
    Kind    ir.ErrorKind `json:"kind"`
    Status  int          `json:"status"`
    Posture ir.ErrorPosture `json:"posture"`
}

func DefaultInfo() Info {
    return Info{
        Pagination: PaginationDefaults{
            DefaultLimit: DefaultPageLimit,    // 20
            MaxLimit:     DefaultMaxPageLimit, // 100
        },
        ReservedErrors: StandardReservedErrors(),
    }
}
```

Standard reserved errors include:
- `NotFound`: `ErrorKindNotFound` (404, `PostureBusiness`)
- `Validation`: `ErrorKindValidation` (400, `PostureBusiness`)
- `Conflict`: `ErrorKindConflict` (409, `PostureBusiness`)
- `Unauthenticated`: `ErrorKindUnauthenticated` (401, `PostureBusiness`)
- `Unauthorized`: `ErrorKindUnauthorized` (403, `PostureBusiness`)
- `Forbidden`: `ErrorKindForbidden` (403, `PostureBusiness`)
- `Internal`: `ErrorKindInternal` (500, `PostureTechnical`)

Re-export `ReservedError` and `PaginationDefaults` in `pkg/compiler/compiler_protocol.go` and alias them in `pkg/emitterkit/plugin.go`.

---

### 2. Canonical Path Safety Export

Export `SafeRelPath` in `pkg/emitterkit`:

```go
package emitterkit

import (
    "fmt"
    "path/filepath"
    "strings"
)

// SafeRelPath verifies that rel is a clean relative path that remains within rootDir.
// It rejects absolute paths, empty paths, and path traversal sequences (..).
func SafeRelPath(rootDir, rel string) (string, error) {
    cleanRel := filepath.Clean(filepath.FromSlash(rel))
    if cleanRel == "." || cleanRel == "" {
        return "", fmt.Errorf("invalid empty or root file path: %q", rel)
    }
    if filepath.IsAbs(cleanRel) {
        return "", fmt.Errorf("security violation: absolute file path %q is forbidden", rel)
    }
    if cleanRel == ".." || strings.HasPrefix(cleanRel, ".."+string(filepath.Separator)) {
        return "", fmt.Errorf("security violation: file path %q escapes root directory", rel)
    }

    diskPath := filepath.Join(rootDir, cleanRel)
    relToRoot, err := filepath.Rel(rootDir, diskPath)
    if err != nil || relToRoot == ".." || strings.HasPrefix(relToRoot, ".."+string(filepath.Separator)) {
        return "", fmt.Errorf("security violation: resolved path %q escapes root directory %q", diskPath, rootDir)
    }

    return diskPath, nil
}
```

Use `SafeRelPath` inside `pkg/compiler/internal/emit/manifest.go` to eliminate internal duplicate code.

---

### 3. Resolved Table Names and Route Parameter Markers

Extend semantic IR types in `pkg/compiler/internal/ir/semantic.go`:

```go
type EntityDecl struct {
    Name      string      `json:"name"`
    TableName string      `json:"table_name"` // Resolved physical table name (e.g. "orders")
    Fields    []FieldDecl `json:"fields"`
    Traits    []TraitDecl `json:"traits,omitempty"`
    Span      Span        `json:"span"`
}

type ParamDecl struct {
    Name   string    `json:"name"`
    Type   string    `json:"type"`
    Kind   FieldKind `json:"kind"`
    InPath bool      `json:"in_path"` // true if parameter appears as {name} in action route
    Span   Span      `json:"span"`
}
```

During semantic lowering or post-validation lowering:
1. Populate `EntityDecl.TableName` with the value from `@table(name: "...")` if present, or the pluralized snake_case name of the entity.
2. Inspect action `@route(path: "...")` or `@route("...")` traits. If an action parameter name appears in the path string as `{param_name}`, set `InPath = true`.

---

### 4. Fixture Corpus, Determinism Tests, and Plugin Test Harness

#### 4.1 Fixture Corpus
Create a dedicated testdata fixture directory:
- `testdata/fixtures/minimal.fdn`
- `testdata/fixtures/ecommerce.fdn`
- `testdata/fixtures/composite_traits.fdn`

#### 4.2 Determinism Test Suite
Create `pkg/compiler/compiler_determinism_test.go`:
- Compile the fixture blueprint twice in separate calls.
- Assert that all emitted files, tracked files, stubs, and diagnostics are byte-identical.
- Confirm map iteration stability and canonical ordering.

#### 4.3 Plugin Test Harness Package (`pkg/emitterkit/testing`)
Create `pkg/emitterkit/testing/harness.go`:
- `NewHarness(t *testing.T) *Harness`
- `LoadFixture(path string) (*emitterkit.SemanticIR, error)`
- `AssertDeterministic(runFunc func() (emitterkit.GenerationResponse, error))`
- `AssertFileContent(resp emitterkit.GenerationResponse, path, expectedContent string)`

---

### 5. Centralized Diagnostic Registry (`FDNxxxx`)

Create `pkg/compiler/internal/diagnostics/registry.go`:

```go
package diagnostics

import "github.com/galihanggara68/foundation/pkg/compiler/internal/ir"

type Definition struct {
    Code            string
    Severity        ir.Severity
    Category        string
    MessageTemplate string
    Description     string
    DocAnchor       string
}

// Registry stores all registered compiler diagnostic definitions.
var Registry = map[string]Definition{
    // Syntax & Lexer (FDN0001 - FDN0099)
    // Declarations & Structure (FDN0100 - FDN0199)
    // Semantics & Types (FDN0200 - FDN0299)
    // Emitter & Generator Protocol (FDN0300 - FDN0399)
    // Invariants & Policy (FDN0400 - FDN0499)
}
```

Disambiguate overloaded `FDN0203` codes:
- `FDN0203`: Unknown entity reference.
- `FDN0230`: Unknown field reference in composite trait.
- `FDN0231`: Undeclared event target in event handler.
- `FDN0232`: Unknown scenario step target.

Add automated verification tests:
- `TestRegistry_AllCodesUnique`: Asserts zero duplicate code registrations.
- `TestRegistry_CodeFormat`: Asserts regex pattern `^FDN[0-9]{4}$`.
- `TestRegistry_CodebaseDrift`: Scans all Go files in `pkg/` and asserts every emitted diagnostic code exists in `Registry`.

---

### 6. Bidirectional Protocol Versioning and Evolution

Update `pkg/compiler/internal/protocol/protocol.go`:

```go
type GenerationResponse struct {
    ProtocolVersion int               `json:"protocol_version"`
    Files           map[string]string `json:"files,omitempty"`
    Stubs           map[string]string `json:"stubs,omitempty"`
    Diagnostics     []ir.Diagnostic   `json:"diagnostics,omitempty"`
    TestCommand     string            `json:"test_command,omitempty"`
}

func DecodeResponse(data []byte) (GenerationResponse, error) {
    var resp GenerationResponse
    if err := json.Unmarshal(data, &resp); err != nil {
        return GenerationResponse{}, fmt.Errorf("failed to decode generation response: %w", err)
    }
    // Backward compatibility: default unversioned response to ProtocolVersion (1)
    if resp.ProtocolVersion == 0 {
        resp.ProtocolVersion = ProtocolVersion
    }
    if resp.ProtocolVersion > ProtocolVersion {
        return GenerationResponse{}, fmt.Errorf("unsupported generator protocol version %d (compiler supports %d)", resp.ProtocolVersion, ProtocolVersion)
    }
    if resp.Files == nil {
        resp.Files = make(map[string]string)
    }
    if resp.Stubs == nil {
        resp.Stubs = make(map[string]string)
    }
    if resp.Diagnostics == nil {
        resp.Diagnostics = make([]ir.Diagnostic, 0)
    }
    return resp, nil
}
```

Update `docs/blueprints/generator-plugins.md` to document the response protocol version echo and additive schema evolution policy.

---

## Acceptance Scenarios

### Scenario 1: Platform Facts Publication on Request
- **Given** a blueprint compiled with generator targets
- **When** the compiler constructs a `GenerationRequest`
- **Then** `req.Platform` contains `PaginationDefaults` with `DefaultLimit = 20` and `MaxLimit = 100`
- **And** `req.Platform.ReservedErrors` contains standard platform error definitions for `NotFound`, `Validation`, `Conflict`, `Unauthenticated`, `Unauthorized`, `Forbidden`, and `Internal`.

### Scenario 2: Canonical Path Safety Sanitization
- **Given** an arbitrary root directory
- **When** calling `emitterkit.SafeRelPath` with a clean relative path `"domain/order.go"`
- **Then** the function returns the absolute joined path within the root directory
- **When** calling `emitterkit.SafeRelPath` with an escaping path `"../../etc/passwd"` or absolute path `"/root/file"`
- **Then** the function returns a security violation error.

### Scenario 3: Resolved Table Name and InPath Parameter Extraction
- **Given** an entity `Order` with no table trait and an entity `UserProfile` with trait `@table(name: "users")`
- **And** an action `GetOrder` with `@route("/orders/{order_id}")` taking parameters `order_id: string` and `filter: string`
- **When** the compiler generates `SemanticIR`
- **Then** `Order.TableName` equals `"orders"` and `UserProfile.TableName` equals `"users"`
- **And** parameter `order_id` has `InPath == true`
- **And** parameter `filter` has `InPath == false`.

### Scenario 4: Compiler Determinism and Plugin Harness
- **Given** the fixture blueprint `testdata/fixtures/ecommerce.fdn`
- **When** compiling the blueprint two times consecutively
- **Then** all generated files, stubs, and diagnostics are byte-identical across runs
- **And** plugin authors can run `testing.AssertDeterministic` from `pkg/emitterkit/testing` to verify plugin determinism.

### Scenario 5: Centralized Diagnostic Registry and Code Disambiguation
- **Given** the centralized diagnostic registry
- **When** running the diagnostic drift test
- **Then** every `FDNxxxx` code found in `pkg/` exists in `diagnostics.Registry`
- **And** all diagnostic codes are unique
- **And** composite trait unknown field errors emit `FDN0230` instead of `FDN0203`
- **And** undeclared event handler errors emit `FDN0231` instead of `FDN0203`.

### Scenario 6: Protocol Response Version Echo and Validation
- **Given** an external generator plugin response
- **When** the response includes `"protocol_version": 1`
- **Then** `DecodeResponse` succeeds and returns the parsed response
- **When** the response omits `protocol_version`
- **Then** `DecodeResponse` defaults the version to 1 for backward compatibility
- **When** the response includes `"protocol_version": 99`
- **Then** `DecodeResponse` returns an unsupported protocol version error.

---

## File Breakdown

### Existing Files Modified
- `pkg/compiler/internal/platform/platform.go`: Add `ReservedErrors` slice, constructor, and error definitions.
- `pkg/compiler/compiler_protocol.go`: Re-export `ReservedError` and `PaginationDefaults`.
- `pkg/emitterkit/plugin.go`: Re-export `ReservedError` and `PaginationDefaults`.
- `pkg/emitterkit/scaffold.go`: Export `SafeRelPath` and refactor path sanitization.
- `pkg/compiler/internal/emit/manifest.go`: Delegate path validation to `SafeRelPath`.
- `pkg/compiler/internal/ir/semantic.go`: Add `TableName` to `EntityDecl` and `InPath` to `ParamDecl`.
- `pkg/compiler/internal/validate/pass_traits.go`: Use disambiguated code `FDN0230` for composite unknown field errors.
- `pkg/compiler/internal/validate/pass_event_handler_test.go`: Use disambiguated code `FDN0231` for undeclared event errors.
- `pkg/compiler/internal/protocol/protocol.go`: Add `ProtocolVersion` to `GenerationResponse` and validate version in `DecodeResponse`.
- `docs/blueprints/generator-plugins.md`: Document response protocol version and platform facts.

### New Files Created
- `docs/adr/0023-platform-facts-diagnostic-registry-and-protocol-evolution.md`: ADR documenting the architectural decision.
- `docs/specs/08-platform-facts-diagnostic-registry-and-protocol-evolution.md`: This specification.
- `pkg/compiler/internal/diagnostics/registry.go`: Centralized diagnostic registry definition and catalog.
- `pkg/compiler/internal/diagnostics/registry_test.go`: Drift, uniqueness, and pattern tests for diagnostic codes.
- `pkg/emitterkit/testing/harness.go`: Plugin testing harness helper.
- `pkg/emitterkit/testing/harness_test.go`: Tests for plugin testing harness.
- `pkg/compiler/compiler_determinism_test.go`: Byte-identical compiler determinism tests.
- `testdata/fixtures/minimal.fdn`: Minimal syntax fixture blueprint.
- `testdata/fixtures/ecommerce.fdn`: Comprehensive domain fixture blueprint.
- `testdata/fixtures/composite_traits.fdn`: Composite index and unique constraints fixture blueprint.

---

## Non-Goals

- Modifying the underlying wire format from JSON to Protobuf or gRPC.
- Adding arbitrary plugin-defined configuration hooks outside the standard `target.options` dictionary.
- Changing runtime error handling semantics in generated code targets.
