# Post-Commit In-Process Event Dispatch

`@event_handler` lowers to a synchronous in-process dispatcher emitted as tracked code: handlers — ordinary Actions with a trigger trait rather than a new construct — run in declaration order after the emitting Action's transaction commits, so a handler failure can never roll back the state change it announces, and each handler's own Traits (`@retry`, `@transactional`, `@outbox`) govern its robustness instead of new per-handler delivery semantics. Delivery is at-most-once in-process; the `@outbox` trait remains the durability mechanism for guaranteed or cross-service delivery. `@cron` is cut from the grammar — a parse error marked "reserved" — until there is an honest answer to leader election, because an emitted ticker loop silently double-fires under two replicas, the same class of lie as emitting nothing.

## Consequences

- Handler Actions are fully ordinary: they carry their own Scenarios, policies, and Traits, and compose with them rather than defining separate dispatch semantics.
- Blueprints using `@cron` now fail fast with a clear message instead of compiling to nothing — strictly more honest, and free because no runtime ever depended on it.
- In-process dispatch is single-node by design; scaling handlers across processes is the outbox/broker story, not the dispatcher's.
