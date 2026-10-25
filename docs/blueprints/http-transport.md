# HTTP Transport & Parameter Binding Guide

Foundation compilers translate declarative `@http` action traits into idiomatic HTTP routing adapters (Chi in Go, Express in TypeScript).

---

## 1. Defining HTTP Endpoints (`@http`)

Actions exposed over HTTP declare the `@http` trait:

```fdn
action GetOrder(id: uuid) -> Order
  @http(method: GET, path: "/v1/orders/{id}")
{
  let o = query one Order where id == id else OrderNotFound
  return o
}
```

### Supported HTTP Methods
- `GET`: Idempotent resource retrieval. Returns `200 OK`.
- `POST`: Resource creation or command execution. Returns `201 Created`.
- `PUT`: Resource replacement or state updates. Returns `200 OK`.
- `DELETE`: Resource removal. Returns `200 OK`.

---

## 2. URL Path Parameter Binding (`{param}`)

Path segments wrapped in curly braces (e.g. `{id}`, `{account_id}`) are parsed as **path parameters**.

### Syntax
```fdn
action UpdateAccount(account_id: uuid, nickname: string) -> Account
  @http(method: PUT, path: "/v1/accounts/{account_id}")
{
  ...
}
```

### Binding Mechanics
1. **Identifier Matching**: The token inside `{account_id}` must match an Action parameter name (`account_id`).
2. **Adapter Lowering**:
   - In Go (Chi): Extracted via `chi.URLParam(r, "account_id")`.
   - In TypeScript (Express): Extracted via `req.params["account_id"]`.
3. **Multi-Segment Paths**: Blueprints support arbitrary path hierarchies:
   ```fdn
   @http(method: GET, path: "/v1/tenants/{tenant_id}/users/{user_id}/permissions")
   ```

---

## 3. Request Body Parameter Binding

Non-path parameters on mutating requests (`POST`, `PUT`, `DELETE`) are bound from the incoming JSON request body.

### Case A: Single Composite Value Object
When an action defines a single non-path parameter of a `value` type:

```fdn
value CreateCustomerInput {
  email: string
  full_name: string
}

action CreateCustomer(input: CreateCustomerInput) -> Customer
  @http(method: POST, path: "/v1/customers")
{
  ...
}
```

The entire JSON body maps directly to the `input` struct:
```json
{
  "email": "jane@example.com",
  "full_name": "Jane Doe"
}
```

### Case B: Mixed Path & Multiple Body Parameters
When an action combines path parameters with primitive or multiple parameters:

```fdn
action SetPrice(product_id: uuid, price: decimal, reason: string) -> Product
  @http(method: PUT, path: "/v1/products/{product_id}/price")
{
  ...
}
```

The compiler:
1. Binds `product_id` from the URL path (`/v1/products/00000000-.../price`).
2. Synthesizes an unmarshaling target for the remaining non-path parameters (`price`, `reason`):
   ```json
   {
     "price": 49.99,
     "reason": "seasonal discount"
   }
   ```

---

## 4. Caller Context & Actor Header Binding

When actions reference the built-in `actor` keyword, caller identity is extracted from standard transport headers:

| HTTP Request Header | Actor Property | Description |
| :--- | :--- | :--- |
| `X-Actor-ID` | `actor.id` (`uuid`) | Unique identifier of the authenticated caller |
| `X-Actor-Role` | `actor.role` (`string`) | Caller role (e.g. `"admin"`, `"customer"`) |
| `X-Actor-Verified` | `actor.is_verified` (`bool`) | Evaluated as `true` when header value is `"true"` |

In Go, actor attributes are injected into `context.Context` via `domain.WithActor(...)`. In TypeScript, actor properties are passed as an argument to use-case controllers.

---

## 5. Optimistic Concurrency Control (OCC) Headers

When updating entities with an `@version` field, the HTTP layer automatically manages HTTP concurrency headers:

### `ETag` on `GET` Requests
Any `GET` endpoint returning a versioned entity emits an `ETag` response header with the quoted version integer:
```http
HTTP/1.1 200 OK
Content-Type: application/json
ETag: "1"

{
  "id": "11111111-1111-1111-1111-111111111111",
  "version": 1,
  "total": 500
}
```

### `If-Match` on Mutating Requests
Mutating endpoints (`POST`, `PUT`, `DELETE`) with an expected version parameter evaluate the incoming `If-Match` request header before executing the action pipeline:
- `If-Match: "1"`: Verified against expected version. If matching, the action proceeds.
- `If-Match: *`: Wildcard match; updates regardless of version.
- Mismatch: If header does not match expected version, returns **HTTP 409 Conflict** immediately without executing business logic or holding locks.

---

## 6. HTTP Status Code Mapping

When an Action pipeline returns an error, the transport layer inspects the declared `error.kind`:

| Error Kind | HTTP Status Code | Description |
| :--- | :--- | :--- |
| `validation` | `400 Bad Request` | Failed `guard`, invalid parameter format, or domain invariant violation |
| `unauthorized` | `401 Unauthorized` | Missing authentication credentials |
| `forbidden` | `403 Forbidden` | Failed `require <Policy>` check |
| `not_found` | `404 Not Found` | Failed `query one ... else <Error>` |
| `conflict` | `409 Conflict` | OCC version mismatch or unique constraint collision |
| `internal` | `500 Internal Server Error` | Unhandled runtime failure or hardware fault |

---

## 7. Internal Actions & Event Handlers

Actions without public HTTP endpoints are excluded from routing:
- `@internal`: Private backend actions excluded from public routing tables.
- `@event_handler(EventName)`: In-process event subscribers triggered asynchronously by domain events rather than external HTTP requests.
