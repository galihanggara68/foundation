package validate_test

import (
	"testing"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/frontend"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/validate"
)

func TestValidate_FieldKind_ClassifiesDeclaredTypes(t *testing.T) {
	src := `
enum OrderStatus {
  pending
  shipped
}

value Address {
  street: string
}

value Meta {
  actor: Actor
  page: PageInfo
  lookup: LookupResult
}

event Ordered {
  order_id: uuid
}

entity Customer {
  id: uuid @pk
  name: string
  age: int
  balance: decimal
  active: bool
  created_at: timestamp
  status: OrderStatus
  home: Address
}

entity Order {
  id: uuid @pk
  customer: Customer
  last_event: Ordered
  tags: []string
}
`
	sem, parseDiags := frontend.ParseFile("test.fdn", src)
	if len(parseDiags) > 0 {
		t.Fatalf("unexpected parse diags: %+v", parseDiags)
	}

	if diags := validate.Validate(sem); len(diags) > 0 {
		t.Fatalf("unexpected validation diags: %+v", diags)
	}

	got := map[string]ir.FieldKind{}
	for _, e := range sem.Entities {
		for _, f := range e.Fields {
			got[e.Name+"."+f.Name] = f.Kind
		}
	}
	for _, v := range sem.Values {
		for _, f := range v.Fields {
			got[v.Name+"."+f.Name] = f.Kind
		}
	}
	for _, ev := range sem.Events {
		for _, f := range ev.Fields {
			got[ev.Name+"."+f.Name] = f.Kind
		}
	}

	want := map[string]ir.FieldKind{
		"Customer.id":         ir.KindUUID,
		"Customer.name":       ir.KindString,
		"Customer.age":        ir.KindInt,
		"Customer.balance":    ir.KindDecimal,
		"Customer.active":     ir.KindBool,
		"Customer.created_at": ir.KindTimestamp,
		"Customer.status":     ir.KindEnum,
		"Customer.home":       ir.KindValue,
		"Order.customer":      ir.KindEntity,
		"Order.last_event":    ir.KindEvent,
		"Order.tags":          ir.KindString,
		"Meta.actor":          ir.KindActor,
		"Meta.page":           ir.KindPage,
		"Meta.lookup":         ir.KindResult,
		"Ordered.order_id":    ir.KindUUID,
	}
	for key, wantKind := range want {
		if got[key] != wantKind {
			t.Errorf("field %s: got kind %q, want %q", key, got[key], wantKind)
		}
	}
}

func TestValidate_FieldKind_DeclaredTypeWinsOverResultSuffix(t *testing.T) {
	src := `
entity OrderResult {
  id: uuid @pk
}

entity Audit {
  id: uuid @pk
  summary: OrderResult
}
`
	sem, parseDiags := frontend.ParseFile("test.fdn", src)
	if len(parseDiags) > 0 {
		t.Fatalf("unexpected parse diags: %+v", parseDiags)
	}

	if diags := validate.Validate(sem); len(diags) > 0 {
		t.Fatalf("unexpected validation diags: %+v", diags)
	}

	found := false
	for _, e := range sem.Entities {
		for _, f := range e.Fields {
			if e.Name == "Audit" && f.Name == "summary" {
				found = true
				if f.Kind != ir.KindEntity {
					t.Fatalf("a declared entity named OrderResult must classify as %q, got %q", ir.KindEntity, f.Kind)
				}
			}
		}
	}
	if !found {
		t.Fatal("expected Audit.summary field to be present")
	}
}

func TestValidate_FieldKind_UnclassifiableFieldType_ReturnsFDN0218(t *testing.T) {
	src := `
entity Broken {
  id: uuid @pk
  mystery: strng
}
`
	sem, parseDiags := frontend.ParseFile("broken.fdn", src)
	if len(parseDiags) > 0 {
		t.Fatalf("unexpected parse diags: %+v", parseDiags)
	}

	diags := validate.Validate(sem)
	found := false
	for _, d := range diags {
		if d.Code == "FDN0218" {
			found = true
		}
		if d.Code == "FDN0202" {
			t.Errorf("field type failures must be reported as FDN0218, not the legacy FDN0202: %+v", d)
		}
	}
	if !found {
		t.Fatalf("expected FDN0218 for unclassifiable field type, got: %+v", diags)
	}
}
