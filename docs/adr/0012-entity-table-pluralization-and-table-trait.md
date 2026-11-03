# Deterministic Entity Table Pluralization and @table Trait

## Context

Singular table names like `order` and `user` conflict with ANSI SQL reserved keywords.
Those conflicts require brittle quote escaping and cause SQL syntax errors across database tools.
External pluralization libraries violate the requirement for offline hermetic compilation.

## Decision

Entities lower to relational database tables with deterministic English pluralization rules.
The compiler implements compact pluralization rules for standard English suffixes (`-s`, `-es`, `-ies`, `-ves`).
Authors specify custom table names with the `@table("name")` trait for irregular nouns or legacy databases.

## Consequences

- The compiler operates offline without third-party pluralization packages.
- Common table names avoid collisions with SQL reserved keywords automatically.
- Irregular nouns (such as `Person` to `people`) require explicit `@table` annotations.
