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

	// cacheGen is bumped whenever the open documents or workspace roots change.
	// Cached source snapshots and per-document symbol indexes are validated against it.
	cacheGen    uint64
	sourceCache map[string]*sourceSnapshot // workspace root -> snapshot
	indexCache  map[string]*DocumentIndex  // URI -> parsed symbol index
}

// sourceSnapshot is a gathered view of a workspace root taken at a given generation.
type sourceSnapshot struct {
	gen     uint64
	sources []SourceDoc
	pathMap map[string]string
}

// skippedDirs are dependency and build directories that never hold Blueprint
// sources. Skipping them keeps workspace scans cheap on large repositories:
// node_modules alone can hold orders of magnitude more entries than the source
// tree, and a scan runs on every hover, definition and edit request.
var skippedDirs = map[string]bool{
	"node_modules": true,
	"vendor":       true,
	"dist":         true,
	"build":        true,
	"out":          true,
	"target":       true,
	"tmp":          true,
	"coverage":     true,
}

// NewWorkspace creates a Workspace initialized with the provided compiler function.
func NewWorkspace(compileFn CompileFunc) *Workspace {
	return &Workspace{
		docs:        make(map[string]string),
		compileFn:   compileFn,
		sourceCache: make(map[string]*sourceSnapshot),
		indexCache:  make(map[string]*DocumentIndex),
	}
}

// invalidateLocked drops cached source snapshots after the document set changes.
// Callers must hold the write lock.
func (w *Workspace) invalidateLocked() {
	w.cacheGen++
	clear(w.sourceCache)
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
	w.invalidateLocked()
}

// SetDocument stores or updates in-memory content for a document URI.
func (w *Workspace) SetDocument(uri, content string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if existing, ok := w.docs[uri]; ok && existing == content {
		return
	}
	w.docs[uri] = content
	w.invalidateLocked()
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
	if _, ok := w.docs[uri]; !ok {
		return
	}
	delete(w.docs, uri)
	delete(w.indexCache, uri)
	w.invalidateLocked()
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

// cachedSources returns the gathered sources for root, reusing a snapshot unless
// the open documents or workspace roots changed since it was captured.
// Callers must hold the write lock.
func (w *Workspace) cachedSources(root string) ([]SourceDoc, map[string]string, error) {
	if snap, ok := w.sourceCache[root]; ok && snap.gen == w.cacheGen {
		return snap.sources, snap.pathMap, nil
	}
	sources, pathMap, err := w.gatherSources(root)
	if err != nil {
		return nil, nil, err
	}
	w.sourceCache[root] = &sourceSnapshot{gen: w.cacheGen, sources: sources, pathMap: pathMap}
	return sources, pathMap, nil
}

// SymbolIndexes parses every Blueprint source in the workspace that contains
// targetURI, returning one index per file for hover and go-to-definition.
// Per-document indexes are cached and reused while their content is unchanged.
func (w *Workspace) SymbolIndexes(targetURI string) []*DocumentIndex {
	w.mu.Lock()
	defer w.mu.Unlock()

	targetPath, err := URIToPath(targetURI)
	if err != nil {
		return nil
	}

	root := w.findMatchingRoot(targetPath)
	sources, pathMap, err := w.cachedSources(root)
	if err != nil {
		return nil
	}

	indexes := make([]*DocumentIndex, 0, len(sources))
	for _, src := range sources {
		uri := pathMap[filepath.Clean(filepath.FromSlash(src.Path))]
		if uri == "" {
			uri = PathToURI(src.Path)
		}
		if cached, ok := w.indexCache[uri]; ok && cached.Content == src.Content {
			indexes = append(indexes, cached)
			continue
		}
		idx := IndexDocument(uri, src.Path, src.Content)
		w.indexCache[uri] = idx
		indexes = append(indexes, idx)
	}
	return indexes
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

	// 1. Scan filesystem for .fdn files, pruning hidden and build directories
	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if p != root && (strings.HasPrefix(d.Name(), ".") || skippedDirs[d.Name()]) {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(d.Name(), ".fdn") || strings.HasSuffix(d.Name(), ".fdn.json") {
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
