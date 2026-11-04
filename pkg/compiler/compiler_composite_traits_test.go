package compiler_test

import (
	"reflect"
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
