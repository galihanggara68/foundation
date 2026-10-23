# Static Architectural Invariant Enforcement

The Foundation semantic analyzer rejects or warns against runtime anti-patterns at compile time—specifically forbidding external I/O **Slot** calls inside `@transactional` blocks and flagging N+1 `query` operations inside bounded `for` loops. Attempting to silently auto-batch imperative loops in the middle-end obscures execution semantics when loops interleave side effects, whereas allowing unconstrained I/O inside database transactions leads to connection pool exhaustion and partial-failure bugs across every target language.
