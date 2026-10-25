package compiler_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/galihanggara68/foundation/pkg/compiler"
)

func TestBuild_QueryMany_KeysetSQLAndBase64URLCursor(t *testing.T) {
	dir := t.TempDir()

	bp := `entity Product {
  id: uuid @pk
  category: string
  name: string
}

action ListProducts(category: string, cursor: string, direction: string) -> []Product
  @http(method: GET, path: "/v1/categories/{category}/products")
{
  let products = query many Product where category == category order_by id asc limit 20 cursor cursor
  return products
}
`
	if err := os.WriteFile(filepath.Join(dir, "products.fdn"), []byte(bp), 0644); err != nil {
		t.Fatalf("write blueprint: %v", err)
	}

	buildProfile := `project:
  name: test-products
  module: example.com/products

storage:
  dialect: postgres
  mode: stateful

targets:
  - name: backend
    kind: backend
    language: go
    architecture: hexagonal
    out_dir: gen
`
	if err := os.WriteFile(filepath.Join(dir, "foundation.build.yaml"), []byte(buildProfile), 0644); err != nil {
		t.Fatalf("write foundation.build.yaml: %v", err)
	}

	_, buildDiags := compiler.Build(dir, compiler.BuildOptions{})
	if len(buildDiags) > 0 {
		t.Fatalf("compiler.Build failed: %+v", buildDiags)
	}

	// 1. Verify PostgreSQL repository emits keyset seek queries and base64url cursor helpers
	pgPath := filepath.Join(dir, "gen", "internal", "adapters", "postgres", "product_repository.go")
	pgBytes, err := os.ReadFile(pgPath)
	if err != nil {
		t.Fatalf("read postgres/product_repository.go: %v", err)
	}
	pgSrc := string(pgBytes)
	if !strings.Contains(pgSrc, "id > $") || !strings.Contains(pgSrc, "ORDER BY id ASC") {
		t.Errorf("postgres product_repository.go missing expected forward keyset seek query, got:\n%s", pgSrc)
	}
	if !strings.Contains(pgSrc, "id < $") || !strings.Contains(pgSrc, "ORDER BY id DESC") {
		t.Errorf("postgres product_repository.go missing expected backward keyset seek query, got:\n%s", pgSrc)
	}
	if !strings.Contains(pgSrc, "encodeCursor") || !strings.Contains(pgSrc, "decodeCursor") {
		t.Errorf("postgres product_repository.go missing expected cursor invocation, got:\n%s", pgSrc)
	}
	cursorPath := filepath.Join(dir, "gen", "internal", "adapters", "postgres", "cursor.go")
	cursorBytes, err := os.ReadFile(cursorPath)
	if err != nil {
		t.Fatalf("read postgres/cursor.go: %v", err)
	}
	if !strings.Contains(string(cursorBytes), "encodeCursor") || !strings.Contains(string(cursorBytes), "base64.RawURLEncoding") {
		t.Errorf("postgres cursor.go missing expected base64url cursor encoding, got:\n%s", string(cursorBytes))
	}

	// 2. Verify In-Memory repository emits forward/backward keyset seek
	memPath := filepath.Join(dir, "gen", "internal", "adapters", "memory", "product_repository.go")
	memBytes, err := os.ReadFile(memPath)
	if err != nil {
		t.Fatalf("read memory/product_repository.go: %v", err)
	}
	memSrc := string(memBytes)
	if !strings.Contains(memSrc, "direction == \"prev\"") || !strings.Contains(memSrc, "seeked = seeked[:limit]") {
		t.Errorf("memory product_repository.go missing expected bidirectional keyset slicing, got:\n%s", memSrc)
	}

	// 3. Verify HTTP route extracts query parameters for GET
	httpPath := filepath.Join(dir, "gen", "internal", "adapters", "http", "router.go")
	httpBytes, err := os.ReadFile(httpPath)
	if err != nil {
		t.Fatalf("read http/router.go: %v", err)
	}
	httpSrc := string(httpBytes)
	if !strings.Contains(httpSrc, "r.URL.Query().Get(\"cursor\")") {
		t.Errorf("http router.go missing cursor query param binding, got:\n%s", httpSrc)
	}
	if !strings.Contains(httpSrc, "r.URL.Query().Get(\"direction\")") {
		t.Errorf("http router.go missing direction query param binding, got:\n%s", httpSrc)
	}
}

