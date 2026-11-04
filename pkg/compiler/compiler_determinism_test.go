package compiler

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/arch"
)

// TestCompilerDeterminism confirms that two compile passes on fixture blueprints produce byte-identical outputs.
func TestCompilerDeterminism(t *testing.T) {
	fixtureFiles := []string{
		"../../testdata/fixtures/minimal.fdn",
		"../../testdata/fixtures/ecommerce.fdn",
		"../../testdata/fixtures/composite_traits.fdn",
	}

	targets := []TargetConfig{
		{
			Name:         "go-backend",
			Kind:         "backend",
			Language:     "go",
			Architecture: "hexagonal",
		},
		{
			Name:         "go-sdk",
			Kind:         "sdk",
			Language:     "go",
			Architecture: "client",
		},
		{
			Name:         "ts-backend",
			Kind:         "backend",
			Language:     "typescript",
			Architecture: "clean",
		},
		{
			Name:         "ts-sdk",
			Kind:         "sdk",
			Language:     "typescript",
			Architecture: "client",
		},
	}

	for _, fixturePath := range fixtureFiles {
		fixtureName := filepath.Base(fixturePath)
		t.Run(fixtureName, func(t *testing.T) {
			content, readErr := os.ReadFile(fixturePath)
			if readErr != nil {
				t.Fatalf("failed to read fixture file %s: %v", fixturePath, readErr)
			}

			sources := []SourceFile{
				{
					Path:    fixtureName,
					Content: string(content),
				},
			}

			// Compile run 1
			sem1, diags1 := Compile(sources)
			if HasErrors(diags1) {
				t.Fatalf("run 1 produced compile errors on %s: %+v", fixtureName, diags1)
			}

			// Compile run 2
			sem2, diags2 := Compile(sources)
			if HasErrors(diags2) {
				t.Fatalf("run 2 produced compile errors on %s: %+v", fixtureName, diags2)
			}

			// Assert identical diagnostics from compiler.Compile
			assertDiagnosticsMatch(t, diags1, diags2)

			// Assert identical SemanticIR JSON bytes
			json1, mErr1 := json.Marshal(sem1)
			if mErr1 != nil {
				t.Fatalf("failed to marshal sem1: %v", mErr1)
			}
			json2, mErr2 := json.Marshal(sem2)
			if mErr2 != nil {
				t.Fatalf("failed to marshal sem2: %v", mErr2)
			}
			if !bytes.Equal(json1, json2) {
				t.Fatalf("SemanticIR JSON differs across compile runs for %s", fixtureName)
			}

			// Build profile for lowering targets
			bp := &arch.BuildProfile{
				Project: arch.ProjectConfig{
					Name:   "fixturetest",
					Module: "example.com/fixturetest",
				},
				Storage: arch.StorageConfig{
					Dialect: "postgres",
					Mode:    "declarative",
				},
				Targets: targets,
			}

			for _, target := range targets {
				targetLabel := target.Language + "-" + target.Architecture
				t.Run(targetLabel, func(t *testing.T) {
					// Run 1: lower and emit
					storagePlan1, _, _ := arch.BuildStoragePlan("", sem1, bp, false)
					archIR1, lowDiags1 := arch.Lower(sem1, bp, target)
					if HasErrors(lowDiags1) {
						t.Fatalf("run 1 lowering failed for target %s: %+v", targetLabel, lowDiags1)
					}
					if target.Kind == "backend" {
						archIR1.Storage = storagePlan1
					}
					emission1, emitDiags1 := EmitBuiltin(archIR1)
					if HasErrors(emitDiags1) {
						t.Fatalf("run 1 emission failed for target %s: %+v", targetLabel, emitDiags1)
					}

					// Run 2: lower and emit
					storagePlan2, _, _ := arch.BuildStoragePlan("", sem2, bp, false)
					archIR2, lowDiags2 := arch.Lower(sem2, bp, target)
					if HasErrors(lowDiags2) {
						t.Fatalf("run 2 lowering failed for target %s: %+v", targetLabel, lowDiags2)
					}
					if target.Kind == "backend" {
						archIR2.Storage = storagePlan2
					}
					emission2, emitDiags2 := EmitBuiltin(archIR2)
					if HasErrors(emitDiags2) {
						t.Fatalf("run 2 emission failed for target %s: %+v", targetLabel, emitDiags2)
					}

					// Assert diagnostic determinism
					assertDiagnosticsMatch(t, lowDiags1, lowDiags2)
					assertDiagnosticsMatch(t, emitDiags1, emitDiags2)

					// Assert test command determinism
					if emission1.TestCommand != emission2.TestCommand {
						t.Errorf("test command mismatch: %q != %q", emission1.TestCommand, emission2.TestCommand)
					}

					// Assert tracked files byte-for-byte determinism
					assertFileMapsMatch(t, "tracked file", emission1.TrackedFiles, emission2.TrackedFiles)

					// Assert stub files byte-for-byte determinism
					assertFileMapsMatch(t, "stub file", emission1.StubFiles, emission2.StubFiles)
				})
			}
		})
	}
}

func assertDiagnosticsMatch(t *testing.T, d1, d2 []Diagnostic) {
	t.Helper()
	if len(d1) != len(d2) {
		t.Fatalf("diagnostic count mismatch: %d != %d", len(d1), len(d2))
	}
	for i := range d1 {
		if d1[i].Code != d2[i].Code ||
			d1[i].Severity != d2[i].Severity ||
			d1[i].Message != d2[i].Message ||
			d1[i].Span != d2[i].Span {
			t.Errorf("diagnostic %d differs: %+v vs %+v", i, d1[i], d2[i])
		}
	}
}

func assertFileMapsMatch(t *testing.T, label string, m1, m2 map[string][]byte) {
	t.Helper()
	if len(m1) != len(m2) {
		t.Fatalf("%s count mismatch: run 1 had %d, run 2 had %d", label, len(m1), len(m2))
	}

	keys1 := make([]string, 0, len(m1))
	for k := range m1 {
		keys1 = append(keys1, k)
	}
	sort.Strings(keys1)

	keys2 := make([]string, 0, len(m2))
	for k := range m2 {
		keys2 = append(keys2, k)
	}
	sort.Strings(keys2)

	for i := range keys1 {
		if keys1[i] != keys2[i] {
			t.Errorf("%s key set mismatch at index %d: %q vs %q", label, i, keys1[i], keys2[i])
		}
	}

	for _, k := range keys1 {
		v1 := m1[k]
		v2, ok := m2[k]
		if !ok {
			t.Errorf("second run missing %s %q", label, k)
			continue
		}
		if !bytes.Equal(v1, v2) {
			t.Errorf("%s %q byte content mismatch across runs", label, k)
		}
	}
}
