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
  stage(tx: unknown, event: OutboxEvent): Promise<void>;
  relay(publisher: (event: OutboxEvent) => Promise<void>): Promise<void>;
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

  async stage(_tx: unknown, event: OutboxEvent) {
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
