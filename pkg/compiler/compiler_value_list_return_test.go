package compiler_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/galihanggara68/foundation/pkg/compiler"
)

func TestBuild_ValueListReturn_ResultFieldAndLiteralAgreeOnPointerness(t *testing.T) {
	dir := t.TempDir()

	bp := `value Money {
  amount: decimal
  currency: string
}

action ListSummaries(id: uuid) -> []Money {
  return [Money { amount: 1.0, currency: "USD" }]
}
`
	if err := os.WriteFile(filepath.Join(dir, "catalog.fdn"), []byte(bp), 0644); err != nil {
		t.Fatalf("write blueprint: %v", err)
	}

	buildProfile := `project:
  name: catalog
  module: example.com/catalog

storage:
  dialect: memory
  mode: in_memory

targets:
  - name: backend
    kind: backend
    language: go
    architecture: hexagonal
    http: chi
    out_dir: gen
`
	if err := os.WriteFile(filepath.Join(dir, "foundation.build.yaml"), []byte(buildProfile), 0644); err != nil {
		t.Fatalf("write foundation.build.yaml: %v", err)
	}

	_, diags := compiler.Build(dir, compiler.BuildOptions{})
	if compiler.HasErrors(diags) {
		t.Fatalf("compiler.Build failed: %+v", diags)
	}

	valuesGo := readEmittedFile(t, filepath.Join(dir, "gen", "internal", "domain", "values.go"))
	fieldMatch := regexp.MustCompile(`Items\s+(\[\]\*?Money)\s+`).FindStringSubmatch(valuesGo)
	if fieldMatch == nil {
		t.Fatalf("expected a ListSummariesResult Items field of Money, got:\n%s", valuesGo)
	}
	fieldType := fieldMatch[1]

	usecaseFiles, err := filepath.Glob(filepath.Join(dir, "gen", "internal", "usecases", "*.go"))
	if err != nil || len(usecaseFiles) == 0 {
		t.Fatalf("expected an emitted usecase file: %v", err)
	}
	var litType string
	for _, f := range usecaseFiles {
		if m := regexp.MustCompile(`Items:\s*(\[\]\*?domain\.Money)\{`).FindStringSubmatch(readEmittedFile(t, f)); m != nil {
			litType = m[1]
			break
		}
	}
	if litType == "" {
		t.Fatalf("expected an emitted result literal of []domain.Money, got no match in usecases")
	}

	if strings.HasPrefix(fieldType, "[]*") != strings.HasPrefix(litType, "[]*") {
		t.Errorf("result field and literal disagree on pointer-ness: field %q, literal %q", fieldType, litType)
	}
	if strings.HasPrefix(fieldType, "[]*") {
		t.Errorf("a value list has no identity and must be a value slice, got field %q", fieldType)
	}

	if _, err := exec.LookPath("go"); err != nil {
		t.Logf("skipping emitted-module compile: go toolchain unavailable: %v", err)
		return
	}
	cmd := exec.Command("go", "build", "./...")
	cmd.Dir = filepath.Join(dir, "gen")
	cmd.Env = append(os.Environ(), "GOPROXY=off", "GOWORK=off")
	out, err := cmd.CombinedOutput()
	if err != nil {
		if isGoModuleResolutionError(string(out)) {
			t.Logf("skipping emitted-module compile: dependencies not resolvable offline:\n%s", out)
			return
		}
		t.Fatalf("emitted module failed to compile: %v\n%s", err, out)
	}
}

func isGoModuleResolutionError(out string) bool {
	for _, needle := range []string{
		"module lookup disabled",
		"cannot find module",
		"missing go.sum entry",
		"dial tcp",
		"no such host",
	} {
		if strings.Contains(out, needle) {
			return true
		}
	}
	return false
}
