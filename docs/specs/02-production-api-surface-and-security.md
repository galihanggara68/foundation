---
status: ready-for-agent
---

# Specification 02: Production API Surface & Security (Phases 3–4)

## Problem Statement

While Foundation successfully generates domain models, write actions, and transactional mutations, the language and runtime lack the essential read and security capabilities required to expose complete, production-grade HTTP APIs:
- There is no authentication layer: generated backends accept an unverified `Actor` context verbatim from incoming HTTP headers (`X-Actor-ID`, `X-Actor-Role`), leaving endpoints vulnerable to header spoofing unless an external reverse proxy enforces authentication.
- The error taxonomy cannot express authentication failures (HTTP 401); developers can only specify `forbidden` (HTTP 403), confusing callers by treating unauthenticated requests as authorization denials.
- The `Actor` structure is hardcoded in the compiler to `{ id: string, role: string, verified: bool }`. Multi-tenant applications cannot express or validate custom tenant claims (such as `actor.tenant_id == order.tenant_id` or `actor.tier == "enterprise"`) in compile-time policies.
- The read surface of an API is severely restricted: the language only supports `query one`. There is no mechanism to retrieve lists or collections of entities (`query many`), forcing developers to escape into handwritten Slots for standard read endpoints and breaking the 90/10 productivity promise.
- The `where` predicate grammar is restricted to single-field equality (`==`). Querying entities by numeric ranges, status conjunctions (`status == "pending" and total > 100`), or dates is impossible in the pipeline language.
- `timestamp` fields are treated as plain strings with no date/time arithmetic or comparison, and `decimal` fields compile to standard IEEE floating-point numbers (`float64` / `number`), risking floating-point rounding errors in currency and financial calculations.

## Solution

Deliver the Production API Surface & Security milestone (Phases 3 and 4) across the compiler, grammar, and target emitters:
1. Introduce the **Actor Resolver Port**: an architectural port present in every generated backend that intercepts incoming requests, verifies cryptographic credentials (such as JWT/OIDC Bearer tokens), and resolves them into a verified `Actor` before any policy evaluates.
2. Provide a built-in **`unauthenticated` Error Kind** that maps cleanly to HTTP 401 Unauthorized across all target transports and SDK error contracts.
3. Support a **Flexible Actor Schema**:
   - Zero-overhead when an API declares no policies referencing `actor`.
   - An implicit baseline `{ id: string, role: string, verified: bool }` for standard role-based access.
   - An optional custom `actor <Name> { ... }` block in `.fdn` Blueprints declaring domain-specific claims and tenant identifiers (`tenant_id`, `organization_id`, `tier`), fully type-checked at compile time in Policies and Scenarios.
4. Publish first-party **Authn Contrib Adapters**: a production `jwt-oidc` adapter (verifying JWT signatures via JWKS endpoints) and a dev-only `trusted-headers` adapter that prints an unmistakable boot banner and structured warning log line.
5. Introduce deterministic **`query many`**: a first-class Action Pipeline step for filtered, ordered, cursor-paginated reads that compiles into standard `GET /v1/<entities>` routes across all targets.
6. Enforce **Deterministic Keyset Cursoring**:
   - Require an `order_by` clause on a unique field (such as `@pk`), failing with a compile-time diagnostic if missing.
   - Enforce mandatory limits with a compiler-enforced hard ceiling.
   - Emit opaque keyset pagination cursors (`?cursor=<token>&limit=25`), permanently rejecting slow, non-deterministic offset pagination.
7. Expand the **Filter Predicate Grammar**: support comparison operators (`>`, `<`, `>=`, `<=`, `!=`) and logical conjunction (`and`) in `where` clauses, ensuring sargable, index-friendly query generation.
8. Upgrade to **Real Domain Types**:
   - `timestamp`: Native `time.Time` (Go) and `Date` (TypeScript) with RFC3339 serialization and temporal comparisons.
   - `decimal`: Fixed-precision arithmetic across all targets to guarantee currency-grade financial calculations.

