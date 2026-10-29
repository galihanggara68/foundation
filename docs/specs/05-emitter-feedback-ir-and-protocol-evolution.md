---
status: ready-for-agent
---

# Specification 05: Emitter-Feedback IR & Protocol Evolution

ADR basis: extends [ADR 0011](../adr/0011-pluggable-emitter-protocol.md) (pluggable emitter protocol) and [ADR 0014](../adr/0014-multi-generator-plugin-pipelines.md) (multi-generator pipelines) without breaking either.

## Problem Statement

The first production external emitter — a Go-service generator following the emitterkit pattern (Semantic IR → intermediate model → templates) — has completed its skeleton, error machinery, storage-snapshot, and identity phases. Building it surfaced every seam where the `SemanticIR` and `GenerationRequest` force an emitter to **guess, invent, or compensate** for facts the compiler owns. Each gap is re-work for every future emitter following the same pattern (spec 04's Rust/Python/reference generators included), and three of them produced real defects inside the emitter itself:

1. **Open type vocabulary.** `FieldDecl.Type` is a free string. The emitter maps strings → Go types → SQL types → scalar-ness in three parallel switches, and the open vocabulary caused: `irTypeToGoType("error")` emitting a non-existent `"Error"` identifier (real bug, fixed); enum fields indistinguishable from JSON/blob fields downstream (forcing enums out of the emitter's sortable-fields whitelist — a rule the emitter would rather not have); and `uuid` vs plain `string` needing a derive-side discriminator to pick `VARCHAR(255)`.
2. **Platform facts invented emitter-side.** The IR carries no middleware section, no identity/platform posture, no schema target, no pagination defaults — the emitter defined a middleware-selection model + defaults, hard-coded `X-User-ID`/`X-Request-ID`/bearer posture, and invented a `db_schema` request option. A second emitter reinvents all of it, and two emitters will drift.
3. **`ErrorDecl` is too thin.** `Name`/`Kind`/`Message` force every emitter to invent posture (Business/Technical), HTTP status mapping, and localization. The emitter's deferred backlog includes two items blocked exactly here: per-locale messages (its ID bundle's parity guard is green only because both locales carry English text) and collision diagnostics against platform-standard vocabulary (the compiler validates nothing, the emitter cannot see what it collides with — its own "translate, don't hide" rule fails silently across the process boundary).
4. **Trait taxonomy is unnamespaced.** `TraitDecl{Name, Args map[string]string}` makes `@max_len` (universal), `@unique`/`@index`/`@fk`/`@table` (storage-standard), and `@acme_readonly` (emitter-private) indistinguishable classes with unvalidated string args. An emitter cannot know what it may safely ignore, and nobody type-checks `@max_len(32)`.
5. **Implied capabilities are re-derived per emitter.** Declaring events implies an `EventDispatcher` port; the emitter synthesizes it. Whatever the compiler means by "declaring events implies X" should be materialized once in the IR, not re-derived N times.
6. **Debug and testing seams are per-emitter DIY.** `-dump-model` (IR→model inspection) and the four-seam harness (fixture → byte-determinism, model JSON goldens, tree goldens, `smoke` build-and-test) were built inside the emitter. Both are generator-agnostic shapes that belong in `emitterkit`.
7. **Dependency pins have no platform source.** Each emitter freezes its own gorm/otel/driver versions; the emitter composes pins from its model (a known merge-conflict spot in its own repo). Versions are platform decisions.

## Solution

Evolve the IR and request payload **additively under the existing `protocol_version`** (ADR 0011's stability promise intact — emitters that ignore new optional fields are unaffected, and defaults live compiler-side or in `emitterkit` helpers):

1. **Closed field-kind taxonomy** — `FieldDecl` carries a validated `Kind` (`uuid | string | int | decimal | bool | timestamp | enum | json | …`), checked at compile time; unknown kinds are a compiler diagnostic, never an emitter string-map fallback.
2. **Platform section in `GenerationRequest`** — one object carrying the facts every emitter needs as platform policy: `middleware` selection intent, identity posture (edge-minted headers, auth scheme), `db_schema` target, pagination defaults. The compiler owns defaults; emitters consume.
3. **`ErrorDecl` v2** — explicit posture, declared HTTP status (with type-default fallback), locale-keyed messages, and **compiler-side collision diagnostics** against the platform's standard vocabulary.
4. **Namespaced, validated traits** — reserved emitter namespaces (e.g. `@acme:*`), a validated standard set with typed args, and documentation of which standard traits every emitter must honor vs may ignore.
5. **Implied capabilities materialized** — the compiler inserts implied ports/slots into the `SemanticIR` (flagged as derived), so all emitters see the same capability surface.
6. **`emitterkit` promotion** — a standardized `-dump-model` contract (flag, stderr discipline, version stamp) and an `emitterkit/testing` package (fixture harness: byte-determinism, golden trees, smoke build-and-test) plus the tracked-vs-stub merge driver, so the next emitter wires fixtures instead of building a harness.
7. **Platform BOM** — compiler-published dependency versions (per language) emitters compose into their pins, ending per-emitter version drift.
8. **Determinism contract, written down** — every IR collection is either canonically sorted or explicitly declaration-ordered (documented per collection; `Canonicalize()` already does this for top-level slices, field order is only an accident today); and the storage-plan concept is decided — populated usefully or deleted — rather than left as a documented-but-empty trap.

Items 1–3 are prioritized: all three are already-paid emitter-side complexity, so the ROI is provable from the reference emitter's own tickets.

## User Stories

1. As an emitter author, I want every field to carry a closed, compiler-validated `Kind`, so that my type mapping is a total function and unknown kinds fail at compile time, never as a wrong identifier in generated code.
2. As an emitter author, I want enum fields distinguishable from JSON fields in the IR, so that I don't need emitter-side rules (like excluding enums from sort whitelists) whose only purpose is to compensate for lost information.
3. As an emitter author, I want platform policy (middleware intent, identity posture, schema target, pagination defaults) in the request payload, so that I consume platform decisions instead of hard-coding them and drifting from sibling emitters.
4. As an emitter author, I want error declarations to carry posture, status, and per-locale messages, so that error machinery is stamped from declaration data rather than invented per emitter.
5. As a blueprint author, I want the compiler to warn when my error code collides with platform-standard vocabulary, so that silent filtering never eats my declaration.
6. As an emitter author, I want `@acme:*`-style namespacing, so that I can ignore traits outside my namespace with compiler guarantee, and standard trait args are type-checked once, compiler-side.
7. As an emitter author, I want implied ports materialized in the IR, so that my capability surface matches every other emitter's without each of us re-deriving the implications.
8. As an emitter author, I want `emitterkit` to give me `-dump-model` and a fixture/golden/smoke harness, so that my emitter's testing story is wiring, not building.
9. As a platform engineer, I want one published BOM per language, so that all generated services share dependency versions and an upgrade is a compiler release, not an N-emitter migration.
10. As an emitter author, I want the IR's ordering guarantees documented per collection, so that byte-determinism in my output rests on contract, not observation.

## Implementation Plan

- **Ticket 01: Closed field-kind taxonomy** — `FieldKind` enum + compile-time validation + `FieldDecl.Kind` (JSON Schema additive field); emitterkit mapping helpers per language.
- **Ticket 02: Platform section in `GenerationRequest`** — middleware/identity/schema/pagination object with compiler defaults; `db_schema` promoted from an emitter-private request option to platform ownership.
- **Ticket 03: `ErrorDecl` v2** — posture, status, locale messages, collision diagnostics (`FDN` code) against the platform vocabulary.
- **Ticket 04: Trait namespaces & validation** — reserved namespaces, standard-trait registry with typed args, per-emitter ignore guarantees.
- **Ticket 05: Implied capability materialization** — event→dispatcher port (and future implications) inserted at IR construction, flagged derived.
- **Ticket 06: emitterkit harness** — `-dump-model` contract, `emitterkit/testing` (golden + smoke), tracked/stub merge driver extraction from reference emitters.
- **Ticket 07: Platform BOM** — published version sets per language; emitterkit pin-composition helper.
- **Ticket 08: Determinism & storage-plan disposition** — per-collection ordering contract in the JSON Schema docs; storage plan populated or removed from spec language.

## Testing Decisions & Agreed Seams

- **Schema seam**: published JSON Schema golden tests — additive evolution asserted (v1 request documents still parse).
- **Validation seam**: unknown `Kind`, unnamespaced trait args, and vocabulary collisions each produce named diagnostics (fixture blueprints per case).
- **Emitterkit seam**: the reference emitter consumes the harness unchanged (its existing fixture catalogue becomes the reference consumer); `-dump-model` output is golden-tested.
- **BOM seam**: generated `go.mod` from a reference emitter matches the BOM byte-for-byte.

## Decision Records Needed

Item 1 (closed taxonomy) and item 3 (error v2 + platform-vocabulary ownership) warrant ADRs at scheduling: both are hard to reverse once emitters depend on them, surprising without context, and real trade-offs (openness vs safety; declaration vs convention).

## Out of Scope

Breaking changes to `GenerationRequest`/`GenerationResponse` under the current `protocol_version`; `ArchitectureIR` changes; runtime LLM synthesis in plugins (per ADR 0011); non-JSON transports (stdio subprocess model stands).

## Evidence Base

Findings and defects cited above come from the reference emitter's private implementation history: its error-machinery and middleware-selection specs, its DDL storage snapshot, its platform purge, its deferred error-vocabulary backlog, and its batch review records (2026-10-28). No names or paths — the emitter is not public.
