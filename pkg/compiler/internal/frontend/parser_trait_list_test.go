package frontend_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/frontend"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
)

func TestParseTrait_ListArgument_Named(t *testing.T) {
	src := `
@index(fields: ["customer_id", "status"])
entity Order {
  id: uuid @pk
  customer_id: uuid
  status: string
}
`
	sem, diags := frontend.ParseFile("test.fdn", src)
	for _, d := range diags {
		t.Errorf("unexpected diagnostic: %s: %s", d.Code, d.Message)
	}
	if len(sem.Entities) != 1 {
		t.Fatalf("expected 1 entity, got %d", len(sem.Entities))
	}
	entity := sem.Entities[0]
	if len(entity.Traits) != 1 {
		t.Fatalf("expected 1 trait, got %d", len(entity.Traits))
	}
	tr := entity.Traits[0]
	if tr.Name != "index" {
		t.Errorf("expected trait name 'index', got %q", tr.Name)
	}
	fields, ok := tr.GetListArg("fields")
	if !ok {
		t.Fatalf("expected list arg 'fields' to exist")
	}
	wantFields := []string{"customer_id", "status"}
	if !reflect.DeepEqual(fields, wantFields) {
		t.Errorf("expected fields %v, got %v", wantFields, fields)
	}
	if _, ok := tr.GetStringArg("fields"); ok {
		t.Errorf("expected GetStringArg('fields') to return false")
	}
}

func TestParseTrait_MixedScalarAndListArgs(t *testing.T) {
	src := `
@index(name: "idx_custom", fields: ["a", "b"])
entity Order {
  id: uuid @pk
  a: string
  b: string
}
`
	sem, diags := frontend.ParseFile("test.fdn", src)
	for _, d := range diags {
		t.Errorf("unexpected diagnostic: %s: %s", d.Code, d.Message)
	}
	if len(sem.Entities) != 1 || len(sem.Entities[0].Traits) != 1 {
		t.Fatalf("expected 1 entity with 1 trait")
	}
	tr := sem.Entities[0].Traits[0]
	name, ok := tr.GetStringArg("name")
	if !ok || name != "idx_custom" {
		t.Errorf("expected name 'idx_custom', got %q (ok=%v)", name, ok)
	}
	fields, ok := tr.GetListArg("fields")
	if !ok || !reflect.DeepEqual(fields, []string{"a", "b"}) {
		t.Errorf("expected fields ['a', 'b'], got %v (ok=%v)", fields, ok)
	}
}

func TestParseTrait_MixedScalarAndListArgs_ReverseOrder(t *testing.T) {
	src := `
@index(fields: ["a", "b"], name: "idx_custom")
entity Order {
  id: uuid @pk
  a: string
  b: string
}
`
	sem, diags := frontend.ParseFile("test.fdn", src)
	for _, d := range diags {
		t.Errorf("unexpected diagnostic: %s: %s", d.Code, d.Message)
	}
	if len(sem.Entities) != 1 || len(sem.Entities[0].Traits) != 1 {
		t.Fatalf("expected 1 entity with 1 trait")
	}
	tr := sem.Entities[0].Traits[0]
	name, ok := tr.GetStringArg("name")
	if !ok || name != "idx_custom" {
		t.Errorf("expected name 'idx_custom', got %q (ok=%v)", name, ok)
	}
	fields, ok := tr.GetListArg("fields")
	if !ok || !reflect.DeepEqual(fields, []string{"a", "b"}) {
		t.Errorf("expected fields ['a', 'b'], got %v (ok=%v)", fields, ok)
	}
}

