# Slots and Handwritten Boundaries

The Foundation architecture uses the **90/10 Rule**:
- **90 Percent Generated**: Domain entities, CRUD pipelines, input validation, HTTP transport, database migrations, authentication guards, and durability traits.
- **10 Percent Handcrafted**: External APIs (Stripe, Twilio), proprietary algorithms, and custom hardware protocols.

**Slots** define the boundary between generated code and handcrafted logic.

---

## 1. Declaring a Slot

A `slot` defines an interface contract within the `.fdn` blueprint:

```fdn
value ChargeResult {
  charge_id: string
  status: string
}

slot PaymentGateway {
  @external Charge(token: string, amount: int) -> ChargeResult
  @external Refund(charge_id: string, amount: int) -> bool
}

action Checkout(req: CheckoutRequest) -> CheckoutResponse
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

## 2. The `@external` Attribute and Transaction Safety

When a slot interacts with a third-party service, external database, or network endpoint, annotate methods with `@external`:

```fdn
slot EmailService {
  @external SendWelcomeEmail(to: string, name: string) -> bool
}
```

### Architectural Guardrails

1. **Network Latency and Connection Pools**: Slow network calls inside a database transaction (`@transactional`) hold database connections open. This pattern exhausts connection pools under load.
2. **Compiler Diagnostics**: The Foundation compiler checks action call graphs. If an action calls an `@external` slot inside `@transactional`, the compiler warns or enforces compensation strategies (such as publishing an event through `@outbox`).

---

## 3. Non-Destructive Starter Stubs

When `foundation build` encounters a newly declared slot, the compiler generates a starter stub:

- **Go Target**: `internal/slots/payment_gateway.go`
- **TypeScript Target**: `src/slots/payment_gateway.ts`

### Manifest Protection

Foundation manages generated files with a build manifest (`.foundation-manifest.json`).
- The compiler tracks generated files (repositories, routers, models) with cryptographic hashes.
- The compiler regenerates tracked files on every build.
- **The manifest excludes slot implementations.** Once generated, builds never overwrite or delete custom slot files.

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

## 4. Dependency Injection and Wiring

The generated application entrypoint (`cmd/server/main.go` or `src/server.ts`) instantiates slots and passes them to the container:

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

Foundation generates mock implementations for every slot in `test/mocks/`:

- **Go**: The compiler generates `MockPaymentGateway` to record calls or stub return values.
- **TypeScript**: The compiler generates mock objects compatible with Jest and Vitest.
- **Scenario Testing**: The test runner uses mocks automatically to run blueprint scenarios without network access.
