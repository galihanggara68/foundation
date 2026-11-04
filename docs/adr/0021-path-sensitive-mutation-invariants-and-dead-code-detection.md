# Path-Sensitive Mutation Invariants and Action AST Dead Code Detection

## Context

Action pipelines that run multiple mutations without database transactions risk partial execution and data corruption.
When a mid-pipeline step fails after earlier writes succeed, databases retain inconsistent state.
The compiler enforces architectural safety at compile time, but actions lacked an atomicity check for multi-step mutations.
The compiler already allocates diagnostic code `FDN0214` to prevent relational N+1 loops.
Reusing `FDN0214` for transaction checks causes diagnostic collisions.
Furthermore, action pipelines often contain dead queries and unused `let` variables that waste database connections.

## Decision

1. **Diagnostic Code `FDN0227` for Multi-Write Invariants**:
   The compiler emits warning `FDN0227` when an action pipeline contains multiple write operations without the `@transactional` trait.
2. **Path-Sensitive Control Flow Analysis**:
   The invariant analyzer evaluates write steps along individual execution paths.
   Mutually exclusive conditional branches (`if`/`else` or `match`) do not trigger warnings if each branch contains only one write step.
3. **Mutation Step Scope**:
   The invariant analyzer counts all native blueprint write operations (`create`, `update`, and `delete`).
   Two writes along the same execution path trigger `FDN0227` even when both writes target the same entity.
   The analyzer ignores slot method calls because slot interfaces represent opaque boundaries.
4. **Diagnostic Code `FDN0228` for Dead Identifiers**:
   The compiler emits warning `FDN0228` when an action pipeline declares an input parameter or a `let` variable that subsequent steps never reference.
5. **Wildcard Suppression**:
   Authors suppress `FDN0228` warnings by prefixing identifier names with an underscore (`_`).
6. **Side-Effect Clarification**:
   When an unreferenced variable binds the result of a slot call or a mutation step, the diagnostic message explicitly notes that the underlying operation still runs.

## Consequences

- Authors receive compile-time warnings about atomicity risks before deploying non-transactional actions.
- Path-sensitive analysis prevents false positive warnings on conditional branches.
- Compiler diagnostic code `FDN0214` remains dedicated to N+1 loop detection without semantic drift.
- Unused database queries and dead variables become visible during blueprint compilation.
