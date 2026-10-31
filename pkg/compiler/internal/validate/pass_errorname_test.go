package validate_test

import (
	"strings"
	"testing"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
)

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

// The pre-seeded pair is already reported as a duplicate declaration; the collision
// warning is about the name, so both are reported and neither replaces the other.
func TestValidate_ErrorName_PreSeededBuiltin_KeepsDuplicateAndWarns(t *testing.T) {
	_, diags := parseAndValidate(t, `
error Conflict {
  kind: conflict
  message: "my own conflict message"
}
`)

	if !hasCode(diags, "FDN0200") {
		t.Errorf("expected the pre-seeded built-in to still report a duplicate declaration, got %+v", diags)
	}
	if !hasCode(diags, "FDN0219") {
		t.Errorf("expected a reserved name to warn whether or not it is a built-in, got %+v", diags)
	}
}
