package runtime_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/galihanggara68/foundation/contrib/go/runtime"
)

type fakeConn struct {
	mu          sync.Mutex
	queries     []string
	idempotency map[string]idempotencyRecord
	outbox      []outboxRecord
}

type idempotencyRecord struct {
	status    string
	payload   []byte
	expiresAt time.Time
}

type outboxRecord struct {
	id        string
	topic     string
	eventName string
	payload   []byte
	createdAt time.Time
	published bool
}

type mockDriver struct {
	conn *fakeConn
}

func (d *mockDriver) Open(name string) (driver.Conn, error) {
	return d.conn, nil
}

func (c *fakeConn) Prepare(query string) (driver.Stmt, error) {
	return &fakeStmt{conn: c, query: query}, nil
}

func (c *fakeConn) Close() error { return nil }
func (c *fakeConn) Begin() (driver.Tx, error) {
	return &fakeTx{conn: c}, nil
}

type fakeTx struct {
	conn *fakeConn
}

func (t *fakeTx) Commit() error   { return nil }
func (t *fakeTx) Rollback() error { return nil }

type fakeStmt struct {
	conn  *fakeConn
	query string
}

func (s *fakeStmt) Close() error  { return nil }
func (s *fakeStmt) NumInput() int { return -1 }

func (s *fakeStmt) Exec(args []driver.Value) (driver.Result, error) {
	s.conn.mu.Lock()
	defer s.conn.mu.Unlock()
	s.conn.queries = append(s.conn.queries, s.query)

	if len(args) == 2 && s.query[:6] == "INSERT" {
		// Idempotency lock acquire: key, expires
		key := fmt.Sprintf("%v", args[0])
		expires, _ := args[1].(time.Time)
		if cur, ok := s.conn.idempotency[key]; ok && cur.status == "started" && cur.expiresAt.After(time.Now()) {
			return driver.RowsAffected(0), nil
		}
		s.conn.idempotency[key] = idempotencyRecord{
			status:    "started",
			expiresAt: expires,
		}
		return driver.RowsAffected(1), nil
	}

	if s.query[:6] == "DELETE" && len(args) == 1 {
		key := fmt.Sprintf("%v", args[0])
		delete(s.conn.idempotency, key)
		return driver.RowsAffected(1), nil
	}

	if len(args) >= 3 && (s.query[:6] == "INSERT" || s.query[:6] == "UPDATE") {
		// Mock outbox / idempotency upsert
		if len(args) == 5 {
			// Outbox insert: id, topic, event_name, payload, created_at
			id := fmt.Sprintf("%v", args[0])
			topic := fmt.Sprintf("%v", args[1])
			evName := fmt.Sprintf("%v", args[2])
			payload, _ := args[3].([]byte)
			cAt, _ := args[4].(time.Time)
			s.conn.outbox = append(s.conn.outbox, outboxRecord{
				id:        id,
				topic:     topic,
				eventName: evName,
				payload:   payload,
				createdAt: cAt,
				published: false,
			})
			return driver.RowsAffected(1), nil
		}
		if len(args) == 3 {
			// Idempotency upsert: key, responsePayload, expires
			key := fmt.Sprintf("%v", args[0])
			payload, _ := args[1].([]byte)
			expires, _ := args[2].(time.Time)
			s.conn.idempotency[key] = idempotencyRecord{
				status:    "completed",
				payload:   payload,
				expiresAt: expires,
			}
			return driver.RowsAffected(1), nil
		}
	}
	if len(args) == 1 && s.query[:6] == "UPDATE" {
		// Mark outbox published
		id := fmt.Sprintf("%v", args[0])
		for i := range s.conn.outbox {
			if s.conn.outbox[i].id == id {
				s.conn.outbox[i].published = true
			}
		}
		return driver.RowsAffected(1), nil
	}
	return driver.RowsAffected(1), nil
}

func (s *fakeStmt) Query(args []driver.Value) (driver.Rows, error) {
	s.conn.mu.Lock()
	defer s.conn.mu.Unlock()
	s.conn.queries = append(s.conn.queries, s.query)

	if len(args) == 1 {
		// Query idempotency
		key := fmt.Sprintf("%v", args[0])
		rec, ok := s.conn.idempotency[key]
		if !ok {
			return &fakeRows{cols: []string{"status", "response_payload", "expires_at"}, rows: nil}, nil
		}
		return &fakeRows{
			cols: []string{"status", "response_payload", "expires_at"},
			rows: [][]driver.Value{{rec.status, rec.payload, rec.expiresAt}},
		}, nil
	}

	// Query outbox
	var rows [][]driver.Value
	for _, r := range s.conn.outbox {
		if !r.published {
			rows = append(rows, []driver.Value{r.id, r.topic, r.eventName, r.payload, r.createdAt})
		}
	}
	return &fakeRows{
		cols: []string{"id", "topic", "event_name", "payload", "created_at"},
		rows: rows,
	}, nil
}

