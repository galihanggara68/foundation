package gosdk

import (
	"fmt"
	"strings"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/arch"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/platform"
)

// standardErrorTypes pairs every standard kind with the Go type emitted for it, named
// after the kind's canonical platform name.
func standardErrorTypes() map[ir.ErrorKind]string {
	types := make(map[ir.ErrorKind]string, len(platform.StandardErrorKinds()))
	for _, name := range platform.ReservedErrorNames() {
		if kind, ok := platform.ReservedErrorName(name); ok {
			types[kind] = name + "Error"
		}
	}
	return types
}

// parentTypeFor returns the declared error type a kind embeds, falling back to the base
// error for a kind outside the standard vocabulary.
func parentTypeFor(kind ir.ErrorKind, types map[ir.ErrorKind]string) string {
	if parent, ok := types[kind]; ok {
		return parent
	}
	return "APIError"
}

// statusFallbacks derives the status-to-kind fallbacks from the vocabulary in its stable
// order, and reports the status two kinds answer — its first kind wins, as a tie-break.
func statusFallbacks() (statuses []int, kindByStatus map[int]ir.ErrorKind, tieStatus int, tieKinds []ir.ErrorKind) {
	kindByStatus = make(map[int]ir.ErrorKind)
	for _, kind := range platform.StandardErrorKinds() {
		policy, ok := platform.ErrorPolicyFor(kind)
		if !ok {
			continue
		}
		winner, exists := kindByStatus[policy.Status]
		if !exists {
			kindByStatus[policy.Status] = kind
			statuses = append(statuses, policy.Status)
			continue
		}
		if tieStatus == 0 {
			tieStatus, tieKinds = policy.Status, []ir.ErrorKind{winner, kind}
			continue
		}
		if policy.Status == tieStatus {
			tieKinds = append(tieKinds, kind)
		}
	}
	return statuses, kindByStatus, tieStatus, tieKinds
}

func emitErrors(archIR *arch.ArchitectureIR) string {
	var sb strings.Builder

	types := standardErrorTypes()
	kinds := platform.StandardErrorKinds()

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
`)

	for _, kind := range kinds {
		fmt.Fprintf(&sb, "type %s struct{ APIError }\n", types[kind])
	}
	sb.WriteString("\n")

	for _, kind := range kinds {
		fmt.Fprintf(&sb, "func (e *%s) Unwrap() error { return &e.APIError }\n", types[kind])
	}
	aliases := make([]string, len(kinds))
	aliasWidth := 0
	for i, kind := range kinds {
		aliases[i] = "Err" + strings.TrimSuffix(types[kind], "Error")
		if len(aliases[i]) > aliasWidth {
			aliasWidth = len(aliases[i])
		}
	}
	sb.WriteString("\n// Aliases matching standard naming conventions\ntype (\n")
	for i, kind := range kinds {
		fmt.Fprintf(&sb, "\t%-*s = %s\n", aliasWidth, aliases[i], types[kind])
	}
	sb.WriteString(")\n\n")

	for _, dErr := range archIR.DomainErrors {
		parent := parentTypeFor(dErr.Kind, types)
		sb.WriteString(fmt.Sprintf("// %sError represents the domain error %s (%s).\ntype %sError struct{\n\t%s\n}\n\nfunc (e *%sError) Unwrap() error {\n\treturn &e.%s\n}\n\n",
			dErr.Name, dErr.Name, dErr.Kind, dErr.Name, parent, dErr.Name, parent))
	}

	statuses, kindByStatus, tieStatus, tieKinds := statusFallbacks()

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
`)
	for _, status := range statuses {
		if status == tieStatus && len(tieKinds) == 2 {
			fmt.Fprintf(&sb, "\t\t// %d is answered by both %s and %s; %s wins as the tie-break.\n", tieStatus, tieKinds[0], tieKinds[1], tieKinds[0])
		}
		fmt.Fprintf(&sb, "\t\tcase %d:\n\t\t\traw.Kind = %q\n", status, kindByStatus[status])
	}
	sb.WriteString(`		default:
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
		parent := parentTypeFor(dErr.Kind, types)
		if parent == "APIError" {
			sb.WriteString(fmt.Sprintf("\tcase %q:\n\t\treturn &%sError{APIError: base}\n", dErr.Name, dErr.Name))
			continue
		}
		sb.WriteString(fmt.Sprintf("\tcase %q:\n\t\treturn &%sError{%s: %s{APIError: base}}\n", dErr.Name, dErr.Name, parent, parent))
	}
	sb.WriteString(`	}

	switch raw.Kind {
`)
	for _, kind := range kinds {
		fmt.Fprintf(&sb, "\tcase %q:\n\t\treturn &%s{APIError: base}\n", kind, types[kind])
	}
	sb.WriteString(`	default:
		return &base
	}
}
`)

	return sb.String()
}
