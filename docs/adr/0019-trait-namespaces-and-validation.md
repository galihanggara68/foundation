# Trait Namespaces and Trait Validation

Traits attach declarative metadata to entities, fields, actions, and slot methods. Previously, traits had no formal registry, syntax for external tools, or argument validation. Any unknown trait was passed along silently without verification, and typoed traits went unnoticed. We establish a single source of truth for standard traits, introduce syntax for namespaced traits (`@ns:name`), and define clear validation and diagnostic boundaries.

## Context

As the compiler evolved, different subsystems read traits (`@table`, `@version`, `@http`, `@event_handler`) ad-hoc. Downstream code generators and third-party plugins also need domain metadata annotations without risking collisions with current or future compiler traits. Furthermore, misspelled traits or malformed arguments (such as non-numeric TTLs or missing required event parameters) were previously ignored silently, causing runtime misbehavior.

## Decision

1. **Single Source of Truth**: The compiler maintains a canonical trait registry in `pkg/compiler/internal/platform/traits.go` defining standard traits, their allowed hosts, and expected argument contracts.
2. **Trait Namespace Syntax (`@ns:name`)**: Traits support an optional namespace qualifier separated by a colon. Namespaced traits designate metadata owned by external tools or downstream generator plugins.
3. **The Ignore Guarantee**: Any namespaced trait outside the reserved namespace is passed through untouched on `ir.TraitDecl.Name` as `"ns:local"` and is never judged, validated, or rejected by the compiler.
4. **Reserved `foundation` Namespace (`FDN0225`)**: The `foundation` namespace is strictly reserved for compiler-owned features. Declaring `@foundation:...` is rejected by the parser as an error (`FDN0225`); standard compiler traits must always be written un-namespaced (e.g. `@table`).
5. **Validation Diagnostics as Warnings (`FDN0224`, `FDN0226`)**:
   - An unrecognized un-namespaced trait is reported as a warning (`FDN0224`). Treating it as a warning preserves forward compatibility while catching typos.
   - An argument problem on a standard trait (unknown argument, missing required argument, or non-numeric value for numeric arguments) is reported as a warning (`FDN0226`) rather than an error, avoiding redundant fatal diagnostics when semantic passes (such as `FDN0203` for missing events) already fail.
6. **LSP Registry Synchronization**: The Language Server hover and completion catalogues are populated directly from the platform registry, with a drift test guaranteeing that every standard trait is documented.

## Considered Options

- **Reject unknown un-namespaced traits as fatal errors**: Rigid and breaks forward compatibility when blueprints are shared across different compiler versions. Emitting `FDN0224` warning alerts authors to typos while permitting graceful degradation.
- **Require registration for all external traits**: Overly restrictive for plugins and generators. The namespace prefix provides natural isolation without central registration.
- **Allow `@foundation:` for standard traits**: Introduces duplicate spellings (`@table` vs `@foundation:table`). Forcing un-namespaced standard traits keeps blueprints concise and reserves the namespace cleanly.

## Consequences

- Blueprints can cleanly embed tool-specific annotations via `@ns:name` with guaranteed compiler pass-through.
- Misspelled standard traits and malformed arguments emit actionable warnings (`FDN0224`, `FDN0226`).
- The Language Server catalogue automatically reflects all standard traits without drift.
