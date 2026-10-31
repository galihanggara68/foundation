package protocol_test

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/platform"
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
		"ErrorDecl":          requestDefs["ErrorDecl"],
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

// TestPublishedSchemaDocumentsPlatform asserts the published bundle exposes the
// compiler-owned platform policy as an optional object on the request.
func TestPublishedSchemaDocumentsPlatform(t *testing.T) {
	published := schemaReadObject(t, publishedSchemaRelPath)
	defs := schemaObject(t, published, "definitions")
	request := schemaObject(t, defs, "GenerationRequest")

	if schemaContains(schemaStrings(t, request, "required"), "platform") {
		t.Fatalf("platform must be optional, but it is listed as required: %v",
			schemaStrings(t, request, "required"))
	}

	platformProp := schemaObject(t, schemaObject(t, request, "properties"), "platform")
	paginationProps := schemaObject(t, schemaObject(t, schemaObject(t, platformProp, "properties"), "pagination"), "properties")

	for _, field := range []string{"default_limit", "max_limit"} {
		if got := schemaString(t, schemaObject(t, paginationProps, field), "type"); got != "integer" {
			t.Fatalf("platform.pagination.%s must be an integer, got %q", field, got)
		}
	}
}

// TestAdditiveCompatibility_V1RequestWithoutPlatform proves the additive-evolution
// promise of ADR 0011 for the platform section: a request written before it existed
// still decodes and validates, and its platform policy is simply absent.
func TestAdditiveCompatibility_V1RequestWithoutPlatform(t *testing.T) {
	const prePlatformRequest = `{
  "protocol_version": 1,
  "workspace_root": "/workspace",
  "target": {"name": "backend", "kind": "backend", "language": "go", "out_dir": "gen"},
  "semantic_ir": {"entities": []}
}`

	req, err := protocol.DecodeRequest([]byte(prePlatformRequest))
	if err != nil {
		t.Fatalf("DecodeRequest rejected a v1 document without platform: %v", err)
	}
	if err := req.Validate(); err != nil {
		t.Fatalf("v1 document without platform failed validation: %v", err)
	}
	if req.Platform != nil {
		t.Fatalf("an absent platform section must decode to nil, got %+v", req.Platform)
	}

	withPlatform := strings.Replace(prePlatformRequest, `"semantic_ir"`,
		`"platform": {"pagination": {"default_limit": 7, "max_limit": 42}}, "semantic_ir"`, 1)
	reqWith, err := protocol.DecodeRequest([]byte(withPlatform))
	if err != nil {
		t.Fatalf("DecodeRequest rejected a v1 document with platform: %v", err)
	}
	if err := reqWith.Validate(); err != nil {
		t.Fatalf("v1 document with platform failed validation: %v", err)
	}
	if reqWith.Platform == nil {
		t.Fatal("expected a decoded platform section")
	}
	if got := reqWith.Platform.Pagination; got.DefaultLimit != 7 || got.MaxLimit != 42 {
		t.Fatalf("unexpected decoded pagination policy: %+v", got)
	}
}

// TestPublishedSchemaDocumentsErrorDecl asserts the published bundle exposes the v2
// error shape as an optional, additive definition and wires it into both error arrays.
func TestPublishedSchemaDocumentsErrorDecl(t *testing.T) {
	published := schemaReadObject(t, publishedSchemaRelPath)
	defs := schemaObject(t, published, "definitions")

	errorDecl := schemaObject(t, defs, "ErrorDecl")
	props := schemaObject(t, errorDecl, "properties")

	wantKinds := errorKindVocabulary()
	wantPostures := postureVocabulary()

	for field, wantType := range map[string]string{"name": "string", "message": "string", "status": "integer"} {
		if got := schemaString(t, schemaObject(t, props, field), "type"); got != wantType {
			t.Errorf("ErrorDecl.%s must be a %s, got %q", field, wantType, got)
		}
	}

	kind := schemaObject(t, props, "kind")
	if got := schemaString(t, kind, "type"); got != "string" {
		t.Errorf("ErrorDecl.kind must be a string, got %q", got)
	}
	if got := schemaStrings(t, kind, "enum"); !reflect.DeepEqual(got, wantKinds) {
		t.Fatalf("ErrorDecl.kind enum mismatch:\n  got:  %v\n  want: %v", got, wantKinds)
	}

	if got := schemaStrings(t, schemaObject(t, props, "posture"), "enum"); !reflect.DeepEqual(got, wantPostures) {
		t.Fatalf("ErrorDecl.posture enum mismatch:\n  got:  %v\n  want: %v", got, wantPostures)
	}

	messages := schemaObject(t, props, "messages")
	if got := schemaString(t, messages, "type"); got != "object" {
		t.Errorf("ErrorDecl.messages must be an object, got %q", got)
	}
	if got := schemaString(t, schemaObject(t, messages, "additionalProperties"), "type"); got != "string" {
		t.Errorf("ErrorDecl.messages must map locale tags to strings, got %q", got)
	}

	// What a v1 document always carried is required; everything v2 added is not, which is
	// what lets an old document satisfy this schema.
	required := schemaStrings(t, errorDecl, "required")
	for _, must := range []string{"name", "kind", "message"} {
		if !schemaContains(required, must) {
			t.Errorf("ErrorDecl.%s is on every document and must be required, but required lists %v", must, required)
		}
	}
	for _, optional := range []string{"messages", "status", "posture"} {
		if schemaContains(required, optional) {
			t.Errorf("ErrorDecl.%s must stay optional so v1 documents remain valid, but required lists %v", optional, required)
		}
	}

	request := schemaObject(t, defs, "GenerationRequest")
	requestProps := schemaObject(t, request, "properties")
	semanticProps := schemaObject(t, schemaObject(t, requestProps, "semantic_ir"), "properties")
	if ref := schemaString(t, schemaObject(t, schemaObject(t, semanticProps, "errors"), "items"), "$ref"); ref != "#/definitions/ErrorDecl" {
		t.Errorf("semantic_ir.errors must reference ErrorDecl, got %q", ref)
	}
	archProps := schemaObject(t, schemaObject(t, requestProps, "architecture_ir"), "properties")
	if ref := schemaString(t, schemaObject(t, schemaObject(t, archProps, "domain_errors"), "items"), "$ref"); ref != "#/definitions/ErrorDecl" {
		t.Errorf("architecture_ir.domain_errors must reference ErrorDecl, got %q", ref)
	}
}

