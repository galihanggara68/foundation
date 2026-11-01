# JWT Verification Contract for the `jwt-oidc` Adapter

The contrib `jwt-oidc` adapter decoded a token's payload and trusted it, never reading the header or the signature, so a token with `alg: "none"` and a literal `"sig"` suffix authenticated — a bypass a security test recorded as current state rather than failing on. Verification is now a contract: signatures are checked against a key set configured out of band, only `RS256`/`ES256` are accepted, and any doubt fails closed with 401.

## Context

Generated backends resolve an `Actor` at the boundary and every policy builds on it, so a resolver trusting unverified claims does not weaken authorization — it removes it. The adapter decoded claims because its first deployments sat behind an ingress that verified tokens; as the boundary itself, that assumption is a bypass. Spec 02 requires cryptographic verification through JWKS, and Phase 3's exit criterion is that generated backends verify real tokens.

## Decision

1. **Verify before reading.** The signature over `base64url(header) + "." + base64url(payload)` is checked against the key named by `kid`; claims are read only afterwards. `exp` is required, `nbf` is honoured (both with skew), `iss`/`aud` are validated when configured, and requiring `iss` outright is a follow-up.
2. **An allow-list of two algorithms.** Only `RS256`/`ES256`: `alg: none` is unrepresentable, HMAC is refused (a public key must not double as an HMAC secret), and the key type must match the algorithm.
3. **Configuration is out of band.** The key set comes from `AUTH_JWKS_URL`, never from the token's `jku`, `x5u` or embedded `jwk` — no SSRF, and a token cannot choose its verifier.
4. **Fail closed.** No key set, an unreachable endpoint, an unknown `kid`, an unsupported `alg`, a bad signature or a bad claim all produce 401.
5. **Standard library only, in both runtimes**, so the zero-dependency claim holds.

## Considered Options

- **Leave verification to the ingress.** The status quo: insecure by default.
- **Delegate to a JWT library.** Less crypto to own, but it adds dependencies to modules whose zero-dependency promise is public.
- **Stdlib verification with an allow-list (chosen).** No new dependencies, one behaviour across Go and TypeScript.

## Consequences

- A `jwt-oidc` deployment with no `AUTH_JWKS_URL` now answers 401 instead of trusting tokens. That break is intended.
- Key-set refreshes are throttled (one per 10s) so unknown `kid`s cannot turn every request into a round trip to the identity provider; a rotation is still picked up immediately.
- Generated applications receive the fix through the runtime module, so the emitted pin moves with the adapter's tag; until then, only builds resolving the contrib source are protected.
- The TypeScript runtime's `.ts`/`.js` pair has no build step, so verification lives in two copies.
