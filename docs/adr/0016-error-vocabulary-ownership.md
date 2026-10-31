# Compiler-Owned Standard Error Vocabulary

A declared error reached emitters as a name, a free-form `kind` and a message; each emitter re-derived the rest — `gohex` and `tsclean` carried their own kind→status switch, `gosdk` and `tssdk` their own reverse status→kind map — and none could express whether a message was the caller's business or the service's. We therefore made the compiler the owner of a **standard error vocabulary**: one table of kinds, each with its default HTTP status, its posture, and its name reserved. A declaration may override that default with `status:` and `posture:`, and carry per-locale text with `messages:`.

## Context

A status code instantiates a client-side error type, so error mapping is a contract, and it was implicit in each emitter's switch and in the reference docs. Those copies had begun to disagree: the documented kind list omitted `unauthenticated` while listing `unauthorized`. Nothing checks that two emitters agree, exactly as nothing checked field-type mapping before ADR 0015.

## Decision

1. **The compiler owns kind → `{status, posture}`.** `pkg/compiler/internal/platform` holds the one table; the validator and `gohex` read it rather than repeating a switch.
2. **The compiler reserves each kind's name.** `NotFound`, `Conflict` and the rest belong to the vocabulary, so a declaration reusing one stays legal but is reported (`FDN0219`): it risks being shadowed downstream.
3. **A declaration wins over the kind default.** Resolved `Status` and `Posture` are written onto every `ErrorDecl`, so omitting `status:`/`posture:` reproduces the previous behaviour. The generated transport mirrors the rule: a stamped value wins, and the kind's policy is the fallback.
4. **Posture is data, not a convention.** `business` text is the caller's; a `technical` message is an internal diagnostic, which `gohex` logs and replaces with a generic body.

## Considered Options

- **Per-emitter kind tables (the status quo).** N copies free to disagree, nothing to detect it.
- **Free-form kind strings, documented.** No enforcement; a typo becomes a wrong status at runtime.
- **Compiler-owned vocabulary and reserved names (chosen).** One authority, total mappings downstream, decidable at validation time.

## Consequences

- Changing a kind's default status is a **breaking change**: generated servers and SDKs branch on the status and on the kind, so the table changes only deliberately.
- The vocabulary keeps both `unauthenticated` and `unauthorized`, each answering 401; dropping a kind would invalidate Blueprints that declare it.
- The vocabulary is not published on the wire: its consumers are in-process emitters, which never see a `GenerationRequest`. Publishing it later is additive.
- Locales are recorded, never negotiated: the compiler does not invent a default locale.
- Migration is incremental: `gohex` consumes the resolved values today; `tsclean`, `gosdk` and `tssdk` still map kinds themselves, so masking reaches a caller in Go only.
