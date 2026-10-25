# Deterministic Query-Many with Mandatory Ordering

The Action Pipeline gains a `query many` step — filtered, ordered, cursor-paginated reads — and list-returning Actions lower to list HTTP routes. Because an unconstrained SELECT has no guaranteed row order, determinism is enforced by the grammar rather than requested of the developer: `order_by` on a unique field (typically the `@pk`) is a hard validation error when missing, the limit is mandatory with a compiler-enforced hard cap, and pagination is keyset cursoring on the ordering key — never offset. Handwritten Slot repository queries were rejected as the list mechanism because the primary read surface of an API is precisely the boilerplate Foundation exists to eliminate, and Slots are reserved for logic that cannot be expressed in a pipeline, not for reads the language declined to express.

## Consequences

- The read surface stays in the deterministic 90%: one Blueprint yields the same routes, SQL, and pagination contract on every target.
- Largest single language addition to date — grammar, validator, Semantic IR, Architecture lowerings, and every emitter must agree; the validator owns the invariants (unique ordering key, hard cap) so no emitter re-derives them.
- Offset pagination is permanently out: keyset cursors are not just the default but the only emitted pagination.