## User Stories

### Authentication & Actor Resolution (Phase 3)
1. As a security architect, I want every generated backend to include an `ActorResolver` port, so that token verification occurs at the boundary before any domain policy evaluates.
2. As a platform engineer, I want to configure the authentication adapter in the Build Profile (`authn: jwt-oidc` or `authn: dev-headers`), so that security configuration remains separated from domain Blueprints.
3. As a developer, I want the compiler to provide a built-in `unauthenticated` error kind, so that failed token verification or missing credentials deterministically return an HTTP 401 Unauthorized response.
4. As a domain author, I want to declare a custom `actor` block in my Blueprint (e.g., `actor User { id: uuid, role: Role, tenant_id: uuid }`), so that my Policies can safely reference domain-specific claims like `actor.tenant_id == order.tenant_id`.
5. As a domain author, I want the compiler to validate custom actor fields at compile time, rejecting typos or undeclared attributes with exact source spans.
6. As a developer building a public or unauthenticated service, I want zero authn boilerplate or overhead when my Blueprint declares no policies referencing `actor`.
7. As a developer prototyping locally, I want an unconfigured Build Profile to default to a trusted-headers dev adapter, so that I can test endpoints with curl without generating mock JWT tokens.
8. As a security auditor, I want the dev-headers adapter to emit an unmistakable warning banner and structured log at startup, so that insecure dev configurations are never deployed to production undetected.
9. As a backend developer, I want the `jwt-oidc` contrib adapter to validate standard JWT claims (`sub`, `iss`, `aud`, `exp`) and retrieve public keys from a JWKS endpoint, so that real user identities are verified cryptographically.
10. As a domain author, I want to declare Scenarios with authenticated actors (e.g., `given actor with { id: "...", role: "admin", tenant_id: "..." }`), so that access control rules are verifiable in offline unit tests.

### Deterministic Query Many & Keyset Pagination (Phase 4)
11. As a domain author, I want to write `query many Order where customer_id == input.customer_id order_by id asc limit 20`, so that list-reading endpoints can be expressed natively in Action Pipelines.
12. As a backend engineer, I want the compiler to reject any `query many` step that omits `order_by` on a unique field, so that query results are strictly deterministic across database engines.
13. As a backend engineer, I want the compiler to reject any `query many` step that omits `limit` or specifies a limit exceeding the hard cap (e.g., > 100), so that runaway table scans are impossible by construction.
14. As a frontend developer, I want list-returning Actions to generate `GET /v1/<entities>` HTTP endpoints that accept `?cursor=<token>&limit=N` query parameters, so that paginated reads follow standard REST conventions.
15. As a frontend developer, I want paginated list responses to return an opaque `next_cursor` token when additional items exist, so that my application can paginate forward safely without offset drift.
16. As a database administrator, I want emitted SQL queries for pagination to execute keyset filters (`WHERE ... AND id > $cursor_id ORDER BY id ASC LIMIT $limit`), so that pagination performance remains constant regardless of table size.
17. As an API designer, I want offset pagination to be completely unavailable in the language, so that developers are prevented from shipping non-scalable `OFFSET / LIMIT` queries.

### Filter Operators & Real Domain Types (Phase 4)
18. As a domain author, I want to use comparison operators (`>`, `<`, `>=`, `<=`, `!=`) in `where` clauses and validation guards, so that numeric thresholds and date ranges can be evaluated in domain logic.
19. As a domain author, I want to combine filter conditions using `and` (e.g., `where status == "active" and created_at >= input.from_date`), so that compound query predicates are expressible in-language.
20. As a database administrator, I want the compiler to restrict compound filters to conjunctions (`and`), so that emitted SQL queries remain sargable and can leverage composite B-tree indexes.
21. As a domain author, I want `timestamp` fields to compile to native date/time types (`time.Time` in Go, `Date` in TypeScript) with RFC3339 serialization, so that date comparisons are chronologically accurate.
22. As a financial software engineer, I want `decimal` fields to compile to fixed-precision decimal arithmetic instead of IEEE floating point, so that currency calculations never suffer rounding loss.

