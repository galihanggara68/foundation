# Build Profile and Configuration

A single configuration file configures Foundation projects: `foundation.build.yaml`.
This file sets target architectures, storage dialects, migration strategies, generator plugins, and test commands.

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

authn: "jwt-oidc"              # "jwt-oidc" | "dev-headers" (default, dev-only)

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

### Authentication (`authn:`)

The `authn:` setting selects the actor resolver for generated backends.
The compiler validates both values at build time (`FDN0310`):

- `dev-headers` (default): Trusts `X-Actor-ID` and `X-Actor-*` headers without cryptographic checks. The server prints a startup banner and a warning log line. Use this mode only for local development.
- `jwt-oidc`: Verifies `Authorization: Bearer <token>` against the identity provider key set before resolving the actor.

The `jwt-oidc` resolver reads configuration from environment variables:

| Variable | Purpose |
| :--- | :--- |
| `AUTH_JWKS_URL` | Key set endpoint for signature verification. This variable is required. The server rejects requests when unset. |
| `AUTH_ISSUER` | Expected `iss` claim. The resolver validates this claim when set. |
| `AUTH_AUDIENCE` | Expected `aud` claim. The resolver validates this claim when set. |

The resolver accepts only `RS256` and `ES256` algorithms.
Tokens must include an `exp` claim.
The resolver reads claims only after confirming the signature.
Forged tokens never reach authorization policies.
See [ADR 0017](../adr/0017-jwt-verification-contract.md) for more details.

## 2. Target Configuration Parameters

| Field | Type | Description |
|---|---|---|
| `name` | `string` | Unique identifier for compilation target. |
| `kind` | `string` | `"backend"` (server application) or `"sdk"` (client SDK). |
| `language` | `string` | Target language (`"go"`, `"typescript"`, or external language name). |
| `architecture` | `string` | Architecture pattern (`"hexagonal"`, `"layered"`, `"clean"`). |
| `http` | `string` | HTTP framework adapter (`"chi"`, `"express"`). |
| `out_dir` | `string` | Output directory relative to repository root. |
| `generator` | `string` | *(Optional)* Name or path of a single external generator binary. |
| `generators` | `[]string` | *(Optional)* List of generator plugin binaries run in sequence. |
| `scaffold` | `string` | *(Optional)* `"builtin"` (pre-populates built-in scaffolding) or `"none"` (empty tree). |
| `options` | `map[string]string` | *(Optional)* Custom key-value options passed to external plugins. |
| `test_command` | `string` | *(Optional)* Command executed by `foundation test` for this target. |
| `test_timeout` | `string` | *(Optional)* Test runner timeout duration (for example `"60s"` or `"2m"`). |
| `diagnostic_prefix` | `string` | *(Optional)* Diagnostic code prefix owned by this target's generators (for example `ACME`). The `FDN` prefix is reserved. |

---

## 3. Storage Modes and DDL Generation

Foundation provides flexible database schema strategies controlled by `storage.mode`:

| Mode | Generated Artifacts | Usage Scenario |
|---|---|---|
| `declarative` | `schema.sql` | Idempotent DDL statements (`CREATE TABLE IF NOT EXISTS`). Use for rapid prototyping, Docker development, and serverless environments. |
| `migrations` | `migrations/000001_initial.up.sql`<br>`migrations/000001_initial.down.sql` | Versioned sequential migrations for tools like `golang-migrate`, `flyway`, `goose`, or `atlas`. Use for production CI/CD. |
| `both` | Declarative DDL and sequential migrations | Provides full DDL for local test containers while maintaining clean migration tracks for production releases. |

### Tactical Tables in Generated Schemas

When blueprints use operational traits (`@outbox`, `@idempotent`), the compiler injects table definitions (`outbox_events`, `idempotency_keys`) into declarative schemas and migration scripts.

---

## 4. CLI Command Reference

The Foundation CLI (`foundation`) provides commands for validating, compiling, and testing blueprints:

### `foundation check`

The `foundation check` command analyzes all `.fdn` files in the workspace:
- Validates syntax and AST structure.
- Resolves type references (`entity`, `value`, `enum`, `error`).
- Verifies authorization policies and actor references.
- Flags transaction safety violations (for example calling an `@external` slot inside `@transactional`).

```bash
foundation check
```

### `foundation build`

The `foundation build` command compiles `.fdn` blueprints and configuration into production code:
- Generates domain models, repositories, routers, and dependency containers.
- Generates tactical table DDL and sequential migrations.
- Runs external generator pipelines (`generators: [...]`) if configured.
- Reconciles `.foundation-manifest.json`. The compiler removes obsolete files and protects developer stubs.

```bash
foundation build
```

### `foundation test`

The `foundation test` command runs test suites across all configured targets:
- Built-in Go targets: Runs scenario tests with `go test ./...` offline.
- Built-in TypeScript targets: Runs scenario tests with `npm test` offline.
- External generator targets: Runs `test_command` (for example `pytest` or `cargo test`) with process group isolation and timeout limits.

```bash
foundation test
```

### `foundation ir`

The `foundation ir` command prints compiler Semantic IR JSON to standard output.
Use this command for inspection, tooling, and third-party linters.

```bash
foundation ir
```

---

## 5. The Build Manifest (`.foundation-manifest.json`)

Foundation generates a manifest to track compiler-managed files.
This manifest prevents overwrites and deletes obsolete files:

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

- **Safety**: The manifest omits handwritten slot files and plugin stubs (`config/*.starter.json`). Builds never overwrite or delete these files.
- **Cleanup**: When you delete an entity or action from a blueprint, the compiler removes only the corresponding generated files.
- **Plugin Provenance**: The compiler warns (`FDN0702`) if downstream plugins mutate files from upstream generators.
