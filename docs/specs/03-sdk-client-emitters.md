---
status: done
---

# Specification 03: Strongly-Typed Go & TypeScript SDK Client Emitters (Phase 5)

## Problem Statement

Foundation generates production-grade, architecturally clean HTTP servers in Go (Hexagonal, Layered) and TypeScript (Clean Architecture). However, consuming these APIs currently requires handwritten HTTP client code:
- Callers must manually construct URLs, encode query parameters for `query many` keyset cursoring (`?limit=20&cursor=...`), and format JSON bodies for mutation actions.
- There is no type safety at the boundary: consumers must manually define TypeScript interfaces or Go structs mirroring the server's Value types and enums, leading to schema drift when Blueprints evolve.
- Error handling is brittle and untyped: callers parse raw JSON error payloads (`{"code": "...", "kind": "...", "message": "..."}`) and write ad-hoc status code checks instead of matching on structured, compile-time typed error discriminators (`unauthenticated`, `forbidden`, `not_found`, `conflict`, and custom domain errors).
- Authentication requires manual header boilerplate (`Authorization: Bearer <token>`) across every request instead of idiomatic client options or interceptors.
- Keyset cursor pagination traversal requires repetitive, error-prone manual loops to extract `page_info.next_cursor` and pass it to subsequent calls.

## Solution

Deliver Phase 5 of the Foundation Roadmap by implementing first-class **SDK Client Emitters** for both Go and TypeScript:
1. Support `kind: sdk` targets in `foundation.build.yaml`:
   - `language: go`: Emits a standalone, zero-dependency Go client package.
   - `language: typescript`: Emits a standalone, zero-dependency TypeScript/JavaScript client package using native standard `fetch`.
2. Generate **strongly-typed methods for every public Action** (excluding `@internal` actions):
   - Mutation actions take typed input structs/interfaces and return typed output structs/interfaces.
   - Keyset-paginated `query many` actions expose pagination-aware query methods returning items alongside pagination metadata (`next_cursor`, `prev_cursor`, `has_next`, `has_prev`).
3. Generate **Typed Error Discriminators**:
   - Built-in error kinds (`unauthenticated` 401, `forbidden` 403, `not_found` 404, `conflict` 409, `validation` 400).
   - Domain-specific errors declared in `.fdn` blueprints (`error InsufficientStock { kind: conflict, ... }`).
   - Idiomatic target language errors (Go: typed error struct implementing `error` and `errors.As`; TypeScript: custom `FoundationError` subclasses).
4. Provide **Pluggable Authentication & Client Options**:
   - Functional options in Go (`WithAuthToken(token)`, `WithHTTPClient(client)`, `WithHeader(key, value)`).
   - Configuration options in TypeScript (`authToken`, `fetch`, `headers`, `baseURL`).
5. **Zero External Dependencies**:
   - The Go SDK relies solely on the Go standard library (`net/http`, `encoding/json`, `context`, `time`).
   - The TypeScript SDK relies solely on the Web standard Fetch API (`fetch`, `Headers`, `URLSearchParams`), compatible with modern Node.js (18+), Bun, Deno, and browser runtimes without external npm packages.
6. Enforce **Full Manifest Lifecycle Hygiene**:
   - All emitted SDK files are tracked by `.foundation-manifest.json` with cryptographic SHA-256 hashes, ensuring automated pruning of stale files when Blueprint actions or values are renamed or removed.

## User Stories

### Configuration & Generation
1. As a platform engineer, I want to add `kind: sdk` targets with `language: go` or `language: typescript` in `foundation.build.yaml`, so that client libraries are emitted during `foundation build`.
2. As a SDK consumer, I want emitted client code to have zero third-party dependencies, so that importing the SDK introduces no supply-chain risks or transitive dependency bloat.
3. As a developer, I want `@internal` actions omitted from the SDK, so that internal background tasks or admin routines are never exposed in public client APIs.

### Go SDK Client
4. As a Go backend developer, I want to instantiate the client via `client := sdk.NewClient("https://api.example.com", sdk.WithBearerToken(token))`, so that configuring base URLs and authentication is idiomatic.
5. As a Go developer, I want every public action to expose a typed method accepting `context.Context` and typed input values, so that request payload types and timeouts are enforced at compile time.
6. As a Go developer, I want failed calls to return typed error structs inspectable via `errors.As(err, &domainErr)`, so that I can branch on specific business error codes or standard kinds (`sdk.ErrNotFound`, `sdk.ErrConflict`).
7. As a Go developer, I want paginated actions (`query many`) to accept cursor options and return strongly-typed `PageInfo`, so that keyset navigation is clean and deterministic.

