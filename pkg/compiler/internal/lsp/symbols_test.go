package lsp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
)

const sampleBlueprint = `entity Wallet {
  id: uuid
  balance: decimal(12, 2)
}

error WalletNotFound {
  kind: not_found
  message: "Wallet does not exist"
}

action TransferMoney(input: TransferInput) -> bool {
  let sender = query one Wallet where id == input.id else WalletNotFound
  return true
}
`

func indexSample(t *testing.T) (*DocumentIndex, []*DocumentIndex) {
	t.Helper()
	const uri = "file:///ws/order.fdn"
	idx := IndexDocument(uri, "order.fdn", sampleBlueprint)
	if len(idx.Symbols) == 0 {
		t.Fatalf("expected symbols to be indexed, got none")
	}
	return idx, []*DocumentIndex{idx}
}

func findSymbol(idx *DocumentIndex, name string, kind SymbolKind) *Symbol {
	for i := range idx.Symbols {
		if idx.Symbols[i].Name == name && idx.Symbols[i].Kind == kind {
			return &idx.Symbols[i]
		}
	}
	return nil
}

func textAt(t *testing.T, content string, r Range) string {
	t.Helper()
	lines := strings.Split(content, "\n")
	if r.Start.Line < 0 || r.Start.Line >= len(lines) {
		t.Fatalf("range start line %d out of bounds", r.Start.Line)
	}
	line := lines[r.Start.Line]
	if r.Start.Character < 0 || r.End.Character > len(line) {
		t.Fatalf("range %+v out of bounds for line %q", r, line)
	}
	return line[r.Start.Character:r.End.Character]
}

func TestIndexDocument_CollectsDeclarations(t *testing.T) {
	idx, _ := indexSample(t)

	cases := []struct {
		name string
		kind SymbolKind
		line int // 1-based
	}{
		{"Wallet", KindEntity, 1},
		{"id", KindField, 2},
		{"balance", KindField, 3},
		{"WalletNotFound", KindError, 6},
		{"TransferMoney", KindAction, 11},
		{"input", KindParam, 11},
		{"sender", KindVariable, 12},
	}
	for _, tc := range cases {
		sym := findSymbol(idx, tc.name, tc.kind)
		if sym == nil {
			t.Fatalf("expected %s %q to be indexed", tc.kind, tc.name)
		}
		if sym.Line != tc.line {
			t.Errorf("%s %q: line = %d, want %d", tc.kind, tc.name, sym.Line, tc.line)
		}
		if textAt(t, sampleBlueprint, symbolRange(sym)) != tc.name {
			t.Errorf("%s %q: range %+v does not cover the name", tc.kind, tc.name, symbolRange(sym))
		}
	}
}

func TestLookupSymbol_PrefersEnclosingLocal(t *testing.T) {
	idx, _ := indexSample(t)

	// `sender` is declared on line 12; a reference on the same line resolves to it.
	got := LookupSymbol([]*DocumentIndex{idx}, idx.URI, "sender", 12)
	if got == nil || got.Kind != KindVariable {
		t.Fatalf("expected variable symbol for sender, got %+v", got)
	}

	// Before the declaration there is no local with that name.
	if got := LookupSymbol([]*DocumentIndex{idx}, idx.URI, "sender", 5); got != nil {
		t.Fatalf("expected no symbol before declaration, got %+v", got)
	}

	// Declarations in other files are still reachable.
	other := IndexDocument("file:///ws/other.fdn", "other.fdn", "entity Ledger {\n  id: uuid\n}\n")
	got = LookupSymbol([]*DocumentIndex{idx, other}, idx.URI, "Ledger", 12)
	if got == nil || got.Kind != KindEntity || got.URI != other.URI {
		t.Fatalf("expected cross-file entity lookup, got %+v", got)
	}
}

func TestComputeDefinition_ResolvesReferences(t *testing.T) {
	idx, indexes := indexSample(t)

	cases := []struct {
		line, character int // 0-based cursor
		want            string
		wantLine        int // 0-based declaration line
	}{
		{11, 27, "Wallet", 0},         // `Wallet` in the query
		{11, 46, "input", 10},         // `input` parameter reference
		{11, 8, "sender", 11},         // `sender` binding itself
		{11, 61, "WalletNotFound", 5}, // `else WalletNotFound`
	}

	for _, tc := range cases {
		locs := ComputeDefinition(indexes, idx.URI, sampleBlueprint, Position{Line: tc.line, Character: tc.character})
		if len(locs) == 0 {
			t.Fatalf("line %d col %d: expected a definition, got none", tc.line, tc.character)
		}
		if got := textAt(t, sampleBlueprint, locs[0].Range); got != tc.want {
			t.Errorf("line %d col %d: definition covers %q, want %q", tc.line, tc.character, got, tc.want)
		}
		if locs[0].Range.Start.Line != tc.wantLine {
			t.Errorf("%s: definition line = %d, want %d", tc.want, locs[0].Range.Start.Line, tc.wantLine)
		}
	}
}

