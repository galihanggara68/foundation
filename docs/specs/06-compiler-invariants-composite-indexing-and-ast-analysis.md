---
status: ready-for-agent
---

# Specification 06: Compiler Invariants, Composite Indexing, and AST Analysis

ADR basis: extends [ADR 0004](../adr/0004-static-architectural-invariants.md), [ADR 0018](../adr/0018-trait-attachment.md), [ADR 0019](../adr/0019-trait-namespaces-and-validation.md), [ADR 0020](../adr/0020-composite-entity-traits-and-list-arguments.md), and [ADR 0021](../adr/0021-path-sensitive-mutation-invariants-and-dead-code-detection.md).

## Problem Statement

The Foundation compiler validates syntax and types, but authors face three architectural blind spots during blueprint development:

1. **Atomicity Blind Spots in Action Pipelines**:
   Action pipelines can chain multiple state mutations across one or more entities without declaring the `@transactional` trait. When a database error or network failure interrupts a pipeline halfway through execution, the database retains partial state changes. While the compiler enforces invariant rule `FDN0213` (no `@external` slot calls inside `@transactional`), the compiler does not alert developers when actions perform multiple mutations without transaction protection. Furthermore, code `FDN0214` already guards against relational N+1 loops, so new invariant rules must not collide with that diagnostic code.

2. **Inability to Declare Multi-Column Database Constraints**:
   Relational databases require composite indexes and composite unique constraints for query performance and data integrity. Upstream Foundation defines `@index` and `@unique` strictly as field-level traits (`TraitOnField`). Additionally, the `.fdn` grammar limits trait arguments to scalar strings stored in `map[string]string`. Blueprints cannot express composite indexes such as `@index(fields: ["customer_id", "status"])`, forcing teams to manage multi-column indexes through manual SQL migrations outside compiler control.

3. **Silent Dead Queries and Unused Pipeline Variables**:
   Action pipelines allow developers to declare local bindings via `let` statements. When a developer writes a database read (such as `let d = query one Dataset where id == input.id`) but never reads `d` in subsequent steps, the compiled application still executes the query. This dead query consumes database connection pool capacity and introduces latency without purpose. The compiler does not track variable usage across action steps, leaving dead code and unused action parameters undetected.

## Solution

Implement three compiler enhancements across the grammar, IR, validation passes, and diagnostic engine:

1. **Path-Sensitive Multi-Write Invariant Pass (`FDN0227`)**:
   Introduce a compile-time check within `pkg/compiler/internal/validate/pass_invariants.go`. The check inspects action pipelines and emits warning `FDN0227` whenever an action without `@transactional` contains two or more mutation steps along any single control flow path.

2. **Trait List Literal Grammar and Composite Entity Constraints**:
   Update the lexer and parser in `pkg/compiler/internal/frontend` to accept bracketed list literals (`fields: ["col_a", "col_b"]`) in trait arguments. Expand `ir.TraitDecl` to preserve list arguments. Register `TraitOnEntity` for `@index` and `@unique` in `pkg/compiler/internal/platform/traits.go`. Add validation to confirm field existence, enforce a two-field minimum, and support an optional index name.

3. **AST Dead Code and Unused Identifier Analysis (`FDN0228`)**:
   Add an AST analysis pass in `pkg/compiler/internal/validate` that constructs a symbol usage map for every action. The pass marks action parameters and `let` variable declarations as dead when no subsequent step references them. The pass emits warning `FDN0228`, supports underscore wildcard suppression (`_`), and warns when unused bindings wrap side-effecting operations.

---

## Detailed Technical Design

### 1. Multi-Write Transaction Invariant Pass

#### Diagnostic Specification
- **Code**: `FDN0227`
- **Severity**: `ir.SeverityWarning`
- **Message Format**: `action %q performs multiple write operations (%s) along an execution path without @transactional; wrap the action in @transactional to guarantee atomicity`

#### Analysis Rules
- **Target Operations**: The invariant analyzer counts operations of kind `ir.StepCreate`, `ir.StepUpdate`, and `ir.StepDelete`.
- **Path Sensitivity**: The analyzer traverses control flow trees:
  - Sequential steps add directly to the active path count.
  - An `if` statement splits into two independent paths: `then` and `else`. Each branch evaluates with the pre-condition count. The analyzer reports a warning only if at least one branch accumulates two or more write steps.
  - A `match` statement splits into independent paths for each pattern arm.
  - Bounded `for` loops evaluate loop body steps. If the loop body contains at least one write step, the analyzer flags it (independent of `FDN0214`).
- **Entity Scope**: The analyzer increments the path counter for every write step, regardless of whether the steps target identical or distinct entities.
- **Slot Boundary**: Slot method invocations (`ir.StepSlot`) do not increment the write counter, because slot contracts hide internal implementation details.
- **Suppression**: If the action carries the `@transactional` trait, the analyzer skips write counting completely.

---

### 2. Composite Entity Traits and Grammar Extension

#### Grammar and Lexer Changes
Extend the trait argument parser in `pkg/compiler/internal/frontend/parser_decl.go`:
```
TraitArgList  := '[' StringLiteral (',' StringLiteral)* ']'
TraitArgValue := Ident | StringLiteral | TraitArgList
```
The tokenizer already emits `tokLBracket` (`[`) and `tokRBracket` (`]`). The parser reads comma-separated string tokens between brackets when it encounters `tokLBracket`.

