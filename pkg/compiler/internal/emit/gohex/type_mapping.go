package gohex

import (
	"strings"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
)

// isEntity reports whether a declared type names an entity, which carries identity.
func (lay goLayout) isEntity(name string) bool {
	return lay.Entities[name]
}

// identityGoType renders a declared type as a list element: an entity is a pointer because
// it has identity, every other declared type is a value.
func (lay goLayout) identityGoType(name string, inDomainPkg bool) string {
	if lay.isEntity(name) {
		return "*" + lay.goType(name, inDomainPkg)
	}
	return lay.goType(name, inDomainPkg)
}

// fileNeedsTime decides the time import for every emitted file that can reach it: entity
// and value structs, port signatures, adapters, and HTTP request bodies.
func fileNeedsTime(declaredTypes []string) bool {
	for _, t := range declaredTypes {
		if strings.TrimPrefix(t, "[]") == "timestamp" {
			return true
		}
	}
	return false
}

// declaredFieldTypes lists the declared types of a field list.
func declaredFieldTypes(fields []ir.FieldDecl) []string {
	types := make([]string, 0, len(fields))
	for _, f := range fields {
		types = append(types, f.Type)
	}
	return types
}
