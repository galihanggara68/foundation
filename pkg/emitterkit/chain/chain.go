package chain

import (
	"errors"
	"strings"
	"unicode"
)

var (
	// ErrAnchorNotFound shows that the requested pipeline anchor does not exist.
	ErrAnchorNotFound = errors.New("pipeline anchor not found")

	// ErrDuplicateAnchor shows that multiple matching pipeline anchors exist.
	ErrDuplicateAnchor = errors.New("duplicate pipeline anchor detected")
)

const anchorPrefix = "// foundation:anchor:"

type lineData struct {
	text   string
	indent string
}

// InsertBefore finds the anchor in content and inserts snippet before the anchor comment.
func InsertBefore(content, anchorName, snippet string) (string, error) {
	idx, lines, err := findAnchor(content, anchorName)
	if err != nil {
		return "", err
	}

	formatted := formatSnippet(snippet, lines[idx].indent)
	var sb strings.Builder
	for i, l := range lines {
		if i == idx {
			sb.WriteString(formatted)
		}
		sb.WriteString(l.text)
	}
	return sb.String(), nil
}

// InsertAfter finds the anchor in content and inserts snippet after the anchor comment.
func InsertAfter(content, anchorName, snippet string) (string, error) {
	idx, lines, err := findAnchor(content, anchorName)
	if err != nil {
		return "", err
	}

	formatted := formatSnippet(snippet, lines[idx].indent)
	var sb strings.Builder
	for i, l := range lines {
		sb.WriteString(l.text)
		if i == idx {
			sb.WriteString(formatted)
		}
	}
	return sb.String(), nil
}

// ReplaceAnchor finds the anchor in content and replaces the anchor line with snippet.
func ReplaceAnchor(content, anchorName, snippet string) (string, error) {
	idx, lines, err := findAnchor(content, anchorName)
	if err != nil {
		return "", err
	}

	formatted := formatSnippet(snippet, lines[idx].indent)
	var sb strings.Builder
	for i, l := range lines {
		if i == idx {
			sb.WriteString(formatted)
			continue
		}
		sb.WriteString(l.text)
	}
	return sb.String(), nil
}

func findAnchor(content, anchorName string) (int, []lineData, error) {
	name := cleanAnchorName(anchorName)
	if name == "" {
		return -1, nil, ErrAnchorNotFound
	}

	target := anchorPrefix + name
	lines := splitLines(content)

	matchedIndex := -1
	matchCount := 0

	for i, l := range lines {
		trimmed := strings.TrimSpace(l.text)
		if trimmed == target || strings.HasPrefix(trimmed, target+" ") {
			matchedIndex = i
			matchCount++
		}
	}

	if matchCount == 0 {
		return -1, nil, ErrAnchorNotFound
	}
	if matchCount > 1 {
		return -1, nil, ErrDuplicateAnchor
	}

	return matchedIndex, lines, nil
}

func cleanAnchorName(name string) string {
	n := strings.TrimPrefix(name, "//")
	n = strings.TrimPrefix(n, "foundation:anchor:")
	return strings.TrimSpace(n)
}

func splitLines(content string) []lineData {
	if content == "" {
		return nil
	}

	var lines []lineData
	start := 0
	for i := 0; i < len(content); i++ {
		if content[i] == '\n' {
			lineText := content[start : i+1]
			lines = append(lines, lineData{
				text:   lineText,
				indent: extractIndent(lineText),
			})
			start = i + 1
		}
	}
	if start < len(content) {
		lineText := content[start:]
		lines = append(lines, lineData{
			text:   lineText,
			indent: extractIndent(lineText),
		})
	}
	return lines
}

func extractIndent(s string) string {
	var sb strings.Builder
	for _, r := range s {
		if r == ' ' || r == '\t' {
			sb.WriteRune(r)
			continue
		}
		break
	}
	return sb.String()
}

func formatSnippet(snippet, indent string) string {
	if snippet == "" {
		return ""
	}

	normalized := strings.ReplaceAll(snippet, "\r\n", "\n")
	parts := strings.Split(normalized, "\n")
	if len(parts) > 0 && parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}

	var sb strings.Builder
	for _, p := range parts {
		if strings.TrimSpace(p) == "" {
			sb.WriteString("\n")
			continue
		}
		if hasAnyPrefix(p, indent) {
			sb.WriteString(p)
		} else {
			sb.WriteString(indent)
			sb.WriteString(p)
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

func hasAnyPrefix(s, prefix string) bool {
	if prefix == "" {
		return true
	}
	if strings.HasPrefix(s, prefix) {
		return true
	}
	trimmed := strings.TrimLeftFunc(s, unicode.IsSpace)
	return len(s)-len(trimmed) >= len(prefix)
}
