# ErrorDecl v2: MVP Brief

**MVP:** As an emitter author, I want every declared error to carry its posture, its HTTP status (declared, else defaulted from its kind), and its per-locale messages, backed by a compiler-owned standard vocabulary — so that error machinery is stamped from declaration data and a collision with the platform vocabulary is reported instead of being silently shadowed.

Scope source: item 3 of [spec 05](../specs/05-emitter-feedback-ir-and-protocol-evolution.md) — the item the spec says warrants an ADR ("error v2 + platform-vocabulary ownership").

## Target user

The author of an emitter or an SDK generator that has to turn a declared error into a status code and a message. Today each one re-derives both: `gohex` and `tsclean` each carry their own kind→status switch, `gosdk` and `tssdk` each carry their own reverse status→kind map, and none of them can express "this error is the caller's problem" versus "this error is ours".

## Success signal

- A single table in `internal/platform` is the only place the standard vocabulary lives; the validator and the emitters read it instead of repeating it.
- `gohex`'s generated `writeError` stops switching on `Kind` and writes the status the compiler resolved.
- For every blueprint that declares no `status`/`posture`, generated behaviour is unchanged.
- A declared error name that collides with the platform vocabulary is reported (`FDN0219`) instead of being silently shadowed downstream.

## In scope

- **Standard vocabulary in the leaf `internal/platform` package**: kind → `{status, posture}`, plus the reserved names derived from it.
- **`ErrorDecl` gains resolved `Status`, `Posture`, and `Messages`**, with `Kind` becoming a closed typed set (`ir.ErrorKind`, mirroring `FieldKind`). Resolved values are always emitted, so no emitter re-derives them.
- **Grammar**: `status: <int>`, `posture: business|technical`, and a `messages: { <locale>: "<text>" }` block. `message:` is unchanged and stays the fallback.
- **Diagnostics**: `FDN0220` (status outside 100–599), `FDN0221` (posture not `business`/`technical`), `FDN0219` (name collides with the standard vocabulary, warning), `FDN0108` (duplicate locale key).
- **`gohex` consumption** as the proof consumer.

## Out of scope

- **`tsclean`, `gosdk`, `tssdk` migration** — same table, their own tickets. The drift they carry is recorded, not fixed here.
- **Validating `throws` clauses.** `SlotMethodDecl.Throws` is parsed and emitted but never checked against declared errors; `validateErrorRef` exists but is wired only to else-clauses.
- **Publishing the vocabulary on the request's `platform` section.** Its only consumers are in-process emitters, which never see a `GenerationRequest` — publishing it would repeat the unconsumed-field trap trimmed from `identity`/`db_schema`. Adding it later is additive.
- **Locale negotiation and a default locale.** The compiler records declared locales; it does not invent one.
- Spec items 4–8, any `protocol_version` bump, any `ArchitectureIR` change.

## Key assumptions and risks

1. **The kind defaults must reproduce today's switches exactly**, or every existing blueprint silently changes behaviour through no fault of its author. The vocabulary table plus a per-kind equivalence test is the guard.
2. **Byte-identical output is not a goal here** (unlike items 1–2). Consuming a resolved status necessarily changes the emitted `DomainError` and `writeError`, so assertions pinning error emission are updated deliberately rather than held.
3. **`validate.Validate`'s caller aborts on any diagnostic**, warning included (`compiler.go:94`). A warning-severity diagnostic must therefore arrive with a fix to that check, or it silently becomes fatal.
4. **Error vocabulary is a contract.** Once generated apps and SDKs branch on a status, changing a kind's default status is a breaking change, so the table is versioned by the ADR.

## Next step

Work the tickets in `.scratch/errordecl-v2/issues/` — 01 first; 02–05 follow.
