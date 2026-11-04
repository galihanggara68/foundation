package validate

import (
	"fmt"
	"strings"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/diagnostics"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
)

// HTTPPathParamsPass confirms that action parameters supply all route path segments (FDN0222).
// The pass also sets InPath to true for parameters that appear in the route path.
type HTTPPathParamsPass struct{}

func (HTTPPathParamsPass) Validate(sem *ir.SemanticIR, sym *SymbolTable) []ir.Diagnostic {
	var diags []ir.Diagnostic

	for i := range sem.Actions {
		a := &sem.Actions[i]
		ir.EnrichActionParamsInPath(a)
		sym.actions[a.Name] = *a

		paths := ir.ActionRoutePaths(*a)
		if len(paths) == 0 {
			continue
		}

		params := make(map[string]bool, len(a.Params))
		for _, p := range a.Params {
			params[p.Name] = true
		}

		for _, path := range paths {
			reported := make(map[string]bool)
			for _, name := range ir.HTTPPathParamNames(path) {
				base := strings.Split(name, ":")[0]
				if base == "" || params[base] || reported[base] {
					continue
				}
				reported[base] = true
				diags = append(diags, diagnostics.NewWithDetail(
					"FDN0222",
					a.Span,
					fmt.Sprintf("@http route path %q on action %q routes path segment {%s} but no parameter of that name supplies it", path, a.Name, name),
				))
			}
		}
	}

	return diags
}

// actionHTTPPath reads an action's route path from @http or @route traits.
func actionHTTPPath(a ir.ActionDecl) (string, bool) {
	paths := ir.ActionRoutePaths(a)
	if len(paths) == 0 {
		return "", false
	}
	return paths[0], true
}
