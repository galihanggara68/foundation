package compiler_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/galihanggara68/foundation/pkg/compiler"
)

func TestBuild_HTTPPathSegmentWithoutParam_ReturnsFDN0222(t *testing.T) {
	dir := t.TempDir()

	bp := `error PreviewRowNotFound {
  kind: not_found
  message: "requested preview row does not exist"
}

entity PreviewRow {
  id: uuid @pk
  name: string
}

action PreviewTable(foo: uuid, table_name: string) -> PreviewRow @http(method: "GET", path: "/v1/tables/{id}/preview") {
  let row = query one PreviewRow where id == foo else PreviewRowNotFound
  return row
}
`
	if err := os.WriteFile(filepath.Join(dir, "preview.fdn"), []byte(bp), 0644); err != nil {
		t.Fatalf("write blueprint: %v", err)
	}

	buildProfile := `project:
  name: test-preview
  module: example.com/preview

storage:
  dialect: postgres
  mode: stateful

targets:
  - name: backend
    kind: backend
    language: go
    architecture: hexagonal
    http: chi
    out_dir: gen
`
	if err := os.WriteFile(filepath.Join(dir, "foundation.build.yaml"), []byte(buildProfile), 0644); err != nil {
		t.Fatalf("write foundation.build.yaml: %v", err)
	}

	sem, buildDiags := compiler.Build(dir, compiler.BuildOptions{})
	if !compiler.HasErrors(buildDiags) {
		t.Fatalf("expected the build to fail on a routed path segment with no matching action parameter, got sem=%v diags=%+v", sem, buildDiags)
	}
	diag := findHTTPParamDiag(buildDiags)
	if diag == nil {
		t.Fatalf("expected FDN0222 for the unbound {id} path segment, got %+v", buildDiags)
	}
	if diag.Severity != compiler.SeverityError {
		t.Errorf("FDN0222 must be fatal to a build: got severity %q", diag.Severity)
	}
	if diag.Span.File != "preview.fdn" || diag.Span.Line == 0 {
		t.Errorf("expected the diagnostic to point at the action declaration in preview.fdn, got %+v", diag.Span)
	}
}

func findHTTPParamDiag(diags []compiler.Diagnostic) *compiler.Diagnostic {
	for i := range diags {
		if diags[i].Code == "FDN0222" {
			return &diags[i]
		}
	}
	return nil
}
