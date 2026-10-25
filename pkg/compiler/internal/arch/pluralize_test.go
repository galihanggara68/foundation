package arch_test

import (
	"testing"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/arch"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
)

func TestPluralize(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"order", "orders"},
		{"user", "users"},
		{"customer", "customers"},
		{"category", "categories"},
		{"tax", "taxes"},
		{"batch", "batches"},
		{"bus", "buses"},
		{"glass", "glasses"},
		{"shelf", "shelves"},
		{"life", "lives"},
		{"day", "days"},
		{"person", "people"},
		{"orders", "orders"},
		{"categories", "categories"},
	}

	for _, tc := range cases {
		got := arch.Pluralize(tc.input)
		if got != tc.expected {
			t.Errorf("Pluralize(%q) = %q, expected %q", tc.input, got, tc.expected)
		}
	}
}

func TestEntityTableName(t *testing.T) {
	entDefault := ir.EntityDecl{Name: "OrderItem"}
	if got := arch.EntityTableName(entDefault); got != "order_items" {
		t.Fatalf("expected order_items, got %q", got)
	}

	entCustom := ir.EntityDecl{
		Name: "Customer",
		Traits: []ir.TraitDecl{
			{Name: "table", Args: map[string]string{"_": "\"legacy_customers\""}},
		},
	}
	if got := arch.EntityTableName(entCustom); got != "legacy_customers" {
		t.Fatalf("expected legacy_customers, got %q", got)
	}
}
