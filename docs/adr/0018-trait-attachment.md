# Trait Attachment Binds to the Following Declaration

## Context

The compiler lexer treats newlines as whitespace without semantic meaning.
A trailing loop in slot method parsing consumed traits across line boundaries.
That behavior bound leading traits of subsequent methods to previous methods.
Consequently, the compiler applied `@external` traits to wrong slot methods.

## Decision

1. **Forward Attachment**: Traits attach to the declaration that follows the trait.
2. **Remove Trailing Loop**: The slot parser collects traits that precede each method name.
3. **Explicit Diagnostic**: The compiler rejects traits before `slot` keywords with error `FDN0223`.
4. **Preserve Whitespace Rules**: The language retains newline-insensitive parsing across declarations.

## Consequences

- Slot methods receive only their own declared traits.
- The compiler rejects traits written before `slot` blocks instead of discarding them.
- Documentation standardizes on method-level `@external` annotations.
