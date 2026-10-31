package platform

import "github.com/galihanggara68/foundation/pkg/compiler/internal/ir"

// MinHTTPStatus and MaxHTTPStatus bound the status a declared error may carry.
const (
	MinHTTPStatus = 100
	MaxHTTPStatus = 599
)

// ErrorPolicy is the platform's standard treatment of one error kind — what the
// kind resolves to when a blueprint does not declare its own.
type ErrorPolicy struct {
	Status  int
	Posture ir.ErrorPosture
}

// standardError is one entry of the vocabulary: the kind, the name reserved for it, the
// treatment it defaults to, and — for the kinds the platform raises itself — the message
// of the error every service pre-declares. Builtin is empty for kinds the platform does
// not raise on its own.
type standardError struct {
	Kind    ir.ErrorKind
	Name    string
	Policy  ErrorPolicy
	Builtin string
}

// errorVocabulary is the single source of truth for the standard vocabulary. Its
// order is stable so that everything enumerating it — the status map an emitter
// generates, the reserved names — produces deterministic output.
var errorVocabulary = []standardError{
	{Kind: ir.ErrorKindNotFound, Name: "NotFound", Policy: ErrorPolicy{Status: 404, Posture: ir.PostureBusiness}},
	{Kind: ir.ErrorKindValidation, Name: "Validation", Policy: ErrorPolicy{Status: 400, Posture: ir.PostureBusiness}},
	{Kind: ir.ErrorKindConflict, Name: "Conflict", Policy: ErrorPolicy{Status: 409, Posture: ir.PostureBusiness}, Builtin: "optimistic concurrency conflict"},
	{Kind: ir.ErrorKindUnauthenticated, Name: "Unauthenticated", Policy: ErrorPolicy{Status: 401, Posture: ir.PostureBusiness}, Builtin: "unauthenticated actor"},
	{Kind: ir.ErrorKindUnauthorized, Name: "Unauthorized", Policy: ErrorPolicy{Status: 401, Posture: ir.PostureBusiness}},
	{Kind: ir.ErrorKindForbidden, Name: "Forbidden", Policy: ErrorPolicy{Status: 403, Posture: ir.PostureBusiness}},
	{Kind: ir.ErrorKindInternal, Name: "Internal", Policy: ErrorPolicy{Status: 500, Posture: ir.PostureTechnical}},
}

// ErrorPolicyFor returns the platform's standard policy for an error kind, and
// whether the kind belongs to the standard vocabulary at all — the single lookup
// the validator and the emitters both read, so neither repeats the mapping.
func ErrorPolicyFor(kind ir.ErrorKind) (ErrorPolicy, bool) {
	for _, entry := range errorVocabulary {
		if entry.Kind == kind {
			return entry.Policy, true
		}
	}
	return ErrorPolicy{}, false
}

// StandardErrorKinds returns every kind in the standard vocabulary, in stable order.
func StandardErrorKinds() []ir.ErrorKind {
	kinds := make([]ir.ErrorKind, 0, len(errorVocabulary))
	for _, entry := range errorVocabulary {
		kinds = append(kinds, entry.Kind)
	}
	return kinds
}

// ReservedErrorName reports the kind whose name the vocabulary owns, so a declaration
// reusing a standard name is recognised without anyone restating the reserved set.
func ReservedErrorName(name string) (ir.ErrorKind, bool) {
	for _, entry := range errorVocabulary {
		if entry.Name == name {
			return entry.Kind, true
		}
	}
	return "", false
}

// ReservedErrorNames returns every name the vocabulary reserves, in stable order. The
// table is what keeps it one name per kind: its test holds the names and the kinds to a
// bijection, so a kind added without a name is caught rather than silently unreserved.
func ReservedErrorNames() []string {
	names := make([]string, 0, len(errorVocabulary))
	for _, entry := range errorVocabulary {
		names = append(names, entry.Name)
	}
	return names
}

// BuiltinErrors returns the errors every service pre-declares — the ones the platform
// raises itself — with their status and posture already read from the vocabulary, so the
// validator and the emitters both seed from here instead of writing the names, kinds and
// messages out again.
func BuiltinErrors() []ir.ErrorDecl {
	var builtins []ir.ErrorDecl
	for _, entry := range errorVocabulary {
		if entry.Builtin == "" {
			continue
		}
		builtins = append(builtins, ir.ErrorDecl{
			Name:    entry.Name,
			Kind:    entry.Kind,
			Message: entry.Builtin,
			Status:  entry.Policy.Status,
			Posture: entry.Policy.Posture,
		})
	}
	return builtins
}

// IsBuiltinErrorName reports whether name is claimed by an error every service ships
// with, which is what an emitter needs before declaring one of the same name.
func IsBuiltinErrorName(name string) bool {
	for _, entry := range errorVocabulary {
		if entry.Name == name && entry.Builtin != "" {
			return true
		}
	}
	return false
}
