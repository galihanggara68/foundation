# Closed Field-Kind Taxonomy: MVP Brief

**MVP:** As an emitter author, I want every type-bearing IR position to carry a closed, compiler-validated Field Kind, so that my type mapping is a total function and an unknown kind fails the build instead of emitting a wrong identifier.

Scope source: item 1 of [spec 05](../specs/05-emitter-feedback-ir-and-protocol-evolution.md), narrowed to its smallest demoable slice.

## Target user

The author of an external generator plugin (`foundation-gen-<target>`) — someone building a new language or stack target on the `emitterkit` pattern (Rust, Python, or a bespoke Go-service generator).

Today they must re-invent an open-string → target-type mapping in every emitter and compensate for information the IR has thrown away: enums are indistinguishable from JSON/blob columns downstream, and `uuid` vs plain `string` needs a derive-side discriminator to pick the right SQL width. The cost is already paid — the private reference emitter shipped a real defect (`irTypeToGoType("error")` producing a non-existent `"Error"` identifier) and carries rules whose only purpose is to recover lost facts. Every future emitter pays it again.

## Success signal

The `gohex` Go emitter is migrated to switch on Field Kind with no string fallback, and both of these hold:

- `foundation build` output is byte-identical to today for the existing golden fixtures — proving the migration changed nothing for valid input; and
- a new fixture proves an unresolvable kind fails compilation with the named diagnostic — proving unknown kinds can no longer reach an emitter.

## In scope

- `Kind` carried on **every type-bearing IR position**: `FieldDecl`, `ParamDecl`, and `SlotMethodDecl.ReturnType` (not `FieldDecl` alone).
- A **closed vocabulary** classifying only the types that already exist: the six scalars (`uuid`, `string`, `int`, `decimal`, `bool`, `timestamp`), `enum`, the reference kinds (`entity`, `value`, `event`), and the built-ins (`actor`, `page`, `result`). Array-ness stays carried by the existing `[]` prefix on `type`; Kind holds the element kind.
- `kind` is **always emitted** (no `omitempty`), so new emitters can rely on its presence.
- **Compile-time resolution**: a pass fills Kind after symbol resolution; an unresolvable or unknown kind is a **compile error** with a dedicated named diagnostic (next free `FDN` code).
- **Additive schema**: `kind` added to the published JSON Schema in both copies (Go-embedded and `docs/schemas/generation-protocol-v1.json`) as an optional field, with a golden test asserting the two copies stay in sync and that v1 request documents without `kind` still parse.
- The **`gohex` emitter migration** (the proof consumer).
- An **ADR** recording the closed-taxonomy decision (hard to reverse, surprising without context, a real openness-vs-safety trade-off).

## Out of scope

- **Spec 05 items 2–8**: the Platform section in `GenerationRequest`; `ErrorDecl` v2; namespaced/validated traits; implied-capability materialization; the `emitterkit` harness and `-dump-model`; the Platform BOM; determinism-contract and storage-plan disposition.
- The **`emitterkit` per-language `Kind → target type` mapping helper** — deferred; the migrated consumer is the proof instead.
- **New declarable types.** No grammar change: `json`/`blob` is dropped from the MVP taxonomy because nothing maps to it today. Introducing a declarable `json` type is a language-feature effort, not a classification one.
- Migrating **any other in-repo consumer** (the storage-plan SQL mapper, `tsclean`, `gosdk`, or the `rich-ddd`/`otel`/`python-fastapi` plugins).
- Migrating the **private reference emitter** — it validates out-of-band after release; `gohex` stands in as the in-repo proxy.
- Any **`protocol_version` bump** or breaking change to `GenerationRequest`/`GenerationResponse`.
- `ArchitectureIR` changes; runtime LLM synthesis in plugins (ADR 0011).

## Key assumptions and risks

Riskiest first:

1. **A closed Kind is sufficient to make mapping total.** If the taxonomy cannot express something emitters genuinely need, the central thesis fails and the emitter still compensates. The migrated consumer is the test.
2. **A real consumer will adopt it.** The private reference emitter is out of repo, so `gohex` is a proxy; if no external emitter adopts Kind, the value stays theoretical.
3. **"Rest of the types" is a stable closed set.** The set is derived from today's validator (`scalarTypes`, plus `Actor`/`PageInfo`/`*Result`), so it can drift if new built-ins appear.
4. **Additivity holds.** Old emitters that ignore the new optional field remain unaffected — low risk, but it is the promise the whole approach rests on.

## Open questions

Unresolved; `/to-spec` must not silently assume:

- The exact `FDN` code for the unknown-kind diagnostic.
- `SlotMethodDecl.ReturnType` may be empty (no return) — does that yield no Kind at all, or a distinguished `void` kind?
- Do the built-ins `Actor`, `PageInfo`, and `*Result` get first-class kinds (`actor`/`page`/`result`), or a single `builtin` kind?
- Does `ParamDecl` need array/optional parity with `FieldDecl`, given it today carries only `Name` and `Type`?
- Should the two schema copies be consolidated into one source, or merely kept in sync by the golden test?

## Next step

Run `/to-spec` to turn this brief into a spec, then `/to-tickets`.
