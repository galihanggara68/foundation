# Purpose-Built `.fdn` Grammar over YAML/JSON

## Context

YAML and JSON work well for flat configuration data.
Expressing action pipelines, guards, queries, and type relationships in YAML forces logic into unvalidated string literals.
That format requires custom string parsers and loses accurate source line diagnostics.

## Decision

Developers author blueprints in a purpose-built, strongly typed `.fdn` grammar.
The compiler parses `.fdn` source files directly into the **Semantic IR**.
External tools can inspect the Semantic IR through canonical JSON serialization.

## Consequences

- Authors receive precise source line and column diagnostics during parsing.
- Blueprint syntax remains readable and compact.
- Tooling receives structured JSON through `foundation ir`.
