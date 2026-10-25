package compiler_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/galihanggara68/foundation/pkg/compiler"
)

func TestBuild_GuardConjunction_EmitsBooleanAnd(t *testing.T) {
	dir := t.TempDir()

	bp := `error InvalidRange {
  kind: validation
}

action ValidateThreshold(val: int) -> bool {
  guard val > 0 and val <= 100 else InvalidRange
  return true
}
`
	if err := os.WriteFile(filepath.Join(dir, "guard.fdn"), []byte(bp), 0644); err != nil {
		t.Fatalf("write blueprint: %v", err)
	}

	buildProfile := `project:
  name: test-guard
  module: example.com/guard

targets:
  - name: backend
    kind: backend
    language: go
    architecture: hexagonal
    out_dir: gen
`
	if err := os.WriteFile(filepath.Join(dir, "foundation.build.yaml"), []byte(buildProfile), 0644); err != nil {
		t.Fatalf("write foundation.build.yaml: %v", err)
	}

	_, buildDiags := compiler.Build(dir, compiler.BuildOptions{})
	if len(buildDiags) > 0 {
		t.Fatalf("compiler.Build failed: %+v", buildDiags)
	}

	ucPath := filepath.Join(dir, "gen", "internal", "usecases", "validatethreshold_usecase.go")
	ucBytes, err := os.ReadFile(ucPath)
	if err != nil {
		t.Fatalf("read usecase: %v", err)
	}
	ucSrc := string(ucBytes)
	if strings.Contains(ucSrc, " and ") {
		t.Errorf("usecase contains raw 'and' keyword in Go code:\n%s", ucSrc)
	}
	if !strings.Contains(ucSrc, "&&") {
		t.Errorf("usecase missing '&&' boolean operator:\n%s", ucSrc)
	}
}

func TestBuild_SingleFieldRange_DisambiguatesParamNames(t *testing.T) {
	dir := t.TempDir()

	bp := `error NotFound {
  kind: not_found
}

entity AuditLog {
  id: uuid @pk
  timestamp: timestamp
  event: string
}

action FindRange(from: timestamp, to: timestamp) -> AuditLog {
  let log = query one AuditLog where timestamp >= from and timestamp <= to else NotFound
  return log
}
`
	if err := os.WriteFile(filepath.Join(dir, "range.fdn"), []byte(bp), 0644); err != nil {
		t.Fatalf("write blueprint: %v", err)
	}

	buildProfile := `project:
  name: test-range
  module: example.com/range

targets:
  - name: backend
    kind: backend
    language: go
    architecture: hexagonal
    out_dir: gen
`
	if err := os.WriteFile(filepath.Join(dir, "foundation.build.yaml"), []byte(buildProfile), 0644); err != nil {
		t.Fatalf("write foundation.build.yaml: %v", err)
	}

	_, buildDiags := compiler.Build(dir, compiler.BuildOptions{})
	if len(buildDiags) > 0 {
		t.Fatalf("compiler.Build failed: %+v", buildDiags)
	}

	portPath := filepath.Join(dir, "gen", "internal", "ports", "auditlog_repository.go")
	portBytes, err := os.ReadFile(portPath)
	if err != nil {
		t.Fatalf("read port: %v", err)
	}
	portSrc := string(portBytes)
	// Must have disambiguated parameter names (timestamp_gte and timestamp_lte)
	if !strings.Contains(portSrc, "timestamp_gte time.Time, timestamp_lte time.Time") {
		t.Errorf("expected disambiguated parameters in port, got:\n%s", portSrc)
	}
}
