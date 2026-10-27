# Generator Plugins & Custom Emitters

Foundation provides a standardized, subprocess-based plugin architecture modeled after the `protoc` plugin ecosystem ([ADR 0011](../adr/0011-pluggable-emitter-protocol.md) and [ADR 0014](../adr/0014-multi-generator-plugin-pipelines.md)).

With Generator Plugins, you can:
1. **Target Any Language or Framework**: Generate backends for languages outside the core compiler (e.g., Python FastAPI, Rust Axum, C# .NET) with clean-slate generation (`scaffold: none`).
2. **Compose Specialized Pipelines**: Chain multiple plugins sequentially (`generators: [...]`) to customize specific slices (e.g., Rich DDD Aggregate Roots, OpenTelemetry distributed tracing, custom middleware) while reusing 90% of Foundation's built-in scaffolding (`scaffold: builtin`).
3. **Run Native Test Suites**: Run integration tests directly with native test runners (`pytest`, `cargo test`, `go test`) through `foundation test`.

---

## 1. Configuring Plugins in `foundation.build.yaml`

To use an external generator, declare `generator:` (single plugin) or `generators:` (sequential pipeline) under a target in `foundation.build.yaml`:

### Pattern A: Chained Pipeline with Scaffolding Pre-population
This pattern pre-populates Foundation's standard built-in scaffolding (ports, repositories, HTTP routing, database migrations) and passes the virtual files through a sequence of plugins:

```yaml
version: "1"
name: "order-service"

targets:
  - name: backend
    kind: backend
    language: go
    architecture: hexagonal
    http: chi
    scaffold: builtin # Default: built-in emitter runs first in memory
    generators:
      - "foundation-gen-rich-ddd" # Overwrites entities with Aggregate Roots
      - "foundation-gen-otel"     # Injects OpenTelemetry tracing middleware
    options:
      service_name: "order-service"
      sample_rate: 1.0
    out_dir: ./dist/backend
```

### Pattern B: Clean Slate Generation (New Languages & Frameworks)
This pattern starts with an empty file tree (`scaffold: none`) so a plugin has complete freedom to generate a custom stack:

```yaml
version: "1"
name: "catalog-service"

targets:
  - name: python-api
    kind: backend
    language: python
    architecture: clean
    scaffold: none # Virtual file tree starts empty
    generator: "foundation-gen-python-fastapi"
    test_command: "pytest" # Overrides test runner used by `foundation test`
    timeout_seconds: 45
    options:
      async_driver: true
    out_dir: ./dist/python-api
```

---

## 2. Target Configuration Reference

| Option | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `generator` | `string` | `""` | Name or path of a single external generator executable (e.g., `foundation-gen-python-fastapi`). |
| `generators` | `[]string` | `[]` | Sequential list of generator executables to run in order as a filter pipeline. |
| `scaffold` | `string` | `"builtin"` | `"builtin"` pre-populates Foundation's built-in target files in memory before plugins run; `"none"` starts with an empty file tree. |
| `options` | `map[string]any` | `{}` | Key-value options passed to the plugin via `GenerationRequest.target.config` and CLI flags (`--key=value`). |
| `test_command` | `string` | `""` | Command executed by `foundation test` (e.g., `pytest`, `cargo test`). If omitted, falls back to the plugin's response or built-in runner. |
| `timeout_seconds` | `int` | `60` | Maximum execution time in seconds for plugin execution and test commands. |

---

## 3. Plugin Discovery Rules

Foundation resolves generator binary names using a deterministic **3-tier lookup**:

1. **Direct Path**: If the name contains path separators (e.g., `./bin/my-gen` or `/opt/bin/my-gen`), Foundation invokes that exact binary.
2. **Workspace Local Toolchains**: Foundation checks local project directories:
   - `./node_modules/.bin/<name>`
   - `./bin/<name>`
3. **System Environment (`$PATH`)**: Foundation searches system `$PATH` for the executable.

If the executable cannot be resolved, the compiler aborts with error `FDN0701`.

---

## 4. The Standard Wire Protocol

External generators communicate with Foundation over standard I/O (`stdin` and `stdout`) using a strictly versioned JSON protocol. A formal JSON Schema is published at [`docs/schemas/generation-protocol-v1.json`](../schemas/generation-protocol-v1.json).

```
┌─────────────────────────────────┐
│        Foundation Compiler      │
│  (Manifest Engine & Lowerer)    │
└──────────────┬──────────────────┘
               │ stdin (GenerationRequest JSON)
               ▼
┌─────────────────────────────────┐
│     foundation-gen-<plugin>     │
│  (Custom Emitter / Filter CLI)  │
└──────────────┬──────────────────┘
               │ stdout (GenerationResponse JSON)
               ▼
┌─────────────────────────────────┐
│        Foundation Compiler      │
│  (Manifest Engine writes disk)  │
└─────────────────────────────────┘
```

### `GenerationRequest` (Input to Plugin)
Delivered to the plugin's standard input (`stdin`):

```json
{
  "protocol_version": "1.0",
  "target": {
    "name": "backend",
    "language": "go",
    "architecture": "hexagonal",
    "out_dir": "./dist/backend",
    "config": {
      "service_name": "order-service"
    }
  },
  "semantic_ir": {
    "entities": [ ... ],
    "actions": [ ... ],
    "values": [ ... ],
    "errors": [ ... ]
  },
  "architecture_ir": { ... },
  "files": {
    "internal/domain/order_entity.go": "package domain\n\ntype Order struct { ... }"
  }
}
```

* `protocol_version`: Current protocol version (`"1.0"`).
* `target`: Target parameters and custom options declared in `foundation.build.yaml`.
* `semantic_ir`: Validated domain IR (entities, actions, values, errors, policies, scenarios).
* `architecture_ir`: Optional pre-lowered middle-end IR (use cases, ports, routes, adapters).
* `files`: Current virtual file tree. When `scaffold: builtin`, this contains all built-in scaffolding; in multi-plugin pipelines, it contains modifications from upstream plugins.

### `GenerationResponse` (Output from Plugin)
Returned by the plugin on standard output (`stdout`):

```json
{
  "protocol_version": "1.0",
  "files": [
    {
      "path": "internal/domain/order_entity.go",
      "content": "package domain\n\n// Rich Aggregate Root with invariants\ntype Order struct { ... }"
    }
  ],
  "stubs": [
    {
      "path": "config/service.starter.json",
      "content": "{\n  \"sample_rate\": 1.0\n}\n"
    }
  ],
  "diagnostics": [],
  "test_command": "go test ./..."
}
```

* `files`: Fully tracked files managed by `.foundation-manifest.json`. Overwritten on rebuild and cleanly pruned when blueprints remove declarations.
* `stubs`: Create-if-missing starter files (e.g., Slot implementations, config starters). Written to disk once; **never overwritten or deleted** on subsequent builds.
* `diagnostics`: Non-fatal warnings or fatal compile errors surfaced directly in the Foundation compiler's build output.
* `test_command`: Optional default test command executed during `foundation test`.

---

## 5. Filesystem Safety & The Manifest Engine

Plugins **never** write directly to disk. The compiler's Manifest Engine retains exclusive ownership of the filesystem:
- **Hashing & Overwrite Protection**: Every tracked file is checksummed in `.foundation-manifest.json`.
- **Slot & Stub Exemption**: Files in `stubs` are omitted from the manifest, guaranteeing that developer edits are preserved indefinitely.
- **Cross-Plugin Mutation Warnings (`FDN0702`)**: If Plugin B modifies a file originally created or altered by Plugin A, Foundation emits a diagnostic warning:
  ```text
  warning [FDN0702]: generator 'foundation-gen-otel' modified file 'internal/transport/router.go' previously modified by 'foundation-gen-rich-ddd'
  ```
- **Process Group Isolation & Timeout (`FDN0700`)**: Plugins run in isolated process groups. If a plugin hangs or exceeds `timeout_seconds`, Foundation terminates the entire process group with `SIGKILL`.

---

## 6. Writing a Generator in Go using `pkg/emitterkit`

Foundation provides a dedicated SDK package, [`pkg/emitterkit`](../../pkg/emitterkit), allowing plugin authors to build robust generators in minutes.

### Example: A Rich DDD Aggregate Generator

```go
package main

import (
	"fmt"
	"strings"

	"github.com/galihanggara68/foundation/pkg/emitterkit"
)

func main() {
	emitterkit.RunPlugin(func(req *emitterkit.GenerationRequest) (*emitterkit.GenerationResponse, error) {
		// Extract existing files from the request (pre-populated by builtin scaffold)
		files := emitterkit.ExtractFiles(req)

		// Transform entities into rich Aggregate Roots
		for _, entity := range req.SemanticIR.Entities {
			fileName := fmt.Sprintf("internal/domain/%s_entity.go", strings.ToLower(entity.Name))

			var sb strings.Builder
			sb.WriteString("package domain\n\n")
			sb.WriteString(fmt.Sprintf("type %s struct {\n", entity.Name))
			for _, f := range entity.Fields {
				sb.WriteString(fmt.Sprintf("\t%s string\n", emitterkit.ExportIdent(f.Name)))
			}
			sb.WriteString("}\n\n")

			// Add constructor enforcing domain invariants
			sb.WriteString(fmt.Sprintf("func New%s(id string) *%s {\n", entity.Name, entity.Name))
			sb.WriteString(fmt.Sprintf("\treturn &%s{ID: id}\n", entity.Name))
			sb.WriteString("}\n")

			// Overwrite the entity file in the virtual file map
			files[fileName] = sb.String()
		}

		// Return updated files and optional create-if-missing starter stubs
		return &emitterkit.GenerationResponse{
			ProtocolVersion: "1.0",
			Files:           emitterkit.EmitFiles(files),
			Stubs: []emitterkit.OutputFile{
				{
					Path:    "config/ddd.starter.json",
					Content: "{\n  \"strict_invariants\": true\n}\n",
				},
			},
		}, nil
	})
}
```

### Building and Testing Your Generator
Compile your generator binary and place it in your `$PATH` or local `./bin`:

```bash
# Build the plugin binary
go build -o ./bin/foundation-gen-rich-ddd ./cmd/foundation-gen-rich-ddd

# Build the Foundation project using your plugin
foundation build

# Run automated tests
foundation test
```

---

## 7. Reference Generators

Inspect the reference implementations shipped in this repository:
* [`cmd/foundation-gen-rich-ddd/main.go`](../../cmd/foundation-gen-rich-ddd/main.go): Transforms domain models into rich DDD Aggregate Roots with validation and mutation methods.
* [`cmd/foundation-gen-otel/main.go`](../../cmd/foundation-gen-otel/main.go): Injects W3C distributed tracing middleware (`traceparent`, `X-Trace-ID`) and wraps HTTP server routing.
* [`cmd/foundation-gen-python-fastapi/main.go`](../../cmd/foundation-gen-python-fastapi/main.go): Clean-slate Python generator emitting runnable FastAPI services tested via `pytest`.