func TestParseTrait_ListArgument_Positional(t *testing.T) {
	src := `
@index(["customer_id", "status"])
entity Order {
  id: uuid @pk
}
`
	sem, diags := frontend.ParseFile("test.fdn", src)
	for _, d := range diags {
		t.Errorf("unexpected diagnostic: %s: %s", d.Code, d.Message)
	}
	if len(sem.Entities) != 1 || len(sem.Entities[0].Traits) != 1 {
		t.Fatalf("expected 1 entity with 1 trait")
	}
	tr := sem.Entities[0].Traits[0]
	fields, ok := tr.GetListArg("fields")
	if !ok || !reflect.DeepEqual(fields, []string{"customer_id", "status"}) {
		t.Errorf("expected positional list ['customer_id', 'status'] under 'fields', got %v (ok=%v)", fields, ok)
	}
}

func TestParseTrait_ListArgument_IdentifierTokens_RejectsWithFDN0102(t *testing.T) {
	src := `
@index(fields: [col_a, col_b])
entity Order {
  id: uuid @pk
}
`
	_, diags := frontend.ParseFile("test.fdn", src)
	hasDiag := false
	for _, d := range diags {
		if d.Code == "FDN0102" {
			hasDiag = true
			break
		}
	}
	if !hasDiag {
		t.Fatalf("expected FDN0102 for identifier token in list argument, got %+v", diags)
	}
}

func TestParseTrait_ListArgument_TrailingComma(t *testing.T) {
	src := `
@index(fields: ["a", "b",])
entity Order {
  id: uuid @pk
}
`
	sem, diags := frontend.ParseFile("test.fdn", src)
	for _, d := range diags {
		t.Errorf("unexpected diagnostic: %s: %s", d.Code, d.Message)
	}
	if len(sem.Entities) != 1 || len(sem.Entities[0].Traits) != 1 {
		t.Fatalf("expected 1 entity with 1 trait")
	}
	tr := sem.Entities[0].Traits[0]
	fields, ok := tr.GetListArg("fields")
	if !ok || !reflect.DeepEqual(fields, []string{"a", "b"}) {
		t.Errorf("expected fields ['a', 'b'], got %v (ok=%v)", fields, ok)
	}
}

func TestParseTrait_ListArgument_EmptyList(t *testing.T) {
	src := `
@index(fields: [])
entity Order {
  id: uuid @pk
}
`
	sem, diags := frontend.ParseFile("test.fdn", src)
	for _, d := range diags {
		t.Errorf("unexpected diagnostic: %s: %s", d.Code, d.Message)
	}
	if len(sem.Entities) != 1 || len(sem.Entities[0].Traits) != 1 {
		t.Fatalf("expected 1 entity with 1 trait")
	}
	tr := sem.Entities[0].Traits[0]
	fields, ok := tr.GetListArg("fields")
	if !ok {
		t.Fatalf("expected empty list to exist")
	}
	if len(fields) != 0 {
		t.Errorf("expected empty list, got %v", fields)
	}
}

func TestParseTrait_ListArgument_InvalidTokenSpan(t *testing.T) {
	// Line 2: @index(fields: [123])
	//                         ^ line 2 column 18
	src := `
@index(fields: [123])
entity Order {
  id: uuid @pk
}
`
	_, diags := frontend.ParseFile("test.fdn", src)
	var found bool
	for _, d := range diags {
		if d.Code == "FDN0102" && strings.Contains(d.Message, "expected string literal in list") {
			found = true
			if d.Span.Line != 2 || d.Span.Column != 17 {
				t.Errorf("expected span Line=2 Column=17, got Line=%d Column=%d", d.Span.Line, d.Span.Column)
			}
		}
	}
	if !found {
		t.Fatalf("expected diagnostic for unexpected token in list, got: %+v", diags)
	}
}

func TestParseTrait_ListArgument_UnclosedBracketSpan(t *testing.T) {
	// Line 2: @index(fields: ["a", "b")
	//                                 ^ line 2 column 27
	src := `
@index(fields: ["a", "b")
entity Order {
  id: uuid @pk
}
`
	_, diags := frontend.ParseFile("test.fdn", src)
	var found bool
	for _, d := range diags {
		if d.Code == "FDN0102" && strings.Contains(d.Message, "expected ',' or ']'") {
			found = true
			if d.Span.Line != 2 || d.Span.Column != 25 {
				t.Errorf("expected span Line=2 Column=25, got Line=%d Column=%d", d.Span.Line, d.Span.Column)
			}
		}
	}
	if !found {
		t.Fatalf("expected diagnostic for unclosed list bracket, got: %+v", diags)
	}
}