#### Semantic IR Extension
Update `ir.TraitDecl` in `pkg/compiler/internal/ir/semantic.go`:
```go
type TraitDecl struct {
    Name     string              `json:"name"`
    Args     map[string]string   `json:"args,omitempty"`
    ListArgs map[string][]string `json:"list_args,omitempty"`
    Span     Span                `json:"span"`
}
```
Helper methods on `ir.TraitDecl` provide unified lookup:
- `GetStringArg(key string) (string, bool)`
- `GetListArg(key string) ([]string, bool)`

#### Platform Registry Configuration
Update standard trait definitions in `pkg/compiler/internal/platform/traits.go`:
- `@index`: Hosts include `TraitOnField` and `TraitOnEntity`. Permitted arguments include `fields` (list) and `name` (string).
- `@unique`: Hosts include `TraitOnField` and `TraitOnEntity`. Permitted arguments include `fields` (list).

#### Entity Trait Validation Rules
In `pkg/compiler/internal/validate/pass_traits.go`:
1. When `@index` or `@unique` attaches to an entity:
   - The trait must supply the `fields` argument. If missing, emit warning `FDN0226`.
   - The `fields` list must contain at least two field names. If the list contains one field, emit warning `FDN0226` and recommend the field-level trait.
   - Every field name in `fields` must match a field declared on that entity. If a field does not exist, emit error `FDN0203` ("unknown field %q on entity %q in @%s trait").
2. When `@index` attaches to a field:
   - The trait accepts no arguments. If arguments exist, emit warning `FDN0226`.
3. Multiple entity traits:
   - An entity can declare multiple `@index` and `@unique` traits with distinct column sets.

---

### 3. Action AST Dead Code and Unused Variable Pass

#### Diagnostic Specification
- **Code**: `FDN0228`
- **Severity**: `ir.SeverityWarning`
- **Message Format (Pure Variable)**: `variable %q declared in action %q is never read; remove the declaration or prefix with '_' to ignore`
- **Message Format (Side Effect)**: `variable %q declared in action %q is never read, but its initializer performs a side effect (%s); discard the binding with '_' or remove the let prefix`
- **Message Format (Parameter)**: `action parameter %q in action %q is never read; prefix with '_' if intentionally unused`

#### Usage Resolution Rules
- The pass collects declared identifiers from:
  - Action parameters (`action.Params`)
  - `let` statements (`s.VarName`)
  - Iteration variables in `for` loops (`s.IterVar`)
- The pass scans every expression (`ir.Expr`) across subsequent action steps:
  - Variable references (`ir.ExprIdent`)
  - Property access expressions (`ir.ExprMember` matching `obj.field`)
  - Target assignments, policy arguments, and return expressions
- Entity field names in `where` predicates represent schema columns and do not mark action parameters as referenced.
- Re-binding an existing variable name within an inner branch reassigns the outer variable.
- Platform-bound parameter conventions:
  - Parameters referenced in action traits (such as `@idempotent(key: id)`) are marked as used.
  - Event parameters in `@event_handler` actions are required by compiler rule `FDN0204` and are marked as used.
  - Runtime contract parameters (`direction` on `query many` actions and `expected_version` on versioned updates) are marked as used.
- Suppression: If the identifier name starts with `_`, the pass does not report a warning.

---

## User Stories and Acceptance Scenarios

### Scenario 1: Multi-Write Action Emits Invariant Warning
- **Given**: An action `TransferFunds` without `@transactional` that updates `Account` and then creates `TransferRecord`.
- **When**: The compiler validates the blueprint.
- **Then**: The compiler produces warning `FDN0227` pointing to `TransferFunds`.
- **And**: The compile command succeeds with non-zero warning count.

### Scenario 2: Conditional Branches with Single Writes Pass
- **Given**: An action `ProcessRefund` without `@transactional` with an `if/else` block. The `if` branch updates `Order`. The `else` branch updates `Claim`.
- **When**: The compiler validates the blueprint.
- **Then**: The compiler produces zero `FDN0227` warnings because no single path runs more than one write step.

### Scenario 3: Composite Index Declared on Entity
- **Given**: An entity declaration:
  ```fdn
  entity Order @index(fields: ["customer_id", "status"]) {
    id: uuid @pk
    customer_id: uuid
    status: string
  }
  ```
- **When**: The compiler parses and validates the blueprint.
- **Then**: Parsing succeeds, `ir.TraitDecl.ListArgs["fields"]` contains `["customer_id", "status"]`, and validation emits zero diagnostics.

### Scenario 4: Composite Index Rejects Non-Existent Field
- **Given**: An entity declaring `@index(fields: ["customer_id", "missing_field"])`.
- **When**: The compiler validates the entity.
- **Then**: Validation returns error `FDN0203` with message citing unknown field `"missing_field"`.

### Scenario 5: Composite Index Rejects Single-Field List
- **Given**: An entity declaring `@index(fields: ["customer_id"])`.
- **When**: The compiler validates the entity.
- **Then**: Validation returns warning `FDN0226` instructing the author to use field-level `@index`.

### Scenario 6: Unused Let Variable Emits Warning
- **Given**: An action declaring `let d = query one Dataset where id == input.dataset_id` but never referencing `d`.
- **When**: The compiler validates the action pipeline.
- **Then**: Validation returns warning `FDN0228` for identifier `d`.

### Scenario 7: Underscore Variable Suppresses Warning
- **Given**: An action declaring `let _audit = query one AuditLog where id == input.id`.
- **When**: The compiler validates the action pipeline.
- **Then**: Validation emits no `FDN0228` warning for `_audit`.
