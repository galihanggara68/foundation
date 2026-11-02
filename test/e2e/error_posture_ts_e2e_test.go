package e2e_test

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/galihanggara68/foundation/pkg/compiler"
)

// The TypeScript backend mirrors the Go posture contract: a business error's message is
// written for the caller, so it reaches them with every declared locale, while a technical
// error's message is a diagnostic and must stay out of the response.
func TestBuild_ErrorPostureTS_BusinessMessageReturnedAndTechnicalMessageWithheld(t *testing.T) {
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
  name: error-posture-ts
  module: example.com/errorposturets

storage:
  dialect: memory
  mode: in_memory

targets:
  - name: server
    kind: backend
    language: typescript
    architecture: clean
    http: express
    out_dir: gen
`
	if err := os.WriteFile(filepath.Join(dir, "foundation.build.yaml"), []byte(buildProfile), 0644); err != nil {
		t.Fatalf("failed to write build profile: %v", err)
	}

	if _, diags := compiler.Build(dir, compiler.BuildOptions{}); len(diags) > 0 {
		t.Fatalf("unexpected build diagnostics: %+v", diags)
	}

	baseURL, stop := startTSServer(t, filepath.Join(dir, "gen"), "E2E_TS_ERROR_POSTURE=1")
	defer stop()

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
	if strings.Contains(technicalBody, `"message"`) {
		t.Errorf("a technical error must not return a message field, got %s", technicalBody)
	}
}
