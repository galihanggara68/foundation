package lsp

import (
	"strings"
	"unicode"
)

// ComputeHover finds semantic documentation for the token under the cursor.
func ComputeHover(docContent string, pos Position) *Hover {
	word := extractWordAt(docContent, pos)
	if word == "" {
		return nil
	}

	lookup := strings.ToLower(word)
	doc, ok := hoverCatalog[lookup]
	if !ok && strings.HasPrefix(lookup, "@") {
		doc, ok = hoverCatalog[lookup[1:]]
	}
	if !ok {
		doc, ok = hoverCatalog[word]
	}
	if !ok {
		return nil
	}

	return &Hover{
		Contents: MarkupContent{
			Kind:  "markdown",
			Value: doc,
		},
	}
}

// ComputeCompletion provides context-aware completion suggestions.
func ComputeCompletion(docContent string, pos Position) *CompletionList {
	prefix, isTraitContext := extractPrefixAt(docContent, pos)
	isTypeContext := isTypeContextAt(docContent, pos)

	var items []CompletionItem

	if isTraitContext {
		for _, item := range traitCompletions {
			if strings.HasPrefix(item.Label, prefix) {
				items = append(items, item)
			}
		}
		return &CompletionList{IsIncomplete: false, Items: items}
	}

	if isTypeContext {
		for _, item := range typeCompletions {
			if prefix == "" || strings.HasPrefix(strings.ToLower(item.Label), strings.ToLower(prefix)) {
				items = append(items, item)
			}
		}
		return &CompletionList{IsIncomplete: false, Items: items}
	}

	// General suggestions: keywords, types, traits
	for _, item := range keywordCompletions {
		if prefix == "" || strings.HasPrefix(item.Label, prefix) {
			items = append(items, item)
		}
	}
	for _, item := range typeCompletions {
		if prefix == "" || strings.HasPrefix(strings.ToLower(item.Label), strings.ToLower(prefix)) {
			items = append(items, item)
		}
	}
	for _, item := range traitCompletions {
		if prefix == "" || strings.HasPrefix(item.Label, prefix) {
			items = append(items, item)
		}
	}

	return &CompletionList{IsIncomplete: false, Items: items}
}

func isTypeContextAt(docContent string, pos Position) bool {
	lines := strings.Split(docContent, "\n")
	if pos.Line < 0 || pos.Line >= len(lines) {
		return false
	}
	line := lines[pos.Line]
	colonIdx := strings.LastIndex(line[:pos.Character], ":")
	return colonIdx >= 0
}

func extractWordAt(docContent string, pos Position) string {
	lines := strings.Split(docContent, "\n")
	if pos.Line < 0 || pos.Line >= len(lines) {
		return ""
	}
	line := lines[pos.Line]
	if pos.Character < 0 || pos.Character >= len(line) {
		return ""
	}

	// Must be over a word character or trait '@'
	currentChar := rune(line[pos.Character])
	if !isWordRune(currentChar) && currentChar != '@' {
		return ""
	}

	// Find word boundaries
	start := pos.Character
	for start > 0 && isWordRune(rune(line[start-1])) {
		start--
	}
	// Check for leading '@' trait marker
	if start > 0 && line[start-1] == '@' {
		start--
	}

	end := pos.Character
	for end < len(line) && isWordRune(rune(line[end])) {
		end++
	}

	if start >= end {
		return ""
	}
	return line[start:end]
}

func extractPrefixAt(docContent string, pos Position) (string, bool) {
	lines := strings.Split(docContent, "\n")
	if pos.Line < 0 || pos.Line >= len(lines) {
		return "", false
	}
	line := lines[pos.Line]
	if pos.Character < 0 || pos.Character > len(line) {
		return "", false
	}

	start := pos.Character
	for start > 0 && isWordRune(rune(line[start-1])) {
		start--
	}

	isTrait := false
	if start > 0 && line[start-1] == '@' {
		start--
		isTrait = true
	}

	return line[start:pos.Character], isTrait
}

func isWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}

