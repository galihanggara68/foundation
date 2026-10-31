package e2e_test

import (
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/galihanggara68/foundation/pkg/compiler"
)

// A business error's message is written for the caller, so it reaches them — with
// every locale the blueprint declared. A technical error's message is a diagnostic,
// so the response must carry the failure without the internal text.
func TestBuild_ErrorPosture_BusinessMessageReturnedAndTechnicalMessageWithheld(t *testing.T) {
	dir := t.TempDir()

	blueprint := `
entity Item {
  id: uuid @pk
  name: string
}

error ItemNotFound {
  kind: not_found
  message: "item not found"
  messages: {
    en: "item not found"
    id: "item tidak ditemukan"
  }
}

error Boom {
  kind: internal
  message: "pg: connection reset by peer"
}

action GetItem(id: uuid) -> Item
  @http(method: GET, path: "/items/{id}")
{
  let it = query one Item where id == id else ItemNotFound
  return it
}

action Probe(id: uuid) -> Item
  @http(method: GET, path: "/probe/{id}")
{
  let it = query one Item where id == id else Boom
  return it
}
`
	if err := os.WriteFile(filepath.Join(dir, "app.fdn"), []byte(blueprint), 0644); err != nil {
		t.Fatalf("failed to write app.fdn: %v", err)
	}

	buildProfile := `
project:
  name: error-posture-test
  module: example.com/errorposture

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

	if _, diags := compiler.Build(dir, compiler.BuildOptions{}); len(diags) > 0 {
		t.Fatalf("unexpected build diagnostics: %+v", diags)
	}

	binPath := filepath.Join(dir, "server-error-posture-bin")
	buildCmd := exec.Command("go", "build", "-o", binPath, "./cmd/server")
	buildCmd.Dir = dir
	buildCmd.Env = append(os.Environ(), "GOPROXY=off")
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("go build ./cmd/server failed: %v\noutput:\n%s", err, string(out))
	}

	port := "19879"
	srvCmd := exec.Command(binPath)
	srvCmd.Dir = dir
	srvCmd.Env = append(os.Environ(), "PORT="+port)
	if err := srvCmd.Start(); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	defer func() {
		_ = srvCmd.Process.Kill()
	}()

	baseURL := "http://localhost:" + port
	missingID := "3f0c8f1e-0000-4000-8000-000000000000"

	businessStatus, businessBody := getWithRetry(t, baseURL+"/items/"+missingID)
	if businessStatus != http.StatusNotFound {
		t.Fatalf("expected the not_found kind to resolve to 404, got %d (body %s)", businessStatus, businessBody)
	}
	if !strings.Contains(businessBody, "item not found") {
		t.Errorf("expected a business error's message to reach the caller, got %s", businessBody)
	}
	if !strings.Contains(businessBody, "item tidak ditemukan") {
		t.Errorf("expected the declared locale to reach the caller, got %s", businessBody)
	}

	technicalStatus, technicalBody := getWithRetry(t, baseURL+"/probe/"+missingID)
	if technicalStatus != http.StatusInternalServerError {
		t.Fatalf("expected the internal kind to resolve to 500, got %d (body %s)", technicalStatus, technicalBody)
	}
	if strings.Contains(technicalBody, "connection reset") {
		t.Errorf("a technical error's diagnostic must not reach the caller, got %s", technicalBody)
	}
	if !strings.Contains(technicalBody, "Boom") {
		t.Errorf("expected the error code so the failure is still reportable, got %s", technicalBody)
	}
}

func getWithRetry(t *testing.T, url string) (int, string) {
	t.Helper()
	for i := 0; i < 30; i++ {
		time.Sleep(100 * time.Millisecond)
		resp, err := http.Get(url)
		if err != nil {
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return resp.StatusCode, string(body)
	}
	t.Fatalf("server never accepted a request at %s", url)
	return 0, ""
}
