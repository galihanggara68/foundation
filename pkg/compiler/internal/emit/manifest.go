package emit

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/arch"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
)

// Emission holds the deterministic virtual file trees produced by a TargetEmitter.
type Emission struct {
	TrackedFiles map[string][]byte
	StubFiles    map[string][]byte
}

// TargetEmitter translates a lowered ArchitectureIR into target source files (Strategy pattern).
type TargetEmitter interface {
	Emit(archIR *arch.ArchitectureIR) (Emission, []ir.Diagnostic)
}

const ManifestFileName = ".foundation-manifest.json"

type GenerationManifest struct {
	Version int               `json:"version"`
	Files   map[string]string `json:"files"`
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// pruneEmptyDirs removes directories that became empty after pruning stale generated files,
// so switching architectures never leaves the previous layout's empty package tree behind.
// It climbs from each pruned file's directory toward outDir and stops at the first non-empty directory.
func pruneEmptyDirs(outDir string, relDirs []string) {
	seen := make(map[string]bool)
	for _, d := range relDirs {
		if d == "." || seen[d] {
			continue
		}
		seen[d] = true
		for cur := d; cur != "."; cur = path.Dir(cur) {
			abs := filepath.Join(outDir, filepath.FromSlash(cur))
			entries, err := os.ReadDir(abs)
			if err != nil || len(entries) > 0 {
				break
			}
			if err := os.Remove(abs); err != nil {
				break
			}
		}
	}
}

// ReconcileManifest compares the newly emitted files with the existing .foundation-manifest.json,
// detects human conflicts, prunes stale generated files, writes tracked files, and writes create-if-missing stubs.
func ReconcileManifest(outDir string, trackedFiles map[string][]byte, stubFiles map[string][]byte, force bool) []ir.Diagnostic {
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return []ir.Diagnostic{{
			Code:     "FDN0400",
			Severity: ir.SeverityError,
			Message:  fmt.Sprintf("failed to create output directory %s: %v", outDir, err),
			Span:     ir.Span{File: outDir, Line: 1, Column: 1},
		}}
	}

	manifestPath := filepath.Join(outDir, ManifestFileName)
	var prev GenerationManifest
	prev.Files = make(map[string]string)

	if raw, err := os.ReadFile(manifestPath); err == nil {
		if jsonErr := json.Unmarshal(raw, &prev); jsonErr != nil {
			return []ir.Diagnostic{{
				Code:     "FDN0401",
				Severity: ir.SeverityError,
				Message:  fmt.Sprintf("corrupt %s: %v", ManifestFileName, jsonErr),
				Span:     ir.Span{File: manifestPath, Line: 1, Column: 1},
			}}
		}
	}

	// 1. Check for manual modifications to previously tracked files (unless --force)
	if !force {
		var conflicts []ir.Diagnostic
		var prevPaths []string
		for rel := range prev.Files {
			prevPaths = append(prevPaths, rel)
		}
		sort.Strings(prevPaths)

		for _, rel := range prevPaths {
			expectedHash := prev.Files[rel]
			diskPath := filepath.Join(outDir, filepath.FromSlash(rel))
			diskBytes, err := os.ReadFile(diskPath)
			if err != nil {
				continue
			}
			actualHash := sha256Hex(diskBytes)
			if actualHash != expectedHash {
				conflicts = append(conflicts, ir.Diagnostic{
					Code:     "FDN0402",
					Severity: ir.SeverityError,
					Message:  fmt.Sprintf("manifest conflict: tracked file %q was manually modified (use --force to overwrite)", rel),
					Span:     ir.Span{File: rel, Line: 1, Column: 1},
				})
			}
		}
		if len(conflicts) > 0 {
			return conflicts
		}
	}

	// 2. Prune stale files that were in the previous manifest but are no longer emitted
	var prunedDirs []string
	for rel := range prev.Files {
		if _, stillPresent := trackedFiles[rel]; !stillPresent {
			diskPath := filepath.Join(outDir, filepath.FromSlash(rel))
			_ = os.Remove(diskPath)
			prunedDirs = append(prunedDirs, path.Dir(rel))
		}
	}
	pruneEmptyDirs(outDir, prunedDirs)

	// 3. Write all tracked files in deterministic order and compute new manifest hashes
	nextManifest := GenerationManifest{
		Version: 1,
		Files:   make(map[string]string, len(trackedFiles)),
	}

	var paths []string
	for rel := range trackedFiles {
		paths = append(paths, rel)
	}
	sort.Strings(paths)

	for _, rel := range paths {
		content := trackedFiles[rel]
		diskPath := filepath.Join(outDir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(diskPath), 0755); err != nil {
			return []ir.Diagnostic{{
				Code:     "FDN0400",
				Severity: ir.SeverityError,
				Message:  fmt.Sprintf("failed to create parent directory for %s: %v", rel, err),
				Span:     ir.Span{File: rel, Line: 1, Column: 1},
			}}
		}
		if err := os.WriteFile(diskPath, content, 0644); err != nil {
			return []ir.Diagnostic{{
				Code:     "FDN0400",
				Severity: ir.SeverityError,
				Message:  fmt.Sprintf("failed to write file %s: %v", rel, err),
				Span:     ir.Span{File: rel, Line: 1, Column: 1},
			}}
		}
		nextManifest.Files[rel] = sha256Hex(content)
	}

	// 4. Write create-if-missing stub files (never recorded in manifest)
	var stubPaths []string
	for rel := range stubFiles {
		stubPaths = append(stubPaths, rel)
	}
	sort.Strings(stubPaths)

	for _, rel := range stubPaths {
		diskPath := filepath.Join(outDir, filepath.FromSlash(rel))
		if _, err := os.Stat(diskPath); err == nil {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(diskPath), 0755); err != nil {
			return []ir.Diagnostic{{
				Code:     "FDN0400",
				Severity: ir.SeverityError,
				Message:  fmt.Sprintf("failed to create stub directory for %s: %v", rel, err),
				Span:     ir.Span{File: rel, Line: 1, Column: 1},
			}}
		}
		if err := os.WriteFile(diskPath, stubFiles[rel], 0644); err != nil {
			return []ir.Diagnostic{{
				Code:     "FDN0400",
				Severity: ir.SeverityError,
				Message:  fmt.Sprintf("failed to write stub %s: %v", rel, err),
				Span:     ir.Span{File: rel, Line: 1, Column: 1},
			}}
		}
	}

	// 5. Write deterministic .foundation-manifest.json
	manifestBytes, err := json.MarshalIndent(nextManifest, "", "  ")
	if err != nil {
		return []ir.Diagnostic{{
			Code:     "FDN0401",
			Severity: ir.SeverityError,
			Message:  fmt.Sprintf("failed to serialize manifest: %v", err),
			Span:     ir.Span{File: manifestPath, Line: 1, Column: 1},
		}}
	}
	manifestBytes = append(manifestBytes, '\n')
	if err := os.WriteFile(manifestPath, manifestBytes, 0644); err != nil {
		return []ir.Diagnostic{{
			Code:     "FDN0401",
			Severity: ir.SeverityError,
			Message:  fmt.Sprintf("failed to write manifest: %v", err),
			Span:     ir.Span{File: manifestPath, Line: 1, Column: 1},
		}}
	}

	return nil
}
