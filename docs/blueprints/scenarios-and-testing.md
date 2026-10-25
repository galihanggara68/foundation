# Declarative Scenarios & Testing

Foundation allows you to write executable specification tests directly within your `.fdn` blueprints using `scenario` blocks. These scenarios compile into native Go (`go test`) or TypeScript test suites that execute 100% offline with zero external dependencies.

---

## 1. The Anatomy of a `scenario`

A scenario is composed of three structured phases: **Given**, **When**, and **Expect**:

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

## 2. Testing Failure Paths & Errors

Scenarios can verify that pipelines abort correctly and return expected domain errors:

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

| Keyword / Pattern | Phase | Description |
|---|---|---|
| `EntityName(...)` | `given` | Seeds an initial record into the test database. |
| `ActionName(args...)` | `when` | Dispatches an action with specified arguments. |
| `EventName(payload...)` | `when` | Dispatches an asynchronous event to trigger an event handler. |
| `EntityName(expected...)` | `expect` | Asserts that an entity record matches the expected field values. |
| `fail ErrorName` | `expect` | Asserts that the action failed with the designated error. |
| `emitted EventName(...)` | `expect` | Asserts that a domain event was published to the outbox/bus. |
| `result == Value` | `expect` | Asserts the returned value of the action. |

---

## 4. Slot Mocking in Scenarios

When an action interacts with a `slot`, scenarios use generated test doubles:

```fdn
slot FraudDetection @external {
  Evaluate(user_id: string, amount: int) -> bool
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

## 5. Offline & Hermetic Execution

Scenarios compile into standard idiomatic test code in your target language:

```bash
# Run blueprint scenario tests via the Foundation CLI
foundation test

# Or run directly via native tooling
go test -v ./test/scenarios/...
# In TypeScript:
npm test
```

### Key Guarantees
- **Hermeticity**: Tests require no internet connection, external microservices, or external third-party API credentials.
- **In-Memory Storage**: Tests automatically execute against an ephemeral in-memory SQLite or mock repository instance unless configured to run against a test PostgreSQL container.
- **Fast Execution**: Entire scenario suites execute in milliseconds.
