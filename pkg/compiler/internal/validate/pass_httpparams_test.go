package validate_test

import (
	"strings"
	"testing"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
)

const previewBlueprintPrelude = `
error PreviewRowNotFound {
  kind: not_found
  message: "requested preview row does not exist"
}

entity PreviewRow {
  id: uuid @pk
  name: string
}
`

func httpParamDiags(diags []ir.Diagnostic) []ir.Diagnostic {
	var out []ir.Diagnostic
	for _, d := range diags {
		if d.Code == "FDN0222" {
			out = append(out, d)
		}
	}
	return out
}

func TestValidate_HTTPPathParam_SegmentSuppliedByParameter_NoFDN0222(t *testing.T) {
	_, diags := parseAndValidate(t, previewBlueprintPrelude+`
action PreviewTable(id: uuid, table_name: string) -> PreviewRow @http(method: "GET", path: "/v1/tables/{id}/preview") {
  let row = query one PreviewRow where id == id else PreviewRowNotFound
  return row
}
`)
	if hasCode(diags, "FDN0222") {
		t.Errorf("a path segment the action parameter supplies is routable and readable, got %+v", diags)
	}
}

func TestValidate_HTTPPathParam_SegmentNotSupplied_ReturnsFDN0222(t *testing.T) {
	_, diags := parseAndValidate(t, previewBlueprintPrelude+`
action PreviewTable(foo: uuid, table_name: string) -> PreviewRow @http(method: "GET", path: "/v1/tables/{id}/preview") {
  let row = query one PreviewRow where id == foo else PreviewRowNotFound
  return row
}
`)
	got := httpParamDiags(diags)
	if len(got) != 1 {
		t.Fatalf("expected exactly one FDN0222 for the unbound {id} segment, got %+v", diags)
	}
	if got[0].Severity != ir.SeverityError {
		t.Errorf("a routed-but-unread path segment generates broken routing, so it is fatal: got severity %q", got[0].Severity)
	}
	for _, want := range []string{`"PreviewTable"`, "{id}"} {
		if !strings.Contains(got[0].Message, want) {
			t.Errorf("expected the diagnostic to name %s, got %q", want, got[0].Message)
		}
	}
	if got[0].Span.Line == 0 {
		t.Errorf("expected the diagnostic to carry the action's declaration span, got %+v", got[0].Span)
	}
}

func TestValidate_HTTPPathParam_OneBoundOneUnbound_NamesTheUnbound(t *testing.T) {
	_, diags := parseAndValidate(t, previewBlueprintPrelude+`
action PreviewTable(id: uuid, foo: string) -> PreviewRow @http(method: "GET", path: "/v1/tables/{id}/versions/{version}") {
  let row = query one PreviewRow where id == id else PreviewRowNotFound
  return row
}
`)
	got := httpParamDiags(diags)
	if len(got) != 1 {
		t.Fatalf("expected one diagnostic for the one unbound segment, got %+v", diags)
	}
	if !strings.Contains(got[0].Message, "segment {version}") {
		t.Errorf("expected the diagnostic to name the unbound segment {version}, got %q", got[0].Message)
	}
}

func TestValidate_HTTPPathParam_RepeatedSegment_OneDiagnosticPerName(t *testing.T) {
	_, diags := parseAndValidate(t, previewBlueprintPrelude+`
action PreviewTable(foo: uuid) -> PreviewRow @http(method: "GET", path: "/v1/{id}/tables/{id}/preview") {
  let row = query one PreviewRow where id == foo else PreviewRowNotFound
  return row
}
`)
	if got := httpParamDiags(diags); len(got) != 1 {
		t.Errorf("a name repeated in the path is one mistake, not one per occurrence, got %+v", got)
	}
}

func TestValidate_HTTPPathParam_NoHTTPTrait_NoFDN0222(t *testing.T) {
	_, diags := parseAndValidate(t, previewBlueprintPrelude+`
action PreviewTable(foo: uuid) -> PreviewRow {
  let row = query one PreviewRow where id == foo else PreviewRowNotFound
  return row
}
`)
	if hasCode(diags, "FDN0222") {
		t.Errorf("an unrouted action has no path to bind, got %+v", diags)
	}
}

func TestValidate_HTTPPathParam_NoPathArg_NoFDN0222(t *testing.T) {
	_, diags := parseAndValidate(t, previewBlueprintPrelude+`
action PreviewTable(foo: uuid) -> PreviewRow @http(method: "GET") {
  let row = query one PreviewRow where id == foo else PreviewRowNotFound
  return row
}
`)
	if hasCode(diags, "FDN0222") {
		t.Errorf("an @http trait without a path falls back to the lowered default route, which routes no segments, got %+v", diags)
	}
}