func TestBuild_QueryMany_TypeScriptClean(t *testing.T) {
	dir := t.TempDir()

	bp := `entity Product {
  id: uuid @pk
  category: string
  name: string
}

action ListProducts(category: string, cursor: string, direction: string) -> []Product
  @http(method: GET, path: "/v1/categories/{category}/products")
{
  let products = query many Product where category == category order_by id asc limit 20 cursor cursor
  return products
}
`
	if err := os.WriteFile(filepath.Join(dir, "products.fdn"), []byte(bp), 0644); err != nil {
		t.Fatalf("write blueprint: %v", err)
	}

	buildProfile := `project:
  name: test-products-ts
  module: "@products/backend"

storage:
  dialect: postgres
  mode: stateful

targets:
  - name: backend
    kind: backend
    language: typescript
    architecture: clean
    out_dir: gen
`
	if err := os.WriteFile(filepath.Join(dir, "foundation.build.yaml"), []byte(buildProfile), 0644); err != nil {
		t.Fatalf("write foundation.build.yaml: %v", err)
	}

	_, buildDiags := compiler.Build(dir, compiler.BuildOptions{})
	if len(buildDiags) > 0 {
		t.Fatalf("compiler.Build failed: %+v", buildDiags)
	}

	// 1. Verify ports interface declares PageInfo return
	portsPath := filepath.Join(dir, "gen", "src", "usecases", "ports.ts")
	portsBytes, err := os.ReadFile(portsPath)
	if err != nil {
		t.Fatalf("read ports.ts: %v", err)
	}
	portsSrc := string(portsBytes)
	if !strings.Contains(portsSrc, "findManyByCategory") || !strings.Contains(portsSrc, "page_info: domain.PageInfo") {
		t.Errorf("ports.ts missing expected findManyByCategory method, got:\n%s", portsSrc)
	}

	// 2. Verify postgres gateway emits base64url keyset seek
	pgPath := filepath.Join(dir, "gen", "src", "gateways", "postgres_repositories.ts")
	pgBytes, err := os.ReadFile(pgPath)
	if err != nil {
		t.Fatalf("read postgres_repositories.ts: %v", err)
	}
	pgSrc := string(pgBytes)
	if !strings.Contains(pgSrc, "encodeCursor") || !strings.Contains(pgSrc, "base64url") {
		t.Errorf("postgres_repositories.ts missing expected cursor encoding, got:\n%s", pgSrc)
	}
	if !strings.Contains(pgSrc, "ORDER BY id ASC") || !strings.Contains(pgSrc, "ORDER BY id DESC") {
		t.Errorf("postgres_repositories.ts missing expected seek queries, got:\n%s", pgSrc)
	}

	// 3. Verify Express controller extracts query params
	ctrlPath := filepath.Join(dir, "gen", "src", "controllers", "express_router.ts")
	ctrlBytes, err := os.ReadFile(ctrlPath)
	if err != nil {
		t.Fatalf("read express_router.ts: %v", err)
	}
	ctrlSrc := string(ctrlBytes)
	if !strings.Contains(ctrlSrc, "req.query[\"cursor\"]") {
		t.Errorf("express_router.ts missing query param extraction, got:\n%s", ctrlSrc)
	}
}

func TestTest_QueryMany_HermeticBidirectionalPagination(t *testing.T) {
	dir := t.TempDir()

	bp := `entity Item {
  id: uuid @pk
  name: string
}

action ListItems(cursor: string, direction: string) -> []Item {
  let items = query many Item order_by id asc limit 2 cursor cursor
  return items
}

scenario "Page 1 retrieves first two items with has_next true" {
  given Item with {
    id: "00000000-0000-0000-0000-000000000001",
    name: "Alpha"
  }
  given Item with {
    id: "00000000-0000-0000-0000-000000000002",
    name: "Beta"
  }
  given Item with {
    id: "00000000-0000-0000-0000-000000000003",
    name: "Gamma"
  }
  when ListItems("", "next")
  expect result.items.count() == 2
  expect result.page_info.has_next == true
  expect result.page_info.has_prev == false
}

scenario "Page 2 retrieves remaining item with has_next false and has_prev true" {
  given Item with {
    id: "00000000-0000-0000-0000-000000000001",
    name: "Alpha"
  }
  given Item with {
    id: "00000000-0000-0000-0000-000000000002",
    name: "Beta"
  }
  given Item with {
    id: "00000000-0000-0000-0000-000000000003",
    name: "Gamma"
  }
  when ListItems("eyJpZCI6IjAwMDAwMDAwLTAwMDAtMDAwMC0wMDAwLTAwMDAwMDAwMDAwMiJ9", "next")
  expect result.items.count() == 1
  expect result.page_info.has_next == false
  expect result.page_info.has_prev == true
}

scenario "Backward pagination from item 3 retrieves earlier items with has_next true" {
  given Item with {
    id: "00000000-0000-0000-0000-000000000001",
    name: "Alpha"
  }
  given Item with {
    id: "00000000-0000-0000-0000-000000000002",
    name: "Beta"
  }
  given Item with {
    id: "00000000-0000-0000-0000-000000000003",
    name: "Gamma"
  }
  when ListItems("eyJpZCI6IjAwMDAwMDAwLTAwMDAtMDAwMC0wMDAwLTAwMDAwMDAwMDAwMyJ9", "prev")
  expect result.items.count() == 2
  expect result.page_info.has_next == true
}
`
	if err := os.WriteFile(filepath.Join(dir, "pagination.fdn"), []byte(bp), 0644); err != nil {
		t.Fatalf("write blueprint: %v", err)
	}

	buildProfile := `target:
  name: test-pagination
  kind: backend
  language: go
  architecture: hexagonal
`
	if err := os.WriteFile(filepath.Join(dir, "foundation.build.yaml"), []byte(buildProfile), 0644); err != nil {
		t.Fatalf("write foundation.build.yaml: %v", err)
	}

	_, buildDiags := compiler.Build(dir, compiler.BuildOptions{})
	if len(buildDiags) > 0 {
		t.Fatalf("compiler.Build failed: %+v", buildDiags)
	}

	testDiags := compiler.Test(dir)
	if len(testDiags) > 0 {
		t.Fatalf("compiler.Test failed on keyset pagination scenario: %+v", testDiags)
	}
}
