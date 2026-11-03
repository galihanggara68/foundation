# Trait Namespaces and Trait Validation

## Context

Subsystems previously read traits without a central registry or argument validation.
External plugins and generators needed custom annotations without risking naming collisions.
Misspelled traits and invalid arguments passed silently and caused runtime errors.

## Decision

1. **Central Trait Registry**: The platform package defines standard traits, allowed host declarations, and argument contracts.
2. **Namespace Syntax (`@ns:name`)**: Traits support an optional namespace prefix separated by a colon.
3. **The Ignore Guarantee**: The compiler passes non-reserved namespaced traits through to IR without validation.
4. **Reserved Namespace**: The compiler reserves the `foundation` namespace and rejects `@foundation:...` with error `FDN0225`.
5. **Diagnostic Warnings**:
   - Unknown un-namespaced traits emit warning `FDN0224`.
   - Invalid trait arguments emit warning `FDN0226`.
6. **LSP Catalogue Sync**: Language server hover and completion catalogues populate directly from the platform registry.

## Consequences

- Blueprints embed plugin-specific metadata safely using `@ns:name`.
- Authors receive immediate warnings for misspelled traits and invalid arguments.
- Language server documentation stays synchronized with the compiler trait registry.
