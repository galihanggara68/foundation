package validate_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
)

func findErrorSeverity(diags []ir.Diagnostic) *ir.Diagnostic {
	for i := range diags {
		if diags[i].Severity == ir.SeverityError {
			return &diags[i]
		}
	}
	return nil
}

func findCode(diags []ir.Diagnostic, code string) *ir.Diagnostic {
	for i := range diags {
		if diags[i].Code == code {
			return &diags[i]
		}
	}
	return nil
}

func TestValidate_ErrorName_ReservedByVocabulary_WarnsFDN0219(t *testing.T) {
	_, diags := parseAndValidate(t, `
error NotFound {
  kind: not_found
  message: "no such item"
}
`)

	warn := findCode(diags, "FDN0219")
	if warn == nil {
		t.Fatalf("expected FDN0219 for a name the vocabulary reserves, got %+v", diags)
	}
	if warn.Severity != ir.SeverityWarning {
		t.Errorf("a collision is a warning — the declaration stays legal — got severity %q", warn.Severity)
	}
	for _, want := range []string{`"NotFound"`, "not_found"} {
		if !strings.Contains(warn.Message, want) {
			t.Errorf("expected the warning to name %s, got %q", want, warn.Message)
		}
	}
}

func TestValidate_ErrorName_OutsideVocabulary_NoCollisionWarning(t *testing.T) {
	_, diags := parseAndValidate(t, `
error ItemNotFound {
  kind: not_found
  message: "no such item"
}
`)

	if hasCode(diags, "FDN0219") {
		t.Errorf("a name the vocabulary does not own must not warn, got %+v", diags)
	}
}

// Every name the vocabulary owns is legal: a collision is warned about, never fatal. That
// must hold whether or not the platform ships the error itself — a shipped built-in keeps
// the name and shadows the declaration instead of the declaration re-declaring the
// built-in, which is what made one reserved name behave differently from another.
func TestValidate_ErrorName_ReservedNames_AreLegalAndWarned(t *testing.T) {
	cases := []struct {
		name string
		kind string
	}{
		{name: "Conflict", kind: "conflict"},               // reserved and shipped: already pre-seeded
		{name: "Unauthenticated", kind: "unauthenticated"}, // reserved and shipped
		{name: "Validation", kind: "validation"},           // reserved but not shipped
		{name: "Internal", kind: "internal"},               // reserved but not shipped
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, diags := parseAndValidate(t, fmt.Sprintf(`
error %s {
  kind: %s
  message: "my own message"
}
`, tc.name, tc.kind))

			warn := findCode(diags, "FDN0219")
			if warn == nil {
				t.Fatalf("expected a reserved name to warn, got %+v", diags)
			}
			if warn.Severity != ir.SeverityWarning {
				t.Errorf("a collision is a warning — the declaration stays legal — got severity %q", warn.Severity)
			}
			if hasCode(diags, "FDN0200") {
				t.Errorf("a reserved name must not be a duplicate declaration; the built-in keeps the symbol and shadows the declaration, got %+v", diags)
			}
			if bad := findErrorSeverity(diags); bad != nil {
				t.Errorf("a reserved name must not abort the build, got error %s: %s", bad.Code, bad.Message)
			}
		})
	}
}
