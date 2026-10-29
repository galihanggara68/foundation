package lsp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
)

func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", rel, err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

func newTestWorkspace(t *testing.T, root string) *Workspace {
	t.Helper()
	w := NewWorkspace(func(sources []SourceDoc) ([]ir.Diagnostic, error) { return nil, nil })
	w.AddRoot(root)
	return w
}

func TestGatherSources_PrunesBuildAndDependencyDirs(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "src/a.fdn", "entity A {\n  id: uuid\n}\n")
	writeFile(t, root, "node_modules/pkg/b.fdn", "entity B {\n  id: uuid\n}\n")
	writeFile(t, root, "dist/c.fdn", "entity C {\n  id: uuid\n}\n")
	writeFile(t, root, ".hidden/d.fdn", "entity D {\n  id: uuid\n}\n")

	w := newTestWorkspace(t, root)
	target := PathToURI(filepath.Join(root, "src", "a.fdn"))

	indexes := w.SymbolIndexes(target)
	if len(indexes) != 1 {
		t.Fatalf("expected only the source file to be indexed, got %d", len(indexes))
	}
	if !strings.HasSuffix(indexes[0].URI, "/src/a.fdn") {
		t.Fatalf("unexpected indexed file %q", indexes[0].URI)
	}
}

func TestSymbolIndexes_ReusesCacheUntilDocumentsChange(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "a.fdn", "entity A {\n  id: uuid\n}\n")

	w := newTestWorkspace(t, root)
	uri := PathToURI(filepath.Join(root, "a.fdn"))

	first := w.SymbolIndexes(uri)
	if len(first) != 1 {
		t.Fatalf("expected one indexed file, got %d", len(first))
	}

	// Unchanged workspace: the cached index must be reused as-is.
	second := w.SymbolIndexes(uri)
	if len(second) != 1 || second[0] != first[0] {
		t.Fatalf("expected the cached index to be reused")
	}

	// A document change invalidates the cache and re-parses the file.
	w.SetDocument(uri, "entity Renamed {\n  id: uuid\n}\n")
	third := w.SymbolIndexes(uri)
	if len(third) != 1 {
		t.Fatalf("expected one indexed file after edit, got %d", len(third))
	}
	if third[0] == first[0] {
		t.Fatalf("expected a fresh index after the document changed")
	}
	if findSymbol(third[0], "Renamed", KindEntity) == nil {
		t.Fatalf("expected the edited document to be re-indexed")
	}

	// Re-sending identical content must not invalidate anything.
	w.SetDocument(uri, "entity Renamed {\n  id: uuid\n}\n")
	fourth := w.SymbolIndexes(uri)
	if len(fourth) != 1 || fourth[0] != third[0] {
		t.Fatalf("expected cache reuse when document content is unchanged")
	}
}

func TestCompileWorkspace_SeesOpenDocumentEdits(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "a.fdn", "entity A {\n  id: uuid\n}\n")

	var compiled []SourceDoc
	w := NewWorkspace(func(sources []SourceDoc) ([]ir.Diagnostic, error) {
		compiled = sources
		return nil, nil
	})
	w.AddRoot(root)

	uri := PathToURI(filepath.Join(root, "a.fdn"))
	w.SetDocument(uri, "entity Edited {\n  id: uuid\n}\n")

	if _, err := w.CompileWorkspace(uri); err != nil {
		t.Fatalf("compile workspace: %v", err)
	}
	if len(compiled) != 1 || !strings.Contains(compiled[0].Content, "entity Edited") {
		t.Fatalf("expected the open document to override the on-disk content, got %+v", compiled)
	}
}
