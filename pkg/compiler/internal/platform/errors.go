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
	Status int
}

// errorVocabulary is the single source of truth for the standard vocabulary. Its
// order is stable so that everything enumerating it — the status map an emitter
// generates, the reserved names — produces deterministic output.
var errorVocabulary = []struct {
	Kind   ir.ErrorKind
	Policy ErrorPolicy
}{
	{ir.ErrorKindNotFound, ErrorPolicy{Status: 404}},
	{ir.ErrorKindValidation, ErrorPolicy{Status: 400}},
	{ir.ErrorKindConflict, ErrorPolicy{Status: 409}},
	{ir.ErrorKindUnauthenticated, ErrorPolicy{Status: 401}},
	{ir.ErrorKindUnauthorized, ErrorPolicy{Status: 401}},
	{ir.ErrorKindForbidden, ErrorPolicy{Status: 403}},
	{ir.ErrorKindInternal, ErrorPolicy{Status: 500}},
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
