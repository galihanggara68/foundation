# Extension Patterns, Architectural Seams, and Platform Boundaries

Foundation uses deterministic code compilation based on the **90/10 Rule**:
- **90 Percent Generated Core**: Domain models, relational schemas, optimistic concurrency control, deterministic action pipelines, input validation, HTTP routing, and port interfaces.
- **10 Percent Handcrafted Seam**: Custom business calculations, custom database drivers, complex SQL joins, third-party APIs, and authentication issuance.

This document provides solutions for real-world backend requirements that touch Foundation seams or platform boundaries.

---

## Architectural Seam Overview

Foundation compiles blueprints into **Hexagonal Architecture** (Ports and Adapters):

```
       ┌────────────────────────────────────────────────────────┐
       │                   Primary Ingress                      │
       │     (HTTP Router, Middleware, Actor Resolver)          │
       └───────────────────────────┬────────────────────────────┘
                                   │
                                   ▼
       ┌────────────────────────────────────────────────────────┐
       │                   Application Core                     │
       │    Use Cases (Action Pipelines) & Domain Contracts     │
       │          (Entities, Values, Errors, Policies)          │
       └───────────────────────────┬────────────────────────────┘
                                   │
                                   ▼
       ┌────────────────────────────────────────────────────────┐
       │                  Secondary Egress                      │
       │   Repository Ports (Postgres, Memory) & Custom Slots   │
       └────────────────────────────────────────────────────────┘
```

Generated code defines ports as Go interfaces in `internal/domain`.
The boundary between generated and handwritten code remains clean, testable, and swappable.

---

## 1. Join Tables and Relational Aggregates

### The Boundary

Foundation maps each `entity` to one database table and one repository interface.
The `.fdn` grammar does not include SQL `JOIN` keywords or relational association syntax (`hasMany`, `belongsToMany`).

### Extension Patterns

#### Pattern A: Embedded Value Objects (JSONB)

Use this pattern when child records belong exclusively to an Aggregate Root.
Child records must not need foreign keys from other tables.
Examples include order line items, metadata tags, and address blocks:

```fdn
value OrderItem {
  product_id: uuid
  title: string
  quantity: int
  unit_price: decimal(18, 4)
}

entity Order {
  id: uuid @pk
  buyer_id: uuid
  items: list<OrderItem>
  total: decimal(18, 4)
}
```

*Mechanism*: The PostgreSQL adapter serializes `list<OrderItem>` into a native `JSONB` column.
A single query loads and saves the aggregate atomically.

#### Pattern B: Multi-Query Action Pipelines (Normalized Entities)

Use this pattern when child entities require independent lifecycles, queries, or foreign keys:

```fdn
entity Order {
  id: uuid @pk
  buyer_id: uuid
  status: string
}

entity OrderItem {
  id: uuid @pk
  order_id: uuid @fk(Order.id)
  product_id: uuid
  quantity: int
}

action GetOrderDetails(id: uuid) -> OrderDetailsView {
  query order from Order where id == id
  query items from OrderItem where order_id == order.id
  return OrderDetailsView(order: order, items: items)
}
```

*Mechanism*: The generated use case runs two indexed SQL queries (`SELECT ... FROM orders WHERE id = $1` and `SELECT ... FROM order_items WHERE order_id = $1`).
Both queries participate in the active transaction context (`connFromContext`).

#### Pattern C: CQRS Read-Model Query Slots (True SQL Joins)

Use this pattern for reporting, multi-table joins, aggregations (`COUNT`, `SUM`), and analytical views:

```fdn
value OrderSummaryView {
  order_id: uuid
  customer_name: string
  total_items: int
  grand_total: decimal(18, 4)
}

slot OrderQueryService {
  getOrderSummary(id: uuid) -> OrderSummaryView throws NotFound
}

action GetOrderSummary(id: uuid) -> OrderSummaryView {
  call summary = OrderQueryService.getOrderSummary(id)
  return summary
}
```

