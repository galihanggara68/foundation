# Tactical Traits and Runtime Durability

Tactical traits in Foundation (`.fdn`) annotate actions and entities with operational patterns.
These patterns include ACID transactions, deduplication, transactional outboxes, retries, and caching.
Traits keep operational concerns separate from pure business logic.

---

## 1. `@transactional`

The `@transactional` trait wraps action execution within a database transaction boundary.

```fdn
action TransferFunds(req: TransferRequest) -> TransferResponse
  @transactional
  @http(method: POST, path: "/wallets/transfer")
{
  let source = get Wallet where id == req.source_id or fail WalletNotFound
  let target = get Wallet where id == req.target_id or fail WalletNotFound

  source.balance = source.balance - req.amount
  target.balance = target.balance + req.amount

  save source
  save target

  publish MoneyTransferred(from: source.id, to: target.id, amount: req.amount)

  return TransferResponse(success: true)
}
```

### Runtime Behavior

1. **Transaction Lifecycle**: The runtime driver calls `BeginTx(ctx)`. All repository reads, writes, and outbox event insertions run on that active transaction.
2. **Commit on Success**: If every pipeline step completes without errors, the driver calls `tx.Commit()` before returning the response.
3. **Rollback on Error**: If any step returns a failure, the driver calls `tx.Rollback()` automatically. The database remains unchanged.
4. **Boundary Checks**: Calling long-running external API slots (marked with `@external`) inside a `@transactional` block triggers a compiler warning or error. This check prevents database connection pool exhaustion.

---

## 2. `@idempotent(key: <expression>)`

The `@idempotent` trait guarantees at-most-once execution for mutating actions.
The runtime caches responses and serializes concurrent duplicate requests with a deterministic key.

```fdn
action ProcessPayment(req: PaymentRequest) -> PaymentReceipt
  @transactional
  @idempotent(key: req.idempotency_key)
  @http(method: POST, path: "/payments/process")
{
  let charge = PaymentGateway.Charge(req.card_token, req.amount)
  save Payment(id: charge.id, amount: req.amount, status: charge.status)
  return PaymentReceipt(id: charge.id, status: charge.status)
}
```

### Key Expression

- The `key:` argument takes an identifier or property path from action parameters (for example `req.idempotency_key`).
- When receiving an HTTP request, the runtime extracts the key value before running the pipeline.

### State Transitions and Locking

The runtime queries the underlying `IdempotencyStore`:

1. **New Request**: The runtime claims an in-flight lease in the database (`status = 'IN_FLIGHT'` with an expiration window).
2. **Duplicate Request in Flight**: If a duplicate request arrives before the lease expires, the runtime returns HTTP 409 Conflict.
3. **Completed Request**: Upon successful execution, the runtime stores the response payload and HTTP status (`status = 'COMPLETED'`). Future duplicate requests return the cached response immediately.
4. **Lease Expiration and Crash Recovery**: If a server node crashes mid-flight, subsequent retry attempts can safely re-acquire expired leases.

---

## 3. `@outbox`

The `@outbox` trait implements the Transactional Outbox Pattern to guarantee event delivery without distributed two-phase commit transactions.

```fdn
action PlaceOrder(req: CreateOrder) -> OrderSummary
  @transactional
  @outbox
  @http(method: POST, path: "/orders")
{
  save Order(id: generate_id(), customer_id: req.customer_id, total: req.total)
  publish OrderPlaced(order_id: req.order_id, customer_id: req.customer_id)
  return OrderSummary(order_id: req.order_id)
}
```

### Pipeline Integration

When an action declares `@outbox`:
- Any `publish EventName(...)` statement writes a row directly to `outbox_events` within the active transaction.
- If the transaction rolls back, the database removes the event row. If the transaction commits, the database records the event row permanently.

### The Outbox Relay Worker

Foundation generates a background relay worker (`OutboxRelayWorker`):
- **Safe Polling**: In PostgreSQL, the worker queries pending events using `SELECT ... FOR UPDATE SKIP LOCKED`. Multiple worker replicas pull events in parallel without contention.
- **Delivery**: The worker forwards events to the configured event bus.
- **Status Updates**: After the broker acknowledges delivery, the worker updates the record to `PUBLISHED` with a timestamp.

---

## 4. PostgreSQL DDL Generation for Tactical Traits

When blueprints use `@idempotent` or `@outbox`, the compiler adds required tables to `schema.sql` and migration scripts:

```sql
-- Generated for @idempotent
CREATE TABLE IF NOT EXISTS idempotency_keys (
    key VARCHAR(255) PRIMARY KEY,
    response BYTEA,
    status VARCHAR(50) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL
);

-- Generated for @outbox
CREATE TABLE IF NOT EXISTS outbox_events (
    id VARCHAR(64) PRIMARY KEY,
    event_type VARCHAR(255) NOT NULL,
    payload BYTEA NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'PENDING',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    published_at TIMESTAMPTZ,
    retry_count INT NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_outbox_events_pending 
ON outbox_events (status, created_at) 
WHERE status = 'PENDING';
```

---

## 5. Additional Tactical Traits

### `@cache(ttl: "<duration>")`

Caches idempotent query responses in an in-memory cache or Redis cluster for the specified duration.

```fdn
action GetCatalogItem(id: string) -> ItemDetails
  @cache(ttl: "10m")
  @http(method: GET, path: "/catalog/{id}")
{
  let item = get Item where id == id or fail ItemNotFound
  return ItemDetails(item: item)
}
```

### `@retry(attempts: <int>, backoff: "<duration>")`

Retries failed operations that encounter transient errors with exponential backoff.

```fdn
action SyncInventory(warehouse_id: string) -> SyncResult
  @retry(attempts: 3, backoff: "200ms")
{
  return InventoryService.Sync(warehouse_id)
}
```

### `@event_handler(EventName)`

Marks an action as an asynchronous domain event consumer rather than an HTTP endpoint.

```fdn
action OnOrderPlaced(evt: OrderPlaced)
  @event_handler(OrderPlaced)
{
  NotificationService.SendReceipt(evt.customer_id, evt.order_id)
}
```

---

## Summary Matrix

| Trait | Target | Purpose | Key Artifacts |
|---|---|---|---|
| `@transactional` | `action` | ACID boundary across reads and writes | `tx.Begin()`, `tx.Commit()`, `tx.Rollback()` |
| `@idempotent(key)` | `action` | Request deduplication and locking | `idempotency_keys` table, `IdempotencyStore` |
| `@outbox` | `action` | Reliable event publication | `outbox_events` table, `OutboxRelayWorker` |
| `@cache(ttl)` | `action` | Read-through response caching | In-memory or Redis cache keys |
| `@retry(attempts, backoff)` | `action` | Transient failure fault tolerance | Exponential backoff wrapper |
| `@event_handler(Event)` | `action` | Event consumer binding | Domain event listener registration |
