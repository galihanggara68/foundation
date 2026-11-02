package validate

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/platform"
)

// TraitValidationPass validates un-namespaced traits against the standard compiler traits registry:
// - reports FDN0224 (warning) for unknown un-namespaced traits
// - reports FDN0226 (warning) for argument problems on standard traits (unknown arg, missing required arg, non-numeric arg)
// Namespaced traits outside the reserved "foundation" namespace are passed through and never judged.
type TraitValidationPass struct{}

func (TraitValidationPass) Validate(sem *ir.SemanticIR, sym *SymbolTable) []ir.Diagnostic {
	var diags []ir.Diagnostic

	checkTrait := func(tr ir.TraitDecl) {
		if platform.IsNamespacedTrait(tr.Name) {
			// Namespaced traits belong to third-party tools or downstream generators and are never judged.
			return
		}

		stdTrait, ok := platform.StandardTrait(tr.Name)
		if !ok {
			diags = append(diags, ir.Diagnostic{
				Code:     "FDN0224",
				Severity: ir.SeverityWarning,
				Message:  fmt.Sprintf("unknown trait @%s; un-namespaced traits outside the standard vocabulary are ignored", tr.Name),
				Span:     tr.Span,
			})
			return
		}

		if stdTrait.Rejected {
			return
		}

		// 1. Check for unknown arguments
		if len(stdTrait.Args) > 0 {
			for argName := range tr.Args {
				if _, found := stdTrait.Arg(argName); !found {
					diags = append(diags, ir.Diagnostic{
						Code:     "FDN0226",
						Severity: ir.SeverityWarning,
						Message:  fmt.Sprintf("unknown argument %q for standard trait @%s", argName, tr.Name),
						Span:     tr.Span,
					})
				}
			}
		}

		// 2. Check required arguments
		for _, arg := range stdTrait.Args {
			if !arg.Required {
				continue
			}
			val := tr.Args[arg.Name]
			if val == "" {
				for _, alias := range arg.Aliases {
					if v, ok := tr.Args[alias]; ok && v != "" {
						val = v
						break
					}
				}
			}
			if strings.TrimSpace(val) == "" {
				diags = append(diags, ir.Diagnostic{
					Code:     "FDN0226",
					Severity: ir.SeverityWarning,
					Message:  fmt.Sprintf("standard trait @%s requires argument %q", tr.Name, arg.Name),
					Span:     tr.Span,
				})
			}
		}

		// 3. Check numeric arguments
		for _, arg := range stdTrait.Args {
			if !arg.Numeric {
				continue
			}
			val, found := tr.Args[arg.Name]
			if !found {
				for _, alias := range arg.Aliases {
					if v, ok := tr.Args[alias]; ok {
						val, found = v, true
						break
					}
				}
			}
			if found && val != "" {
				unquoted := strings.Trim(val, `"'`)
				if _, err := strconv.Atoi(unquoted); err != nil {
					diags = append(diags, ir.Diagnostic{
						Code:     "FDN0226",
						Severity: ir.SeverityWarning,
						Message:  fmt.Sprintf("argument %q on standard trait @%s must be numeric, got %q", arg.Name, tr.Name, val),
						Span:     tr.Span,
					})
				}
			}
		}
	}

	// Entities and their fields
	for _, e := range sem.Entities {
		for _, tr := range e.Traits {
			checkTrait(tr)
		}
		for _, f := range e.Fields {
			for _, tr := range f.Traits {
				checkTrait(tr)
			}
		}
	}

	// Value types and their fields
	for _, v := range sem.Values {
		for _, f := range v.Fields {
			for _, tr := range f.Traits {
				checkTrait(tr)
			}
		}
	}

	// Actor and its fields
	if sem.Actor != nil {
		for _, f := range sem.Actor.Fields {
			for _, tr := range f.Traits {
				checkTrait(tr)
			}
		}
	}

	// Actions
	for _, a := range sem.Actions {
		for _, tr := range a.Traits {
			checkTrait(tr)
		}
	}

	// Slots and their methods
	for _, s := range sem.Slots {
		for _, m := range s.Methods {
			for _, tr := range m.Traits {
				checkTrait(tr)
			}
		}
	}

	return diags
}
