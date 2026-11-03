# HTTP Transport and Parameter Binding Guide

Foundation translates declarative `@http` action traits into HTTP routing adapters (Chi in Go, Express in TypeScript).

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

- `GET`: Resource retrieval. Returns `200 OK`.
- `POST`: Resource creation or command execution. Returns `201 Created`.
- `PUT`: Resource replacement or state updates. Returns `200 OK`.
- `DELETE`: Resource removal. Returns `200 OK`.

---

## 2. URL Path Parameter Binding (`{param}`)

Path segments in curly braces (for example `{id}` or `{account_id}`) define **path parameters**.

### Syntax

```fdn
action UpdateAccount(account_id: uuid, nickname: string) -> Account
  @http(method: PUT, path: "/v1/accounts/{account_id}")
{
  ...
}
```

### Parameter Binding Mechanics

1. **Identifier Matching**: The token inside `{account_id}` must match an action parameter name (`account_id`).
2. **Adapter Lowering**:
   - In Go (Chi): The router extracts parameters with `chi.URLParam(r, "account_id")`.
   - In TypeScript (Express): The router extracts parameters with `req.params["account_id"]`.
3. **Multi-Segment Paths**: Blueprints support multi-segment path hierarchies:
   ```fdn
   @http(method: GET, path: "/v1/tenants/{tenant_id}/users/{user_id}/permissions")
   ```

---

## 3. Request Body Parameter Binding

Mutating requests (`POST`, `PUT`, `DELETE`) bind non-path parameters from JSON request bodies.

### Case A: Single Composite Value Object

When an action defines one non-path parameter of a `value` type:

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

The router maps the JSON body directly to the `input` struct:

```json
{
  "email": "jane@example.com",
  "full_name": "Jane Doe"
}
```

### Case B: Mixed Path and Multiple Body Parameters

When an action combines path parameters with multiple input parameters:

```fdn
action SetPrice(product_id: uuid, price: decimal, reason: string) -> Product
  @http(method: PUT, path: "/v1/products/{product_id}/price")
{
  ...
}
```

The compiler completes these binding steps:
1. Binds `product_id` from the URL path (`/v1/products/00000000-.../price`).
2. Synthesizes a JSON target for remaining parameters (`price`, `reason`):
   ```json
   {
     "price": 49.99,
     "reason": "seasonal discount"
   }
   ```

---

## 4. Caller Context and Actor Header Binding

When actions reference the `actor` keyword, the transport extracts caller identity from HTTP headers:

| HTTP Request Header | Actor Property | Description |
| :--- | :--- | :--- |
| `X-Actor-ID` | `actor.id` (`uuid`) | Unique identifier of authenticated caller |
| `X-Actor-Role` | `actor.role` (`string`) | Caller role (for example `"admin"` or `"customer"`) |
| `X-Actor-Verified` | `actor.is_verified` (`bool`) | Evaluated as `true` when header value is `"true"` |

In Go, the adapter injects actor attributes into `context.Context` using `domain.WithActor(...)`.
In TypeScript, the adapter passes actor properties to use case controllers.

---

## 5. Optimistic Concurrency Control (OCC) Headers

When updating entities with an `@version` field, the HTTP layer manages concurrency headers:

### `ETag` on `GET` Requests

A `GET` endpoint returning a versioned entity emits an `ETag` response header with the quoted version integer:

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

Mutating endpoints (`POST`, `PUT`, `DELETE`) evaluate the incoming `If-Match` header before running the action pipeline:
- `If-Match: "1"`: Matches the expected version. The action proceeds.
- `If-Match: *`: Wildcard match. The action updates the entity regardless of version.
- Mismatch: When the header does not match the expected version, the server returns HTTP 409 Conflict immediately.

---

## 6. HTTP Status Code Mapping

When an action pipeline returns an error, the transport layer inspects `error.kind`:

| Error Kind | HTTP Status Code | Description |
| :--- | :--- | :--- |
| `validation` | `400 Bad Request` | Failed `guard`, invalid parameter format, or domain invariant violation |
| `unauthenticated` | `401 Unauthorized` | Missing or invalid actor credentials |
| `unauthorized` | `401 Unauthorized` | Authentication failure declared by a Blueprint |
| `forbidden` | `403 Forbidden` | Failed `require <Policy>` check |
| `not_found` | `404 Not Found` | Failed `query one ... else <Error>` |
| `conflict` | `409 Conflict` | OCC version mismatch or unique constraint collision |
| `internal` | `500 Internal Server Error` | System error or unhandled runtime failure |

---

## 7. Internal Actions and Event Handlers

The compiler excludes actions without public HTTP endpoints from router tables:
- `@internal`: Private backend actions excluded from public routing tables.
- `@event_handler(EventName)`: In-process event subscribers triggered by domain events rather than HTTP requests.
