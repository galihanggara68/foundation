---
status: ready-for-agent
---

# Specification 07: Generator Protocol Integrity and Storage Emission

ADR basis: extends [ADR 0011](../adr/0011-pluggable-emitter-protocol.md), [ADR 0014](../adr/0014-multi-generator-plugin-pipelines.md), [ADR 0018](../adr/0018-trait-attachment.md), [ADR 0019](../adr/0019-trait-namespaces-and-validation.md), [ADR 0020](../adr/0020-composite-entity-traits-and-list-arguments.md), and [ADR 0022](../adr/0022-generator-protocol-integrity-and-storage-emission.md).

## Problem Statement

The Foundation compiler exposes an external generator plugin interface over standard I/O via JSON payloads. Downstream generator plugins face six integration defects across protocol boundaries and SQL generation:

1. **Incomplete Public IR Surface**:
   The compiler exports only `EntityDecl`, `FieldDecl`, and `ActionDecl` in `pkg/compiler/compiler_protocol.go` and `pkg/emitterkit/plugin.go`. Downstream plugins cannot import internal packages under `pkg/compiler/internal/ir`. Consequently, plugins maintain private mirror structs across four type families and round-trip models through JSON. This causes silent field drops: new IR fields such as `TraitDecl.ListArgs` and `Expr.Span` disappear across the plugin boundary.

2. **Factual Errors in Generator Documentation**:
   The plugin documentation in `docs/blueprints/generator-plugins.md` contradicts the real wire protocol in four places:
   - Protocol version documented as string `"1.0"` instead of integer `1`.
   - Plugin file responses documented as arrays of objects instead of `map[string]string`.
   - Target configuration options documented under `target.config` instead of `target.options`.
   - Documentation claims a `timeout_seconds` target option exists, while the compiler hardcodes a constant 30-second execution timeout.

3. **Unvalidated Request Payloads on Decode**:
   The protocol helper `DecodeRequest` deserializes incoming JSON payloads into `GenerationRequest` but never calls `req.Validate()`. A malformed payload with a nil `semantic_ir` passes decoding without an error, forcing plugins to implement duplicate defensive guards.

4. **Missing Storage Plan on Custom Generator Targets**:
   The compiler calculates a complete relational `StoragePlanIR` (tables, columns, foreign keys, declarative DDL, and migration scripts) during compilation. However, `compiler.go` attaches this storage plan only to built-in targets. Generator targets receive a custom `ArchitectureIR` with a nil `Storage` pointer. Downstream plugins must re-derive database schemas from scratch and run live Postgres tests to compensate for absent compiler storage plans.

5. **Incomplete Composite Trait Emission**:
   Compiler release v0.8.0 validates entity-level `@index` and `@unique` traits with `fields: ["col_a", "col_b"]`. However, the storage plan emitter (`pkg/compiler/internal/arch/storage_plan.go`) only reads field-level traits. As a result, the compiler emits zero SQL statements for composite entity traits. Furthermore, `docs/blueprints/declarations.md` continues to document `@index` and `@unique` as field-only traits.

6. **Destructive Positional Trait Argument Parsing**:
   The trait parser (`pkg/compiler/internal/frontend/parser_traits.go`) assigns the single key `"_"` to all positional arguments. When a trait declares multiple positional arguments (such as `@len(3, 30)` or `@range(1, 100)`), subsequent arguments overwrite earlier ones. Downstream plugins must implement guessing ladders and unquoting heuristics to recover lost values.

---

## Solution

Implement Phase 1 (Protocol Integrity) and Phase 2 (Emission and Trait Completeness) across the compiler frontend, protocol layer, storage emitter, and public SDK packages:

1. **Export Full IR Surface**:
   Re-export all semantic and expression IR types in `pkg/compiler/compiler_protocol.go` and `pkg/emitterkit/plugin.go`. Eliminate downstream mirror models.

2. **Correct Generator Plugin Blueprint Documentation**:
   Update `docs/blueprints/generator-plugins.md` to reflect real wire protocol types, keys, and timeout behaviors.

3. **Enforce Decode-Time Request Validation**:
   Invoke `req.Validate()` inside `protocol.DecodeRequest`. Reject malformed or version-incompatible payloads immediately upon receipt.

4. **Attach Storage Plan to Generator Targets**:
   Populate `archIR.Storage` with the compiled `StoragePlanIR` for backend targets executing external generators in `pkg/compiler/compiler.go`.

5. **End-to-End Composite Trait SQL Generation**:
   Extend `pkg/compiler/internal/arch/storage_plan.go` to inspect entity traits. Generate table-level `UNIQUE` constraints and `CREATE INDEX` statements for composite `@unique` and `@index` declarations. Update `docs/blueprints/declarations.md`.

