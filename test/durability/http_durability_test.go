package durability_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/galihanggara68/foundation/pkg/compiler"
)

func getFreePort(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to allocate free port: %v", err)
	}
	defer l.Close()
	return fmt.Sprintf("%d", l.Addr().(*net.TCPAddr).Port)
}

func setupChaosServer(t *testing.T) (string, func()) {
	t.Helper()
	dir := t.TempDir()

	blueprint := `
entity Wallet {
  id: uuid @pk
  balance: int
  version: int @version
}
error WalletNotFound {
  kind: not_found
  message: "not found"
}
action CreateWallet(balance: int) -> Wallet {
  let w = create Wallet { balance: balance }
  return w
}
action GetWallet(id: uuid) -> Wallet {
  let w = query one Wallet where id == id else WalletNotFound
  return w
}
action Deposit(id: uuid, amount: int, request_id: string) -> Wallet
  @http(method: POST, path: "/wallets/deposit")
  @idempotent(key: request_id)
  @transactional
{
  let w = query one Wallet where id == id else WalletNotFound
  update Wallet where id == w.id set { balance: w.balance + amount }
  let updated = query one Wallet where id == w.id else WalletNotFound
  return updated
}
`
	if err := os.WriteFile(filepath.Join(dir, "wallet.fdn"), []byte(blueprint), 0644); err != nil {
		t.Fatalf("failed to write wallet.fdn: %v", err)
	}

	buildProfile := `project:
  name: wallet-chaos
  module: example.com/walletchaos
storage:
  dialect: memory
  mode: in_memory
targets:
  - name: server
    kind: backend
    language: go
    architecture: hexagonal
`
	if err := os.WriteFile(filepath.Join(dir, "foundation.build.yaml"), []byte(buildProfile), 0644); err != nil {
		t.Fatalf("failed to write build profile: %v", err)
	}

	_, diags := compiler.Build(dir, compiler.BuildOptions{})
	if len(diags) > 0 {
		t.Fatalf("unexpected build diagnostics: %+v", diags)
	}

	binPath := filepath.Join(dir, "wallet-chaos-bin")
	buildCmd := exec.Command("go", "build", "-o", binPath, "./cmd/server")
	buildCmd.Dir = dir
	buildCmd.Env = append(os.Environ(), "GOPROXY=off")
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("go build ./cmd/server failed: %v\noutput:\n%s", err, string(out))
	}

	port := getFreePort(t)
	srvCmd := exec.Command(binPath)
	srvCmd.Dir = dir
	srvCmd.Env = append(os.Environ(), "PORT="+port)
	if err := srvCmd.Start(); err != nil {
		t.Fatalf("failed to start server binary: %v", err)
	}

	baseURL := "http://127.0.0.1:" + port
	client := &http.Client{Timeout: 3 * time.Second}

	// Wait for server readiness
	ready := false
	for i := 0; i < 30; i++ {
		time.Sleep(50 * time.Millisecond)
		resp, err := client.Post(baseURL+"/createwallet", "application/json", strings.NewReader(`{"balance":100}`))
		if err == nil && (resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated) {
			resp.Body.Close()
			ready = true
			break
		}
	}
	if !ready {
		_ = srvCmd.Process.Kill()
		t.Fatalf("server failed to become ready on port %s", port)
	}

	cleanup := func() {
		if srvCmd.Process != nil {
			_ = srvCmd.Process.Kill()
			_ = srvCmd.Wait()
		}
	}

	return baseURL, cleanup
}

