# Contrib Runtime Libraries for Stateful Tactical Traits

Stateful tactical traits — `@idempotent`, `@cache`, `@outbox` — stop being emitted plumbing and resolve through published contrib runtime libraries with swappable drivers (`memory`, `postgres`) selected by the Build Profile's storage mode; stateless traits (`@retry`, `@transactional`) remain emitted inline control flow. This generalizes ADR 0006: Foundation already treats frameworks (chi, express, pg) as imports rather than generated code, and durable runtime machinery — outbox relay with at-least-once redelivery, concurrency-safe idempotency keys, cache stores — is infrastructure in the same sense. Emitting it would pin every production fix to a compiler release and a full project regeneration. The principle: the compiler owns the shape (structure, wiring, ports); libraries own the behavior (anything with state or a trust boundary).

## Consequences

- Composes with ADR 0007: the happy path remains post-commit in-process dispatch; the library's relay replays from the durable outbox table after a crash. Correctness holds in every driver — postgres persists `idempotency_keys` and outbox rows inside the action transaction; the memory driver preserves dev/test semantics with documented limits.
- `@cache` gains a driver seam (memory today, external stores later) without grammar or compiler changes.
- Generated projects gain compile-time dependencies on the runtime libraries; a declared, checkable compatibility contract between compiler and library versions becomes mandatory before the first release (see the versioning policy).
- Scenarios and contract tests run against the memory driver of the same libraries production uses — one runtime, two drivers.
- Open for the implementation ticket: DDL ownership for library-owned tables (compiler-emitted migrations vs. migrations shipped with the library).
