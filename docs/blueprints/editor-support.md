# Editor Support and Language Server

Foundation includes an **in-tree language server**.
This server supports editors that implement the [Language Server Protocol](https://microsoft.github.io/language-server-protocol/).
The server runs offline, uses the compiler core, and requires no external daemon.

---

## Editor Integrations

Sibling repositories provide official editor integrations:

| Project | Editor or Tool | Provided Features |
| :--- | :--- | :--- |
| [`tree-sitter-foundation`](https://github.com/galihanggara68/tree-sitter-foundation) | Tree-sitter grammar for **Zed**, **Neovim**, and **Helix** | Blueprint grammar with syntax highlights, indentation, brackets, and code folding. |
| [`foundation-zed`](https://github.com/galihanggara68/foundation-zed) | [Zed](https://zed.dev) | Tree-sitter highlighting, block folding, indentation, and automatic discovery of `foundation lsp`. |
| [`foundation-vscode`](https://github.com/galihanggara68/foundation-vscode) | VS Code and Open VSX | Syntax highlighting, compiler diagnostics, hover help, and declaration snippets. |

Every client launches the same `foundation lsp` server.
The integrations differ only in how they discover the binary and render syntax.
Read the README file in each repository for installation instructions.

---

## Launching the Server

The language server is a CLI subcommand and communicates over **standard I/O** using LSP 3.17:

```bash
foundation lsp [--dir <path>] [--debug]
```

| Flag | Description |
| :--- | :--- |
| `--dir <path>` | Workspace directory to resolve relative to (defaults to `.`). |
| `--debug` | Enables verbose diagnostic tracing. |

An editor client launches `foundation lsp` as a background server process and exchanges JSON-RPC messages.
The server receives workspace roots from the client `initialize` request (`rootUri` and `workspaceFolders`).
The server then indexes files in all open folders.

Integrations discover `foundation` on your `$PATH` by default.
You can configure a direct path to a project binary (for example `bin/foundation`).
When you configure an explicit path, you must include the `lsp` argument.

---

## Capabilities

| Capability | Supported | Notes |
| :--- | :---: | :--- |
| Live diagnostics | Yes | The server publishes diagnostics when you open, edit, and save files. |
| Hover | Yes | Shows DSL catalog documentation and symbol types. |
| Completion | Yes | Suggests keywords, scalar types, traits, and struct members with snippet tab stops. |
| Go-to-definition | Yes | Resolves declarations across all workspace files. |
| Formatting / Rename / Code actions | No | The server does not implement these capabilities. Unknown requests return `method not found`. |

### Diagnostics

When you open, edit, or save a document, the server recompiles the workspace.
The server publishes diagnostics for each affected `.fdn` document.
Diagnostics show the same codes, severities, and source spans as `foundation check`.

### Hover

Hover resolves documentation in two layers:

1. **DSL catalog**: Explains constructs (`entity`, `action`, `slot`, `policy`), steps (`query`, `guard`, `require`, `emit`), scalar types (`uuid`, `decimal`, `timestamp`), scenario clauses (`given`, `when`, `expect`), and traits (`@version`, `@table`, `@index`).
2. **Workspace symbols**: Displays declarations found in indexed blueprints with kind, source line, and inferred type.

The server infers types for local variables.
Variables in `let` and `for` blocks report entity types, slot return types, and action return types.
The server propagates types to later bindings in the same scope.

### Completion

Completions are context-aware and use trigger characters `@`, `:`, and `.`:

- After `@`, the server suggests trait completions.
- After `:`, the server suggests scalar type completions.
- After `.`, the server suggests member completions for receiver struct fields. When editing incomplete declarations, the server falls back to scanning visible source text.
- In general positions, the server suggests keywords, scalar types, and traits.

Items with insertion templates declare `insertTextFormat: Snippet`.
Editor clients expand `${1:placeholder}` tab stops instead of inserting literal text.

### Go-to-Definition

Go-to-definition resolves entities, values, enums, events, errors, policies, slots, actions, and local variables across all indexed files.

---

## How the Server Indexes the Workspace

- The server searches workspace root directories for `*.fdn` files. The search ignores hidden directories and dependency folders (`node_modules`, `vendor`, `dist`, `build`, `out`, `target`, `tmp`, `coverage`).
- The server caches parsed sources and document symbol indexes. Repeated requests reuse cached indexes. Editing a file invalidates only that file's index.

---

## Related Documentation

- [Declarations and Grammar](./declarations.md): Reference for constructs and steps documented by hover and completion.
- [Build Profile and Configuration](./build-profile.md): Reference for the CLI and `foundation check`.
