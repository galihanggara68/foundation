package runtime_test

import (
	"context"
	"testing"
	"time"

	"github.com/galihanggara68/foundation/contrib/go/runtime"
)

func TestMemoryIdempotencyStore_Lifecycle(t *testing.T) {
	ctx := context.Background()
	store := runtime.NewMemoryIdempotencyStore()

	key := "req-123"
	ttl := 1 * time.Minute

	// 1. First request gets lock with status started
	status, payload, unlock, err := store.GetOrLock(ctx, key, ttl)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != runtime.IdempotencyStatusStarted {
		t.Fatalf("expected status started, got %s", status)
	}
	if len(payload) > 0 {
		t.Fatalf("expected empty payload, got %s", string(payload))
	}

	// 2. Concurrent request while in-flight gets locked/in-progress error
	_, _, _, err2 := store.GetOrLock(ctx, key, ttl)
	if err2 == nil {
		t.Fatalf("expected error for in-flight locked key, got nil")
	}

	// 3. Complete and unlock with response payload
	respData := []byte(`{"order_id":"abc"}`)
	if err := unlock(ctx, true, respData); err != nil {
		t.Fatalf("failed to unlock: %v", err)
	}

	// 4. Duplicate request returns status completed and stored payload
	status3, payload3, _, err3 := store.GetOrLock(ctx, key, ttl)
	if err3 != nil {
		t.Fatalf("unexpected error: %v", err3)
	}
	if status3 != runtime.IdempotencyStatusCompleted {
		t.Fatalf("expected status completed, got %s", status3)
	}
	if string(payload3) != string(respData) {
		t.Fatalf("expected payload %s, got %s", string(respData), string(payload3))
	}
}

func TestMemoryCacheStore_Lifecycle(t *testing.T) {
	ctx := context.Background()
	store := runtime.NewMemoryCacheStore()

	key := "user:1"
	val := []byte("John Doe")

	// Missing key
	_, ok, err := store.Get(ctx, key)
	if err != nil || ok {
		t.Fatalf("expected missing key, got ok=%v err=%v", ok, err)
	}

	// Set with short TTL
	if err := store.Set(ctx, key, val, 50*time.Millisecond); err != nil {
		t.Fatalf("failed to set: %v", err)
	}

	// Get before expiry
	got, ok, err := store.Get(ctx, key)
	if err != nil || !ok {
		t.Fatalf("expected key to exist, got ok=%v err=%v", ok, err)
	}
	if string(got) != string(val) {
		t.Fatalf("expected %s, got %s", string(val), string(got))
	}

	// Wait for expiry
	time.Sleep(60 * time.Millisecond)
	_, ok, err = store.Get(ctx, key)
	if err != nil || ok {
		t.Fatalf("expected expired key, got ok=%v", ok)
	}

	// Set and Delete
	if err := store.Set(ctx, key, val, 1*time.Minute); err != nil {
		t.Fatalf("failed to set: %v", err)
	}
	if err := store.Delete(ctx, key); err != nil {
		t.Fatalf("failed to delete: %v", err)
	}
	_, ok, _ = store.Get(ctx, key)
	if ok {
		t.Fatalf("expected key to be deleted")
	}
}

func TestMemoryOutboxStore_StageAndRelay(t *testing.T) {
	ctx := context.Background()
	store := runtime.NewMemoryOutboxStore()

	ev := runtime.OutboxEvent{
		ID:        "evt-1",
		Topic:     "orders",
		EventName: "OrderCreated",
		Payload:   []byte(`{"id":"order-1"}`),
		CreatedAt: time.Now(),
	}

	if err := store.Stage(ctx, nil, ev); err != nil {
		t.Fatalf("failed to stage: %v", err)
	}

	var published []runtime.OutboxEvent
	err := store.Relay(ctx, func(ctx context.Context, e runtime.OutboxEvent) error {
		published = append(published, e)
		return nil
	})
	if err != nil {
		t.Fatalf("failed to relay: %v", err)
	}

	if len(published) != 1 || published[0].ID != "evt-1" {
		t.Fatalf("expected 1 published event, got %d", len(published))
	}

	// Subsequent relay publishes nothing new
	var published2 []runtime.OutboxEvent
	_ = store.Relay(ctx, func(ctx context.Context, e runtime.OutboxEvent) error {
		published2 = append(published2, e)
		return nil
	})
	if len(published2) != 0 {
		t.Fatalf("expected 0 published events on second relay, got %d", len(published2))
	}
}
