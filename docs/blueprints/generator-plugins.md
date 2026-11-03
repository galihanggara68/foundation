# Generator Plugins and Custom Emitters

Foundation provides a subprocess plugin architecture based on the `protoc` model ([ADR 0011](../adr/0011-pluggable-emitter-protocol.md) and [ADR 0014](../adr/0014-multi-generator-plugin-pipelines.md)).

With Generator Plugins, you can:
1. **Target Any Language or Framework**: Generate backends for other languages (for example Python FastAPI, Rust Axum, or C#) with `scaffold: none`.
2. **Compose Specialized Pipelines**: Run multiple plugins in sequence (`generators: [...]`) while reusing built-in scaffolding (`scaffold: builtin`).
3. **Run Native Test Suites**: Run integration tests directly with native test runners (`pytest`, `cargo test`, `go test`) using `foundation test`.

---

## 1. Configuring Plugins in `foundation.build.yaml`

To use an external generator, declare `generator:` (single plugin) or `generators:` (sequential pipeline) under a target in `foundation.build.yaml`:

### Pattern A: Chained Pipeline with Scaffolding Pre-population

This pattern pre-populates built-in scaffolding (ports, repositories, HTTP routing, database migrations) and passes virtual files through sequential plugins:

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

### Pattern B: Clean Slate Generation (New Languages and Frameworks)

This pattern starts with an empty file tree (`scaffold: none`) so a plugin generates a custom stack:

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
| `generator` | `string` | `""` | Name or path of a single external generator executable (for example `foundation-gen-python-fastapi`). |
| `generators` | `[]string` | `[]` | Sequential list of generator executables to run in order as a filter pipeline. |
| `scaffold` | `string` | `"builtin"` | `"builtin"` pre-populates target files in memory before plugins run; `"none"` starts with an empty file tree. |
| `options` | `map[string]any` | `{}` | Key-value options passed to the plugin through `GenerationRequest.target.config` and CLI flags (`--key=value`). |
| `test_command` | `string` | `""` | Command executed by `foundation test` (for example `pytest` or `cargo test`). |
| `timeout_seconds` | `int` | `60` | Maximum execution time in seconds for plugin execution and test commands. |

---

## 3. Plugin Discovery Rules

Foundation resolves generator binary names using a three-tier lookup:

1. **Direct Path**: If the name contains path separators (for example `./bin/my-gen` or `/opt/bin/my-gen`), Foundation runs that binary.
2. **Workspace Local Toolchains**: Foundation checks local project directories:
   - `./node_modules/.bin/<name>`
   - `./bin/<name>`
3. **System Environment (`$PATH`)**: Foundation searches system `$PATH` for the executable.

If Foundation cannot find the executable, the compiler stops with error `FDN0701`.

---

## 4. The Standard Wire Protocol

External generators communicate with Foundation over standard I/O (`stdin` and `stdout`) with JSON messages.
See the formal JSON Schema at [`docs/schemas/generation-protocol-v1.json`](../schemas/generation-protocol-v1.json).

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

Foundation delivers `GenerationRequest` to plugin standard input (`stdin`):

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

- `protocol_version`: Current protocol version (`"1.0"`).
- `target`: Target parameters and custom options declared in `foundation.build.yaml`.
- `semantic_ir`: Validated domain IR (entities, actions, values, errors, policies, scenarios). Declared errors carry closed kinds, resolved HTTP statuses, postures, and per-locale messages ([ADR 0016](../adr/0016-error-vocabulary-ownership.md)).
- `architecture_ir`: Optional pre-lowered middle-end IR (use cases, ports, routes, adapters).
- `files`: Current virtual file tree. When `scaffold: builtin`, this contains built-in scaffolding. In multi-plugin pipelines, it contains modifications from upstream plugins.

### `GenerationResponse` (Output from Plugin)

The plugin returns `GenerationResponse` on standard output (`stdout`):

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

- `files`: Fully tracked files managed by `.foundation-manifest.json`. Builds overwrite these files and prune obsolete files.
- `stubs`: Create-if-missing starter files (for example slot implementations or config starters). The compiler writes them once and never overwrites them.
- `diagnostics`: Warnings or compile errors displayed in compiler output.
- `test_command`: Optional default test command executed during `foundation test`.

---

## 5. Filesystem Safety and Manifest Engine

Plugins do not write directly to disk.
The compiler manifest engine manages the filesystem:
- **Checksums**: The manifest (`.foundation-manifest.json`) records checksums for all tracked files.
- **Stub Protection**: The manifest omits stub files. Builds preserve developer edits permanently.
- **Cross-Plugin Warnings (`FDN0702`)**: Foundation emits a warning if a plugin modifies a file from an earlier plugin.
- **Process Isolation and Timeouts (`FDN0700`)**: Plugins run in isolated process groups. If a plugin exceeds `timeout_seconds`, Foundation stops the process group with `SIGKILL`.

---

## 6. Writing a Generator in Go Using `pkg/emitterkit`

Foundation provides an SDK package, [`pkg/emitterkit`](../../pkg/emitterkit), to build custom generator plugins.

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
- [`cmd/foundation-gen-rich-ddd/main.go`](../../cmd/foundation-gen-rich-ddd/main.go): Transforms domain models into rich DDD Aggregate Roots with validation and mutation methods.
- [`cmd/foundation-gen-otel/main.go`](../../cmd/foundation-gen-otel/main.go): Injects W3C distributed tracing middleware (`traceparent`, `X-Trace-ID`) and wraps HTTP server routing.
- [`cmd/foundation-gen-python-fastapi/main.go`](../../cmd/foundation-gen-python-fastapi/main.go): Clean-slate Python generator emitting runnable FastAPI services tested with `pytest`.
