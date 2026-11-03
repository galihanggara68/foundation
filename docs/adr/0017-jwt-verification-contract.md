# JWT Verification Contract for the `jwt-oidc` Adapter

## Context

The initial `jwt-oidc` adapter decoded payloads without cryptographic signature checks.
That implementation accepted tokens with `alg: "none"`, which bypassed authentication.
Generated backends require cryptographic verification before resolving actor identities.

## Decision

1. **Verify Before Read**: The adapter verifies the signature before reading token payload claims.
2. **Strict Algorithm Allow-List**: The adapter accepts only `RS256` and `ES256`. The adapter rejects HMAC algorithms.
3. **External JWKS Configuration**: The adapter reads key sets from `AUTH_JWKS_URL`. The adapter ignores token header URLs to prevent SSRF attacks.
4. **Fail Closed**: Missing keys, unknown key identifiers, invalid claims, and signature mismatches return HTTP 401.
5. **Standard Library Implementation**: Runtimes use standard cryptographic libraries without third-party dependencies.

## Consequences

- Deployments without `AUTH_JWKS_URL` reject all requests with HTTP 401.
- Rate limits on key refreshes prevent denial of service attacks against identity providers.
- Runtimes verify real tokens securely across Go and TypeScript backends.
