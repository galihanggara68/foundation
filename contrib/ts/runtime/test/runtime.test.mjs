import test from "node:test";
import assert from "node:assert";
import { createHash, generateKeyPairSync, sign as cryptoSign } from "node:crypto";
import { createServer } from "node:http";
import { MemoryIdempotencyStore, MemoryCacheStore, MemoryOutboxStore, PostgresIdempotencyStore, PostgresOutboxStore, createPostgresDriver } from "../src/index.js";

// createSigningKey mints real RS256 signatures and the JWKS that publishes the matching public
// half, so the resolver is exercised against a signature rather than a token shaped like one.
function createSigningKey() {
  const { privateKey, publicKey } = generateKeyPairSync("rsa", { modulusLength: 2048 });
  const jwk = publicKey.export({ format: "jwk" });
  const kid = "test-" + createHash("sha256").update(JSON.stringify(jwk)).digest("base64url").slice(0, 8);

  return {
    kid,
    jwk: { ...jwk, kid, alg: "RS256", use: "sig" },
    token(claims, header = { alg: "RS256", typ: "JWT", kid }) {
      const encodedHeader = Buffer.from(JSON.stringify(header)).toString("base64url");
      const encodedPayload = Buffer.from(JSON.stringify(claims)).toString("base64url");
      const signingInput = `${encodedHeader}.${encodedPayload}`;
      const signature = cryptoSign("sha256", Buffer.from(signingInput), privateKey).toString("base64url");
      return `${signingInput}.${signature}`;
    },
    // Rewrites the payload while keeping the signature: what an attacker sends when nothing checks.
    tamper(token, claims) {
      const [encodedHeader, , signature] = token.split(".");
      const encodedPayload = Buffer.from(JSON.stringify(claims)).toString("base64url");
      return `${encodedHeader}.${encodedPayload}.${signature}`;
    },
  };
}

function serveJWKS(keys) {
  const body = JSON.stringify({ keys: keys.map((k) => k.jwk) });
  const server = createServer((_req, res) => {
    res.setHeader("Content-Type", "application/json");
    res.end(body);
  });
  return new Promise((resolve) => {
    server.listen(0, "127.0.0.1", () => {
      resolve({
        url: `http://127.0.0.1:${server.address().port}`,
        close: () => new Promise((done) => server.close(done)),
      });
    });
  });
}

function claimsFor(iss, aud, overrides = {}) {
  const now = Math.floor(Date.now() / 1000);
  return { sub: "jwt-user-1", role: "user", iss, aud, exp: now + 3600, iat: now - 60, ...overrides };
}

test("MemoryIdempotencyStore lifecycle", async () => {
  const store = new MemoryIdempotencyStore();
  const key = "k1";

  const { status, unlock } = await store.getOrLock(key, 60000);
  assert.strictEqual(status, "started");
  assert.strictEqual(typeof unlock, "function");

  await assert.rejects(
    async () => store.getOrLock(key, 60000),
    /locked by an in-flight operation/
  );

  const payload = new Uint8Array([1, 2, 3]);
  await unlock(true, payload);

  const res2 = await store.getOrLock(key, 60000);
  assert.strictEqual(res2.status, "completed");
  assert.deepStrictEqual(res2.payload, payload);
});

test("MemoryCacheStore lifecycle", async () => {
  const store = new MemoryCacheStore();
  const key = "cache:1";
  const val = new Uint8Array([42]);

  const miss = await store.get(key);
  assert.strictEqual(miss.found, false);

  await store.set(key, val, 50);
  const hit = await store.get(key);
  assert.strictEqual(hit.found, true);
  assert.deepStrictEqual(hit.value, val);

  await new Promise((r) => setTimeout(r, 60));
  const expired = await store.get(key);
  assert.strictEqual(expired.found, false);

  await store.set(key, val, 60000);
  await store.delete(key);
  const deleted = await store.get(key);
  assert.strictEqual(deleted.found, false);
});

test("MemoryOutboxStore stage and relay", async () => {
  const store = new MemoryOutboxStore();
  const ev = {
    id: "evt-1",
    topic: "users",
    eventName: "UserCreated",
    payload: new Uint8Array([99]),
    createdAt: new Date(),
  };

  await store.stage(ev);

  const published = [];
  await store.relay(async (e) => {
    published.push(e);
  });

  assert.strictEqual(published.length, 1);
  assert.strictEqual(published[0].id, "evt-1");

  const published2 = [];
  await store.relay(async (e) => {
    published2.push(e);
  });
  assert.strictEqual(published2.length, 0);
});


test("PostgresIdempotencyStore lifecycle", async () => {
  const table = new Map();
  const fakeDB = {
    async query(sql, params) {
      if (sql.startsWith("SELECT")) {
        const row = table.get(params[0]);
        return { rows: row ? [row] : [] };
      }
      if (sql.startsWith("INSERT")) {
        if (params.length === 2) {
          const cur = table.get(params[0]);
          if (cur && cur.status === "started") {
            return { rows: [], rowCount: 0 };
          }
          table.set(params[0], {
            status: "started",
            expires_at: params[1],
          });
          return { rows: [], rowCount: 1 };
        }
        table.set(params[0], {
          status: "completed",
          response_payload: params[1],
          expires_at: params[2],
        });
        return { rows: [], rowCount: 1 };
      }
      if (sql.startsWith("DELETE")) {
        table.delete(params[0]);
        return { rows: [], rowCount: 1 };
      }
      return { rows: [] };
    },
  };

  const drv = createPostgresDriver(fakeDB);
  const store = drv.idempotency();
  const key = "k-pg-1";

  const { status, unlock } = await store.getOrLock(key, 60000);
  assert.strictEqual(status, "started");

  await assert.rejects(
    async () => store.getOrLock(key, 60000),
    /locked by an in-flight operation/
  );

  const payload = new Uint8Array([5, 6, 7]);
  await unlock(true, payload);

  const res2 = await store.getOrLock(key, 60000);
  assert.strictEqual(res2.status, "completed");
  assert.deepStrictEqual(res2.payload, payload);
});

