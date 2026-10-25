export type IdempotencyStatus = "started" | "completed";

export type UnlockFunc = (completed: boolean, responsePayload?: Uint8Array) => Promise<void>;

export interface IdempotencyStore {
  getOrLock(key: string, ttlMs: number): Promise<{
    status: IdempotencyStatus;
    payload?: Uint8Array;
    unlock?: UnlockFunc;
  }>;
}

export interface OutboxEvent {
  id: string;
  topic: string;
  eventName: string;
  payload: Uint8Array;
  createdAt: Date;
}

export interface OutboxStore {
  stage(event: OutboxEvent): Promise<void>;
  relay(publisher: (event: OutboxEvent) => Promise<void>): Promise<void>;
  markPublished(id: string): Promise<void>;
}

export interface CacheStore {
  get(key: string): Promise<{ found: boolean; value?: Uint8Array }>;
  set(key: string, val: Uint8Array, ttlMs: number): Promise<void>;
  delete(key: string): Promise<void>;
}

export interface Driver {
  idempotency(): IdempotencyStore;
  outbox(): OutboxStore;
  cache(): CacheStore;
}

interface MemoryIdemEntry {
  status: IdempotencyStatus;
  payload?: Uint8Array;
  expiresAt: number;
}

export class MemoryIdempotencyStore implements IdempotencyStore {
  private entries = new Map<string, MemoryIdemEntry>();

  async getOrLock(key: string, ttlMs: number) {
    const now = Date.now();
    const entry = this.entries.get(key);
    if (entry && entry.expiresAt > now) {
      if (entry.status === "completed") {
        return { status: "completed" as const, payload: entry.payload };
      }
      throw new Error(`Idempotency key "${key}" is currently locked by an in-flight operation`);
    }

    this.entries.set(key, {
      status: "started",
      expiresAt: now + ttlMs,
    });

    const unlock: UnlockFunc = async (completed: boolean, responsePayload?: Uint8Array) => {
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

    return { status: "started" as const, unlock };
  }
}

interface MemoryCacheEntry {
  val: Uint8Array;
  expiresAt: number;
}

export class MemoryCacheStore implements CacheStore {
  private entries = new Map<string, MemoryCacheEntry>();

  async get(key: string) {
    const now = Date.now();
    const entry = this.entries.get(key);
    if (!entry || entry.expiresAt <= now) {
      return { found: false };
    }
    return { found: true, value: entry.val };
  }

  async set(key: string, val: Uint8Array, ttlMs: number) {
    this.entries.set(key, {
      val,
      expiresAt: Date.now() + ttlMs,
    });
  }

  async delete(key: string) {
    this.entries.delete(key);
  }
}

interface MemoryOutboxRecord {
  event: OutboxEvent;
  published: boolean;
}

export class MemoryOutboxStore implements OutboxStore {
  private records: MemoryOutboxRecord[] = [];

  async stage(event: OutboxEvent) {
    this.records.push({ event, published: false });
  }

  async relay(publisher: (event: OutboxEvent) => Promise<void>) {
    for (const rec of this.records) {
      if (!rec.published) {
        await publisher(rec.event);
        rec.published = true;
      }
    }
  }

  async markPublished(id: string): Promise<void> {
    for (const rec of this.records) {
      if (rec.event.id === id) {
        rec.published = true;
        return;
      }
    }
  }
}

export class MemoryDriver implements Driver {
  private idem = new MemoryIdempotencyStore();
  private out = new MemoryOutboxStore();
  private c = new MemoryCacheStore();

  idempotency() { return this.idem; }
  outbox() { return this.out; }
  cache() { return this.c; }
}

export function createMemoryDriver(): Driver {
  return new MemoryDriver();
}

export interface PgQueryable {
  query(text: string, values?: unknown[]): Promise<{ rows: Record<string, unknown>[]; rowCount?: number }>;
}

export class PostgresIdempotencyStore implements IdempotencyStore {
  private db: PgQueryable;

  constructor(db: PgQueryable) {
    this.db = db;
  }

  async getOrLock(key: string, ttlMs: number) {
    const now = Date.now();

    const res = await this.db.query("SELECT status, response_payload, expires_at FROM idempotency_keys WHERE key = $1", [key]);
    if (res.rows.length > 0) {
      const row = res.rows[0];
      const expiresAt = new Date(row.expires_at as string | number).getTime();
      if (expiresAt > now) {
        if (row.status === "completed") {
          const payload = row.response_payload ? new Uint8Array(row.response_payload as any) : undefined;
          return { status: "completed" as const, payload };
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

    const unlock: UnlockFunc = async (completed: boolean, responsePayload?: Uint8Array) => {
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

    return { status: "started" as const, unlock };
  }
}

export class PostgresOutboxStore implements OutboxStore {
  private db: PgQueryable;

  constructor(db: PgQueryable) {
    this.db = db;
  }

  async stage(event: OutboxEvent): Promise<void> {
    const createdAt = event.createdAt || new Date();
    const query = `INSERT INTO outbox_events (id, topic, event_name, payload, created_at, published)
VALUES ($1, $2, $3, $4, $5, FALSE)`;
    await this.db.query(query, [event.id, event.topic, event.eventName, event.payload, createdAt]);
  }

  async relay(publisher: (event: OutboxEvent) => Promise<void>): Promise<void> {
    const res = await this.db.query("SELECT id, topic, event_name, payload, created_at FROM outbox_events WHERE published = FALSE ORDER BY created_at ASC FOR UPDATE SKIP LOCKED");
    for (const row of res.rows) {
      const ev: OutboxEvent = {
        id: row.id as string,
        topic: row.topic as string,
        eventName: row.event_name as string,
        payload: row.payload as Uint8Array,
        createdAt: new Date(row.created_at as string | number),
      };
      await publisher(ev);
      await this.db.query("UPDATE outbox_events SET published = TRUE WHERE id = $1", [ev.id]);
    }
  }

  async markPublished(id: string): Promise<void> {
    await this.db.query("UPDATE outbox_events SET published = TRUE WHERE id = $1", [id]);
  }
}

export class OutboxRelayWorker {
  private store: OutboxStore;
  private publisher: (event: OutboxEvent) => Promise<void>;
  private intervalMs: number;
  private timer?: NodeJS.Timeout;

  constructor(store: OutboxStore, publisher: (event: OutboxEvent) => Promise<void>, intervalMs = 100) {
    this.store = store;
    this.publisher = publisher;
    this.intervalMs = intervalMs;
  }

  async pollOnce(): Promise<void> {
    await this.store.relay(this.publisher);
  }

  start(): void {
    this.timer = setInterval(() => {
      void this.pollOnce();
    }, this.intervalMs);
  }

  stop(): void {
    if (this.timer) {
      clearInterval(this.timer);
      this.timer = undefined;
    }
  }
}

export class PostgresDriver implements Driver {
  private idem: IdempotencyStore;
  private out: OutboxStore;
  private c = new MemoryCacheStore();

  constructor(db: PgQueryable) {
    this.idem = new PostgresIdempotencyStore(db);
    this.out = new PostgresOutboxStore(db);
  }

  idempotency() { return this.idem; }
  outbox() { return this.out; }
  cache() { return this.c; }
}

export function createPostgresDriver(db: PgQueryable): Driver {
  return new PostgresDriver(db);
}