*Handwritten Implementation (`internal/adapters/postgres/order_query_service.go`)*:

```go
package postgres

import (
	"context"
	"database/sql"
	"your-module/internal/domain"
)

type OrderQueryServiceAdapter struct {
	db *sql.DB
}

func NewOrderQueryServiceAdapter(db *sql.DB) *OrderQueryServiceAdapter {
	return &OrderQueryServiceAdapter{db: db}
}

func (s *OrderQueryServiceAdapter) GetOrderSummary(ctx context.Context, id string) (domain.OrderSummaryView, error) {
	const query = `
		SELECT o.id, c.name, COUNT(i.id), COALESCE(SUM(i.quantity * i.unit_price), 0)
		FROM orders o
		JOIN customers c ON c.id = o.customer_id
		LEFT JOIN order_items i ON i.order_id = o.id
		WHERE o.id = $1
		GROUP BY o.id, c.name`

	var view domain.OrderSummaryView
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&view.OrderID,
		&view.CustomerName,
		&view.TotalItems,
		&view.GrandTotal,
	)
	if err == sql.ErrNoRows {
		return domain.OrderSummaryView{}, domain.ErrNotFound
	}
	return view, err
}
```

---

## 2. Custom UUIDs and Swapping Repository Adapters

### The Boundary

Foundation emits `uuid` fields as Go `string` values in `internal/domain`.
This design prevents third-party library lock-in.
Default repository adapters use standard library `database/sql`.

### Extension Patterns

#### Swapping to `pgx` or `gorm`

Generated use cases depend only on repository interfaces in `internal/domain`.
You can replace the default adapter with a custom implementation without changing use cases:

1. **Write the Custom Adapter**:
   ```go
   package pgxadapter

   import (
   	"context"
   	"github.com/jackc/pgx/v5/pgxpool"
   	"your-module/internal/domain"
   )

   type ProductRepository struct {
   	pool *pgxpool.Pool
   }

   func NewProductRepository(pool *pgxpool.Pool) domain.ProductRepository {
   	return &ProductRepository{pool: pool}
   }

   func (r *ProductRepository) FindByID(ctx context.Context, id string) (*domain.Product, error) {
   	var p domain.Product
   	err := r.pool.QueryRow(ctx, "SELECT id, title, price, version FROM products WHERE id = $1", id).
   		Scan(&p.ID, &p.Title, &p.Price, &p.Version)
   	if err != nil {
   		return nil, domain.ErrNotFound
   	}
   	return &p, nil
   }
   ```

2. **Wire the Custom Adapter in `cmd/app/main.go`**:
   ```go
   pool, _ := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
   productRepo := pgxadapter.NewProductRepository(pool)
   listProductsUC := usecases.NewListProductsUseCase(productRepo)
   ```

#### ID Generation Strategies

- **Client-Provided**: The caller passes `id` in the input payload.
- **Database-Generated**: Configure `DEFAULT gen_random_uuid()` on PostgreSQL primary keys.
- **Slot Generator**: Declare a slot and implement it with `google/uuid`, ULID, or Snowflake identifiers.

```fdn
slot IDGenerator {
  newID() -> uuid
}
```

---

## 3. Custom HTTP Headers and Context Enrichment

### The Boundary

Foundation maps route paths, query parameters, and JSON request or response bodies.
The DSL does not provide syntax to read or write arbitrary HTTP headers inside action pipelines.

### Extension Patterns

#### Ingress Request Headers through HTTP Middleware

The generated `httpAdapter.Server` implements `net/http.Handler`.
Wrap the server with custom HTTP middleware in `cmd/app/main.go`:

```go
func CorrelationAndHeaderMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := r.Header.Get("X-Request-ID")
		if reqID == "" {
			reqID = uuid.NewString()
		}
		w.Header().Set("X-Request-ID", reqID)

		ctx := context.WithValue(r.Context(), "request_id", reqID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// In main.go:
httpServer := httpAdapter.NewServer(actorResolver, usecases...)
wrappedHandler := CorrelationAndHeaderMiddleware(httpServer)
http.ListenAndServe(":8080", wrappedHandler)
```