// errorKindVocabulary reads the compiler's own closed kind set rather than restating it,
// so the published enum cannot drift from the kinds the validator accepts.
func errorKindVocabulary() []string {
	kinds := platform.StandardErrorKinds()
	names := make([]string, len(kinds))
	for i, kind := range kinds {
		names[i] = string(kind)
	}
	return names
}

// postureVocabulary reads the compiler's closed posture set for the same reason as
// errorKindVocabulary.
func postureVocabulary() []string {
	return []string{string(ir.PostureBusiness), string(ir.PostureTechnical)}
}

// TestAdditiveCompatibility_V1RequestWithoutResolvedErrorFields proves the
// additive-evolution promise of ADR 0011 for the v2 error shape: a request written before
// status, posture and locale messages existed still decodes and validates, and the new
// fields simply decode to their zero values.
func TestAdditiveCompatibility_V1RequestWithoutResolvedErrorFields(t *testing.T) {
	const preV2Request = `{
  "protocol_version": 1,
  "workspace_root": "/workspace",
  "target": {"name": "backend", "kind": "backend", "language": "go", "out_dir": "gen"},
  "semantic_ir": {
    "errors": [
      {"name": "OrderNotFound", "kind": "not_found", "message": "no such order"}
    ]
  }
}`

	req, err := protocol.DecodeRequest([]byte(preV2Request))
	if err != nil {
		t.Fatalf("DecodeRequest rejected a v1 document without the resolved error fields: %v", err)
	}
	if err := req.Validate(); err != nil {
		t.Fatalf("v1 document without the resolved error fields failed validation: %v", err)
	}
	if req.ProtocolVersion != protocol.ProtocolVersion {
		t.Fatalf("expected protocol_version %d to be unchanged, got %d", protocol.ProtocolVersion, req.ProtocolVersion)
	}
	if req.SemanticIR == nil || len(req.SemanticIR.Errors) != 1 {
		t.Fatalf("unexpected decoded semantic IR: %+v", req.SemanticIR)
	}
	got := req.SemanticIR.Errors[0]
	if got.Name != "OrderNotFound" || got.Kind != ir.ErrorKindNotFound || got.Message != "no such order" {
		t.Fatalf("v1 error fields did not survive decoding: %+v", got)
	}
	if got.Status != 0 || got.Posture != "" || got.Messages != nil {
		t.Fatalf("absent resolved fields must decode to their zero values, got %+v", got)
	}

	// The published schema has to accept that same document, so nothing it requires may be
	// absent from what a v1 compiler produced.
	var v1Doc struct {
		SemanticIR struct {
			Errors []map[string]any `json:"errors"`
		} `json:"semantic_ir"`
	}
	if err := json.Unmarshal([]byte(preV2Request), &v1Doc); err != nil {
		t.Fatalf("fixture is not valid JSON: %v", err)
	}
	defs := schemaObject(t, schemaReadObject(t, publishedSchemaRelPath), "definitions")
	for _, field := range schemaStrings(t, schemaObject(t, defs, "ErrorDecl"), "required") {
		if _, present := v1Doc.SemanticIR.Errors[0][field]; !present {
			t.Errorf("the published schema requires ErrorDecl.%s, which a v1 document does not carry", field)
		}
	}

	// The same document carrying the v2 fields still decodes and populates them.
	withResolved := strings.Replace(preV2Request, `"message": "no such order"`,
		`"message": "no such order", "messages": {"en": "no such order"}, "status": 404, "posture": "business"`, 1)
	reqWith, err := protocol.DecodeRequest([]byte(withResolved))
	if err != nil {
		t.Fatalf("DecodeRequest rejected a v2 document with the resolved error fields: %v", err)
	}
	if err := reqWith.Validate(); err != nil {
		t.Fatalf("v2 document with the resolved error fields failed validation: %v", err)
	}
	resolved := reqWith.SemanticIR.Errors[0]
	if resolved.Status != 404 || resolved.Posture != ir.PostureBusiness {
		t.Fatalf("resolved error fields did not decode: %+v", resolved)
	}
	if found := resolved.Messages["en"]; found != "no such order" {
		t.Fatalf("locale messages did not decode: %+v", resolved.Messages)
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
