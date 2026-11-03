# Foundation Blueprint (`.fdn`) Documentation

Foundation blueprints (`.fdn`) are declarative domain specifications.
Blueprints describe domain models, business pipelines, authorization rules, operational traits, and testing contracts.
Foundation compiles blueprints into multi-target backend architectures with zero runtime LLM dependencies.

## Documentation Index

- [Top-Level Declarations and Grammar](./declarations.md): Guide to `enum`, `value`, `entity`, `error`, `event`, `policy`, `slot`, `action`, `scenario`, scalar types, optionals, and collection combinators.
- [HTTP Transport and Parameter Binding](./http-transport.md): Guide to `@http` routes, path parameters, request bodies, actor authentication headers, `ETag` headers, and transport error codes.
- [Tactical Traits and Runtime Durability](./tactical-traits.md): Guide to `@transactional`, `@idempotent`, `@outbox`, `@cache`, `@retry`, `@event_handler`, PostgreSQL DDL, and background outbox relay workers.
- [Optimistic Concurrency Control (OCC)](./concurrency-control.md): Field versioning with `@version`, conditional SQL updates, HTTP 409 conflict handling, and client concurrency workflows.
- [Slots and Handwritten Boundaries](./slots-and-contracts.md): Custom code boundaries (`slot`), `@external` safety checks, manifest-excluded starter stubs, and contract tests.
- [Declarative Scenarios and Testing](./scenarios-and-testing.md): Test scenarios with `given`, `when`, and `expect` blocks, slot dependency stubs, and offline test runs.
- [Build Profile and Configuration](./build-profile.md): Reference for `foundation.build.yaml`, target languages (Go, TypeScript), storage dialects, schema migrations, and CLI commands.
- [Generator Plugins and Custom Emitters](./generator-plugins.md): Pluggable generator architecture (`foundation-gen-*`), wire protocol, multi-generator pipelines, scaffolding pre-population, and authoring plugins with `pkg/emitterkit`.
- [Editor Support and Language Server](./editor-support.md): The in-tree LSP server (`foundation lsp`), live diagnostics, hover, completions, member completion, and go-to-definition.
- [Extension Patterns and Platform Boundaries](./extension-patterns-and-limitations.md): Solutions for SQL joins, database drivers, custom headers, aggregate invariants, and JWT authentication.
