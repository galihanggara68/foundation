# Contrib Runtime Libraries for Stateful Tactical Traits

## Context

Stateful mechanisms (outbox relays, idempotency stores, and cache drivers) represent shared infrastructure.
Emitting stateful runtime machinery inline pins every bug fix to a new compiler release.
The compiler needed a separation between structural code generation and runtime infrastructure.

## Decision

Stateful tactical traits (`@idempotent`, `@cache`, `@outbox`) resolve through published contrib runtime libraries.
The Build Profile storage mode selects swappable storage drivers (`memory` or `postgres`).
Stateless traits (`@retry`, `@transactional`) remain inline generated control flow.
The compiler defines structural ports, while runtime libraries implement stateful behavior.

## Consequences

- Developers update runtime behavior through library dependency updates without regenerating blueprints.
- In-memory drivers allow tests to execute offline without external databases.
- The PostgreSQL driver persists idempotency keys and outbox rows inside the action transaction.
- The `@cache` trait supports alternative storage drivers without grammar modifications.