#### Passing Headers into Actions through Actor Claims

When an action or policy requires header values (for example `X-Tenant-ID`):

1. **Declare the claim on the actor**:
   ```fdn
   actor User {
     id: uuid
     role: string
     tenant_id: string
   }
   ```
2. **Read the header in the Actor Resolver**:
   ```go
   func (a *CustomActorResolver) ResolveActor(ctx context.Context, r *http.Request) (domain.Actor, error) {
       tenantID := r.Header.Get("X-Tenant-ID")
       return domain.Actor{
           ID: claims.Subject,
           Role: claims.Role,
           TenantID: tenantID,
       }, nil
   }
   ```
3. **Reference the claim in Policies or Pipelines**:
   ```fdn
   policy BelongsToTenant(actor: Actor, order: Order) {
     actor.tenant_id == order.tenant_id
   }
   ```

#### Tracing and Header Middleware with Generator Plugins

Pluggable generator pipelines support middleware injection at build time:

```yaml
targets:
  - name: backend
    language: go
    architecture: hexagonal
    scaffold: builtin
    generators:
      - "foundation-gen-otel"
```

The plugin completes these tasks:
- Emits distributed tracing middleware for W3C `traceparent` and `X-Trace-ID` headers.
- Wraps the generated Chi router so incoming requests create OpenTelemetry spans.
- Preserves standard route handling and error mapping.

---

## 4. Rich Domain Models versus Action Pipelines (DDD Style)

### The Boundary

Domain-Driven Design aggregates protect invariants through methods (such as `order.Cancel()`).
Foundation places business behavior in action pipelines and keeps entities as plain data structures.

### Extension Patterns

#### Declarative Invariant Guards

Encode business invariants directly in the action pipeline:

```fdn
action CancelOrder(id: uuid, reason: string) -> Order {
  query order from Order where id == id
  guard order.status != "shipped" else throws CannotCancelShippedOrder
  guard order.status != "cancelled" else throws OrderAlreadyCancelled
  mutate order {
    status = "cancelled"
  }
  emit OrderCancelled(order_id: order.id, reason: reason)
  return order
}
```

#### Complex Domain Services through Slots

Use slots for complex calculations that exceed pipeline expressions (for example risk scoring or tax calculations):

```fdn
slot RiskEngine {
  evaluateTransaction(account_id: uuid, amount: decimal) -> bool throws RiskRejected
}

action TransferFunds(from_id: uuid, to_id: uuid, amount: decimal) -> TransferResult {
  call RiskEngine.evaluateTransaction(from_id, amount)
  // Proceed with balance updates
}
```

#### Non-Generated Domain Methods

In Go, all files in `internal/domain` share one package.
You can add a handwritten file such as `internal/domain/order_behavior.go`:

```go
package domain

func (o *Order) CanBeRefunded() bool {
	return o.Status == "delivered" && o.RefundWindowDays <= 30
}
```

Custom slots or background workers can share this logic.

#### Rich Aggregate Roots through Generator Plugins

If your domain requires DDD aggregate roots with private invariant constructors and encapsulated mutation methods, use `foundation-gen-rich-ddd`:

```yaml
targets:
  - name: backend
    language: go
    architecture: hexagonal
    scaffold: builtin
    generators:
      - "foundation-gen-rich-ddd"
```

When you build the project, Foundation completes these steps:
1. Emits standard hexagonal scaffolding in memory (`scaffold: builtin`).
2. Runs `foundation-gen-rich-ddd` to transform entities into rich aggregates.
3. Writes starter configuration files (`config/ddd.starter.json`) to disk without tracking them in the manifest.

---

## 5. JWT Token Generation and Local Authentication

### The Boundary

