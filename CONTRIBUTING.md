# Contributing to Foundation

Thank you for your interest in contributing to **Foundation**!

Foundation is a deterministic, domain-specific compiler and developer platform designed to eliminate repetitive backend plumbing without runtime magic or non-deterministic code generation.

To maintain architectural integrity, determinism, and extreme performance, we uphold strict engineering standards across our codebase. Please review these guidelines before submitting a contribution.

---

## 🏛️ Core Architectural Principles

Every contribution must align with Foundation's fundamental architectural pillars:

1. **Zero External Dependencies in Core (`pkg/compiler`)**:
   The Foundation compiler core depends strictly on the Go standard library. We do not accept third-party dependencies in `pkg/compiler`. This protects our users from supply-chain vulnerabilities, build bloat, and version conflicts.
2. **Deterministic, Offline Compilation**:
   Given identical inputs, the compiler must emit bit-for-bit identical outputs across any machine or operating system. Avoid un-ordered map iteration in emitters, non-deterministic random IDs, or runtime timestamps in generated code.
3. **Defense-in-Depth & Filesystem Respect**:
   File generation must strictly honor directory confinement (`safeDiskPath`). Subprocesses must be isolated with strict timeout and signal handling.
4. **The 90/10 Seam (Slots)**:
   Generated code must cleanly separate compiled plumbing from handwritten logic via strongly-typed Slots. Custom code should never be overwritten.
5. **Strict File Size Discipline ($\le 450$ Lines)**:
   Every `.go` file in `cmd/`, `pkg/`, `contrib/`, and `test/` must remain strictly $\le 450$ lines. Decompose large files into focused, deep modules with high cohesion and low coupling.

---

## 🛠️ Development Setup & Prerequisites

* **Go**: `1.23` or newer
* **Node.js**: `20+` (optional, needed only for TypeScript runtime tests in `contrib/ts/runtime`)
* **Make**: GNU Make

### Clone and Build

```bash
# Clone the repository
git clone https://github.com/galihanggara68/foundation.git
cd foundation

# Build the Foundation CLI and reference plugins into ./bin/
make build

# Verify installation
./bin/foundation --help
```

---

## 🧪 Testing Workflow

Foundation categorizes its test suites into distinct layers. Run only what you need during rapid iteration, and run full verification before submitting changes:

```bash
# Rapid Unit Testing (<1s, hermetic)
make test-unit

# Integration & Protocol Tests
make test-e2e

# Adversarial & Sandboxing Security Suite
make test-security

# Live HTTP & Concurrency Durability Suite
make test-durability

# Heavy Compiler Scaling & Stress Suite
make test-scale

# Contrib Runtime Tests (Go & TypeScript)
make test-contrib

# Full Test Sweep
make test-all
```

---

## 🛡️ Pre-Commit Quality Gate

Before opening a Pull Request or pushing changes, **always run the local verification gate**:

```bash
make verify
```

`make verify` executes:
1. `make fmt-check`: Verifies all Go files adhere to standard `gofmt`.
2. `make line-check`: Asserts that **every** `.go` file across the repository is $\le 450$ lines.
3. `make vet`: Runs `go vet` across all packages.
4. `make test-unit`: Runs the fast unit test suite.

If any check fails, resolve the issue before opening a pull request. (Use `make fmt` to automatically format your code).

---

## 📐 Architecture Decision Records (ADRs)

If your contribution introduces:
* A new syntax construct in `.fdn` blueprints,
* A change to the generator plugin wire protocol (`foundation-gen-*`),
* A fundamental change to tactical traits or transaction semantics,

Please document the design rationale in an **Architecture Decision Record** in [`docs/adr/`](./docs/adr/). Reference existing ADRs (such as [ADR 0001](./docs/adr/0001-action-pipelines-and-slot-interfaces.md)) for structure.

---

## 🚀 Submitting a Pull Request

1. **Create a Topic Branch**:
   ```bash
   git checkout -b feat/my-new-feature
   # or
   git checkout -b fix/issue-description
   ```
2. **Keep Commits Atomic & Clear**:
   We recommend Conventional Commit messages:
   * `feat: add @ttl modifier to @cache trait`
   * `fix: prevent path traversal in manifest writer`
   * `docs: add recipe for custom sql join slot`
   * `test: add stress fuzzer for action parser`
3. **Verify Your Branch**:
   Ensure `make verify` and all relevant test suites pass cleanly.
4. **Submit Your PR**:
   Open your Pull Request against the `main` branch with a clear description of the problem solved and test coverage included.

Thank you for helping make Foundation the gold standard for reliable backend compilers!
