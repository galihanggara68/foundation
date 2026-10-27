package compiler_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/galihanggara68/foundation/pkg/compiler"
)

var (
	buildPluginsOnce sync.Once
	richDDDBin       string
	otelBin          string
	pythonFastAPIBin string
	buildPluginsErr  error
)

func ensurePluginBinaries(t *testing.T) (string, string, string) {
	t.Helper()
	buildPluginsOnce.Do(func() {
		tmpDir, err := os.MkdirTemp("", "foundation-plugins-*")
		if err != nil {
			buildPluginsErr = err
			return
		}

		richDDDBin = filepath.Join(tmpDir, "foundation-gen-rich-ddd")
		otelBin = filepath.Join(tmpDir, "foundation-gen-otel")
		pythonFastAPIBin = filepath.Join(tmpDir, "foundation-gen-python-fastapi")

		// Find workspace root (parent containing go.mod)
		wd, err := os.Getwd()
		if err != nil {
			buildPluginsErr = err
			return
		}
		root := wd
		for {
			if _, err := os.Stat(filepath.Join(root, "go.mod")); err == nil {
				break
			}
			parent := filepath.Dir(root)
			if parent == root {
				break
			}
			root = parent
		}

		buildCmd := func(pkg, dest string) error {
			cmd := exec.Command("go", "build", "-o", dest, pkg)
			cmd.Dir = root
			out, err := cmd.CombinedOutput()
			if err != nil {
				return err
			}
			_ = out
			return nil
		}

		if err := buildCmd("./cmd/foundation-gen-rich-ddd", richDDDBin); err != nil {
			buildPluginsErr = err
			return
		}
		if err := buildCmd("./cmd/foundation-gen-otel", otelBin); err != nil {
			buildPluginsErr = err
			return
		}
		if err := buildCmd("./cmd/foundation-gen-python-fastapi", pythonFastAPIBin); err != nil {
			buildPluginsErr = err
			return
		}
	})

	if buildPluginsErr != nil {
		t.Fatalf("failed to build reference generator plugins: %v", buildPluginsErr)
	}

	return richDDDBin, otelBin, pythonFastAPIBin
}

