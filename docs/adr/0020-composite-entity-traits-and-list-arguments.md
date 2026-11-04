# Composite Entity Traits and Trait List Arguments

## Context

Relational database designs require multi-column composite indexes and composite unique constraints.
The compiler previously defined `@index` and `@unique` as field-only traits.
The grammar and semantic IR supported only scalar string arguments in trait declarations.
Blueprints could not express composite database constraints without handwritten schema migrations.
Developers needed direct control over composite database indexes in `.fdn` source files.

## Decision

1. **Trait List Literal Grammar**:
   The compiler parser supports string list literals as trait argument values:
   ```fdn
   entity Order @index(fields: ["customer_id", "status"]) {
     ...
   }
   ```
2. **Semantic IR Support**:
   The semantic IR expands trait argument storage to preserve list structures alongside scalar string arguments.
3. **Entity Host Attachment**:
   The platform trait registry adds `TraitOnEntity` as an approved host for `@index` and `@unique`.
4. **Composite Constraint Validation**:
   - The compiler confirms that every field name in the `fields` argument exists on the target entity.
   - The compiler requires at least two fields in entity-level composite traits.
   - The compiler directs authors to field-level traits when an entity trait lists only one field.
5. **Optional Index Naming**:
   The `@index` trait accepts an optional `name` argument. Target emitters generate deterministic index names when authors omit the `name` argument.
6. **Multiple Trait Declarations**:
   Entities can declare multiple `@index` and `@unique` traits to define distinct column combinations.

## Consequences

- Domain authors declare composite database indexes and uniqueness constraints directly in blueprints.
- Downstream emitters generate precise multi-column DDL without manual intervention.
- The grammar retains clean syntax without comma-separated string splitting.
- The compiler rejects invalid field references at compile time before code generation.
