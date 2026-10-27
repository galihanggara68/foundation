import test from "node:test";
import assert from "node:assert";
import { MemoryIdempotencyStore, MemoryOutboxStore } from "../src/index.js";

test("TS MemoryIdempotencyStore 500 Concurrent Promise Lock Storm", async () => {
  const store = new MemoryIdempotencyStore();
  const key = "ts-concurrent-storm-key";
  const concurrency = 500;

  let started = 0;
  let conflicts = 0;

  const tasks = Array.from({ length: concurrency }, async (_, i) => {
    try {
      const { status, unlock } = await store.getOrLock(key, 10000);
      if (status === "started") {
        started++;
        // Small async yield to simulate work
        await new Promise((r) => setTimeout(r, 10));
        await unlock(true, new Uint8Array([1, 2, 3]));
      }
    } catch (err) {
      if (/locked by an in-flight operation/.test(err.message)) {
        conflicts++;
      } else {
        throw err;
      }
    }
  });

  await Promise.all(tasks);

  assert.strictEqual(started, 1, `Expected exactly 1 lock winner, got ${started}`);
  assert.strictEqual(conflicts, concurrency - 1, `Expected ${concurrency - 1} conflicts, got ${conflicts}`);

  const postResult = await store.getOrLock(key, 10000);
  assert.strictEqual(postResult.status, "completed");
  assert.deepStrictEqual(postResult.payload, new Uint8Array([1, 2, 3]));
});

test("TS MemoryOutboxStore Crash Recovery and Resume", async () => {
  const store = new MemoryOutboxStore();
  const total = 100;

  for (let i = 1; i <= total; i++) {
    await store.stage({
      id: `ts-evt-${i}`,
      topic: "billing",
      eventName: "InvoiceIssued",
      payload: new Uint8Array([i]),
      createdAt: new Date(),
    });
  }

  const delivered = new Set();

  // First relay crashes at 40 events
  await assert.rejects(async () => {
    await store.relay(async (evt) => {
      if (delivered.size >= 40) {
        throw new Error("TS_SIMULATED_CRASH");
      }
      delivered.add(evt.id);
    });
  }, /TS_SIMULATED_CRASH/);

  assert.strictEqual(delivered.size, 40);

  // Resume relay
  await store.relay(async (evt) => {
    delivered.add(evt.id);
  });

  assert.strictEqual(delivered.size, total, "All 100 events must be delivered without loss");
});