func TestTraitDecl_JSONSerialization(t *testing.T) {
	tr := ir.TraitDecl{
		Name: "index",
		Args: map[string]string{
			"name": "idx_order_status",
		},
		ListArgs: map[string][]string{
			"fields": {"customer_id", "status"},
		},
		Span: ir.Span{File: "test.fdn", Line: 1, Column: 1},
	}

	bytes, err := json.Marshal(tr)
	if err != nil {
		t.Fatalf("failed to marshal TraitDecl: %v", err)
	}
	jsonStr := string(bytes)
	if !strings.Contains(jsonStr, `"list_args":{"fields":["customer_id","status"]}`) {
		t.Errorf("expected list_args in JSON, got: %s", jsonStr)
	}
	if !strings.Contains(jsonStr, `"args":{"name":"idx_order_status"}`) {
		t.Errorf("expected args in JSON, got: %s", jsonStr)
	}

	var decoded ir.TraitDecl
	if err := json.Unmarshal(bytes, &decoded); err != nil {
		t.Fatalf("failed to unmarshal TraitDecl: %v", err)
	}
	if decoded.Name != tr.Name {
		t.Errorf("expected name %q, got %q", tr.Name, decoded.Name)
	}
	if !reflect.DeepEqual(decoded.Args, tr.Args) {
		t.Errorf("expected args %v, got %v", tr.Args, decoded.Args)
	}
	if !reflect.DeepEqual(decoded.ListArgs, tr.ListArgs) {
		t.Errorf("expected listArgs %v, got %v", tr.ListArgs, decoded.ListArgs)
	}

	// Verify omitempty behavior when ListArgs is empty
	scalarOnly := ir.TraitDecl{
		Name: "pk",
		Span: ir.Span{File: "test.fdn", Line: 1, Column: 1},
	}
	scalarBytes, err := json.Marshal(scalarOnly)
	if err != nil {
		t.Fatalf("failed to marshal scalar-only TraitDecl: %v", err)
	}
	if strings.Contains(string(scalarBytes), "list_args") {
		t.Errorf("expected list_args to be omitted when empty, got: %s", string(scalarBytes))
	}
}

func TestTraitDecl_HelperMethods(t *testing.T) {
	emptyTr := ir.TraitDecl{Name: "empty"}
	if val, ok := emptyTr.GetStringArg("key"); ok || val != "" {
		t.Errorf("expected ('', false) from empty TraitDecl, got (%q, %v)", val, ok)
	}
	if list, ok := emptyTr.GetListArg("key"); ok || list != nil {
		t.Errorf("expected (nil, false) from empty TraitDecl, got (%v, %v)", list, ok)
	}

	tr := ir.TraitDecl{
		Name: "test",
		Args: map[string]string{
			"color": "blue",
		},
		ListArgs: map[string][]string{
			"tags": {"alpha", "beta"},
		},
	}
	if val, ok := tr.GetStringArg("color"); !ok || val != "blue" {
		t.Errorf("expected ('blue', true), got (%q, %v)", val, ok)
	}
	if val, ok := tr.GetStringArg("missing"); ok || val != "" {
		t.Errorf("expected ('', false), got (%q, %v)", val, ok)
	}
	if list, ok := tr.GetListArg("tags"); !ok || !reflect.DeepEqual(list, []string{"alpha", "beta"}) {
		t.Errorf("expected (['alpha', 'beta'], true), got (%v, %v)", list, ok)
	}
	if list, ok := tr.GetListArg("missing"); ok || list != nil {
		t.Errorf("expected (nil, false), got (%v, %v)", list, ok)
	}
}
