package e2e_test

import (
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/galihanggara68/foundation/pkg/compiler"
)

func TestBuild_OCC_ScenariosAndConcurrentUpdates(t *testing.T) {
	dir := t.TempDir()

	blueprint := `
entity Order {
  id: uuid @pk
  version: int @version
  item: string
  total: int
}

error OrderNotFound {
  kind: not_found
  message: "order not found"
}

action CreateOrder(item: string, total: int) -> Order {
  let o = create Order {
    item: item,
    total: total
  }
  return o
}

action UpdateOrderStatus(id: uuid, expected_version: int, total: int) -> Order {
  update Order where id == id set {
    total: total
  }
  let o = query one Order where id == id else OrderNotFound
  return o
}

scenario "CreateOrder initializes version to 1" {
  when CreateOrder("Laptop", 1200)
  expect result.version == 1
}

scenario "UpdateOrderStatus with matching version succeeds and increments version" {
  given Order {
    id: "11111111-1111-1111-1111-111111111111"
    version: 1
    item: "Laptop"
    total: 1000
  }
  when UpdateOrderStatus("11111111-1111-1111-1111-111111111111", 1, 1100)
  expect result.version == 2
}

scenario "UpdateOrderStatus with stale version fails with Conflict error" {
  given Order {
    id: "22222222-2222-2222-2222-222222222222"
    version: 2
    item: "Monitor"
    total: 500
  }
  when UpdateOrderStatus("22222222-2222-2222-2222-222222222222", 1, 600)
  expect error Conflict
}
`
	if err := os.WriteFile(filepath.Join(dir, "app.fdn"), []byte(blueprint), 0644); err != nil {
		t.Fatalf("failed to write app.fdn: %v", err)
	}

	buildProfile := `
project:
  name: occ-test
  module: example.com/occtest

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

	testDiags := compiler.Test(dir)
	if len(testDiags) > 0 {
		t.Fatalf("expected OCC scenarios to pass, got: %+v", testDiags)
	}
}

func TestBuild_OCC_HTTPHeaders_ETagAndIfMatch(t *testing.T) {
	dir := t.TempDir()

	blueprint := `
entity Item {
  id: uuid @pk
  version: int @version
  name: string
  price: int
}

error ItemNotFound {
  kind: not_found
  message: "item not found"
}

action CreateItem(name: string, price: int) -> Item
  @http(method: POST, path: "/items")
{
  let it = create Item {
    name: name,
    price: price
  }
  return it
}

action GetItem(id: uuid) -> Item
  @http(method: GET, path: "/items/{id}")
{
  let it = query one Item where id == id else ItemNotFound
  return it
}

action UpdateItem(id: uuid, expected_version: int, price: int) -> Item
  @http(method: PUT, path: "/items/{id}")
{
  update Item where id == id set {
    price: price
  }
  let it = query one Item where id == id else ItemNotFound
  return it
}
`
	if err := os.WriteFile(filepath.Join(dir, "app.fdn"), []byte(blueprint), 0644); err != nil {
		t.Fatalf("failed to write app.fdn: %v", err)
	}

	buildProfile := `
project:
  name: occ-http-test
  module: example.com/occhttptest

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

	// Compile server binary
	binPath := filepath.Join(dir, "server-http-bin")
	buildCmd := exec.Command("go", "build", "-o", binPath, "./cmd/server")
	buildCmd.Dir = dir
	buildCmd.Env = append(os.Environ(), "GOPROXY=off")
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("go build ./cmd/server failed: %v\noutput:\n%s", err, string(out))
	}

	port := "19877"
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
	var createdID string

	// 1. Create item
	for i := 0; i < 30; i++ {
		time.Sleep(100 * time.Millisecond)
		resp, err := http.Post(baseURL+"/items", "application/json", strings.NewReader(`{"name":"Tablet","price":500}`))
		if err == nil && resp.StatusCode == http.StatusCreated {
			var body struct {
				ID      string `json:"id"`
				Version int    `json:"version"`
			}
			_ = json.NewDecoder(resp.Body).Decode(&body)
			resp.Body.Close()
			createdID = body.ID
			if body.Version != 1 {
				t.Fatalf("expected initial version 1, got %d", body.Version)
			}
			break
		}
	}
	if createdID == "" {
		t.Fatal("failed to create item via HTTP")
	}

	// 2. GET item -> assert ETag: "1"
	getResp, err := http.Get(baseURL + "/items/" + createdID)
	if err != nil {
		t.Fatalf("GET /items/%s failed: %v", createdID, err)
	}
	defer getResp.Body.Close()
	etag := getResp.Header.Get("ETag")
	if etag != `"1"` {
		t.Fatalf("expected ETag \"1\", got %q", etag)
	}

	// 3. Mutate with mismatching If-Match -> assert HTTP 409 Conflict
	client := &http.Client{}
	req, _ := http.NewRequest(http.MethodPut, baseURL+"/items/"+createdID, strings.NewReader(`{"expected_version":2,"price":550}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("If-Match", `"2"`) // Header is "2", but resource version in DB is 1
	// When action executes pipeline, repo.Update detects mismatch against DB version 1 and returns 409!
	conflictResp, err := client.Do(req)
	if err != nil {
		t.Fatalf("PUT with stale version failed: %v", err)
	}
	defer conflictResp.Body.Close()
	if conflictResp.StatusCode != http.StatusConflict {
		t.Fatalf("expected status 409 Conflict for stale update, got %d", conflictResp.StatusCode)
	}

	// 4. Mutate with header mismatching param -> assert HTTP 409 Conflict rejected before action
	req2, _ := http.NewRequest(http.MethodPut, baseURL+"/items/"+createdID, strings.NewReader(`{"expected_version":1,"price":550}`))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("If-Match", `"99"`) // Header doesn't match expected_version 1
	mismatchResp, err := client.Do(req2)
	if err != nil {
		t.Fatalf("PUT with mismatching If-Match failed: %v", err)
	}
	defer mismatchResp.Body.Close()
	if mismatchResp.StatusCode != http.StatusConflict {
		t.Fatalf("expected status 409 Conflict for If-Match mismatch, got %d", mismatchResp.StatusCode)
	}

	// 5. Mutate with matching version -> assert HTTP 200/201 and mutated version
	req3, _ := http.NewRequest(http.MethodPut, baseURL+"/items/"+createdID, strings.NewReader(`{"expected_version":1,"price":550}`))
	req3.Header.Set("Content-Type", "application/json")
	req3.Header.Set("If-Match", `"1"`)
	successResp, err := client.Do(req3)
	if err != nil {
		t.Fatalf("PUT with valid version failed: %v", err)
	}
	defer successResp.Body.Close()
	if successResp.StatusCode != http.StatusOK && successResp.StatusCode != http.StatusCreated {
		t.Fatalf("expected success status 200/201, got %d", successResp.StatusCode)
	}
	var updatedBody struct {
		ID      string `json:"id"`
		Version int    `json:"version"`
	}
	_ = json.NewDecoder(successResp.Body).Decode(&updatedBody)
	if updatedBody.Version != 2 {
		t.Fatalf("expected updated entity version 2, got %d", updatedBody.Version)
	}

	// 6. GET item again -> assert ETag is updated to "2"
	getResp2, err := http.Get(baseURL + "/items/" + createdID)
	if err != nil {
		t.Fatalf("GET /items/%s failed: %v", createdID, err)
	}
	defer getResp2.Body.Close()
	if etag2 := getResp2.Header.Get("ETag"); etag2 != `"2"` {
		t.Fatalf("expected updated ETag \"2\" on GET, got %q", etag2)
	}
}

func TestBuild_OCC_TypeScriptClean_PassesScenarios(t *testing.T) {
	dir := t.TempDir()

	blueprint := `
entity Document {
  id: uuid @pk
  version: int @version
  title: string
}

error DocumentNotFound {
  kind: not_found
  message: "document not found"
}

action CreateDocument(title: string) -> Document {
  let doc = create Document {
    title: title
  }
  return doc
}

action UpdateDocument(id: uuid, expected_version: int, title: string) -> Document {
  update Document where id == id set {
    title: title
  }
  let doc = query one Document where id == id else DocumentNotFound
  return doc
}

scenario "CreateDocument initializes version to 1 in TypeScript" {
  when CreateDocument("Roadmap")
  expect result.version == 1
}

scenario "UpdateDocument with matching version succeeds and increments in TypeScript" {
  given Document {
    id: "33333333-3333-3333-3333-333333333333"
    version: 1
    title: "Draft"
  }
  when UpdateDocument("33333333-3333-3333-3333-333333333333", 1, "Published")
  expect result.version == 2
}

scenario "UpdateDocument with stale version raises Conflict in TypeScript" {
  given Document {
    id: "44444444-4444-4444-4444-444444444444"
    version: 2
    title: "V2"
  }
  when UpdateDocument("44444444-4444-4444-4444-444444444444", 1, "V3")
  expect error Conflict
}
`
	if err := os.WriteFile(filepath.Join(dir, "app.fdn"), []byte(blueprint), 0644); err != nil {
		t.Fatalf("failed to write app.fdn: %v", err)
	}

	buildProfile := `
project:
  name: occ-ts-test
  module: example.com/occtstest

storage:
  dialect: memory
  mode: in_memory

targets:
  - name: backend-ts
    kind: backend
    language: typescript
    architecture: clean
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

	testDiags := compiler.Test(dir)
	if len(testDiags) > 0 {
		t.Fatalf("expected TypeScript OCC scenarios to pass, got: %+v", testDiags)
	}
}
