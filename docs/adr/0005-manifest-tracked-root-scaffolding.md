# Manifest-Tracked Root Scaffolding over Isolated `gen/` Subtrees

## Context

Confining generated code to an isolated `gen/` folder breaks standard project folder layouts.
Isolated folders prevent natural integration with language idioms (such as Hexagonal Architecture in Go).
The compiler required a method to track generated files without corrupting handwritten code.

## Decision

Foundation emits code directly into idiomatic root directories (for example `cmd/` and `internal/`).
The compiler tracks all compiler-owned files in a **Generation Manifest** (`.foundation-manifest.json`).
The manifest records file paths and content checksums for every build.

## Consequences

- Generated code follows standard project layouts for Go and TypeScript.
- The compiler deletes obsolete generated files safely when blueprints change.
- The compiler preserves handwritten slot implementations and custom test files.
