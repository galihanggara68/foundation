package e2e_test

import (
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/galihanggara68/foundation/pkg/compiler"
)

func TestBuild_GoServerEntrypoint_TrackedInManifestAndRunnable(t *testing.T) {
	dir := t.TempDir()

	blueprint := `
entity Product {
  id: uuid @pk
  name: string
  price: int
}

error ProductNotFound {
  kind: not_found
  message: "product not found"
}

action CreateProduct(name: string, price: int) -> Product {
  let p = create Product {
    name: name,
    price: price
  }
  return p
}

action GetProduct(id: uuid) -> Product {
  let p = query one Product where id == id else ProductNotFound
  return p
}
`
	if err := os.WriteFile(filepath.Join(dir, "app.fdn"), []byte(blueprint), 0644); err != nil {
		t.Fatalf("failed to write app.fdn: %v", err)
	}

	buildProfile := `
project:
  name: server-run-test
  module: example.com/serverruntest

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

	res, diags := compiler.Build(dir, compiler.BuildOptions{})
	if len(diags) > 0 {
		t.Fatalf("unexpected build diagnostics: %+v", diags)
	}
	if res == nil {
		t.Fatal("expected non-nil build result")
	}

	// 1. Verify cmd/server/main.go is emitted and tracked in manifest with SHA-256
	mainGoPath := filepath.Join(dir, "cmd", "server", "main.go")
	if _, err := os.Stat(mainGoPath); os.IsNotExist(err) {
		t.Fatalf("expected %s to exist on disk", mainGoPath)
	}

	manifestBytes, err := os.ReadFile(filepath.Join(dir, ".foundation-manifest.json"))
	if err != nil {
		t.Fatalf("failed to read manifest: %v", err)
	}
	var manifest struct {
		Files map[string]string `json:"files"`
	}
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatalf("failed to parse manifest: %v", err)
	}
	if _, ok := manifest.Files["cmd/server/main.go"]; !ok {
		t.Fatalf("expected cmd/server/main.go to be tracked in manifest, got: %+v", manifest.Files)
	}

	// 2. Compile cmd/server/main.go to binary
	binPath := filepath.Join(dir, "server-bin")
	buildCmd := exec.Command("go", "build", "-o", binPath, "./cmd/server")
	buildCmd.Dir = dir
	buildCmd.Env = append(os.Environ(), "GOPROXY=off")
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("go build ./cmd/server failed: %v\noutput:\n%s", err, string(out))
	}

	// 3. Boot server and verify HTTP response and graceful shutdown
	port := "19876"
	srvCmd := exec.Command(binPath)
	srvCmd.Dir = dir
	srvCmd.Env = append(os.Environ(), "PORT="+port)

	if err := srvCmd.Start(); err != nil {
		t.Fatalf("failed to start server binary: %v", err)
	}

	// Poll until server is ready
	baseURL := "http://localhost:" + port
	var lastErr error
	ready := false
	for i := 0; i < 30; i++ {
		time.Sleep(100 * time.Millisecond)
		resp, err := http.Post(baseURL+"/products", "application/json", strings.NewReader(`{"name":"Keyboard","price":75}`))
		if err == nil {
			resp.Body.Close()
			ready = true
			break
		}
		lastErr = err
	}
	if !ready {
		_ = srvCmd.Process.Kill()
		t.Fatalf("server did not become ready in time: %v", lastErr)
	}

	// 4. Send SIGTERM and verify graceful shutdown with exit code 0
	if err := srvCmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("failed to send SIGTERM: %v", err)
	}

	waitDone := make(chan error, 1)
	go func() {
		waitDone <- srvCmd.Wait()
	}()

	select {
	case err := <-waitDone:
		if err != nil {
			t.Fatalf("expected server process to exit with code 0 on SIGTERM, got: %v", err)
		}
	case <-time.After(6 * time.Second):
		_ = srvCmd.Process.Kill()
		t.Fatal("server timed out during graceful shutdown")
	}
}

func TestBuild_GoServerEntrypoint_PrunedWhenTargetRemoved(t *testing.T) {
	dir := t.TempDir()

	blueprint := `
action Health() -> string {
  return "ok"
}
`
	if err := os.WriteFile(filepath.Join(dir, "app.fdn"), []byte(blueprint), 0644); err != nil {
		t.Fatalf("failed to write app.fdn: %v", err)
	}

	goProfile := `
project:
  name: prune-test
  module: example.com/prunetest
storage:
  dialect: memory
  mode: in_memory
targets:
  - name: backend
    kind: backend
    language: go
    architecture: hexagonal
`
	if err := os.WriteFile(filepath.Join(dir, "foundation.build.yaml"), []byte(goProfile), 0644); err != nil {
		t.Fatalf("failed to write build profile: %v", err)
	}

	if _, diags := compiler.Build(dir, compiler.BuildOptions{}); len(diags) > 0 {
		t.Fatalf("build 1 failed: %+v", diags)
	}

	mainGoPath := filepath.Join(dir, "cmd", "server", "main.go")
	if _, err := os.Stat(mainGoPath); os.IsNotExist(err) {
		t.Fatalf("expected %s to exist after build 1", mainGoPath)
	}

	// Rebuild with TypeScript target only -> cmd/server/main.go must be pruned
	tsProfile := `
project:
  name: prune-test
  module: example.com/prunetest
storage:
  dialect: memory
  mode: in_memory
targets:
  - name: backend
    kind: backend
    language: typescript
    architecture: clean
`
	if err := os.WriteFile(filepath.Join(dir, "foundation.build.yaml"), []byte(tsProfile), 0644); err != nil {
		t.Fatalf("failed to write build profile: %v", err)
	}

	if _, diags := compiler.Build(dir, compiler.BuildOptions{}); len(diags) > 0 {
		t.Fatalf("build 2 failed: %+v", diags)
	}

	if _, err := os.Stat(mainGoPath); !os.IsNotExist(err) {
		t.Fatalf("expected %s to be pruned after removing Go target", mainGoPath)
	}
}
