package platform

import "strings"

// ReservedTraitNamespace names the namespace the compiler owns. A namespaced trait in it is
// an error: the standard traits are written without a namespace.
const ReservedTraitNamespace = "foundation"

// TraitHost is the part of a declaration a trait is meaningful on.
type TraitHost string

const (
	TraitOnField      TraitHost = "field"
	TraitOnEntity     TraitHost = "entity"
	TraitOnAction     TraitHost = "action"
	TraitOnSlotMethod TraitHost = "slot method"
)

// TraitArg is an argument the compiler reads from a standard trait.
type TraitArg struct {
	Name string
	// Aliases are accepted spellings of the same argument. The parser records a positional
	// argument under "_", which is why a trait taking one value accepts both forms.
	Aliases []string
	// Required marks an argument a consumer cannot work without.
	Required bool
	// Numeric marks an argument a consumer parses as an integer.
	Numeric bool
}

// Trait is a standard trait the compiler consumes itself. A trait outside this table and
// outside any namespace is unknown to the compiler and is reported rather than passed along
// silently. A namespaced trait belongs to whoever reserved the namespace, so it is always
// passed along and never judged.
type Trait struct {
	Name  string
	Hosts []TraitHost
	// Args are the arguments the compiler reads. A trait that declares none passes whatever
	// arguments it carries: only a trait with an argument contract can have it violated.
	Args []TraitArg
	// Rejected marks a standard trait the grammar refuses outright.
	Rejected bool
}

var standardTraits = []Trait{
	{Name: "auto_now_add", Hosts: []TraitHost{TraitOnField}},
	{Name: "cache", Hosts: []TraitHost{TraitOnAction}, Args: []TraitArg{{Name: "ttl", Numeric: true}}},
	{Name: "cron", Hosts: []TraitHost{TraitOnAction}, Rejected: true},
	{Name: "event_handler", Hosts: []TraitHost{TraitOnAction}, Args: []TraitArg{{Name: "event", Aliases: []string{"_"}, Required: true}}},
	{Name: "external", Hosts: []TraitHost{TraitOnSlotMethod}},
	{Name: "fk", Hosts: []TraitHost{TraitOnField}},
	{Name: "http", Hosts: []TraitHost{TraitOnAction}, Args: []TraitArg{{Name: "method"}, {Name: "path"}}},
	{Name: "idempotent", Hosts: []TraitHost{TraitOnAction}, Args: []TraitArg{{Name: "key"}}},
	{Name: "index", Hosts: []TraitHost{TraitOnField}},
	{Name: "internal", Hosts: []TraitHost{TraitOnAction}},
	{Name: "outbox", Hosts: []TraitHost{TraitOnAction}},
	{Name: "pk", Hosts: []TraitHost{TraitOnField}},
	{Name: "pure", Hosts: []TraitHost{TraitOnSlotMethod}},
	{Name: "renamed_from", Hosts: []TraitHost{TraitOnField}},
	{Name: "retry", Hosts: []TraitHost{TraitOnAction}, Args: []TraitArg{{Name: "attempts", Numeric: true}, {Name: "backoff", Numeric: true}}},
	{Name: "table", Hosts: []TraitHost{TraitOnEntity}, Args: []TraitArg{{Name: "name", Aliases: []string{"_"}}}},
	{Name: "transactional", Hosts: []TraitHost{TraitOnAction}},
	{Name: "unique", Hosts: []TraitHost{TraitOnField}},
	{Name: "version", Hosts: []TraitHost{TraitOnField}},
}

var standardTraitIndex = func() map[string]Trait {
	index := make(map[string]Trait, len(standardTraits))
	for _, tr := range standardTraits {
		index[tr.Name] = tr
	}
	return index
}()

// StandardTraits returns the compiler's standard traits, in a stable order.
func StandardTraits() []Trait {
	return append([]Trait(nil), standardTraits...)
}

// StandardTrait returns the standard trait of that name.
func StandardTrait(name string) (Trait, bool) {
	tr, ok := standardTraitIndex[name]
	return tr, ok
}

// IsStandardTrait reports whether the compiler consumes a trait of that name.
func IsStandardTrait(name string) bool {
	_, ok := standardTraitIndex[name]
	return ok
}

// TraitNamespace splits a trait name into its namespace and local name. An un-namespaced
// trait yields an empty namespace and the name unchanged.
func TraitNamespace(name string) (namespace, local string) {
	namespace, local, found := strings.Cut(name, ":")
	if !found {
		return "", name
	}
	return namespace, local
}

// IsNamespacedTrait reports whether a trait name carries a namespace.
func IsNamespacedTrait(name string) bool {
	namespace, _ := TraitNamespace(name)
	return namespace != ""
}

// Arg returns the argument a trait reads under any of its spellings.
func (tr Trait) Arg(argName string) (TraitArg, bool) {
	for _, a := range tr.Args {
		if a.Name == argName {
			return a, true
		}
		for _, alias := range a.Aliases {
			if alias == argName {
				return a, true
			}
		}
	}
	return TraitArg{}, false
}
