# Foundation Glossary

Foundation is an application compiler.
The compiler converts domain blueprints into production code without large language models.

## Language

**Blueprint**:
A source file (`.fdn`) that declares domain data structures, contracts, policies, and action pipelines.
The blueprint does not depend on target languages or architectures.
_Avoid_: Schema, spec, template, config

**Action Pipeline**:
A sequence of operations that expresses application behavior.
The pipeline contains validation guards, policy checks, queries, mutations, slot calls, and event emissions.
The pipeline always stops in finite steps.
_Avoid_: Workflow, script, controller, handler

**Event**:
A domain occurrence that an action emits after a state change.
The system delivers the event to event handlers after the transaction commits.
_Avoid_: Message, notification, signal

**Event Handler**:
An action that starts when the system emits a subscribed event.
The event handler runs in-process after the emitting transaction commits.
Traits control handler reliability and delivery rules.
_Avoid_: Listener, subscriber, cron job

**Entity**:
A persistent domain object with identity and lifecycle.
The compiler lowers entities into database schemas and domain models.
_Avoid_: Table, model, record, struct

**Entity Version**:
An integer field on an entity with the `@version` trait.
The field enables optimistic concurrency control during updates to prevent lost updates.
_Avoid_: Revision, sequence number, timestamp, generation

**Value**:
An immutable data structure with no identity.
Blueprints use values for action inputs, action outputs, and embedded fields.
_Avoid_: DTO, payload, schema, POJO

**Policy**:
A reusable boolean rule with no side effects.
A policy checks if an actor can run an action or access an entity.
_Avoid_: Middleware, guard, RBAC filter, interceptor

**Actor**:
The principal that requests an action.
The actor resolver provides the actor to policies for authorization checks.
_Avoid_: User, session, caller

**Actor Resolver**:
A built-in port that extracts an actor from an incoming request.
The build profile selects the adapter for the actor resolver.
_Avoid_: Auth middleware, session filter, identity provider

**Conflict**:
A built-in domain error that maps to HTTP 409.
The error occurs when a concurrent update causes an entity version mismatch.
_Avoid_: Race condition, concurrency error, stale data, mismatch

**Error Posture**:
The visibility of an error message.
A `business` error shows text to the caller.
A `technical` error hides text from the caller and logs details on the server.
_Avoid_: Severity, error level, log level, visibility

**Standard Error Vocabulary**:
A compiler table that maps built-in error kinds to default HTTP status codes and postures.
The vocabulary also reserves error kind names.
_Avoid_: Error enum, status map, error registry, error taxonomy

**Keyset Cursor**:
A deterministic pagination token that holds the unique key of a boundary item.
Callers use the cursor to navigate pages without database offset scans.
_Avoid_: Page number, offset, seek token, continuation token

**Slot**:
A typed contract in a blueprint for custom logic outside an action pipeline.
The compiler emits a slot as an interface in the target language.
The developer writes the slot implementation by hand.
_Avoid_: Escape hatch, hook, callback, protected region

**Trait**:
A metadata annotation prefixed with `@` on an entity, field, action, or slot method.
Traits control storage mapping, transport exposure, concurrency, and transactions.
_Avoid_: Decorator, annotation, tag, attribute, directive

**Trait Namespace**:
A prefix before a colon in a trait name (such as `@acme:cached`).
A trait namespace assigns trait ownership to external tools or plugins.
The compiler passes namespaced traits through without validation checks.
_Avoid_: Trait prefix, plugin tag, custom attribute scope

**Build Profile**:
A configuration file (`foundation.build.yaml`) that binds blueprints to compilation targets.
The build profile sets the architecture pattern, target language, and infrastructure adapters.
_Avoid_: Compiler config, target settings, manifest

**Semantic IR**:
An intermediate data structure that represents validated domain concepts.
The Semantic IR does not depend on target languages or architectures.
_Avoid_: AST, domain graph

**Architecture IR**:
An intermediate data structure produced by applying an architecture pattern to the Semantic IR.
The compiler uses the Architecture IR to emit target code.
_Avoid_: Codegen tree, file template tree

**Field Kind**:
The verified category of a field value type.
Field kinds include scalars, enums, declared references, and built-in types.
_Avoid_: Field type, type tag, data type

**Emitter**:
A backend compiler module.
The emitter translates Architecture IR into source files for a specific target stack.
_Avoid_: Transpiler, generator, printer

**External Generator Plugin**:
An executable program (`foundation-gen-<target>`) that the compiler runs through standard input and output.
The program receives a generation request and returns generated files.
_Avoid_: Template engine, custom script, codegen hook

**Generation Protocol**:
The JSON contract for communication between the compiler and generator plugins over standard input and output.
_Avoid_: Plugin API, RPC protocol

**Capture**:
A compiler operation that exports a complete Generation Request wire envelope for a selected target.
Plugin authors run `foundation capture` to build authoritative test fixtures.
_Avoid_: Dump, snapshot, export

**Generator Pipeline**:
An ordered sequence of generator plugins.
The pipeline transforms the virtual file tree before the compiler writes files to disk.
_Avoid_: Plugin chain, codegen pipeline, filter pipe

**Scaffolding Pre-population**:
A compiler feature that fills the virtual file tree with built-in emitter files before plugins run.
Plugins can override specific files without regenerating the full backend.
_Avoid_: Template priming, base generation

**SDK Client**:
A typed client package generated from actions, values, and errors.
External services and frontends use the SDK client to call the backend.
_Avoid_: API wrapper, frontend stub

**Scenario**:
A declarative test block in a blueprint with `given`, `when`, and `expect` clauses.
The compiler compiles scenarios into native unit tests.
_Avoid_: Test case, spec test

**Generation Manifest**:
A record file (`.foundation-manifest.json`) that tracks emitted files and content hashes.
The compiler uses the manifest to clean obsolete files without touching handwritten slot code.
_Avoid_: Build cache, file list

**Slot Stub**:
A template file generated once for a new slot.
The developer adds custom logic to this file.
The compiler never overwrites an existing slot stub.
_Avoid_: Boilerplate file, placeholder

**Contract Test**:
A generated test suite that runs against a handwritten slot implementation.
The test confirms that the implementation satisfies blueprint invariants.
_Avoid_: Integration test, manual unit test

**Pessimistic Row Lock**:
A database row lock (`SELECT ... FOR UPDATE`) requested with `@lock(exclusive)`.
The lock prevents concurrent mutations on high-contention resources.
_Avoid_: Table lock, spinlock, advisory lock

**Worker**:
A background process declared in a blueprint.
The worker reads domain state at scheduled intervals with row locks (`FOR UPDATE SKIP LOCKED`).
_Avoid_: Cron job, ticker loop, daemon thread

**Server-Sent Events Stream**:
An HTTP streaming response (`text/event-stream`) for real-time actions.
The stream pushes events to clients and supports offset resumption through `Last-Event-ID`.
_Avoid_: WebSocket channel, long polling, socket connection

**Language Server**:
The language server protocol service (`foundation lsp`).
The server provides diagnostics, hover documentation, completion, and definition lookups to editors.
_Avoid_: IDE plugin, editor extension, autocomplete daemon

**Symbol Index**:
A workspace index that tracks all blueprint declarations across files.
The language server uses the symbol index to resolve cross-file references.
_Avoid_: AST cache, symbol table dump
