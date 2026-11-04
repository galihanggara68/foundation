package frontend_test

import (
	"testing"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/frontend"
)

func TestParseFile_NamespacedTrait_ParsesSuccessfully(t *testing.T) {
	src := `
@acme:cached(ttl: 60)
entity Product {
  id: uuid @pk
  name: string
}
`
	sem, diags := frontend.ParseFile("test.fdn", src)
	for _, d := range diags {
		t.Errorf("unexpected diagnostic: %s: %s", d.Code, d.Message)
	}
	if len(sem.Entities) != 1 {
		t.Fatalf("expected 1 entity, got %d", len(sem.Entities))
	}
	entity := sem.Entities[0]
	if len(entity.Traits) != 1 {
		t.Fatalf("expected 1 trait on entity, got %d", len(entity.Traits))
	}
	tr := entity.Traits[0]
	if tr.Name != "acme:cached" {
		t.Errorf("expected trait name 'acme:cached', got %q", tr.Name)
	}
	if tr.Args["ttl"] != "60" {
		t.Errorf("expected ttl arg '60', got %q", tr.Args["ttl"])
	}
}

func TestParseFile_ReservedNamespace_ReturnsFDN0225(t *testing.T) {
	src := `
@foundation:custom
entity Product {
  id: uuid @pk
}
`
	_, diags := frontend.ParseFile("test.fdn", src)
	var found bool
	for _, d := range diags {
		if d.Code == "FDN0225" {
			found = true
			if d.Severity != "error" {
				t.Errorf("expected FDN0225 severity 'error', got %q", d.Severity)
			}
		}
	}
	if !found {
		t.Fatalf("expected diagnostic code FDN0225 for reserved 'foundation' namespace, got: %+v", diags)
	}
}

func TestParseFile_TrailingColonInTrait_ReportsError(t *testing.T) {
	src := `
@acme:
entity Product {
  id: uuid @pk
}
`
	_, diags := frontend.ParseFile("test.fdn", src)
	if len(diags) == 0 {
		t.Fatalf("expected error for incomplete namespaced trait '@acme:', got 0")
	}
}

func TestParseFile_NamespacedTraitsOnAllHosts(t *testing.T) {
	src := `
@acme:entity
entity Product {
  id: uuid @pk @acme:field(column: "prod_id")
}

@acme:action
action CreateProduct() -> int {
  return 1
}

slot Inventory {
  @acme:method
  CheckStock() -> bool
}
`
	sem, diags := frontend.ParseFile("test.fdn", src)
	for _, d := range diags {
		t.Errorf("unexpected diagnostic: %s: %s", d.Code, d.Message)
	}
	if len(sem.Entities) != 1 || len(sem.Entities[0].Traits) != 1 || sem.Entities[0].Traits[0].Name != "acme:entity" {
		t.Errorf("expected entity trait acme:entity")
	}
	if len(sem.Entities[0].Fields[0].Traits) != 2 || sem.Entities[0].Fields[0].Traits[1].Name != "acme:field" {
		t.Errorf("expected field trait acme:field")
	}
	if len(sem.Actions) != 1 || len(sem.Actions[0].Traits) != 1 || sem.Actions[0].Traits[0].Name != "acme:action" {
		t.Errorf("expected action trait acme:action")
	}
	if len(sem.Slots) != 1 || len(sem.Slots[0].Methods) != 1 || len(sem.Slots[0].Methods[0].Traits) != 1 || sem.Slots[0].Methods[0].Traits[0].Name != "acme:method" {
		t.Errorf("expected slot method trait acme:method")
	}
}

