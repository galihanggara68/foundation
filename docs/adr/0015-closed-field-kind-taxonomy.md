# Closed Field-Kind Taxonomy in the Semantic IR

## Context

The Semantic IR previously stored field types as free-form strings (`FieldDecl.Type`).
Each emitter maintained custom string switches to classify scalars, enums, and references.
Unrecognized type strings caused silent code generation errors in downstream emitters.

## Decision

The compiler validates and assigns a closed **Field Kind** to every declared field.
The vocabulary covers six scalars, `enum`, reference kinds (`entity`, `value`, `event`), and built-in kinds (`actor`, `page`, `result`).
The compiler rejects unclassifiable field types with error `FDN0218`.

## Consequences

- The compiler acts as the single authority for field classifications.
- Emitters read both the element `Kind` and the original declared type string.
- New declarable types require compiler modifications rather than emitter workarounds.
- Field type errors emit specific diagnostic `FDN0218`.
