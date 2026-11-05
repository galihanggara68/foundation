package arch

import (
	"testing"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
)

// TestEmittableSlots_FiltersDerived confirms built-in emitters never see derived slots.
func TestEmittableSlots_FiltersDerived(t *testing.T) {
	slots := []ir.SlotDecl{
		{Name: "AuditSlot"},
		{Name: "EventDispatcher", Derived: true},
	}

	got := emittableSlots(slots)
	if len(got) != 1 || got[0].Name != "AuditSlot" {
		t.Fatalf("expected only source-declared slots, got: %+v", got)
	}
}
