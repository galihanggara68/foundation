# Deterministic Query-Many with Mandatory Ordering

## Context

SQL queries without explicit ordering return rows in unpredictable order across database engines.
Offset pagination causes performance degradation and missed rows during concurrent data insertion.
Handwritten slot queries for list operations reintroduce repetitive boilerplate code.

## Decision

The action pipeline provides a `query many` statement for list reads.
The grammar enforces ordering by requiring an `order_by` clause on a unique field.
The query requires a mandatory limit with a compiler-enforced maximum value.
The compiler emits keyset cursor pagination on the ordering key instead of offset pagination.

## Consequences

- Blueprints generate deterministic list routes and SQL queries on all targets.
- Keyset cursor pagination prevents pagination drift and slow queries.
- The compiler validator enforces ordering uniqueness and limit caps centrally.
