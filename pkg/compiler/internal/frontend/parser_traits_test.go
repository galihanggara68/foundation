package frontend_test

import (
	"testing"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/frontend"
)

func TestParseFile_NamespacedTrait_ParsesSuccessfully(t *testing.T) {
	src := `
@acme:cached(ttl: 60)
entity Product {
  id: uuid @pk
  name: string
}
`
	sem, diags := frontend.ParseFile("test.fdn", src)
	for _, d := range diags {
		t.Errorf("unexpected diagnostic: %s: %s", d.Code, d.Message)
	}
	if len(sem.Entities) != 1 {
		t.Fatalf("expected 1 entity, got %d", len(sem.Entities))
	}
	entity := sem.Entities[0]
	if len(entity.Traits) != 1 {
		t.Fatalf("expected 1 trait on entity, got %d", len(entity.Traits))
	}
	tr := entity.Traits[0]
	if tr.Name != "acme:cached" {
		t.Errorf("expected trait name 'acme:cached', got %q", tr.Name)
	}
	if tr.Args["ttl"] != "60" {
		t.Errorf("expected ttl arg '60', got %q", tr.Args["ttl"])
	}
}

func TestParseFile_ReservedNamespace_ReturnsFDN0225(t *testing.T) {
	src := `
@foundation:custom
entity Product {
  id: uuid @pk
}
`
	_, diags := frontend.ParseFile("test.fdn", src)
	var found bool
	for _, d := range diags {
		if d.Code == "FDN0225" {
			found = true
			if d.Severity != "error" {
				t.Errorf("expected FDN0225 severity 'error', got %q", d.Severity)
			}
		}
	}
	if !found {
		t.Fatalf("expected diagnostic code FDN0225 for reserved 'foundation' namespace, got: %+v", diags)
	}
}

func TestParseFile_TrailingColonInTrait_ReportsError(t *testing.T) {
	src := `
@acme:
entity Product {
  id: uuid @pk
}
`
	_, diags := frontend.ParseFile("test.fdn", src)
	if len(diags) == 0 {
		t.Fatalf("expected error for incomplete namespaced trait '@acme:', got 0")
	}
}

func TestParseFile_NamespacedTraitsOnAllHosts(t *testing.T) {
	src := `
@acme:entity
entity Product {
  id: uuid @pk @acme:field(column: "prod_id")
}

@acme:action
action CreateProduct() -> int {
  return 1
}

slot Inventory {
  @acme:method
  CheckStock() -> bool
}
`
	sem, diags := frontend.ParseFile("test.fdn", src)
	for _, d := range diags {
		t.Errorf("unexpected diagnostic: %s: %s", d.Code, d.Message)
	}
	if len(sem.Entities) != 1 || len(sem.Entities[0].Traits) != 1 || sem.Entities[0].Traits[0].Name != "acme:entity" {
		t.Errorf("expected entity trait acme:entity")
	}
	if len(sem.Entities[0].Fields[0].Traits) != 2 || sem.Entities[0].Fields[0].Traits[1].Name != "acme:field" {
		t.Errorf("expected field trait acme:field")
	}
	if len(sem.Actions) != 1 || len(sem.Actions[0].Traits) != 1 || sem.Actions[0].Traits[0].Name != "acme:action" {
		t.Errorf("expected action trait acme:action")
	}
	if len(sem.Slots) != 1 || len(sem.Slots[0].Methods) != 1 || len(sem.Slots[0].Methods[0].Traits) != 1 || sem.Slots[0].Methods[0].Traits[0].Name != "acme:method" {
		t.Errorf("expected slot method trait acme:method")
	}
}
