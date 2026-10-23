package compiler_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/galihanggara68/foundation/pkg/compiler"
)

func TestCheck_ValidMinimalBlueprint_ReturnsSemanticIR(t *testing.T) {
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

action GetBookBySKU(sku: string) -> Book
  @http(method: GET, path: "/v1/books/{sku}")
{
  let book = query one Book where sku == sku else BookNotFound
  return book
}

scenario "Create and query a book" {
  given Book {
    id: "00000000-0000-0000-0000-000000000001",
    sku: "FDN-101",
    title: "Domain Compilers",
    price: 45
  }
  when GetBookBySKU("FDN-101")
  expect result.title == "Domain Compilers"
  expect result.price == 45
}
`
	if err := os.WriteFile(filepath.Join(dir, "catalog.fdn"), []byte(blueprint), 0644); err != nil {
		t.Fatalf("write blueprint: %v", err)
	}

	ir, diags := compiler.Check(dir)
	if len(diags) != 0 {
		t.Fatalf("expected 0 diagnostics, got %+v", diags)
	}
	if ir == nil {
		t.Fatal("expected non-nil SemanticIR")
	}
	if len(ir.Entities) != 1 || ir.Entities[0].Name != "Book" {
		t.Fatalf("expected 1 entity 'Book', got %+v", ir.Entities)
	}
	if len(ir.Values) != 1 || ir.Values[0].Name != "CreateBookInput" {
		t.Fatalf("expected 1 value 'CreateBookInput', got %+v", ir.Values)
	}
	if len(ir.Errors) != 1 || ir.Errors[0].Name != "BookNotFound" {
		t.Fatalf("expected 1 error 'BookNotFound', got %+v", ir.Errors)
	}
	if len(ir.Actions) != 2 {
		t.Fatalf("expected 2 actions, got %d", len(ir.Actions))
	}
	if len(ir.Scenarios) != 1 || ir.Scenarios[0].Name != "Create and query a book" {
		t.Fatalf("expected 1 scenario, got %+v", ir.Scenarios)
	}
}

func TestBuild_GoHexagonal_LowersAndEmitsTestableWorkspace(t *testing.T) {
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

action GetBookBySKU(sku: string) -> Book
  @http(method: GET, path: "/v1/books/{sku}")
{
  let book = query one Book where sku == sku else BookNotFound
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

scenario "GetBookBySKU returns existing seeded book" {
  given Book {
    id: "00000000-0000-0000-0000-000000000001",
    sku: "FDN-101",
    title: "Domain Compilers",
    price: 45
  }
  when GetBookBySKU("FDN-101")
  expect result.title == "Domain Compilers"
  expect result.price == 45
}

scenario "GetBookBySKU raises BookNotFound when missing" {
  when GetBookBySKU("MISSING-SKU")
  expect error BookNotFound
}
`
	buildProfile := `
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
    out_dir: ./dist/backend
`

	if err := os.WriteFile(filepath.Join(dir, "catalog.fdn"), []byte(blueprint), 0644); err != nil {
		t.Fatalf("write blueprint: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "foundation.build.yaml"), []byte(buildProfile), 0644); err != nil {
		t.Fatalf("write build profile: %v", err)
	}

	res, diags := compiler.Build(dir, compiler.BuildOptions{})
	if len(diags) != 0 {
		t.Fatalf("expected 0 diagnostics, got %+v", diags)
	}
	if res == nil || len(res.Architectures) != 1 {
		t.Fatalf("expected 1 lowered ArchitectureIR, got %+v", res)
	}
	arch := res.Architectures[0]
	if arch.Pattern != "hexagonal" || arch.Language != "go" {
		t.Fatalf("expected go hexagonal ArchitectureIR, got %+v", arch)
	}

	outDir := filepath.Join(dir, "dist", "backend")
	expectedFiles := []string{
		".foundation-manifest.json",
		"go.mod",
		"internal/domain/book_entity.go",
		"internal/domain/values.go",
		"internal/domain/errors.go",
		"internal/ports/book_repository.go",
		"internal/usecases/createbook_usecase.go",
		"internal/usecases/getbookbysku_usecase.go",
		"internal/adapters/http/router.go",
		"internal/adapters/memory/book_repository.go",
		"internal/bootstrap/container.go",
		"scenarios_test.go",
	}
	for _, rel := range expectedFiles {
		full := filepath.Join(outDir, rel)
		if _, err := os.Stat(full); err != nil {
			t.Errorf("expected emitted file %s to exist: %v", rel, err)
		}
	}

	testDiags := compiler.Test(dir)
	if len(testDiags) != 0 {
		t.Fatalf("expected emitted Go hexagonal workspace `go test ./...` to pass, got diagnostics: %+v", testDiags)
	}
}

func TestBuild_GenerationManifest_PrunesStaleAndDetectsManualConflicts(t *testing.T) {
	dir := t.TempDir()

	blueprintV1 := `
entity Book {
  id: uuid @pk
  sku: string
}

entity Author {
  id: uuid @pk
  name: string
}

action CreateBook(sku: string) -> Book {
  let b = create Book { sku: sku }
  return b
}

action CreateAuthor(name: string) -> Author {
  let a = create Author { name: name }
  return a
}
`
	buildProfile := `
project:
  name: library
  module: example.com/library

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
`
	fdnPath := filepath.Join(dir, "library.fdn")
	if err := os.WriteFile(fdnPath, []byte(blueprintV1), 0644); err != nil {
		t.Fatalf("write blueprint v1: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "foundation.build.yaml"), []byte(buildProfile), 0644); err != nil {
		t.Fatalf("write build profile: %v", err)
	}

	if _, diags := compiler.Build(dir, compiler.BuildOptions{}); len(diags) != 0 {
		t.Fatalf("v1 build failed: %+v", diags)
	}

	outDir := filepath.Join(dir, "dist", "backend")
	authorEntityFile := filepath.Join(outDir, "internal/domain/author_entity.go")
	authorUseCaseFile := filepath.Join(outDir, "internal/usecases/createauthor_usecase.go")
	if _, err := os.Stat(authorEntityFile); err != nil {
		t.Fatalf("expected author_entity.go in v1 build: %v", err)
	}
	if _, err := os.Stat(authorUseCaseFile); err != nil {
		t.Fatalf("expected createauthor_usecase.go in v1 build: %v", err)
	}

	blueprintV2 := `
entity Book {
  id: uuid @pk
  sku: string
}

action CreateBook(sku: string) -> Book {
  let b = create Book { sku: sku }
  return b
}
`
	if err := os.WriteFile(fdnPath, []byte(blueprintV2), 0644); err != nil {
		t.Fatalf("write blueprint v2: %v", err)
	}

	if _, diags := compiler.Build(dir, compiler.BuildOptions{}); len(diags) != 0 {
		t.Fatalf("v2 build failed: %+v", diags)
	}

	if _, err := os.Stat(authorEntityFile); !os.IsNotExist(err) {
		t.Errorf("expected stale file author_entity.go to be pruned, got err=%v", err)
	}
	if _, err := os.Stat(authorUseCaseFile); !os.IsNotExist(err) {
		t.Errorf("expected stale file createauthor_usecase.go to be pruned, got err=%v", err)
	}

	bookEntityFile := filepath.Join(outDir, "internal/domain/book_entity.go")
	if err := os.WriteFile(bookEntityFile, []byte("// manual human edit\npackage domain\n"), 0644); err != nil {
		t.Fatalf("tamper with book_entity.go: %v", err)
	}

	_, conflictDiags := compiler.Build(dir, compiler.BuildOptions{Force: false})
	if len(conflictDiags) == 0 {
		t.Fatal("expected conflict diagnostic when tracked file was manually edited, got none")
	}
	if conflictDiags[0].Code != "FDN0402" {
		t.Fatalf("expected diagnostic code FDN0402, got %+v", conflictDiags)
	}

	if _, forceDiags := compiler.Build(dir, compiler.BuildOptions{Force: true}); len(forceDiags) != 0 {
		t.Fatalf("expected forced build to succeed, got %+v", forceDiags)
	}
}
