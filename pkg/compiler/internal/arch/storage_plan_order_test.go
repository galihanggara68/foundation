package arch_test

import (
	"testing"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/arch"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
)

// TestBuildStoragePlan_TablesSortedByPhysicalName confirms storage tables are ordered by physical
// table name, not by entity declaration order.
func TestBuildStoragePlan_TablesSortedByPhysicalName(t *testing.T) {
	dir := t.TempDir()
	sem := &ir.SemanticIR{
		Entities: []ir.EntityDecl{
			{Name: "Zebra", TableName: "accounts"},
			{Name: "Alpha", TableName: "zebras"},
		},
	}
	bp := &arch.BuildProfile{}

	plan, _, diags := arch.BuildStoragePlan(dir, sem, bp, false)
	if len(diags) > 0 {
		t.Fatalf("unexpected diagnostics: %+v", diags)
	}

	names := make([]string, 0, len(plan.Tables))
	for _, tbl := range plan.Tables {
		names = append(names, tbl.Name)
	}
	if len(names) != 2 || names[0] != "accounts" || names[1] != "zebras" {
		t.Fatalf("expected tables sorted by physical name, got: %v", names)
	}
}
