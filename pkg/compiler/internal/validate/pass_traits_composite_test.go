package validate_test

import (
	"strings"
	"testing"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/frontend"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/validate"
)

func TestValidate_CompositeIndex_ValidTwoFields_ReturnsNoDiagnostics(t *testing.T) {
	src := `
entity Order @index(fields: ["customer_id", "status"]) {
  id: uuid @pk
  customer_id: uuid
  status: string
}
`
	sem, parseDiags := frontend.ParseFile("test.fdn", src)
	if len(parseDiags) > 0 {
		t.Fatalf("unexpected parse diagnostics: %+v", parseDiags)
	}

	diags := validate.Validate(sem)
	if len(diags) > 0 {
		t.Fatalf("expected 0 diagnostics, got %d: %+v", len(diags), diags)
	}
}

func TestValidate_CompositeIndex_WithCustomName_ReturnsNoDiagnostics(t *testing.T) {
	src := `
entity Order @index(fields: ["customer_id", "status"], name: "idx_cust_status") {
  id: uuid @pk
  customer_id: uuid
  status: string
}
`
	sem, parseDiags := frontend.ParseFile("test.fdn", src)
	if len(parseDiags) > 0 {
		t.Fatalf("unexpected parse diagnostics: %+v", parseDiags)
	}

	diags := validate.Validate(sem)
	if len(diags) > 0 {
		t.Fatalf("expected 0 diagnostics, got %d: %+v", len(diags), diags)
	}
}

func TestValidate_CompositeIndex_PositionalList_ReturnsNoDiagnostics(t *testing.T) {
	src := `
entity Order @index(["customer_id", "status"]) {
  id: uuid @pk
  customer_id: uuid
  status: string
}
`
	sem, parseDiags := frontend.ParseFile("test.fdn", src)
	if len(parseDiags) > 0 {
		t.Fatalf("unexpected parse diagnostics: %+v", parseDiags)
	}

	diags := validate.Validate(sem)
	if len(diags) > 0 {
		t.Fatalf("expected 0 diagnostics, got %d: %+v", len(diags), diags)
	}
}

func TestValidate_CompositeUnique_ValidTwoFields_ReturnsNoDiagnostics(t *testing.T) {
	src := `
entity Membership @unique(fields: ["tenant_id", "user_id"]) {
  id: uuid @pk
  tenant_id: uuid
  user_id: uuid
}
`
	sem, parseDiags := frontend.ParseFile("test.fdn", src)
	if len(parseDiags) > 0 {
		t.Fatalf("unexpected parse diagnostics: %+v", parseDiags)
	}

	diags := validate.Validate(sem)
	if len(diags) > 0 {
		t.Fatalf("expected 0 diagnostics, got %d: %+v", len(diags), diags)
	}
}

func TestValidate_CompositeTraits_MultipleDistinctDeclarations_ReturnsNoDiagnostics(t *testing.T) {
	src := `
entity Order @index(fields: ["customer_id", "status"]) @index(fields: ["customer_id", "created_at"]) @unique(fields: ["tenant_id", "order_number"]) {
  id: uuid @pk
  customer_id: uuid
  status: string
  created_at: timestamp
  tenant_id: uuid
  order_number: string
}
`
	sem, parseDiags := frontend.ParseFile("test.fdn", src)
	if len(parseDiags) > 0 {
		t.Fatalf("unexpected parse diagnostics: %+v", parseDiags)
	}

	diags := validate.Validate(sem)
	if len(diags) > 0 {
		t.Fatalf("expected 0 diagnostics, got %d: %+v", len(diags), diags)
	}
}

