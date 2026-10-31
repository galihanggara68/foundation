package validate_test

import (
	"testing"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/frontend"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/validate"
)

const paramKindSrc = `
value Address {
  city: string
}

entity Order {
  id: uuid @pk
  total: decimal
}

error NotFound {
  kind: not_found
  message: "order not found"
}

policy CanView(actor: Actor, order: Order) {
  actor.role == "admin"
}

slot Inventory {
  @external
  Reserve(order_id: uuid, quantity: int) -> Order throws NotFound
  Heartbeat()
}

action PlaceOrder(customer_id: uuid, line_totals: []decimal) -> bool {
  return true
}
`

func TestValidate_ParamKind_ClassifiesActionSlotAndPolicyParams(t *testing.T) {
	sem, parseDiags := frontend.ParseFile("test.fdn", paramKindSrc)
	if len(parseDiags) > 0 {
		t.Fatalf("unexpected parse diags: %+v", parseDiags)
	}
	if diags := validate.Validate(sem); ir.HasErrors(diags) {
		t.Fatalf("unexpected validation diags: %+v", diags)
	}

	got := map[string]ir.FieldKind{}
	for _, a := range sem.Actions {
		for _, p := range a.Params {
			got["action."+a.Name+"."+p.Name] = p.Kind
		}
	}
	for _, sl := range sem.Slots {
		for _, m := range sl.Methods {
			for _, p := range m.Params {
				got["slot."+sl.Name+"."+m.Name+"."+p.Name] = p.Kind
			}
		}
	}
	for _, pol := range sem.Policies {
		for _, p := range pol.Params {
			got["policy."+pol.Name+"."+p.Name] = p.Kind
		}
	}

	want := map[string]ir.FieldKind{
		"action.PlaceOrder.customer_id":   ir.KindUUID,
		"action.PlaceOrder.line_totals":   ir.KindDecimal,
		"slot.Inventory.Reserve.order_id": ir.KindUUID,
		"slot.Inventory.Reserve.quantity": ir.KindInt,
		"policy.CanView.actor":            ir.KindActor,
		"policy.CanView.order":            ir.KindEntity,
	}
	for key, wantKind := range want {
		if got[key] != wantKind {
			t.Errorf("param %s: got kind %q, want %q", key, got[key], wantKind)
		}
	}
}

func TestValidate_ReturnKind_ClassifiesSlotReturnAndOmitsMissingReturn(t *testing.T) {
	sem, parseDiags := frontend.ParseFile("test.fdn", paramKindSrc)
	if len(parseDiags) > 0 {
		t.Fatalf("unexpected parse diags: %+v", parseDiags)
	}
	if diags := validate.Validate(sem); ir.HasErrors(diags) {
		t.Fatalf("unexpected validation diags: %+v", diags)
	}

	got := map[string]ir.FieldKind{}
	for _, sl := range sem.Slots {
		for _, m := range sl.Methods {
			got[sl.Name+"."+m.Name] = m.ReturnKind
		}
	}

	if got["Inventory.Reserve"] != ir.KindEntity {
		t.Errorf("slot Inventory.Reserve return: got kind %q, want %q", got["Inventory.Reserve"], ir.KindEntity)
	}
	if got["Inventory.Heartbeat"] != "" {
		t.Errorf("slot method with no return must carry an empty ReturnKind, got %q", got["Inventory.Heartbeat"])
	}
}

func TestValidate_ParamKind_UnknownTypeStaysFDN0202AndLeavesKindEmpty(t *testing.T) {
	src := `
entity Order {
  id: uuid @pk
}

action Broken(mystery: strng) -> bool {
  return true
}
`
	sem, parseDiags := frontend.ParseFile("broken.fdn", src)
	if len(parseDiags) > 0 {
		t.Fatalf("unexpected parse diags: %+v", parseDiags)
	}

	diags := validate.Validate(sem)
	found := false
	for _, d := range diags {
		if d.Code == "FDN0202" {
			found = true
		}
		if d.Code == "FDN0218" {
			t.Errorf("action/slot/policy param failures must stay FDN0202, not FDN0218: %+v", d)
		}
	}
	if !found {
		t.Fatalf("expected FDN0202 for unknown action param type, got: %+v", diags)
	}

	for _, a := range sem.Actions {
		for _, p := range a.Params {
			if p.Kind != "" {
				t.Errorf("param %s with unknown type must keep an empty Kind, got %q", p.Name, p.Kind)
			}
		}
	}
}