6. **Preserve and Normalize Trait Arguments**:
   Map positional arguments to platform trait parameter schemas in `pkg/compiler/internal/frontend/parser_traits.go`. Preserve non-colliding argument order for unregistered traits.

---

## Detailed Technical Design

### 1. IR Type Surface Export

Export all internal IR types from `pkg/compiler/internal/ir` into public package `pkg/compiler` and alias them in `pkg/emitterkit`:

#### Type Declarations to Re-Export
- **Root & Structure**: `SemanticIR`, `ActorDecl`, `SlotDecl`, `SlotMethodDecl`, `GivenSlotDecl`, `EnumDecl`, `EventDecl`, `PolicyDecl`
- **Fields & Types**: `FieldKind`, `FieldDecl`
- **Traits**: `TraitDecl`
- **Values & Entities**: `ValueDecl`, `EntityDecl`
- **Errors**: `ErrorKind`, `ErrorPosture`, `ErrorDecl`
- **Actions & Parameters**: `ParamDecl`, `ActionDecl`
- **Scenarios & Testing**: `GivenSeedDecl`, `ExpectDecl`, `ScenarioDecl`
- **Steps & Expressions**: `StepKind`, `ExprKind`, `Expr`, `FieldInitDecl`, `MatchArmDecl`, `WherePredicate`, `StepDecl`
- **Diagnostics**: `Span`, `Severity`, `Diagnostic`
- **Severity Constants**: `SeverityError`, `SeverityWarning`, `SeverityInfo`

#### Storage Plan Type Exports
Export storage plan structures from `pkg/compiler/internal/arch`:
- `StoragePlanIR`, `TableSchema`, `ColumnSchema`, `ForeignKey`, `IndexSchema`, `UniqueConstraint`, `MigrationPlan`

---

### 2. Generator Plugin Guide Alignment

Update `docs/blueprints/generator-plugins.md` with exact JSON wire examples:

1. **Protocol Version**: Set `"protocol_version": 1`.
2. **Virtual Files**: Show `"files": { "path/to/file.go": "content..." }` and `"stubs": { "path/to/stub.go": "content..." }`.
3. **Target Options**: Replace `"config"` with `"options"`.
4. **Execution Timeout**: Remove `timeout_seconds` from the target options table. Add a clear technical note: the compiler enforces a 30-second default execution timeout per generator plugin process.

---

### 3. Request Validation Enforcement

Update `DecodeRequest` in `pkg/compiler/internal/protocol/protocol.go`:

```go
func DecodeRequest(data []byte) (GenerationRequest, error) {
    var req GenerationRequest
    if err := json.Unmarshal(data, &req); err != nil {
        return GenerationRequest{}, fmt.Errorf("failed to decode generation request: %w", err)
    }
    if req.ProtocolVersion < 0 {
        return GenerationRequest{}, fmt.Errorf("invalid protocol version %d (must be >= 1)", req.ProtocolVersion)
    }
    if req.ProtocolVersion == 0 {
        req.ProtocolVersion = ProtocolVersion
    }
    if req.Files == nil {
        req.Files = make(map[string]string)
    }
    if req.Target.Options == nil {
        req.Target.Options = make(map[string]string)
    }
    if err := req.Validate(); err != nil {
        return GenerationRequest{}, fmt.Errorf("invalid generation request: %w", err)
    }
    return req, nil
}
```

Add unit tests verifying that `DecodeRequest` returns an error when:
- `semantic_ir` is missing or null.
- `workspace_root` is empty.
- Target fields (`name`, `kind`, `language`, `out_dir`) are empty.

---

### 4. Storage Plan Attachment for Generator Targets

In `pkg/compiler/compiler.go`, assign the compiled `storagePlan` to `archIR.Storage` when `target.Kind == "backend"`, regardless of whether `target` uses built-in emitters or custom generator plugins:

```go
} else {
    archIR = arch.NewCustomArchitectureIR(target, bp.Project.Module)
    if target.Kind == "backend" {
        archIR.Storage = storagePlan
    }
}
```

This guarantees external plugins receive full schema details in `GenerationRequest.ArchitectureIR.Storage`.

---

### 5. Composite Trait SQL Generation and Documentation

#### Storage Plan SQL Generator
Update `pkg/compiler/internal/arch/storage_plan.go`:

1. **Composite Unique Constraints**:
   - Inspect `ent.Traits` for `@unique`.
   - Extract column list from `trait.GetListArg("fields")`.
   - Resolve constraint name: use string argument `name` if present; otherwise generate `uq_<table_name>_<col1>_<col2>`.
   - Append `CONSTRAINT <name> UNIQUE (<col1>, <col2>, ...)` to the `CREATE TABLE` column definitions.

