# Foundation Blueprint (`.fdn`) Documentation

Foundation Blueprints (`.fdn`) are declarative domain specifications describing your domain model, business pipelines, authorization rules, operational traits, and testing contracts. Blueprints are compiled deterministically into multi-target backend architectures with zero runtime LLM dependencies.

## Documentation Index

- [Top-Level Declarations & Grammar](./declarations.md): Complete guide to `enum`, `value`, `entity`, `error`, `event`, `policy`, `slot`, `action`, and `scenario`, scalar types, optionals, and collection combinators.
- [HTTP Transport & Parameter Binding](./http-transport.md): How `@http` maps routes, path parameters (`/items/{id}`), request bodies, actor authentication headers, `ETag` / `If-Match` headers, and transport error codes.
- [Tactical Traits & Runtime Durability](./tactical-traits.md): In-depth guide to `@transactional`, `@idempotent`, `@outbox`, `@cache`, `@retry`, `@event_handler`, PostgreSQL DDL generation, and the background outbox relay worker.
- [Optimistic Concurrency Control (OCC)](./concurrency-control.md): Field versioning with `@version`, conditional SQL updates, HTTP 409 Conflict error handling, and client concurrency workflows.
- [Slots & Handwritten Boundaries](./slots-and-contracts.md): The 10% custom code boundary (`slot`), `@external` safety checks, manifest-excluded starter stubs, and contract tests.
- [Declarative Scenarios & Testing](./scenarios-and-testing.md): Writing test scenarios with `given`, `when`, and `expect`, stubbing slot dependencies, and executing 100% offline hermetic tests.
- [Build Profile & Configuration](./build-profile.md): Reference for `foundation.build.yaml`, target languages (Go, TypeScript), storage dialects, schema migrations, and CLI commands.
- [Generator Plugins & Custom Emitters](./generator-plugins.md): Pluggable generator architecture (`foundation-gen-*`), wire protocol, multi-generator pipelines, scaffolding pre-population, and authoring plugins with `pkg/emitterkit`.
- [Extension Patterns & Platform Boundaries](./extension-patterns-and-limitations.md): Cookbook guide for real-world backend needs: SQL `JOIN`s, swapping DB drivers (`pgx`), custom headers, DDD aggregate invariants, and JWT token issuance.
