# Foundation

A deterministic, multi-target architecture and application compiler that transforms declarative domain blueprints and action pipelines into production source code with zero runtime LLM dependency.

## Language

**Blueprint**:
A user-authored source file (`.fdn`) that declares a domain's data structures, contracts, policies, and action pipelines independent of any programming language or architectural pattern.
_Avoid_: Schema, spec, template, config

**Action Pipeline**:
A deterministic, guaranteed-terminating sequence of domain operations (validation guards, policy checks, state queries, mutations, bounded iterations, conditional branches, slot calls, and event emissions) declared within a Blueprint to express application behavior.
_Avoid_: Workflow, script, controller, handler

**Entity**:
A persistent domain object with identity and lifecycle declared in a Blueprint, lowered by the compiler into storage schemas and domain models.
_Avoid_: Table, model, record, struct

**Value**:
An immutable, identity-less data structure declared in a Blueprint for action inputs, outputs, or embedded fields.
_Avoid_: DTO, payload, schema, POJO

**Policy**:
A reusable, side-effect-free boolean rule declared in a Blueprint that evaluates whether an actor is authorized to perform an action or access an entity.
_Avoid_: Middleware, guard, RBAC filter, interceptor


**Slot**:
A strongly-typed contract declared in a Blueprint for custom logic that cannot be expressed in an Action Pipeline, emitted as a native interface or trait in the target language for the developer to implement in handwritten files.
_Avoid_: Escape hatch, hook, callback, protected region

**Trait**:
A tactical operational rule attached to a Blueprint element (such as idempotency, caching, retry policy, or transactional outbox) that specifies runtime behavior without dictating structural folder or layer layout.
_Avoid_: Annotation, decorator, pattern directive

**Build Profile**:
The project-level configuration that binds a set of Blueprints to one or more compilation targets, defining the structural architecture pattern, target language, and infrastructure adapters.
_Avoid_: Compiler config, target settings, manifest

**Semantic IR**:
The language-agnostic and architecture-agnostic intermediate representation of validated domain concepts (entities, policies, action pipelines, slots, and traits) produced by parsing Blueprints.
_Avoid_: AST, domain graph

**Architecture IR**:
The lowered intermediate representation produced by applying a structural architecture pattern (such as Hexagonal, Layered CRUD, or CQRS) to the Semantic IR before emitting target language code.
_Avoid_: Codegen tree, file template tree

**Emitter**:
A target-specific backend module that translates the Architecture IR into deterministic, formatted source files for a concrete language or infrastructure stack (such as Go, TypeScript, or PostgreSQL DDL).
_Avoid_: Transpiler, generator, printer

**SDK Client**:
A strongly-typed, zero-boilerplate client package emitted directly from a Blueprint's exposed Actions, Value types, and Error contracts for consumption by external services or frontends.
_Avoid_: API wrapper, frontend stub

**Scenario**:
A declarative domain test block authored in a Blueprint (`given` state/slot behavior, `when` Action invoked, `expect` result/error/events) that compiles into executable native unit tests across every target stack.
_Avoid_: Test case, spec test

**Generation Manifest**:
The deterministic registry (`.foundation-manifest.json`) tracking the exact file paths and content hashes emitted by the compiler, enabling full-project architectural scaffolding and stale-file pruning without touching handwritten Slot files.
_Avoid_: Build cache, file list

**Slot Stub**:
A create-if-missing implementation file emitted once for a newly declared Slot (and excluded from the Generation Manifest) so the generated project compiles immediately while awaiting the developer's handwritten logic.
_Avoid_: Boilerplate file, placeholder

**Contract Test**:
A compiler-emitted native test suite that executes against a developer's handwritten Slot implementation to verify that it satisfies the Blueprint's input/output invariants and error semantics.
_Avoid_: Integration test, manual unit test





