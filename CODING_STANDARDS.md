# Foundation Compiler Coding Standards

This document defines the architectural, SOLID, Clean Code, and Deep Module standards for the `foundation` compiler codebase (`cmd/foundation` and `pkg/compiler`). The `code-review` skill enforces these rules on every change.

---

## 1. Deep Module Architecture & Stage-Aligned Package Layout

- **Single Public Compiler Driver Seam (`pkg/compiler`)**:
  - External callers (`cmd/foundation` and integration tests) interact exclusively through the deep top-level driver interface: `Check`, `Build`, `IR`, `Test`, and `LSP`.
  - Keep the public interface small and stable; hide lexer, parser, pass orchestration, lowering strategies, and emitter implementations inside `pkg/compiler/internal/`.
- **Stage-Aligned Subpackages ([ADR 0002](./docs/adr/0002-three-stage-architecture-lowering.md))**:
  - `pkg/compiler/internal/ir`: Pure domain data structures (`SemanticIR`, `StepDecl`, `Expr`, `Diagnostic`, `Span`) and shared AST/IR visitors (`WalkStepsAndExprs`, `WalkExpr`). Zero I/O or stage logic.
  - `pkg/compiler/internal/frontend` (**Stage 1 Parsing**): Lexer and recursive-descent `.fdn` parser split by grammar concern (`lexer.go`, `parser.go`, `parser_steps.go`, `parser_expr.go`). Zero knowledge of target languages or architectures.
  - `pkg/compiler/internal/validate` (**Stage 1 Validation**): Composable `Pass` implementations (`DeclarationPass`, `ActionTypeCheckPass`, `TerminationPass`, `ScenarioTypeCheckPass`) and `SymbolTable`.
  - `pkg/compiler/internal/arch` (**Stage 2 Middle-End Lowering**): `BuildProfile` loader (`profile.go`), self-contained `ArchitectureIR` (`ir.go`), and `PatternLowerer` strategies (`lower_hexagonal.go`, etc.). `ArchitectureIR` must **never** embed a raw `*ir.SemanticIR` pointer.
  - `pkg/compiler/internal/emit` (**Stage 3 Backend Emission**): `TargetEmitter` interface, `.foundation-manifest.json` reconciliation (`manifest.go`), and per-target emitter subpackages (e.g., `emit/gohex` split into `emitter.go`, `domain.go`, `usecase.go`, `adapters.go`, `scenarios.go`). Emitters must **never** import `frontend` or `validate`.
- **Cognitive Load & File Size Discipline**:
  - Avoid monolithic "God files". Keep individual `.go` files focused on a single sub-concern (strictly $\le 450$ lines per file across `cmd/`, `pkg/`, `contrib/`, and `test/`). When a parser, validator, or emitter grows, split it along clear conceptual seams (`domain.go`, `usecase.go`, `adapters.go`, `scenarios.go`).
- **Test Taxonomy & Packaging**:
  - **Unit Tests**: Co-located directly with source code (`*_test.go` inside `cmd/`, `pkg/compiler/internal/...`, and `contrib/...`). Focused on fast, hermetic, in-memory validation of individual components.
  - **Integration & Battle Tests (`test/`)**: Separated into top-level black-box suites that consume `pkg/compiler` strictly through public interfaces:
    - `test/e2e/`: Full compiler-to-runnable-app journeys, generator plugins, and generated client SDKs.
    - `test/security/`: Manifest sandboxing, path traversal (Zip-Slip) prevention, and rogue plugin isolation.
    - `test/durability/`: Live HTTP concurrency floods, abrupt socket disconnections, and graceful drain (`SIGTERM`).
    - `test/scale/`: Extreme scale benchmarks (500+ entities, 5,000+ fields, deep DAGs).

---

## 2. SOLID Principles & Design Patterns in the Compiler

- **Single Responsibility Principle (SRP)**:
  - Each file and type in `pkg/compiler` and `pkg/compiler/internal/*` must have one clear responsibility and one reason to change.
  - Keep functions focused and small. Extract cohesive helper methods rather than writing multi-screen switch cascades.
- **Open-Closed Principle (OCP) via Strategy & Pipeline Patterns**:
  - **Frontend Validation Passes (`validate.Pass`)**: Semantic analysis and architectural invariant checks ([ADR 0004](./docs/adr/0004-static-architectural-invariants.md)) are implemented as composable `validate.Pass` strategies executed in sequence. Adding a new static check adds a pass without tangling existing type-checking logic.
  - **Middle-End Lowerers (`arch.PatternLowerer`)**: Architecture lowering is dispatched through an `arch.PatternLowerer` strategy keyed by `arch.TargetKey` (`Kind`, `Language`, `Architecture`). Adding `layered` or `clean` lowering must not require modifying `hexagonal` lowering logic.
  - **Backend Emitters (`emit.TargetEmitter`)**: Code generation is dispatched through an `emit.TargetEmitter` strategy keyed by `arch.TargetKey`, with each target isolated in its own subpackage under `pkg/compiler/internal/emit/<target>`.
- **Dependency Inversion Principle (DIP)**:
  - High-level compiler orchestration (`Check`, `Build` in `pkg/compiler`) depends on the `validate.Validate`, `arch.Lower`, and `emit.TargetEmitter` abstractions rather than hardcoded concrete target branches.
- **Interface Segregation & Liskov Substitution (ISP & LSP)**:
  - Keep internal interfaces narrow (1–2 methods, such as `validate.Validate(*ir.SemanticIR) []ir.Diagnostic`, `arch.Lower(...)`, `emit.TargetEmitter.Emit(...)`) so every implementation fulfills the full contract without no-op methods or refused bequests.

---

## 3. Clean Code Conventions

- **Domain Vocabulary ([GLOSSARY.md](./GLOSSARY.md))**:
  - Use canonical domain terms (`Blueprint`, `Entity`, `Value`, `Policy`, `Action`, `Slot`, `Trait`, `BuildProfile`, `SemanticIR`, `ArchitectureIR`, `Emitter`, `Scenario`, `GenerationManifest`, `SlotStub`, `ContractTest`) in type names, variable names, and diagnostics.
- **Avoid Primitive Obsession**:
  - Use dedicated typed constants/structs (such as `Severity`, `StepKind`, `ExprKind`, `TargetKey`, `Emission`) instead of ad-hoc untyped maps or multi-string tuples.
- **Zero Duplication (DRY)**:
  - Share common AST/IR lookup and traversal helpers (such as `ir.WalkStepsAndExprs` and `validate.SymbolTable`) instead of duplicating recursive switches across passes and lowerers.
- **Deterministic Output**:
  - Never iterate over Go `map`s to emit source code, SQL, or JSON without sorting keys first. Every compiler build must be byte-for-byte deterministic.
