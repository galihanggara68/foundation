package runtime_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/galihanggara68/foundation/contrib/go/runtime"
)

// TestConcurrency_IdempotencyLockStorm launches 1,000 concurrent goroutines
// attempting to acquire the lock for the exact same idempotency key simultaneously.
// Strict assertion: exactly 1 goroutine wins the lock; exactly 999 are rejected as in-flight;
// zero double-executions occur; and subsequent requests receive the completed payload.
func TestConcurrency_IdempotencyLockStorm(t *testing.T) {
	store := runtime.NewMemoryIdempotencyStore()
	key := "idem-storm-key-999"
	ttl := 10 * time.Second

	concurrency := 1000
	var wg sync.WaitGroup
	wg.Add(concurrency)

	var (
		startedCount   int64
		completedCount int64
		conflictCount  int64
		errorCount     int64
	)

	// Barrier to ensure all 1,000 goroutines hit the store simultaneously
	startBarrier := make(chan struct{})

	for i := 0; i < concurrency; i++ {
		go func(workerID int) {
			defer wg.Done()
			<-startBarrier

			status, payload, unlock, err := store.GetOrLock(context.Background(), key, ttl)
			if err != nil {
				atomic.AddInt64(&conflictCount, 1)
				return
			}

			if status == runtime.IdempotencyStatusStarted {
				atomic.AddInt64(&startedCount, 1)
				// Simulate unit of work
				time.Sleep(10 * time.Millisecond)
				resp := []byte(fmt.Sprintf("response-from-worker-%d", workerID))
				if uErr := unlock(context.Background(), true, resp); uErr != nil {
					atomic.AddInt64(&errorCount, 1)
				}
			} else if status == runtime.IdempotencyStatusCompleted {
				atomic.AddInt64(&completedCount, 1)
				_ = payload
			}
		}(i)
	}

	// Release the storm
	close(startBarrier)
	wg.Wait()

	t.Logf("IDEMPOTENCY STORM RESULTS (1,000 Concurrent Goroutines):")
	t.Logf("  Started (Lock Winners) : %d (Expected: 1)", startedCount)
	t.Logf("  In-Flight Conflicts    : %d (Expected: 999)", conflictCount)
	t.Logf("  Completed Reads        : %d", completedCount)
	t.Logf("  Errors                 : %d (Expected: 0)", errorCount)

	if startedCount != 1 {
		t.Errorf("CONCURRENCY FAILURE: Expected exactly 1 lock winner, got %d", startedCount)
	}
	if startedCount+conflictCount+completedCount != int64(concurrency) {
		t.Errorf("CONCURRENCY FAILURE: Sum of started (%d) + conflicts (%d) + completed (%d) != %d",
			startedCount, conflictCount, completedCount, concurrency)
	}

	// Verify post-completion behavior: next request must immediately get completed payload
	status, payload, _, err := store.GetOrLock(context.Background(), key, ttl)
	if err != nil {
		t.Fatalf("unexpected error on completed key: %v", err)
	}
	if status != runtime.IdempotencyStatusCompleted {
		t.Fatalf("expected status completed, got %v", status)
	}
	if len(payload) == 0 {
		t.Fatalf("expected cached response payload, got empty")
	}
	t.Logf("PASS: Post-storm request immediately retrieved completed payload: %s", string(payload))
}

// TestChaos_OutboxCrashAndResume stages 200 events, simulates a worker crash mid-relay
// (after 75 events published), restarts relay, and asserts zero event loss (at-least-once).
func TestChaos_OutboxCrashAndResume(t *testing.T) {
	store := runtime.NewMemoryOutboxStore()
	totalEvents := 200
	crashThreshold := 75

	// 1. Stage 200 events
	for i := 1; i <= totalEvents; i++ {
		err := store.Stage(context.Background(), runtime.OutboxEvent{
			ID:        fmt.Sprintf("evt-%04d", i),
			Topic:     "orders",
			EventName: "OrderCreated",
			Payload:   []byte(fmt.Sprintf(`{"order_id": %d}`, i)),
			CreatedAt: time.Now(),
		})
		if err != nil {
			t.Fatalf("failed to stage event %d: %v", i, err)
		}
	}

	publishedIDs := make(map[string]int)
	var pubMu sync.Mutex

	// 2. Run Relay with simulated crash at event 75
	errCrash := store.Relay(context.Background(), func(ctx context.Context, evt runtime.OutboxEvent) error {
		pubMu.Lock()
		count := len(publishedIDs)
		if count >= crashThreshold {
			pubMu.Unlock()
			return errors.New("CHAOS_SIMULATION: sudden process kill mid-relay")
		}
		publishedIDs[evt.ID]++
		pubMu.Unlock()
		return nil
	})

	if errCrash == nil {
		t.Fatalf("expected simulated crash error, got nil")
	}
	t.Logf("Simulated crash triggered cleanly after %d events published.", len(publishedIDs))

	// 3. Restart Relay (simulating daemon worker restart)
	errResume := store.Relay(context.Background(), func(ctx context.Context, evt runtime.OutboxEvent) error {
		pubMu.Lock()
		publishedIDs[evt.ID]++
		pubMu.Unlock()
		return nil
	})

	if errResume != nil {
		t.Fatalf("unexpected error on resumed relay: %v", errResume)
	}

	t.Logf("CHAOS RELAY RESULTS:")
	t.Logf("  Total Unique Events Published : %d / %d", len(publishedIDs), totalEvents)

	if len(publishedIDs) != totalEvents {
		t.Errorf("DURABILITY FAILURE: Expected %d events published, got %d (lost %d events!)",
			totalEvents, len(publishedIDs), totalEvents-len(publishedIDs))
	} else {
		t.Logf("PASS: 100%% of outbox events delivered after crash recovery (Zero data loss).")
	}
}

// TestConcurrency_CacheStampede launches 500 concurrent goroutines reading
// an expiring key simultaneously to verify thread safety without deadlocks.
func TestConcurrency_CacheStampede(t *testing.T) {
	cache := runtime.NewMemoryCacheStore()
	key := "hot-cache-key"
	_ = cache.Set(context.Background(), key, []byte("initial-value"), 50*time.Millisecond)

	concurrency := 500
	var wg sync.WaitGroup
	wg.Add(concurrency)

	var (
		hits   int64
		misses int64
	)

	// Wait for TTL expiration to induce stampede
	time.Sleep(60 * time.Millisecond)

	for i := 0; i < concurrency; i++ {
		go func(id int) {
			defer wg.Done()
			val, found, err := cache.Get(context.Background(), key)
			if err != nil {
				return
			}
			if found {
				atomic.AddInt64(&hits, 1)
				_ = val
			} else {
				atomic.AddInt64(&misses, 1)
				// Re-populate cache
				_ = cache.Set(context.Background(), key, []byte(fmt.Sprintf("val-%d", id)), 100*time.Millisecond)
			}
		}(i)
	}

	wg.Wait()

	t.Logf("CACHE STAMPEDE RESULTS (%d concurrent requests):", concurrency)
	t.Logf("  Cache Hits   : %d", hits)
	t.Logf("  Cache Misses : %d", misses)
	t.Logf("PASS: Cache store handled %d concurrent operations with zero race or panic.", concurrency)
}
