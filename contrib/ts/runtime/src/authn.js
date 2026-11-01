import { createPublicKey, verify as cryptoVerify } from "node:crypto";

/**
 * Defaults mirror the Go resolver: an allow-list of the two asymmetric algorithms identity
 * providers publish key sets for. Nothing HMAC is ever accepted — a public key must never be
 * usable as an HMAC secret — and "none" is not representable at all.
 */
const DEFAULT_ALGORITHMS = ["RS256", "ES256"];
const DEFAULT_CACHE_TTL_MS = 5 * 60 * 1000;
const DEFAULT_REFRESH_THROTTLE_MS = 10 * 1000;
const MAX_JWKS_BYTES = 1 << 20;
const JWKS_FETCH_TIMEOUT_MS = 10_000;

/** Turns a JWKS document into public keys by key id, skipping entries it cannot represent. */
function parseJwks(body) {
  let doc;
  try {
    doc = JSON.parse(body);
  } catch {
    throw new Error("unauthenticated: key set is not valid JSON");
  }

  const keys = new Map();
  for (const entry of doc.keys ?? []) {
    if (!entry?.kid) continue;
    try {
      if (entry.kty === "RSA" && entry.n && entry.e) {
        keys.set(entry.kid, createPublicKey({ key: { kty: "RSA", n: entry.n, e: entry.e }, format: "jwk" }));
      } else if (entry.kty === "EC" && entry.crv === "P-256" && entry.x && entry.y) {
        keys.set(entry.kid, createPublicKey({ key: { kty: "EC", crv: entry.crv, x: entry.x, y: entry.y }, format: "jwk" }));
      }
    } catch {
      // An entry we cannot represent is skipped rather than failing the whole set.
    }
  }
  return keys;
}

/** Holds a key set fetched from one endpoint, refreshing it when a token names an unknown key. */
class JwksCache {
  constructor(url, ttlMs, throttleMs) {
    this.url = url;
    this.ttlMs = ttlMs;
    this.throttleMs = throttleMs;
    this.keys = new Map();
    this.fetchedAt = 0;
    this.attemptedAt = 0;
  }

  async key(kid) {
    const known = this.fresh(kid);
    if (known) return known;

    await this.refresh();

    const afterRefresh = this.fresh(kid);
    if (afterRefresh) return afterRefresh;
    throw new Error(`unauthenticated: key "${kid}" is not in the configured key set`);
  }

  fresh(kid) {
    if (!this.fetchedAt || Date.now() - this.fetchedAt > this.ttlMs) return undefined;
    return this.keys.get(kid);
  }

  async refresh() {
    if (this.throttleMs > 0 && this.attemptedAt && Date.now() - this.attemptedAt < this.throttleMs) {
      // We asked recently and it did not give us this key. Refusing rather than fetching again
      // keeps a flood of tokens naming unknown keys from turning every request into a round
      // trip to the identity provider.
      throw new Error("unauthenticated: key set was refreshed too recently");
    }
    this.attemptedAt = Date.now();

    const response = await fetch(this.url, { signal: AbortSignal.timeout(JWKS_FETCH_TIMEOUT_MS) });
    if (!response.ok) {
      throw new Error(`unauthenticated: key set endpoint answered ${response.status}`);
    }
    const body = await response.text();
    if (body.length > MAX_JWKS_BYTES) {
      throw new Error("unauthenticated: key set is too large");
    }

    this.keys = parseJwks(body);
    this.fetchedAt = Date.now();
  }
}

export class DevHeadersResolver {
  constructor() {
    this.logWarningBanner();
  }

  logWarningBanner() {
    const banner = `
================================================================================
[SECURITY WARNING] dev-headers Authn Adapter is active!
Plain HTTP headers (X-Actor-ID, etc.) are trusted without cryptographic verification.
DO NOT RUN IN PRODUCTION!
================================================================================`;
    console.warn(banner);
  }

  async resolveActor(headers) {
    const getHeader = (k) => {
      const v = headers[k.toLowerCase()] ?? headers[k];
      return Array.isArray(v) ? v[0] : v;
    };

    const actorId = getHeader("x-actor-id");
    if (!actorId) {
      throw new Error("unauthenticated: missing X-Actor-ID header");
    }

    const claims = {
      id: actorId,
      role: getHeader("x-actor-role") ?? "",
    };

    for (const [k, v] of Object.entries(headers)) {
      const lowerK = k.toLowerCase();
      if (lowerK.startsWith("x-actor-")) {
        let key = lowerK.replace("x-actor-", "");
        if (key.startsWith("claim-")) {
          key = key.replace("claim-", "");
        }
        key = key.replaceAll("-", "_");
        const raw = Array.isArray(v) ? v[0] : v;
        if (raw === "true") {
          claims[key] = true;
        } else if (raw === "false") {
          claims[key] = false;
        } else {
          claims[key] = raw;
        }
      }
    }
    if (typeof claims.verified === "boolean") {
      claims.is_verified = claims.verified;
    } else if (typeof claims.is_verified === "boolean") {
      claims.verified = claims.is_verified;
    }

    return claims;
  }
}

