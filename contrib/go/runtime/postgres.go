package runtime

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// DBTX is the common interface implemented by *sql.DB and *sql.Tx.
type DBTX interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func resolveDBTX(ctx context.Context, defaultDB *sql.DB, explicitTx any) DBTX {
	if explicitTx != nil {
		if dbtx, ok := explicitTx.(DBTX); ok {
			return dbtx
		}
	}
	if t := TxFromContext(ctx); t != nil {
		if dbtx, ok := t.(DBTX); ok {
			return dbtx
		}
	}
	return defaultDB
}

// PostgresDriver provides PostgreSQL-backed implementations of runtime stores.
type PostgresDriver struct {
	db     *sql.DB
	idem   IdempotencyStore
	outbox OutboxStore
	cache  CacheStore
}

// NewPostgresDriver creates a PostgresDriver for the given database connection pool.
func NewPostgresDriver(db *sql.DB) *PostgresDriver {
	return &PostgresDriver{
		db:     db,
		idem:   NewPostgresIdempotencyStore(db),
		outbox: NewPostgresOutboxStore(db),
		cache:  NewMemoryCacheStore(),
	}
}

func (d *PostgresDriver) Idempotency() IdempotencyStore { return d.idem }
func (d *PostgresDriver) Outbox() OutboxStore           { return d.outbox }
func (d *PostgresDriver) Cache() CacheStore             { return d.cache }

// PostgresIdempotencyStore manages idempotency records in PostgreSQL with distributed locking.
type PostgresIdempotencyStore struct {
	db *sql.DB
}

// NewPostgresIdempotencyStore returns a new PostgresIdempotencyStore.
func NewPostgresIdempotencyStore(db *sql.DB) *PostgresIdempotencyStore {
	return &PostgresIdempotencyStore{db: db}
}

func (s *PostgresIdempotencyStore) GetOrLock(ctx context.Context, key string, ttl time.Duration) (IdempotencyStatus, []byte, UnlockFunc, error) {
	now := time.Now()
	expires := now.Add(ttl)

	var status string
	var payload []byte
	var expiresAt time.Time
	err := s.db.QueryRowContext(ctx, "SELECT status, response_payload, expires_at FROM idempotency_keys WHERE key = $1", key).
		Scan(&status, &payload, &expiresAt)
	if err == nil {
		if expiresAt.After(now) {
			if status == string(IdempotencyStatusCompleted) {
				return IdempotencyStatusCompleted, payload, func(context.Context, bool, []byte) error { return nil }, nil
			}
			if status == string(IdempotencyStatusStarted) {
				return IdempotencyStatusStarted, nil, nil, fmt.Errorf("idempotency key %q is currently locked by an in-flight operation", key)
			}
		}
	}

	upsertStarted := `INSERT INTO idempotency_keys (key, status, response_payload, created_at, expires_at)
VALUES ($1, 'started', NULL, NOW(), $2)
ON CONFLICT (key) DO UPDATE
SET status = 'started', response_payload = NULL, created_at = NOW(), expires_at = EXCLUDED.expires_at
WHERE idempotency_keys.expires_at <= NOW()`
	res, err := s.db.ExecContext(ctx, upsertStarted, key, expires)
	if err != nil {
		return IdempotencyStatusStarted, nil, nil, fmt.Errorf("failed to acquire idempotency lock: %w", err)
	}
	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return IdempotencyStatusStarted, nil, nil, fmt.Errorf("idempotency key %q is currently locked by an in-flight operation", key)
	}

	unlock := func(uCtx context.Context, completed bool, responsePayload []byte) error {
		if !completed {
			_, err := resolveDBTX(uCtx, s.db, nil).ExecContext(uCtx, "DELETE FROM idempotency_keys WHERE key = $1 AND status = 'started'", key)
			return err
		}

		completedExpires := time.Now().Add(ttl)
		upsertCompleted := `INSERT INTO idempotency_keys (key, status, response_payload, created_at, expires_at)
VALUES ($1, 'completed', $2, NOW(), $3)
ON CONFLICT (key) DO UPDATE
SET status = 'completed', response_payload = EXCLUDED.response_payload, expires_at = EXCLUDED.expires_at`
		_, err := resolveDBTX(uCtx, s.db, nil).ExecContext(uCtx, upsertCompleted, key, responsePayload, completedExpires)
		return err
	}

	return IdempotencyStatusStarted, nil, unlock, nil
}

