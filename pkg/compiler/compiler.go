package compiler

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/arch"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/emit"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/frontend"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
)

// Public type aliases preserving the Single Public Compiler Driver Seam (pkg/compiler).
type (
	Span           = ir.Span
	Severity       = ir.Severity
	Diagnostic     = ir.Diagnostic
	SemanticIR     = ir.SemanticIR
	BuildProfile   = arch.BuildProfile
	ArchitectureIR = arch.ArchitectureIR
)

const (
	SeverityError   = ir.SeverityError
	SeverityWarning = ir.SeverityWarning
)

// SourceFile represents an in-memory or loaded Blueprint source document.
type SourceFile struct {
	Path    string
	Content string
}

// Compile validates an in-memory collection of Blueprint source files and returns their canonical SemanticIR.
// It performs no filesystem I/O, allowing pure in-process compilation for testing, IDEs, or language servers.
func Compile(sources []SourceFile) (*SemanticIR, []Diagnostic) {
	if len(sources) == 0 {
		return nil, []Diagnostic{{
			Code:     "FDN0002",
			Severity: SeverityError,
			Message:  "no .fdn Blueprint files found in workspace",
			Span:     Span{File: "workspace", Line: 1, Column: 1},
		}}
	}

	sem := &SemanticIR{}
	var diags []Diagnostic

	for _, sf := range sources {
		rel := filepath.ToSlash(sf.Path)
		var fileIR *SemanticIR
		var fileDiags []Diagnostic
		if strings.HasSuffix(rel, ".fdn.json") {
			fileIR = &SemanticIR{}
			if uErr := json.Unmarshal([]byte(sf.Content), fileIR); uErr != nil {
				fileDiags = append(fileDiags, Diagnostic{
					Code:     "FDN0006",
					Severity: SeverityError,
					Message:  fmt.Sprintf("failed to parse SemanticIR JSON: %v", uErr),
					Span:     Span{File: rel, Line: 1, Column: 1},
				})
				fileIR = nil
			}
		} else {
			fileIR, fileDiags = frontend.ParseFile(rel, sf.Content)
		}
		diags = append(diags, fileDiags...)
		if fileIR != nil {
			sem.Actors = append(sem.Actors, fileIR.Actors...)
			if sem.Actor == nil && len(fileIR.Actors) > 0 {
				sem.Actor = &fileIR.Actors[0]
			}
			sem.Enums = append(sem.Enums, fileIR.Enums...)
			sem.Values = append(sem.Values, fileIR.Values...)
			sem.Entities = append(sem.Entities, fileIR.Entities...)
			sem.Errors = append(sem.Errors, fileIR.Errors...)
			sem.Events = append(sem.Events, fileIR.Events...)
			sem.Policies = append(sem.Policies, fileIR.Policies...)
			sem.Slots = append(sem.Slots, fileIR.Slots...)
			sem.Actions = append(sem.Actions, fileIR.Actions...)
			sem.Scenarios = append(sem.Scenarios, fileIR.Scenarios...)
		}
	}

	if len(diags) > 0 {
		return nil, diags
	}

	valDiags := validateSemanticIR(sem)
	if len(valDiags) > 0 {
		return nil, valDiags
	}

	sem.Canonicalize()
	return sem, nil
}

// Check scans workspaceDir for .fdn and .fdn.json Blueprints, parses them, and runs all semantic validation passes.
func Check(workspaceDir string) (*SemanticIR, []Diagnostic) {
	files, err := findFDNFiles(workspaceDir)
	if err != nil {
		return nil, []Diagnostic{{
			Code:     "FDN0001",
			Severity: SeverityError,
			Message:  fmt.Sprintf("failed to scan workspace: %v", err),
			Span:     Span{File: workspaceDir, Line: 1, Column: 1},
		}}
	}
	if len(files) == 0 {
		return nil, []Diagnostic{{
			Code:     "FDN0002",
			Severity: SeverityError,
			Message:  "no .fdn Blueprint files found in workspace",
			Span:     Span{File: workspaceDir, Line: 1, Column: 1},
		}}
	}

	var sources []SourceFile
	var diags []Diagnostic

	for _, f := range files {
		src, readErr := os.ReadFile(f)
		if readErr != nil {
			diags = append(diags, Diagnostic{
				Code:     "FDN0001",
				Severity: SeverityError,
				Message:  fmt.Sprintf("failed to read file: %v", readErr),
				Span:     Span{File: f, Line: 1, Column: 1},
			})
			continue
		}
		rel, relErr := filepath.Rel(workspaceDir, f)
		if relErr != nil {
			rel = filepath.Base(f)
		}
		sources = append(sources, SourceFile{
			Path:    rel,
			Content: string(src),
		})
	}

	if len(diags) > 0 {
		return nil, diags
	}

	return Compile(sources)
}

