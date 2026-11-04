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
// - reports FDN0230 (error) for unknown field references in composite entity traits
// Namespaced traits outside the reserved "foundation" namespace are passed through and never judged.
type TraitValidationPass struct{}

type traitLocation struct {
	tr     ir.TraitDecl
	host   platform.TraitHost
	entity *ir.EntityDecl
}

func (TraitValidationPass) Validate(sem *ir.SemanticIR, sym *SymbolTable) []ir.Diagnostic {
	var diags []ir.Diagnostic

	for _, loc := range collectAllTraitsWithHost(sem) {
		tr := loc.tr
		if platform.IsNamespacedTrait(tr.Name) {
			// Namespaced traits belong to third-party tools or downstream generators and are never judged.
			continue
		}

		stdTrait, ok := platform.StandardTrait(tr.Name)
		if !ok {
			diags = append(diags, ir.Diagnostic{
				Code:     "FDN0224",
				Severity: ir.SeverityWarning,
				Message:  fmt.Sprintf("unknown trait @%s; un-namespaced traits outside the standard vocabulary are ignored", tr.Name),
				Span:     tr.Span,
			})
			continue
		}

		if stdTrait.Rejected {
			continue
		}

		// Field-level @index and @unique traits accept no arguments
		if loc.host == platform.TraitOnField && (tr.Name == "index" || tr.Name == "unique") {
			if len(tr.Args) > 0 || len(tr.ListArgs) > 0 {
				diags = append(diags, ir.Diagnostic{
					Code:     "FDN0226",
					Severity: ir.SeverityWarning,
					Message:  fmt.Sprintf("field-level trait @%s accepts no arguments", tr.Name),
					Span:     tr.Span,
				})
			}
			continue
		}

		// Entity-level composite traits (@index and @unique)
		if loc.host == platform.TraitOnEntity && (tr.Name == "index" || tr.Name == "unique") {
			checkCompositeEntityTrait(&diags, tr, loc.entity, sym, stdTrait)
			continue
		}

		// 1. Check for unknown arguments
		validateTraitArgNames(&diags, tr, stdTrait)

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

	return diags
}

// checkCompositeEntityTrait validates @index and @unique traits on entity declarations.
func checkCompositeEntityTrait(diags *[]ir.Diagnostic, tr ir.TraitDecl, entity *ir.EntityDecl, sym *SymbolTable, stdTrait platform.Trait) {
	// 1. Check for unknown arguments
	validateTraitArgNames(diags, tr, stdTrait)

	// 2. Validate fields list argument
	fields, hasList := tr.GetListArg("fields")
	if !hasList {
		fields, hasList = tr.GetListArg("_")
	}

	entName := ""
	if entity != nil {
		entName = entity.Name
	}

	if !hasList {
		_, hasScalar := tr.GetStringArg("fields")
		if !hasScalar {
			_, hasScalar = tr.GetStringArg("_")
		}
		if hasScalar {
			*diags = append(*diags, ir.Diagnostic{
				Code:     "FDN0226",
				Severity: ir.SeverityWarning,
				Message:  fmt.Sprintf("composite @%s trait on entity %q requires at least two fields; use field-level @%s for single-field constraints", tr.Name, entName, tr.Name),
				Span:     tr.Span,
			})
		} else {
			*diags = append(*diags, ir.Diagnostic{
				Code:     "FDN0226",
				Severity: ir.SeverityWarning,
				Message:  fmt.Sprintf("composite @%s trait on entity %q requires argument %q", tr.Name, entName, "fields"),
				Span:     tr.Span,
			})
		}
		return
	}

	// 3. Check minimum field count
	if len(fields) == 0 {
		*diags = append(*diags, ir.Diagnostic{
			Code:     "FDN0226",
			Severity: ir.SeverityWarning,
			Message:  fmt.Sprintf("composite @%s trait on entity %q requires at least two fields", tr.Name, entName),
			Span:     tr.Span,
		})
		return
	}
	if len(fields) == 1 {
		*diags = append(*diags, ir.Diagnostic{
			Code:     "FDN0226",
			Severity: ir.SeverityWarning,
			Message:  fmt.Sprintf("composite @%s trait on entity %q requires at least two fields; use field-level @%s for single-field constraints", tr.Name, entName, tr.Name),
			Span:     tr.Span,
		})
	}

	// 4. Confirm that every field exists on target entity
	var entFields map[string]ir.FieldDecl
	if sym != nil {
		entFields = sym.fieldsOf(entName)
	}
	if len(entFields) == 0 && entity != nil {
		entFields = make(map[string]ir.FieldDecl, len(entity.Fields))
		for _, f := range entity.Fields {
			entFields[f.Name] = f
		}
	}
	for _, f := range fields {
		if _, exists := entFields[f]; !exists {
			*diags = append(*diags, ir.Diagnostic{
				Code:     "FDN0230",
				Severity: ir.SeverityError,
				Message:  fmt.Sprintf("unknown field %q on entity %q in @%s trait", f, entName, tr.Name),
				Span:     tr.Span,
			})
		}
	}
}

// validateTraitArgNames checks for unknown arguments on standard traits.
func validateTraitArgNames(diags *[]ir.Diagnostic, tr ir.TraitDecl, stdTrait platform.Trait) {
	allArgKeys := make(map[string]bool)
	for k := range tr.Args {
		allArgKeys[k] = true
	}
	for k := range tr.ListArgs {
		allArgKeys[k] = true
	}
	if len(allArgKeys) == 0 {
		return
	}
	argNames := make([]string, 0, len(allArgKeys))
	for k := range allArgKeys {
		argNames = append(argNames, k)
	}
	sort.Strings(argNames)
	for _, argName := range argNames {
		if _, found := stdTrait.Arg(argName); !found {
			*diags = append(*diags, ir.Diagnostic{
				Code:     "FDN0226",
				Severity: ir.SeverityWarning,
				Message:  fmt.Sprintf("unknown argument %q for standard trait @%s", argName, tr.Name),
				Span:     tr.Span,
			})
		}
	}
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

// collectAllTraitsWithHost traverses declarations in the semantic IR and records trait hosts.
func collectAllTraitsWithHost(sem *ir.SemanticIR) []traitLocation {
	var locs []traitLocation
	for i := range sem.Entities {
		e := &sem.Entities[i]
		for _, tr := range e.Traits {
			locs = append(locs, traitLocation{tr: tr, host: platform.TraitOnEntity, entity: e})
		}
		for _, f := range e.Fields {
			for _, tr := range f.Traits {
				locs = append(locs, traitLocation{tr: tr, host: platform.TraitOnField, entity: e})
			}
		}
	}
	for i := range sem.Values {
		v := &sem.Values[i]
		for _, f := range v.Fields {
			for _, tr := range f.Traits {
				locs = append(locs, traitLocation{tr: tr, host: platform.TraitOnField})
			}
		}
	}
	if sem.Actor != nil {
		for _, f := range sem.Actor.Fields {
			for _, tr := range f.Traits {
				locs = append(locs, traitLocation{tr: tr, host: platform.TraitOnField})
			}
		}
	}
	for i := range sem.Actions {
		a := &sem.Actions[i]
		for _, tr := range a.Traits {
			locs = append(locs, traitLocation{tr: tr, host: platform.TraitOnAction})
		}
	}
	for _, s := range sem.Slots {
		for _, m := range s.Methods {
			for _, tr := range m.Traits {
				locs = append(locs, traitLocation{tr: tr, host: platform.TraitOnSlotMethod})
			}
		}
	}
	return locs
}