var hoverCatalog = map[string]string{
	// Traits
	"@table": "### `@table` Trait\n\nMaps the entity to a relational SQL database table.\n\n" +
		"**Arguments:**\n" +
		"- `name`: Override default physical table name (e.g. `@table(name = \"app_users\")`).\n" +
		"- `audit`: Automatically provision and track audit metadata (`created_at`, `updated_at`).",
	"@version": "### `@version` Trait\n\nMarks an integer field as the optimistic concurrency control (OCC) column.\n\n" +
		"The compiler guarantees atomic compare-and-swap on mutations, aborting with `409 Conflict` on race conditions.",
	"@event_handler": "### `@event_handler` Trait\n\nDeclares an asynchronous consumer action triggered whenever the named domain event is published.",
	"@route": "### `@route` Trait\n\nExposes an action as a public HTTP API endpoint.\n\n" +
		"**Examples:**\n" +
		"- `@route(post = \"/orders\")`\n" +
		"- `@route(get = \"/orders/{id}\")`",
	"@cron":        "### `@cron` Trait\n\nSchedules periodic background execution using standard 5-field cron syntax (e.g. `@cron(\"0 * * * *\")`).",
	"@audit":       "### `@audit` Trait\n\nEnforces immutable audit trail logging for all mutations executed in this transaction pipeline.",
	"@lock":        "### `@lock` Trait\n\nDeclares pessimistic row-level locking (`SELECT ... FOR UPDATE`) on read queries within this action.",
	"@idempotency": "### `@idempotency` Trait\n\nEnforces client idempotency keys to guarantee at-most-once mutation semantics across network retries.",

	// Scalar Types
	"uuid":      "### `UUID` Primitive Type\n\n128-bit RFC 4122 Universally Unique Identifier. Stored as `UUID` in PostgreSQL.",
	"string":    "### `String` Primitive Type\n\nVariable-length UTF-8 text. Stored as `TEXT` in SQL.",
	"int":       "### `Int` Primitive Type\n\n64-bit signed integer. Stored as `BIGINT` in SQL.",
	"decimal":   "### `Decimal` Primitive Type\n\nArbitrary-precision fixed-point number. Stored as `NUMERIC(18, 4)` in SQL.",
	"money":     "### `Money` Primitive Type\n\nMonetary amount with fractional subunits stored as `NUMERIC(18, 4)` in SQL and serialized as string in JSON.",
	"email":     "### `Email` Primitive Type\n\nValidated RFC 5322 email string stored as `VARCHAR(320)` in SQL.",
	"bool":      "### `Boolean` Primitive Type\n\nBoolean true/false value. Stored as `BOOLEAN` in SQL.",
	"boolean":   "### `Boolean` Primitive Type\n\nBoolean true/false value. Stored as `BOOLEAN` in SQL.",
	"timestamp": "### `Timestamp` Primitive Type\n\nUTC timestamp with microsecond precision. Stored as `TIMESTAMPTZ` in SQL and serialized in ISO-8601.",
	"date":      "### `Date` Primitive Type\n\nCalendar date (YYYY-MM-DD). Stored as `DATE` in SQL.",
	"json":      "### `JSON` Primitive Type\n\nStructured JSON object or array. Stored as `JSONB` in PostgreSQL.",
	"bytes":     "### `Bytes` Primitive Type\n\nRaw binary data stored as `BYTEA` in PostgreSQL and base64-encoded in JSON.",

	// Constructs
	"entity":   "### `entity` Construct\n\nDeclares a domain aggregate/entity with persistent identity, typed fields, and traits.",
	"action":   "### `action` Construct\n\nDeclares an orchestrating transaction pipeline with steps, queries, validations, mutations, and events.",
	"query":    "### `query` Step\n\nExecutes a sargable query against an entity table returning a single record or collection.",
	"scenario": "### `scenario` Construct\n\nDeclares an end-to-end integration and scenario test running against memory or database drivers.",
	"actor":    "### `actor` Construct\n\nDeclares the authentication subject/principal model passed via request security context.",
	"value":    "### `value` Construct\n\nDeclares an immutable value object without independent persistence identity.",
	"enum":     "### `enum` Construct\n\nDeclares a closed set of enumerated string or integer variants.",
	"event":    "### `event` Construct\n\nDeclares a domain event payload published to brokers or asynchronous event handlers.",
}

