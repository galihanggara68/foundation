import test from "node:test";
import assert from "node:assert";
import { MemoryIdempotencyStore, MemoryCacheStore, MemoryOutboxStore } from "../src/index.js";

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

  await store.stage(null, ev);

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
