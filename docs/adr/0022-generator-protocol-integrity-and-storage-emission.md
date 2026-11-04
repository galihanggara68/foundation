# Generator Protocol Integrity and Storage Plan Emission

## Context

External generator plugins interact with the Foundation compiler over standard I/O with JSON payloads.
Downstream plugins had to maintain hand-written mirrors of internal compiler types across multiple packages.
This duplication caused silent field loss when the compiler added new AST and IR fields.
Furthermore, the generator plugin developer guide contradicted the wire protocol regarding version formats, virtual file trees, and execution timeouts.
Incoming requests with nil semantic models passed decoding without validation errors.
In addition, external generator targets received architecture models with nil storage plans, forcing plugins to derive relational schemas independently.
Finally, composite entity traits (`@index` and `@unique`) lacked SQL emission in storage plans, and positional trait arguments suffered destructive overwrites during parsing.

## Decision

1. **Re-Export Full IR and Storage Type Surface**:
   The compiler re-exports all semantic IR, expression, diagnostic, and storage plan types in `pkg/compiler/compiler_protocol.go` and `pkg/emitterkit/plugin.go`.
   Downstream plugins import official compiler types directly without private mirrors.

2. **Decode-Time Request Validation**:
   The decoder `protocol.DecodeRequest` invokes `req.Validate()` immediately upon unmarshaling.
   Requests with nil semantic models, empty workspace roots, or invalid target parameters fail with an explicit validation error.

3. **Wire Protocol Documentation Alignment**:
   Documentation in `docs/blueprints/generator-plugins.md` reflects the wire protocol:
   - Protocol version is an integer (`1`).
   - Virtual files and stubs use `map[string]string` format.
   - Target configuration options use the `options` object key.
   - The documentation records the default 30-second generator plugin timeout limit.

4. **Storage Plan Population and Attachment**:
   The compiler constructs `TableSchema` definitions for all entities and attaches `StoragePlanIR` to custom backend generator targets in `pkg/compiler/compiler.go`.
   Generator plugins receive complete relational schema models without re-deriving DDL.

5. **Composite Trait SQL Emission and Naming**:
   The storage plan generator inspects entity traits for composite `@unique` and `@index` declarations.
   The generator emits table constraints and index statements with explicit names or canonical names (`uq_<table_name>_<cols>` and `idx_<table_name>_<cols>`).
   Platform registry entries for `@unique` and `@index` permit both `fields` and `name` arguments.

6. **Positional Trait Argument Normalization**:
   The trait parser maps positional arguments to declared parameter schemas in `pkg/compiler/internal/platform/traits.go`.
   The parser preserves argument values with string unquoting and accepts both colon (`:`) and equals (`=`) separators for named arguments to match existing LSP completion templates.

## Consequences

- Generator plugins eliminate duplicated mirror structs and receive all IR fields without loss.
- Malformed payloads fail immediately at the decode boundary with descriptive error messages.
- Generator plugin authors reference accurate documentation that matches wire formats and timeouts.
- Backend generator targets receive populated relational table models and composite constraint SQL.
- Trait parsing prevents argument collisions and supports canonical LSP completion snippets.
