export interface AuthnResolver {
  resolveActor(headers: Record<string, string | string[] | undefined>): Promise<Record<string, unknown>>;
}

export class DevHeadersResolver implements AuthnResolver {
  constructor() {
    this.logWarningBanner();
  }

  private logWarningBanner(): void {
    const banner = `
================================================================================
[SECURITY WARNING] dev-headers Authn Adapter is active!
Plain HTTP headers (X-Actor-ID, etc.) are trusted without cryptographic verification.
DO NOT RUN IN PRODUCTION!
================================================================================`;
    console.warn(banner);
  }

  async resolveActor(headers: Record<string, string | string[] | undefined>): Promise<Record<string, unknown>> {
    const getHeader = (k: string): string | undefined => {
      const v = headers[k.toLowerCase()] ?? headers[k];
      return Array.isArray(v) ? v[0] : v;
    };

    const actorId = getHeader("x-actor-id");
    if (!actorId) {
      throw new Error("unauthenticated: missing X-Actor-ID header");
    }

    const claims: Record<string, unknown> = {
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

export class JWTOIDCResolver implements AuthnResolver {
  readonly issuer: string;
  readonly audience: string;

  constructor(issuer: string, audience: string) {
    this.issuer = issuer;
    this.audience = audience;
  }

  async resolveActor(headers: Record<string, string | string[] | undefined>): Promise<Record<string, unknown>> {
    const auth = headers["authorization"] ?? headers["Authorization"];
    const authHeader = Array.isArray(auth) ? auth[0] : auth;
    if (!authHeader || !authHeader.startsWith("Bearer ")) {
      throw new Error("unauthenticated: missing or invalid authorization header");
    }

    const token = authHeader.substring(7);
    const parts = token.split(".");
    if (parts.length !== 3) {
      throw new Error("unauthenticated: malformed jwt token");
    }

    try {
      const payloadStr = Buffer.from(parts[1], "base64url").toString("utf8");
      const claims = JSON.parse(payloadStr) as Record<string, unknown>;
      if (claims.sub) {
        claims.id = claims.sub;
      }
      if (!claims.id) {
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
      return claims;
    } catch (e: any) {
      throw new Error(`unauthenticated: ${e.message}`);
    }
  }
}