func TestParseTrait_PositionalArgs_NormalizeToParamNames(t *testing.T) {
	src := `
entity Product {
  id: uuid @pk
  name: string @len(3, 30)
  quantity: int @range(1, 100)
  short_code: string @len(5)
  custom: string @len(min: 2, max: 20)
  extra: string @len(1, 10, 20)
}
`
	sem, diags := frontend.ParseFile("test.fdn", src)
	for _, d := range diags {
		t.Errorf("unexpected diagnostic: %s: %s", d.Code, d.Message)
	}
	if len(sem.Entities) != 1 {
		t.Fatalf("expected 1 entity, got %d", len(sem.Entities))
	}
	fields := sem.Entities[0].Fields

	// name: string @len(3, 30) -> min: "3", max: "30"
	nameField := fields[1]
	if len(nameField.Traits) != 1 || nameField.Traits[0].Name != "len" {
		t.Fatalf("expected @len trait on name field")
	}
	lenTr := nameField.Traits[0]
	if lenTr.Args["min"] != "3" || lenTr.Args["max"] != "30" {
		t.Errorf("expected @len min='3' max='30', got %+v", lenTr.Args)
	}
	if _, ok := lenTr.Args["_"]; ok {
		t.Errorf("expected positional args not to populate '_', got %+v", lenTr.Args)
	}

	// quantity: int @range(1, 100) -> min: "1", max: "100"
	qtyField := fields[2]
	if len(qtyField.Traits) != 1 || qtyField.Traits[0].Name != "range" {
		t.Fatalf("expected @range trait on quantity field")
	}
	rangeTr := qtyField.Traits[0]
	if rangeTr.Args["min"] != "1" || rangeTr.Args["max"] != "100" {
		t.Errorf("expected @range min='1' max='100', got %+v", rangeTr.Args)
	}

	// short_code: string @len(5) -> min: "5"
	codeField := fields[3]
	singleTr := codeField.Traits[0]
	if singleTr.Args["min"] != "5" {
		t.Errorf("expected @len min='5', got %+v", singleTr.Args)
	}

	// custom: string @len(min: 2, max: 20) -> min: "2", max: "20"
	customField := fields[4]
	namedTr := customField.Traits[0]
	if namedTr.Args["min"] != "2" || namedTr.Args["max"] != "20" {
		t.Errorf("expected named @len min='2' max='20', got %+v", namedTr.Args)
	}

	// extra: string @len(1, 10, 20) -> min: "1", max: "10", _2: "20"
	extraField := fields[5]
	overflowTr := extraField.Traits[0]
	if overflowTr.Args["min"] != "1" || overflowTr.Args["max"] != "10" || overflowTr.Args["_2"] != "20" {
		t.Errorf("expected overflow @len min='1' max='10' _2='20', got %+v", overflowTr.Args)
	}
}

func TestParseTrait_UnregisteredTrait_PreservesDistinctKeys(t *testing.T) {
	src := `
@custom("alpha", "beta", "gamma")
@vendor:plugin("x", "y")
entity Product {
  id: uuid @pk
}
`
	sem, _ := frontend.ParseFile("test.fdn", src)
	if len(sem.Entities) != 1 || len(sem.Entities[0].Traits) != 2 {
		t.Fatalf("expected 1 entity with 2 traits")
	}

	customTr := sem.Entities[0].Traits[0]
	if customTr.Args["_0"] != "alpha" || customTr.Args["_1"] != "beta" || customTr.Args["_2"] != "gamma" {
		t.Errorf("expected distinct positional keys _0, _1, _2 on unregistered trait, got %+v", customTr.Args)
	}

	pluginTr := sem.Entities[0].Traits[1]
	if pluginTr.Args["_0"] != "x" || pluginTr.Args["_1"] != "y" {
		t.Errorf("expected distinct positional keys _0, _1 on namespaced trait, got %+v", pluginTr.Args)
	}
}

func TestParseTrait_StringUnquoting(t *testing.T) {
	src := `
@table("app_orders")
entity Order {
  id: uuid @pk
  status: string @renamed_from("order_state")
}
`
	sem, diags := frontend.ParseFile("test.fdn", src)
	for _, d := range diags {
		t.Errorf("unexpected diagnostic: %s: %s", d.Code, d.Message)
	}
	if len(sem.Entities) != 1 {
		t.Fatalf("expected 1 entity")
	}

	// @table("app_orders") -> name: "app_orders" (unquoted)
	tableTr := sem.Entities[0].Traits[0]
	if tableTr.Args["name"] != "app_orders" {
		t.Errorf("expected table name 'app_orders', got %q", tableTr.Args["name"])
	}

	// @renamed_from("order_state") -> name: "order_state" (unquoted)
	statusField := sem.Entities[0].Fields[1]
	rfTr := statusField.Traits[0]
	if rfTr.Args["name"] != "order_state" {
		t.Errorf("expected renamed_from name 'order_state', got %q", rfTr.Args["name"])
	}
}
