package compiler_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/galihanggara68/foundation/pkg/compiler"
)

func TestCompile_CompositeTraits_SucceedsAndPreservesIR(t *testing.T) {
	sources := []compiler.SourceFile{
		{
			Path: "orders.fdn",
			Content: `
entity Order @index(fields: ["customer_id", "status"], name: "idx_cust_status") @unique(fields: ["tenant_id", "order_number"]) {
  id: uuid @pk
  customer_id: uuid
  status: string
  tenant_id: uuid
  order_number: string
}
`,
		},
	}

	sem, diags := compiler.Compile(sources)
	if compiler.HasErrors(diags) {
		t.Fatalf("unexpected compile errors: %+v", diags)
	}
	if len(diags) > 0 {
		t.Fatalf("expected 0 diagnostics, got: %+v", diags)
	}
	if sem == nil {
		t.Fatalf("expected non-nil SemanticIR")
	}

	if len(sem.Entities) != 1 {
		t.Fatalf("expected 1 entity, got %d", len(sem.Entities))
	}

	order := sem.Entities[0]
	if len(order.Traits) != 2 {
		t.Fatalf("expected 2 traits on Order entity, got %d: %+v", len(order.Traits), order.Traits)
	}

	// Verify @index trait
	idxTrait := order.Traits[0]
	if idxTrait.Name != "index" {
		t.Errorf("expected first trait 'index', got %q", idxTrait.Name)
	}
	idxFields, ok := idxTrait.GetListArg("fields")
	if !ok || !reflect.DeepEqual(idxFields, []string{"customer_id", "status"}) {
		t.Errorf("expected @index fields ['customer_id', 'status'], got %+v (ok=%v)", idxFields, ok)
	}
	idxName, ok := idxTrait.GetStringArg("name")
	if !ok || idxName != "idx_cust_status" {
		t.Errorf("expected @index name 'idx_cust_status', got %q (ok=%v)", idxName, ok)
	}

	// Verify @unique trait
	uniqTrait := order.Traits[1]
	if uniqTrait.Name != "unique" {
		t.Errorf("expected second trait 'unique', got %q", uniqTrait.Name)
	}
	uniqFields, ok := uniqTrait.GetListArg("fields")
	if !ok || !reflect.DeepEqual(uniqFields, []string{"tenant_id", "order_number"}) {
		t.Errorf("expected @unique fields ['tenant_id', 'order_number'], got %+v (ok=%v)", uniqFields, ok)
	}
}

func TestBuild_CompositeTraits_EmitsSQL(t *testing.T) {
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
entity Member @unique(fields: ["organization_id", "email"]) {
  id: uuid @pk
  organization_id: uuid
  email: string
}

entity AuditLog @index(fields: ["tenant_id", "created_at"], name: "idx_custom_audit") {
  id: uuid @pk
  tenant_id: uuid
  created_at: timestamp
}
`
	if err := os.WriteFile(filepath.Join(dir, "models.fdn"), []byte(fdn), 0644); err != nil {
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

	// Verify composite unique constraint (Spec 07 Scenario 4)
	expectedConstraint := "CONSTRAINT uq_members_organization_id_email UNIQUE (organization_id, email)"
	if !strings.Contains(schema, expectedConstraint) {
		t.Errorf("expected migration to contain %q, got:\n%s", expectedConstraint, schema)
	}

	// Verify composite index (Spec 07 Scenario 5)
	expectedIndex := "CREATE INDEX idx_custom_audit ON audit_logs (tenant_id, created_at);\n"
	if !strings.Contains(schema, expectedIndex) {
		t.Errorf("expected migration to contain %q, got:\n%s", expectedIndex, schema)
	}
}
