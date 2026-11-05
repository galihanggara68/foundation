package protocol_test

import (
	"reflect"
	"testing"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/protocol"
)

// TestPublishedSchemaDocumentsSemanticIR asserts every semantic_ir collection resolves to
// a typed declaration, and that an action pipeline documents its ordered steps.
func TestPublishedSchemaDocumentsSemanticIR(t *testing.T) {
	defs := publishedDefinitions(t)
	semanticProps := semanticIRProperties(t)

	for collection, declName := range semanticIRCollectionDefs {
		entry := schemaObject(t, semanticProps, collection)
		if ref, ok := entry["$ref"].(string); ok {
			if ref != "#/definitions/"+declName {
				t.Errorf("semantic_ir.%s must reference %s, got %q", collection, declName, ref)
			}
			continue
		}
		items := schemaObject(t, entry, "items")
		if ref := schemaString(t, items, "$ref"); ref != "#/definitions/"+declName {
			t.Errorf("semantic_ir.%s.items must reference %s, got %q", collection, declName, ref)
		}
	}

	actionSteps := schemaObject(t, schemaObject(t, schemaObject(t, defs, "ActionDecl"), "properties"), "steps")
	if ref := schemaString(t, schemaObject(t, actionSteps, "items"), "$ref"); ref != "#/definitions/StepDecl" {
		t.Errorf("ActionDecl.steps.items must reference StepDecl, got %q", ref)
	}
}

// TestPublishedSchemaDocumentsStepDecl asserts the recursive discriminated union is
// documented: the closed step-kind vocabulary and the nested step arrays that make the
// union recursive.
func TestPublishedSchemaDocumentsStepDecl(t *testing.T) {
	defs := publishedDefinitions(t)
	step := schemaObject(t, defs, "StepDecl")
	props := schemaObject(t, step, "properties")

	kind := schemaObject(t, props, "kind")
	want := []string{
		"let", "query", "query_many", "create", "update", "delete", "require",
		"guard", "emit", "slot", "if", "match", "for", "while", "return",
	}
	if got := schemaStrings(t, kind, "enum"); !reflect.DeepEqual(got, want) {
		t.Fatalf("StepDecl.kind enum mismatch:\n  got:  %v\n  want: %v", got, want)
	}
	if !schemaContains(schemaStrings(t, step, "required"), "kind") {
		t.Errorf("StepDecl.kind must be required")
	}
	for _, nested := range []string{"then_steps", "else_steps"} {
		items := schemaObject(t, schemaObject(t, props, nested), "items")
		if ref := schemaString(t, items, "$ref"); ref != "#/definitions/StepDecl" {
			t.Errorf("StepDecl.%s.items must reference StepDecl, got %q", nested, ref)
		}
	}

	// The union is discriminated: one oneOf variant per kind, each pinning "kind".
	variants, ok := step["oneOf"].([]any)
	if !ok {
		t.Fatalf("StepDecl must be a discriminated union with a oneOf list, got %T", step["oneOf"])
	}
	if len(variants) != len(want) {
		t.Fatalf("expected %d step variants, got %d", len(want), len(variants))
	}
	seen := make(map[string]bool)
	for _, v := range variants {
		variant, ok := v.(map[string]any)
		if !ok {
			t.Fatalf("step variant must be an object, got %T", v)
		}
		kindProps := schemaObject(t, schemaObject(t, variant, "properties"), "kind")
		seen[schemaString(t, kindProps, "const")] = true
	}
	for _, kind := range want {
		if !seen[kind] {
			t.Errorf("StepDecl oneOf is missing a variant for kind %q", kind)
		}
	}
}

// TestAdditiveCompatibility_RequestWithActionSteps confirms an action pipeline with nested
// steps still decodes and validates under the additive-evolution promise of ADR 0011.
func TestAdditiveCompatibility_RequestWithActionSteps(t *testing.T) {
	const request = `{
  "protocol_version": 1,
  "workspace_root": "/workspace",
  "target": {"name": "backend", "kind": "backend", "language": "go", "out_dir": "gen"},
  "semantic_ir": {
    "actions": [
      {
        "name": "CreateItem",
        "params": [{"name": "id", "type": "uuid", "in_path": false}],
        "return_type": "Item",
        "steps": [
          {"kind": "let", "var_name": "x"},
          {"kind": "if", "then_steps": [{"kind": "return"}], "else_steps": []}
        ]
      }
    ]
  }
}`

	req, err := protocol.DecodeRequest([]byte(request))
	if err != nil {
		t.Fatalf("DecodeRequest rejected an action pipeline: %v", err)
	}
	if err := req.Validate(); err != nil {
		t.Fatalf("action pipeline failed validation: %v", err)
	}
	if len(req.SemanticIR.Actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(req.SemanticIR.Actions))
	}
	steps := req.SemanticIR.Actions[0].Steps
	if len(steps) != 2 || steps[0].Kind != ir.StepLet || steps[1].Kind != ir.StepIf {
		t.Fatalf("unexpected decoded steps: %+v", steps)
	}
	if len(steps[1].ThenSteps) != 1 || steps[1].ThenSteps[0].Kind != ir.StepReturn {
		t.Fatalf("unexpected nested then_steps: %+v", steps[1].ThenSteps)
	}
}