test("PostgresOutboxStore stage and relay", async () => {
  const events = [];
  const fakeDB = {
    async query(sql, params) {
      if (sql.startsWith("INSERT")) {
        events.push({
          id: params[0],
          topic: params[1],
          event_name: params[2],
          payload: params[3],
          created_at: params[4],
          published: false,
        });
        return { rows: [], rowCount: 1 };
      }
      if (sql.startsWith("SELECT")) {
        const unpub = events.filter((e) => !e.published);
        return { rows: unpub };
      }
      if (sql.startsWith("UPDATE")) {
        const target = events.find((e) => e.id === params[0]);
        if (target) target.published = true;
        return { rows: [], rowCount: 1 };
      }
      return { rows: [] };
    },
  };

  const drv = createPostgresDriver(fakeDB);
  const store = drv.outbox();
  const ev = {
    id: "evt-pg-1",
    topic: "orders",
    eventName: "OrderPaid",
    payload: new Uint8Array([10, 20]),
    createdAt: new Date(),
  };

  await store.stage(ev);

  const published = [];
  await store.relay(async (e) => {
    published.push(e);
  });
  assert.strictEqual(published.length, 1);
  assert.strictEqual(published[0].id, "evt-pg-1");

  const published2 = [];
  await store.relay(async (e) => {
    published2.push(e);
  });
  assert.strictEqual(published2.length, 0);
});

test("DevHeadersResolver extracts claims and detects missing headers", async () => {
  const { DevHeadersResolver } = await import("../src/index.js");
  const resolver = new DevHeadersResolver();

  const headers = {
    "x-actor-id": "act-1",
    "x-actor-role": "admin",
    "x-actor-verified": "true",
    "x-actor-tenant-id": "t-1",
  };

  const claims = await resolver.resolveActor(headers);
  assert.strictEqual(claims.id, "act-1");
  assert.strictEqual(claims.role, "admin");
  assert.strictEqual(claims.verified, true);
  assert.strictEqual(claims.tenant_id, "t-1");

  await assert.rejects(
    async () => resolver.resolveActor({}),
    /unauthenticated/
  );
});

test("JWTOIDCResolver verifies an RS256 signature against the key set", async () => {
  const { JWTOIDCResolver } = await import("../src/index.js");
  const key = createSigningKey();
  const jwks = await serveJWKS([key]);
  const previous = process.env.AUTH_JWKS_URL;
  process.env.AUTH_JWKS_URL = jwks.url;

  try {
    const resolver = new JWTOIDCResolver("https://auth.example.com", "api");

    const claims = await resolver.resolveActor({
      authorization: `Bearer ${key.token(claimsFor("https://auth.example.com", "api"))}`,
    });
    assert.strictEqual(claims.id, "jwt-user-1");
    assert.strictEqual(claims.role, "user");

    const forgedClaims = claimsFor("https://auth.example.com", "api", { role: "admin" });
    const forged = key.tamper(key.token(claimsFor("https://auth.example.com", "api")), forgedClaims);
    await assert.rejects(
      async () => resolver.resolveActor({ authorization: `Bearer ${forged}` }),
      /unauthenticated/,
      "a payload edited after signing must not authenticate"
    );

    const unsigned = key.token(claimsFor("https://auth.example.com", "api"), { alg: "none", typ: "JWT" });
    await assert.rejects(
      async () => resolver.resolveActor({ authorization: `Bearer ${unsigned}` }),
      /unauthenticated/,
      "an alg:none token must not authenticate"
    );

    const expired = key.token(claimsFor("https://auth.example.com", "api", { exp: Math.floor(Date.now() / 1000) - 3600 }));
    await assert.rejects(
      async () => resolver.resolveActor({ authorization: `Bearer ${expired}` }),
      /unauthenticated/,
      "an expired token must not authenticate"
    );

    const noExpiry = key.token(claimsFor("https://auth.example.com", "api", { exp: undefined }));
    await assert.rejects(
      async () => resolver.resolveActor({ authorization: `Bearer ${noExpiry}` }),
      /unauthenticated/,
      "a token with no expiry must not authenticate"
    );

    await assert.rejects(
      async () => resolver.resolveActor({}),
      /unauthenticated/
    );
  } finally {
    if (previous === undefined) {
      delete process.env.AUTH_JWKS_URL;
    } else {
      process.env.AUTH_JWKS_URL = previous;
    }
    await jwks.close();
  }
});

// The key set is the only trust anchor: with none configured, nothing can be verified, so
// nothing may be accepted.
test("JWTOIDCResolver without a key set rejects", async () => {
  const { JWTOIDCResolver } = await import("../src/index.js");
  const key = createSigningKey();
  const resolver = new JWTOIDCResolver("https://auth.example.com", "api");

  await assert.rejects(
    async () => resolver.resolveActor({
      authorization: `Bearer ${key.token(claimsFor("https://auth.example.com", "api"))}`,
    }),
    /unauthenticated/
  );
});


