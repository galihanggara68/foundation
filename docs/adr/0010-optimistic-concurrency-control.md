# Optimistic Concurrency Control with Entity Versions

## Context

Pessimistic row locking (`SELECT FOR UPDATE`) causes deadlocks and lowers throughput in distributed systems.
Concurrent entity updates can cause silent lost updates when multiple workers mutate state simultaneously.
Patch updates bypass invariant validation checks on the complete entity state.

## Decision

Foundation implements **Optimistic Concurrency Control (OCC)** through the `@version` field trait.
The compiler lowers entity updates to conditional version increments in database queries.
When zero rows change during an update, the runtime raises a built-in `conflict` error (HTTP 409).
HTTP adapters emit `ETag` headers on read requests and evaluate `If-Match` headers on mutations.

## Consequences

- The compiler permits only one `@version` integer field per entity.
- The runtime error taxonomy includes `conflict` (mapped to HTTP 409).
- Clients handle concurrency conflicts through standard HTTP conditional headers and `@retry` traits.