## Implementation Decisions

### 1. Flexible Actor Schema Lowering
- If no `actor` block is declared in the Blueprint:
  - If policies reference `actor.<field>`, the compiler uses the implicit baseline:
    ```fdn
    actor Actor {
      id: string
      role: string
      verified: bool
    }
    ```
  - If no policies reference `actor`, the `ActorResolver` port is omitted from the container.
- If a custom `actor` is declared:
  ```fdn
  actor AccountUser {
    id: uuid
    role: Role
    tenant_id: uuid
    tier: string
  }
  ```
  The compiler generates typed context structs and `ActorResolver` interfaces matching this exact shape in the target language.

### 2. Query Many Grammar & AST Representation
- Pipeline step grammar:
  ```fdn
  orders = query many Order
    where customer_id == input.customer_id and status == OrderStatus.confirmed
    order_by id asc
    limit input.page_size
    cursor input.cursor
  ```
- Validation passes:
  - `ValidateQueryManyUniqueOrder`: Verifies that the ordering field has a `@pk` or `@unique` trait.
  - `ValidateQueryManyLimitCap`: Verifies that `limit` has a static ceiling (max 100).
  - `ValidateSargableWhere`: Verifies that predicates are conjunctions of single-field comparisons.

### 3. Keyset Cursor Format
- The cursor is an opaque base64url-encoded JSON string containing the ordering key value of the last item in the page:
  ```json
  {"id": "018f3a21-72a3-7b2c-9a4f-5231a498b8c1"}
  ```
- Keyset SQL Generation:
  ```sql
  SELECT id, customer_id, status, total, version, created_at
  FROM orders
  WHERE customer_id = $1 AND status = $2 AND id > $3
  ORDER BY id ASC
  LIMIT $4;
  ```

### 4. Transport Conventions for List Queries
- List-returning Actions automatically map to HTTP `GET`:
  - Path parameters map from action inputs declared in `@http(path: "/v1/customers/{customer_id}/orders")`.
  - Non-path inputs map to HTTP query string parameters: `?limit=20&cursor=eyJpZCI6...`.
  - Returns a paginated envelope:
    ```json
    {
      "items": [...],
      "next_cursor": "eyJpZCI6..."
    }
    ```

## Testing Decisions

- **Testing Seam**: Primary compiler driver seam — `compiler.Build` and `compiler.Test`.
- **Validation Unit Tests**:
  - Compiler tests asserting that `query many` without `order_by` or on non-unique fields fails with `FDN0215`.
  - Compiler tests asserting that missing limit or limits exceeding 100 fail with `FDN0216`.
  - Compiler tests asserting that invalid actor claims in policies fail with exact type-check diagnostics.
- **Scenario Tests**:
  - Blueprint scenarios verify `query many` reads against in-memory entity stores with filter matching and cursor pagination.
  - Blueprint scenarios verify policy authorization using `given actor with { ... }`.
- **Integration Tests**:
  - Emitted HTTP servers tested with HTTP client requests asserting on 401 Unauthorized without token, 403 Forbidden with unprivileged token, and 200 OK with valid JWKS-signed token.
  - Paginated `GET` requests asserting that sequential pages yield disjoint, strictly ordered items and terminate when `next_cursor` is null.

## Out of Scope

- Offset-based pagination (`OFFSET N`) — permanently rejected due to non-deterministic ordering and poor performance at scale.
- Arbitrary nested disjunctions (`OR (A AND (B OR C))`) in `where` clauses — deferred to preserve sargable query plans.
- GraphQL and gRPC emitters — REST/HTTP list conventions are the standard target.
- SDK Client emission — deferred to Phase 5.

## Further Notes

This specification completes the HTTP read and security surfaces, directly setting up the SDK Client Emitter (Phase 5 / Ticket #07) to generate fully-featured clients with token injectors and paginated iterators.
