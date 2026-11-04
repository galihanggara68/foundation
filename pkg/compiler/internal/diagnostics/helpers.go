package diagnostics

import (
	"fmt"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
)

// Lookup returns the diagnostic definition for the given code.
func Lookup(code string) (Definition, bool) {
	def, ok := Registry[code]
	return def, ok
}

// severityOf returns the registered severity for code, or ir.SeverityError when missing.
func severityOf(code string) ir.Severity {
	if def, ok := Registry[code]; ok {
		return def.Severity
	}
	return ir.SeverityError
}

// New creates an ir.Diagnostic for the code, span, and optional message arguments.
// When the code exists in the registry, New formats MessageTemplate with args.
// When the registry does not contain the code, New creates an error diagnostic with a fallback message.
func New(code string, span ir.Span, args ...any) ir.Diagnostic {
	def, ok := Registry[code]
	if !ok {
		msg := code
		if len(args) > 0 {
			msg = fmt.Sprint(args...)
		}
		return ir.Diagnostic{
			Code:     code,
			Severity: ir.SeverityError,
			Message:  msg,
			Span:     span,
		}
	}
	msg := def.MessageTemplate
	if len(args) > 0 {
		msg = fmt.Sprintf(def.MessageTemplate, args...)
	}
	return ir.Diagnostic{
		Code:     def.Code,
		Severity: def.Severity,
		Message:  msg,
		Span:     span,
	}
}

// NewWithDetail creates an ir.Diagnostic with a custom message.
// The diagnostic inherits its severity from the registry definition.
func NewWithDetail(code string, span ir.Span, msg string) ir.Diagnostic {
	return ir.Diagnostic{
		Code:     code,
		Severity: severityOf(code),
		Message:  msg,
		Span:     span,
	}
}
