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

func TestBuild_HTTPRouteAndRouteTraits_PopulateInPathFlags(t *testing.T) {
	src := `
error ItemNotFound {
  kind: not_found
  message: "item not found"
}

entity Item {
  id: uuid @pk
  name: string
}

action GetItem(org_id: uuid, item_id: uuid, verbose: bool) -> Item
  @http(method: "GET", path: "/v1/orgs/{org_id}/items/{item_id}")
{
  let found = query one Item where id == item_id else ItemNotFound
  if verbose && org_id == item_id {
    return found
  }
  return found
}

action UpdateItem(project_id: uuid, title: string) -> Item
  @route("/v1/projects/{project_id}")
{
  let found = query one Item where id == project_id else ItemNotFound
  if title == "test" {
    return found
  }
  return found
}

action InspectItem(file_id: uuid) -> Item
  @route(path: "/v1/files/{file_id}")
{
  let found = query one Item where id == file_id else ItemNotFound
  return found
}
`
	sources := []compiler.SourceFile{
		{Path: "items.fdn", Content: src},
	}
	sem, diags := compiler.Compile(sources)
	if compiler.HasErrors(diags) {
		t.Fatalf("unexpected compile errors: %+v", diags)
	}

	actions := make(map[string]map[string]bool)
	for _, a := range sem.Actions {
		params := make(map[string]bool)
		for _, p := range a.Params {
			params[p.Name] = p.InPath
		}
		actions[a.Name] = params
	}

	// 1. Check GetItem parameters
	getItem := actions["GetItem"]
	if !getItem["org_id"] {
		t.Errorf("GetItem org_id InPath = false, want true")
	}
	if !getItem["item_id"] {
		t.Errorf("GetItem item_id InPath = false, want true")
	}
	if getItem["verbose"] {
		t.Errorf("GetItem verbose InPath = true, want false")
	}

	// 2. Check UpdateItem parameters
	updateItem := actions["UpdateItem"]
	if !updateItem["project_id"] {
		t.Errorf("UpdateItem project_id InPath = false, want true")
	}
	if updateItem["title"] {
		t.Errorf("UpdateItem title InPath = true, want false")
	}

	// 3. Check InspectItem parameters
	inspectItem := actions["InspectItem"]
	if !inspectItem["file_id"] {
		t.Errorf("InspectItem file_id InPath = false, want true")
	}
}
