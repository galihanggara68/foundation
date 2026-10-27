package lsp

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
)

// SourceDoc represents a Blueprint file for compilation.
type SourceDoc struct {
	Path    string
	Content string
}

// CompileFunc abstracts the compiler pipeline for testing and decoupling.
type CompileFunc func(sources []SourceDoc) ([]ir.Diagnostic, error)

// Workspace tracks open documents, active workspace roots, and coordinates compilation.
type Workspace struct {
	mu        sync.RWMutex
	docs      map[string]string // URI -> file content
	roots     []string          // Local filesystem workspace roots
	compileFn CompileFunc
}

// NewWorkspace creates a Workspace initialized with the provided compiler function.
func NewWorkspace(compileFn CompileFunc) *Workspace {
	return &Workspace{
		docs:      make(map[string]string),
		compileFn: compileFn,
	}
}

// AddRoot registers a workspace root directory.
func (w *Workspace) AddRoot(rootPath string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	cleaned := filepath.Clean(rootPath)
	for _, r := range w.roots {
		if r == cleaned {
			return
		}
	}
	w.roots = append(w.roots, cleaned)
}

// SetDocument stores or updates in-memory content for a document URI.
func (w *Workspace) SetDocument(uri, content string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.docs[uri] = content
}

// GetDocument retrieves the current in-memory text for a document URI.
func (w *Workspace) GetDocument(uri string) (string, bool) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	c, ok := w.docs[uri]
	return c, ok
}

// RemoveDocument removes an open document from memory.
func (w *Workspace) RemoveDocument(uri string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	delete(w.docs, uri)
}

// TrackedURIs returns a copy of all document URIs currently open in memory.
func (w *Workspace) TrackedURIs() []string {
	w.mu.RLock()
	defer w.mu.RUnlock()
	uris := make([]string, 0, len(w.docs))
	for u := range w.docs {
		uris = append(uris, u)
	}
	return uris
}

// CompileWorkspace compiles the workspace relevant to targetURI and returns diagnostics keyed by URI.
func (w *Workspace) CompileWorkspace(targetURI string) (map[string][]Diagnostic, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	targetPath, err := URIToPath(targetURI)
	if err != nil {
		return nil, fmt.Errorf("invalid target URI %q: %w", targetURI, err)
	}

	root := w.findMatchingRoot(targetPath)
	sources, pathMap, err := w.gatherSources(root)
	if err != nil {
		return nil, err
	}

	rawDiags, err := w.compileFn(sources)
	if err != nil {
		return nil, err
	}

	// Initialize empty diagnostics for all tracked documents in this workspace
	results := make(map[string][]Diagnostic)
	for docURI := range w.docs {
		docPath, uErr := URIToPath(docURI)
		if uErr == nil && strings.HasPrefix(docPath, root) {
			results[docURI] = []Diagnostic{}
		}
	}

	for _, d := range rawDiags {
		diagFile := filepath.Clean(filepath.FromSlash(d.Span.File))
		uri, ok := pathMap[diagFile]
		if !ok {
			// Try matching against relative path or base name
			for p, u := range pathMap {
				if strings.HasSuffix(p, diagFile) || filepath.Base(p) == filepath.Base(diagFile) {
					uri = u
					break
				}
			}
		}
		if uri == "" {
			uri = targetURI
		}

		lspDiag := convertDiagnostic(d, w.docs[uri])
		results[uri] = append(results[uri], lspDiag)
	}

	return results, nil
}

func (w *Workspace) findMatchingRoot(filePath string) string {
	for _, r := range w.roots {
		rel, err := filepath.Rel(r, filePath)
		if err == nil && !strings.HasPrefix(rel, "..") {
			return r
		}
	}
	return filepath.Dir(filePath)
}

func (w *Workspace) gatherSources(root string) ([]SourceDoc, map[string]string, error) {
	pathMap := make(map[string]string) // filePath -> URI
	sourcesByPath := make(map[string]string)

	// 1. Scan filesystem for .fdn files
	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() && strings.HasPrefix(d.Name(), ".") && p != root {
			return filepath.SkipDir
		}
		if !d.IsDir() && (strings.HasSuffix(d.Name(), ".fdn") || strings.HasSuffix(d.Name(), ".fdn.json")) {
			cleanP := filepath.Clean(p)
			sourcesByPath[cleanP] = "" // Mark discovered, load later if not overridden
			pathMap[cleanP] = PathToURI(cleanP)
		}
		return nil
	})

	// 2. Overlay in-memory open documents
	for uri, content := range w.docs {
		p, err := URIToPath(uri)
		if err == nil {
			cleanP := filepath.Clean(p)
			sourcesByPath[cleanP] = content
			pathMap[cleanP] = uri
		}
	}

	// 3. Assemble SourceDoc slice in deterministic sorted path order
	sortedPaths := make([]string, 0, len(sourcesByPath))
	for p := range sourcesByPath {
		sortedPaths = append(sortedPaths, p)
	}
	sort.Strings(sortedPaths)

	var sources []SourceDoc
	for _, p := range sortedPaths {
		content := sourcesByPath[p]
		if content == "" {
			diskBytes, rErr := os.ReadFile(p)
			if rErr != nil {
				continue
			}
			content = string(diskBytes)
		}
		sources = append(sources, SourceDoc{
			Path:    filepath.ToSlash(p),
			Content: content,
		})
	}

	return sources, pathMap, nil
}

func convertDiagnostic(d ir.Diagnostic, docContent string) Diagnostic {
	lineIdx := d.Span.Line - 1
	if lineIdx < 0 {
		lineIdx = 0
	}
	colIdx := d.Span.Column - 1
	if colIdx < 0 {
		colIdx = 0
	}

	endCol := colIdx + 1
	if docContent != "" {
		lines := strings.Split(docContent, "\n")
		if lineIdx < len(lines) {
			lineText := lines[lineIdx]
			if colIdx < len(lineText) {
				// Extend to the end of the current word/token
				endCol = colIdx
				for endCol < len(lineText) && isTokenRune(rune(lineText[endCol])) {
					endCol++
				}
				if endCol == colIdx {
					endCol = colIdx + 1
				}
			}
		}
	}

	sev := SeverityError
	if d.Severity == ir.SeverityWarning {
		sev = SeverityWarning
	}

	return Diagnostic{
		Range: Range{
			Start: Position{Line: lineIdx, Character: colIdx},
			End:   Position{Line: lineIdx, Character: endCol},
		},
		Severity: sev,
		Code:     d.Code,
		Source:   "foundation",
		Message:  d.Message,
	}
}

func isTokenRune(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '@'
}
