# Action Pipelines as Application Services and Hexagonal Extension Seams

Foundation lowers business logic into declarative Action Pipelines (Application Use Cases) and emits domain entities as pure IR data structures, rather than compiling mutable, stateful aggregate methods. For operations that exceed the declarative pipeline DSL—such as multi-table SQL joins, alternative database drivers (`pgx`), external cryptographic libraries, and token issuance—Foundation routes them through strongly typed Hexagonal port seams (Slots, Repository interfaces, and Actor Resolvers) rather than expanding the compiler into an all-encompassing ORM or identity provider.

## Considered Options

- **Rich Domain Models with compiler-generated aggregate methods**: Rejected because mapping arbitrary behavior and object encapsulation across multiple target languages (Go, TypeScript) erodes offline determinism and leaks target-specific OOP assumptions into domain blueprints.
- **Embedded ORM / Dynamic SQL Query Builder**: Rejected because Foundation compiles predictable, non-leaky storage adapters following the 90/10 rule. Complex queries are better isolated behind CQRS read-model Slots.
- **Built-in Identity Provider and Token Issuer**: Rejected because enterprise architectures strictly decouple ingress token verification (`ActorResolver`) from token issuance, delegating identity management to established IdPs (Keycloak, Auth0, Supabase) or handwritten cryptographic Slots.

## Consequences

- The compiler core remains 100% deterministic, hermetic, and free of external runtime dependencies.
- Developers use **Slots** for custom algorithms, CQRS projections, and authentication crypto without fighting the compiler or risking overwrites.
- Primary and secondary adapters (HTTP middleware, repository drivers, actor resolvers) remain swappable at the domain interface boundary in `internal/domain`.
