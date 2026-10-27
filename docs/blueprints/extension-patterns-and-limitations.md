# Extension Patterns, Architectural Seams & Platform Boundaries

Foundation is designed around deterministic code compilation following the **90/10 Rule**:
- **90% Generated Core**: Domain models, relational schemas, optimistic concurrency control, deterministic action pipelines, input validation, HTTP routing, and port interfaces.
- **10% Handcrafted Seam**: Proprietary business calculations, custom database drivers, complex SQL joins, third-party APIs, and authentication issuance.

This document serves as a practical, cookbook-style reference for real-world backend engineering requirements that fall along Foundation's architectural seams or current platform boundaries.

---

## Architectural Seam Overview

Foundation lowers blueprints into **Hexagonal Architecture** (Ports and Adapters):

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

Because ports are pure Go interfaces emitted in `internal/domain`, the boundary between generated and handwritten code is clean, testable, and swappable without modifying the compiler.

---

## 1. Join Tables & Relational Aggregates

### The Boundary
Foundation maps each `entity` 1:1 to a database table and repository interface. There is no SQL `JOIN` keyword or relational association syntax (`hasMany`, `belongsToMany`) in the `.fdn` grammar.

### Extension Patterns

#### Pattern A: Embedded Value Objects (JSONB)
When child records belong exclusively to an Aggregate Root and do not require foreign keys from other tables (e.g., line items in an order, metadata tags, address blocks):

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

*Mechanism*: The PostgreSQL adapter serializes `list<OrderItem>` into a native `JSONB` column. The entire aggregate is loaded and persisted atomically in a single query.

#### Pattern B: Multi-Query Action Pipelines (Normalized Entities)
When child entities require their own lifecycle, independent querying, or database foreign keys:

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

*Mechanism*: The generated use case executes two indexed SQL queries (`SELECT ... FROM orders WHERE id = $1` and `SELECT ... FROM order_items WHERE order_id = $1`). Both queries participate in the active transaction context (`connFromContext`).

#### Pattern C: CQRS Read-Model Query Slots (True SQL JOIN)
For complex reporting, multi-table joins, aggregations (`COUNT`, `SUM`), or analytical views:

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

## 2. Custom UUIDs & Swapping Repository Adapters

### The Boundary
Foundation emits `uuid` fields as Go `string`s in `internal/domain` to prevent third-party library lock-in. Default repository adapters use standard library `database/sql`.

### Extension Patterns

#### Swapping to `pgx` or `gorm`
Because all use cases depend on the repository interfaces in `internal/domain`, you can replace the generated `internal/adapters/postgres` adapter with a custom implementation without changing generated use cases or domain models:

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
   	// High-performance binary pgx scanning
   	var p domain.Product
   	err := r.pool.QueryRow(ctx, "SELECT id, title, price, version FROM products WHERE id = $1", id).
   		Scan(&p.ID, &p.Title, &p.Price, &p.Version)
   	if err != nil {
   		return nil, domain.ErrNotFound
   	}
   	return &p, nil
   }
   // Implement Save, Delete, FindMany...
   ```

2. **Wire in `cmd/app/main.go`**:
   ```go
   // Instantiate custom pgx pool
   pool, _ := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
   
   // Inject custom adapter into generated use cases
   productRepo := pgxadapter.NewProductRepository(pool)
   listProductsUC := usecases.NewListProductsUseCase(productRepo)
   ```

#### ID Generation Strategies
- **Client-Provided**: The caller passes `id` in the input payload.
- **Database-Generated**: Configure `DEFAULT gen_random_uuid()` on PostgreSQL primary keys.
- **Slot Generator**:
  ```fdn
  slot IDGenerator {
    newID() -> uuid
  }
  ```
  Implement the slot with `github.com/google/uuid`, `oklog/ulid`, or Snowflake IDs.

---

## 3. Custom HTTP Headers & Context Enrichment

### The Boundary
Foundation's HTTP transport maps route paths, query strings, and JSON request/response bodies. It does not provide direct DSL syntax for reading or writing arbitrary HTTP headers inside an action pipeline.

### Extension Patterns

#### Ingress Request Headers via HTTP Middleware
The generated `httpAdapter.Server` satisfies standard `net/http.Handler`. Wrap it with custom Go middleware in `cmd/app/main.go`:

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

#### Passing Headers into Actions via Actor Claims
If an Action or Policy requires a header value (e.g. `X-Tenant-ID` or `X-Organization-ID`):

1. **Declare the claim on `actor`**:
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
       // Validate bearer token and extract claims...
       return domain.Actor{
           ID: claims.Subject,
           Role: claims.Role,
           TenantID: tenantID,
       }, nil
   }
   ```
3. **Reference in Policies or Pipelines**:
   ```fdn
   policy BelongsToTenant(actor: Actor, order: Order) {
     actor.tenant_id == order.tenant_id
   }
   ```

#### Build-Time Tracing & Header Middleware via Generator Plugins
Rather than manually wrapping HTTP servers in handwritten code, Foundation's pluggable generator pipeline enables build-time middleware injection:

```yaml
targets:
  - name: backend
    language: go
    architecture: hexagonal
    scaffold: builtin
    generators:
      - "foundation-gen-otel"
```

