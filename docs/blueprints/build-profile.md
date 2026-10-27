# Build Profile & Configuration

Foundation projects are configured through a single root configuration file: `foundation.build.yaml`. This file governs target architecture selection, storage dialects, migration strategies, plugin generators, and test commands.

---

## 1. `foundation.build.yaml` Reference

```yaml
version: "1"

project:
  name: "payment-service"
  module: "github.com/example/payment-service"

storage:
  dialect: "postgres"          # "postgres" | "sqlite"
  mode: "both"                 # "declarative" | "migrations" | "both"

authn: "jwt-oidc"              # "jwt-oidc" | "trusted-headers" (dev)

targets:
  # Built-in Go Hexagonal Target
  - name: backend
    kind: backend
    language: go               # "go" | "typescript"
    architecture: hexagonal    # "hexagonal" | "layered" (Go) | "clean" (TS)
    http: chi                  # "chi" (Go) | "express" (TS)
    out_dir: ./dist/backend

  # Target with Multi-Generator Plugin Pipeline
  - name: enterprise-backend
    kind: backend
    language: go
    architecture: hexagonal
    http: chi
    scaffold: builtin          # "builtin" (default) | "none"
    generators:
      - "foundation-gen-rich-ddd" # Overwrites models with Aggregate Roots
      - "foundation-gen-otel"     # Injects OpenTelemetry tracing middleware
    options:
      service_name: "payment-service"
    out_dir: ./dist/enterprise

  # Clean-Slate External Target (Python / Rust)
  - name: python-client-api
    kind: backend
    language: python
    architecture: clean
    scaffold: none             # Starts with empty virtual file tree
    generator: "foundation-gen-python-fastapi"
    test_command: "pytest"     # Command run by `foundation test`
    test_timeout: "60s"        # Test timeout (default 60s)
    out_dir: ./dist/python-api

  # Strongly-Typed Client SDK
  - name: ts-client-sdk
    kind: sdk
    language: typescript
    out_dir: ./dist/sdk/ts
```

---

## 2. Target Configuration Parameters

| Field | Type | Description |
|---|---|---|
| `name` | `string` | Unique identifier for the compilation target. |
| `kind` | `string` | `"backend"` (server application) or `"sdk"` (client SDK). |
| `language` | `string` | Target language (`"go"`, `"typescript"`, or external language name). |
| `architecture` | `string` | Architecture pattern (`"hexagonal"`, `"layered"`, `"clean"`). |
| `http` | `string` | HTTP framework adapter (`"chi"`, `"express"`). |
| `out_dir` | `string` | Output directory relative to repository root. |
| `generator` | `string` | *(Optional)* Name or path of a single external generator binary. |
| `generators` | `[]string` | *(Optional)* List of generator plugin binaries executed sequentially in a pipeline. |
| `scaffold` | `string` | *(Optional)* `"builtin"` (pre-populates built-in target scaffolding) or `"none"` (clean slate). |
| `options` | `map[string]string` | *(Optional)* Custom key-value options passed to external plugins. |
| `test_command` | `string` | *(Optional)* Command executed by `foundation test` for this target. |
| `test_timeout` | `string` | *(Optional)* Test runner timeout duration (e.g., `"60s"`, `"2m"`). |

---

## 3. Storage Modes & DDL Generation

Foundation provides flexible database schema strategies controlled by `storage.mode`:

| Mode | Generated Artifacts | Usage Scenario |
|---|---|---|
| `declarative` | `schema.sql` | Idempotent DDL statements (`CREATE TABLE IF NOT EXISTS`). Ideal for rapid prototyping, Docker local dev, and serverless environments. |
| `migrations` | `migrations/000001_initial.up.sql`<br>`migrations/000001_initial.down.sql` | Versioned sequential migrations for tools like `golang-migrate`, `flyway`, `goose`, or `atlas`. Ideal for enterprise production CI/CD. |
| `both` | Both declarative DDL and sequential migrations | Provides full DDL for local test containers while maintaining clean migration tracks for production releases. |

### Tactical Tables in Generated Schemas
When blueprints use operational traits (`@outbox`, `@idempotent`), the compiler automatically injects table definitions (`outbox_events`, `idempotency_keys`) into both declarative schemas and migration scripts.

---

## 4. CLI Command Reference

The Foundation CLI (`foundation`) provides a developer workflow for validating, compiling, and testing blueprints:

### `foundation check`
Performs static analysis on all `.fdn` files in the current workspace.
- Validates syntax and AST integrity.
- Resolves all type references (`entity`, `value`, `enum`, `error`).
- Verifies authorization policies and actor references.
- Flags transaction safety violations (e.g. `@external` slot called inside `@transactional`).

```bash
foundation check
```

### `foundation build`
Compiles `.fdn` blueprints and configuration into executable production code.
- Generates domain models, repositories, routers, and dependency containers.
- Generates tactical table DDL and sequential migrations.
- Executes external generator pipelines (`generators: [...]`) if configured.
- Reconciles `.foundation-manifest.json`, pruning obsolete files while protecting developer stubs.

```bash
foundation build
```

### `foundation test`
Executes test suites across all configured targets:
- Built-in Go targets: runs scenario tests with `go test ./...` 100% offline.
- Built-in TypeScript targets: runs scenario tests with `npm test` 100% offline.
- External generator targets: delegates execution to `test_command` (e.g., `pytest`, `cargo test`) with process group isolation and timeout enforcement.

```bash
foundation test
```

### `foundation ir`
Dumps the Compiler Intermediate Representation (IR) to stdout. Useful for inspection, tooling, and third-party linters.

```bash
foundation ir
```

---

## 5. The Build Manifest (`.foundation-manifest.json`)

To prevent accidental overwrites and cleanly clean up deleted routes, Foundation generates a manifest tracking all compiler-managed files:

```json
{
  "version": "1.0.0",
  "blueprint_hash": "a1b2c3d4...",
  "generated_files": [
    {
      "path": "internal/domain/wallet_entity.go",
      "hash": "e5f6g7h8..."
    },
    {
      "path": "internal/transport/router.go",
      "hash": "i9j0k1l2..."
    }
  ],
  "test_command": "go test ./..."
}
```

- **Safety**: Handwritten slot files and plugin stubs (`config/*.starter.json`) are intentionally omitted from `generated_files` and are never overwritten or deleted by subsequent builds.
- **Cleanup**: If an entity or action is deleted from a `.fdn` blueprint, the compiler uses the manifest to delete only the corresponding generated files, keeping the repository clean.
- **Cross-Plugin Provenance**: Detects and warns (`FDN0702`) if downstream plugins mutate files authored by upstream generators.