// TestDurability_HTTP_IdempotencyConcurrencyFlood sends 100 concurrent HTTP requests
// with the exact same idempotency key to verify that only 1 deposit executes on the server.
func TestDurability_HTTP_IdempotencyConcurrencyFlood(t *testing.T) {
	baseURL, cleanup := setupChaosServer(t)
	defer cleanup()

	client := &http.Client{Timeout: 5 * time.Second}

	// 1. Create a wallet with initial balance 100
	createResp, err := client.Post(baseURL+"/createwallet", "application/json", strings.NewReader(`{"balance":100}`))
	if err != nil {
		t.Fatalf("failed to create wallet: %v", err)
	}
	var wallet struct {
		ID      string `json:"id"`
		Balance int    `json:"balance"`
	}
	if err := json.NewDecoder(createResp.Body).Decode(&wallet); err != nil {
		t.Fatalf("failed to decode wallet: %v", err)
	}
	createResp.Body.Close()

	// 2. Launch 100 concurrent HTTP requests with the exact same request_id
	concurrency := 100
	idempotencyKey := "chaos-idem-key-8888"
	depositPayload := fmt.Sprintf(`{"id":%q,"amount":50,"request_id":%q}`, wallet.ID, idempotencyKey)

	var wg sync.WaitGroup
	wg.Add(concurrency)

	var (
		successCount  int64
		conflictCount int64
		otherCount    int64
	)

	startBarrier := make(chan struct{})

	for i := 0; i < concurrency; i++ {
		go func() {
			defer wg.Done()
			<-startBarrier

			req, _ := http.NewRequest(http.MethodPost, baseURL+"/wallets/deposit", strings.NewReader(depositPayload))
			req.Header.Set("Content-Type", "application/json")
			resp, err := client.Do(req)
			if err != nil {
				atomic.AddInt64(&otherCount, 1)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
				atomic.AddInt64(&successCount, 1)
			} else if resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusConflict {
				atomic.AddInt64(&conflictCount, 1)
			} else {
				atomic.AddInt64(&otherCount, 1)
			}
		}()
	}

	close(startBarrier)
	wg.Wait()

	t.Logf("HTTP IDEMPOTENCY FLOOD RESULTS (100 Concurrent Requests):")
	t.Logf("  Successful (200/201)     : %d", successCount)
	t.Logf("  In-Flight Conflicts (400): %d", conflictCount)
	t.Logf("  Other Responses / Errors : %d", otherCount)

	if successCount == 0 {
		t.Fatalf("Expected at least 1 successful deposit, got 0")
	}

	// 3. Query wallet balance: must be exactly 150 (initial 100 + single deposit of 50).
	getResp, err := client.Post(baseURL+"/getwallet", "application/json", strings.NewReader(fmt.Sprintf(`{"id":%q}`, wallet.ID)))
	if err != nil {
		t.Fatalf("failed to query wallet: %v", err)
	}
	var finalWallet struct {
		Balance int `json:"balance"`
	}
	_ = json.NewDecoder(getResp.Body).Decode(&finalWallet)
	getResp.Body.Close()

	t.Logf("FINAL WALLET BALANCE: %d (Expected: 150)", finalWallet.Balance)
	if finalWallet.Balance != 150 {
		t.Errorf("CRITICAL DURABILITY FAILURE: Idempotency failed! Balance = %d (expected exactly 150)", finalWallet.Balance)
	} else {
		t.Logf("PASS: Exactly one deposit committed. Zero double-spend / duplicate execution.")
	}
}

// TestDurability_HTTP_ClientAbruptDisconnections cancels HTTP requests mid-flight
// and asserts that the server remains stable, healthy, and non-blocking.
func TestDurability_HTTP_ClientAbruptDisconnections(t *testing.T) {
	baseURL, cleanup := setupChaosServer(t)
	defer cleanup()

	// 1. Create a wallet
	resp, err := http.Post(baseURL+"/createwallet", "application/json", strings.NewReader(`{"balance":500}`))
	if err != nil {
		t.Fatalf("failed to create wallet: %v", err)
	}
	var wallet struct {
		ID string `json:"id"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&wallet)
	resp.Body.Close()

	// 2. Blast 30 abrupt client cancellations
	var cancelWg sync.WaitGroup
	cancelWg.Add(30)
	for i := 0; i < 30; i++ {
		go func(id int) {
			defer cancelWg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
			defer cancel()

			req, _ := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/wallets/deposit",
				strings.NewReader(fmt.Sprintf(`{"id":%q,"amount":10,"request_id":"cancel-%d"}`, wallet.ID, id)))
			req.Header.Set("Content-Type", "application/json")
			c := &http.Client{}
			r, err := c.Do(req)
			if err == nil {
				r.Body.Close()
			}
		}(i)
	}
	cancelWg.Wait()

	// 3. Immediately verify server health with normal requests
	healthyClient := &http.Client{Timeout: 2 * time.Second}
	for i := 1; i <= 5; i++ {
		hResp, err := healthyClient.Post(baseURL+"/getwallet", "application/json", strings.NewReader(fmt.Sprintf(`{"id":%q}`, wallet.ID)))
		if err != nil {
			t.Fatalf("server unhealthy after abrupt disconnections: %v", err)
		}
		hResp.Body.Close()
		if hResp.StatusCode != http.StatusOK && hResp.StatusCode != http.StatusCreated {
			t.Fatalf("expected HTTP 200/201, got %d", hResp.StatusCode)
		}
	}
	t.Logf("PASS: Server remained healthy and resilient through 30 abrupt socket cancellations.")
}

// TestDurability_HTTP_BurstThroughput benchmarks latency and throughput across 200 requests.
func TestDurability_HTTP_BurstThroughput(t *testing.T) {
	baseURL, cleanup := setupChaosServer(t)
	defer cleanup()

	resp, err := http.Post(baseURL+"/createwallet", "application/json", strings.NewReader(`{"balance":1000}`))
	if err != nil {
		t.Fatalf("failed to create wallet: %v", err)
	}
	var wallet struct {
		ID string `json:"id"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&wallet)
	resp.Body.Close()

	totalRequests := 200
	concurrency := 10
	requestsPerWorker := totalRequests / concurrency

	var latencies []time.Duration
	var latMu sync.Mutex
	var errCount int64

	client := &http.Client{
		Transport: &http.Transport{
			MaxIdleConns:        50,
			MaxIdleConnsPerHost: 50,
		},
		Timeout: 5 * time.Second,
	}

	start := time.Now()
	var wg sync.WaitGroup
	wg.Add(concurrency)

	for w := 0; w < concurrency; w++ {
		go func(workerID int) {
			defer wg.Done()
			for r := 0; r < requestsPerWorker; r++ {
				reqStart := time.Now()
				reqBody := fmt.Sprintf(`{"id":%q}`, wallet.ID)
				res, err := client.Post(baseURL+"/getwallet", "application/json", strings.NewReader(reqBody))
				dur := time.Since(reqStart)

				if err != nil || (res.StatusCode != http.StatusOK && res.StatusCode != http.StatusCreated) {
					atomic.AddInt64(&errCount, 1)
				} else {
					res.Body.Close()
					latMu.Lock()
					latencies = append(latencies, dur)
					latMu.Unlock()
				}
			}
		}(w)
	}

	wg.Wait()
	totalElapsed := time.Since(start)

	if errCount > 0 {
		t.Fatalf("Encountered %d errors during burst load", errCount)
	}

	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	p50 := latencies[len(latencies)*50/100]
	p95 := latencies[len(latencies)*95/100]
	p99 := latencies[len(latencies)*99/100]
	rps := float64(totalRequests) / totalElapsed.Seconds()

	t.Logf("HTTP BURST THROUGHPUT REPORT (%d requests, %d workers):", totalRequests, concurrency)
	t.Logf("  Total Time   : %v", totalElapsed)
	t.Logf("  Throughput   : %.1f req/sec", rps)
	t.Logf("  p50 Latency  : %v", p50)
	t.Logf("  p95 Latency  : %v", p95)
	t.Logf("  p99 Latency  : %v", p99)
	t.Logf("  Error Rate   : 0.00%%")
}

