package lsp

import (
	"regexp"
	"strings"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
)

// memberContextAt reports whether the cursor sits after a member access such as
// `value.` or `value.pre`, returning the receiver identifier and the partial
// member name typed so far.
func memberContextAt(docContent string, pos Position) (receiver, prefix string, ok bool) {
	lines := strings.Split(docContent, "\n")
	if pos.Line < 0 || pos.Line >= len(lines) {
		return "", "", false
	}
	line := lines[pos.Line]
	if pos.Character < 0 || pos.Character > len(line) {
		return "", "", false
	}

	before := line[:pos.Character]

	prefixStart := len(before)
	for prefixStart > 0 && isIdentByte(before[prefixStart-1]) {
		prefixStart--
	}
	prefix = before[prefixStart:]

	dot := prefixStart - 1
	if dot < 0 || before[dot] != '.' {
		return "", "", false
	}

	receiverEnd := dot
	receiverStart := receiverEnd
	for receiverStart > 0 && isIdentByte(before[receiverStart-1]) {
		receiverStart--
	}
	receiver = before[receiverStart:receiverEnd]
	// Require an identifier receiver so numeric literals like `12.5` are ignored.
	if receiver == "" || !isIdentStart(receiver[0]) {
		return "", "", false
	}
	return receiver, prefix, true
}

func isIdentStart(b byte) bool {
	return b == '_' || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

var (
	letTypeRe   = regexp.MustCompile(`\blet\s+([A-Za-z_]\w*)\s*=\s*(?:query\s+(?:one|many|single)\s+|create\s+)?([A-Za-z_]\w*)`)
	paramTypeRe = regexp.MustCompile(`\b([A-Za-z_]\w*)\s*:\s*([A-Za-z_]\w*)`)
)

// fallbackLocalType recovers a binding's type from the source text. The parser
// discards a whole declaration when one of its statements is malformed, which is
// the normal state while typing (e.g. `sender.`), so member completion falls back
// to scanning the text when the symbol index carries no type for the receiver.
func fallbackLocalType(content, receiver string, line int) string {
	lines := strings.Split(content, "\n")
	limit := line
	if limit > len(lines) {
		limit = len(lines)
	}

	// The closest preceding `let` wins.
	for i := 0; i < limit; i++ {
		for _, m := range letTypeRe.FindAllStringSubmatch(lines[i], -1) {
			if m[1] == receiver {
				return m[2]
			}
		}
	}
	for i := 0; i < limit; i++ {
		for _, m := range paramTypeRe.FindAllStringSubmatch(lines[i], -1) {
			if m[1] == receiver {
				return m[2]
			}
		}
	}
	return ""
}

// LookupFields returns the fields of the named struct type, preferring the
// document the request came from.
func LookupFields(indexes []*DocumentIndex, currentURI, typeName string) []ir.FieldDecl {
	if typeName == "" {
		return nil
	}
	for _, idx := range indexes {
		if idx.URI != currentURI {
			continue
		}
		if fields := idx.Structs[typeName]; len(fields) > 0 {
			return fields
		}
	}
	for _, idx := range indexes {
		if fields := idx.Structs[typeName]; len(fields) > 0 {
			return fields
		}
	}
	return nil
}

// ComputeCompletionAt serves member completions for `<value>.` and otherwise
// falls back to the keyword/type/trait catalog.
func ComputeCompletionAt(indexes []*DocumentIndex, currentURI, docContent string, pos Position) *CompletionList {
	if receiver, prefix, ok := memberContextAt(docContent, pos); ok {
		return &CompletionList{Items: memberCompletion(indexes, currentURI, docContent, receiver, prefix, pos.Line+1)}
	}
	return ComputeCompletion(docContent, pos)
}

func memberCompletion(indexes []*DocumentIndex, currentURI, docContent, receiver, prefix string, line int) []CompletionItem {
	typeName := ""
	if sym := bestLocal(indexes, currentURI, receiver, line, true); sym != nil {
		typeName = sym.Type
	} else if sym := bestLocal(indexes, currentURI, receiver, line, false); sym != nil {
		typeName = sym.Type
	}
	if typeName == "" {
		typeName = fallbackLocalType(docContent, receiver, line)
	}
	if typeName == "" {
		return nil
	}

	fields := LookupFields(indexes, currentURI, strings.TrimPrefix(typeName, "[]"))
	if len(fields) == 0 {
		return nil
	}

	items := make([]CompletionItem, 0, len(fields))
	for _, f := range fields {
		if prefix != "" && !strings.HasPrefix(f.Name, prefix) {
			continue
		}
		items = append(items, CompletionItem{
			Label:  f.Name,
			Kind:   CompletionKindField,
			Detail: fieldType(f),
		})
	}
	return items
}
