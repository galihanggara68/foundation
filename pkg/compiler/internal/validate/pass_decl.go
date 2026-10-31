package validate

import (
	"fmt"
	"strings"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/arch"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/platform"
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
	for i := range sem.Errors {
		errDecl := &sem.Errors[i]
		policy, standard := platform.ErrorPolicyFor(errDecl.Kind)
		if !standard {
			diags = append(diags, ir.Diagnostic{
				Code:     "FDN0201",
				Severity: ir.SeverityError,
				Message:  fmt.Sprintf("invalid error kind %q in error %q", errDecl.Kind, errDecl.Name),
				Span:     errDecl.Span,
			})
		}
		if errDecl.Status == 0 {
			errDecl.Status = policy.Status
		}
		if errDecl.Posture == "" {
			errDecl.Posture = policy.Posture
		}
		if standard && !errDecl.Posture.IsStandard() {
			diags = append(diags, ir.Diagnostic{
				Code:     "FDN0221",
				Severity: ir.SeverityError,
				Message:  fmt.Sprintf("invalid error posture %q in error %q", errDecl.Posture, errDecl.Name),
				Span:     errDecl.Span,
			})
		}
		if errDecl.Status != 0 && (errDecl.Status < platform.MinHTTPStatus || errDecl.Status > platform.MaxHTTPStatus) {
			diags = append(diags, ir.Diagnostic{
				Code:     "FDN0220",
				Severity: ir.SeverityError,
				Message: fmt.Sprintf("error status %d must be between %d and %d in error %q",
					errDecl.Status, platform.MinHTTPStatus, platform.MaxHTTPStatus, errDecl.Name),
				Span: errDecl.Span,
			})
		}
		registerUniqueDecl(sym.errors, "error", errDecl.Name, *errDecl, errDecl.Span, &diags)
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
		for i := range fields {
			f := &fields[i]
			kind, ok := sym.classifyType(f.Type)
			if !ok {
				diags = append(diags, ir.Diagnostic{
					Code:     "FDN0218",
					Severity: ir.SeverityError,
					Message:  fmt.Sprintf("cannot classify type %q for field %q", f.Type, f.Name),
					Span:     f.Span,
				})
				continue
			}
			f.Kind = kind
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

	checkNoVersionTrait := func(constructKind, constructName string, fields []ir.FieldDecl) {
		for _, f := range fields {
			for _, tr := range f.Traits {
				if tr.Name == "version" {
					diags = append(diags, ir.Diagnostic{
						Code:     "FDN0215",
						Severity: ir.SeverityError,
						Message:  fmt.Sprintf("@version trait is only valid on entity fields, found on %s %q field %q", constructKind, constructName, f.Name),
						Span:     tr.Span,
					})
				}
			}
		}
	}

	if len(sem.Actors) > 1 {
		for i := 1; i < len(sem.Actors); i++ {
			diags = append(diags, ir.Diagnostic{
				Code:     "FDN0104",
				Severity: ir.SeverityError,
				Message:  "duplicate actor declaration across blueprint files; only one actor is permitted",
				Span:     sem.Actors[i].Span,
			})
		}
	}

	if sem.Actor != nil {
		validateFields(sem.Actor.Fields)
		checkNoVersionTrait("actor", sem.Actor.Name, sem.Actor.Fields)
	}
	for _, v := range sem.Values {
		validateFields(v.Fields)
		checkNoVersionTrait("value", v.Name, v.Fields)
	}
	tablesSeen := make(map[string]string)
	for _, e := range sem.Entities {
		validateFields(e.Fields)
		for _, tr := range e.Traits {
			if tr.Name == "table" {
				tblName := strings.Trim(tr.Args["_"], `"'`)
				if tblName == "" {
					tblName = strings.Trim(tr.Args["name"], `"'`)
				}
				if tblName == "" {
					diags = append(diags, ir.Diagnostic{
						Code:     "FDN0216",
						Severity: ir.SeverityError,
						Message:  fmt.Sprintf("entity %q has @table trait with empty table name", e.Name),
						Span:     tr.Span,
					})
				}
			}
		}
		physTable := arch.EntityTableName(e)
		if prevEnt, exists := tablesSeen[physTable]; exists {
			diags = append(diags, ir.Diagnostic{
				Code:     "FDN0216",
				Severity: ir.SeverityError,
				Message:  fmt.Sprintf("duplicate physical table name %q declared on entity %q (conflicts with entity %q)", physTable, e.Name, prevEnt),
				Span:     e.Span,
			})
		} else {
			tablesSeen[physTable] = e.Name
		}
		versionCount := 0
		for _, f := range e.Fields {
			for _, tr := range f.Traits {
				if tr.Name == "version" {
					versionCount++
					if f.Type != "int" {
						diags = append(diags, ir.Diagnostic{
							Code:     "FDN0205",
							Severity: ir.SeverityError,
							Message:  fmt.Sprintf("field %q with @version trait must be of type int, got %s", f.Name, f.Type),
							Span:     f.Span,
						})
					}
					if versionCount > 1 {
						diags = append(diags, ir.Diagnostic{
							Code:     "FDN0215",
							Severity: ir.SeverityError,
							Message:  fmt.Sprintf("entity %q declares multiple @version fields; at most one is allowed", e.Name),
							Span:     tr.Span,
						})
					}
				}
			}
		}
	}
	for _, ev := range sem.Events {
		validateFields(ev.Fields)
		checkNoVersionTrait("event", ev.Name, ev.Fields)
	}
	for si := range sem.Slots {
		sl := &sem.Slots[si]
		for mi := range sl.Methods {
			m := &sl.Methods[mi]
			for pi := range m.Params {
				p := &m.Params[pi]
				kind, ok := sym.classifyType(p.Type)
				if !ok {
					diags = append(diags, ir.Diagnostic{
						Code:     "FDN0202",
						Severity: ir.SeverityError,
						Message:  fmt.Sprintf("unknown parameter type %q in slot %s.%s", p.Type, sl.Name, m.Name),
						Span:     p.Span,
					})
					continue
				}
				p.Kind = kind
			}
			if m.ReturnType != "" {
				kind, ok := sym.classifyType(m.ReturnType)
				if !ok {
					diags = append(diags, ir.Diagnostic{
						Code:     "FDN0202",
						Severity: ir.SeverityError,
						Message:  fmt.Sprintf("unknown return type %q in slot %s.%s", m.ReturnType, sl.Name, m.Name),
						Span:     m.Span,
					})
				} else {
					m.ReturnKind = kind
				}
			}
			for _, th := range m.Throws {
				validateErrorRef(th, fmt.Sprintf("slot %s.%s throws", sl.Name, m.Name), m.Span, sym, &diags)
			}
		}
	}

	for i := range sem.Policies {
		pol := &sem.Policies[i]
		pScope := make(map[string]string)
		for pi := range pol.Params {
			param := &pol.Params[pi]
			kind, ok := sym.classifyType(param.Type)
			if !ok {
				diags = append(diags, ir.Diagnostic{
					Code:     "FDN0202",
					Severity: ir.SeverityError,
					Message:  fmt.Sprintf("unknown parameter type %q in policy %q", param.Type, pol.Name),
					Span:     param.Span,
				})
			} else {
				param.Kind = kind
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

	for _, a := range sem.Actions {
		for _, tr := range a.Traits {
			if tr.Name == "event_handler" {
				evName := tr.Args["_"]
				if evName == "" {
					evName = tr.Args["event"]
				}
				if evName == "" {
					diags = append(diags, ir.Diagnostic{
						Code:     "FDN0203",
						Severity: ir.SeverityError,
						Message:  fmt.Sprintf("@event_handler on action %q requires an event name", a.Name),
						Span:     tr.Span,
					})
					continue
				}
				if _, ok := sym.events[evName]; !ok {
					diags = append(diags, ir.Diagnostic{
						Code:     "FDN0203",
						Severity: ir.SeverityError,
						Message:  fmt.Sprintf("@event_handler on action %q subscribes to undeclared event %q", a.Name, evName),
						Span:     tr.Span,
					})
					continue
				}
				hasParam := false
				for _, p := range a.Params {
					if p.Type == evName {
						hasParam = true
						break
					}
				}
				if !hasParam {
					diags = append(diags, ir.Diagnostic{
						Code:     "FDN0204",
						Severity: ir.SeverityError,
						Message:  fmt.Sprintf("@event_handler action %q must accept a parameter of event type %q", a.Name, evName),
						Span:     a.Span,
					})
				}
			}
		}
	}

	return diags
}
