# Compiler-Owned Standard Error Vocabulary

## Context

Declared errors previously reached emitters as arbitrary `kind` strings.
Each emitter implemented custom mappings between kinds and HTTP status codes.
Emitter mappings drifted and lacked distinction between caller-facing messages and internal diagnostics.

## Decision

1. **Central Ownership**: The compiler platform package maintains the standard error vocabulary.
2. **Reserved Names**: The compiler reserves standard kind names and warns with `FDN0219` if declarations reuse them.
3. **Explicit Overrides**: Declarations can override default statuses with `status:` and posture with `posture:`.
4. **Structured Posture**: Posture classifies errors as `business` (caller visible) or `technical` (internal diagnostic).
5. **Locale Messages**: Declarations support per-locale translations with `messages:`.

## Consequences

- Compilers and emitters share one source of truth for error mappings.
- Generated transports log technical messages and hide internal details from callers.
- Blueprints record locale strings without runtime content negotiation.
