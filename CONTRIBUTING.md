# Contributing to Foundation

This guide explains how to contribute to Foundation.
Foundation is an application compiler.
The compiler converts declarative blueprints into production code without large language models.

---

## Core Architectural Rules

Every contribution must follow these rules:

1. **Zero External Dependencies in Core (`pkg/compiler`)**:
   The compiler core depends only on the Go standard library.
   Do not add third-party dependencies to `pkg/compiler`.
2. **Deterministic Offline Compilation**:
   The compiler must produce identical outputs for identical inputs on all operating systems.
   Do not use unsorted map iterations, random identifiers, or timestamps in generated code.
3. **Defense in Depth**:
   File generation must stay inside the target output directory.
   Subprocesses must stop after strict timeouts.
4. **The Slot Seam**:
   Generated code separates compiled plumbing from handwritten logic through typed slots.
   The compiler never overwrites existing slot implementations.
5. **File Size Limit (450 Lines)**:
   Every Go source file in `cmd/`, `pkg/`, `contrib/`, and `test/` must have 450 lines or fewer.
   Split large files into small modules with clear responsibilities.

---

## Development Prerequisites

Install these tools before you start development:

- **Go**: Version `1.23` or newer
- **Node.js**: Version `20` or newer (optional, required for TypeScript tests)
- **Make**: GNU Make

### Clone and Build

1. Clone the repository:
   ```bash
   git clone https://github.com/galihanggara68/foundation.git
   cd foundation
   ```
2. Build the CLI and plugins:
   ```bash
   make build
   ```
3. Confirm the installation:
   ```bash
   ./bin/foundation version
   ```

---

## Test Workflow

Run tests during local development:

```bash
# Run unit tests
make test-unit

# Run end-to-end integration tests
make test-e2e

# Run security tests
make test-security

# Run durability and concurrency tests
make test-durability

# Run compiler scale tests
make test-scale

# Run all test suites
make test-all
```

---

## Pre-Commit Verification Gate

Always run the verification gate before you commit changes:

```bash
make verify
```

The verification command runs four checks:
1. `make fmt-check`: Confirms code formatting with `gofmt`.
2. `make line-check`: Confirms that every Go file has 450 lines or fewer.
3. `make vet`: Runs `go vet` across all packages.
4. `make test-unit`: Runs the unit test suite.

Fix all errors before you submit a pull request.
Run `make fmt` to format Go code automatically.

---

## Architecture Decision Records (ADRs)

Write an Architecture Decision Record in `docs/adr/` if your change adds:
- A new syntax feature in `.fdn` blueprints,
- A change to the generator plugin protocol, or
- A change to trait semantics or transaction rules.

Follow the format of existing records, such as [ADR 0001](./docs/adr/0001-action-pipelines-and-slot-interfaces.md).

---

## Pull Request Submission

Follow these steps to submit changes:

1. Create a topic branch:
   ```bash
   git checkout -b feat/my-new-feature
   ```
2. Write clear commit messages in the conventional commit format:
   - `feat: add ttl argument to cache trait`
   - `fix: prevent path traversal in manifest writer`
   - `docs: update blueprint declarations guide`
3. Run `make verify` and make sure that all tests pass.
4. Open a pull request against the `main` branch.
   Describe the problem and include test coverage.
