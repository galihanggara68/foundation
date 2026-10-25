# Slots & Handwritten Boundaries

The Foundation architecture is built around the **90/10 Rule**:
- **90% Generated**: Domain entities, CRUD pipelines, input validation, HTTP transport, database migrations, authentication guards, and durability traits are declared in `.fdn` blueprints and generated deterministically.
- **10% Handcrafted**: External APIs (Stripe, Twilio), proprietary math or ML models, and custom hardware/legacy protocols live in handwritten code.

The boundary between generated code and handcrafted logic is defined declaratively by **Slots**.

---

## 1. Declaring a Slot

A `slot` defines an interface contract within the `.fdn` blueprint:

```fdn
value ChargeResult {
  charge_id: string
  status: string
}

slot PaymentGateway @external {
  Charge(token: string, amount: int) -> ChargeResult
  Refund(charge_id: string, amount: int) -> bool
}

action Checkout(req: CheckoutRequest) -> CheckoutResponse
  @transactional
  @http(method: POST, path: "/checkout")
{
  let order = get Order where id == req.order_id or fail OrderNotFound

  // Call the handcrafted slot
  let charge = PaymentGateway.Charge(req.card_token, order.total)

  order.payment_id = charge.charge_id
  order.status = "PAID"
  save order

  return CheckoutResponse(order_id: order.id, status: order.status)
}
```

---

## 2. The `@external` Attribute & Transaction Safety

When a slot interacts with a third-party service, external database, or network endpoint, it should be annotated with `@external`:

```fdn
slot EmailService @external {
  SendWelcomeEmail(to: string, name: string) -> bool
}
```

### Architectural Guardrails
1. **Network Latency & Connection Pooling**: Calling slow network operations inside a database transaction (`@transactional`) holds connection pool slots open, leading to starvation under load.
2. **Compiler Diagnostics**: The Foundation compiler checks action call graphs. If an `@external` slot is called inside a `@transactional` block, the compiler warns or enforces compensation strategies (such as publishing an event via `@outbox` and handling the external call in an asynchronous worker).

---

## 3. Non-Destructive Starter Stubs

When `foundation build` encounters a newly declared slot, it generates a starter stub:

- **Go Target**: `internal/slots/payment_gateway.go`
- **TypeScript Target**: `src/slots/payment_gateway.ts`

### Manifest Protection
Foundation manages generated files using a build manifest (`.foundation.manifest.json`). 
- Generated files (repositories, routers, models) are tracked with cryptographic hashes and regenerated on every build.
- **Slot implementations are explicitly excluded from the manifest.** Once generated, your custom slot implementation is never overwritten, wiped, or modified by subsequent runs of `foundation build`.

### Example Generated Go Stub
```go
package slots

import (
    "context"
    "your-module/internal/domain"
)

// PaymentGatewayImpl provides handwritten integration for PaymentGateway.
type PaymentGatewayImpl struct {
    // Inject custom HTTP clients, API keys, or logger here
}

func NewPaymentGateway() *PaymentGatewayImpl {
    return &PaymentGatewayImpl{}
}

func (s *PaymentGatewayImpl) Charge(ctx context.Context, token string, amount int64) (domain.ChargeResult, error) {
    // TODO: Implement actual Stripe / Adyen API call
    return domain.ChargeResult{
        ChargeID: "ch_mock_123",
        Status:   "SUCCEEDED",
    }, nil
}
```

---

## 4. Dependency Injection & Wire-up

The generated application entrypoint (`cmd/server/main.go` or `src/server.ts`) instantiates slots and passes them into the dependency injection container:

```go
func main() {
    ctx := context.Background()
    db := initDatabase()

    // Instantiate your handwritten slot implementations
    paymentGateway := slots.NewPaymentGateway()
    emailService := slots.NewEmailService()

    // Pass into the generated container
    container := app.NewContainer(db, paymentGateway, emailService)
    router := transport.NewRouter(container)

    http.ListenAndServe(":8080", router)
}
```

---

## 5. Automated Slot Mocks for Testing

For hermetic testing, Foundation automatically generates mock implementations for every slot in `test/mocks/`:

- **Go**: Generated `MockPaymentGateway` with struct methods to record invocations or stub returns.
- **TypeScript**: Jest/Vitest compatible mock objects.
- **Scenario Testing**: Used automatically by `foundation test` to run blueprint scenarios without live network access.
