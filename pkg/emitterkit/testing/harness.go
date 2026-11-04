package testing

import (
	"fmt"
	"os"
	stdtesting "testing"

	"github.com/galihanggara68/foundation/pkg/compiler"
	"github.com/galihanggara68/foundation/pkg/emitterkit"
)

// TB defines the minimal testing interface for test helpers.
type TB interface {
	Helper()
	Errorf(format string, args ...any)
	Fatalf(format string, args ...any)
}

// Harness provides test helpers for external generator plugin developers.
type Harness struct {
	t TB
}

// NewHarness creates a new testing Harness bound to t.
func NewHarness(t *stdtesting.T) *Harness {
	return &Harness{t: t}
}

// NewHarnessTB creates a new testing Harness bound to tb.
func NewHarnessTB(tb TB) *Harness {
	return &Harness{t: tb}
}

// LoadFixture loads and compiles a fixture file into SemanticIR.
func (h *Harness) LoadFixture(path string) (*emitterkit.SemanticIR, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read fixture file %s: %w", path, err)
	}
	sem, diags := compiler.Compile([]compiler.SourceFile{
		{
			Path:    path,
			Content: string(data),
		},
	})
	if compiler.HasErrors(diags) {
		return nil, fmt.Errorf("compile fixture %s failed: %+v", path, diags)
	}
	return sem, nil
}

// AssertDeterministic runs genFunc twice.
// The helper confirms that both runs produce identical responses.
func (h *Harness) AssertDeterministic(genFunc func() (emitterkit.GenerationResponse, error)) {
	h.t.Helper()
	resp1, err1 := genFunc()
	if err1 != nil {
		h.t.Fatalf("first generation run failed: %v", err1)
		return
	}
	resp2, err2 := genFunc()
	if err2 != nil {
		h.t.Fatalf("second generation run failed: %v", err2)
		return
	}

	if resp1.ProtocolVersion != resp2.ProtocolVersion {
		h.t.Errorf("protocol version mismatch: first run had %d, second run had %d", resp1.ProtocolVersion, resp2.ProtocolVersion)
	}

	if resp1.TestCommand != resp2.TestCommand {
		h.t.Errorf("test command mismatch: first run had %q, second run had %q", resp1.TestCommand, resp2.TestCommand)
	}

	compareStringMaps(h.t, "file", resp1.Files, resp2.Files)
	compareStringMaps(h.t, "stub", resp1.Stubs, resp2.Stubs)
	compareDiagnostics(h.t, resp1.Diagnostics, resp2.Diagnostics)
}

// AssertFileContent confirms that path exists in resp and matches expected.
func (h *Harness) AssertFileContent(resp emitterkit.GenerationResponse, path string, expected string) {
	h.t.Helper()
	content, ok := resp.Files[path]
	if !ok {
		content, ok = resp.Stubs[path]
	}
	if !ok {
		h.t.Errorf("response does not contain file or stub %q", path)
		return
	}
	if content != expected {
		h.t.Errorf("file %q content mismatch: got %q, want %q", path, content, expected)
	}
}

func compareStringMaps(t TB, kind string, m1, m2 map[string]string) {
	t.Helper()
	if len(m1) != len(m2) {
		t.Errorf("%s count mismatch: first run produced %d entries, second run produced %d", kind, len(m1), len(m2))
	}
	for k, v1 := range m1 {
		v2, ok := m2[k]
		if !ok {
			t.Errorf("second run missing %s %q", kind, k)
			continue
		}
		if v1 != v2 {
			t.Errorf("%s %q content mismatch between runs", kind, k)
		}
	}
	for k := range m2 {
		if _, ok := m1[k]; !ok {
			t.Errorf("first run missing %s %q", kind, k)
		}
	}
}

func compareDiagnostics(t TB, d1, d2 []emitterkit.Diagnostic) {
	t.Helper()
	if len(d1) != len(d2) {
		t.Errorf("diagnostic count mismatch: first run had %d, second run had %d", len(d1), len(d2))
		return
	}
	for i := range d1 {
		if d1[i].Code != d2[i].Code ||
			d1[i].Severity != d2[i].Severity ||
			d1[i].Message != d2[i].Message ||
			d1[i].Span != d2[i].Span {
			t.Errorf("diagnostic at index %d differs: %+v vs %+v", i, d1[i], d2[i])
		}
	}
}
