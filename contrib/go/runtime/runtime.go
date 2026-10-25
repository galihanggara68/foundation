package runtime

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

type txContextKey struct{}

// ContextWithTx injects an active transaction (e.g. *sql.Tx) into ctx.
func ContextWithTx(ctx context.Context, tx any) context.Context {
	return context.WithValue(ctx, txContextKey{}, tx)
}

// TxFromContext extracts an active transaction from ctx, if present.
func TxFromContext(ctx context.Context) any {
	if ctx == nil {
		return nil
	}
	return ctx.Value(txContextKey{})
}

// IdempotencyStatus indicates whether an operation is in progress or completed.
type IdempotencyStatus string

const (
	IdempotencyStatusStarted   IdempotencyStatus = "started"
	IdempotencyStatusCompleted IdempotencyStatus = "completed"
)

// UnlockFunc is invoked to unlock a held idempotency key, optionally recording completion and response data.
type UnlockFunc func(ctx context.Context, completed bool, responsePayload []byte) error

// IdempotencyStore manages idempotency keys and cached response payloads.
type IdempotencyStore interface {
	GetOrLock(ctx context.Context, key string, ttl time.Duration) (IdempotencyStatus, []byte, UnlockFunc, error)
}

// OutboxEvent represents a domain event queued for durable delivery.
type OutboxEvent struct {
	ID        string    `json:"id"`
	Topic     string    `json:"topic"`
	EventName string    `json:"event_name"`
	Payload   []byte    `json:"payload"`
	CreatedAt time.Time `json:"created_at"`
}

// OutboxStore provides transactional staging and relaying of outbox events.
type OutboxStore interface {
	Stage(ctx context.Context, event OutboxEvent) error
	Relay(ctx context.Context, publisher func(ctx context.Context, event OutboxEvent) error) error
	MarkPublished(ctx context.Context, id string) error
}

// CacheStore manages temporary key-value data with time-to-live expiration.
type CacheStore interface {
	Get(ctx context.Context, key string) ([]byte, bool, error)
	Set(ctx context.Context, key string, val []byte, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
}

// Driver combines stores into a unified runtime interface.
type Driver interface {
	Idempotency() IdempotencyStore
	Outbox() OutboxStore
	Cache() CacheStore
}

// MemoryDriver provides a zero-dependency in-memory implementation of the Driver.
type MemoryDriver struct {
	idem   IdempotencyStore
	outbox OutboxStore
	cache  CacheStore
}

func (d *MemoryDriver) Idempotency() IdempotencyStore { return d.idem }
func (d *MemoryDriver) Outbox() OutboxStore           { return d.outbox }
func (d *MemoryDriver) Cache() CacheStore             { return d.cache }

// NewMemoryDriver constructs an in-memory Driver suitable for offline hermetic testing.
func NewMemoryDriver() *MemoryDriver {
	return &MemoryDriver{
		idem:   NewMemoryIdempotencyStore(),
		outbox: NewMemoryOutboxStore(),
		cache:  NewMemoryCacheStore(),
	}
}

type memoryIdemEntry struct {
	status    IdempotencyStatus
	payload   []byte
	expiresAt time.Time
}

// MemoryIdempotencyStore stores idempotency records in memory with mutex synchronization.
type MemoryIdempotencyStore struct {
	mu      sync.Mutex
	entries map[string]memoryIdemEntry
}

// NewMemoryIdempotencyStore returns a new zero-dependency in-memory idempotency store.
func NewMemoryIdempotencyStore() *MemoryIdempotencyStore {
	return &MemoryIdempotencyStore{
		entries: make(map[string]memoryIdemEntry),
	}
}

func (s *MemoryIdempotencyStore) GetOrLock(ctx context.Context, key string, ttl time.Duration) (IdempotencyStatus, []byte, UnlockFunc, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	entry, exists := s.entries[key]
	if exists && entry.expiresAt.After(now) {
		if entry.status == IdempotencyStatusCompleted {
			return IdempotencyStatusCompleted, entry.payload, func(context.Context, bool, []byte) error { return nil }, nil
		}
		return IdempotencyStatusStarted, nil, nil, fmt.Errorf("idempotency key %q is currently locked by an in-flight operation", key)
	}

	// Lock the key
	s.entries[key] = memoryIdemEntry{
		status:    IdempotencyStatusStarted,
		expiresAt: now.Add(ttl),
	}

	unlock := func(ctx context.Context, completed bool, responsePayload []byte) error {
		s.mu.Lock()
		defer s.mu.Unlock()
		if !completed {
			delete(s.entries, key)
			return nil
		}
		s.entries[key] = memoryIdemEntry{
			status:    IdempotencyStatusCompleted,
			payload:   responsePayload,
			expiresAt: time.Now().Add(ttl),
		}
		return nil
	}

	return IdempotencyStatusStarted, nil, unlock, nil
}

type memoryCacheEntry struct {
	val       []byte
	expiresAt time.Time
}

// MemoryCacheStore stores key-value records in memory with TTL.
type MemoryCacheStore struct {
	mu      sync.RWMutex
	entries map[string]memoryCacheEntry
}

// NewMemoryCacheStore returns a new in-memory cache store.
func NewMemoryCacheStore() *MemoryCacheStore {
	return &MemoryCacheStore{
		entries: make(map[string]memoryCacheEntry),
	}
}

func (s *MemoryCacheStore) Get(ctx context.Context, key string) ([]byte, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry, exists := s.entries[key]
	if !exists || time.Now().After(entry.expiresAt) {
		return nil, false, nil
	}
	return entry.val, true, nil
}

func (s *MemoryCacheStore) Set(ctx context.Context, key string, val []byte, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.entries[key] = memoryCacheEntry{
		val:       val,
		expiresAt: time.Now().Add(ttl),
	}
	return nil
}

func (s *MemoryCacheStore) Delete(ctx context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.entries, key)
	return nil
}

type memoryOutboxRecord struct {
	event     OutboxEvent
	published bool
}

// MemoryOutboxStore buffers events in memory for testing.
type MemoryOutboxStore struct {
	mu      sync.Mutex
	records []memoryOutboxRecord
}

// NewMemoryOutboxStore returns a new in-memory outbox store.
func NewMemoryOutboxStore() *MemoryOutboxStore {
	return &MemoryOutboxStore{}
}

func (s *MemoryOutboxStore) Stage(ctx context.Context, event OutboxEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.records = append(s.records, memoryOutboxRecord{
		event:     event,
		published: false,
	})
	return nil
}

func (s *MemoryOutboxStore) Relay(ctx context.Context, publisher func(ctx context.Context, event OutboxEvent) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.records {
		if !s.records[i].published {
			if err := publisher(ctx, s.records[i].event); err != nil {
				return errors.Join(fmt.Errorf("failed to publish outbox event %s: %w", s.records[i].event.ID, err))
			}
			s.records[i].published = true
		}
	}
	return nil
}

func (s *MemoryOutboxStore) MarkPublished(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.records {
		if s.records[i].event.ID == id {
			s.records[i].published = true
			return nil
		}
	}
	return nil
}
