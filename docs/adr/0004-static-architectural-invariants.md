# Static Architectural Invariant Enforcement

## Context

External I/O calls inside database transactions cause database connection pool exhaustion.
Unconstrained database queries inside loops cause relational N+1 performance bottlenecks.
Silent automatic batching obscures execution behavior when loops contain side effects.

## Decision

The Foundation semantic analyzer rejects architectural anti-patterns at compile time.
The compiler forbids external I/O **Slot** calls inside `@transactional` action pipelines.
The compiler also detects and rejects relational query statements inside bounded `for` loops.

## Consequences

- The compiler prevents transaction starvation before deployment.
- Developers write efficient batch queries instead of N+1 loop queries.
- Action pipelines maintain predictable execution semantics across all target runtimes.