func TestValidate_CompositeIndex_UnknownField_ReturnsFDN0203Error(t *testing.T) {
	src := `
entity Order @index(fields: ["customer_id", "non_existent"]) {
  id: uuid @pk
  customer_id: uuid
}
`
	sem, parseDiags := frontend.ParseFile("test.fdn", src)
	if len(parseDiags) > 0 {
		t.Fatalf("unexpected parse diagnostics: %+v", parseDiags)
	}

	diags := validate.Validate(sem)
	matches := findDiags(diags, "FDN0203")
	if len(matches) == 0 {
		t.Fatalf("expected FDN0203 error for unknown field, got none: %+v", diags)
	}
	if matches[0].Severity != ir.SeverityError {
		t.Errorf("expected FDN0203 to be an error, got %s", matches[0].Severity)
	}
	expectedMsg := `unknown field "non_existent" on entity "Order" in @index trait`
	if !strings.Contains(matches[0].Message, expectedMsg) {
		t.Errorf("expected message %q, got %q", expectedMsg, matches[0].Message)
	}
}

func TestValidate_CompositeUnique_UnknownField_ReturnsFDN0203Error(t *testing.T) {
	src := `
entity Account @unique(fields: ["email", "bogus_field"]) {
  id: uuid @pk
  email: string
}
`
	sem, parseDiags := frontend.ParseFile("test.fdn", src)
	if len(parseDiags) > 0 {
		t.Fatalf("unexpected parse diagnostics: %+v", parseDiags)
	}

	diags := validate.Validate(sem)
	matches := findDiags(diags, "FDN0203")
	if len(matches) == 0 {
		t.Fatalf("expected FDN0203 error for unknown field on @unique, got none: %+v", diags)
	}
	if matches[0].Severity != ir.SeverityError {
		t.Errorf("expected FDN0203 to be an error, got %s", matches[0].Severity)
	}
	expectedMsg := `unknown field "bogus_field" on entity "Account" in @unique trait`
	if !strings.Contains(matches[0].Message, expectedMsg) {
		t.Errorf("expected message %q, got %q", expectedMsg, matches[0].Message)
	}
}

func TestValidate_CompositeIndex_SingleFieldList_ReturnsFDN0226Warning(t *testing.T) {
	src := `
entity Order @index(fields: ["customer_id"]) {
  id: uuid @pk
  customer_id: uuid
}
`
	sem, parseDiags := frontend.ParseFile("test.fdn", src)
	if len(parseDiags) > 0 {
		t.Fatalf("unexpected parse diagnostics: %+v", parseDiags)
	}

	diags := validate.Validate(sem)
	matches := findDiags(diags, "FDN0226")
	if len(matches) == 0 {
		t.Fatalf("expected FDN0226 warning for single field composite index, got none: %+v", diags)
	}
	if matches[0].Severity != ir.SeverityWarning {
		t.Errorf("expected FDN0226 to be a warning, got %s", matches[0].Severity)
	}
	expectedMsg := `composite @index trait on entity "Order" requires at least two fields; use field-level @index for single-field constraints`
	if !strings.Contains(matches[0].Message, expectedMsg) {
		t.Errorf("expected message %q, got %q", expectedMsg, matches[0].Message)
	}
}

func TestValidate_CompositeIndex_MissingFieldsArg_ReturnsFDN0226Warning(t *testing.T) {
	src := `
entity Order @index(name: "custom_idx") {
  id: uuid @pk
  customer_id: uuid
}
`
	sem, parseDiags := frontend.ParseFile("test.fdn", src)
	if len(parseDiags) > 0 {
		t.Fatalf("unexpected parse diagnostics: %+v", parseDiags)
	}

	diags := validate.Validate(sem)
	matches := findDiags(diags, "FDN0226")
	if len(matches) == 0 {
		t.Fatalf("expected FDN0226 warning for missing fields arg, got none: %+v", diags)
	}
	expectedMsg := `composite @index trait on entity "Order" requires argument "fields"`
	if !strings.Contains(matches[0].Message, expectedMsg) {
		t.Errorf("expected message %q, got %q", expectedMsg, matches[0].Message)
	}
}

