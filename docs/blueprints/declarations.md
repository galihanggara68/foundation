# Blueprint Declarations & Grammar Reference

Foundation Blueprints (`.fdn`) are organized into 9 top-level declarations that define domain entities, business logic pipelines, and testing specs.

---

## 1. Data Types

### Scalar & Composite Types

| Type | Description | Target Go Type | PostgreSQL Dialect |
| :--- | :--- | :--- | :--- |
| `uuid` | RFC 4122 universally unique identifier | `string` | `UUID` |
| `string` | UTF-8 encoded text string | `string` | `TEXT` |
| `int` | 64-bit signed integer | `int` | `INTEGER` |
| `decimal` / `decimal(P, S)` | Fixed-precision decimal number | `float64` | `NUMERIC(P, S)` (default `10, 2`) |
| `bool` | Boolean (`true` / `false`) | `bool` | `BOOLEAN` |
| `timestamp` | UTC date and time | `time.Time` / `string` | `TIMESTAMPTZ` |
| `<Type>?` | Optional / nullable field | Pointer / optional | Omits `NOT NULL` |
| `[]<Type>` | Homogeneous list | Slice / Array | `JSONB` |
| `Actor` | Caller context (`id`, `role`, `is_verified`, `tenant_id`) | `domain.Actor` | — |

---

## 2. Top-Level Declarations

### 1. `enum`
Exhaustively checked enumeration. Variants are referenced using leading dot syntax (e.g. `.pending`, `.active`).

```fdn
enum OrderStatus {
  pending
  confirmed
  cancelled
}
```

### 2. `value`
Immutable, identity-less data structure used for action parameters, nested structures, or computed responses.

```fdn
value PlaceOrderInput {
  request_id: string
  product_id: uuid
  quantity: int
}
```

### 3. `entity`
Persistent domain model mapped to database tables and repository ports. Supports field annotations:
- `@pk`: Primary key column.
- `@unique`: Unique constraint.
- `@index`: Emits b-tree index.
- `@fk(OtherEntity.id)`: Foreign key reference.
- `@auto_now_add`: Default to current timestamp (`DEFAULT NOW()`).
- `@version`: Integer column used for Optimistic Concurrency Control (OCC).
- `@renamed_from("old_name")`: Safe schema evolution hint for column renames.

```fdn
entity Product {
  id: uuid @pk
  sku: string @unique
  price: decimal(10, 2)
  stock: int = 0
  version: int @version
  created_at: timestamp @auto_now_add
}
```

### 4. `error`
Strongly-typed domain error categorized by a built-in `kind`:
- `not_found`: Missing resource (HTTP 404).
- `validation`: Invalid payload or violated business invariant (HTTP 400).
- `conflict`: Concurrency collision or state conflict (HTTP 409).
- `unauthorized`: Missing authentication (HTTP 401).
- `forbidden`: Insufficient permissions (HTTP 403).
- `internal`: Unrecoverable system failure (HTTP 500).

```fdn
error ProductNotFound {
  kind: not_found
  message: "requested product does not exist"
}
```

### 5. `event`
Published domain event payload emitted when state transitions occur.

```fdn
event OrderPlaced {
  order_id: uuid
  total: decimal(10, 2)
}
```

### 6. `policy`
Side-effect-free boolean expression evaluating authorization rules against an `Actor` or entity fields.

```fdn
policy CanPlaceOrder(actor: Actor) {
  actor.role == "customer" && actor.is_verified == true
}
```

### 7. `slot`
Interface contract separating generated code from handwritten integration code.
- `@external`: Marks methods executing external third-party I/O (cannot be called inside `@transactional`).

```fdn
slot PaymentGateway {
  @external Charge(token: string, amount: int) -> string throws PaymentDeclined
}
```

### 8. `action`
Sequential, guaranteed-terminating business pipeline.

```fdn
action PlaceOrder(input: PlaceOrderInput) -> Order
  @http(method: POST, path: "/v1/orders")
  @transactional
{
  require CanPlaceOrder(actor) else UnauthorizedCustomer
  let product = query one Product where id == input.product_id else ProductNotFound
  guard product.stock >= input.quantity else InsufficientStock

  update Product where id == product.id set {
    stock: product.stock - input.quantity
  }
  let order = create Order {
    product_id: product.id,
    customer_id: actor.id,
    total: product.price * input.quantity
  }
  emit OrderPlaced {
    order_id: order.id,
    total: order.total
  }
  return order
}
```

### 9. `scenario`
Declarative unit test specification compiled into native tests.

```fdn
scenario "Order placement succeeds with valid stock" {
  given Product {
    id: "00000000-0000-0000-0000-000000000001",
    price: 100.00,
    stock: 5
  }
  given actor {
    id: "00000000-0000-0000-0000-000000000099",
    role: "customer",
    is_verified: true
  }
  when PlaceOrder(PlaceOrderInput {
    request_id: "req-1",
    product_id: "00000000-0000-0000-0000-000000000001",
    quantity: 2
  })
  expect result.total == 200.00
  expect stored Product where id == "00000000-0000-0000-0000-000000000001" {
    stock: 3
  }
}
```

---

## 3. Action Pipeline Statements

Statements inside an `action` execute in sequential declaration order:

1. **`require <Policy>(...) else <Error>`**: Evaluates boolean policy, halts and returns error if false.
2. **`guard <bool_expr> else <Error>`**: Invariant check, halts and returns error if false.
3. **`let <var> = <expr>`**: Declares or rebinds a local immutable variable.
4. **`create <Entity> { ... }`**: Persists a new entity instance.
5. **`query one <Entity> where <field> == <expr> else <Error>`**: Finds a single entity by indexed field.
6. **`update <Entity> where <field> == <expr> set { ... }`**: Updates fields on a matching entity.
7. **`delete <Entity> where <field> == <expr>`**: Deletes matching entity rows.
8. **`slot <Slot>.<Method>(...) [else <Error>]`**: Calls custom handwritten slot interface.
9. **`if <bool_expr> { ... } else { ... }`**: Conditional branching.
10. **`match <enum_expr> { .variant => { ... } }`**: Exhaustive pattern match over all enum variants.
11. **`for <var> in <list_expr> { ... }`**: Bounded iteration over collections. (No unbounded loops).
12. **`emit <Event> { ... }`**: Dispatches a domain event.
13. **`return <expr>`**: Terminates pipeline with success payload.

---

## 4. Collection Combinators

Lists support pure functional combinators:

```fdn
let active_items = input.items.filter(item => item.quantity > 0)
let item_totals  = active_items.map(item => item.price * item.quantity)
let grand_total  = item_totals.sum()
let item_count   = active_items.count()
let has_bulky    = active_items.any(item => item.weight > 50)
let all_in_stock = active_items.all(item => item.in_stock == true)
```
