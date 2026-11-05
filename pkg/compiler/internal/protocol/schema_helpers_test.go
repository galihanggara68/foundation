package protocol_test

import (
	"encoding/json"
	"os"
	"testing"
)

func schemaReadObject(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading published schema %s: %v", path, err)
	}
	return schemaParseObject(t, string(data))
}

func schemaParseObject(t *testing.T, data string) map[string]any {
	t.Helper()
	var obj map[string]any
	if err := json.Unmarshal([]byte(data), &obj); err != nil {
		t.Fatalf("schema is not valid JSON: %v", err)
	}
	return obj
}

func schemaObject(t *testing.T, obj map[string]any, key string) map[string]any {
	t.Helper()
	value, ok := obj[key].(map[string]any)
	if !ok {
		t.Fatalf("expected %q to be an object, got %T", key, obj[key])
	}
	return value
}

func schemaString(t *testing.T, obj map[string]any, key string) string {
	t.Helper()
	value, ok := obj[key].(string)
	if !ok {
		t.Fatalf("expected %q to be a string, got %T", key, obj[key])
	}
	return value
}

func schemaStrings(t *testing.T, obj map[string]any, key string) []string {
	t.Helper()
	raw, ok := obj[key].([]any)
	if !ok {
		t.Fatalf("expected %q to be an array, got %T", key, obj[key])
	}
	out := make([]string, len(raw))
	for i, item := range raw {
		value, ok := item.(string)
		if !ok {
			t.Fatalf("expected %q[%d] to be a string, got %T", key, i, item)
		}
		out[i] = value
	}
	return out
}

func schemaContains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func schemaMarshal(t *testing.T, value any) string {
	t.Helper()
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatalf("marshaling schema for diagnostics: %v", err)
	}
	return string(data)
}

// publishedDefinitions returns the published bundle's root definitions object.
func publishedDefinitions(t *testing.T) map[string]any {
	t.Helper()
	return schemaObject(t, schemaReadObject(t, publishedSchemaRelPath), "definitions")
}

// semanticIRProperties returns the "properties" object of semantic_ir in the published request.
func semanticIRProperties(t *testing.T) map[string]any {
	t.Helper()
	request := schemaObject(t, publishedDefinitions(t), "GenerationRequest")
	return schemaObject(t, schemaObject(t, schemaObject(t, request, "properties"), "semantic_ir"), "properties")
}

// semanticIRCollectionDefs maps each semantic_ir collection to the declaration it references.
var semanticIRCollectionDefs = map[string]string{
	"actor":     "ActorDecl",
	"actors":    "ActorDecl",
	"enums":     "EnumDecl",
	"values":    "ValueDecl",
	"entities":  "EntityDecl",
	"errors":    "ErrorDecl",
	"events":    "EventDecl",
	"policies":  "PolicyDecl",
	"slots":     "SlotDecl",
	"actions":   "ActionDecl",
	"scenarios": "ScenarioDecl",
}
