export class MemoryIdempotencyStore {
  constructor() {
    this.entries = new Map();
  }

  async getOrLock(key, ttlMs) {
    const now = Date.now();
    const entry = this.entries.get(key);
    if (entry && entry.expiresAt > now) {
      if (entry.status === "completed") {
        return { status: "completed", payload: entry.payload };
      }
      throw new Error(`Idempotency key "${key}" is currently locked by an in-flight operation`);
    }

    this.entries.set(key, {
      status: "started",
      expiresAt: now + ttlMs,
    });

    const unlock = async (completed, responsePayload) => {
      if (!completed) {
        this.entries.delete(key);
        return;
      }
      this.entries.set(key, {
        status: "completed",
        payload: responsePayload,
        expiresAt: Date.now() + ttlMs,
      });
    };

    return { status: "started", unlock };
  }
}

export class MemoryCacheStore {
  constructor() {
    this.entries = new Map();
  }

  async get(key) {
    const now = Date.now();
    const entry = this.entries.get(key);
    if (!entry || entry.expiresAt <= now) {
      return { found: false };
    }
    return { found: true, value: entry.val };
  }

  async set(key, val, ttlMs) {
    this.entries.set(key, {
      val,
      expiresAt: Date.now() + ttlMs,
    });
  }

  async delete(key) {
    this.entries.delete(key);
  }
}

export class MemoryOutboxStore {
  constructor() {
    this.records = [];
  }

  async stage(tx, event) {
    this.records.push({ event, published: false });
  }

  async relay(publisher) {
    for (const rec of this.records) {
      if (!rec.published) {
        await publisher(rec.event);
        rec.published = true;
      }
    }
  }
}

export class MemoryDriver {
  constructor() {
    this.idem = new MemoryIdempotencyStore();
    this.out = new MemoryOutboxStore();
    this.c = new MemoryCacheStore();
  }

  idempotency() { return this.idem; }
  outbox() { return this.out; }
  cache() { return this.c; }
}

export function createMemoryDriver() {
  return new MemoryDriver();
}
