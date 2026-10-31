package frontend_test

import (
	"testing"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/frontend"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
)

func TestParseError_MessagesBlock_KeepsEveryDeclaredLocale(t *testing.T) {
	sem, diags := frontend.ParseFile("test.fdn", `
error ProductNotFound {
  kind: not_found
  message: "requested product does not exist"
  messages: {
    en: "requested product does not exist"
    "en-US": "requested product cannot be found"
    id: "produk yang diminta tidak ditemukan"
  }
}
`)
	if len(diags) > 0 {
		t.Fatalf("unexpected parse diags: %+v", diags)
	}

	got := sem.Errors[0].Messages
	for _, locale := range []string{"en", "en-US", "id"} {
		if _, ok := got[locale]; !ok {
			t.Errorf("expected locale %q to be recorded, got %v", locale, got)
		}
	}
	if sem.Errors[0].Message != "requested product does not exist" {
		t.Errorf("the fallback message must survive a messages block: got %q", sem.Errors[0].Message)
	}
}

func TestParseError_DuplicateLocale_ReturnsFDN0108(t *testing.T) {
	_, diags := frontend.ParseFile("test.fdn", `
error Mystery {
  kind: internal
  message: "boom"
  messages: {
    en: "first"
    en: "second"
  }
}
`)
	if !hasParseCode(diags, "FDN0108") {
		t.Errorf("expected FDN0108 for a repeated locale key, got %+v", diags)
	}
}

func hasParseCode(diags []ir.Diagnostic, code string) bool {
	for _, d := range diags {
		if d.Code == code {
			return true
		}
	}
	return false
}
