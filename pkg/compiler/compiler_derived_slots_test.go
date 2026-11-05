package compiler_test

import (
	"testing"

	"github.com/galihanggara68/foundation/pkg/compiler"
)

// TestCompile_MaterializesDerivedEventDispatcherSlot confirms that declaring events makes the
// compiler insert a derived EventDispatcher slot with one dispatch method per event.
func TestCompile_MaterializesDerivedEventDispatcherSlot(t *testing.T) {
	src := compiler.SourceFile{Path: "shop.fdn", Content: `
entity Order {
  id: uuid @pk
  total: decimal(10, 2)
}

event OrderPlaced {
  order_id: uuid
}

action PlaceOrder(total: decimal(10, 2)) -> Order
  @http(method: POST, path: "/orders")
{
  let order = create Order { total: total }
  emit OrderPlaced { order_id: order.id }
  return order
}
`}

	sem, diags := compiler.Compile([]compiler.SourceFile{src})
	if compiler.HasErrors(diags) {
		t.Fatalf("unexpected diagnostics: %+v", diags)
	}

	var dispatcher *compiler.SlotDecl
	for i := range sem.Slots {
		if sem.Slots[i].Name == "EventDispatcher" {
			dispatcher = &sem.Slots[i]
		}
	}
	if dispatcher == nil {
		t.Fatalf("expected a derived EventDispatcher slot, got: %+v", sem.Slots)
	}
	if !dispatcher.Derived {
		t.Errorf("expected the EventDispatcher slot to be derived")
	}
	if len(dispatcher.Methods) != 1 || dispatcher.Methods[0].Name != "DispatchOrderPlaced" {
		t.Fatalf("unexpected dispatcher methods: %+v", dispatcher.Methods)
	}
	if param := dispatcher.Methods[0].Params[0]; param.Name != "event" || param.Type != "OrderPlaced" {
		t.Errorf("unexpected dispatcher parameter: %+v", param)
	}
}

// TestCompile_NoEventsNoDerivedSlot confirms a blueprint without events gets no derived slot.
func TestCompile_NoEventsNoDerivedSlot(t *testing.T) {
	src := compiler.SourceFile{Path: "shop.fdn", Content: `
entity Order {
  id: uuid @pk
}

action GetOrder(id: uuid) -> Order
  @http(method: GET, path: "/orders/{id}")
{
  let order = create Order { id: id }
  return order
}
`}

	sem, diags := compiler.Compile([]compiler.SourceFile{src})
	if compiler.HasErrors(diags) {
		t.Fatalf("unexpected diagnostics: %+v", diags)
	}
	for _, sl := range sem.Slots {
		if sl.Name == "EventDispatcher" {
			t.Fatalf("did not expect a derived slot without events: %+v", sl)
		}
	}
}