func TestE2E_GoRichDDDAndOtelPipeline_ScaffoldBuiltin(t *testing.T) {
	richBin, otelBin, _ := ensurePluginBinaries(t)
	dir := t.TempDir()

	blueprint := `
value CreateBookInput {
  sku: string
  title: string
  price: int
}

entity Book {
  id: uuid @pk
  sku: string @unique
  title: string
  price: int
}

error BookNotFound {
  kind: not_found
  message: "Requested book was not found"
}

action CreateBook(input: CreateBookInput) -> Book
  @http(method: POST, path: "/v1/books")
{
  let book = create Book {
    sku: input.sku,
    title: input.title,
    price: input.price
  }
  return book
}

scenario "CreateBook persists and returns book" {
  when CreateBook(CreateBookInput {
    sku: "FDN-200",
    title: "Hexagonal Go",
    price: 60
  })
  expect result.sku == "FDN-200"
  expect result.title == "Hexagonal Go"
  expect result.price == 60
}
`

	buildProfile := `schema_version: "0.1.0"
project:
  name: bookstore
  module: example.com/bookstore

storage:
  dialect: postgres
  mode: declarative

targets:
  - name: backend
    kind: backend
    language: go
    architecture: hexagonal
    http: chi
    scaffold: builtin
    generators:
      - ` + richBin + `
      - ` + otelBin + `
    out_dir: ./dist/backend
`

	if err := os.WriteFile(filepath.Join(dir, "catalog.fdn"), []byte(blueprint), 0644); err != nil {
		t.Fatalf("write blueprint: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "foundation.build.yaml"), []byte(buildProfile), 0644); err != nil {
		t.Fatalf("write build profile: %v", err)
	}

	// 1. Build
	res, diags := compiler.Build(dir, compiler.BuildOptions{})
	if len(diags) != 0 {
		t.Fatalf("expected 0 diagnostics, got: %+v", diags)
	}
	if res == nil {
		t.Fatal("expected non-nil BuildResult")
	}

	outDir := filepath.Join(dir, "dist", "backend")

	// Verify built-in files exist
	if _, err := os.Stat(filepath.Join(outDir, "go.mod")); err != nil {
		t.Errorf("expected go.mod to exist: %v", err)
	}

	// Verify Rich DDD aggregate root and test exist
	if _, err := os.Stat(filepath.Join(outDir, "internal", "domain", "book_entity.go")); err != nil {
		t.Errorf("expected book_entity.go to exist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outDir, "internal", "domain", "book_aggregate_test.go")); err != nil {
		t.Errorf("expected book_aggregate_test.go to exist: %v", err)
	}

	// Verify OpenTelemetry middleware and test exist
	if _, err := os.Stat(filepath.Join(outDir, "internal", "adapters", "http", "otel_middleware.go")); err != nil {
		t.Errorf("expected otel_middleware.go to exist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outDir, "internal", "adapters", "http", "otel_middleware_test.go")); err != nil {
		t.Errorf("expected otel_middleware_test.go to exist: %v", err)
	}

	// Verify .foundation-manifest.json tracks all emitted files
	manifestBytes, err := os.ReadFile(filepath.Join(outDir, ".foundation-manifest.json"))
	if err != nil {
		t.Fatalf("failed to read manifest: %v", err)
	}
	manStr := string(manifestBytes)
	if !strings.Contains(manStr, "internal/domain/book_entity.go") {
		t.Error("manifest missing book_entity.go")
	}
	if !strings.Contains(manStr, "internal/adapters/http/otel_middleware.go") {
		t.Error("manifest missing otel_middleware.go")
	}

	// Verify HTTP router was wrapped with OpenTelemetry middleware
	routerBytes, err := os.ReadFile(filepath.Join(outDir, "internal", "adapters", "http", "router.go"))
	if err != nil {
		t.Fatalf("failed to read router.go: %v", err)
	}
	if !strings.Contains(string(routerBytes), "OpenTelemetryMiddleware(s.router).ServeHTTP(w, r)") {
		t.Errorf("expected router.go to be wrapped with OpenTelemetryMiddleware, got: %s", string(routerBytes))
	}

	// 2. Test
	testDiags := compiler.Test(dir)
	if len(testDiags) != 0 {
		t.Fatalf("expected tests to pass, got: %+v", testDiags)
	}
}

func TestE2E_PythonFastAPIGenerator_ScaffoldNone_Pytest(t *testing.T) {
	_, _, pyBin := ensurePluginBinaries(t)
	dir := t.TempDir()

	blueprint := `
entity Item {
  id: string
  name: string
  price: int
}

action AddItem(name: string, price: int) -> Item {
  let item = create Item {
    name: name,
    price: price
  }
  return item
}
`

	buildProfile := `schema_version: "0.1.0"
project:
  name: pyservice
  module: example.com/pyservice

targets:
  - name: py-backend
    kind: backend
    language: python
    scaffold: none
    generator: ` + pyBin + `
    out_dir: ./dist/py-service
`

	if err := os.WriteFile(filepath.Join(dir, "service.fdn"), []byte(blueprint), 0644); err != nil {
		t.Fatalf("write blueprint: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "foundation.build.yaml"), []byte(buildProfile), 0644); err != nil {
		t.Fatalf("write build profile: %v", err)
	}

	// 1. Build
	res, diags := compiler.Build(dir, compiler.BuildOptions{})
	if len(diags) != 0 {
		t.Fatalf("expected 0 diagnostics, got: %+v", diags)
	}
	if res == nil {
		t.Fatal("expected non-nil BuildResult")
	}

	outDir := filepath.Join(dir, "dist", "py-service")
	for _, f := range []string{"models.py", "repository.py", "main.py", "test_app.py", ".foundation-manifest.json"} {
		if _, err := os.Stat(filepath.Join(outDir, f)); err != nil {
			t.Errorf("expected %s to exist: %v", f, err)
		}
	}

	// 2. Test (pytest)
	testDiags := compiler.Test(dir)
	if len(testDiags) != 0 {
		t.Fatalf("expected pytest to pass, got: %+v", testDiags)
	}
}

func TestE2E_ManifestPruningAndStubPreservation(t *testing.T) {
	richBin, _, _ := ensurePluginBinaries(t)
	dir := t.TempDir()

	blueprintV1 := `
entity Order {
  id: string
  total: int
}

entity Obsolete {
  id: string
}
`

	buildProfile := `schema_version: "0.1.0"
project:
  name: store
  module: example.com/store
targets:
  - name: backend
    kind: backend
    language: go
    scaffold: none
    generator: ` + richBin + `
    out_dir: ./dist/backend
`

	if err := os.WriteFile(filepath.Join(dir, "app.fdn"), []byte(blueprintV1), 0644); err != nil {
		t.Fatalf("write blueprint v1: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "foundation.build.yaml"), []byte(buildProfile), 0644); err != nil {
		t.Fatalf("write build profile: %v", err)
	}

	// Build V1
	_, diags1 := compiler.Build(dir, compiler.BuildOptions{})
	if len(diags1) != 0 {
		t.Fatalf("build v1 failed: %+v", diags1)
	}

	outDir := filepath.Join(dir, "dist", "backend")
	obsoleteFile := filepath.Join(outDir, "internal", "domain", "obsolete_entity.go")
	if _, err := os.Stat(obsoleteFile); err != nil {
		t.Fatalf("expected obsolete_entity.go to exist after v1: %v", err)
	}

	// Verify plugin emitted starter stub on V1 build
	stubFile := filepath.Join(outDir, "config.starter.json")
	if _, err := os.Stat(stubFile); err != nil {
		t.Fatalf("expected config.starter.json to be created by plugin stubs: %v", err)
	}

	// Verify stub file is NOT tracked in manifest
	manV1, err := os.ReadFile(filepath.Join(outDir, ".foundation-manifest.json"))
	if err != nil {
		t.Fatalf("read manifest v1: %v", err)
	}
	if strings.Contains(string(manV1), "config.starter.json") {
		t.Error("expected config.starter.json to NOT be tracked in manifest")
	}

	// Simulate developer customizing the starter stub file
	if err := os.WriteFile(stubFile, []byte(`{"custom": "user-config"}`), 0644); err != nil {
		t.Fatalf("customize stub file: %v", err)
	}

	// V2: Remove Obsolete entity
	blueprintV2 := `
entity Order {
  id: string
  total: int
}
`
	if err := os.WriteFile(filepath.Join(dir, "app.fdn"), []byte(blueprintV2), 0644); err != nil {
		t.Fatalf("write blueprint v2: %v", err)
	}

	// Build V2
	_, diags2 := compiler.Build(dir, compiler.BuildOptions{})
	if len(diags2) != 0 {
		t.Fatalf("build v2 failed: %+v", diags2)
	}

	// Verify Obsolete entity file was pruned
	if _, err := os.Stat(obsoleteFile); !os.IsNotExist(err) {
		t.Errorf("expected obsolete_entity.go to be pruned, but it still exists")
	}

	// Verify Order entity aggregate still exists
	orderFile := filepath.Join(outDir, "internal", "domain", "order_entity.go")
	if _, err := os.Stat(orderFile); err != nil {
		t.Errorf("expected order_entity.go to exist: %v", err)
	}

	// Verify handwritten stub file was preserved
	stubBytes, err := os.ReadFile(stubFile)
	if err != nil {
		t.Fatalf("expected stub file to be preserved: %v", err)
	}
	if string(stubBytes) != `{"custom": "user-config"}` {
		t.Errorf("stub content modified: %s", string(stubBytes))
	}
}