// PostgresOutboxStore manages durable outbox event staging and relaying in PostgreSQL.
type PostgresOutboxStore struct {
	db *sql.DB
}

// NewPostgresOutboxStore returns a new PostgresOutboxStore.
func NewPostgresOutboxStore(db *sql.DB) *PostgresOutboxStore {
	return &PostgresOutboxStore{db: db}
}

func (s *PostgresOutboxStore) Stage(ctx context.Context, event OutboxEvent) error {
	exec := resolveDBTX(ctx, s.db, nil)
	createdAt := event.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}
	query := `INSERT INTO outbox_events (id, topic, event_name, payload, created_at, published)
VALUES ($1, $2, $3, $4, $5, FALSE)`
	_, err := exec.ExecContext(ctx, query, event.ID, event.Topic, event.EventName, event.Payload, createdAt)
	return err
}

func (s *PostgresOutboxStore) Relay(ctx context.Context, publisher func(ctx context.Context, event OutboxEvent) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	rows, err := tx.QueryContext(ctx, `SELECT id, topic, event_name, payload, created_at
FROM outbox_events
WHERE published = FALSE
ORDER BY created_at ASC
FOR UPDATE SKIP LOCKED`)
	if err != nil {
		return err
	}
	defer rows.Close()

	var events []OutboxEvent
	for rows.Next() {
		var ev OutboxEvent
		if err := rows.Scan(&ev.ID, &ev.Topic, &ev.EventName, &ev.Payload, &ev.CreatedAt); err != nil {
			return err
		}
		events = append(events, ev)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	rows.Close()

	for _, ev := range events {
		if err := publisher(ctx, ev); err != nil {
			return fmt.Errorf("failed to publish outbox event %s: %w", ev.ID, err)
		}
		if _, err := tx.ExecContext(ctx, "UPDATE outbox_events SET published = TRUE WHERE id = $1", ev.ID); err != nil {
			return fmt.Errorf("failed to mark outbox event %s published: %w", ev.ID, err)
		}
	}
	return tx.Commit()
}

func (s *PostgresOutboxStore) MarkPublished(ctx context.Context, id string) error {
	_, err := resolveDBTX(ctx, s.db, nil).ExecContext(ctx, "UPDATE outbox_events SET published = TRUE WHERE id = $1", id)
	return err
}

// OutboxRelayWorker periodically polls outbox_events and publishes pending events.
type OutboxRelayWorker struct {
	store     OutboxStore
	publisher func(ctx context.Context, event OutboxEvent) error
	interval  time.Duration
	stopCh    chan struct{}
	doneCh    chan struct{}
}

// NewOutboxRelayWorker constructs an outbox relay worker.
func NewOutboxRelayWorker(store OutboxStore, publisher func(ctx context.Context, event OutboxEvent) error, interval time.Duration) *OutboxRelayWorker {
	if interval <= 0 {
		interval = 100 * time.Millisecond
	}
	return &OutboxRelayWorker{
		store:     store,
		publisher: publisher,
		interval:  interval,
		stopCh:    make(chan struct{}),
		doneCh:    make(chan struct{}),
	}
}

// PollOnce executes a single relay iteration immediately.
func (w *OutboxRelayWorker) PollOnce(ctx context.Context) error {
	return w.store.Relay(ctx, w.publisher)
}

// Start spawns the background polling loop.
func (w *OutboxRelayWorker) Start(ctx context.Context) {
	go func() {
		defer close(w.doneCh)
		ticker := time.NewTicker(w.interval)
		defer ticker.Stop()

		for {
			select {
			case <-w.stopCh:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				_ = w.PollOnce(ctx)
			}
		}
	}()
}

// Stop cleanly terminates the worker and waits for the loop to exit.
func (w *OutboxRelayWorker) Stop() {
	select {
	case <-w.stopCh:
	default:
		close(w.stopCh)
		<-w.doneCh
	}
}
