package testing_test

import (
	"strings"
	stdtesting "testing"

	"github.com/galihanggara68/foundation/pkg/emitterkit"
	emittertesting "github.com/galihanggara68/foundation/pkg/emitterkit/testing"
)

func TestRunSmokeSuite_PassingGoTest_Succeeds(t *stdtesting.T) {
	h := emittertesting.NewHarness(t)

	resp := emitterkit.GenerationResponse{
		Files: map[string]string{
			"go.mod":       "module testsmokepkg\n\ngo 1.22\n",
			"calc.go":      "package calc\n\nfunc Add(a, b int) int { return a + b }\n",
			"calc_test.go": "package calc\n\nimport \"testing\"\n\nfunc TestAdd(t *testing.T) {\n\tif Add(2, 3) != 5 {\n\t\tt.Fatal(\"wrong sum\")\n\t}\n}\n",
		},
	}

	res := h.RunSmokeSuite(resp, "go test ./...")
	if res.Error != nil {
		t.Fatalf("expected successful smoke test run, got error: %v\noutput: %s", res.Error, res.Output)
	}
	if !res.BuildSuccess {
		t.Fatalf("expected build success to be true")
	}
	if !res.TestSuccess {
		t.Fatalf("expected test success to be true")
	}
}

func TestRunSmokeSuite_BuildFailure_ReturnsBuildError(t *stdtesting.T) {
	h := emittertesting.NewHarness(t)

	resp := emitterkit.GenerationResponse{
		Files: map[string]string{
			"go.mod":       "module testsmokepkg\n\ngo 1.22\n",
			"calc.go":      "package calc\n\nfunc Add(a, b int) int { return a + undefinedVariable }\n",
			"calc_test.go": "package calc\n\nimport \"testing\"\n\nfunc TestAdd(t *testing.T) {}\n",
		},
	}

	res := h.RunSmokeSuite(resp, "go test ./...")
	if res.Error == nil {
		t.Fatalf("expected smoke test run to fail due to build error")
	}
	if res.BuildSuccess {
		t.Fatalf("expected build success to be false on compiler error")
	}
	if res.TestSuccess {
		t.Fatalf("expected test success to be false on compiler error")
	}
}

func TestRunSmokeSuite_TestFailure_ReturnsTestError(t *stdtesting.T) {
	h := emittertesting.NewHarness(t)

	resp := emitterkit.GenerationResponse{
		Files: map[string]string{
			"go.mod":       "module testsmokepkg\n\ngo 1.22\n",
			"calc.go":      "package calc\n\nfunc Add(a, b int) int { return a + b }\n",
			"calc_test.go": "package calc\n\nimport \"testing\"\n\nfunc TestAdd(t *testing.T) {\n\tt.Fatal(\"simulated test assertion failure\")\n}\n",
		},
	}

	res := h.RunSmokeSuite(resp, "go test ./...")
	if res.Error == nil {
		t.Fatalf("expected smoke test run to fail due to test failure")
	}
	if !res.BuildSuccess {
		t.Fatalf("expected build success to be true when compilation passes")
	}
	if res.TestSuccess {
		t.Fatalf("expected test success to be false when assertion fails")
	}
	if !strings.Contains(res.Output, "simulated test assertion failure") {
		t.Fatalf("expected failure output in result, got: %s", res.Output)
	}
}

func TestRunSmokeSuite_CustomCommandResolution_UsesResponseTestCommand(t *stdtesting.T) {
	h := emittertesting.NewHarness(t)

	resp := emitterkit.GenerationResponse{
		TestCommand: "go test -v ./...",
		Files: map[string]string{
			"go.mod":       "module testsmokepkg\n\ngo 1.22\n",
			"calc.go":      "package calc\n\nfunc Value() int { return 42 }\n",
			"calc_test.go": "package calc\n\nimport \"testing\"\n\nfunc TestValue(t *testing.T) {}\n",
		},
	}

	res := h.RunSmokeSuite(resp, "")
	if res.Error != nil {
		t.Fatalf("expected successful smoke test run: %v", res.Error)
	}
	if !res.BuildSuccess || !res.TestSuccess {
		t.Fatalf("expected both build and test success")
	}
	if !strings.Contains(res.Output, "=== RUN   TestValue") {
		t.Fatalf("expected verbose output from TestCommand, got: %s", res.Output)
	}
}

func TestRunSmokeSuite_ExplicitCommandOverridesResponseTestCommand(t *stdtesting.T) {
	h := emittertesting.NewHarness(t)

	resp := emitterkit.GenerationResponse{
		TestCommand: "invalid-command-should-not-run",
		Files: map[string]string{
			"go.mod":  "module testsmokepkg\n\ngo 1.22\n",
			"calc.go": "package calc\n\nfunc Value() int { return 1 }\n",
		},
	}

	res := h.RunSmokeSuite(resp, "go build ./...")
	if res.Error != nil {
		t.Fatalf("expected explicit command to override resp.TestCommand, got error: %v", res.Error)
	}
	if !res.BuildSuccess {
		t.Fatalf("expected build success")
	}
}

func TestRunSmokeSuite_SafeRelPath_RejectsPathTraversal(t *stdtesting.T) {
	h := emittertesting.NewHarness(t)

	resp := emitterkit.GenerationResponse{
		Files: map[string]string{
			"../../escape.txt": "hazardous content",
		},
	}

	res := h.RunSmokeSuite(resp, "go test ./...")
	if res.Error == nil {
		t.Fatalf("expected path traversal to return error")
	}
	if res.BuildSuccess || res.TestSuccess {
		t.Fatalf("expected false build and test success on path traversal error")
	}
}
