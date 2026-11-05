package ir_test

import (
	"testing"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
)

// TestCanonicalize_SortsTopLevelSlices confirms top-level declaration slices are sorted by name.
func TestCanonicalize_SortsTopLevelSlices(t *testing.T) {
	sem := &ir.SemanticIR{
		Entities: []ir.EntityDecl{{Name: "Zebra"}, {Name: "Alpha"}},
		Actions:  []ir.ActionDecl{{Name: "Zeta"}, {Name: "Alpha"}},
		Slots:    []ir.SlotDecl{{Name: "ZSlot"}, {Name: "ASlot"}},
		Enums:    []ir.EnumDecl{{Name: "ZEnum"}, {Name: "AEnum"}},
	}

	sem.Canonicalize()

	if sem.Entities[0].Name != "Alpha" {
		t.Errorf("entities not sorted: %+v", sem.Entities)
	}
	if sem.Actions[0].Name != "Alpha" {
		t.Errorf("actions not sorted: %+v", sem.Actions)
	}
	if sem.Slots[0].Name != "ASlot" {
		t.Errorf("slots not sorted: %+v", sem.Slots)
	}
	if sem.Enums[0].Name != "AEnum" {
		t.Errorf("enums not sorted: %+v", sem.Enums)
	}
}
