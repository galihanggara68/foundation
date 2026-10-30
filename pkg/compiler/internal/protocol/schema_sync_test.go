package protocol_test

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/protocol"
)

// publishedSchemaRelPath locates the standalone published schema bundle relative to
// this test package directory. `go test` runs with the package directory as its
// working directory, so this path resolves to <repo>/docs/schemas/generation-protocol-v1.json.
const publishedSchemaRelPath = "../../../../docs/schemas/generation-protocol-v1.json"

// fieldKindVocabulary is the closed FieldKind vocabulary the published schema must expose.
var fieldKindVocabulary = []string{
	"uuid", "string", "int", "decimal", "bool", "timestamp",
	"enum", "entity", "value", "event", "actor", "page", "result",
}

// TestPublishedSchemaInSync asserts the Go-embedded schema consts and the standalone
// published schema file never drift apart.
//
// The two copies are not byte-identical by construction, so their two structural
// differences are reconciled before the deep comparison:
//
//  1. The published file is a bundle: it declares "$schema" once at its root, while
//     each embedded const is a standalone schema carrying its own "$schema".
//  2. The published bundle hoists the shared FieldDecl definition to its own root
//     "definitions" section, while the embedded GenerationRequest schema keeps it
//     under its own root "definitions".
//
// Everything else must be deeply equal, so any future divergence fails this test.
func TestPublishedSchemaInSync(t *testing.T) {
	published := schemaReadObject(t, publishedSchemaRelPath)
	if got := schemaString(t, published, "$schema"); got != "http://json-schema.org/draft-07/schema#" {
		t.Fatalf("published schema declares unexpected root $schema: %q", got)
	}
	publishedDefs := schemaObject(t, published, "definitions")

	embeddedRequest := schemaParseObject(t, protocol.GenerationRequestSchemaJSON)
	embeddedResponse := schemaParseObject(t, protocol.GenerationResponseSchemaJSON)

	requestDefs := schemaObject(t, embeddedRequest, "definitions")
	delete(embeddedRequest, "definitions")
	delete(embeddedRequest, "$schema")
	delete(embeddedResponse, "$schema")

	embeddedDefs := map[string]any{
		"GenerationRequest":  embeddedRequest,
		"GenerationResponse": embeddedResponse,
		"FieldDecl":          requestDefs["FieldDecl"],
	}

	if !reflect.DeepEqual(embeddedDefs, publishedDefs) {
		t.Fatalf("embedded and published protocol schemas have diverged:\n  embedded:  %s\n  published: %s",
			schemaMarshal(t, embeddedDefs), schemaMarshal(t, publishedDefs))
	}
}

// TestPublishedSchemaDocumentsFieldKind asserts the published bundle exposes the closed
// field-kind vocabulary as an optional property of FieldDecl and wires it into the
// entity/value/event field arrays inside semantic_ir.
func TestPublishedSchemaDocumentsFieldKind(t *testing.T) {
	published := schemaReadObject(t, publishedSchemaRelPath)
	defs := schemaObject(t, published, "definitions")

	fieldDecl := schemaObject(t, defs, "FieldDecl")
	kind := schemaObject(t, schemaObject(t, fieldDecl, "properties"), "kind")

	if got := schemaString(t, kind, "type"); got != "string" {
		t.Fatalf("FieldDecl.kind must be a string, got %q", got)
	}
	if schemaContains(schemaStrings(t, fieldDecl, "required"), "kind") {
		t.Fatalf("FieldDecl.kind must be optional, but it is listed as required: %v",
			schemaStrings(t, fieldDecl, "required"))
	}

	got := schemaStrings(t, kind, "enum")
	if !reflect.DeepEqual(got, fieldKindVocabulary) {
		t.Fatalf("FieldDecl.kind enum mismatch:\n  got:  %v\n  want: %v", got, fieldKindVocabulary)
	}

	request := schemaObject(t, defs, "GenerationRequest")
	semanticProps := schemaObject(t, schemaObject(t, schemaObject(t, request, "properties"), "semantic_ir"), "properties")
	for _, collection := range []string{"entities", "values", "events"} {
		items := schemaObject(t, schemaObject(t, semanticProps, collection), "items")
		fields := schemaObject(t, schemaObject(t, items, "properties"), "fields")
		if ref := schemaString(t, schemaObject(t, fields, "items"), "$ref"); ref != "#/definitions/FieldDecl" {
			t.Fatalf("semantic_ir.%s.fields must reference FieldDecl, got %q", collection, ref)
		}
	}
}

// TestAdditiveCompatibility_V1RequestWithoutKind proves the additive-evolution promise
// of ADR 0011: a GenerationRequest document written before the field-kind taxonomy
// existed (no "kind" anywhere) still decodes and validates. The published schema
// documents "kind" as optional, which is what keeps that promise safe.
func TestAdditiveCompatibility_V1RequestWithoutKind(t *testing.T) {
	const preTaxonomyRequest = `{
  "protocol_version": 1,
  "workspace_root": "/workspace",
  "target": {"name": "backend", "kind": "backend", "language": "go", "out_dir": "gen"},
  "semantic_ir": {
    "entities": [
      {"name": "Order", "fields": [
        {"name": "id", "type": "uuid"},
        {"name": "total", "type": "decimal", "optional": true}
      ]}
    ]
  }
}`

	req, err := protocol.DecodeRequest([]byte(preTaxonomyRequest))
	if err != nil {
		t.Fatalf("DecodeRequest rejected a v1 document without kind: %v", err)
	}
	if err := req.Validate(); err != nil {
		t.Fatalf("v1 document without kind failed validation: %v", err)
	}
	if req.ProtocolVersion != protocol.ProtocolVersion {
		t.Fatalf("expected protocol_version %d, got %d", protocol.ProtocolVersion, req.ProtocolVersion)
	}
	if req.SemanticIR == nil || len(req.SemanticIR.Entities) != 1 || len(req.SemanticIR.Entities[0].Fields) != 2 {
		t.Fatalf("unexpected decoded semantic IR: %+v", req.SemanticIR)
	}
	if got := req.SemanticIR.Entities[0].Fields[0].Kind; got != "" {
		t.Fatalf("expected an absent kind to decode to the zero value, got %q", got)
	}

	// The same document carrying an explicit kind still decodes and populates it.
	withKind := strings.Replace(preTaxonomyRequest, `"type": "uuid"`, `"type": "uuid", "kind": "uuid"`, 1)
	reqWithKind, err := protocol.DecodeRequest([]byte(withKind))
	if err != nil {
		t.Fatalf("DecodeRequest rejected a v1 document with kind: %v", err)
	}
	if err := reqWithKind.Validate(); err != nil {
		t.Fatalf("v1 document with kind failed validation: %v", err)
	}
	if got := reqWithKind.SemanticIR.Entities[0].Fields[0].Kind; got != ir.KindUUID {
		t.Fatalf("expected decoded kind %q, got %q", ir.KindUUID, got)
	}
}

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
