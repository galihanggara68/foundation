# Foundation

A deterministic, multi-target architecture and application compiler that transforms declarative domain blueprints and action pipelines into production source code with zero runtime LLM dependency.

## Language

**Blueprint**:
A user-authored source file (`.fdn`) that declares a domain's data structures, contracts, policies, and action pipelines independent of any programming language or architectural pattern.
_Avoid_: Schema, spec, template, config

**Action Pipeline**:
A deterministic, guaranteed-terminating sequence of domain operations (validation guards, policy checks, state queries, mutations, bounded iterations, conditional branches, slot calls, and event emissions) declared within a Blueprint to express application behavior.
_Avoid_: Workflow, script, controller, handler

**Event**:
A declared domain occurrence that an Action emits to announce a completed state change, delivered to subscribed Event Handlers after the emitting Action commits.
_Avoid_: Message, notification, signal

**Event Handler**:
An Action triggered by a subscribed Event instead of an HTTP route, executed in-process after the emitting Action's transaction commits; its own Traits govern its robustness and delivery guarantees.
_Avoid_: Listener, subscriber, cron job

**Entity**:
A persistent domain object with identity and lifecycle declared in a Blueprint, lowered by the compiler into storage schemas and domain models.
_Avoid_: Table, model, record, struct

**Entity Version**:
A monotonically increasing integer field on an Entity annotated with `@version` that enables Optimistic Concurrency Control during mutations to prevent silent lost updates.
_Avoid_: Revision, sequence number, timestamp, generation

**Value**:
An immutable, identity-less data structure declared in a Blueprint for action inputs, outputs, or embedded fields.
_Avoid_: DTO, payload, schema, POJO

**Policy**:
A reusable, side-effect-free boolean rule declared in a Blueprint that evaluates whether an actor is authorized to perform an action or access an entity.
_Avoid_: Middleware, guard, RBAC filter, interceptor


**Actor**:
The principal on whose behalf an Action is performed, supplied to Policies for authorization decisions; resolved from an incoming request by the Actor Resolver.
_Avoid_: User, session, caller


**Actor Resolver**:
The built-in port present in every generated backend that resolves an incoming request into a verified Actor before any Policy evaluates; its adapter is selected via the Build Profile's authn setting, defaulting to the trusted-headers dev adapter.
_Avoid_: Auth middleware, session filter, identity provider

**Conflict**:
A built-in domain error indicating that a concurrent mutation has rendered the target Entity state stale due to an Entity Version mismatch (mapped to HTTP 409).
_Avoid_: Race condition, concurrency error, stale data, mismatch

**Keyset Cursor**:
An opaque, deterministic pagination token representing the unique ordering key of the first or last item in a paginated read, used to navigate forward (`next_cursor`) or backward (`prev_cursor`) without offset scanning.
_Avoid_: Page number, offset, seek token, continuation token


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
The lowered intermediate representation produced by applying a structural architecture pattern (such as Hexagonal, Layered CRUD, or Clean Architecture) to the Semantic IR before emitting target language code.
_Avoid_: Codegen tree, file template tree

**Field Kind**:
The compiler-validated classification of a type-bearing position's value type — a scalar, an enum, a declared reference, or a built-in — carried by the Semantic IR so an Emitter maps types as a total function instead of guessing from an open type string.
_Avoid_: Field type, type tag, data type

**Emitter**:
A target-specific backend module that translates the Architecture IR into deterministic, formatted source files for a concrete language or infrastructure stack (such as Go, TypeScript, or PostgreSQL DDL).
_Avoid_: Transpiler, generator, printer

**External Generator Plugin**:
An executable binary (`foundation-gen-<target>`) invoked by the compiler over standard I/O that consumes a serialized `GenerationRequest` and returns a `GenerationResponse` containing generated file paths and contents.
_Avoid_: Template engine, custom script, codegen hook

**Generation Protocol**:
The standardized JSON schema contract governing wire communication between the Foundation compiler and external generator plugins over standard I/O.
_Avoid_: Plugin API, RPC protocol

**Generator Pipeline**:
An ordered sequence of External Generator Plugins invoked over standard I/O that iteratively transforms, augments, or replaces an in-memory virtual file tree before disk reconciliation.
_Avoid_: Plugin chain, codegen pipeline, filter pipe

**Scaffolding Pre-population**:
The compiler capability that pre-populates a GenerationRequest's virtual file tree with built-in emitter files (`scaffold: builtin`) before invoking external plugins, enabling partial overrides (such as Rich DDD or custom middleware) without rewriting complete backends.
_Avoid_: Template priming, base generation

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

**Pessimistic Row Lock**:
A transactional database row lock (`SELECT ... FOR UPDATE` in PostgreSQL, fine-grained mutex in memory) acquired during an Action Pipeline query via `@lock(exclusive)` to serialize concurrent mutations on high-contention domain resources without triggering Optimistic Concurrency Control conflict aborts.
_Avoid_: Table lock, spinlock, advisory lock

**Worker**:
A scheduled background process declared in a Blueprint that polls and processes persistent domain state at fixed intervals using transactional row locking (`FOR UPDATE SKIP LOCKED`), ensuring safe multi-replica execution without distributed leader election.
_Avoid_: Cron job, ticker loop, daemon thread

**Server-Sent Events Stream**:
An HTTP streaming response (`text/event-stream`) emitted for real-time query Actions that continuously pushes domain Events to clients with deterministic offset resumption via `Last-Event-ID`.
_Avoid_: WebSocket channel, long polling, socket connection

**Language Server**:
The in-tree Language Server Protocol implementation (`foundation lsp`) that serves live diagnostics, hover, completion, and go-to-definition for Blueprints by running the same compiler as `foundation check` over standard I/O.
_Avoid_: IDE plugin, editor extension, autocomplete daemon

**Symbol Index**:
The workspace-wide declaration index built and memoized by the Language Server, resolving Blueprints' declarations across files so hover and go-to-definition work on symbols declared outside the current document.
_Avoid_: AST cache, symbol table dump






