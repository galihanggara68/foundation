# Actor Resolver Port with Contrib Authn Adapters

## Context

Generated backends require secure actor authentication before evaluating policies.
Inlined authentication code prevents security patching through standard package managers.
The compiler needed a modular way to update authentication adapters without project regeneration.

## Decision

Authentication uses an **Actor Resolver** port in every generated backend.
The resolver validates incoming requests into a verified `Actor` before any policy runs.
The `authn` setting in `foundation.build.yaml` selects the active resolver adapter.
Built-in adapters (including JWT/OIDC) ship as published contrib runtime libraries.
An unconfigured profile keeps the `dev-headers` adapter for local development.

## Consequences

- Security patches propagate through dependency updates without compiler releases.
- The blueprint grammar includes an `unauthenticated` error kind (HTTP 401).
- The `dev-headers` adapter remains restricted to local development.
- The server prints a startup banner and a warning log line when running in development mode.
