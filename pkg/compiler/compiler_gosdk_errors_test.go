package compiler_test

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/galihanggara68/foundation/pkg/compiler"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/platform"
)

// TestCompiler_GoSDKErrorsComeFromPlatformTable guards that the emitted Go SDK takes
// its error vocabulary from the platform table rather than from literals: every standard
// kind gets a declared type, the kind switch covers every kind, and every parent type an
// emitted case references is actually declared.
func TestCompiler_GoSDKErrorsComeFromPlatformTable(t *testing.T) {
	dir := buildGoSDKErrorFixture(t)

	errorsGo := readGoSDKFile(t, filepath.Join(dir, "clients", "go", "errors.go"))

	// derive the Go type emitted for each standard kind from the platform table.
	canonicalType := map[string]string{}
	for _, kind := range platform.StandardErrorKinds() {
		canonicalType[string(kind)] = ""
	}
	for _, name := range platform.ReservedErrorNames() {
		kind, ok := platform.ReservedErrorName(name)
		if !ok {
			t.Fatalf("reserved name %q has no kind", name)
		}
		canonicalType[string(kind)] = name + "Error"
	}

	t.Run("every standard kind has a declared type", func(t *testing.T) {
		for _, kind := range platform.StandardErrorKinds() {
			name := canonicalType[string(kind)]
			if name == "" {
				t.Fatalf("kind %q has no canonical name", kind)
			}
			if !strings.Contains(errorsGo, "type "+name+" struct") {
				t.Errorf("errors.go does not declare type %s for kind %q", name, kind)
			}
		}
	})

	t.Run("every built-in error is declared", func(t *testing.T) {
		for _, builtin := range platform.BuiltinErrors() {
			name := builtin.Name + "Error"
			if !strings.Contains(errorsGo, "type "+name+" struct") {
				t.Errorf("errors.go does not declare built-in type %s", name)
			}
		}
	})

	t.Run("every standard kind has a kind switch case", func(t *testing.T) {
		kindSwitch := extractGoSDKBlock(t, errorsGo, "switch raw.Kind {")
		for _, kind := range platform.StandardErrorKinds() {
			if !strings.Contains(kindSwitch, fmt.Sprintf("case %q:", kind)) {
				t.Errorf("emitted kind switch has no case for %q", kind)
			}
		}
	})

	t.Run("domain errors embed the parent for their kind", func(t *testing.T) {
		for _, tc := range []struct{ name, kind string }{
			{"TicketNotFound", "not_found"},
			{"InvalidTicket", "validation"},
			{"TicketConflict", "conflict"},
			{"SessionMissing", "unauthenticated"},
			{"NotAllowed", "unauthorized"},
			{"AccessBlocked", "forbidden"},
			{"BackendDown", "internal"},
		} {
			parent := canonicalType[tc.kind]
			want := fmt.Sprintf("type %sError struct{\n\t%s\n}", tc.name, parent)
			if !strings.Contains(errorsGo, want) {
				t.Errorf("domain error %s (kind %q) should embed %s, want:\n%s", tc.name, tc.kind, parent, want)
			}
		}
	})

	t.Run("every referenced parent type is declared", func(t *testing.T) {
		declared := map[string]bool{}
		for _, m := range regexp.MustCompile(`(?m)^type\s+(\w+)`).FindAllStringSubmatch(errorsGo, -1) {
			declared[m[1]] = true
		}

		assert := func(where, ref string) {
			if !declared[ref] {
				t.Errorf("%s references undeclared type %q", where, ref)
			}
		}

		embed := regexp.MustCompile(`type\s+\w+Error\s+struct\s*\{\s*(\w+)\s*\}`)
		for _, m := range embed.FindAllStringSubmatch(errorsGo, -1) {
			assert("domain error struct", m[1])
		}

		literal := regexp.MustCompile(`&(\w+Error)\{(\w+): \w+\{APIError: base\}\}`)
		for _, m := range literal.FindAllStringSubmatch(errorsGo, -1) {
			assert("code switch returned type", m[1])
			assert("code switch parent", m[2])
		}

		unwrap := regexp.MustCompile(`&e\.(\w+)`)
		for _, m := range unwrap.FindAllStringSubmatch(errorsGo, -1) {
			assert("Unwrap", m[1])
		}
	})

	t.Run("401 tie-break resolves to unauthenticated", func(t *testing.T) {
		statusSwitch := extractGoSDKBlock(t, errorsGo, "switch statusCode {")
		if !regexp.MustCompile(`case 401:\s*\n\s*raw\.Kind = "unauthenticated"`).MatchString(statusSwitch) {
			t.Errorf("expected 401 to fall back to unauthenticated, got:\n%s", statusSwitch)
		}
		if !strings.Contains(statusSwitch, "tie-break") {
			t.Errorf("expected the 401 fallback to carry a tie-break comment, got:\n%s", statusSwitch)
		}
	})
}

func buildGoSDKErrorFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	blueprint := `
entity Ticket {
  id: uuid @pk
  title: string
}

error TicketNotFound {
  kind: not_found
  message: "ticket not found"
}

error InvalidTicket {
  kind: validation
  message: "ticket is invalid"
}

error TicketConflict {
  kind: conflict
  message: "ticket was modified"
}

error SessionMissing {
  kind: unauthenticated
  message: "no session"
}

error NotAllowed {
  kind: unauthorized
  message: "not allowed"
}

error AccessBlocked {
  kind: forbidden
  message: "access blocked"
}

error BackendDown {
  kind: internal
  message: "backend unavailable"
}

action GetTicket(id: uuid) -> Ticket
  @http(method: "GET", path: "/v1/tickets")
{
  let ticket = query one Ticket where id == id else TicketNotFound
  return ticket
}
`
	if err := os.WriteFile(filepath.Join(dir, "tickets.fdn"), []byte(blueprint), 0644); err != nil {
		t.Fatalf("write blueprint: %v", err)
	}

	buildYAML := `
project:
  name: tickets
  module: example.com/tickets

targets:
  - name: go-client
    kind: sdk
    language: go
    out_dir: clients/go
`
	if err := os.WriteFile(filepath.Join(dir, "foundation.build.yaml"), []byte(buildYAML), 0644); err != nil {
		t.Fatalf("write foundation.build.yaml: %v", err)
	}

	if _, diags := compiler.Build(dir, compiler.BuildOptions{}); len(diags) > 0 {
		t.Fatalf("build diagnostics: %v", diags)
	}

	return dir
}

func readGoSDKFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(content)
}

// extractGoSDKBlock returns src from header up to and including its closing brace.
func extractGoSDKBlock(t *testing.T, src, header string) string {
	t.Helper()
	start := strings.Index(src, header)
	if start < 0 {
		t.Fatalf("emitted file has no %q", header)
	}
	open := strings.Index(src[start:], "{")
	if open < 0 {
		t.Fatalf("%q has no opening brace", header)
	}
	open += start

	depth := 0
	for i := open; i < len(src); i++ {
		switch src[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return src[start : i+1]
			}
		}
	}
	t.Fatalf("%q has no closing brace", header)
	return ""
}
