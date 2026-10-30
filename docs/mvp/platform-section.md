# Platform Section in the Generation Request: MVP Brief

**MVP:** As an emitter author, I want the generation request to carry a compiler-owned `platform` section — starting with pagination defaults — so that I consume platform policy instead of inventing it and drifting from sibling emitters.

Scope source: item 2 of [spec 05](../specs/05-emitter-feedback-ir-and-protocol-evolution.md), narrowed to the facts that have a real consumer today: compiler-computed defaults, published on the request, consumed by an in-repo plugin.

## Target user

The author of an external generator plugin (`foundation-gen-<target>`) following the `emitterkit` pattern — and specifically the *second* such author, who currently reinvents everything the first one invented.

The `GenerationRequest` today carries only `protocol_version`, `workspace_root`, `target`, `semantic_ir`, `architecture_ir`, and `files`. There is no middleware section, no identity posture, no schema target, and no pagination default anywhere in the compiler. So each emitter invents its own: one hard-codes `x-actor-*` fallback headers (`emit/tsclean/controllers.go`), several hard-code actor claim key names (`emit/gohex/authn.go`), one invented a `db_schema` request option (per the spec's evidence base), and the query-many limit cap of `100` sits as a literal in the validator (`validate/pass_query_many.go`). Two emitters producing the same service will disagree, and nothing tells an emitter what the platform decided.

The generic `Options map[string]string` on `TargetInfo` is the only escape hatch today; it carries no semantics, no defaults, and no type.

## Success signal

The compiler publishes the platform policy it owns, and a real plugin consumes it — with all of these holding:

- `foundation-gen-python-fastapi` reads `platform.pagination` and binds its repository reads to the compiler-owned limits — proving a plugin consumes platform policy instead of inventing its own;
- the compiler's own pagination ceiling has **one** source rather than a literal repeated across the validator, the emitter and the request; and
- the published protocol schema documents `platform` as an **optional** addition in both copies, with a test proving a `GenerationRequest` written before it existed still decodes and validates.

The in-process emitters (`gohex`, `tsclean`, `gosdk`, `tssdk`) are invoked as `emitter.Emit(archIR)` and never see a `GenerationRequest`, so they structurally cannot consume the section. Consumption is proven through a plugin, which is the audience the section exists for.

## In scope

- A `platform` object on `GenerationRequest` (additive, optional, no `protocol_version` bump) carrying **one** sub-object today:
  - **`pagination`** — pagination defaults (default limit and maximum limit).
- `identity` posture and a `db_schema` target were **dropped before publication**: neither had a consumer, and shipping unconsumed fields is the documented-but-empty trap spec item 8 warns about. They return as additive sub-objects once they have one, which the nested shape leaves room for.
- **Compiler-owned defaults in a leaf package**: the limits live in `internal/platform`, so the validator's ceiling, the emitter's fallback page size and the published request all read one source. The regenerated plugin output changes only where it previously invented its own number.
- **`foundation-gen-python-fastapi` consumption** as the proof consumer: the platform pagination policy drives the repository read limits it emits.
- **Additive schema**: `platform` added to the published JSON Schema in both copies (Go-embedded and `docs/schemas/generation-protocol-v1.json`) as an optional object, with a golden test asserting the two copies stay in sync and that v1 request documents without `platform` still parse.

## Out of scope

- **Spec 05 items 4–8**: namespaced/validated traits; implied-capability materialization; the `emitterkit` harness and `-dump-model`; the Platform BOM; determinism-contract and storage-plan disposition.
- **Identity posture and the database schema target.** Neither has a reachable consumer: identity's only in-repo reader is `gohex`'s actor claim keys, which sit on the in-process path and cannot be reached without an `ArchitectureIR` or emitter-interface change; and nothing in the compiler or build profile derives a non-empty schema name at all.
- **Middleware selection intent.** The compiler has no middleware model at all — the `otel` plugin injects middleware by string surgery on the emitted router (`cmd/foundation-gen-otel/main.go:31`). It is the only in-repo plugin that reinvents a platform fact, and the fact it reinvents is middleware wiring, so it moves only once an honest middleware model exists.
- **Author-facing declaration of platform policy.** No `platform:` build-profile section and no new validation or `FDN` diagnostics in this MVP; values are compiler-computed. A follow-up ticket adds declaration once the shape has proven itself.
- Migrating **any other in-repo consumer** (`gohex`, `tsclean`, `gosdk`, or the `rich-ddd`/`otel` plugins) — in-process emitters never see the request, and the remaining plugins have no platform-shaped hard-code to replace.
- Migrating the **private reference emitter** — out of repo; `foundation-gen-python-fastapi` stands in as the in-repo proxy.
- **Pagination grammar changes.** Keyset-cursor semantics and `query many` lowering stay exactly as they are (ROADMAP Phase 4); only the limit ceiling becomes platform-sourced.
- Any **`protocol_version` bump**, breaking change to `GenerationRequest`/`GenerationResponse`, or `ArchitectureIR` change.

## Key assumptions and risks

Riskiest first:

1. **Additivity holds against a real plugin.** A generator written before the section existed receives a request it never asked for and must keep working — the python generator keeps its unbounded read when `platform` is absent. The additive-decode test plus the plugin's nil-platform test are the proof.
2. **The `platform` shape is hard to change once emitters depend on it.** Like the Kind taxonomy, this is a contract that other repos will parse, so the enclosing object is nested rather than flat: adding `identity`/`db_schema` later is additive, removing a published field is not.
3. **A one-field section looks thin, and thin is the point.** The alternative was publishing `identity` and `db_schema` with no reader anywhere in the repo — verified, not assumed: no in-repo plugin hard-codes an identity, schema or pagination value, and the two that string-match types (python-fastapi, rich-ddd) are doing Kind work, not platform work.

## Open questions

Unresolved; later tickets must not silently assume:

- Does adding a protocol contract section warrant its own ADR? (The spec requires ADRs for items 1 and 3, not 2 — but emitters will depend on this shape.)
- Is `default_limit` meaningful yet, given `limit` is mandatory in the grammar? It currently serves request consumers only; the compiler's own validator uses `max_limit`.

Settled: the object is nested (`platform.pagination`), not flat, so `identity`/`db_schema` can return additively; the values are `{default_limit: 20, max_limit: 100}`, owned by the leaf `internal/platform` package.

## Next step

Tickets 01, 03 and 04 have landed; 02 is dropped. Next: commit the batch, then spec 05 item 3 (`ErrorDecl` v2).
