package lsp

import (
	"strings"
	"testing"
)

func TestComputeHover_Traits(t *testing.T) {
	doc := `entity Order @table(name = "orders") {
  id: uuid @pk
  version: int @version
}`

	// 1. Hover on '@table' (Line 0, Col 15)
	h1 := ComputeHover(doc, Position{Line: 0, Character: 15})
	if h1 == nil {
		t.Fatalf("expected hover for @table, got nil")
	}
	if !strings.Contains(h1.Contents.Value, "@table") || !strings.Contains(h1.Contents.Value, "relational SQL") {
		t.Fatalf("expected @table doc, got: %s", h1.Contents.Value)
	}

	// 2. Hover on '@version' (Line 2, Col 17)
	h2 := ComputeHover(doc, Position{Line: 2, Character: 17})
	if h2 == nil {
		t.Fatalf("expected hover for @version, got nil")
	}
	if !strings.Contains(h2.Contents.Value, "optimistic concurrency control") {
		t.Fatalf("expected @version OCC doc, got: %s", h2.Contents.Value)
	}

	// 3. Hover on '@route'
	routeDoc := `action CreateItem() -> Item @route(post = "/items") { return item }`
	h3 := ComputeHover(routeDoc, Position{Line: 0, Character: 32})
	if h3 == nil {
		t.Fatalf("expected hover for @route, got nil")
	}
	if !strings.Contains(h3.Contents.Value, "@route") || !strings.Contains(h3.Contents.Value, "HTTP API") {
		t.Fatalf("expected @route doc, got: %s", h3.Contents.Value)
	}
}

func TestComputeHover_TypesAndKeywords(t *testing.T) {
	doc := `entity Customer {
  id: uuid
  balance: decimal
}`

	// 1. Hover on 'UUID' (uppercase lookup)
	hUUID := ComputeHover("entity Customer { id: UUID }", Position{Line: 0, Character: 22})
	if hUUID == nil {
		t.Fatalf("expected hover for uppercase UUID, got nil")
	}
	if !strings.Contains(hUUID.Contents.Value, "128-bit") {
		t.Fatalf("expected UUID doc, got: %s", hUUID.Contents.Value)
	}

	// 2. Hover on 'Money'
	hMoney := ComputeHover("entity Account { balance: Money }", Position{Line: 0, Character: 27})
	if hMoney == nil {
		t.Fatalf("expected hover for Money, got nil")
	}
	if !strings.Contains(hMoney.Contents.Value, "Monetary amount") {
		t.Fatalf("expected Money doc, got: %s", hMoney.Contents.Value)
	}

	// 2. Hover on 'entity' (Line 0, Col 2)
	hEntity := ComputeHover(doc, Position{Line: 0, Character: 2})
	if hEntity == nil {
		t.Fatalf("expected hover for entity keyword, got nil")
	}
	if !strings.Contains(hEntity.Contents.Value, "domain aggregate") {
		t.Fatalf("expected entity doc, got: %s", hEntity.Contents.Value)
	}

	// 3. Hover on 'slot'
	hSlot := ComputeHover("slot ShippingCalculator {}", Position{Line: 0, Character: 2})
	if hSlot == nil {
		t.Fatalf("expected hover for slot keyword, got nil")
	}
	if !strings.Contains(hSlot.Contents.Value, "outbound integration port") {
		t.Fatalf("expected slot doc, got: %s", hSlot.Contents.Value)
	}

	// 4. Hover on whitespace (Line 0, Col 6) -> nil
	hWhitespace := ComputeHover(doc, Position{Line: 0, Character: 6})
	if hWhitespace != nil {
		t.Fatalf("expected nil for whitespace, got: %+v", hWhitespace)
	}
}
