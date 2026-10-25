package compiler_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/galihanggara68/foundation/pkg/compiler"
)

func TestCheck_SyntaxAndTypeErrors_ReturnsExactSpans(t *testing.T) {
	dir := t.TempDir()

	badBlueprint := `entity Book {
  id: uuid @pk
  sku: string
  price: int
}

action CreateBook(sku: string) -> Book {
  let book = create Book {
    sku: sku,
    price: "not-an-int",
    missing_field: 10
  }
  let found = query one UnknownEntity where id == sku else MissingError
  return undeclared_var
}
`
	if err := os.WriteFile(filepath.Join(dir, "bad.fdn"), []byte(badBlueprint), 0644); err != nil {
		t.Fatalf("write blueprint: %v", err)
	}

	ir, diags := compiler.Check(dir)
	if ir != nil {
		t.Fatalf("expected nil SemanticIR on validation errors, got %+v", ir)
	}
	if len(diags) < 4 {
		t.Fatalf("expected at least 4 diagnostics, got %d: %+v", len(diags), diags)
	}

	foundPriceMismatch := false
	foundMissingField := false
	foundUnknownEntity := false
	foundUndeclaredVar := false

	for _, d := range diags {
		if d.Span.File != "bad.fdn" {
			t.Errorf("expected diagnostic file 'bad.fdn', got %q", d.Span.File)
		}
		if d.Code == "FDN0205" && d.Span.Line == 10 && d.Span.Column == 12 {
			foundPriceMismatch = true
		}
		if d.Code == "FDN0204" && d.Span.Line == 11 && d.Span.Column == 5 {
			foundMissingField = true
		}
		if d.Code == "FDN0203" && d.Span.Line == 13 && d.Span.Column == 3 {
			foundUnknownEntity = true
		}
		if d.Code == "FDN0208" && d.Span.Line == 14 && d.Span.Column == 10 {
			foundUndeclaredVar = true
		}
	}

	if !foundPriceMismatch {
		t.Errorf("expected FDN0205 price type mismatch at bad.fdn:10:12, got %+v", diags)
	}
	if !foundMissingField {
		t.Errorf("expected FDN0204 unknown field at bad.fdn:11:5, got %+v", diags)
	}
	if !foundUnknownEntity {
		t.Errorf("expected FDN0203 unknown entity at bad.fdn:13:3, got %+v", diags)
	}
	if !foundUndeclaredVar {
		t.Errorf("expected FDN0208 undeclared var at bad.fdn:14:10, got %+v", diags)
	}
}

func TestCheck_RejectsUnboundedWhileRecursionAndNonExhaustiveMatch_WithExactSpans(t *testing.T) {
	dir := t.TempDir()

	badGuardsFDN := `enum OrderStatus {
  pending
  confirmed
  cancelled
}

action BadControlFlow(status: OrderStatus, n: int) -> int {
  while n > 0 {
    let n = n - 1
  }

  match status {
    .pending => {
      let x = 1
    }
  }

  let recurse = BadControlFlow(status, n - 1)
  return recurse
}
`
	if err := os.WriteFile(filepath.Join(dir, "guards.fdn"), []byte(badGuardsFDN), 0644); err != nil {
		t.Fatalf("write guards.fdn: %v", err)
	}

	ir, diags := compiler.Check(dir)
	if ir != nil {
		t.Fatalf("expected nil SemanticIR when static guardrails fail, got %+v", ir)
	}

	var foundWhile, foundNonExhaustive, foundRecursion bool
	for _, d := range diags {
		if d.Span.File != "guards.fdn" {
			t.Errorf("expected diagnostic file 'guards.fdn', got %q", d.Span.File)
		}
		if d.Code == "FDN0210" && d.Span.Line == 8 && d.Span.Column == 3 {
			foundWhile = true
		}
		if d.Code == "FDN0212" && d.Span.Line == 12 && d.Span.Column == 3 {
			foundNonExhaustive = true
		}
		if d.Code == "FDN0211" && d.Span.Line == 18 && d.Span.Column == 17 {
			foundRecursion = true
		}
	}

	if !foundWhile {
		t.Errorf("expected FDN0210 unbounded while diagnostic at guards.fdn:8:3, got %+v", diags)
	}
	if !foundNonExhaustive {
		t.Errorf("expected FDN0212 non-exhaustive match diagnostic at guards.fdn:12:3, got %+v", diags)
	}
	if !foundRecursion {
		t.Errorf("expected FDN0211 recursive action call diagnostic at guards.fdn:18:17, got %+v", diags)
	}
}
