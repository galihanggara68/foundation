# Declarative Scenarios and Testing

You can write executable test specifications in `.fdn` blueprints with `scenario` blocks.
These scenarios compile into native Go or TypeScript test suites.
Scenarios run offline without external dependencies.

---

## 1. The Anatomy of a `scenario`

A scenario contains three structured phases: **Given**, **When**, and **Expect**:

```fdn
scenario SuccessfulMoneyTransfer {
  given {
    Wallet(id: "w_alice", balance: 500)
    Wallet(id: "w_bob", balance: 100)
  }
  when {
    TransferFunds(TransferRequest(
      source_id: "w_alice",
      target_id: "w_bob",
      amount: 150
    ))
  }
  expect {
    Wallet(id: "w_alice", balance: 350)
    Wallet(id: "w_bob", balance: 250)
    emitted MoneyTransferred(from: "w_alice", to: "w_bob", amount: 150)
  }
}
```

---

## 2. Testing Failure Paths and Errors

Scenarios can confirm that pipelines abort correctly and return expected domain errors:

```fdn
scenario TransferFailsWhenBalanceIsInsufficient {
  given {
    Wallet(id: "w_alice", balance: 50)
    Wallet(id: "w_bob", balance: 100)
  }
  when {
    TransferFunds(TransferRequest(
      source_id: "w_alice",
      target_id: "w_bob",
      amount: 1000
    ))
  }
  expect {
    fail InsufficientFunds
    // Assert database state remained unchanged (rolled back)
    Wallet(id: "w_alice", balance: 50)
    Wallet(id: "w_bob", balance: 100)
  }
}
```

---

## 3. Scenario Assertions Reference

| Keyword or Pattern | Phase | Description |
|---|---|---|
| `EntityName(...)` | `given` | Seeds an initial record into the test database. |
| `ActionName(args...)` | `when` | Dispatches an action with specified arguments. |
| `EventName(payload...)` | `when` | Dispatches an asynchronous event to trigger an event handler. |
| `EntityName(expected...)` | `expect` | Asserts that an entity record matches expected field values. |
| `fail ErrorName` | `expect` | Asserts that the action returned the specified error. |
| `emitted EventName(...)` | `expect` | Asserts that the action emitted a domain event. |
| `result == Value` | `expect` | Asserts the return value of the action. |

---

## 4. Slot Mocking in Scenarios

When an action calls a `slot`, scenarios use generated test doubles:

```fdn
slot FraudDetection {
  @external Evaluate(user_id: string, amount: int) -> bool
}

scenario HighRiskTransferBlocked {
  given {
    Wallet(id: "w_user", balance: 5000)
    // Stub slot return value
    mock FraudDetection.Evaluate("w_user", 4000) -> true
  }
  when {
    TransferFunds(TransferRequest(source_id: "w_user", target_id: "w_ext", amount: 4000))
  }
  expect {
    fail HighRiskTransactionBlocked
    Wallet(id: "w_user", balance: 5000)
  }
}
```

---

## 5. Offline and Hermetic Execution

Scenarios compile into standard test code in your target language:

```bash
# Run blueprint scenario tests through Foundation CLI
foundation test

# Or run directly with native tooling
go test -v ./test/scenarios/...
# In TypeScript:
npm test
```

### Key Guarantees

- **Hermeticity**: Tests require no network connection, external microservices, or third-party API keys.
- **In-Memory Storage**: Tests run against an ephemeral in-memory repository unless configured to use a PostgreSQL container.
- **Fast Runs**: Full scenario suites run in milliseconds.
