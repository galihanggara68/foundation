# Closed Field-Kind Taxonomy in the Semantic IR

The Semantic IR previously exposed a field's type only as a free-form string (`FieldDecl.Type`), which forced every Emitter to re-derive type semantics — scalar-ness, enum vs JSON/blob, `uuid` vs plain `string` — through its own parallel string switches. Because emitters are external to the compiler, their mistakes were invisible: an unrecognized spelling silently produced a wrong identifier in generated code. We therefore gave every FieldDecl a closed, compiler-validated **Field Kind**, resolved by the compiler from the declared type and drawn from a fixed vocabulary (the six scalars, `enum`, the reference kinds `entity`/`value`/`event`, and the built-in kinds `actor`/`page`/`result`). A type the compiler cannot classify is now a compile error (`FDN0218`) rather than a string-map fallback in the emitter.

## Considered Options

- **Keep the type open, add conventions/docs.** No enforcement; relies on every emitter to agree, which is exactly what already drifted.
- **A per-emitter kind map.** N duplications of the same classification, still free to disagree with each other and with the compiler.
- **Closed, compiler-owned taxonomy (chosen).** The compiler is the single authority; emitters map declared positions as a total function and can never receive an unclassified field. Migration is incremental — the Go hexagonal emitter is the first consumer.

## Consequences

- The declared type string is retained alongside the Kind: it carries the declared name (e.g. `Customer`) and array-ness (`[]string`), while the Kind is the *element* kind. Emitters read both.
- Adding a new declarable type is now a compiler change (a new Kind), not something an emitter can introduce on its own.
- The vocabulary is deliberately limited to types that already exist; no `json`/`blob` type is declarable today, so none is classified.
- Field-type failures change diagnostic from `FDN0202` to `FDN0218`, distinguishing "cannot be classified into a field kind" from the open-ended unknown-type check still used for parameters and returns.
- Emitter migration is incremental: the Go hexagonal emitter consumes Kind today, while other in-tree emitters and generator plugins keep string-matching the declared type until they migrate.
