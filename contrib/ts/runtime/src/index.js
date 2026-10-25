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

  async markPublished(id) {
    for (const rec of this.records) {
      if (rec.event.id === id) {
        rec.published = true;
        return;
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

export class PostgresIdempotencyStore {
  constructor(db) {
    this.db = db;
  }

  async getOrLock(key, ttlMs) {
    const now = Date.now();

    const res = await this.db.query("SELECT status, response_payload, expires_at FROM idempotency_keys WHERE key = $1", [key]);
    if (res.rows && res.rows.length > 0) {
      const row = res.rows[0];
      const expiresAt = new Date(row.expires_at).getTime();
      if (expiresAt > now) {
        if (row.status === "completed") {
          const payload = row.response_payload ? new Uint8Array(row.response_payload) : undefined;
          return { status: "completed", payload };
        }
        if (row.status === "started") {
          throw new Error(`Idempotency key "${key}" is currently locked by an in-flight operation`);
        }
      }
    }

    const upsertStarted = `INSERT INTO idempotency_keys (key, status, response_payload, created_at, expires_at)
VALUES ($1, 'started', NULL, NOW(), $2)
ON CONFLICT (key) DO UPDATE
SET status = 'started', response_payload = NULL, created_at = NOW(), expires_at = EXCLUDED.expires_at
WHERE idempotency_keys.expires_at <= NOW()`;
    const lockRes = await this.db.query(upsertStarted, [key, new Date(now + ttlMs)]);
    if (lockRes.rowCount === 0) {
      throw new Error(`Idempotency key "${key}" is currently locked by an in-flight operation`);
    }

    const unlock = async (completed, responsePayload) => {
      if (!completed) {
        await this.db.query("DELETE FROM idempotency_keys WHERE key = $1 AND status = 'started'", [key]);
        return;
      }
      const expires = new Date(Date.now() + ttlMs);
      const upsertSQL = `INSERT INTO idempotency_keys (key, status, response_payload, created_at, expires_at)
VALUES ($1, 'completed', $2, NOW(), $3)
ON CONFLICT (key) DO UPDATE
SET status = 'completed', response_payload = EXCLUDED.response_payload, expires_at = EXCLUDED.expires_at`;
      await this.db.query(upsertSQL, [key, responsePayload, expires]);
    };

    return { status: "started", unlock };
  }
}

export class PostgresOutboxStore {
  constructor(db) {
    this.db = db;
  }

  async stage(tx, event) {
    const client = (tx && typeof tx.query === "function") ? tx : this.db;
    const createdAt = event.createdAt || new Date();
    const query = `INSERT INTO outbox_events (id, topic, event_name, payload, created_at, published)
VALUES ($1, $2, $3, $4, $5, FALSE)`;
    await client.query(query, [event.id, event.topic, event.eventName, event.payload, createdAt]);
  }

  async relay(publisher) {
    const res = await this.db.query("SELECT id, topic, event_name, payload, created_at FROM outbox_events WHERE published = FALSE ORDER BY created_at ASC FOR UPDATE SKIP LOCKED");
    for (const row of res.rows) {
      const ev = {
        id: row.id,
        topic: row.topic,
        eventName: row.event_name,
        payload: row.payload,
        createdAt: new Date(row.created_at),
      };
      await publisher(ev);
      await this.db.query("UPDATE outbox_events SET published = TRUE WHERE id = $1", [ev.id]);
    }
  }

  async markPublished(id) {
    await this.db.query("UPDATE outbox_events SET published = TRUE WHERE id = $1", [id]);
  }
}

export class OutboxRelayWorker {
  constructor(store, publisher, intervalMs = 100) {
    this.store = store;
    this.publisher = publisher;
    this.intervalMs = intervalMs;
  }

  async pollOnce() {
    await this.store.relay(this.publisher);
  }

  start() {
    this.timer = setInterval(() => {
      void this.pollOnce();
    }, this.intervalMs);
  }

  stop() {
    if (this.timer) {
      clearInterval(this.timer);
      this.timer = undefined;
    }
  }
}

export class PostgresDriver {
  constructor(db) {
    this.idem = new PostgresIdempotencyStore(db);
    this.out = new PostgresOutboxStore(db);
    this.c = new MemoryCacheStore();
  }

  idempotency() { return this.idem; }
  outbox() { return this.out; }
  cache() { return this.c; }
}

export function createPostgresDriver(db) {
  return new PostgresDriver(db);
}

