package validate_test

import (
	"testing"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/frontend"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/validate"
)

func findDiags(diags []ir.Diagnostic, code string) []ir.Diagnostic {
	var matches []ir.Diagnostic
	for _, d := range diags {
		if d.Code == code {
			matches = append(matches, d)
		}
	}
	return matches
}

func TestValidate_UnknownTrait_ReturnsFDN0224Warning(t *testing.T) {
	src := `
@unknown_trait
entity Product {
  id: uuid @pk
}
`
	sem, parseDiags := frontend.ParseFile("test.fdn", src)
	if len(parseDiags) > 0 {
		t.Fatalf("unexpected parse diagnostics: %+v", parseDiags)
	}

	diags := validate.Validate(sem)
	matches := findDiags(diags, "FDN0224")
	if len(matches) == 0 {
		t.Fatalf("expected FDN0224 warning for @unknown_trait, got none. All diags: %+v", diags)
	}
	if matches[0].Severity != ir.SeverityWarning {
		t.Errorf("expected FDN0224 to be a warning, got %s", matches[0].Severity)
	}
}

func TestValidate_NamespacedTrait_IsNeverJudged(t *testing.T) {
	src := `
@acme:unknown_or_custom(foo: "bar", num: "not_a_number")
entity Product {
  id: uuid @pk
}
`
	sem, parseDiags := frontend.ParseFile("test.fdn", src)
	if len(parseDiags) > 0 {
		t.Fatalf("unexpected parse diagnostics: %+v", parseDiags)
	}

	diags := validate.Validate(sem)
	if len(findDiags(diags, "FDN0224")) > 0 {
		t.Errorf("namespaced trait should never yield FDN0224")
	}
	if len(findDiags(diags, "FDN0226")) > 0 {
		t.Errorf("namespaced trait should never yield FDN0226")
	}
}

func TestValidate_StandardTrait_UnknownArgument_ReturnsFDN0226Warning(t *testing.T) {
	src := `
action GetItem() -> int @cache(ttl: 60, bogus: "val") {
  return 1
}
`
	sem, parseDiags := frontend.ParseFile("test.fdn", src)
	if len(parseDiags) > 0 {
		t.Fatalf("unexpected parse diagnostics: %+v", parseDiags)
	}

	diags := validate.Validate(sem)
	matches := findDiags(diags, "FDN0226")
	if len(matches) == 0 {
		t.Fatalf("expected FDN0226 warning for unknown argument 'bogus', got none: %+v", diags)
	}
	if matches[0].Severity != ir.SeverityWarning {
		t.Errorf("expected FDN0226 to be a warning, got %s", matches[0].Severity)
	}
}

func TestValidate_StandardTrait_NonNumericArgument_ReturnsFDN0226Warning(t *testing.T) {
	src := `
action GetItem() -> int @cache(ttl: "sixty") {
  return 1
}
`
	sem, parseDiags := frontend.ParseFile("test.fdn", src)
	if len(parseDiags) > 0 {
		t.Fatalf("unexpected parse diagnostics: %+v", parseDiags)
	}

	diags := validate.Validate(sem)
	matches := findDiags(diags, "FDN0226")
	if len(matches) == 0 {
		t.Fatalf("expected FDN0226 warning for non-numeric ttl, got none: %+v", diags)
	}
}

func TestValidate_StandardTrait_MissingRequiredArgument_ReturnsFDN0226Warning(t *testing.T) {
	src := `
action Handle() -> int @event_handler {
  return 1
}
`
	sem, parseDiags := frontend.ParseFile("test.fdn", src)
	if len(parseDiags) > 0 {
		t.Fatalf("unexpected parse diagnostics: %+v", parseDiags)
	}

	diags := validate.Validate(sem)
	matches := findDiags(diags, "FDN0226")
	if len(matches) == 0 {
		t.Fatalf("expected FDN0226 warning for missing required 'event' argument, got none: %+v", diags)
	}
}
