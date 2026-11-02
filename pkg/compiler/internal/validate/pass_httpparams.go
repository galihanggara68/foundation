package validate

import (
	"fmt"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
)

// HTTPPathParamsPass enforces that every {segment} in an action's @http route path is supplied by
// an action parameter of exactly that name (FDN0222), so routing never dispatches on a path
// variable the handler does not read.
type HTTPPathParamsPass struct{}

func (HTTPPathParamsPass) Validate(sem *ir.SemanticIR, sym *SymbolTable) []ir.Diagnostic {
	var diags []ir.Diagnostic

	for i := range sem.Actions {
		a := &sem.Actions[i]
		path, ok := actionHTTPPath(*a)
		if !ok {
			continue
		}

		params := make(map[string]bool, len(a.Params))
		for _, p := range a.Params {
			params[p.Name] = true
		}

		reported := make(map[string]bool)
		for _, name := range ir.HTTPPathParamNames(path) {
			if name == "" || params[name] || reported[name] {
				continue
			}
			reported[name] = true
			diags = append(diags, ir.Diagnostic{
				Code:     "FDN0222",
				Severity: ir.SeverityError,
				Message:  fmt.Sprintf("@http route path %q on action %q routes path segment {%s} but no parameter of that name supplies it", path, a.Name, name),
				Span:     a.Span,
			})
		}
	}

	return diags
}

// actionHTTPPath reads an action's @http route path the same way LowerActionHTTP does, reporting
// false when the action is unrouted or the trait carries no path and the lowered default applies.
func actionHTTPPath(a ir.ActionDecl) (string, bool) {
	path, ok := "", false
	for _, tr := range a.Traits {
		if tr.Name != "http" {
			continue
		}
		if p, has := tr.Args["path"]; has {
			path, ok = p, true
		}
	}
	return path, ok && path != ""
}