type fakeRows struct {
	cols []string
	rows [][]driver.Value
	idx  int
}

func (r *fakeRows) Columns() []string { return r.cols }
func (r *fakeRows) Close() error      { return nil }
func (r *fakeRows) Next(dest []driver.Value) error {
	if r.idx >= len(r.rows) {
		return io.EOF
	}
	for i, v := range r.rows[r.idx] {
		dest[i] = v
	}
	r.idx++
	return nil
}

var (
	driverOnce sync.Once
	testConn   = &fakeConn{
		idempotency: make(map[string]idempotencyRecord),
	}
)

func initMockDB(t *testing.T) *sql.DB {
	t.Helper()
	driverOnce.Do(func() {
		sql.Register("test_postgres", &mockDriver{conn: testConn})
	})
	db, err := sql.Open("test_postgres", "")
	if err != nil {
		t.Fatalf("failed to open mock db: %v", err)
	}
	return db
}

func TestPostgresDriver_IdempotencyLifecycle(t *testing.T) {
	db := initMockDB(t)
	drv := runtime.NewPostgresDriver(db)
	store := drv.Idempotency()

	ctx := context.Background()
	key := "req-abc"

	status, payload, unlock, err := store.GetOrLock(ctx, key, 5*time.Minute)
	if err != nil {
		t.Fatalf("unexpected GetOrLock error: %v", err)
	}
	if status != runtime.IdempotencyStatusStarted {
		t.Fatalf("expected started status, got %s", status)
	}
	if len(payload) != 0 {
		t.Fatalf("expected empty payload, got %s", string(payload))
	}

	// Second concurrent call should fail lock
	_, _, _, lockErr := store.GetOrLock(ctx, key, 5*time.Minute)
	if lockErr == nil {
		t.Fatalf("expected concurrent GetOrLock to fail")
	}

	// Unlock with completion
	if err := unlock(ctx, true, []byte(`{"result":"ok"}`)); err != nil {
		t.Fatalf("unlock failed: %v", err)
	}

	// Re-query should return completed with payload
	st2, p2, _, err2 := store.GetOrLock(ctx, key, 5*time.Minute)
	if err2 != nil {
		t.Fatalf("second GetOrLock failed: %v", err2)
	}
	if st2 != runtime.IdempotencyStatusCompleted {
		t.Fatalf("expected completed status, got %s", st2)
	}
	if string(p2) != `{"result":"ok"}` {
		t.Fatalf("unexpected payload: %s", string(p2))
	}
}

func TestPostgresDriver_OutboxAndRelayWorker(t *testing.T) {
	db := initMockDB(t)
	drv := runtime.NewPostgresDriver(db)
	store := drv.Outbox()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	ev := runtime.OutboxEvent{
		ID:        "ev-1",
		Topic:     "OrderCreated",
		EventName: "OrderCreated",
		Payload:   []byte(`{"order_id":"123"}`),
		CreatedAt: time.Now(),
	}

	if err := store.Stage(ctx, ev); err != nil {
		t.Fatalf("Stage failed: %v", err)
	}

	var relayed []runtime.OutboxEvent
	var relayMu sync.Mutex
	worker := runtime.NewOutboxRelayWorker(store, func(pCtx context.Context, e runtime.OutboxEvent) error {
		relayMu.Lock()
		defer relayMu.Unlock()
		relayed = append(relayed, e)
		return nil
	}, 20*time.Millisecond)

	worker.Start(ctx)
	defer worker.Stop()

	// Wait for worker to poll and publish
	time.Sleep(100 * time.Millisecond)

	relayMu.Lock()
	count := len(relayed)
	relayMu.Unlock()

	if count != 1 {
		t.Fatalf("expected 1 relayed event, got %d", count)
	}

	// Second poll should see 0 new events because event was marked published
	if err := worker.PollOnce(ctx); err != nil {
		t.Fatalf("PollOnce failed: %v", err)
	}
	relayMu.Lock()
	countAfter := len(relayed)
	relayMu.Unlock()
	if countAfter != 1 {
		t.Fatalf("expected still 1 relayed event, got %d", countAfter)
	}
}

func TestTxContextHelpers(t *testing.T) {
	ctx := context.Background()
	if runtime.TxFromContext(ctx) != nil {
		t.Fatalf("expected nil tx from empty ctx")
	}

	dummyTx := "dummy-tx"
	txCtx := runtime.ContextWithTx(ctx, dummyTx)
	if val := runtime.TxFromContext(txCtx); val != dummyTx {
		t.Fatalf("expected dummyTx, got %v", val)
	}
}
