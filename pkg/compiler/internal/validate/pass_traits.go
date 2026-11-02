package validate

import (
	"fmt"
	"sort"
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

		// 1. Check for unknown arguments (sorted to guarantee deterministic diagnostic ordering)
		if len(tr.Args) > 0 {
			argNames := make([]string, 0, len(tr.Args))
			for k := range tr.Args {
				argNames = append(argNames, k)
			}
			sort.Strings(argNames)

			for _, argName := range argNames {
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
			val, found := resolveTraitArg(tr.Args, arg)
			if !found || strings.TrimSpace(val) == "" {
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
			val, found := resolveTraitArg(tr.Args, arg)
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

	for _, tr := range collectAllTraits(sem) {
		checkTrait(tr)
	}

	return diags
}

// resolveTraitArg resolves a trait argument value by its primary name or accepted aliases.
func resolveTraitArg(args map[string]string, arg platform.TraitArg) (string, bool) {
	if val, ok := args[arg.Name]; ok && val != "" {
		return val, true
	}
	for _, alias := range arg.Aliases {
		if val, ok := args[alias]; ok && val != "" {
			return val, true
		}
	}
	return "", false
}

// collectAllTraits traverses all declarations in the semantic IR and returns attached traits.
func collectAllTraits(sem *ir.SemanticIR) []ir.TraitDecl {
	var traits []ir.TraitDecl
	for _, e := range sem.Entities {
		traits = append(traits, e.Traits...)
		for _, f := range e.Fields {
			traits = append(traits, f.Traits...)
		}
	}
	for _, v := range sem.Values {
		for _, f := range v.Fields {
			traits = append(traits, f.Traits...)
		}
	}
	if sem.Actor != nil {
		for _, f := range sem.Actor.Fields {
			traits = append(traits, f.Traits...)
		}
	}
	for _, a := range sem.Actions {
		traits = append(traits, a.Traits...)
	}
	for _, s := range sem.Slots {
		for _, m := range s.Methods {
			traits = append(traits, m.Traits...)
		}
	}
	return traits
}
