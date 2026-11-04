package compiler_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/galihanggara68/foundation/pkg/compiler"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/protocol"
)

// TestBuild_CustomGeneratorBackendReceivesStoragePlan confirms that custom backend generator targets
// receive the compiled storage plan in GenerationRequest.
func TestBuild_CustomGeneratorBackendReceivesStoragePlan(t *testing.T) {
	tmpDir := t.TempDir()

	binDir := filepath.Join(tmpDir, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatalf("failed to create bin dir: %v", err)
	}

	capturedFile := filepath.Join(tmpDir, "captured_request.json")
	mockPlugin := filepath.Join(binDir, "mock-backend-gen")
	pluginScript := fmt.Sprintf(`#!/bin/sh
cat > %q
cat << 'EOF'
{
  "protocol_version": 1,
  "files": {
    "output.txt": "generated"
  }
}
EOF
`, capturedFile)

	if err := os.WriteFile(mockPlugin, []byte(pluginScript), 0755); err != nil {
		t.Fatalf("failed to write mock plugin: %v", err)
	}

	fdnSrc := `entity Order {
  id: uuid @pk
  total: decimal(10, 2)
}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "app.fdn"), []byte(fdnSrc), 0644); err != nil {
		t.Fatalf("failed to write fdn source: %v", err)
	}

	buildYAML := `schema_version: "0.1.0"
project:
  name: test-project
  module: example.com/test
targets:
  - name: custom-backend
    kind: backend
    language: python
    generator: ./bin/mock-backend-gen
    scaffold: none
    out_dir: dist/backend
`
	if err := os.WriteFile(filepath.Join(tmpDir, "foundation.build.yaml"), []byte(buildYAML), 0644); err != nil {
		t.Fatalf("failed to write build yaml: %v", err)
	}

	res, buildDiags := compiler.Build(tmpDir, compiler.BuildOptions{})
	if len(buildDiags) > 0 {
		t.Fatalf("build failed with diagnostics: %+v", buildDiags)
	}
	if res == nil {
		t.Fatal("expected non-nil build result")
	}

	if len(res.Architectures) != 1 {
		t.Fatalf("expected 1 architecture IR, got %d", len(res.Architectures))
	}
	archIR := res.Architectures[0]
	if archIR.Storage.Dialect != "postgres" {
		t.Errorf("expected dialect postgres, got %q", archIR.Storage.Dialect)
	}
	if !strings.Contains(archIR.Storage.SchemaSQL, "CREATE TABLE orders") {
		t.Errorf("expected SchemaSQL to contain CREATE TABLE orders, got:\n%s", archIR.Storage.SchemaSQL)
	}

	reqData, err := os.ReadFile(capturedFile)
	if err != nil {
		t.Fatalf("failed to read captured request file: %v", err)
	}

	var req protocol.GenerationRequest
	if err := json.Unmarshal(reqData, &req); err != nil {
		t.Fatalf("failed to unmarshal captured request: %v", err)
	}

	if req.ArchitectureIR == nil {
		t.Fatal("expected non-nil ArchitectureIR in GenerationRequest")
	}
	if req.ArchitectureIR.Storage.Dialect != "postgres" {
		t.Errorf("expected request dialect postgres, got %q", req.ArchitectureIR.Storage.Dialect)
	}
	if !strings.Contains(req.ArchitectureIR.Storage.SchemaSQL, "CREATE TABLE orders") {
		t.Errorf("expected request SchemaSQL to contain CREATE TABLE orders, got:\n%s", req.ArchitectureIR.Storage.SchemaSQL)
	}
}

// TestBuild_CustomGeneratorSDKTargetStoragePlanEmpty confirms that non-backend custom generator targets
// receive an empty storage plan.
func TestBuild_CustomGeneratorSDKTargetStoragePlanEmpty(t *testing.T) {
	tmpDir := t.TempDir()

	binDir := filepath.Join(tmpDir, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatalf("failed to create bin dir: %v", err)
	}

	capturedFile := filepath.Join(tmpDir, "captured_sdk_request.json")
	mockPlugin := filepath.Join(binDir, "mock-sdk-gen")
	pluginScript := fmt.Sprintf(`#!/bin/sh
cat > %q
cat << 'EOF'
{
  "protocol_version": 1,
  "files": {
    "sdk_output.txt": "sdk generated"
  }
}
EOF
`, capturedFile)

	if err := os.WriteFile(mockPlugin, []byte(pluginScript), 0755); err != nil {
		t.Fatalf("failed to write mock plugin: %v", err)
	}

	fdnSrc := `entity Item {
  id: uuid @pk
  name: string
}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "app.fdn"), []byte(fdnSrc), 0644); err != nil {
		t.Fatalf("failed to write fdn source: %v", err)
	}

	buildYAML := `schema_version: "0.1.0"
project:
  name: test-sdk-project
  module: example.com/testsdk
targets:
  - name: custom-sdk
    kind: sdk
    language: python
    generator: ./bin/mock-sdk-gen
    scaffold: none
    out_dir: dist/sdk
`
	if err := os.WriteFile(filepath.Join(tmpDir, "foundation.build.yaml"), []byte(buildYAML), 0644); err != nil {
		t.Fatalf("failed to write build yaml: %v", err)
	}

	res, buildDiags := compiler.Build(tmpDir, compiler.BuildOptions{})
	if len(buildDiags) > 0 {
		t.Fatalf("build failed with diagnostics: %+v", buildDiags)
	}
	if res == nil {
		t.Fatal("expected non-nil build result")
	}

	if len(res.Architectures) != 1 {
		t.Fatalf("expected 1 architecture IR, got %d", len(res.Architectures))
	}
	archIR := res.Architectures[0]
	if archIR.Storage.SchemaSQL != "" {
		t.Errorf("expected empty SchemaSQL for SDK target, got %q", archIR.Storage.SchemaSQL)
	}

	reqData, err := os.ReadFile(capturedFile)
	if err != nil {
		t.Fatalf("failed to read captured request file: %v", err)
	}

	var req protocol.GenerationRequest
	if err := json.Unmarshal(reqData, &req); err != nil {
		t.Fatalf("failed to unmarshal captured request: %v", err)
	}

	if req.ArchitectureIR == nil {
		t.Fatal("expected non-nil ArchitectureIR in GenerationRequest")
	}
	if req.ArchitectureIR.Storage.SchemaSQL != "" {
		t.Errorf("expected empty SchemaSQL in request for SDK target, got %q", req.ArchitectureIR.Storage.SchemaSQL)
	}
}
