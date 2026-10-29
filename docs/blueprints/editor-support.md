# Editor Support & Language Server

Foundation ships an **in-tree language server** so `.fdn` Blueprints are first-class in any editor that speaks the [Language Server Protocol](https://microsoft.github.io/language-server-protocol/). It runs offline, uses the same compiler as `foundation check`, and needs no separate build or daemon.

---

## Editor Integrations

Official editor integrations live in sibling repositories and are under active development:

| Project | Editor / Use | What it adds |
| :--- | :--- | :--- |
| [`tree-sitter-foundation`](https://github.com/galihanggara68/tree-sitter-foundation) | Tree-sitter grammar for **Zed**, **Neovim**, and **Helix** | The `.fdn` grammar plus highlights, indents, brackets, and code-folding queries. |
| [`foundation-zed`](https://github.com/galihanggara68/foundation-zed) | [Zed](https://zed.dev) | Tree-sitter highlighting, block folding and indentation, and auto-discovery of the `foundation` CLI to launch `foundation lsp`. |
| [`foundation-vscode`](https://github.com/galihanggara68/foundation-vscode) | VS Code / Open VSX | TextMate syntax highlighting, `foundation lsp` diagnostics and hover, and `entity` / `action` / `scenario` snippets. |

Every client launches the same `foundation lsp` server; the integrations differ only in how they locate the binary and render syntax. See each repository's README for installation and configuration.

---

## Launching the Server

The server is a subcommand of the CLI and communicates over **stdio** using LSP 3.17:

```bash
foundation lsp [--dir <path>] [--debug]
```

| Flag | Description |
| :--- | :--- |
| `--dir <path>` | Workspace directory to resolve relative to (defaults to `.`). |
| `--debug` | Reserved for verbose diagnostic tracing. |

An editor client launches `foundation` with the argument `lsp` as a stdio server and then exchanges JSON-RPC frames with it. The workspace roots come from the client's `initialize` request (`rootUri` and any `workspaceFolders`), so the server indexes whatever folders the editor has open.

The integrations discover `foundation` on your `$PATH` by default and let you point at a specific binary when it lives elsewhere (for example, a project-local `bin/foundation`). When a client sets an explicit binary path, it must also pass the `lsp` argument — the server is a subcommand, not the default entry point.

---

## Capabilities

| Capability | Supported | Notes |
| :--- | :---: | :--- |
| Live diagnostics | ✅ | Compiler error/warning/information diagnostics published on open, change, and save. |
| Hover | ✅ | DSL catalog documentation and workspace-declared symbols with inferred types. |
| Completion | ✅ | Keywords, scalar types, traits, and struct members, with snippet tab stops. |
| Go-to-definition | ✅ | Resolves declarations across files. |
| Formatting / Rename / Code actions | ❌ | Not implemented; unknown requests are answered with `method not found`. |

### Diagnostics

Every open, edit, and save re-compiles the workspace and publishes diagnostics for each affected `.fdn` document, so errors from one file surface even while you are editing another. Diagnostics map to the same codes, severities, and spans that `foundation check` reports on the command line.

### Hover

Hover resolves in two layers:

1. **DSL catalog** — documentation for constructs (`entity`, `action`, `slot`, `policy`, …), pipeline steps (`query`, `validate`, `require`, `emit`), scalar types (`uuid`, `decimal`, `timestamp`, …), scenario clauses (`given`, `when`, `expect`), and traits (`@version`, `@table`, `@lock`, …).
2. **Workspace symbols** — declarations found in the indexed Blueprints, rendered with their kind, source line, and inferred type.

Local bindings are type-inferred: `let` and `for` variables report entity types (including `[]T` for `query many`), slot method return types, action return types, and struct field types reached through expressions, with types propagated to later bindings in the same scope.

### Completion

Completions are context-aware, with trigger characters `@`, `:`, and `.`:

- After `@`, trait completions.
- In a type position (after `:`), scalar type completions.
- After `value.`, **member completion** — the fields of the receiver's struct type. When a declaration is mid-edit the parser can discard it, so the server falls back to scanning the visible source text for the receiver's type.
- Otherwise, the keyword catalog plus scalar types and traits.

Items that carry an insertion template declare `insertTextFormat: Snippet`, so clients expand `${1:placeholder}` tab stops rather than inserting the literal text.

### Go-to-Definition

Definition resolves entities, values, enums, events, errors, policies, slots, and their fields/parameters/methods, actions separated from scenarios, and `let`/`for` locals — across every indexed file, not just the current one.

---

## How the Workspace is Indexed

- The server walks each workspace root for `*.fdn` sources, skipping hidden directories and dependency/build directories (`node_modules`, `vendor`, `dist`, `build`, `out`, `target`, `tmp`, `coverage`). On large repositories this keeps a scan from walking the installed dependency tree.
- Gathered sources and per-document symbol indexes are **memoized**. Repeated hover, definition, and member-completion requests reuse the cache, and an edit invalidates only the changed document's index.

---

## Related Documentation

- [Declarations & Grammar](./declarations.md): the constructs and steps the server offers documentation and completions for.
- [Build Profile & Configuration](./build-profile.md): the CLI surface, including `foundation check` whose diagnostics the server mirrors.
