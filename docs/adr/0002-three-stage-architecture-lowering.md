# Three-Stage Compilation with Architecture Lowering

## Context

Coupling structural patterns directly into blueprint files causes maintenance problems.
Teams had to rewrite domain specifications whenever their application architecture changed.
The compiler needed a modular pipeline to support multiple architectural patterns.

## Decision

Foundation separates compilation into three distinct stages:
1. Blueprint source to **Semantic IR** (pure domain representation).
2. Semantic IR to **Architecture IR** (structural pattern lowering guided by the **Build Profile**).
3. Architecture IR to target language source code.

Tactical behavioral guarantees (such as idempotency, caching, and retries) remain declared inline as **Traits** in blueprints.

## Consequences

- Teams recompile one blueprint into different architectural patterns by changing the Build Profile.
- The Semantic IR stays independent of target frameworks and folder structures.
- Middle-end lowering isolates architectural concerns from domain logic.
