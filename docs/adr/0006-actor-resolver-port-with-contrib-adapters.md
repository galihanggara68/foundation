# Actor Resolver Port with Contrib Authn Adapters

Authentication becomes an **Actor Resolver** — a built-in port present in every generated backend that resolves an incoming request into a verified Actor before any Policy evaluates, selected through the Build Profile's `authn` setting. Built-in adapters (starting with JWT/OIDC) ship as separately published contrib libraries that generated code imports, rather than files the compiler emits inline or hands to the developer: a security patch in an adapter must propagate through a dependency update without a compiler release or project regeneration, so the compiler never owns security-sensitive code it cannot patch in place. An unconfigured profile keeps the trusted-headers dev adapter for backward compatibility (preserving a frictionless quick-start during introduction), but the emitted server prints a boot banner and a structured warning log line on every start; deny-all secure-by-default is to be revisited before any 1.0 release.

## Consequences

- Publishing and SemVer discipline for contrib modules is a prerequisite of the first authn adapter release, not a follow-up; a CVE in a contrib adapter is patched in the module, never by regeneration.
- The Blueprint grammar needs a built-in `unauthenticated` error kind (HTTP 401) — today only `forbidden` (403) exists, so an authentication failure is currently inexpressible in a Blueprint.
- The trusted-headers adapter is documented as dev-only; no Foundation doc may bless it for production use.
