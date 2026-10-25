package compiler_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/galihanggara68/foundation/pkg/compiler"
)

func TestBuild_TableTraitAndPluralization(t *testing.T) {
	dir := t.TempDir()

	bp := `project:
  name: store
  module: example.com/store

storage:
  dialect: postgres
  mode: both

targets:
  - name: backend
    kind: backend
    language: go
    architecture: hexagonal
    http: stdlib
    out_dir: gen
`
	if err := os.WriteFile(filepath.Join(dir, "foundation.build.yaml"), []byte(bp), 0644); err != nil {
		t.Fatalf("write build profile: %v", err)
	}

	fdn := `
entity Category {
  id: uuid @pk
  name: string
}

@table("legacy_order_tbl")
entity Order {
  id: uuid @pk
  category_id: uuid @fk(Category.id)
  total: int
}

entity OrderItem {
  id: uuid @pk
  order_id: uuid @fk(Order.id)
}
`
	if err := os.WriteFile(filepath.Join(dir, "store.fdn"), []byte(fdn), 0644); err != nil {
		t.Fatalf("write blueprint: %v", err)
	}

	res, diags := compiler.Build(dir, compiler.BuildOptions{})
	if len(diags) != 0 {
		t.Fatalf("build failed: %+v", diags)
	}
	_ = res

	schemaBytes, err := os.ReadFile(filepath.Join(dir, "gen/migrations/0001_initial.up.sql"))
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	schema := string(schemaBytes)

	// Category should pluralize to categories
	if !strings.Contains(schema, "CREATE TABLE categories (") {
		t.Fatalf("expected 'CREATE TABLE categories (', got:\n%s", schema)
	}
	// Order should use legacy_order_tbl
	if !strings.Contains(schema, "CREATE TABLE legacy_order_tbl (") {
		t.Fatalf("expected 'CREATE TABLE legacy_order_tbl (', got:\n%s", schema)
	}
	// OrderItem should pluralize to order_items and reference legacy_order_tbl(id)
	if !strings.Contains(schema, "CREATE TABLE order_items (") {
		t.Fatalf("expected 'CREATE TABLE order_items (', got:\n%s", schema)
	}
	if !strings.Contains(schema, "REFERENCES legacy_order_tbl(id)") {
		t.Fatalf("expected 'REFERENCES legacy_order_tbl(id)', got:\n%s", schema)
	}
	// Category FK reference
	if !strings.Contains(schema, "REFERENCES categories(id)") {
		t.Fatalf("expected 'REFERENCES categories(id)', got:\n%s", schema)
	}
}

func TestBuild_TableTraitRenameDiffing(t *testing.T) {
	dir := t.TempDir()

	bp := `project:
  name: store
  module: example.com/store

storage:
  dialect: postgres
  mode: both

targets:
  - name: backend
    kind: backend
    language: go
    architecture: hexagonal
    http: stdlib
    out_dir: gen
`
	if err := os.WriteFile(filepath.Join(dir, "foundation.build.yaml"), []byte(bp), 0644); err != nil {
		t.Fatalf("write build profile: %v", err)
	}

	fdnV1 := `
entity Item {
  id: uuid @pk
  title: string
}
`
	if err := os.WriteFile(filepath.Join(dir, "store.fdn"), []byte(fdnV1), 0644); err != nil {
		t.Fatalf("write blueprint v1: %v", err)
	}

	if _, diags := compiler.Build(dir, compiler.BuildOptions{}); len(diags) != 0 {
		t.Fatalf("v1 build failed: %+v", diags)
	}

	// V2: Add @table("products") to Item
	fdnV2 := `
@table("products")
entity Item {
  id: uuid @pk
  title: string
}
`
	if err := os.WriteFile(filepath.Join(dir, "store.fdn"), []byte(fdnV2), 0644); err != nil {
		t.Fatalf("write blueprint v2: %v", err)
	}

	if _, diags := compiler.Build(dir, compiler.BuildOptions{}); len(diags) != 0 {
		t.Fatalf("v2 build failed: %+v", diags)
	}

	mig2UpBytes, err := os.ReadFile(filepath.Join(dir, "gen/migrations/0002_migration.up.sql"))
	if err != nil {
		t.Fatalf("read mig2 up: %v", err)
	}
	mig2Up := string(mig2UpBytes)
	if !strings.Contains(mig2Up, "ALTER TABLE items RENAME TO products;") {
		t.Fatalf("expected 'ALTER TABLE items RENAME TO products;', got:\n%s", mig2Up)
	}
}

func TestValidate_TableDuplicateCollision(t *testing.T) {
	dir := t.TempDir()

	bp := `project:
  name: store
  module: example.com/store

storage:
  dialect: postgres
  mode: both

targets:
  - name: backend
    kind: backend
    language: go
    architecture: hexagonal
    http: stdlib
    out_dir: gen
`
	if err := os.WriteFile(filepath.Join(dir, "foundation.build.yaml"), []byte(bp), 0644); err != nil {
		t.Fatalf("write build profile: %v", err)
	}

	fdn := `
entity Item {
  id: uuid @pk
}

@table("items")
entity Product {
  id: uuid @pk
}
`
	if err := os.WriteFile(filepath.Join(dir, "store.fdn"), []byte(fdn), 0644); err != nil {
		t.Fatalf("write blueprint: %v", err)
	}

	_, diags := compiler.Check(dir)
	foundCollision := false
	for _, d := range diags {
		if d.Code == "FDN0216" && strings.Contains(d.Message, "duplicate physical table name") {
			foundCollision = true
			break
		}
	}
	if !foundCollision {
		t.Fatalf("expected FDN0216 duplicate table diagnostic, got: %+v", diags)
	}
}
