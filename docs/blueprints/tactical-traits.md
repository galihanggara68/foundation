# Tactical Traits & Runtime Durability

Tactical traits in Foundation (`.fdn`) annotate actions and entities with enterprise operational patterns—such as ACID transactions, deduplication, transactional outboxes, retries, and caching—without polluting the pure business logic of your pipelines.

---

## 1. `@transactional`

The `@transactional` trait wraps the entire execution of an action within a database transaction boundary.

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
1. **Transaction Lifecycle**: The runtime driver calls `BeginTx(ctx)`. All subsequent repository reads, writes, and outbox event insertions run on that active transaction.
2. **Commit on Success**: If every pipeline step completes without returning an error, `tx.Commit()` is called before returning the HTTP response.
3. **Rollback on Error**: If any step returns a failure (e.g., `fail InsufficientFunds` or an unexpected runtime error), `tx.Rollback()` is automatically executed, leaving the database state untouched.
4. **Boundary Checks**: Calling long-running external API slots (marked with `@external`) inside a `@transactional` block triggers a compiler warning or error to prevent database connection pool exhaustion.

---

## 2. `@idempotent(key: <expression>)`

The `@idempotent` trait guarantees at-most-once execution for mutating actions by caching results and serializing concurrent duplicate requests using a deterministic key.

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
- The `key:` argument takes an identifier or property access path from the action request parameters (e.g., `req.idempotency_key` or `id`).
- When an incoming HTTP request is received, the runtime extracts the key value before pipeline execution begins.

### State Transitions & Locking
The runtime queries the underlying `IdempotencyStore`:
1. **New Request**: An in-flight lease is claimed in the database (`status = 'IN_FLIGHT'` with an expiration window).
2. **Duplicate while In-Flight**: If a concurrent request arrives with the same key before the lease expires, the runtime responds with an HTTP `409 Conflict` (or waits on the lease depending on driver configuration).
3. **Completed Request**: Upon successful action execution, the serialized response payload and HTTP status are stored (`status = 'COMPLETED'`). Future requests with identical keys bypass the pipeline and immediately return the cached response.
4. **Lease Expiration / Crash Recovery**: If a node crashes mid-flight, expired leases (`expires_at <= NOW()`) can be safely re-acquired by subsequent retry attempts.

---

## 3. `@outbox`

The `@outbox` trait implements the Transactional Outbox Pattern to guarantee at-least-once domain event publishing without distributed 2PC transactions.

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
When `@outbox` is present on an action:
- Any `publish EventName(...)` statement inside the action writes a row directly to the `outbox_events` table within the same database transaction.
- If the transaction rolls back, the event is rolled back. If the transaction commits, the event is guaranteed to be recorded in durable storage.

### The Outbox Relay Worker
Foundation generates and deploys a background relay worker (`OutboxRelayWorker`):
- **Concurrency-Safe Polling**: In PostgreSQL, the worker queries pending events using `SELECT ... FOR UPDATE SKIP LOCKED`, allowing multiple worker replicas to safely pull events in parallel without contention.
- **Delivery**: Events are forwarded to the configured domain event bus (e.g., NATS, Kafka, or in-memory bus).
- **Status Update**: Once acknowledged by the broker, the worker updates the event record to `PUBLISHED` with a timestamp.

---

## 4. PostgreSQL DDL Generation for Tactical Traits

When `@idempotent` or `@outbox` is used in any blueprint, the Foundation compiler automatically adds the required tactical tables to generated schema DDL (`schema.sql`) and sequential migrations (`000001_initial.up.sql`).

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
Caches idempotent query action responses in an in-memory cache or Redis cluster for the specified duration.

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
Retries failed operations that encounter transient errors (such as slot network timeouts or database deadlocks) using exponential backoff.

```fdn
action SyncInventory(warehouse_id: string) -> SyncResult
  @retry(attempts: 3, backoff: "200ms")
{
  return InventoryService.Sync(warehouse_id)
}
```

### `@event_handler(EventName)`
Marks an action as an asynchronous domain event consumer rather than an HTTP-triggered endpoint.

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
| `@outbox` | `action` | Reliable at-least-once event publication | `outbox_events` table, `OutboxRelayWorker` |
| `@cache(ttl)` | `action` | Read-through response caching | In-memory / Redis cache keys |
| `@retry(attempts, backoff)` | `action` | Transient failure fault tolerance | Exponential backoff wrapper |
| `@event_handler(Event)` | `action` | Event consumer binding | Domain event listener registration |
