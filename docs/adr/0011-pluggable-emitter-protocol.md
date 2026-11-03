# Standardized IR Protocol and External Generator Plugins

## Context

Text templating engines cannot compile statement-level action pipelines with dynamic scoping and type rules.
String interpolation produces brittle macros and causes syntax errors in target languages.
The compiler needed a protocol to support external generator plugins without sacrificing filesystem safety.

## Decision

Foundation adopts a standardized JSON protocol for generator plugins (`foundation-gen-*`) based on the `protoc` model.
The compiler communicates with external generator executables over standard I/O using versioned JSON payloads.
The compiler passes `GenerationRequest` with `SemanticIR` and optional `ArchitectureIR` to the plugin.
The plugin returns `GenerationResponse` with generated file paths and contents.
The compiler Manifest Engine retains exclusive control over file writing, checksums, and stale file cleanup.

## Consequences

- Third-party language targets connect to the compiler through a published JSON schema.
- Official language targets remain built into the compiler binary.
- Plugins never write directly to the disk, which protects handwritten slot files.
- Authors can use `pkg/emitterkit` to build custom generator plugins in Go.