func TestComputeHoverAt_DeclaredSymbolAndCatalog(t *testing.T) {
	idx, indexes := indexSample(t)

	// Hovering a declared entity shows its declaration.
	hover := ComputeHoverAt(indexes, idx.URI, sampleBlueprint, Position{Line: 11, Character: 27})
	if hover == nil {
		t.Fatalf("expected hover for declared entity, got nil")
	}
	if !strings.Contains(hover.Contents.Value, "### entity `Wallet`") {
		t.Errorf("expected entity hover, got: %s", hover.Contents.Value)
	}
	if !strings.Contains(hover.Contents.Value, "balance") {
		t.Errorf("expected fields in hover, got: %s", hover.Contents.Value)
	}

	// Non-symbol words fall back to the built-in keyword/trait/type catalog.
	fallback := ComputeHoverAt(indexes, idx.URI, sampleBlueprint, Position{Line: 5, Character: 2})
	if fallback == nil || !strings.Contains(fallback.Contents.Value, "`error` Construct") {
		t.Errorf("expected error keyword hover, got: %+v", fallback)
	}
}

func TestServer_DefinitionAndCapability(t *testing.T) {
	dir := t.TempDir()
	docPath := filepath.Join(dir, "order.fdn")
	if err := os.WriteFile(docPath, []byte(sampleBlueprint), 0o644); err != nil {
		t.Fatalf("write sample: %v", err)
	}
	rootURI := PathToURI(dir)
	docURI := PathToURI(docPath)

	didOpen := fmt.Sprintf(
		`{"jsonrpc":"2.0","method":"textDocument/didOpen","params":{"textDocument":{"uri":%q,"languageId":"foundation","version":1,"text":%q}}}`,
		docURI, sampleBlueprint)
	definition := fmt.Sprintf(
		`{"jsonrpc":"2.0","id":2,"method":"textDocument/definition","params":{"textDocument":{"uri":%q},"position":{"line":11,"character":27}}}`,
		docURI)
	initialize := fmt.Sprintf(
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"processId":null,"rootUri":%q,"capabilities":{},"workspaceFolders":[{"uri":%q,"name":"ws"}]}}`,
		rootURI, rootURI)
	initialized := `{"jsonrpc":"2.0","method":"initialized","params":{}}`
	shutdown := `{"jsonrpc":"2.0","id":3,"method":"shutdown","params":null}`
	exit := `{"jsonrpc":"2.0","method":"exit","params":{}}`

	var input bytes.Buffer
	for _, msg := range []string{initialize, initialized, didOpen, definition, shutdown, exit} {
		fmt.Fprintf(&input, "Content-Length: %d\r\n\r\n%s", len(msg), msg)
	}

	var output bytes.Buffer
	server := NewServer(&input, &output, func(sources []SourceDoc) ([]ir.Diagnostic, error) {
		return nil, nil
	})
	if err := server.Run(); err != nil {
		t.Fatalf("server run: %v", err)
	}

	reader := NewTransport(&output, &bytes.Buffer{})
	var got *Location
	var definitionProvider bool
	for {
		payload, err := reader.ReadMessage()
		if err != nil {
			break
		}
		var msg struct {
			ID     int             `json:"id"`
			Result json.RawMessage `json:"result"`
		}
		if err := json.Unmarshal(payload, &msg); err != nil {
			continue
		}
		switch msg.ID {
		case 1:
			var init InitializeResult
			if err := json.Unmarshal(msg.Result, &init); err == nil {
				definitionProvider = init.Capabilities.DefinitionProvider
			}
		case 2:
			var locs []Location
			if err := json.Unmarshal(msg.Result, &locs); err == nil && len(locs) > 0 {
				got = &locs[0]
			}
		}
		if msg.ID == 2 {
			break
		}
	}

	if !definitionProvider {
		t.Errorf("initialize must advertise definitionProvider")
	}
	if got == nil {
		t.Fatalf("expected a definition location")
	}
	if got.URI != docURI {
		t.Errorf("definition URI = %q, want %q", got.URI, docURI)
	}
	if text := textAt(t, sampleBlueprint, got.Range); text != "Wallet" {
		t.Errorf("definition covers %q, want %q", text, "Wallet")
	}
}