The plugin automatically:
- Emits distributed tracing middleware extracting W3C `traceparent` and `X-Trace-ID` headers.
- Wraps the generated Chi router so every incoming request creates an OpenTelemetry span context with zero manual wiring.
- Preserves standard route handling and error mapping.

---

## 4. Rich Domain Models vs. Action Pipelines (DDD Style)

### The Boundary
In classic Domain-Driven Design, Aggregates protect their invariants via stateful methods (`order.Cancel()`, `order.AddItem(item)`). Foundation purposefully lowers behavior into **Action Pipelines** (Application Use Cases) and keeps domain entities as pure data structures.

### Extension Patterns

#### Declarative Invariant Guards
Encode business invariants directly in the Action Pipeline:

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

#### Complex Domain Services via Slots
For algorithmic validation or business rules that exceed pipeline expressiveness (e.g., scoring algorithms, complex tax calculations):

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
In Go, all files in `internal/domain` share the same package. You can add a handwritten file `internal/domain/order_behavior.go`:

```go
package domain

func (o *Order) CanBeRefunded() bool {
	return o.Status == "delivered" && o.RefundWindowDays <= 30
}
```
*Note*: The generated use case will not call this method automatically, but custom Slots or background workers can share this logic.

#### Automated Rich Aggregate Roots via Generator Plugins
If your domain requires true DDD Aggregate Roots with private invariant constructors, domain validation, and encapsulated mutation methods, use the `foundation-gen-rich-ddd` generator plugin in your build profile:

```yaml
targets:
  - name: backend
    language: go
    architecture: hexagonal
    scaffold: builtin
    generators:
      - "foundation-gen-rich-ddd"
```

When compiled, Foundation:
1. Emits the standard hexagonal scaffolding in memory (`scaffold: builtin`).
2. Runs `foundation-gen-rich-ddd` to transform data entities into rich DDD aggregates with:
   - Invariant constructors (`NewOrder(...)`) enforcing required fields and identifiers.
   - Validation methods (`Validate() error`).
   - Encapsulated setter methods (`SetStatus(...)`) that guard business rules.
3. Automatically writes create-if-missing configuration stubs (`config/ddd.starter.json`) to disk without tracking them in the manifest.

---

## 5. JWT Token Generation & Local Authentication

### The Boundary
Foundation's `authn: jwt-oidc` provides an **Ingress Token Verifier**. It expects an incoming `Authorization: Bearer <token>` and resolves an `Actor`. Foundation does not generate an Identity Provider or Token Issuer, because enterprise backends typically rely on external IdPs (Keycloak, Auth0, Cognito, Supabase).

### Extension Patterns

#### Building a Local Login Action via a Crypto Slot
To issue tokens locally within the Foundation application:

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

## 6. Schema Migrations & Database Evolution

### The Boundary
Foundation generates deterministic DDL (`schema.sql`) reflecting the exact state of all declared entities, primary keys, foreign keys, unique constraints, and `@table` overrides. It does not track migration history tables (`schema_migrations`) or generate automated `ALTER TABLE` diffs.

### Extension Patterns

#### Pairing with External Migration Runners
Integrate Foundation with production migration tools (`goose`, `atlas`, or `golang-migrate`):

1. **Initial Baseline**:
   Use Foundation's `schema.sql` as `migrations/00001_initial_schema.sql`.
2. **Schema Evolution Workflow**:
   When updating `.fdn` entities:
   - Run `foundation build` to inspect the updated `schema.sql`.
   - Author a sequential migration delta with `goose` or `atlas`:
     ```bash
     atlas migrate diff add_phone_to_users --to file://dist/schema.sql --dev "docker://postgres/16/dev"
     ```
   - Execute migrations in your CI/CD deployment pipeline prior to starting the binary.

---

## Quick Reference: Needs vs. Patterns

| Developer Need | Architectural Seam / Recommendation |
|---|---|
| **SQL `JOIN`s / Grouping / Aggregations** | CQRS Read-Model **Slot** (`internal/adapters/postgres`) |
| **Alternative DB Drivers (`pgx`, `gorm`)** | Implement generated `domain.Repository` port in `internal/adapters/` |
| **High-Performance UUIDs (`google/uuid`, ULID)** | Kept as `string` at domain boundary; convert in custom adapter or Slot |
| **Request Headers / Correlation IDs** | Standard Go HTTP middleware or generator plugin (`foundation-gen-otel`) |
| **Distributed Tracing & Context** | Generator plugin (`foundation-gen-otel`) injecting router tracing middleware |
| **Rich DDD Aggregate Roots** | Generator plugin (`foundation-gen-rich-ddd`) enforcing invariants |
| **Custom Language Targets (Python, Rust)** | External generator plugin (`foundation-gen-*`) with `scaffold: none` |
| **Tenant Isolation via Headers** | Extract in `ActorResolver` adapter; bind to `actor` claims |
| **Custom Response Headers / Cookies** | HTTP response interceptor middleware |
| **Token Issuance (`/login`, JWT signing)** | Implement a `TokenService` **Slot** using `bcrypt` and `golang-jwt` |
| **Schema Migration Lifecycle** | Use generated `schema.sql` with `goose`, `golang-migrate`, or `atlas` |
| **Complex Domain Calculations** | Domain Service **Slot** called from the Action Pipeline |