var keywordCompletions = []CompletionItem{
	{Label: "entity", Kind: CompletionKindKeyword, Detail: "Declare a domain entity", InsertText: "entity ${1:Name} {\n\t$0\n}"},
	{Label: "action", Kind: CompletionKindKeyword, Detail: "Declare a transaction action", InsertText: "action ${1:Name} {\n\t$0\n}"},
	{Label: "scenario", Kind: CompletionKindKeyword, Detail: "Declare a scenario test", InsertText: "scenario ${1:Name} {\n\t$0\n}"},
	{Label: "actor", Kind: CompletionKindKeyword, Detail: "Declare the security actor", InsertText: "actor ${1:Name} {\n\t$0\n}"},
	{Label: "value", Kind: CompletionKindKeyword, Detail: "Declare a value object", InsertText: "value ${1:Name} {\n\t$0\n}"},
	{Label: "enum", Kind: CompletionKindKeyword, Detail: "Declare an enum", InsertText: "enum ${1:Name} {\n\t$0\n}"},
	{Label: "event", Kind: CompletionKindKeyword, Detail: "Declare a domain event", InsertText: "event ${1:Name} {\n\t$0\n}"},
	{Label: "query", Kind: CompletionKindKeyword, Detail: "Query step", InsertText: "query ${1:name} = single ${2:Entity} where ${3:id} == ${4:input.id}"},
	{Label: "step", Kind: CompletionKindKeyword, Detail: "Pipeline step", InsertText: "step ${1:name} {\n\t$0\n}"},
	{Label: "input", Kind: CompletionKindKeyword, Detail: "Input parameters block", InsertText: "input {\n\t$0\n}"},
	{Label: "output", Kind: CompletionKindKeyword, Detail: "Output response block", InsertText: "output {\n\t$0\n}"},
}

var typeCompletions = []CompletionItem{
	{Label: "UUID", Kind: CompletionKindTypeParameter, Detail: "128-bit UUID scalar type"},
	{Label: "String", Kind: CompletionKindTypeParameter, Detail: "UTF-8 text scalar type"},
	{Label: "Int", Kind: CompletionKindTypeParameter, Detail: "64-bit integer scalar type"},
	{Label: "Boolean", Kind: CompletionKindTypeParameter, Detail: "Boolean scalar type"},
	{Label: "Money", Kind: CompletionKindTypeParameter, Detail: "High-precision monetary amount"},
	{Label: "Decimal", Kind: CompletionKindTypeParameter, Detail: "Fixed-point decimal scalar type"},
	{Label: "Timestamp", Kind: CompletionKindTypeParameter, Detail: "ISO-8601 UTC timestamp scalar type"},
	{Label: "Email", Kind: CompletionKindTypeParameter, Detail: "Validated email string scalar type"},
	{Label: "Date", Kind: CompletionKindTypeParameter, Detail: "Date scalar type (YYYY-MM-DD)"},
	{Label: "JSON", Kind: CompletionKindTypeParameter, Detail: "Arbitrary JSON payload scalar type"},
	{Label: "Bytes", Kind: CompletionKindTypeParameter, Detail: "Raw binary bytes scalar type"},
}

var traitCompletions = []CompletionItem{
	{Label: "@table", Kind: CompletionKindProperty, Detail: "Relational table mapping trait", InsertText: "@table(name = \"${1:table_name}\")"},
	{Label: "@version", Kind: CompletionKindProperty, Detail: "OCC optimistic concurrency control trait", InsertText: "@version"},
	{Label: "@route", Kind: CompletionKindProperty, Detail: "HTTP routing endpoint trait", InsertText: "@route(post = \"${1:/path}\")"},
	{Label: "@event_handler", Kind: CompletionKindProperty, Detail: "Asynchronous event subscriber trait", InsertText: "@event_handler(\"${1:EventName}\")"},
	{Label: "@cron", Kind: CompletionKindProperty, Detail: "Scheduled cron execution trait", InsertText: "@cron(\"${1:0 * * * *}\")"},
	{Label: "@audit", Kind: CompletionKindProperty, Detail: "Immutable audit log trait", InsertText: "@audit"},
	{Label: "@lock", Kind: CompletionKindProperty, Detail: "Pessimistic row locking trait", InsertText: "@lock(exclusive)"},
	{Label: "@idempotency", Kind: CompletionKindProperty, Detail: "Idempotency key enforcement trait", InsertText: "@idempotency"},
}