### TypeScript SDK Client
8. As a frontend/fullstack developer, I want to import `import { Client, InsufficientStockError } from "./sdk"`, so that the client feels native to TypeScript applications.
9. As a TypeScript developer, I want every action method to return `Promise<T>` and reject with typed error instances, so that `try/catch` blocks can inspect `error.kind`, `error.code`, and error fields.
10. As a TypeScript developer, I want native `fetch` support without requiring Axios or polyfills, so that the SDK runs seamlessly in Node.js, Next.js, and edge workers.

### Cross-Language Verification
11. As a QA engineer, I want automated cross-language integration tests verifying that the Go SDK and TypeScript SDK successfully execute queries, mutations, and error handling against running Go Hexagonal, Go Layered, and TypeScript Clean HTTP servers.

## Implementation Plan

### 1. Build Profile & Architecture Lowering Seam
- Extend `arch.TargetKey` to support `Kind: "sdk"` for languages `go` and `typescript`.
- Create `GoSDKLowerer` and `TSSDKLowerer` implementing `arch.PatternLowerer`.
- Filter out actions with trait `@internal`.
- Lower exposed actions, domain values, enums, errors, and pagination query shapes into an `ArchitectureIR` specialized for client SDK generation.

### 2. Go SDK Emitter (`pkg/compiler/internal/emit/gosdk`)
- Module/package structure:
  - `client.go`: `Client` struct, `NewClient`, options (`WithHTTPClient`, `WithBearerToken`, `WithHeader`).
  - `types.go`: Struct representations of Blueprint `value`, `enum`, and action inputs/outputs.
  - `actions.go`: Strongly-typed action methods executing HTTP requests, handling query parameter serialization for `GET` routes and JSON body for `POST`/`PUT`/`DELETE`.
  - `errors.go`: Typed error interfaces and concrete error structs for built-in kinds (`ErrUnauthenticated`, `ErrForbidden`, `ErrNotFound`, `ErrConflict`, `ErrValidation`) and custom Blueprint errors.
- Line limit discipline: Keep every Go file $\le 450$ lines per `CODING_STANDARDS.md`.

### 3. TypeScript SDK Emitter (`pkg/compiler/internal/emit/tssdk`)
- File structure:
  - `index.ts`: Public exports.
  - `client.ts`: `FoundationClient` class with fetch wrapper, error parser, and action methods.
  - `types.ts`: TypeScript `interface` and `type` definitions for inputs, outputs, values, and enums.
  - `errors.ts`: Error class hierarchy (`FoundationError`, `ApiError`, `UnauthenticatedError`, `ForbiddenError`, `NotFoundError`, `ConflictError`, and domain errors).

### 4. Manifest Hygiene & Emitter Registration
- Register `gosdk.Emitter` and `tssdk.Emitter` in `pipeline.go`.
- Ensure generated files are fully tracked in `.foundation-manifest.json`.

### 5. Automated Tests
- Compiler build tests (`pkg/compiler/compiler_sdk_test.go`): Test generation of Go and TS SDK targets from Blueprints.
- Unit tests for Go SDK: Mock HTTP server testing success payloads, error parsing, and auth headers.
- Unit tests for TypeScript SDK: Execution via Node.js asserting client behavior.
- Cross-language integration test: Go & TS SDK clients calling generated HTTP backends.

## Testing Decisions & Agreed Seams
- **Seam 1 (Compiler Build Seam)**: `compiler.Build` verifies that configuring `kind: sdk` targets emits the expected file tree and manifest entries.
- **Seam 2 (Go SDK Client Interface Seam)**: The generated `client.Client` interface and methods, tested against standard Go `httptest.Server`.
- **Seam 3 (TypeScript SDK Client Interface Seam)**: The generated `Client` class tested against mock HTTP responses in Node.js.
- **Seam 4 (Cross-Language E2E Seam)**: Running Go and TS SDK clients against generated Go and TS backend servers to verify full wire fidelity.

## Out of Scope
- Code generation for non-HTTP transports (e.g. gRPC / WebSockets).
- Auto-publishing to npm or Go package registries (handled by CI/CD publishing workflows).
