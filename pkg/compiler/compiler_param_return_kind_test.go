package compiler_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/galihanggara68/foundation/pkg/compiler"
)

func TestBuild_ActionParamAndSlotReturnKinds_EmitAndCompile(t *testing.T) {
	dir := t.TempDir()

	blueprint := `
entity Order {
  id: uuid @pk
  total: decimal
}

error NotFound {
  kind: not_found
  message: "order not found"
}

slot OrderLookup {
  @external
  Find(id: uuid) -> Order throws NotFound
}

action DescribeOrder(amounts: []decimal, id: uuid) -> Order {
  let found = slot OrderLookup.Find(id)
  return found
}

scenario "DescribeOrder returns the looked-up order" {
  given slot OrderLookup.Find("00000000-0000-0000-0000-000000000001") returns Order {
    id: "00000000-0000-0000-0000-000000000001",
    total: 10.0
  }
  when DescribeOrder([1.5, 2.5], "00000000-0000-0000-0000-000000000001")
  expect result.id == "00000000-0000-0000-0000-000000000001"
}
`

	buildProfile := `
project:
  name: paramkind
  module: example.com/paramkind

storage:
  dialect: postgres
  mode: declarative

targets:
  - name: backend
    kind: backend
    language: go
    architecture: hexagonal
    http: chi
    out_dir: ./dist/backend
`

	if err := os.WriteFile(filepath.Join(dir, "paramkind.fdn"), []byte(blueprint), 0644); err != nil {
		t.Fatalf("write paramkind.fdn: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "foundation.build.yaml"), []byte(buildProfile), 0644); err != nil {
		t.Fatalf("write foundation.build.yaml: %v", err)
	}

	if _, diags := compiler.Build(dir, compiler.BuildOptions{}); compiler.HasErrors(diags) {
		t.Fatalf("expected build with []decimal action param and entity slot return to succeed, got: %+v", diags)
	}

	outDir := filepath.Join(dir, "dist", "backend")
	useCasePath := filepath.Join(outDir, "internal/usecases/describeorder_usecase.go")
	slotPortPath := filepath.Join(outDir, "internal/ports/orderlookup_slot.go")

	useCase, err := os.ReadFile(useCasePath)
	if err != nil {
		t.Fatalf("read emitted use case: %v", err)
	}
	if !strings.Contains(string(useCase), "amounts []domain.Decimal") {
		t.Fatalf("expected emitted use case to carry the Kind-mapped []decimal param, got:\n%s", useCase)
	}

	slotPort, err := os.ReadFile(slotPortPath)
	if err != nil {
		t.Fatalf("read emitted slot port: %v", err)
	}
	if !strings.Contains(string(slotPort), "(domain.Order, error)") {
		t.Fatalf("expected emitted slot port to return the Kind-mapped entity type, got:\n%s", slotPort)
	}

	// The freshly generated project must compile and pass its generated tests.
	if testDiags := compiler.Test(dir); len(testDiags) != 0 {
		t.Fatalf("expected generated project with mixed param/return kinds to pass, got: %+v", testDiags)
	}
}

func TestBuild_SingleArrayBodyParam_DecodesBareArray(t *testing.T) {
	dir := t.TempDir()

	blueprint := `
entity Batch {
  id: uuid @pk
}

action SubmitBatch(totals: []decimal) -> bool
  @http(method: POST, path: "/v1/batches")
{
  return true
}
`

	buildProfile := `
project:
  name: batchkind
  module: example.com/batchkind

storage:
  dialect: postgres
  mode: declarative

targets:
  - name: backend
    kind: backend
    language: go
    architecture: hexagonal
    http: chi
    out_dir: ./dist/backend
`

	if err := os.WriteFile(filepath.Join(dir, "batchkind.fdn"), []byte(blueprint), 0644); err != nil {
		t.Fatalf("write batchkind.fdn: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "foundation.build.yaml"), []byte(buildProfile), 0644); err != nil {
		t.Fatalf("write foundation.build.yaml: %v", err)
	}

	if _, diags := compiler.Build(dir, compiler.BuildOptions{}); compiler.HasErrors(diags) {
		t.Fatalf("expected build with a single []decimal body param to succeed, got: %+v", diags)
	}

	router, err := os.ReadFile(filepath.Join(dir, "dist/backend/internal/adapters/http/router.go"))
	if err != nil {
		t.Fatalf("read emitted router: %v", err)
	}
	routerSrc := string(router)

	if !strings.Contains(routerSrc, "var totals []domain.Decimal") {
		t.Fatalf("expected the array body param to map to a bare slice, got:\n%s", routerSrc)
	}
	if !strings.Contains(routerSrc, "Decode(&totals)") {
		t.Fatalf("expected the handler to decode the body directly into the array param, got:\n%s", routerSrc)
	}
	if strings.Contains(routerSrc, "reqBody") {
		t.Fatalf("a lone array body param must not be wrapped in a request struct, got:\n%s", routerSrc)
	}
}
