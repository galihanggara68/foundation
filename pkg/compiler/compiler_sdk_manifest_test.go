package compiler_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/galihanggara68/foundation/pkg/compiler"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/emit"
)

func TestRegression_SDKManifestPrunesDeletedActionAndFiles(t *testing.T) {
	dir := t.TempDir()

	bpV1 := `
entity Item {
  id: uuid @pk
  name: string
}

error ItemNotFound {
  kind: not_found
  message: "item not found"
}

action CreateItem(name: string) -> Item
  @http(method: POST, path: "/v1/items")
{
  let it = create Item {
    name: name
  }
  return it
}

action DeleteItem(id: uuid)
  @http(method: DELETE, path: "/v1/items/{id}")
{
  let it = query one Item where id == id else ItemNotFound
}
`
	buildYAML := `
project:
  name: items-service
  module: example.com/items

targets:
  - name: go-sdk
    kind: sdk
    language: go
    out_dir: clients/go
  - name: ts-sdk
    kind: sdk
    language: typescript
    out_dir: clients/ts
`
	if err := os.WriteFile(filepath.Join(dir, "items.fdn"), []byte(bpV1), 0644); err != nil {
		t.Fatalf("failed to write items.fdn v1: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "foundation.build.yaml"), []byte(buildYAML), 0644); err != nil {
		t.Fatalf("failed to write build yaml: %v", err)
	}

	// 1. Initial Build with both CreateItem and DeleteItem
	_, diags := compiler.Build(dir, compiler.BuildOptions{})
	if len(diags) > 0 {
		t.Fatalf("initial build failed: %+v", diags)
	}

	goSDKDir := filepath.Join(dir, "clients", "go")
	tsSDKDir := filepath.Join(dir, "clients", "ts")

	goActionsV1, err := os.ReadFile(filepath.Join(goSDKDir, "actions.go"))
	if err != nil {
		t.Fatalf("read go actions.go v1: %v", err)
	}
	if !strings.Contains(string(goActionsV1), "func (c *Client) CreateItem") {
		t.Errorf("go actions.go missing CreateItem")
	}
	if !strings.Contains(string(goActionsV1), "func (c *Client) DeleteItem") {
		t.Errorf("go actions.go missing DeleteItem")
	}

	tsClientV1, err := os.ReadFile(filepath.Join(tsSDKDir, "client.ts"))
	if err != nil {
		t.Fatalf("read ts client.ts v1: %v", err)
	}
	if !strings.Contains(string(tsClientV1), "async createItem") {
		t.Errorf("ts client.ts missing createItem")
	}
	if !strings.Contains(string(tsClientV1), "async deleteItem") {
		t.Errorf("ts client.ts missing deleteItem")
	}

	// Read initial manifest hashes
	goManifestV1Bytes, err := os.ReadFile(filepath.Join(goSDKDir, ".foundation-manifest.json"))
	if err != nil {
		t.Fatalf("read go manifest v1: %v", err)
	}
	var goManifestV1 struct {
		Files map[string]string `json:"files"`
	}
	if err := json.Unmarshal(goManifestV1Bytes, &goManifestV1); err != nil {
		t.Fatalf("unmarshal go manifest v1: %v", err)
	}
	hashGoV1 := goManifestV1.Files["actions.go"]
	if hashGoV1 == "" {
		t.Fatalf("expected actions.go to be tracked in go manifest v1")
	}

	tsManifestV1Bytes, err := os.ReadFile(filepath.Join(tsSDKDir, ".foundation-manifest.json"))
	if err != nil {
		t.Fatalf("read ts manifest v1: %v", err)
	}
	var tsManifestV1 struct {
		Files map[string]string `json:"files"`
	}
	if err := json.Unmarshal(tsManifestV1Bytes, &tsManifestV1); err != nil {
		t.Fatalf("unmarshal ts manifest v1: %v", err)
	}
	hashTSV1 := tsManifestV1.Files["client.ts"]
	if hashTSV1 == "" {
		t.Fatalf("expected client.ts to be tracked in ts manifest v1")
	}

	// 2. Modify Blueprint to prune DeleteItem
	bpV2 := `
entity Item {
  id: uuid @pk
  name: string
}

action CreateItem(name: string) -> Item
  @http(method: POST, path: "/v1/items")
{
  let it = create Item {
    name: name
  }
  return it
}
`
	if err := os.WriteFile(filepath.Join(dir, "items.fdn"), []byte(bpV2), 0644); err != nil {
		t.Fatalf("failed to write items.fdn v2: %v", err)
	}

	// Rebuild
	_, diags2 := compiler.Build(dir, compiler.BuildOptions{})
	if len(diags2) > 0 {
		t.Fatalf("rebuild v2 failed: %+v", diags2)
	}

	// Verify DeleteItem was cleanly pruned from Go SDK actions.go
	goActionsV2, err := os.ReadFile(filepath.Join(goSDKDir, "actions.go"))
	if err != nil {
		t.Fatalf("read go actions.go v2: %v", err)
	}
	if !strings.Contains(string(goActionsV2), "func (c *Client) CreateItem") {
		t.Errorf("go actions.go should still contain CreateItem")
	}
	if strings.Contains(string(goActionsV2), "DeleteItem") {
		t.Errorf("go actions.go should have pruned DeleteItem method, but it is still present")
	}

	// Verify deleteItem was cleanly pruned from TS SDK client.ts
	tsClientV2, err := os.ReadFile(filepath.Join(tsSDKDir, "client.ts"))
	if err != nil {
		t.Fatalf("read ts client.ts v2: %v", err)
	}
	if !strings.Contains(string(tsClientV2), "async createItem") {
		t.Errorf("ts client.ts should still contain createItem")
	}
	if strings.Contains(string(tsClientV2), "deleteItem") {
		t.Errorf("ts client.ts should have pruned deleteItem method, but it is still present")
	}

	// Verify manifests recorded updated cryptographic hashes
	goManifestV2Bytes, err := os.ReadFile(filepath.Join(goSDKDir, ".foundation-manifest.json"))
	if err != nil {
		t.Fatalf("read go manifest v2: %v", err)
	}
	var goManifestV2 struct {
		Files map[string]string `json:"files"`
	}
	_ = json.Unmarshal(goManifestV2Bytes, &goManifestV2)
	if goManifestV2.Files["actions.go"] == hashGoV1 {
		t.Errorf("expected go actions.go hash to change in manifest after pruning action, got same hash %s", hashGoV1)
	}

	tsManifestV2Bytes, err := os.ReadFile(filepath.Join(tsSDKDir, ".foundation-manifest.json"))
	if err != nil {
		t.Fatalf("read ts manifest v2: %v", err)
	}
	var tsManifestV2 struct {
		Files map[string]string `json:"files"`
	}
	_ = json.Unmarshal(tsManifestV2Bytes, &tsManifestV2)
	if tsManifestV2.Files["client.ts"] == hashTSV1 {
		t.Errorf("expected ts client.ts hash to change in manifest after pruning action, got same hash %s", hashTSV1)
	}
}

func TestRegression_SDKManifestPrunesObsoleteFiles(t *testing.T) {
	outDir := t.TempDir()

	// 1. Initial emission has client.go and obsolete_helper.go
	tracked1 := map[string][]byte{
		"client.go":          []byte("package client\n"),
		"obsolete_helper.go": []byte("package client\n"),
	}
	diags1 := emit.ReconcileManifest(outDir, tracked1, nil, "", false)
	if len(diags1) > 0 {
		t.Fatalf("reconcile 1 failed: %+v", diags1)
	}

	obsoletePath := filepath.Join(outDir, "obsolete_helper.go")
	if _, err := os.Stat(obsoletePath); err != nil {
		t.Fatalf("expected obsolete_helper.go to exist: %v", err)
	}

	// 2. Re-emit without obsolete_helper.go
	tracked2 := map[string][]byte{
		"client.go": []byte("package client\n"),
	}
	diags2 := emit.ReconcileManifest(outDir, tracked2, nil, "", false)
	if len(diags2) > 0 {
		t.Fatalf("reconcile 2 failed: %+v", diags2)
	}

	// Verify obsolete_helper.go was pruned from disk
	if _, err := os.Stat(obsoletePath); !os.IsNotExist(err) {
		t.Fatalf("expected obsolete_helper.go to be pruned from disk, but it still exists")
	}

	// Verify manifest no longer tracks obsolete_helper.go
	manifestBytes, err := os.ReadFile(filepath.Join(outDir, emit.ManifestFileName))
	if err != nil {
		t.Fatalf("failed to read manifest: %v", err)
	}
	var manifest emit.GenerationManifest
	_ = json.Unmarshal(manifestBytes, &manifest)
	if _, ok := manifest.Files["obsolete_helper.go"]; ok {
		t.Fatalf("manifest should not track obsolete_helper.go anymore")
	}
	if _, ok := manifest.Files["client.go"]; !ok {
		t.Fatalf("manifest should still track client.go")
	}
}
