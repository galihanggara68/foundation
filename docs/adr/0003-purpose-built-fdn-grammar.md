# Purpose-Built `.fdn` Grammar over YAML/JSON

Blueprints are authored in a purpose-built, strongly-typed `.fdn` grammar rather than YAML or JSON. While YAML/JSON works for flat configuration, expressing Action Pipelines (guards, queries, expressions, and type relationships) in YAML forces logic into unvalidated string literals—recreating the need for a custom parser while losing line/column diagnostics and readability. The compiler parses `.fdn` directly into the **Semantic IR**, which can later be serialized to canonical JSON for external tooling integration.
