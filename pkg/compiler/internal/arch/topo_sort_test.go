package arch_test

import (
	"testing"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/arch"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
)

func TestSortEntitiesByDependency(t *testing.T) {
	category := ir.EntityDecl{
		Name: "Category",
		Fields: []ir.FieldDecl{
			{Name: "id", Type: "uuid"},
		},
	}
	product := ir.EntityDecl{
		Name: "Product",
		Fields: []ir.FieldDecl{
			{Name: "id", Type: "uuid"},
			{Name: "category_id", Type: "uuid", Traits: []ir.TraitDecl{{Name: "fk", Args: map[string]string{"_": "Category.id"}}}},
		},
	}
	order := ir.EntityDecl{
		Name: "Order",
		Fields: []ir.FieldDecl{
			{Name: "id", Type: "uuid"},
			{Name: "product_id", Type: "uuid", Traits: []ir.TraitDecl{{Name: "fk", Args: map[string]string{"_": "Product.id"}}}},
		},
	}
	audit := ir.EntityDecl{
		Name: "AuditLog",
		Fields: []ir.FieldDecl{
			{Name: "id", Type: "uuid"},
		},
	}

	// Given entities in alphabetical order: AuditLog, Category, Order, Product
	input := []ir.EntityDecl{audit, category, order, product}
	sorted := arch.SortEntitiesByDependency(input)

	if len(sorted) != 4 {
		t.Fatalf("expected 4 entities, got %d", len(sorted))
	}

	nameToIndex := make(map[string]int)
	for i, e := range sorted {
		nameToIndex[e.Name] = i
	}

	if nameToIndex["Category"] > nameToIndex["Product"] {
		t.Errorf("Category (%d) must precede Product (%d)", nameToIndex["Category"], nameToIndex["Product"])
	}
	if nameToIndex["Product"] > nameToIndex["Order"] {
		t.Errorf("Product (%d) must precede Order (%d)", nameToIndex["Product"], nameToIndex["Order"])
	}
}
