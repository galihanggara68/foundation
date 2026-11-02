package compiler_test

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/galihanggara68/foundation/pkg/compiler"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/platform"
)

// Every standard kind has to reach every emitter, carrying the status and posture the
// vocabulary resolves for it. The kinds are enumerated from the table rather than listed
// here, so a kind added to platform cannot be silently missed by three emitters out of
// four — the failure mode that had gosdk and tssdk map `internal` to a type they never
// declared while tsclean answered from a switch of its own.
func TestBuild_EveryStandardKindReachesEveryEmitter(t *testing.T) {
	kinds := platform.StandardErrorKinds()
	canonical := canonicalErrorNames(t)

	var blueprint strings.Builder
	blueprint.WriteString("entity Probe {\n  id: uuid @pk\n  label: string\n}\n\n")
	for _, kind := range kinds {
		fmt.Fprintf(&blueprint, "error Probe%s {\n  kind: %s\n  message: \"probe\"\n}\n\n", canonical[kind], kind)
	}
	fmt.Fprintf(&blueprint, "action GetProbe(id: uuid) -> Probe {\n  let probe = query one Probe where id == id else Probe%s\n  return probe\n}\n", canonical[kinds[0]])

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "probe.fdn"), []byte(blueprint.String()), 0644); err != nil {
		t.Fatalf("write blueprint: %v", err)
	}

	profile := `project:
  name: probe
  module: example.com/probe

storage:
  dialect: memory
  mode: in_memory

targets:
  - name: backend
    kind: backend
    language: go
    architecture: hexagonal
    http: chi
    out_dir: gen/go
  - name: ts-backend
    kind: backend
    language: typescript
    architecture: clean
    http: express
    out_dir: gen/ts
  - name: go-client
    kind: sdk
    language: go
    out_dir: clients/go
  - name: ts-client
    kind: sdk
    language: typescript
    out_dir: clients/ts
`
	if err := os.WriteFile(filepath.Join(dir, "foundation.build.yaml"), []byte(profile), 0644); err != nil {
		t.Fatalf("write build profile: %v", err)
	}

	res, diags := compiler.Build(dir, compiler.BuildOptions{})
	if compiler.HasErrors(diags) {
		t.Fatalf("compiler.Build failed: %+v", diags)
	}
	if len(res.Architectures) != 4 {
		t.Fatalf("expected 4 architectures, got %d", len(res.Architectures))
	}

	emitted := map[string]string{
		"gohex":   readEmittedFile(t, filepath.Join(dir, "gen", "go", "internal", "domain", "errors.go")),
		"tsclean": readEmittedFile(t, filepath.Join(dir, "gen", "ts", "src", "entities", "domain.ts")),
		"gosdk":   readEmittedFile(t, filepath.Join(dir, "clients", "go", "errors.go")),
		"tssdk":   readEmittedFile(t, filepath.Join(dir, "clients", "ts", "errors.ts")),
	}

	for _, kind := range kinds {
		policy, standard := platform.ErrorPolicyFor(kind)
		if !standard {
			t.Errorf("kind %q is missing from the standard vocabulary", kind)
			continue
		}
		name := canonical[kind]

		goBlock := goErrorBlock(emitted["gohex"], "var ErrProbe"+name+" = ")
		if goBlock == "" {
			t.Errorf("gohex emitted no ErrProbe%s for kind %q", name, kind)
		} else if want := regexp.MustCompile(fmt.Sprintf(`Status:\s+%d,`, policy.Status)); !want.MatchString(goBlock) {
			t.Errorf("gohex ErrProbe%s: missing %q in\n%s", name, want, goBlock)
		} else if posture := goPostureConst(policy.Posture); !strings.Contains(goBlock, posture) {
			t.Errorf("gohex ErrProbe%s: missing %q in\n%s", name, posture, goBlock)
		}

		tsLine := lineContaining(emitted["tsclean"], "ErrProbe"+name)
		if tsLine == "" {
			t.Errorf("tsclean emitted no ErrProbe%s for kind %q", name, kind)
		} else if want := fmt.Sprintf("%d, %q)", policy.Status, string(policy.Posture)); !strings.Contains(tsLine, want) {
			t.Errorf("tsclean ErrProbe%s: missing %q in\n%s", name, want, tsLine)
		}

		if want := "type " + name + "Error struct"; !strings.Contains(emitted["gosdk"], want) {
			t.Errorf("gosdk does not declare %q for kind %q", want, kind)
		}
		if want := fmt.Sprintf("case %q:", string(kind)); !strings.Contains(emitted["gosdk"], want) {
			t.Errorf("gosdk kind switch has no %q", want)
		}
		if want := "export class " + name + "Error extends"; !strings.Contains(emitted["tssdk"], want) {
			t.Errorf("tssdk does not declare %q for kind %q", want, kind)
		}
		if want := fmt.Sprintf("case %q:", string(kind)); !strings.Contains(emitted["tssdk"], want) {
			t.Errorf("tssdk kind switch has no %q", want)
		}
	}
}

// canonicalErrorNames maps each standard kind to the name the vocabulary reserves for it.
func canonicalErrorNames(t *testing.T) map[ir.ErrorKind]string {
	t.Helper()

	names := make(map[ir.ErrorKind]string)
	for _, name := range platform.ReservedErrorNames() {
		kind, ok := platform.ReservedErrorName(name)
		if !ok {
			t.Fatalf("reserved name %q resolves to no kind", name)
		}
		names[kind] = name
	}
	for _, kind := range platform.StandardErrorKinds() {
		if _, ok := names[kind]; !ok {
			t.Fatalf("kind %q has no reserved name", kind)
		}
	}
	return names
}

// goErrorBlock returns the emitted `var Err... = &DomainError{...}` block beginning at the
// header, so an assertion cannot match a status that belongs to a different error.
func goErrorBlock(src, header string) string {
	start := strings.Index(src, header)
	if start < 0 {
		return ""
	}
	rest := src[start:]
	if end := strings.Index(rest, "\n}"); end >= 0 {
		return rest[:end]
	}
	return rest
}

func lineContaining(src, needle string) string {
	for _, line := range strings.Split(src, "\n") {
		if strings.Contains(line, needle) {
			return line
		}
	}
	return ""
}

func goPostureConst(posture ir.ErrorPosture) string {
	if posture == ir.PostureBusiness {
		return "PostureBusiness"
	}
	return "PostureTechnical"
}
