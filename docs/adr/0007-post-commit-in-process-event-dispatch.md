# Post-Commit In-Process Event Dispatch

## Context

Running event handlers inside the emitting transaction can cause state rollbacks when non-critical subscribers fail.
Timer loops like `@cron` silently duplicate executions when applications scale to multiple replicas.
The compiler needed deterministic event dispatch semantics for in-process subscribers.

## Decision

The compiler lowers `@event_handler` into a synchronous in-process dispatcher.
Event handlers execute in declaration order after the emitting action transaction commits.
Each handler action uses its own tactical traits (`@retry`, `@transactional`, `@outbox`).
The grammar rejects `@cron` as a reserved keyword until a distributed leader election mechanism exists.

## Consequences

- Handler failures cannot roll back committed state transitions.
- Handlers remain standard actions with scenarios, policies, and operational traits.
- In-process dispatch operates within a single node.
- The `@outbox` trait provides durable delivery across services.
