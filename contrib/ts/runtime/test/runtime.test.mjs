import test from "node:test";
import assert from "node:assert";
import { MemoryIdempotencyStore, MemoryCacheStore, MemoryOutboxStore, PostgresIdempotencyStore, PostgresOutboxStore, createPostgresDriver } from "../src/index.js";

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

test("JWTOIDCResolver verifies token payload", async () => {
  const { JWTOIDCResolver } = await import("../src/index.js");
  const resolver = new JWTOIDCResolver("https://auth.example.com", "api");

  const payload = Buffer.from(JSON.stringify({ sub: "jwt-user-1", role: "user", iss: "https://auth.example.com", aud: "api" })).toString("base64url");
  const token = `header.${payload}.sig`;

  const claims = await resolver.resolveActor({ authorization: `Bearer ${token}` });
  assert.strictEqual(claims.id, "jwt-user-1");
  assert.strictEqual(claims.role, "user");

  await assert.rejects(
    async () => resolver.resolveActor({}),
    /unauthenticated/
  );
});


