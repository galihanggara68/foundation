# Platform Facts, Diagnostic Registry, and Protocol Evolution

## Context

The Foundation compiler and downstream generator plugins interact across process and language boundaries.
Downstream plugins currently show integration frictions and duplicated logic:

1. **Platform Facts Divergence**:
   The compiler defines platform policies such as pagination limits (`DefaultPageLimit = 20`, `DefaultMaxPageLimit = 100`) and standard error classifications.
   However, `platform.Info` publishes only pagination limits, while plugins hardcode pagination limits (`DefaultPageLimit = 10`) and re-implement error-to-status mapping tables.

2. **Duplicated Path Safety Logic**:
   The compiler secures disk writes against directory traversal and zip-slip attacks via `safeDiskPath` in `pkg/compiler/internal/emit/manifest.go`.
   Plugins duplicate this logic as private helpers (`safeRelPath`), creating duplicate maintenance and potential security discrepancies.

3. **Duplicated Identifiers and Route Parameter Heuristics**:
   Plugins maintain over 500 lines of table naming guesswork and path parameter regex matching.
   The compiler already resolves physical table names during lowering and validates HTTP route tokens (`FDN0222`), but does not publish these resolved values directly on `EntityDecl` and `ParamDecl`.

4. **Missing Golden Fixtures and Determinism Tests**:
   The compiler lacks a standard blueprint fixture corpus under `testdata/`, golden semantic IR tests, and byte-determinism tests.
   Furthermore, `pkg/emitterkit/testing` was planned in Specification 05 but not implemented.

5. **Diagnostic Code Drift and Overloading**:
   The compiler repository contains 60 `FDNxxxx` codes as bare string literals across hundreds of sites.
   The code `FDN0203` is overloaded across 7 distinct validation passes.
   No registry or test prevents code collisions, unauthorized reuse, or missing documentation anchors.

6. **Missing Protocol Version Echo in Responses**:
   `GenerationResponse` contains no `protocol_version` field.
   The decoder `DecodeResponse` performs no version checks.
   Without two-way version confirmation, future protocol updates cannot detect incompatible plugin responses.

## Decision

1. **Publish Authoritative Platform Facts**:
   Extend `platform.Info` to publish `PaginationDefaults` and a structured `ReservedErrors` catalog containing standard names, error kinds, HTTP statuses, and error postures.
   Export these structures through `pkg/compiler` and `pkg/emitterkit`.

2. **Export Canonical Path Safety Helper**:
   Export `SafeRelPath(rootDir, relPath string) (string, error)` from `pkg/emitterkit`.
   Both the compiler and downstream plugins use this shared function to sanitize relative target paths against root directories.

3. **Publish Resolved Table Names and Route Parameter Markers**:
   Add `TableName string` to `ir.EntityDecl` so plugins receive the compiler's resolved physical table name.
   Add `InPath bool` to `ir.ParamDecl` so plugins identify route path parameters without regex parsing.

4. **Add Fixture Corpus, Determinism Tests, and Plugin Test Harness**:
   Create a standard `.fdn` test fixture corpus under `testdata/fixtures/`.
   Add golden IR assertion tests and a byte-determinism test suite verifying that compilation runs produce byte-identical outputs.
   Implement `pkg/emitterkit/testing` with reusable test harness helpers (`Harness`, `AssertDeterministic`).

5. **Establish Centralized Diagnostic Registry**:
   Create a centralized diagnostic catalog in `pkg/compiler/internal/diagnostics/registry.go`.
   Disambiguate overloaded codes (assign dedicated codes for composite trait unknown fields and undeclared event handlers).
   Add automated tests to enforce code uniqueness, pattern compliance, and documentation anchor validity.

6. **Implement Bidirectional Protocol Versioning and Evolution Contract**:
   Add `ProtocolVersion int` to `protocol.GenerationResponse`.
   Enforce version validation in `protocol.DecodeResponse` with backward-compatible defaults.
   Document additive-only wire schema stability rules in generator plugin documentation.

## Consequences

- Downstream plugins consume authoritative pagination policies and error definitions from the compiler, eliminating drift.
- Compiler and plugins share a single audited sandbox path sanitizer.
- Plugins receive resolved physical table names and route parameter flags directly on IR nodes, deleting over 500 lines of heuristic code.
- Compiler and plugin builds benefit from golden fixtures, determinism assertions, and reusable test harnesses.
- Centralized diagnostic registry prevents code collisions, eliminates overloaded codes, and guarantees documentation drift protection.
- Protocol version echo in generator responses enables safe forward and backward protocol negotiation.
