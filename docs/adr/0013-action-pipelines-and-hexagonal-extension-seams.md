# Action Pipelines as Application Services and Hexagonal Extension Seams

## Context

Embedding full ORM query builders into blueprints erodes offline determinism.
Generating stateful OOP aggregate methods creates impedance mismatches between Go and TypeScript.
Enterprise architectures decouple token verification from token issuance and identity providers.

## Decision

Foundation compiles business logic into declarative Action Pipelines and emits entities as plain data structures.
Complex operations route through typed Hexagonal port seams (Slots, Repository ports, and Actor Resolvers).
Handwritten slots manage multi-table SQL joins, custom database drivers, and cryptographic token issuance.

## Consequences

- The compiler remains deterministic, hermetic, and free of runtime dependencies.
- Developers write custom algorithms, CQRS read queries, and crypto functions in slots.
- Adapters remain swappable at the domain interface boundary in `internal/domain`.
