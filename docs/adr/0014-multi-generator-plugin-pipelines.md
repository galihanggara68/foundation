# Multi-Generator Plugin Pipelines and Sequential Filter Chains

## Context

Developers often want to customize one architectural slice (such as DDD aggregates or telemetry middleware).
They want to keep built-in HTTP routing, database migrations, and transactional outbox tables.
Requiring custom generators to generate the full application stack causes code duplication and maintenance overhead.

## Decision

1. **Sequential Filter Chain**: Targets declare generator pipelines with `generators: [...]`. The compiler runs plugins sequentially in process isolation.
2. **Scaffolding Pre-population**: With `scaffold: builtin`, the compiler runs the built-in emitter first in memory. With `scaffold: none`, virtual file trees start empty.
3. **Collision Detection**: If a downstream generator mutates a file from an upstream generator, the compiler reports warning `FDN0702`.
4. **Dual Output Lifecycles**: `GenerationResponse` separates managed `files` from create-if-missing `stubs`.
5. **Native Test Delegation**: The `foundation test` command delegates execution to `test_command` for external targets.

## Consequences

- Teams compose specialized plugins without rewriting database or routing layers.
- The Manifest Engine keeps exclusive control over the filesystem.
- The compiler detects file collisions in plugin pipelines before writing to disk.
