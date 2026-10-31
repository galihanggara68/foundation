package ir

import "fmt"

// Span represents an exact source location in a Blueprint file.
type Span struct {
	File   string `json:"file"`
	Line   int    `json:"line"`
	Column int    `json:"column"`
}

func (s Span) String() string {
	return fmt.Sprintf("%s:%d:%d", s.File, s.Line, s.Column)
}

// Severity indicates whether a diagnostic is an error or warning.
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
)

// Diagnostic reports a syntax, type, or architectural invariant violation.
type Diagnostic struct {
	Code     string   `json:"code"`
	Severity Severity `json:"severity"`
	Message  string   `json:"message"`
	Span     Span     `json:"span"`
}

// HasErrors reports whether a diagnostic list contains anything that aborts a build.
// Warnings travel alongside a successful result, so a caller decides on this rather
// than on the list being empty.
func HasErrors(diags []Diagnostic) bool {
	for _, d := range diags {
		if d.Severity == SeverityError {
			return true
		}
	}
	return false
}
