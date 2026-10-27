package frontend_test

import (
	"strings"
	"testing"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/frontend"
)

// FuzzParseFile provides coverage-guided fuzz testing against the Blueprint parser & lexer.
// The parser must never panic or enter an infinite loop on arbitrary or malicious byte sequences.
func FuzzParseFile(f *testing.F) {
	// Seed 1: Empty file
	f.Add("")

	// Seed 2: Minimal entity
	f.Add("entity User {\n  id: uuid @pk\n  name: string\n}")

	// Seed 3: Action declaration
	f.Add(`
action PlaceOrder(total: decimal) -> Order
`)

	// Seed 4: Pathological unbalanced tokens
	f.Add("entity A { id: uuid @pk field: {{{{")
	f.Add(`"unclosed string literal with \ escape`)
	f.Add("entity // comment without newline")
	f.Add("@unknown_trait(arg: 1234, deep: [1, 2, 3])")

	// Seed 5: Unicode and emoji
	f.Add("entity 🚀 {\n  id: uuid @pk\n  🏷️: string\n}")

	f.Fuzz(func(t *testing.T, input string) {
		// Enforce max reasonable input length to prevent unbounded memory in fuzz runs
		if len(input) > 64*1024 {
			return
		}

		// The test passes if ParseFile executes without panicking
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("PANIC during ParseFile on input %q: %v", input, r)
			}
		}()

		sem, diags := frontend.ParseFile("fuzz.fdn", input)
		_ = sem
		_ = diags
	})
}

// TestParser_DeeplyNestedExpressions verifies that deeply nested parentheses or blocks
// do not cause a stack overflow or fatal panic.
func TestParser_DeeplyNestedExpressions(t *testing.T) {
	depth := 500
	var sb strings.Builder
	sb.WriteString("action Deep() -> int {\n  step x = ")
	for i := 0; i < depth; i++ {
		sb.WriteString("(")
	}
	sb.WriteString("1")
	for i := 0; i < depth; i++ {
		sb.WriteString(" + 1)")
	}
	sb.WriteString("\n  step return ok(x)\n}\n")

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("PANIC on deeply nested expression: %v", r)
		}
	}()

	sem, diags := frontend.ParseFile("deep.fdn", sb.String())
	_ = sem
	t.Logf("Deeply nested expression parsed safely with %d diagnostics", len(diags))
}
