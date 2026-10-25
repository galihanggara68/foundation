# Deterministic Entity Table Pluralization and @table Trait

Entities lower to relational database tables using a deterministic, zero-dependency English pluralization rule set by default, clashing with neither SQL reserved keywords (e.g. `order`, `user`, `group`) nor requiring manual pluralization in common cases. For legacy database mapping or irregular domain terms, domain authors can explicitly specify table names using the `@table("custom_name")` trait on Entity declarations.

## Considered Options

- **Keep singular table names (`order`, `user`)**: Rejected because `order` and `user` are ANSI SQL reserved keywords requiring brittle quote-escaping in raw SQL and causing syntax errors across multiple database tools.
- **External pluralization library**: Rejected because the Foundation compiler maintains 100% offline hermetic compilation with zero external runtime or build dependencies.
- **Mandatory `@table` on every Entity**: Rejected because requiring manual table naming violates Foundation's zero-boilerplate 90% principle for greenfield applications.

## Consequences

- The compiler includes a compact, deterministic pluralizer handling standard English suffixes (`-s`, `-es`, `-ies`, `-ves`) without network or external package dependencies.
- Irregular nouns (e.g., `Person` -> `people`) or specific legacy tables must be declared explicitly with `@table("name")`.
- Existing project workspaces migrating to this version will generate `ALTER TABLE <singular> RENAME TO <plural>;` migrations via the schema planner.
