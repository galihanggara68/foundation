# Foundation Compiler Coding Standards

This document defines coding standards for the compiler codebase (`cmd/foundation` and `pkg/compiler`).
The `code-review` agent checks these rules on every change.

---

## 1. Package Layout and Module Boundaries

### Compiler Driver Interface (`pkg/compiler`)
- External callers access the compiler only through top-level driver functions: `Check`, `Build`, `IR`, `Test`, `LSP`, and `CaptureRequest`.
- Keep the public interface small.
- Hide parsers, validators, lowering strategies, and emitters inside `pkg/compiler/internal/`.

### Subpackages by Compiler Stage
- `pkg/compiler/internal/ir`: Pure domain data structures (`SemanticIR`, `StepDecl`, `Expr`, `Diagnostic`, `Span`). Contains zero I/O or stage logic.
- `pkg/compiler/internal/frontend`: Lexer and parser for `.fdn` files. Contains zero target language logic.
- `pkg/compiler/internal/validate`: Validation passes that implement the `validate.Pass` interface.
- `pkg/compiler/internal/arch`: Lowering strategies that convert `SemanticIR` into `ArchitectureIR`. An `ArchitectureIR` must not store raw `*ir.SemanticIR` pointers.
- `pkg/compiler/internal/emit`: Emitters that write code for target stacks. Emitters must not import `frontend` or `validate`.

### File Size Limit
- Keep Go source files to 450 lines or fewer across `cmd/`, `pkg/`, `contrib/`, and `test/`.
- Split large files along clear boundaries (`domain.go`, `usecase.go`, `adapters.go`).

### Test Organization
- Put unit tests next to source code (`*_test.go`).
- Put black-box integration tests in `test/`:
  - `test/e2e/`: Full compilation journeys and SDK verification.
  - `test/security/`: Path traversal prevention and plugin isolation.
  - `test/durability/`: HTTP concurrency and graceful stop tests.
  - `test/scale/`: Large-scale compiler stress tests.

---

## 2. SOLID Design Rules

### Single Responsibility Rule
- Give each file and type one clear responsibility.
- Write small functions.
- Extract helper functions instead of writing long conditional branches.

### Strategy Pattern for Extensions
- Implement static checks as separate `validate.Pass` types.
- Add new architecture lowering patterns through the `arch.PatternLowerer` interface.
- Add new code generation targets through the `emit.TargetEmitter` interface.

### Narrow Interfaces
- Keep internal interfaces small (one or two methods).
- Every implementation must fulfill the complete interface contract.

---

## 3. Clean Code Rules

### Standard Domain Words
- Use canonical domain names from [GLOSSARY.md](./GLOSSARY.md) in types, variables, and diagnostics.
- Use names like `Blueprint`, `Entity`, `Value`, `Action`, `Slot`, and `Trait`.

### Avoid Primitive Obsession
- Use defined types and constants instead of untyped maps and generic strings.

### Do Not Repeat Code (DRY)
- Use shared AST traversal helpers (`ir.WalkStepsAndExprs`) instead of duplicate recursive switches.

### Deterministic Output
- Sort map keys before you generate code, SQL, or JSON.
- Every build must produce identical output bytes for identical input files.
