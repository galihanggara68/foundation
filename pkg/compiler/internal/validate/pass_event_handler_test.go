package validate_test

import (
	"testing"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/frontend"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/validate"
)

func TestValidate_EventHandler_UndeclaredEvent_ReturnsFDN0203(t *testing.T) {
	src := `
event OrderPlaced {
  id: uuid
}

action HandleUnknown(ev: OrderPlaced) -> bool
  @event_handler(NonExistentEvent)
{
  return true
}
`
	sem, parseDiags := frontend.ParseFile("test.fdn", src)
	if len(parseDiags) > 0 {
		t.Fatalf("unexpected parse diags: %+v", parseDiags)
	}

	sym := validate.NewSymbolTable()
	diags := validate.DeclarationPass{}.Validate(sem, sym)
	found := false
	for _, d := range diags {
		if d.Code == "FDN0203" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected diagnostic FDN0203 for undeclared event, got: %+v", diags)
	}
}

func TestValidate_EventHandler_MissingParam_ReturnsFDN0204(t *testing.T) {
	src := `
event OrderPlaced {
  id: uuid
}

action HandleOrder() -> bool
  @event_handler(OrderPlaced)
{
  return true
}
`
	sem, parseDiags := frontend.ParseFile("test.fdn", src)
	if len(parseDiags) > 0 {
		t.Fatalf("unexpected parse diags: %+v", parseDiags)
	}

	sym := validate.NewSymbolTable()
	diags := validate.DeclarationPass{}.Validate(sem, sym)
	found := false
	for _, d := range diags {
		if d.Code == "FDN0204" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected diagnostic FDN0204 for missing event parameter, got: %+v", diags)
	}
}
