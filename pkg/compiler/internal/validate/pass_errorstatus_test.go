package validate_test

import (
	"testing"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/frontend"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/validate"
)

func parseAndValidate(t *testing.T, src string) (*ir.SemanticIR, []ir.Diagnostic) {
	t.Helper()
	sem, parseDiags := frontend.ParseFile("test.fdn", src)
	if len(parseDiags) > 0 {
		t.Fatalf("unexpected parse diags: %+v", parseDiags)
	}
	return sem, validate.Validate(sem)
}

func hasCode(diags []ir.Diagnostic, code string) bool {
	for _, d := range diags {
		if d.Code == code {
			return true
		}
	}
	return false
}

func TestValidate_ErrorStatus_NotDeclared_UsesKindDefault(t *testing.T) {
	sem, diags := parseAndValidate(t, `
error ProductNotFound {
  kind: not_found
  message: "requested product does not exist"
}
`)
	if len(diags) > 0 {
		t.Fatalf("unexpected validation diags: %+v", diags)
	}
	if len(sem.Errors) != 1 {
		t.Fatalf("expected 1 error declaration, got %d", len(sem.Errors))
	}
	if got := sem.Errors[0].Status; got != 404 {
		t.Errorf("status for kind not_found: got %d, want 404 (docs/blueprints/declarations.md)", got)
	}
}

func TestValidate_ErrorStatus_Declared_OverridesKindDefault(t *testing.T) {
	sem, diags := parseAndValidate(t, `
error ProductTeapot {
  kind: not_found
  message: "requested product does not exist"
  status: 418
}
`)
	if len(diags) > 0 {
		t.Fatalf("unexpected validation diags: %+v", diags)
	}
	if got := sem.Errors[0].Status; got != 418 {
		t.Errorf("declared status must win over the kind default: got %d, want 418", got)
	}
}

func TestValidate_ErrorStatus_OutOfRange_ReturnsFDN0220(t *testing.T) {
	_, diags := parseAndValidate(t, `
error Mystery {
  kind: internal
  message: "boom"
  status: 999
}
`)
	if !hasCode(diags, "FDN0220") {
		t.Errorf("expected FDN0220 for a status outside 100-599, got %+v", diags)
	}
}

func TestValidate_ErrorStatus_NotAnInteger_ReturnsFDN0220(t *testing.T) {
	_, parseDiags := frontend.ParseFile("test.fdn", `
error Mystery {
  kind: internal
  message: "boom"
  status: 4.5
}
`)
	if !hasCode(parseDiags, "FDN0220") {
		t.Errorf("expected FDN0220 for a non-integer status, got %+v", parseDiags)
	}
}

func TestValidate_ErrorStatus_Negative_ReturnsFDN0220(t *testing.T) {
	sem, parseDiags := frontend.ParseFile("test.fdn", `
error Mystery {
  kind: internal
  message: "boom"
  status: -1
}
`)
	if len(parseDiags) > 0 {
		t.Fatalf("unexpected parse diags: %+v", parseDiags)
	}
	if diags := validate.Validate(sem); !hasCode(diags, "FDN0220") {
		t.Errorf("expected FDN0220 for a negative status, got %+v", diags)
	}
}

func TestValidate_ErrorPosture_NotDeclared_UsesKindDefault(t *testing.T) {
	sem, diags := parseAndValidate(t, `
error ProductNotFound {
  kind: not_found
  message: "requested product does not exist"
}

error Boom {
  kind: internal
  message: "database connection reset"
}
`)
	if len(diags) > 0 {
		t.Fatalf("unexpected validation diags: %+v", diags)
	}

	got := map[string]ir.ErrorPosture{}
	for _, e := range sem.Errors {
		got[e.Name] = e.Posture
	}
	if got["ProductNotFound"] != ir.PostureBusiness {
		t.Errorf("not_found is the caller's problem, so it is business: got %q", got["ProductNotFound"])
	}
	if got["Boom"] != ir.PostureTechnical {
		t.Errorf("internal is our problem, so it is technical: got %q", got["Boom"])
	}
}

func TestValidate_ErrorPosture_Declared_OverridesKindDefault(t *testing.T) {
	sem, diags := parseAndValidate(t, `
error ProductGone {
  kind: not_found
  message: "requested product does not exist"
  posture: technical
}
`)
	if len(diags) > 0 {
		t.Fatalf("unexpected validation diags: %+v", diags)
	}
	if got := sem.Errors[0].Posture; got != ir.PostureTechnical {
		t.Errorf("declared posture must win over the kind default: got %q, want technical", got)
	}
}

func TestValidate_ErrorPosture_NotStandard_ReturnsFDN0221(t *testing.T) {
	_, diags := parseAndValidate(t, `
error Mystery {
  kind: not_found
  message: "boom"
  posture: whatever
}
`)
	if !hasCode(diags, "FDN0221") {
		t.Errorf("expected FDN0221 for a posture outside the two the language defines, got %+v", diags)
	}
}

func TestValidate_ErrorPosture_InvalidKind_DoesNotAlsoReportPosture(t *testing.T) {
	_, diags := parseAndValidate(t, `
error Mystery {
  kind: bogus
  message: "boom"
  posture: whatever
}
`)
	if !hasCode(diags, "FDN0201") {
		t.Errorf("expected FDN0201 for the invalid kind, got %+v", diags)
	}
	if hasCode(diags, "FDN0221") {
		t.Errorf("expected one diagnostic per mistake, but the posture was reported too: %+v", diags)
	}
}

func TestValidate_ErrorKind_NotInVocabulary_ReturnsFDN0201(t *testing.T) {
	_, diags := parseAndValidate(t, `
error Mystery {
  kind: bogus
  message: "boom"
}
`)
	if !hasCode(diags, "FDN0201") {
		t.Errorf("expected FDN0201 for a kind outside the standard vocabulary, got %+v", diags)
	}
}
