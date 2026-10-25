package compiler_test

import (
	"testing"

	"github.com/galihanggara68/foundation/pkg/compiler"
)

func TestCompile_InMemoryBlueprint_SucceedsWithoutFilesystemIO(t *testing.T) {
	sources := []compiler.SourceFile{
		{
			Path: "domain.fdn",
			Content: `
entity Product {
  id: uuid @pk
  sku: string @unique
  price: decimal(10, 2)
}

error ProductNotFound {
  kind: not_found
  message: "Product not found"
}

action GetProduct(id: uuid) -> Product
  @http(method: GET, path: "/v1/products/{id}")
{
  let product = query one Product where id == id else ProductNotFound
  return product
}
`,
		},
	}

	sem, diags := compiler.Compile(sources)
	if len(diags) > 0 {
		t.Fatalf("unexpected diagnostics: %+v", diags)
	}
	if sem == nil {
		t.Fatalf("expected non-nil SemanticIR")
	}
	if len(sem.Entities) != 1 || sem.Entities[0].Name != "Product" {
		t.Errorf("expected 1 entity Product, got %+v", sem.Entities)
	}
	if len(sem.Actions) != 1 || sem.Actions[0].Name != "GetProduct" {
		t.Errorf("expected 1 action GetProduct, got %+v", sem.Actions)
	}
}

func TestCompile_EmptySources_ReturnsFDN0002(t *testing.T) {
	sem, diags := compiler.Compile(nil)
	if sem != nil {
		t.Fatalf("expected nil SemanticIR on empty sources")
	}
	if len(diags) != 1 || diags[0].Code != "FDN0002" {
		t.Fatalf("expected diagnostic FDN0002, got: %+v", diags)
	}
}
