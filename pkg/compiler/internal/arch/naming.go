package arch

import (
	"strings"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
)

// Pluralize returns the deterministic English pluralization of a singular snake_case noun.
func Pluralize(noun string) string {
	if noun == "" {
		return ""
	}
	switch noun {
	case "person":
		return "people"
	case "child":
		return "children"
	case "ox":
		return "oxen"
	}

	if strings.HasSuffix(noun, "x") || strings.HasSuffix(noun, "z") ||
		strings.HasSuffix(noun, "ch") || strings.HasSuffix(noun, "sh") {
		return noun + "es"
	}

	if strings.HasSuffix(noun, "s") {
		if strings.HasSuffix(noun, "ss") || strings.HasSuffix(noun, "us") || strings.HasSuffix(noun, "is") ||
			strings.HasSuffix(noun, "as") || strings.HasSuffix(noun, "os") || noun == "bus" || noun == "gas" || noun == "lens" {
			return noun + "es"
		}
		return noun
	}

	if strings.HasSuffix(noun, "y") && len(noun) > 1 {
		prev := noun[len(noun)-2]
		if !isVowel(prev) {
			return noun[:len(noun)-1] + "ies"
		}
		return noun + "s"
	}

	if strings.HasSuffix(noun, "fe") && len(noun) > 2 {
		return noun[:len(noun)-2] + "ves"
	}
	if strings.HasSuffix(noun, "f") && len(noun) > 1 {
		prev := noun[len(noun)-2]
		if prev != 'f' {
			return noun[:len(noun)-1] + "ves"
		}
	}

	return noun + "s"
}

func isVowel(c byte) bool {
	return c == 'a' || c == 'e' || c == 'i' || c == 'o' || c == 'u'
}

// EntityTableName returns the physical table name for an entity,
// using explicit @table("name") if present, or defaulting to the deterministic pluralized snake_case name.
func EntityTableName(ent ir.EntityDecl) string {
	for _, tr := range ent.Traits {
		if tr.Name == "table" {
			if custom, ok := tr.Args["_"]; ok && custom != "" {
				return strings.Trim(custom, `"'`)
			}
			if custom, ok := tr.Args["name"]; ok && custom != "" {
				return strings.Trim(custom, `"'`)
			}
		}
	}
	return Pluralize(ToSnakeCase(ent.Name))
}
