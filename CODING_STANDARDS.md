# Foundation Compiler Coding Standards

This document defines the architectural, SOLID, Clean Code, and Deep Module standards for the `foundation` compiler codebase (`cmd/foundation` and `pkg/compiler`). The `code-review` skill enforces these rules on every change.

---

## 1. Deep Module Architecture & Seam Discipline

- **Single Public Compiler Driver Seam (`pkg/compiler`)**:
  - External callers (`cmd/foundation` and integration tests) interact through the deep top-level driver interface: `Check`, `Build`, `IR`, and `Test`.
  - Keep the public interface small and stable; hide lexer, parser, pass orchestration, lowering strategies, and emitter implementations behind this seam.
- **Strict Three-Stage Pipeline Separation ([ADR 0002](./docs/adr/0002-three-stage-architecture-lowering.md))**:
  - **Stage 1 (Frontend)**: `.fdn` source $\rightarrow$ `SemanticIR` + `[]Diagnostic`. Must have zero knowledge of target languages (`go`, `typescript`), HTTP routers (`chi`, `express`), or architecture topologies (`hexagonal`, `layered`, `clean`).
  - **Stage 2 (Middle-End)**: `SemanticIR` + `BuildProfile` $\rightarrow$ `ArchitectureIR` (and `StoragePlan`). `ArchitectureIR` must be completely self-contained and must **never** embed a raw `*SemanticIR` pointer.
  - **Stage 3 (Backend)**: `ArchitectureIR` $\rightarrow$ `Emission` (`TrackedFiles`, `StubFiles`) $\rightarrow$ `ReconcileManifest`. Emitters must read only from `ArchitectureIR` and never reach back into `SemanticIR`.

---

## 2. SOLID Principles & Design Patterns in the Compiler

- **Single Responsibility Principle (SRP)**:
  - Each file and type in `pkg/compiler` must have one clear responsibility and one reason to change (e.g., lexical scanning, parsing, a specific semantic validation pass, a specific architecture lowerer, a specific target emitter, or manifest reconciliation).
  - Keep functions focused and small. Extract cohesive helper methods rather than writing multi-screen switch cascades.
- **Open-Closed Principle (OCP) via Strategy & Pipeline Patterns**:
  - **Frontend Validation Passes (`ValidationPass`)**: Semantic analysis and architectural invariant checks ([ADR 0004](./docs/adr/0004-static-architectural-invariants.md)) are implemented as composable `ValidationPass` strategies executed in sequence. Adding a new static check adds a pass without tangling existing type-checking logic.
  - **Middle-End Lowerers (`PatternLowerer`)**: Architecture lowering is dispatched through a `PatternLowerer` strategy keyed by `TargetKey` (`Kind`, `Language`, `Architecture`). Adding `layered` or `clean` lowering must not require modifying `hexagonal` lowering logic.
  - **Backend Emitters (`TargetEmitter`)**: Code generation is dispatched through a `TargetEmitter` strategy keyed by `TargetKey`. Each target emitter encapsulates its own formatting and template logic.
- **Dependency Inversion Principle (DIP)**:
  - High-level compiler orchestration (`Check`, `Build`) depends on the `ValidationPass`, `PatternLowerer`, and `TargetEmitter` abstractions rather than hardcoded concrete target branches.
- **Interface Segregation & Liskov Substitution (ISP & LSP)**:
  - Keep internal interfaces narrow (1–2 methods, such as `Validate(*SemanticIR) []Diagnostic`, `Lower(...)`, `Emit(...)`) so every implementation fulfills the full contract without no-op methods or refused bequests.

---

## 3. Clean Code Conventions

- **Domain Vocabulary ([GLOSSARY.md](./GLOSSARY.md))**:
  - Use canonical domain terms (`Blueprint`, `Entity`, `Value`, `Policy`, `Action`, `Slot`, `Trait`, `BuildProfile`, `SemanticIR`, `ArchitectureIR`, `Emitter`, `Scenario`, `GenerationManifest`, `SlotStub`, `ContractTest`) in type names, variable names, and diagnostics.
- **Avoid Primitive Obsession**:
  - Use dedicated typed constants/structs (such as `Severity`, `StepKind`, `ExprKind`, `TargetKey`, `Emission`) instead of ad-hoc untyped maps or multi-string tuples.
- **Zero Duplication (DRY)**:
  - Share common AST/IR lookup and validation helpers (such as symbol tables and struct field validators) instead of duplicating loops across `Entity` and `Value` handlers.
- **Deterministic Output**:
  - Never iterate over Go `map`s to emit source code, SQL, or JSON without sorting keys first. Every compiler build must be byte-for-byte deterministic.
