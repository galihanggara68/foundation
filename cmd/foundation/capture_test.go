package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/galihanggara68/foundation/pkg/compiler"
)

// TestCLI_Capture confirms capture flags for directory, target, and output file.
func TestCLI_Capture(t *testing.T) {
	dir := t.TempDir()

	blueprint := `
entity Task {
  id: uuid @pk
  title: string
}

action CreateTask(title: string) -> Task
  @http(method: POST, path: "/v1/tasks")
{
  let t = create Task { title: title }
  return t
}
`
	buildProfile := `
project:
  name: tasktracker
  module: example.com/tasktracker

storage:
  dialect: postgres
  mode: declarative

targets:
  - name: backend
    kind: backend
    language: go
    architecture: hexagonal
    http: chi
    out_dir: ./dist/backend
  - name: sdk
    kind: sdk
    language: go
    architecture: sdk
    out_dir: ./dist/sdk
`
	if err := os.WriteFile(filepath.Join(dir, "tasks.fdn"), []byte(blueprint), 0o644); err != nil {
		t.Fatalf("failed to write blueprint: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "foundation.build.yaml"), []byte(buildProfile), 0o644); err != nil {
		t.Fatalf("failed to write build profile: %v", err)
	}

	t.Run("default target to stdout", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := run([]string{"capture", "--dir", dir}, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("expected code 0, got %d: %s", code, stderr.String())
		}

		var req compiler.GenerationRequest
		if err := json.Unmarshal(stdout.Bytes(), &req); err != nil {
			t.Fatalf("failed to parse stdout JSON: %v", err)
		}
		if req.Target.Name != "backend" {
			t.Errorf("expected target backend, got %q", req.Target.Name)
		}
		if req.SemanticIR == nil || len(req.SemanticIR.Entities) != 1 {
			t.Fatalf("expected 1 entity in semantic IR, got %+v", req.SemanticIR)
		}
	})

	t.Run("explicit target flag", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := run([]string{"capture", "--dir", dir, "--target", "sdk"}, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("expected code 0, got %d: %s", code, stderr.String())
		}

		var req compiler.GenerationRequest
		if err := json.Unmarshal(stdout.Bytes(), &req); err != nil {
			t.Fatalf("failed to parse stdout JSON: %v", err)
		}
		if req.Target.Name != "sdk" {
			t.Errorf("expected target sdk, got %q", req.Target.Name)
		}
	})

	t.Run("output file flag", func(t *testing.T) {
		outFile := filepath.Join(t.TempDir(), "sub", "req.json")
		var stdout, stderr bytes.Buffer
		code := run([]string{"capture", "--dir", dir, "--out", outFile}, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("expected code 0, got %d: %s", code, stderr.String())
		}
		if stdout.Len() != 0 {
			t.Errorf("expected empty stdout with --out, got %s", stdout.String())
		}

		data, err := os.ReadFile(outFile)
		if err != nil {
			t.Fatalf("failed to read out file: %v", err)
		}

		var req compiler.GenerationRequest
		if err := json.Unmarshal(data, &req); err != nil {
			t.Fatalf("failed to parse file JSON: %v", err)
		}
		if req.Target.Name != "backend" {
			t.Errorf("expected target backend, got %q", req.Target.Name)
		}
	})

	t.Run("missing target error", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := run([]string{"capture", "--dir", dir, "--target", "nonexistent"}, &stdout, &stderr)
		if code != 1 {
			t.Fatalf("expected exit code 1, got %d", code)
		}
		if !strings.Contains(stderr.String(), "FDN0302") {
			t.Errorf("expected diagnostic FDN0302, got: %s", stderr.String())
		}
	})

	t.Run("invalid directory error", func(t *testing.T) {
		emptyDir := t.TempDir()
		var stdout, stderr bytes.Buffer
		code := run([]string{"capture", "--dir", emptyDir}, &stdout, &stderr)
		if code != 1 {
			t.Fatalf("expected exit code 1, got %d", code)
		}
	})
}
