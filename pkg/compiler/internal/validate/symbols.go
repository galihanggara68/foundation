package validate

import (
	"fmt"
	"sort"
	"strings"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
)

// Pass represents a single semantic or architectural invariant verification pass
// over the SemanticIR (Composite / Pipeline pattern for SRP & OCP).
type Pass interface {
	Validate(sem *ir.SemanticIR, sym *SymbolTable) []ir.Diagnostic
}

// Validate executes the canonical multi-pass semantic and architectural invariant validation pipeline.
// Pass ordering, symbol resolution, and symbol table lifecycle are fully encapsulated within this entry point.
func Validate(sem *ir.SemanticIR) []ir.Diagnostic {
	sym := NewSymbolTable()
	passes := []Pass{
		DeclarationPass{},
		ActionTypeCheckPass{},
		TerminationPass{},
		InvariantPass{},
		ScenarioTypeCheckPass{},
	}
	var diags []ir.Diagnostic
	for _, pass := range passes {
		diags = append(diags, pass.Validate(sem, sym)...)
	}
	return diags
}

var scalarTypes = map[string]bool{
	"uuid":      true,
	"string":    true,
	"int":       true,
	"decimal":   true,
	"bool":      true,
	"timestamp": true,
}

var validErrorKinds = map[string]bool{
	"not_found":    true,
	"validation":   true,
	"conflict":     true,
	"unauthorized": true,
	"forbidden":    true,
	"internal":     true,
}

// SymbolTable indexes declared domain constructs for fast, DRY lookup across validation passes.
type SymbolTable struct {
	enums       map[string]ir.EnumDecl
	entities    map[string]ir.EntityDecl
	values      map[string]ir.ValueDecl
	errors      map[string]ir.ErrorDecl
	events      map[string]ir.EventDecl
	policies    map[string]ir.PolicyDecl
	slots       map[string]ir.SlotDecl
	actions     map[string]ir.ActionDecl
	actorFields map[string]ir.FieldDecl
}

// NewSymbolTable constructs an empty symbol table with built-in Actor context fields.
func NewSymbolTable() *SymbolTable {
	return &SymbolTable{
		enums:    make(map[string]ir.EnumDecl),
		entities: make(map[string]ir.EntityDecl),
		values:   make(map[string]ir.ValueDecl),
		errors: map[string]ir.ErrorDecl{
			"Conflict": {Name: "Conflict", Kind: "conflict", Message: "optimistic concurrency conflict"},
		},
		events:   make(map[string]ir.EventDecl),
		policies: make(map[string]ir.PolicyDecl),
		slots:    make(map[string]ir.SlotDecl),
		actions:  make(map[string]ir.ActionDecl),
		actorFields: map[string]ir.FieldDecl{
			"id":          {Name: "id", Type: "uuid"},
			"role":        {Name: "role", Type: "string"},
			"is_verified": {Name: "is_verified", Type: "bool"},
			"tenant_id":   {Name: "tenant_id", Type: "uuid"},
		},
	}
}

func (s *SymbolTable) findSlotMethod(slotName, methodName string) (ir.SlotDecl, ir.SlotMethodDecl, bool) {
	sl, ok := s.slots[slotName]
	if !ok {
		return ir.SlotDecl{}, ir.SlotMethodDecl{}, false
	}
	for _, m := range sl.Methods {
		if m.Name == methodName {
			return sl, m, true
		}
	}
	return sl, ir.SlotMethodDecl{}, false
}

func (s *SymbolTable) isKnownType(t string) bool {
	if strings.HasPrefix(t, "[]") {
		return s.isKnownType(strings.TrimPrefix(t, "[]"))
	}
	if scalarTypes[t] || t == "Actor" {
		return true
	}
	if _, ok := s.enums[t]; ok {
		return true
	}
	if _, ok := s.entities[t]; ok {
		return true
	}
	if _, ok := s.values[t]; ok {
		return true
	}
	if _, ok := s.events[t]; ok {
		return true
	}
	return false
}

func (s *SymbolTable) fieldsOf(typeName string) map[string]ir.FieldDecl {
	m := make(map[string]ir.FieldDecl)
	if e, ok := s.entities[typeName]; ok {
		for _, f := range e.Fields {
			m[f.Name] = f
		}
		return m
	}
	if v, ok := s.values[typeName]; ok {
		for _, f := range v.Fields {
			m[f.Name] = f
		}
		return m
	}
	if ev, ok := s.events[typeName]; ok {
		for _, f := range ev.Fields {
			m[f.Name] = f
		}
		return m
	}
	if typeName == "Actor" {
		for k, f := range s.actorFields {
			m[k] = f
		}
	}
	return m
}

func (s *SymbolTable) enumHasVariant(enumName, variant string) bool {
	en, ok := s.enums[enumName]
	if !ok {
		return false
	}
	for _, v := range en.Variants {
		if v == variant {
			return true
		}
	}
	return false
}

func (s *SymbolTable) findEnumByVariant(variant string) (string, bool) {
	var names []string
	for name := range s.enums {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if s.enumHasVariant(name, variant) {
			return name, true
		}
	}
	return "", false
}

func (s *SymbolTable) resolveEnumLiteral(e *ir.Expr, expectedType string) {
	if e != nil && e.Kind == ir.ExprEnumLit && s.enumHasVariant(expectedType, e.Value) {
		e.InferredType = expectedType
	}
}

func isAssignableType(expected, actual string) bool {
	return expected == actual || (expected == "uuid" && actual == "string") || (strings.HasPrefix(expected, "[]") && actual == "[]")
}

func isNumericType(t string) bool {
	return t == "int" || t == "decimal"
}

func cloneScope(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func registerUniqueDecl[T any](m map[string]T, kindLabel, name string, item T, span ir.Span, diags *[]ir.Diagnostic) {
	if _, exists := m[name]; exists {
		*diags = append(*diags, ir.Diagnostic{
			Code:     "FDN0200",
			Severity: ir.SeverityError,
			Message:  fmt.Sprintf("duplicate %s declaration %q", kindLabel, name),
			Span:     span,
		})
	}
	m[name] = item
}
