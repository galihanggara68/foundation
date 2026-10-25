# Optimistic Concurrency Control with Entity Versions

Entity mutations gain **Optimistic Concurrency Control (OCC)** via an optional `@version` field trait that prevents silent lost updates across horizontal replicas, and mutation conflicts resolve through a built-in `conflict` error kind (HTTP 409). Pessimistic row locking (`SELECT FOR UPDATE`) was rejected because holding database locks across multi-step action pipelines invites deadlocks, degrades throughput, and cannot protect workflows that span separate user interactions (e.g., reading a form and submitting an update seconds later). Delta/patch updates were rejected because they bypass domain invariant checking on the full entity state. Under OCC, `update` steps compile to conditional version increments (`WHERE id = $1 AND version = $2` in PostgreSQL and atomic version assertions in memory); if zero rows are updated, the pipeline raises `conflict`, which HTTP adapters map to `409 Conflict` and pair with standard `ETag` and `If-Match` header evaluation.

## Consequences

- The Blueprint grammar adds the `@version` field trait, validated strictly on integer entity fields (`int`), with at most one `@version` field per entity.
- The compiler adds a built-in `conflict` error kind (HTTP 409) to the diagnostic and runtime error taxonomy, joining `unauthenticated` (401), `forbidden` (403), `not_found` (404), `bad_request` (400), and `internal` (500).
- Emitted storage adapters conditionally assert and increment versions: PostgreSQL emits `UPDATE ... SET ..., version = version + 1 WHERE id = $1 AND version = $2 RETURNING version`, raising `conflict` when zero rows are affected; in-memory adapters enforce identical semantics.
- HTTP adapters emit `ETag` headers representing entity versions on reads and validate incoming `If-Match` headers on mutations for versioned entities.
- Clients compose cleanly with `@retry` for transient version conflicts under concurrent access.