// TestDurability_HTTP_GracefulShutdown verifies clean SIGTERM termination and socket drainage.
func TestDurability_HTTP_GracefulShutdown(t *testing.T) {
	dir := t.TempDir()

	blueprint := "entity Status { id: uuid @pk \n code: string }\n action GetStatus() -> Status { let s = create Status { code: \"OK\" }\n return s }\n"
	if err := os.WriteFile(filepath.Join(dir, "status.fdn"), []byte(blueprint), 0644); err != nil {
		t.Fatalf("write blueprint: %v", err)
	}
	buildProfile := "project:\n  name: status-shutdown\n  module: example.com/statusshutdown\nstorage:\n  dialect: memory\n  mode: in_memory\ntargets:\n  - name: server\n    kind: backend\n    language: go\n    architecture: hexagonal\n"
	if err := os.WriteFile(filepath.Join(dir, "foundation.build.yaml"), []byte(buildProfile), 0644); err != nil {
		t.Fatalf("write profile: %v", err)
	}

	if _, diags := compiler.Build(dir, compiler.BuildOptions{}); len(diags) > 0 {
		t.Fatalf("build failed: %v", diags)
	}

	binPath := filepath.Join(dir, "server-term-bin")
	cmd := exec.Command("go", "build", "-o", binPath, "./cmd/server")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOPROXY=off")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build binary: %v\n%s", err, string(out))
	}

	port := getFreePort(t)
	srv := exec.Command(binPath)
	srv.Dir = dir
	srv.Env = append(os.Environ(), "PORT="+port)
	if err := srv.Start(); err != nil {
		t.Fatalf("start server: %v", err)
	}

	// Verify server is responsive
	client := &http.Client{Timeout: 2 * time.Second}
	ready := false
	for i := 0; i < 20; i++ {
		time.Sleep(50 * time.Millisecond)
		if resp, err := client.Post("http://127.0.0.1:"+port+"/getstatus", "application/json", strings.NewReader("{}")); err == nil {
			resp.Body.Close()
			ready = true
			break
		}
	}
	if !ready {
		_ = srv.Process.Kill()
		t.Fatalf("server did not become ready")
	}

	// Send SIGTERM
	if err := srv.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("failed to send SIGTERM: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- srv.Wait()
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("server exited with error after SIGTERM: %v", err)
		}
		t.Logf("PASS: Server gracefully drained connections and exited with code 0 on SIGTERM.")
	case <-time.After(3 * time.Second):
		_ = srv.Process.Kill()
		t.Fatalf("server timed out waiting for graceful shutdown")
	}
}
