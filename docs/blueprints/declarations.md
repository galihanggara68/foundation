# Blueprint Declarations and Grammar Reference

Foundation Blueprints (`.fdn`) contain nine top-level declarations.
These declarations define domain entities, business logic pipelines, and test specifications.

---

## 1. Data Types

### Scalar and Composite Types

| Type | Description | Target Go Type | PostgreSQL Dialect |
| :--- | :--- | :--- | :--- |
| `uuid` | RFC 4122 universally unique identifier | `string` | `UUID` |
| `string` | UTF-8 encoded text string | `string` | `TEXT` |
| `int` | 64-bit signed integer | `int` | `INTEGER` |
| `decimal` / `decimal(P, S)` | Fixed-precision decimal number | `float64` | `NUMERIC(P, S)` (default `10, 2`) |
| `bool` | Boolean (`true` / `false`) | `bool` | `BOOLEAN` |
| `timestamp` | UTC date and time | `time.Time` / `string` | `TIMESTAMPTZ` |
| `<Type>?` | Optional field | Pointer or optional type | Omits `NOT NULL` |
| `[]<Type>` | Homogeneous list | Slice or Array | `JSONB` |
| `Actor` | Caller context (`id`, `role`, `is_verified`, `tenant_id`) | `domain.Actor` | — |

---

## 2. Top-Level Declarations

### 1. `enum`

Enums provide exhaustively checked variants.
Reference variants with leading dot syntax (for example `.pending` or `.active`).

```fdn
enum OrderStatus {
  pending
  confirmed
  cancelled
}
```

### 2. `value`

Value declarations define immutable data structures without identity.
Use value structures for action parameters, nested structures, or computed responses.

```fdn
value PlaceOrderInput {
  request_id: string
  product_id: uuid
  quantity: int
}
```

### 3. `entity`

Entity declarations define persistent domain models.
The compiler maps entities to database tables and repository ports.
Entities support these field annotations:

- `@pk`: Marks the primary key column.
- `@unique`: Enforces a unique constraint.
- `@index`: Emits a b-tree index.
- `@fk(OtherEntity.id)`: Declares a foreign key reference.
- `@auto_now_add`: Defaults to current timestamp (`DEFAULT NOW()`).
- `@version`: Declares an integer column for Optimistic Concurrency Control (OCC).
- `@renamed_from("old_name")`: Records column renames for safe schema migrations.

Entities also support composite annotations at the entity level:

- `@unique(fields: ["col_a", "col_b"], name: "custom_name")`: Enforces a composite unique constraint.
- `@index(fields: ["col_a", "col_b"], name: "custom_name")`: Generates a composite database index.

The compiler generates canonical names when you omit the `name` argument.

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

Error declarations define strongly typed domain errors.
Each error has a built-in `kind`.
The kind sets the default HTTP status and posture (caller text or diagnostic).

- `not_found`: Missing resource (HTTP 404).
- `validation`: Invalid payload or broken business rule (HTTP 400).
- `conflict`: Concurrency collision or state conflict (HTTP 409).
- `unauthenticated`: Missing or invalid actor credentials (HTTP 401).
- `unauthorized`: Authentication failure in a blueprint (HTTP 401).
- `forbidden`: Insufficient permissions (HTTP 403).
- `internal`: System failure (HTTP 500; defaults to `technical` posture).

```fdn
error ProductNotFound {
  kind: not_found
  message: "requested product does not exist"
  messages: {
    en: "requested product does not exist"
    "id-ID": "produk yang diminta tidak ditemukan"
  }
  posture: business
  status: 404
}
```

- `message`: Fallback text when the error carries no locale-specific message.
- `messages`: Text per locale tag (`en`, `id`, `"en-US"`). The compiler does not negotiate locales for callers.
- `status`: HTTP status code from 100 to 599. Set this field to override the default status of the error kind.
- `posture`: Posture is `business` or `technical`. The default is `business` (except `internal`, which defaults to `technical`). Generated transports log technical messages and return generic responses to callers.

The vocabulary reserves each kind name (`NotFound`, `Validation`, `Conflict`, `Unauthenticated`, `Unauthorized`, `Forbidden`, `Internal`).
Declaring an error with a reserved name causes a warning (`FDN0219`).

### 5. `event`

Event declarations define domain payloads.
Pipelines emit events when state transitions occur.

```fdn
event OrderPlaced {
  order_id: uuid
  total: decimal(10, 2)
}
```

### 6. `policy`

Policy declarations evaluate side-effect-free boolean expressions.
Use policies to check authorization rules against an `Actor` or entity fields.

```fdn
policy CanPlaceOrder(actor: Actor) {
  actor.role == "customer" && actor.is_verified == true
}
```

### 7. `slot`

Slot declarations define interfaces between generated code and handwritten logic.
- `@external`: Marks methods that run external third-party I/O.
- The compiler forbids calls to `@external` slots inside `@transactional` actions.
- A trait attaches to the following declaration. Write `@external` on the method. Do not write traits before the `slot` keyword (`FDN0223`).

```fdn
slot PaymentGateway {
  @external Charge(token: string, amount: int) -> string throws PaymentDeclined
}
```