func TestValidate_CompositeIndex_EmptyFieldsList_ReturnsFDN0226Warning(t *testing.T) {
	src := `
entity Order @index(fields: []) {
  id: uuid @pk
  customer_id: uuid
}
`
	sem, parseDiags := frontend.ParseFile("test.fdn", src)
	if len(parseDiags) > 0 {
		t.Fatalf("unexpected parse diagnostics: %+v", parseDiags)
	}

	diags := validate.Validate(sem)
	matches := findDiags(diags, "FDN0226")
	if len(matches) == 0 {
		t.Fatalf("expected FDN0226 warning for empty fields list, got none: %+v", diags)
	}
	expectedMsg := `composite @index trait on entity "Order" requires at least two fields`
	if !strings.Contains(matches[0].Message, expectedMsg) {
		t.Errorf("expected message %q, got %q", expectedMsg, matches[0].Message)
	}
}

func TestValidate_CompositeIndex_MultipleSameEntity_ReturnsNoDiagnostics(t *testing.T) {
	src := `
entity Order @index(fields: ["customer_id", "status"]) @index(fields: ["customer_id", "status"]) {
  id: uuid @pk
  customer_id: uuid
  status: string
}
`
	sem, parseDiags := frontend.ParseFile("test.fdn", src)
	if len(parseDiags) > 0 {
		t.Fatalf("unexpected parse diagnostics: %+v", parseDiags)
	}

	diags := validate.Validate(sem)
	if len(diags) > 0 {
		t.Fatalf("expected 0 diagnostics, got: %+v", diags)
	}
}

func TestValidate_CompositeUnique_UnexpectedNameArg_ReturnsFDN0226Warning(t *testing.T) {
	src := `
entity Order @unique(fields: ["customer_id", "status"], name: "custom_name") {
  id: uuid @pk
  customer_id: uuid
  status: string
}
`
	sem, parseDiags := frontend.ParseFile("test.fdn", src)
	if len(parseDiags) > 0 {
		t.Fatalf("unexpected parse diagnostics: %+v", parseDiags)
	}

	diags := validate.Validate(sem)
	matches := findDiags(diags, "FDN0226")
	if len(matches) == 0 {
		t.Fatalf("expected FDN0226 warning for unexpected name arg on @unique, got none: %+v", diags)
	}
	expectedMsg := `unknown argument "name" for standard trait @unique`
	if !strings.Contains(matches[0].Message, expectedMsg) {
		t.Errorf("expected message %q, got %q", expectedMsg, matches[0].Message)
	}
}

func TestValidate_FieldLevelIndex_WithArguments_ReturnsFDN0226Warning(t *testing.T) {
	src := `
entity Product {
  id: uuid @pk
  code: string @index(name: "custom_idx")
}
`
	sem, parseDiags := frontend.ParseFile("test.fdn", src)
	if len(parseDiags) > 0 {
		t.Fatalf("unexpected parse diagnostics: %+v", parseDiags)
	}

	diags := validate.Validate(sem)
	matches := findDiags(diags, "FDN0226")
	if len(matches) == 0 {
		t.Fatalf("expected FDN0226 warning for field-level @index with arguments, got none: %+v", diags)
	}
	expectedMsg := `field-level trait @index accepts no arguments`
	if !strings.Contains(matches[0].Message, expectedMsg) {
		t.Errorf("expected message %q, got %q", expectedMsg, matches[0].Message)
	}
}

func TestValidate_FieldLevelUnique_WithArguments_ReturnsFDN0226Warning(t *testing.T) {
	src := `
entity Product {
  id: uuid @pk
  code: string @unique(unexpected: "arg")
}
`
	sem, parseDiags := frontend.ParseFile("test.fdn", src)
	if len(parseDiags) > 0 {
		t.Fatalf("unexpected parse diagnostics: %+v", parseDiags)
	}

	diags := validate.Validate(sem)
	matches := findDiags(diags, "FDN0226")
	if len(matches) == 0 {
		t.Fatalf("expected FDN0226 warning for field-level @unique with arguments, got none: %+v", diags)
	}
	expectedMsg := `field-level trait @unique accepts no arguments`
	if !strings.Contains(matches[0].Message, expectedMsg) {
		t.Errorf("expected message %q, got %q", expectedMsg, matches[0].Message)
	}
}