func findFDNFiles(dir string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && strings.HasPrefix(d.Name(), ".") && path != dir {
			return filepath.SkipDir
		}
		if !d.IsDir() && (strings.HasSuffix(d.Name(), ".fdn") || strings.HasSuffix(d.Name(), ".fdn.json")) {
			out = append(out, path)
		}
		return nil
	})
	sort.Strings(out)
	return out, err
}

// IR validates all .fdn Blueprints in workspaceDir and returns the canonical JSON representation of SemanticIR.
func IR(workspaceDir string) ([]byte, []Diagnostic) {
	sem, diags := Check(workspaceDir)
	if len(diags) > 0 {
		return nil, diags
	}
	b, err := json.MarshalIndent(sem, "", "  ")
	if err != nil {
		return nil, []Diagnostic{{
			Code:     "FDN0005",
			Severity: SeverityError,
			Message:  fmt.Sprintf("failed to marshal SemanticIR: %v", err),
			Span:     Span{File: workspaceDir, Line: 1, Column: 1},
		}}
	}
	return append(b, '\n'), nil
}

// BuildOptions configures the compiler Build behavior.
type BuildOptions struct {
	Force bool
}

// BuildResult returns the validated SemanticIR and lowered ArchitectureIRs produced by Build.
type BuildResult struct {
	Semantic      *SemanticIR
	Profile       *BuildProfile
	Architectures []*ArchitectureIR
}

// Build compiles all .fdn Blueprints in workspaceDir according to foundation.build.yaml,
// lowers them into ArchitectureIR, reconciles .foundation-manifest.json, and emits target files.
func Build(workspaceDir string, opts BuildOptions) (*BuildResult, []Diagnostic) {
	sem, diags := Check(workspaceDir)
	if len(diags) > 0 {
		return nil, diags
	}

	bp, bpDiags := arch.LoadBuildProfile(workspaceDir)
	if len(bpDiags) > 0 {
		return nil, bpDiags
	}

	res := &BuildResult{
		Semantic: sem,
		Profile:  bp,
	}

	storagePlan, nextLock, storageDiags := arch.BuildStoragePlan(workspaceDir, sem, bp, opts.Force)
	if len(storageDiags) > 0 {
		return nil, storageDiags
	}

	reg := defaultRegistry()

	for _, target := range bp.Targets {
		archIR, lowDiags := arch.Lower(sem, bp, target)
		if len(lowDiags) > 0 {
			return nil, lowDiags
		}
		if target.Kind == "backend" {
			archIR.Storage = storagePlan
		}
		res.Architectures = append(res.Architectures, archIR)

		key := arch.TargetKey{
			Kind:         target.Kind,
			Language:     target.Language,
			Architecture: target.Architecture,
		}
		emitter, ok := reg.emitters[key]
		if !ok {
			return nil, []Diagnostic{{
				Code:     "FDN0300",
				Severity: SeverityError,
				Message:  fmt.Sprintf("no emitter registered for target: kind=%s language=%s architecture=%s", target.Kind, target.Language, target.Architecture),
				Span:     Span{File: "foundation.build.yaml", Line: 1, Column: 1},
			}}
		}

		emission, emitDiags := emitter.Emit(archIR)
		if len(emitDiags) > 0 {
			return nil, emitDiags
		}
		outDir := filepath.Join(workspaceDir, filepath.FromSlash(target.OutDir))
		recDiags := emit.ReconcileManifest(outDir, emission.TrackedFiles, emission.StubFiles, opts.Force)
		if len(recDiags) > 0 {
			return nil, recDiags
		}
	}

	if lockDiags := arch.WriteLockFile(workspaceDir, nextLock); len(lockDiags) > 0 {
		return nil, lockDiags
	}

	return res, nil
}
