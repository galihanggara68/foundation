# Build Profile & Configuration

Foundation projects are configured through a single root configuration file: `foundation.build.yaml`. This file governs target architecture selection, storage dialects, migration strategies, and code generation parameters.

---

## 1. `foundation.build.yaml` Reference

```yaml
version: "1"
name: "payment-service"

target:
  language: "go"               # "go" | "typescript"
  architecture: "hexagonal"    # "hexagonal" | "clean"
  module: "github.com/example/payment-service"

storage:
  dialect: "postgres"          # "postgres" | "sqlite"
  mode: "both"                 # "declarative" | "migrations" | "both"
  migrations_dir: "db/migrations"
  schema_file: "db/schema.sql"

transport:
  http:
    enabled: true
    port: 8080
    base_path: "/api/v1"

observability:
  tracing: "opentelemetry"
  metrics: "prometheus"
```

---

## 2. Storage Modes & DDL Generation

Foundation provides flexible database schema strategies controlled by `storage.mode`:

| Mode | Generated Artifacts | Usage Scenario |
|---|---|---|
| `declarative` | `db/schema.sql` | Idempotent DDL statements (`CREATE TABLE IF NOT EXISTS`). Ideal for rapid prototyping, Docker local dev, and serverless environments. |
| `migrations` | `db/migrations/000001_initial.up.sql`<br>`db/migrations/000001_initial.down.sql` | Versioned sequential migrations for tools like `golang-migrate`, `flyway`, or Prisma. Ideal for enterprise production CI/CD. |
| `both` | Both declarative DDL and sequential migrations | Provides full DDL for local test containers while maintaining clean migration tracks for production releases. |

### Tactical Tables in Generated Schemas
When blueprints use operational traits (`@outbox`, `@idempotent`), the compiler automatically injects table definitions (`outbox_events`, `idempotency_keys`) into both declarative schemas and migration scripts.

---

## 3. CLI Command Reference

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
- Emits starter stubs for newly added `slot` declarations.
- Updates `.foundation.manifest.json`.

```bash
foundation build
```

### `foundation test`
Compiles and executes all `scenario` blocks defined across your blueprints.
- Seeds test data into ephemeral storage.
- Dispatches scenario actions.
- Asserts expectations, emitted events, and error kinds.
- Executes 100% offline with zero external network access.

```bash
foundation test
```

### `foundation ir`
Dumps the Compiler Intermediate Representation (IR) to stdout or JSON file. Useful for tooling, external linters, or visual diagram generators.

```bash
foundation ir --output ir.json
```

---

## 4. The Build Manifest (`.foundation.manifest.json`)

To prevent accidental overwrites and cleanly clean up deleted routes, Foundation generates a manifest tracking all compiler-managed files:

```json
{
  "version": "1.0.0",
  "blueprint_hash": "a1b2c3d4...",
  "generated_files": [
    {
      "path": "internal/domain/wallet.go",
      "hash": "e5f6g7h8..."
    },
    {
      "path": "internal/transport/router.go",
      "hash": "i9j0k1l2..."
    }
  ]
}
```

- **Safety**: Files not tracked in the manifest (such as `internal/slots/*` or custom middleware) are never touched by the compiler.
- **Cleanup**: If an action is deleted from a `.fdn` blueprint, the compiler uses the manifest to delete only the corresponding generated handlers, keeping the repo clean.