/**
 * Verifies a JWT's signature against a configured key set before trusting any claim. The key
 * set never comes from the token itself (no jku, x5u or embedded jwk), so a token cannot point
 * this verifier at an attacker's keys.
 *
 * options.jwksUrl falls back to AUTH_JWKS_URL, so an application generated before verification
 * existed gains it by setting that one variable rather than being regenerated — the promise
 * ADR 0006 makes for a security fix published in a contrib adapter.
 */
export class JWTOIDCResolver {
  constructor(issuer, audience, options = {}) {
    this.issuer = issuer;
    this.audience = audience;
    this.algorithms = new Set(options.algorithms ?? DEFAULT_ALGORITHMS);
    this.clockSkewSeconds = options.clockSkewSeconds ?? 0;

    const jwksUrl = options.jwksUrl ?? process.env.AUTH_JWKS_URL ?? "";
    if (!jwksUrl) {
      console.warn("[WARN] authn: jwt-oidc adapter has no AUTH_JWKS_URL configured; every request will be rejected");
      return;
    }
    this.jwks = new JwksCache(
      jwksUrl,
      (options.cacheTtlSeconds ?? DEFAULT_CACHE_TTL_MS / 1000) * 1000,
      options.refreshThrottleSeconds === undefined
        ? DEFAULT_REFRESH_THROTTLE_MS
        : options.refreshThrottleSeconds * 1000,
    );
  }

  async resolveActor(headers) {
    try {
      const auth = headers["authorization"] ?? headers["Authorization"];
      const authHeader = Array.isArray(auth) ? auth[0] : auth;
      if (!authHeader || !authHeader.startsWith("Bearer ")) {
        throw new Error("unauthenticated: missing or invalid authorization header");
      }

      const claims = await this.verifyToken(authHeader.substring(7));
      if (typeof claims.sub === "string" && claims.sub) {
        claims.id = claims.sub;
      }
      return claims;
    } catch (e) {
      const message = e instanceof Error ? e.message : String(e);
      // Anything that is not already an authentication failure — a fetch that never reached the
      // key set, say — is still a failure to authenticate, not a reason to trust the request.
      if (message.startsWith("unauthenticated:")) throw e;
      throw new Error(`unauthenticated: ${message}`);
    }
  }

  async verifyToken(token) {
    const parts = token.split(".");
    if (parts.length !== 3) {
      throw new Error("unauthenticated: malformed jwt token");
    }

    let header;
    try {
      header = JSON.parse(Buffer.from(parts[0], "base64url").toString("utf8"));
    } catch {
      throw new Error("unauthenticated: invalid jwt header");
    }
    const alg = header.alg ?? "";
    if (!this.algorithms.has(alg)) {
      throw new Error(`unauthenticated: unsupported jwt alg "${alg}"`);
    }
    if (!this.jwks) {
      throw new Error("unauthenticated: no key set configured");
    }

    const publicKey = await this.jwks.key(header.kid ?? "");
    const signingInput = Buffer.from(`${parts[0]}.${parts[1]}`);
    const signature = Buffer.from(parts[2], "base64url");
    const verified =
      alg === "RS256"
        ? cryptoVerify("sha256", signingInput, publicKey, signature)
        : cryptoVerify("sha256", signingInput, { key: publicKey, dsaEncoding: "ieee-p1363" }, signature);
    if (!verified) {
      throw new Error("unauthenticated: jwt signature does not verify");
    }

    let claims;
    try {
      claims = JSON.parse(Buffer.from(parts[1], "base64url").toString("utf8"));
    } catch {
      throw new Error("unauthenticated: invalid jwt json payload");
    }
    this.checkClaims(claims);
    return claims;
  }

  /** Enforces exp and nbf, requires a subject, and validates iss and aud when configured. */
  checkClaims(claims) {
    const now = Math.floor(Date.now() / 1000);

    const exp = typeof claims.exp === "number" ? claims.exp : undefined;
    if (exp === undefined) {
      throw new Error("unauthenticated: jwt has no exp claim");
    }
    if (now - this.clockSkewSeconds > exp) {
      throw new Error("unauthenticated: jwt is expired");
    }
    if (typeof claims.nbf === "number" && now + this.clockSkewSeconds < claims.nbf) {
      throw new Error("unauthenticated: jwt is not valid yet");
    }
    if (typeof claims.sub !== "string" && typeof claims.id !== "string") {
      throw new Error("unauthenticated: jwt missing sub claim");
    }
    if (this.issuer && claims.iss !== this.issuer) {
      throw new Error("unauthenticated: invalid jwt issuer");
    }
    if (this.audience) {
      const aud = claims.aud;
      const valid = Array.isArray(aud) ? aud.includes(this.audience) : aud === this.audience;
      if (!valid) {
        throw new Error("unauthenticated: invalid jwt audience");
      }
    }
  }
}
