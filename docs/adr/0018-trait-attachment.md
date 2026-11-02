# Trait Attachment Binds to the Following Declaration

The lexer discards newlines as whitespace, so `parseSlot` collected a method's traits with a loop that kept consuming `@` tokens — including ones written above the *next* method. A trait on the line above the following method was therefore read as the previous method's own: with two methods the first absorbed both traits and the second kept none, and with three it split 2/1/0. The mis-binding is not cosmetic — `@external` landed on the wrong method, so `FDN0213` fired on a safe call and stayed silent on the genuinely external one. We deleted the post-signature loop, so a trait binds to the declaration that follows it.

## Context

The grammar is deliberately newline-insensitive: whitespace separates tokens and carries no meaning, so an action's traits may wrap across lines. `parseSlot` leaned on that insensitivity when it scanned for traits after a method's `throws` clause, but the scan could not tell a trailing trait from the next method's leading trait. Only the slot grammar had a trailing position at all; every other declaration already read its traits before its name. The ambiguity stayed latent because nothing in the repository writes a trailing trait — the reference pages taught `slot PaymentGateway @external`, a form the parser never accepted.

## Decision

1. **A trait attaches to the declaration that follows it.** `@external` is written above the method it governs, the rule entities and actions already follow.
2. **The trailing-trait loop is deleted.** A slot method's `Traits` is exactly the traits that preceded its name.
3. **A trait ahead of a `slot` is an error, not a silent no-op.** `SlotDecl` has no trait field to store it, so `@external slot Foo {` is reported as `FDN0223`: attach the trait to each method instead.
4. **No new token.** Newline-insensitivity is unchanged.

## Considered Options

- **A newline token or lexer terminator.** Precise, but the language is deliberately newline-insensitive; giving newlines meaning would touch the whole grammar to resolve one ambiguous construct.
- **An explicit `;` separator between slot methods.** Unambiguous, but churn for every existing slot Blueprint to fix a construct nobody writes.
- **Bind forward and delete the trailing loop (chosen).** One rule, no grammar change, and no known user to migrate.

## Consequences

- A trailing trait no longer binds; the reference docs now write `@external Charge(...)` per method instead of annotating the slot.
- A trait before a `slot` declaration is `FDN0223` rather than silently discarded, so the compiler never drops a declared trait without saying so.
- A future construct that genuinely needs a trailing annotation can reintroduce a terminator, with this ADR as the record of why it was left out.