### 8. `action`

Action declarations define sequential business pipelines with guaranteed termination.

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

Scenario declarations define declarative test cases.
The compiler compiles scenarios into native test suites.

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

Statements inside an `action` execute in declaration order:

1. **`require <Policy>(...) else <Error>`**: Evaluates a policy. Returns an error if the policy returns false.
2. **`guard <bool_expr> else <Error>`**: Checks an invariant condition. Returns an error if false.
3. **`let <var> = <expr>`**: Declares or rebinds a local immutable variable.
4. **`create <Entity> { ... }`**: Persists a new entity instance.
5. **`query one <Entity> where <field> == <expr> else <Error>`**: Finds one entity by an indexed field.
6. **`update <Entity> where <field> == <expr> set { ... }`**: Updates fields on a matching entity.
7. **`delete <Entity> where <field> == <expr>`**: Deletes matching entity rows.
8. **`slot <Slot>.<Method>(...) [else <Error>]`**: Calls a handwritten slot interface method.
9. **`if <bool_expr> { ... } else { ... }`**: Runs conditional branches.
10. **`match <enum_expr> { .variant => { ... } }`**: Matches all enum variants exhaustively.
11. **`for <var> in <list_expr> { ... }`**: Iterates through collections. Unbounded loops are forbidden.
12. **`emit <Event> { ... }`**: Dispatches a domain event.
13. **`return <expr>`**: Completes the pipeline with a return value.

---

## 4. Collection Combinators

Lists support pure combinators:

```fdn
let active_items = input.items.filter(item => item.quantity > 0)
let item_totals  = active_items.map(item => item.price * item.quantity)
let grand_total  = item_totals.sum()
let item_count   = active_items.count()
let has_bulky    = active_items.any(item => item.weight > 50)
let all_in_stock = active_items.all(item => item.in_stock == true)
```

---

## 5. Traits and Trait Namespaces

Traits are declarative annotations prefixed with `@`.
Attach traits to entities, fields, actions, or slot methods.

### Standard Traits

The compiler provides built-in standard traits without a namespace prefix:

| Trait | Host | Arguments | Purpose |
|---|---|---|---|
| `@table` | Entity | `name` (or positional `_`) | Maps entity to physical database table |
| `@unique` | Entity, Field | `fields`, `name` (optional) on Entity; none on Field | Enforces unique constraint |
| `@index` | Entity, Field | `fields`, `name` (optional) on Entity; none on Field | Generates database index |
| `@version` | Field | — | Marks field as OCC version column |
| `@pk` | Field | — | Marks field as primary key |
| `@fk` | Field | `target` (or positional `_`) | Marks field as foreign key to another entity |
| `@auto_now_add` | Field | — | Initializes timestamp upon creation |
| `@renamed_from` | Field | `name` (or positional `_`) | Tracks original column name for migrations |
| `@http` | Action | `method`, `path` | Exposes action as an HTTP endpoint |
| `@event_handler` | Action | `event` (or positional `_`) | Subscribes action to a domain event |
| `@transactional` | Action | — | Runs action inside database transaction |
| `@idempotent` | Action | `key` | Enforces client idempotency key |
| `@retry` | Action | `attempts`, `backoff` | Configures retry policy for transient errors |
| `@cache` | Action | `ttl` | Configures caching duration |
| `@internal` | Action | — | Marks action internal to domain service |
| `@outbox` | Action | — | Emits events through transactional outbox |
| `@external` | Slot Method | — | Marks slot method as external I/O |
| `@pure` | Slot Method | — | Marks slot method as side-effect free |

#### Composite Entity Traits

Entities support composite `@unique` and `@index` traits.
Supply a list of field names in the `fields` argument.
You can supply an optional constraint or index name in the `name` argument.
When you omit the `name` argument, the compiler generates a default name.
The default unique constraint name uses the format `uq_<table_name>_<col1>_<col2>`.
The default index name uses the format `idx_<table_name>_<col1>_<col2>`.

```fdn
entity Member @unique(fields: ["organization_id", "email"]) {
  id: uuid @pk
  organization_id: uuid
  email: string
}
```

#### Trait Diagnostics

- **`FDN0224` (Warning)**: An un-namespaced trait is not part of the standard vocabulary.
- **`FDN0226` (Warning)**: An argument problem exists on a standard trait:
  - Unknown argument name.
  - Missing required argument (for example `event` on `@event_handler`).
  - Non-numeric value for numeric arguments (for example `ttl` on `@cache` or `attempts` on `@retry`).

### Namespaced Traits (`@ns:name`)

Third-party generators, linters, or plugins can define domain metadata with namespaced traits:

```fdn
@acme:export(format: "parquet")
entity Order {
  id: uuid @pk
  total: decimal(10, 2) @acme:metric(kind: "gauge")
}
```

- **The Ignore Guarantee**: The compiler does not validate non-reserved namespaced traits. The compiler retains them in AST and IR structures and delivers them to downstream plugins.
- **Reserved Namespace (`FDN0225`)**: The compiler reserves the `foundation` namespace. The compiler rejects `@foundation:...` with error `FDN0225`. Write standard traits without a namespace prefix.
