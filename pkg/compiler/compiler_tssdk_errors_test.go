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

// The generated TypeScript SDK must take its error vocabulary from platform: a class for
// every standard kind, a parent case for every kind, and no case naming a class the file
// never declares.
func TestCompiler_TypeScriptSDKErrorsReadThePlatformTable(t *testing.T) {
	kinds := platform.StandardErrorKinds()
	names := platform.ReservedErrorNames()
	if len(names) != len(kinds) {
		t.Fatalf("vocabulary is not one name per kind: %d kinds, %d names", len(kinds), len(names))
	}
	nameForKind := make(map[string]string, len(kinds))
	for i, kind := range kinds {
		nameForKind[string(kind)] = names[i]
	}

	// One domain error per standard kind, so the emitted parent of each kind is observable.
	var blueprint strings.Builder
	blueprint.WriteString(`
enum OrderStatus {
  pending
  confirmed
  cancelled
}

entity Order {
  id: uuid @pk
  status: OrderStatus
  created_at: timestamp
}

`)
	for _, kind := range kinds {
		fmt.Fprintf(&blueprint, "error Custom%s {\n  kind: %s\n  message: %q\n}\n\n",
			nameForKind[string(kind)], kind, "a "+string(kind)+" error")
	}
	blueprint.WriteString(`
action PlaceOrder(item_id: uuid) -> Order
  @http(method: "POST", path: "/v1/orders")
{
  let order = query one Order where id == item_id else CustomNotFound
  return order
}
`)

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "shop.fdn"), []byte(blueprint.String()), 0644); err != nil {
		t.Fatalf("failed to write blueprint: %v", err)
	}

	buildYAML := `
project:
  name: shop
  module: example.com/shop

targets:
  - name: ts-client
    kind: sdk
    language: typescript
    out_dir: clients/ts
`
	if err := os.WriteFile(filepath.Join(dir, "foundation.build.yaml"), []byte(buildYAML), 0644); err != nil {
		t.Fatalf("failed to write build yaml: %v", err)
	}

	if _, diags := compiler.Build(dir, compiler.BuildOptions{}); len(diags) > 0 {
		t.Fatalf("unexpected build diagnostics: %v", diags)
	}

	errorsTSBytes, err := os.ReadFile(filepath.Join(dir, "clients", "ts", "errors.ts"))
	if err != nil {
		t.Fatalf("failed to read emitted errors.ts: %v", err)
	}
	errorsTS := string(errorsTSBytes)

	// (a) A class per standard kind, named from the vocabulary — the built-ins included.
	for i, kind := range kinds {
		class := names[i] + "Error"
		if !strings.Contains(errorsTS, "export class "+class+" extends ") {
			t.Errorf("errors.ts must declare %s for standard kind %q", class, kind)
		}
	}
	for _, builtin := range platform.BuiltinErrors() {
		class := builtin.Name + "Error"
		if !strings.Contains(errorsTS, "export class "+class+" extends ") {
			t.Errorf("errors.ts must declare the built-in class %s", class)
		}
	}

	// (b) Every standard kind has a parent case, and the kind switch enumerates them.
	for _, kind := range kinds {
		name := nameForKind[string(kind)]
		want := fmt.Sprintf("export class Custom%sError extends %sError {}", name, name)
		if !strings.Contains(errorsTS, want) {
			t.Errorf("a domain error of kind %q must extend %sError; errors.ts has no %q", kind, name, want)
		}
		if !strings.Contains(errorsTS, "case \""+string(kind)+"\":") {
			t.Errorf("errors.ts kind switch is missing a case for standard kind %q", kind)
		}
	}

	// (c) No emitted case may name a class the file does not declare.
	declared := map[string]bool{}
	for _, m := range regexp.MustCompile(`export class (\w+)`).FindAllStringSubmatch(errorsTS, -1) {
		declared[m[1]] = true
	}
	for _, re := range []*regexp.Regexp{
		regexp.MustCompile(`extends (\w+)`),
		regexp.MustCompile(`new (\w+)\(`),
	} {
		for _, m := range re.FindAllStringSubmatch(errorsTS, -1) {
			if name := m[1]; name != "Error" && !declared[name] {
				t.Errorf("errors.ts references class %q that it never declares", name)
			}
		}
	}

	// (d) The 401 tie-break resolves to the vocabulary's first kind for that status.
	const tieStatus = 401
	var tied []string
	for _, kind := range kinds {
		if policy, _ := platform.ErrorPolicyFor(kind); policy.Status == tieStatus {
			tied = append(tied, string(kind))
		}
	}
	if len(tied) < 2 {
		t.Fatalf("expected more than one standard kind to answer %d, got %v", tieStatus, tied)
	}
	winner := tied[0]
	if winner != "unauthenticated" {
		t.Errorf("the %d tie-break must resolve to unauthenticated, got %q", tieStatus, winner)
	}
	if want := fmt.Sprintf("case %d: return %q;", tieStatus, winner); !strings.Contains(errorsTS, want) {
		t.Errorf("defaultKindForStatus must resolve %d to %q; errors.ts has no %q", tieStatus, winner, want)
	}
	for _, loser := range tied[1:] {
		if wrong := fmt.Sprintf("case %d: return %q;", tieStatus, loser); strings.Contains(errorsTS, wrong) {
			t.Errorf("defaultKindForStatus must not resolve %d to %q: %q wins the tie-break", tieStatus, loser, winner)
		}
	}
	if !strings.Contains(errorsTS, `default: return "internal";`) {
		t.Errorf("defaultKindForStatus must keep internal as the fallback")
	}
}
