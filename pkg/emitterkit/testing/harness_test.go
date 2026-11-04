package testing_test

import (
	"fmt"
	stdtesting "testing"

	"github.com/galihanggara68/foundation/pkg/compiler"
	"github.com/galihanggara68/foundation/pkg/emitterkit"
	emittertesting "github.com/galihanggara68/foundation/pkg/emitterkit/testing"
)

type recordTB struct {
	errors []string
	fatals []string
	failed bool
}

func (m *recordTB) Helper() {}

func (m *recordTB) Errorf(format string, args ...any) {
	m.failed = true
	m.errors = append(m.errors, fmt.Sprintf(format, args...))
}

func (m *recordTB) Fatalf(format string, args ...any) {
	m.failed = true
	m.fatals = append(m.fatals, fmt.Sprintf(format, args...))
}

func TestAssertDeterministic_DeterministicGenerator_Passes(t *stdtesting.T) {
	mock := &recordTB{}
	h := emittertesting.NewHarnessTB(mock)

	genFunc := func() (emitterkit.GenerationResponse, error) {
		return emitterkit.GenerationResponse{
			ProtocolVersion: 1,
			TestCommand:     "go test ./...",
			Files: map[string]string{
				"domain/item.go": "package domain\n\ntype Item struct {}\n",
			},
			Stubs: map[string]string{
				"stub.go": "// stub\n",
			},
			Diagnostics: []emitterkit.Diagnostic{
				{
					Code:     "FDN0001",
					Severity: compiler.SeverityInfo,
					Message:  "all clear",
					Span:     compiler.Span{File: "domain.fdn", Line: 1, Column: 1},
				},
			},
		}, nil
	}

	h.AssertDeterministic(genFunc)

	if mock.failed {
		t.Fatalf("expected deterministic run to pass, got errors: %v", mock.errors)
	}
}

func TestAssertDeterministic_NonDeterministicGenerator_Fails(t *stdtesting.T) {
	mock := &recordTB{}
	h := emittertesting.NewHarnessTB(mock)

	counter := 0
	genFunc := func() (emitterkit.GenerationResponse, error) {
		counter++
		return emitterkit.GenerationResponse{
			ProtocolVersion: 1,
			Files: map[string]string{
				"domain/item.go": fmt.Sprintf("// version %d", counter),
			},
		}, nil
	}

	h.AssertDeterministic(genFunc)

	if !mock.failed {
		t.Fatalf("expected non-deterministic run to fail")
	}
	if len(mock.errors) == 0 {
		t.Fatalf("expected error messages recorded for non-deterministic generator")
	}
}

func TestAssertDeterministic_GenerationError_RecordsFatal(t *stdtesting.T) {
	mock := &recordTB{}
	h := emittertesting.NewHarnessTB(mock)

	counter := 0
	genFunc := func() (emitterkit.GenerationResponse, error) {
		counter++
		if counter == 2 {
			return emitterkit.GenerationResponse{}, fmt.Errorf("simulated generator crash")
		}
		return emitterkit.GenerationResponse{ProtocolVersion: 1}, nil
	}

	h.AssertDeterministic(genFunc)

	if !mock.failed {
		t.Fatalf("expected failing generator to fail")
	}
	if len(mock.fatals) == 0 {
		t.Fatalf("expected fatal message recorded for failing generator")
	}
}

func TestAssertFileContent_MatchesExpected(t *stdtesting.T) {
	mock := &recordTB{}
	h := emittertesting.NewHarnessTB(mock)

	resp := emitterkit.GenerationResponse{
		Files: map[string]string{
			"readme.txt": "hello world",
		},
	}

	h.AssertFileContent(resp, "readme.txt", "hello world")
	if mock.failed {
		t.Fatalf("expected matching content to pass")
	}

	h.AssertFileContent(resp, "readme.txt", "wrong content")
	if !mock.failed {
		t.Fatalf("expected mismatching content to fail")
	}
}

func TestLoadFixture_MinimalFixture_Succeeds(t *stdtesting.T) {
	h := emittertesting.NewHarness(t)
	sem, err := h.LoadFixture("../../../testdata/fixtures/minimal.fdn")
	if err != nil {
		t.Fatalf("failed to load fixture: %v", err)
	}
	if sem == nil {
		t.Fatalf("expected non-nil SemanticIR")
	}
	if len(sem.Entities) != 1 || sem.Entities[0].Name != "Item" {
		t.Fatalf("expected Item entity, got %+v", sem.Entities)
	}
}