2. **Composite Indexes**:
   - Inspect `ent.Traits` for `@index`.
   - Extract column list from `trait.GetListArg("fields")`.
   - Resolve index name: use string argument `name` if present; otherwise generate `idx_<table_name>_<col1>_<col2>`.
   - Append `CREATE INDEX <name> ON <table_name> (<col1>, <col2>, ...);\n` after the `CREATE TABLE` statement.

#### Documentation Update
Update `docs/blueprints/declarations.md`:
- List `@index` and `@unique` under both Entity Traits and Field Traits.
- Document list argument `fields: ["col_a", "col_b"]` and optional string argument `name: "custom_name"`.

---

### 6. Trait Argument Normalization

Update `pkg/compiler/internal/frontend/parser_traits.go`:

1. **Registry Lookup**:
   When parsing trait arguments, inspect `platform.LookupTrait(traitName)`.

2. **Positional Parameter Mapping**:
   If the trait has registered positional parameters (e.g. `@len` defines `min`, `max`; `@range` defines `min`, `max`):
   - Map positional argument at index 0 to the first parameter name.
   - Map positional argument at index 1 to the second parameter name.

3. **Fallback for Unregistered Traits**:
   If the trait is unregistered, store positional arguments with distinct indexed keys (`_0`, `_1`, etc.) or preserve them in a dedicated `PositionalArgs []string` slice to prevent data loss.

4. **Value Normalization**:
   Unquote string literals so plugins receive raw content without surrounding quote characters.

---

## Acceptance Criteria

### Scenario 1: Emitterkit Exports Complete IR Types
- **Given**: An external Go module importing `github.com/galihanggara68/foundation/pkg/emitterkit`.
- **When**: The module references `emitterkit.StepDecl`, `emitterkit.Expr`, `emitterkit.TraitDecl`, and `emitterkit.StoragePlanIR`.
- **Then**: The Go compiler builds the module without undefined type errors.

### Scenario 2: DecodeRequest Rejects Nil Semantic IR
- **Given**: A JSON payload with `"semantic_ir": null` and `"protocol_version": 1`.
- **When**: An emitter calls `protocol.DecodeRequest(data)`.
- **Then**: The function returns a non-nil error stating `semantic_ir must not be nil`.

### Scenario 3: Generator Target Receives Storage Plan
- **Given**: A blueprint defining entity `Order` and a `backend` target using a custom generator plugin.
- **When**: The compiler executes the build pipeline.
- **Then**: The `GenerationRequest` delivered to the plugin contains a non-nil `architecture_ir.storage` containing table `orders`.

### Scenario 4: Storage Plan Emits Composite Unique Constraint
- **Given**: An entity declaration:
  ```fdn
  entity Member @unique(fields: ["organization_id", "email"]) {
    id: UUID @pk
    organization_id: UUID
    email: String
  }
  ```
- **When**: The compiler generates the storage plan.
- **Then**: The generated SQL contains `CONSTRAINT uq_members_organization_id_email UNIQUE (organization_id, email)`.

### Scenario 5: Storage Plan Emits Composite Index
- **Given**: An entity declaration:
  ```fdn
  entity AuditLog @index(fields: ["tenant_id", "created_at"], name: "idx_custom_audit") {
    id: UUID @pk
    tenant_id: UUID
    created_at: DateTime
  }
  ```
- **When**: The compiler generates the storage plan.
- **Then**: The generated SQL contains `CREATE INDEX idx_custom_audit ON audit_logs (tenant_id, created_at);`.

### Scenario 6: Positional Trait Arguments Normalized Without Collision
- **Given**: A field declaration `score: Int @range(1, 100)`.
- **When**: The parser parses the field traits.
- **Then**: The resulting `TraitDecl.Args` contains `min: "1"` and `max: "100"` without key collision.

---

## File Changes Breakdown

| Area | File | Change Description |
|---|---|---|
| Protocol Surface | `pkg/compiler/compiler_protocol.go` | Export all internal IR and storage plan types |
| SDK Surface | `pkg/emitterkit/plugin.go` | Re-export IR and storage types from compiler |
| Documentation | `docs/blueprints/generator-plugins.md` | Fix wire protocol JSON examples and timeout rules |
| Protocol Decode | `pkg/compiler/internal/protocol/protocol.go` | Call `req.Validate()` inside `DecodeRequest` |
| Storage Plan | `pkg/compiler/compiler.go` | Attach `StoragePlanIR` to custom generator targets |
| SQL Emission | `pkg/compiler/internal/arch/storage_plan.go` | Emit composite unique constraints and composite indexes |
| Blueprint Docs | `docs/blueprints/declarations.md` | Document entity-level composite traits |
| Trait Parser | `pkg/compiler/internal/frontend/parser_traits.go` | Map positional args to registered trait parameter names |
