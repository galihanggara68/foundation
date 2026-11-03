# Constrained Action Pipelines with Generated Slot Interfaces

## Context

A general-purpose transpiler fails across languages with incompatible memory and concurrency models (such as Rust, Go, and TypeScript).
Mixing handwritten code into generated files with protected comment blocks breaks deterministic regeneration.
The compiler required a clean boundary between generated logic and custom algorithms.

## Decision

Foundation expresses application logic through constrained, declarative Action Pipelines rather than a general-purpose language.
The compiler isolates custom imperative logic behind generated native interfaces (**Slots**).
Developers write custom algorithms and third-party integrations inside these slot implementations.

## Consequences

- The compiler regenerates all generated files deterministically at any time.
- Custom code lives in dedicated slot files outside the generated core.
- Action pipelines remain verifiable and portable across all target architectures.
