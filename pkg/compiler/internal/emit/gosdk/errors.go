package gosdk

import (
	"fmt"
	"strings"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/arch"
)

func emitErrors(archIR *arch.ArchitectureIR) string {
	var sb strings.Builder
	sb.WriteString(`package client

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// APIError is the base structured error returned by API endpoints.
type APIError struct {
	StatusCode int    ` + "`json:\"status_code\"`" + `
	Code       string ` + "`json:\"code\"`" + `
	Kind       string ` + "`json:\"kind\"`" + `
	Message    string ` + "`json:\"message\"`" + `
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("api error (%d %s): %s", e.StatusCode, e.Code, e.Message)
	}
	return fmt.Sprintf("api error (%d %s)", e.StatusCode, e.Code)
}

// Built-in standard error kinds.
type UnauthenticatedError struct{ APIError }
type ForbiddenError struct{ APIError }
type NotFoundError struct{ APIError }
type ConflictError struct{ APIError }
type ValidationError struct{ APIError }

func (e *UnauthenticatedError) Unwrap() error { return &e.APIError }
func (e *ForbiddenError) Unwrap() error { return &e.APIError }
func (e *NotFoundError) Unwrap() error { return &e.APIError }
func (e *ConflictError) Unwrap() error { return &e.APIError }
func (e *ValidationError) Unwrap() error { return &e.APIError }

// Aliases matching standard naming conventions
type (
	ErrUnauthenticated = UnauthenticatedError
	ErrForbidden       = ForbiddenError
	ErrNotFound        = NotFoundError
	ErrConflict        = ConflictError
	ErrValidation      = ValidationError
)

`)

	for _, dErr := range archIR.DomainErrors {
		parent := "APIError"
		switch dErr.Kind {
		case "unauthenticated":
			parent = "UnauthenticatedError"
		case "forbidden":
			parent = "ForbiddenError"
		case "not_found":
			parent = "NotFoundError"
		case "conflict":
			parent = "ConflictError"
		case "validation":
			parent = "ValidationError"
		case "internal":
			parent = "InternalError"
		}
		sb.WriteString(fmt.Sprintf("// %sError represents the domain error %s (%s).\ntype %sError struct{\n\t%s\n}\n\nfunc (e *%sError) Unwrap() error {\n\treturn &e.%s\n}\n\n",
			dErr.Name, dErr.Name, dErr.Kind, dErr.Name, parent, dErr.Name, parent))
	}

	sb.WriteString(`type rawErrorPayload struct {
	Code    string ` + "`json:\"code\"`" + `
	Kind    string ` + "`json:\"kind\"`" + `
	Message string ` + "`json:\"message\"`" + `
}

func decodeError(statusCode int, body []byte) error {
	var raw rawErrorPayload
	_ = json.Unmarshal(body, &raw)

	if raw.Code == "" {
		raw.Code = http.StatusText(statusCode)
	}
	if raw.Kind == "" {
		switch statusCode {
		case 401:
			raw.Kind = "unauthenticated"
		case 403:
			raw.Kind = "forbidden"
		case 404:
			raw.Kind = "not_found"
		case 409:
			raw.Kind = "conflict"
		case 400:
			raw.Kind = "validation"
		default:
			raw.Kind = "internal"
		}
	}

	base := APIError{
		StatusCode: statusCode,
		Code:       raw.Code,
		Kind:       raw.Kind,
		Message:    raw.Message,
	}

	switch raw.Code {
`)
	for _, dErr := range archIR.DomainErrors {
		switch dErr.Kind {
		case "unauthenticated":
			sb.WriteString(fmt.Sprintf("\tcase %q:\n\t\treturn &%sError{UnauthenticatedError: UnauthenticatedError{APIError: base}}\n", dErr.Name, dErr.Name))
		case "forbidden":
			sb.WriteString(fmt.Sprintf("\tcase %q:\n\t\treturn &%sError{ForbiddenError: ForbiddenError{APIError: base}}\n", dErr.Name, dErr.Name))
		case "not_found":
			sb.WriteString(fmt.Sprintf("\tcase %q:\n\t\treturn &%sError{NotFoundError: NotFoundError{APIError: base}}\n", dErr.Name, dErr.Name))
		case "conflict":
			sb.WriteString(fmt.Sprintf("\tcase %q:\n\t\treturn &%sError{ConflictError: ConflictError{APIError: base}}\n", dErr.Name, dErr.Name))
		case "validation":
			sb.WriteString(fmt.Sprintf("\tcase %q:\n\t\treturn &%sError{ValidationError: ValidationError{APIError: base}}\n", dErr.Name, dErr.Name))
		case "internal":
			sb.WriteString(fmt.Sprintf("\tcase %q:\n\t\treturn &%sError{InternalError: InternalError{APIError: base}}\n", dErr.Name, dErr.Name))
		default:
			sb.WriteString(fmt.Sprintf("\tcase %q:\n\t\treturn &%sError{APIError: base}\n", dErr.Name, dErr.Name))
		}
	}
	sb.WriteString(`	}

	switch raw.Kind {
	case "unauthenticated":
		return &UnauthenticatedError{APIError: base}
	case "forbidden":
		return &ForbiddenError{APIError: base}
	case "not_found":
		return &NotFoundError{APIError: base}
	case "conflict":
		return &ConflictError{APIError: base}
	case "validation":
		return &ValidationError{APIError: base}
	default:
		return &base
	}
}
`)

	return sb.String()
}
