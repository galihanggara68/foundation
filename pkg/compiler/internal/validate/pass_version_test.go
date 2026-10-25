package validate_test

import (
	"testing"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/frontend"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/validate"
)

func TestValidate_Version_NonIntType_ReturnsFDN0205(t *testing.T) {
	src := `
entity Product {
  id: uuid @pk
  version: string @version
  name: string
}
`
	sem, parseDiags := frontend.ParseFile("test.fdn", src)
	if len(parseDiags) > 0 {
		t.Fatalf("unexpected parse diags: %+v", parseDiags)
	}

	diags := validate.Validate(sem)
	found := false
	for _, d := range diags {
		if d.Code == "FDN0205" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected diagnostic FDN0205 for non-int @version field, got: %+v", diags)
	}
}

func TestValidate_Version_MultipleFields_ReturnsFDN0215(t *testing.T) {
	src := `
entity Product {
  id: uuid @pk
  v1: int @version
  v2: int @version
}
`
	sem, parseDiags := frontend.ParseFile("test.fdn", src)
	if len(parseDiags) > 0 {
		t.Fatalf("unexpected parse diags: %+v", parseDiags)
	}

	diags := validate.Validate(sem)
	found := false
	for _, d := range diags {
		if d.Code == "FDN0215" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected diagnostic FDN0215 for multiple @version fields, got: %+v", diags)
	}
}

func TestValidate_Version_OnValueOrEvent_ReturnsFDN0215(t *testing.T) {
	src := `
value PriceSnapshot {
  amount: int
  version: int @version
}
`
	sem, parseDiags := frontend.ParseFile("test.fdn", src)
	if len(parseDiags) > 0 {
		t.Fatalf("unexpected parse diags: %+v", parseDiags)
	}

	diags := validate.Validate(sem)
	found := false
	for _, d := range diags {
		if d.Code == "FDN0215" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected diagnostic FDN0215 for @version on value, got: %+v", diags)
	}
}