The `authn: jwt-oidc` configuration provides an ingress token verifier.
The verifier inspects incoming `Authorization: Bearer <token>` headers and resolves actors.
The verifier checks token signatures against `AUTH_JWKS_URL` before reading claims.
Invalid signatures return HTTP 401 (see [ADR 0017](../adr/0017-jwt-verification-contract.md)).
Foundation does not generate an Identity Provider or token issuer.
Production backends usually rely on external identity providers.

### Extension Patterns

#### Building a Local Login Action through a Crypto Slot

To issue tokens locally, declare and implement a token slot:

```fdn
value AuthToken {
  access_token: string
  token_type: string
  expires_in: int
}

slot TokenService {
  hashPassword(raw: string) -> string
  verifyPassword(raw: string, hash: string) -> bool
  generateToken(user_id: uuid, role: string) -> string
}

action Login(email: string, password: string) -> AuthToken {
  query user from User where email == email
  guard TokenService.verifyPassword(password, user.password_hash) else throws InvalidCredentials
  set token_str = call TokenService.generateToken(user.id, user.role)
  return AuthToken(
    access_token: token_str,
    token_type: "Bearer",
    expires_in: 3600
  )
}
```

*Handwritten Implementation (`internal/adapters/crypto/token_service.go`)*:

```go
package crypto

import (
	"context"
	"time"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type TokenServiceImpl struct {
	secret []byte
}

func (s *TokenServiceImpl) VerifyPassword(ctx context.Context, raw, hash string) (bool, error) {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(raw))
	return err == nil, nil
}

func (s *TokenServiceImpl) GenerateToken(ctx context.Context, userID, role string) (string, error) {
	claims := jwt.MapClaims{
		"sub":  userID,
		"role": role,
		"exp":  time.Now().Add(time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.secret)
}
```

---

## 6. Schema Migrations and Database Evolution

### The Boundary

Foundation generates deterministic DDL (`schema.sql`) for declared entities, primary keys, foreign keys, constraints, and tables.
Foundation does not track migration history tables or generate automatic `ALTER TABLE` diffs.

### Extension Patterns

#### Pairing with External Migration Runners

Integrate Foundation with migration tools (`goose`, `atlas`, or `golang-migrate`):

1. **Initial Baseline**: Use `schema.sql` as `migrations/00001_initial_schema.sql`.
2. **Schema Evolution Workflow**:
   - Run `foundation build` to update `schema.sql`.
   - Create a sequential migration with `goose` or `atlas`:
     ```bash
     atlas migrate diff add_phone_to_users --to file://dist/schema.sql --dev "docker://postgres/16/dev"
     ```
   - Run migrations in your CI/CD pipeline before starting the binary.

---

## Quick Reference: Needs versus Patterns

| Developer Need | Architectural Seam or Recommendation |
|---|---|
| **SQL joins and aggregations** | CQRS Read-Model **Slot** (`internal/adapters/postgres`) |
| **Alternative DB drivers (`pgx`, `gorm`)** | Implement generated `domain.Repository` port in `internal/adapters/` |
| **Custom UUID libraries (ULID, Snowflake)** | Kept as `string` at domain boundary; convert in custom adapter or Slot |
| **Request headers and correlation IDs** | Standard Go HTTP middleware or generator plugin (`foundation-gen-otel`) |
| **Distributed tracing** | Generator plugin (`foundation-gen-otel`) injecting router tracing middleware |
| **Rich DDD aggregate roots** | Generator plugin (`foundation-gen-rich-ddd`) enforcing invariants |
| **Custom language targets (Python, Rust)** | External generator plugin (`foundation-gen-*`) with `scaffold: none` |
| **Tenant isolation via headers** | Extract in `ActorResolver` adapter; bind to `actor` claims |
| **Custom response headers or cookies** | HTTP response interceptor middleware |
| **Token issuance (`/login`, JWT signing)** | Implement a `TokenService` **Slot** using `bcrypt` and `golang-jwt` |
| **Schema migration lifecycle** | Use generated `schema.sql` with `goose`, `golang-migrate`, or `atlas` |
| **Complex domain calculations** | Domain Service **Slot** called from the Action Pipeline |
