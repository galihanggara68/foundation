package compiler_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/lib/pq"

	"github.com/galihanggara68/foundation/contrib/go/runtime"
	"github.com/galihanggara68/foundation/pkg/compiler"
)

func getTestPostgresDB(t *testing.T) (*sql.DB, string) {
	t.Helper()
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://myuser:mysecurepassword@localhost:5432/mydatabase?sslmode=disable"
	}
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		t.Skipf("skipping live postgres test (connection error: %v)", err)
	}
	if err := db.Ping(); err != nil {
		t.Skipf("skipping live postgres test (ping error: %v)", err)
	}
	return db, dbURL
}

func TestBuild_PostgresDurability_AtomicityIdempotencyAndOutboxRelay(t *testing.T) {
	db, dbURL := getTestPostgresDB(t)
	defer db.Close()

	dir := t.TempDir()

	blueprint := `
value TransferInput {
  request_id: string
  source_id: uuid
  amount: int
}

entity Wallet {
  id: uuid @pk
  balance: int
}

event MoneyDebited {
  wallet_id: uuid
  amount: int
}

error WalletNotFound {
  kind: not_found
  message: "wallet not found"
}

error InsufficientFunds {
  kind: validation
  message: "insufficient balance"
}

action TransferFunds(input: TransferInput) -> Wallet
  @http(method: POST, path: "/wallets/transfer")
  @idempotent(key: input.request_id)
  @transactional
  @outbox
{
  let w = query one Wallet where id == input.source_id else WalletNotFound
  guard w.balance >= input.amount else InsufficientFunds
  update Wallet where id == w.id set {
    balance: w.balance - input.amount
  }
  emit MoneyDebited {
    wallet_id: w.id,
    amount: input.amount
  }
  let updated = query one Wallet where id == w.id else WalletNotFound
  return updated
}
`

	buildProfile := `
project:
  name: wallet
  module: example.com/wallet

storage:
  dialect: postgres
  mode: both

targets:
  - name: server
    kind: backend
    language: go
    architecture: hexagonal
    http: chi
`

	if err := os.WriteFile(filepath.Join(dir, "wallet.fdn"), []byte(blueprint), 0644); err != nil {
		t.Fatalf("failed to write wallet.fdn: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "foundation.build.yaml"), []byte(buildProfile), 0644); err != nil {
		t.Fatalf("failed to write build profile: %v", err)
	}

	_, diags := compiler.Build(dir, compiler.BuildOptions{})
	if len(diags) > 0 {
		t.Fatalf("build diagnostics: %+v", diags)
	}

	// 1. Reset tables and apply generated schema.sql
	schemaSQL, err := os.ReadFile(filepath.Join(dir, "schema.sql"))
	if err != nil {
		t.Fatalf("read schema.sql: %v", err)
	}
	_, _ = db.Exec(`
DROP TABLE IF EXISTS outbox_events CASCADE;
DROP TABLE IF EXISTS idempotency_keys CASCADE;
DROP TABLE IF EXISTS wallet CASCADE;
`)
	if _, err := db.Exec(string(schemaSQL)); err != nil {
		t.Fatalf("failed to apply generated schema.sql: %v", err)
	}

	// Seed initial wallet: id = 11111111-1111-1111-1111-111111111111, balance = 500
	walletID := "11111111-1111-1111-1111-111111111111"
	if _, err := db.Exec("INSERT INTO wallet (id, balance) VALUES ($1, $2)", walletID, 500); err != nil {
		t.Fatalf("seed wallet: %v", err)
	}

	// 2. Build server binary
	binPath := filepath.Join(dir, "server-durability-bin")
	buildCmd := exec.Command("go", "build", "-o", binPath, "./cmd/server")
	buildCmd.Dir = dir
	buildCmd.Env = append(os.Environ(), "GOPROXY=off")
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("go build failed: %v\noutput:\n%s", err, string(out))
	}

	port := "19888"
	srvCmd := exec.Command(binPath)
	srvCmd.Dir = dir
	srvCmd.Env = append(os.Environ(), "PORT="+port, "DATABASE_URL="+dbURL)
	var srvBuf bytes.Buffer
	srvCmd.Stdout = &srvBuf
	srvCmd.Stderr = &srvBuf
	if err := srvCmd.Start(); err != nil {
		t.Fatalf("start server: %v", err)
	}
	defer func() {
		_ = srvCmd.Process.Kill()
	}()

	baseURL := "http://localhost:" + port
	client := &http.Client{Timeout: 5 * time.Second}

	// Wait for server readiness
	ready := false
	for i := 0; i < 30; i++ {
		time.Sleep(100 * time.Millisecond)
		resp, err := client.Post(baseURL+"/wallets/transfer", "application/json", strings.NewReader("{}"))
		if err == nil {
			resp.Body.Close()
			ready = true
			break
		}
	}
	if !ready {
		t.Fatalf("server failed to become ready; server log:\n%s", srvBuf.String())
	}

	// 3. Atomicity on Rollback: Transfer amount exceeding balance (700 > 500)
	failReq := fmt.Sprintf(`{"request_id":"req-fail-1","source_id":%q,"amount":700}`, walletID)
	respFail, err := client.Post(baseURL+"/wallets/transfer", "application/json", strings.NewReader(failReq))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	respFail.Body.Close()
	if respFail.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request on guard failure, got %d", respFail.StatusCode)
	}

	// Verify rollback atomicity in PostgreSQL: wallet balance still 500, no outbox, no idempotency
	var balance int
	if err := db.QueryRow("SELECT balance FROM wallet WHERE id = $1", walletID).Scan(&balance); err != nil {
		t.Fatalf("query wallet: %v", err)
	}
	if balance != 500 {
		t.Fatalf("expected wallet balance 500 after rollback, got %d", balance)
	}

	var idemCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM idempotency_keys WHERE key = 'req-fail-1'").Scan(&idemCount); err != nil {
		t.Fatalf("query idempotency_keys: %v", err)
	}
	if idemCount != 0 {
		t.Fatalf("expected 0 idempotency rows after rollback, got %d", idemCount)
	}

	var outboxCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM outbox_events").Scan(&outboxCount); err != nil {
		t.Fatalf("query outbox_events: %v", err)
	}
	if outboxCount != 0 {
		t.Fatalf("expected 0 outbox events after rollback, got %d", outboxCount)
	}

	// 4. Atomicity on Commit: Valid transfer (amount = 200)
	successReq := fmt.Sprintf(`{"request_id":"req-succ-1","source_id":%q,"amount":200}`, walletID)
	respSuccess, err := client.Post(baseURL+"/wallets/transfer", "application/json", strings.NewReader(successReq))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	bodySuccess, _ := io.ReadAll(respSuccess.Body)
	respSuccess.Body.Close()
	if respSuccess.StatusCode != http.StatusOK && respSuccess.StatusCode != http.StatusCreated {
		t.Fatalf("expected success status, got %d: %s", respSuccess.StatusCode, string(bodySuccess))
	}

	// Verify wallet balance is 300
	if err := db.QueryRow("SELECT balance FROM wallet WHERE id = $1", walletID).Scan(&balance); err != nil {
		t.Fatalf("query wallet: %v", err)
	}
	if balance != 300 {
		t.Fatalf("expected wallet balance 300 after commit, got %d", balance)
	}

	// Verify idempotency record exists with status 'completed'
	var idemStatus string
	var idemPayload []byte
	if err := db.QueryRow("SELECT status, response_payload FROM idempotency_keys WHERE key = 'req-succ-1'").Scan(&idemStatus, &idemPayload); err != nil {
		t.Fatalf("query idempotency_keys: %v", err)
	}
	if idemStatus != "completed" {
		t.Fatalf("expected completed status, got %s", idemStatus)
	}
	var resBody map[string]any
	_ = json.Unmarshal(idemPayload, &resBody)
	if int(resBody["balance"].(float64)) != 300 {
		t.Fatalf("expected cached payload balance 300, got %v", resBody["balance"])
	}

	// Verify outbox event exists in outbox_events table
	var outboxTopic string
	var outboxPublished bool
	if err := db.QueryRow("SELECT topic, published FROM outbox_events WHERE topic = 'MoneyDebited'").Scan(&outboxTopic, &outboxPublished); err != nil {
		t.Fatalf("query outbox_events: %v", err)
	}
	if outboxTopic != "MoneyDebited" {
		t.Fatalf("expected outbox event topic MoneyDebited, got %s", outboxTopic)
	}

	// 5. Idempotency Deduplication: Repeat request with req-succ-1
	respDup, err := client.Post(baseURL+"/wallets/transfer", "application/json", strings.NewReader(successReq))
	if err != nil {
		t.Fatalf("duplicate request failed: %v", err)
	}
	bodyDup, _ := io.ReadAll(respDup.Body)
	respDup.Body.Close()
	if !bytes.Equal(bytes.TrimSpace(bodySuccess), bytes.TrimSpace(bodyDup)) {
		t.Fatalf("expected identical cached response on duplicate request, got:\n%s vs\n%s", string(bodySuccess), string(bodyDup))
	}

	// Balance should STILL be 300 (not 100!)
	if err := db.QueryRow("SELECT balance FROM wallet WHERE id = $1", walletID).Scan(&balance); err != nil {
		t.Fatalf("query wallet: %v", err)
	}
	if balance != 300 {
		t.Fatalf("expected wallet balance still 300 on duplicate, but was deducted to %d", balance)
	}

	// 6. Outbox Relay & Crash Recovery (simulate process crash before event was published)
	outboxStore := runtime.NewPostgresOutboxStore(db)
	simulatedEventID := "crash-recovery-ev-1"
	if _, err := db.Exec(`INSERT INTO outbox_events (id, topic, event_name, payload, created_at, published)
VALUES ($1, 'CrashRecovered', 'CrashRecovered', $2, NOW(), FALSE)`, simulatedEventID, []byte(`{"recovered":true}`)); err != nil {
		t.Fatalf("insert simulated crash event: %v", err)
	}

	var relayedEvents []runtime.OutboxEvent
	var relayMu sync.Mutex
	restartedWorker := runtime.NewOutboxRelayWorker(outboxStore, func(ctx context.Context, ev runtime.OutboxEvent) error {
		relayMu.Lock()
		defer relayMu.Unlock()
		relayedEvents = append(relayedEvents, ev)
		return nil
	}, 50*time.Millisecond)

	if err := restartedWorker.PollOnce(context.Background()); err != nil {
		t.Fatalf("restarted worker PollOnce failed: %v", err)
	}

	relayMu.Lock()
	totalRelayed := len(relayedEvents)
	relayMu.Unlock()
	if totalRelayed != 2 {
		t.Fatalf("expected 2 relayed events after crash recovery, got %d", totalRelayed)
	}

	// Verify simulated event is now published in DB
	var recoveredPublished bool
	if err := db.QueryRow("SELECT published FROM outbox_events WHERE id = $1", simulatedEventID).Scan(&recoveredPublished); err != nil {
		t.Fatalf("query recovered outbox: %v", err)
	}
	if !recoveredPublished {
		t.Fatalf("expected recovered event to be marked published in DB")
	}
}
