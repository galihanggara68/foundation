package arch_test

import (
	"testing"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/arch"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
)

func TestLower_SDKTargets(t *testing.T) {
	sem := &ir.SemanticIR{
		Enums: []ir.EnumDecl{
			{Name: "OrderStatus", Variants: []string{"pending", "confirmed"}},
		},
		Values: []ir.ValueDecl{
			{Name: "Money", Fields: []ir.FieldDecl{{Name: "amount", Type: "decimal"}, {Name: "currency", Type: "string"}}},
		},
		Entities: []ir.EntityDecl{
			{
				Name: "Order",
				Fields: []ir.FieldDecl{
					{Name: "id", Type: "uuid", Traits: []ir.TraitDecl{{Name: "pk"}}},
					{Name: "status", Type: "OrderStatus"},
				},
			},
		},
		Errors: []ir.ErrorDecl{
			{Name: "InsufficientStock", Kind: "conflict", Message: "insufficient stock for item"},
		},
		Actions: []ir.ActionDecl{
			{
				Name:       "PlaceOrder",
				ReturnType: "Order",
				Params: []ir.ParamDecl{
					{Name: "item_id", Type: "uuid"},
					{Name: "quantity", Type: "int"},
				},
				Traits: []ir.TraitDecl{
					{Name: "http", Args: map[string]string{"method": "POST", "path": "/v1/orders"}},
				},
			},
			{
				Name:       "ListOrders",
				ReturnType: "[]Order",
				Params: []ir.ParamDecl{
					{Name: "limit", Type: "int"},
					{Name: "cursor", Type: "string"},
				},
				Traits: []ir.TraitDecl{
					{Name: "http", Args: map[string]string{"method": "GET", "path": "/v1/orders"}},
				},
			},
			{
				Name:       "CleanupExpiredOrders",
				ReturnType: "void",
				Traits: []ir.TraitDecl{
					{Name: "internal"},
				},
			},
			{
				Name:       "SendOrderNotification",
				ReturnType: "void",
				Traits: []ir.TraitDecl{
					{Name: "event_handler", Args: map[string]string{"event_handler": "OrderPlaced"}},
				},
			},
		},
	}

	bp := &arch.BuildProfile{
		Project: arch.ProjectConfig{
			Name:   "shop",
			Module: "example.com/shop",
		},
	}

	// 1. Lower Go SDK Target
	goTarget := arch.TargetConfig{
		Name:     "go-sdk",
		Kind:     "sdk",
		Language: "go",
		OutDir:   "clients/go",
	}

	goArch, diags := arch.Lower(sem, bp, goTarget)
	if len(diags) > 0 {
		t.Fatalf("unexpected diagnostics lowering go-sdk: %v", diags)
	}

	if goArch.Kind != "sdk" || goArch.Language != "go" {
		t.Errorf("expected Kind=sdk Language=go, got Kind=%s Language=%s", goArch.Kind, goArch.Language)
	}
	if goArch.Pattern != "sdk" {
		t.Errorf("expected Pattern=sdk, got %s", goArch.Pattern)
	}
	if len(goArch.HTTPHandlers) != 2 {
		t.Fatalf("expected 2 public HTTP handlers, got %d", len(goArch.HTTPHandlers))
	}

	// Check PlaceOrder
	h0 := goArch.HTTPHandlers[0]
	if h0.ActionName != "PlaceOrder" || h0.Method != "POST" || h0.Path != "/v1/orders" {
		t.Errorf("unexpected handler 0: %+v", h0)
	}
	if h0.ReturnType != "Order" || h0.IsListReturning {
		t.Errorf("expected ReturnType=Order and IsListReturning=false, got %s, %v", h0.ReturnType, h0.IsListReturning)
	}

	// Check ListOrders
	h1 := goArch.HTTPHandlers[1]
	if h1.ActionName != "ListOrders" || h1.Method != "GET" || h1.Path != "/v1/orders" {
		t.Errorf("unexpected handler 1: %+v", h1)
	}
	if h1.ReturnType != "[]Order" || !h1.IsListReturning {
		t.Errorf("expected ReturnType=[]Order and IsListReturning=true, got %s, %v", h1.ReturnType, h1.IsListReturning)
	}

	// Verify no storage or ports leaked to SDK
	if len(goArch.Ports) != 0 {
		t.Errorf("expected 0 repository ports in SDK, got %d", len(goArch.Ports))
	}
	if goArch.Storage.Dialect != "" || len(goArch.Storage.Migrations) != 0 {
		t.Errorf("expected empty storage plan in SDK, got %+v", goArch.Storage)
	}

	// Verify domain contracts preserved
	if len(goArch.DomainEnums) != 1 || len(goArch.DomainValues) != 1 || len(goArch.DomainEntities) != 1 || len(goArch.DomainErrors) != 1 {
		t.Errorf("expected domain types to be preserved in SDK architecture")
	}

	// 2. Lower TypeScript SDK Target
	tsTarget := arch.TargetConfig{
		Name:     "ts-sdk",
		Kind:     "sdk",
		Language: "typescript",
		OutDir:   "clients/ts",
	}

	tsArch, diags := arch.Lower(sem, bp, tsTarget)
	if len(diags) > 0 {
		t.Fatalf("unexpected diagnostics lowering ts-sdk: %v", diags)
	}
	if tsArch.Kind != "sdk" || tsArch.Language != "typescript" {
		t.Errorf("expected Kind=sdk Language=typescript, got Kind=%s Language=%s", tsArch.Kind, tsArch.Language)
	}
	if len(tsArch.HTTPHandlers) != 2 {
		t.Fatalf("expected 2 public HTTP handlers in TS SDK, got %d", len(tsArch.HTTPHandlers))
	}

	// 3. Unsupported SDK language
	rubyTarget := arch.TargetConfig{
		Name:     "ruby-sdk",
		Kind:     "sdk",
		Language: "ruby",
		OutDir:   "clients/ruby",
	}
	_, diags = arch.Lower(sem, bp, rubyTarget)
	if len(diags) == 0 {
		t.Fatalf("expected diagnostic for unsupported SDK language ruby")
	}
	if diags[0].Code != "FDN0300" {
		t.Errorf("expected diagnostic FDN0300, got %s", diags[0].Code)
	}
}
