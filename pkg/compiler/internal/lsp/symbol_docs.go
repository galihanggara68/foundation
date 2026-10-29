package lsp

import (
	"fmt"
	"strings"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
)

func newSymbol(uri, content, file string, kind SymbolKind, name string, span ir.Span, doc string) Symbol {
	line, col := namePosition(content, span, name)
	return Symbol{
		Name:   name,
		Kind:   kind,
		Doc:    doc,
		URI:    uri,
		Line:   line,
		Column: col,
		Length: len(name),
	}
}

// namePosition locates a declaration's name on its start line. Parser spans point
// at the leading keyword for top-level declarations, and at the name itself for
// fields and parameters, so the name is searched from the span onwards.
func namePosition(content string, span ir.Span, name string) (line, col int) {
	if span.Line <= 0 || name == "" {
		return span.Line, span.Column
	}
	text := lineTextOf(content, span.Line)
	if i := findIdentifier(text, name, span.Column-1); i >= 0 {
		return span.Line, i + 1
	}
	return span.Line, span.Column
}

// findIdentifier returns the byte index of name in text at or after from, or -1.
func findIdentifier(text, name string, from int) int {
	if from < 0 {
		from = 0
	}
	for i := from; i+len(name) <= len(text); i++ {
		if text[i:i+len(name)] != name {
			continue
		}
		if i > 0 && isIdentByte(text[i-1]) {
			continue
		}
		if i+len(name) < len(text) && isIdentByte(text[i+len(name)]) {
			continue
		}
		return i
	}
	return -1
}

func isIdentByte(b byte) bool {
	return b == '_' || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}

func lineTextOf(content string, line int) string {
	if line <= 0 {
		return ""
	}
	lines := strings.Split(content, "\n")
	if line > len(lines) {
		return ""
	}
	return lines[line-1]
}

func docFor(kind SymbolKind, name string, extra []string, file string, line int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "### %s `%s`\n", kind, name)
	if len(extra) > 0 {
		b.WriteString("\n")
		b.WriteString(strings.Join(extra, "\n"))
		b.WriteString("\n")
	}
	if file != "" && line > 0 {
		fmt.Fprintf(&b, "\n_Declared in %s:%d_\n", file, line)
	}
	return b.String()
}

func fieldType(f ir.FieldDecl) string {
	t := f.Type
	if f.TypeParams != "" {
		t += "(" + f.TypeParams + ")"
	}
	if f.Optional {
		t += "?"
	}
	return t
}

func fieldLines(fields []ir.FieldDecl) []string {
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		line := fmt.Sprintf("- `%s`: `%s`", f.Name, fieldType(f))
		if len(f.Traits) > 0 {
			line += " " + traitNames(f.Traits)
		}
		out = append(out, line)
	}
	return out
}

func traitNames(traits []ir.TraitDecl) string {
	names := make([]string, 0, len(traits))
	for _, t := range traits {
		names = append(names, "`@"+t.Name+"`")
	}
	return strings.Join(names, " ")
}

func paramList(params []ir.ParamDecl) string {
	parts := make([]string, 0, len(params))
	for _, p := range params {
		parts = append(parts, p.Name+": "+p.Type)
	}
	return strings.Join(parts, ", ")
}

func slotMethodSig(m ir.SlotMethodDecl) string {
	sig := fmt.Sprintf("%s(%s)", m.Name, paramList(m.Params))
	if m.ReturnType != "" {
		sig += " -> " + m.ReturnType
	}
	if len(m.Throws) > 0 {
		sig += " throws " + strings.Join(m.Throws, ", ")
	}
	return sig
}
