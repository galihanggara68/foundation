package ir

import "sort"

// SemanticIR represents the validated, architecture-agnostic domain representation.
type SemanticIR struct {
	Enums     []EnumDecl     `json:"enums,omitempty"`
	Values    []ValueDecl    `json:"values,omitempty"`
	Entities  []EntityDecl   `json:"entities,omitempty"`
	Errors    []ErrorDecl    `json:"errors,omitempty"`
	Events    []EventDecl    `json:"events,omitempty"`
	Policies  []PolicyDecl   `json:"policies,omitempty"`
	Slots     []SlotDecl     `json:"slots,omitempty"`
	Actions   []ActionDecl   `json:"actions,omitempty"`
	Scenarios []ScenarioDecl `json:"scenarios,omitempty"`
}

// Canonicalize sorts all top-level slices deterministically by name.
func (s *SemanticIR) Canonicalize() {
	sort.SliceStable(s.Enums, func(i, j int) bool { return s.Enums[i].Name < s.Enums[j].Name })
	sort.SliceStable(s.Values, func(i, j int) bool { return s.Values[i].Name < s.Values[j].Name })
	sort.SliceStable(s.Entities, func(i, j int) bool { return s.Entities[i].Name < s.Entities[j].Name })
	sort.SliceStable(s.Errors, func(i, j int) bool { return s.Errors[i].Name < s.Errors[j].Name })
	sort.SliceStable(s.Events, func(i, j int) bool { return s.Events[i].Name < s.Events[j].Name })
	sort.SliceStable(s.Policies, func(i, j int) bool { return s.Policies[i].Name < s.Policies[j].Name })
	sort.SliceStable(s.Slots, func(i, j int) bool { return s.Slots[i].Name < s.Slots[j].Name })
	sort.SliceStable(s.Actions, func(i, j int) bool { return s.Actions[i].Name < s.Actions[j].Name })
	sort.SliceStable(s.Scenarios, func(i, j int) bool { return s.Scenarios[i].Name < s.Scenarios[j].Name })
}

type SlotMethodDecl struct {
	Name       string      `json:"name"`
	Params     []ParamDecl `json:"params"`
	ReturnType string      `json:"return_type,omitempty"`
	Throws     []string    `json:"throws,omitempty"`
	Traits     []TraitDecl `json:"traits,omitempty"`
	Span       Span        `json:"span"`
}

type SlotDecl struct {
	Name    string           `json:"name"`
	Methods []SlotMethodDecl `json:"methods"`
	Span    Span             `json:"span"`
}

type GivenSlotDecl struct {
	SlotName    string `json:"slot_name"`
	MethodName  string `json:"method_name"`
	Args        []Expr `json:"args,omitempty"`
	ReturnExpr  *Expr  `json:"return_expr,omitempty"`
	ThrowsError string `json:"throws_error,omitempty"`
	Span        Span   `json:"span"`
}

type EnumDecl struct {
	Name     string   `json:"name"`
	Variants []string `json:"variants"`
	Span     Span     `json:"span"`
}

type EventDecl struct {
	Name   string      `json:"name"`
	Fields []FieldDecl `json:"fields"`
	Span   Span        `json:"span"`
}

type PolicyDecl struct {
	Name   string      `json:"name"`
	Params []ParamDecl `json:"params"`
	Body   Expr        `json:"body"`
	Span   Span        `json:"span"`
}

type FieldDecl struct {
	Name       string      `json:"name"`
	Type       string      `json:"type"`
	TypeParams string      `json:"type_params,omitempty"`
	Optional   bool        `json:"optional,omitempty"`
	DefaultVal string      `json:"default_val,omitempty"`
	Traits     []TraitDecl `json:"traits,omitempty"`
	Span       Span        `json:"span"`
}

// HasTrait reports whether the field has a trait with the given name.
func (f FieldDecl) HasTrait(name string) bool {
	for _, t := range f.Traits {
		if t.Name == name {
			return true
		}
	}
	return false
}

type TraitDecl struct {
	Name string            `json:"name"`
	Args map[string]string `json:"args,omitempty"`
	Span Span              `json:"span"`
}

type ValueDecl struct {
	Name   string      `json:"name"`
	Fields []FieldDecl `json:"fields"`
	Span   Span        `json:"span"`
}

type EntityDecl struct {
	Name   string      `json:"name"`
	Fields []FieldDecl `json:"fields"`
	Traits []TraitDecl `json:"traits,omitempty"`
	Span   Span        `json:"span"`
}

type ErrorDecl struct {
	Name    string `json:"name"`
	Kind    string `json:"kind"`
	Message string `json:"message"`
	Span    Span   `json:"span"`
}

type ParamDecl struct {
	Name string `json:"name"`
	Type string `json:"type"`
	Span Span   `json:"span"`
}

type ActionDecl struct {
	Name       string      `json:"name"`
	Params     []ParamDecl `json:"params"`
	ReturnType string      `json:"return_type"`
	Traits     []TraitDecl `json:"traits,omitempty"`
	Steps      []StepDecl  `json:"steps"`
	Span       Span        `json:"span"`
}

// HasTrait reports whether the action has a trait with the given name.
func (a ActionDecl) HasTrait(name string) bool {
	for _, t := range a.Traits {
		if t.Name == name {
			return true
		}
	}
	return false
}

type GivenSeedDecl struct {
	EntityName string          `json:"entity_name"`
	Fields     []FieldInitDecl `json:"fields"`
	Span       Span            `json:"span"`
}

type ExpectDecl struct {
	Left          Expr            `json:"left"`
	Op            string          `json:"op"`
	Right         Expr            `json:"right"`
	ErrorRaised   string          `json:"error_raised,omitempty"`
	EmittedEvent  string          `json:"emitted_event,omitempty"`
	EmittedFields []FieldInitDecl `json:"emitted_fields,omitempty"`
	StoredEntity  string          `json:"stored_entity,omitempty"`
	WhereField    string          `json:"where_field,omitempty"`
	WhereValue    *Expr           `json:"where_value,omitempty"`
	StoredFields  []FieldInitDecl `json:"stored_fields,omitempty"`
	NotStored     bool            `json:"not_stored,omitempty"`
	Span          Span            `json:"span"`
}

type ScenarioDecl struct {
	Name       string          `json:"name"`
	Givens     []GivenSeedDecl `json:"givens,omitempty"`
	GivenSlots []GivenSlotDecl `json:"given_slots,omitempty"`
	WhenAction string          `json:"when_action"`
	WhenArgs   []Expr          `json:"when_args,omitempty"`
	Expects    []ExpectDecl    `json:"expects"`
	Span       Span            `json:"span"`
}
