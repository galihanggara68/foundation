package validate

import (
	"fmt"
	"strings"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
)

// DeclarationPass populates the SymbolTable and validates top-level declarations, error kinds, field types, and policies.
type DeclarationPass struct{}

func (DeclarationPass) Validate(sem *ir.SemanticIR, sym *SymbolTable) []ir.Diagnostic {
	var diags []ir.Diagnostic

	for _, en := range sem.Enums {
		registerUniqueDecl(sym.enums, "enum", en.Name, en, en.Span, &diags)
	}
	for _, v := range sem.Values {
		registerUniqueDecl(sym.values, "value", v.Name, v, v.Span, &diags)
	}
	for _, e := range sem.Entities {
		registerUniqueDecl(sym.entities, "entity", e.Name, e, e.Span, &diags)
	}
	for _, errDecl := range sem.Errors {
		if !validErrorKinds[errDecl.Kind] {
			diags = append(diags, ir.Diagnostic{
				Code:     "FDN0201",
				Severity: ir.SeverityError,
				Message:  fmt.Sprintf("invalid error kind %q in error %q", errDecl.Kind, errDecl.Name),
				Span:     errDecl.Span,
			})
		}
		registerUniqueDecl(sym.errors, "error", errDecl.Name, errDecl, errDecl.Span, &diags)
	}
	for _, ev := range sem.Events {
		registerUniqueDecl(sym.events, "event", ev.Name, ev, ev.Span, &diags)
	}
	for _, pol := range sem.Policies {
		registerUniqueDecl(sym.policies, "policy", pol.Name, pol, pol.Span, &diags)
	}
	for _, sl := range sem.Slots {
		registerUniqueDecl(sym.slots, "slot", sl.Name, sl, sl.Span, &diags)
	}
	for _, a := range sem.Actions {
		registerUniqueDecl(sym.actions, "action", a.Name, a, a.Span, &diags)
	}

	validateFields := func(fields []ir.FieldDecl) {
		for _, f := range fields {
			if !sym.isKnownType(f.Type) {
				diags = append(diags, ir.Diagnostic{
					Code:     "FDN0202",
					Severity: ir.SeverityError,
					Message:  fmt.Sprintf("unknown type %q for field %q", f.Type, f.Name),
					Span:     f.Span,
				})
				continue
			}
			if strings.HasPrefix(f.DefaultVal, ".") {
				variant := strings.TrimPrefix(f.DefaultVal, ".")
				if !sym.enumHasVariant(f.Type, variant) {
					diags = append(diags, ir.Diagnostic{
						Code:     "FDN0205",
						Severity: ir.SeverityError,
						Message:  fmt.Sprintf("invalid default enum variant %s for field %q of type %s", f.DefaultVal, f.Name, f.Type),
						Span:     f.Span,
					})
				}
			}
		}
	}

	for _, v := range sem.Values {
		validateFields(v.Fields)
	}
	for _, e := range sem.Entities {
		validateFields(e.Fields)
	}
	for _, ev := range sem.Events {
		validateFields(ev.Fields)
	}
	for _, sl := range sem.Slots {
		for _, m := range sl.Methods {
			for _, p := range m.Params {
				if !sym.isKnownType(p.Type) {
					diags = append(diags, ir.Diagnostic{
						Code:     "FDN0202",
						Severity: ir.SeverityError,
						Message:  fmt.Sprintf("unknown parameter type %q in slot %s.%s", p.Type, sl.Name, m.Name),
						Span:     p.Span,
					})
				}
			}
			if m.ReturnType != "" && !sym.isKnownType(m.ReturnType) {
				diags = append(diags, ir.Diagnostic{
					Code:     "FDN0202",
					Severity: ir.SeverityError,
					Message:  fmt.Sprintf("unknown return type %q in slot %s.%s", m.ReturnType, sl.Name, m.Name),
					Span:     m.Span,
				})
			}
			for _, th := range m.Throws {
				validateErrorRef(th, fmt.Sprintf("slot %s.%s throws", sl.Name, m.Name), m.Span, sym, &diags)
			}
		}
	}

	for i := range sem.Policies {
		pol := &sem.Policies[i]
		pScope := make(map[string]string)
		for _, param := range pol.Params {
			if !sym.isKnownType(param.Type) {
				diags = append(diags, ir.Diagnostic{
					Code:     "FDN0202",
					Severity: ir.SeverityError,
					Message:  fmt.Sprintf("unknown parameter type %q in policy %q", param.Type, pol.Name),
					Span:     param.Span,
				})
			}
			pScope[param.Name] = param.Type
		}
		bt, ok := inferExprType(&pol.Body, pScope, sym, &diags)
		if ok && bt != "bool" {
			diags = append(diags, ir.Diagnostic{
				Code:     "FDN0205",
				Severity: ir.SeverityError,
				Message:  fmt.Sprintf("policy %q body must evaluate to bool, got %s", pol.Name, bt),
				Span:     pol.Body.Span,
			})
		}
		sym.policies[pol.Name] = *pol
	}

	return diags
}
