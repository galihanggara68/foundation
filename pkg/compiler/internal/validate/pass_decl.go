package validate

import (
	"fmt"
	"strings"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/arch"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/diagnostics"
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
	for i := range sem.Entities {
		e := &sem.Entities[i]
		if e.TableName == "" {
			e.TableName = arch.EntityTableName(*e)
		}
		registerUniqueDecl(sym.entities, "entity", e.Name, *e, e.Span, &diags)
	}
	for i := range sem.Errors {
		errDecl := &sem.Errors[i]
		if kind, reserved := platform.ReservedErrorName(errDecl.Name); reserved {
			diags = append(diags, diagnostics.New(
				"FDN0219",
				errDecl.Span,
				errDecl.Name,
				kind,
			))
		}
		policy, standard := platform.ErrorPolicyFor(errDecl.Kind)
		if !standard {
			diags = append(diags, diagnostics.New(
				"FDN0201",
				errDecl.Span,
				errDecl.Kind,
				errDecl.Name,
			))
		}
		if errDecl.Status == 0 {
			errDecl.Status = policy.Status
		}
		if errDecl.Posture == "" {
			errDecl.Posture = policy.Posture
		}
		if standard && !errDecl.Posture.IsStandard() {
			diags = append(diags, diagnostics.New(
				"FDN0221",
				errDecl.Span,
				errDecl.Posture,
				errDecl.Name,
			))
		}
		if errDecl.Status != 0 && (errDecl.Status < platform.MinHTTPStatus || errDecl.Status > platform.MaxHTTPStatus) {
			diags = append(diags, diagnostics.New(
				"FDN0220",
				errDecl.Span,
				errDecl.Status,
				platform.MinHTTPStatus,
				platform.MaxHTTPStatus,
				errDecl.Name,
			))
		}
		// The platform already ships this error: keep the seeded entry, so the standard
		// error shadows the declaration exactly as FDN0219 warns. Registering it would
		// report the declaration against the symbol it collides with, making a
		// warned-but-legal name fatal.
		if !platform.IsBuiltinErrorName(errDecl.Name) {
			registerUniqueDecl(sym.errors, "error", errDecl.Name, *errDecl, errDecl.Span, &diags)
		}
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
				diags = append(diags, diagnostics.New("FDN0218", f.Span, f.Type, f.Name))
				continue
			}
			f.Kind = kind
			if strings.HasPrefix(f.DefaultVal, ".") {
				variant := strings.TrimPrefix(f.DefaultVal, ".")
				if !sym.enumHasVariant(f.Type, variant) {
					diags = append(diags, diagnostics.NewWithDetail(
						"FDN0205",
						f.Span,
						fmt.Sprintf("invalid default enum variant %s for field %q of type %s", f.DefaultVal, f.Name, f.Type),
					))
				}
			}
		}
	}

	checkNoVersionTrait := func(constructKind, constructName string, fields []ir.FieldDecl) {
		for _, f := range fields {
			for _, tr := range f.Traits {
				if tr.Name == "version" {
					diags = append(diags, diagnostics.NewWithDetail(
						"FDN0215",
						tr.Span,
						fmt.Sprintf("@version trait is only valid on entity fields, found on %s %q field %q", constructKind, constructName, f.Name),
					))
				}
			}
		}
	}

	if len(sem.Actors) > 1 {
		for i := 1; i < len(sem.Actors); i++ {
			diags = append(diags, diagnostics.NewWithDetail(
				"FDN0104",
				sem.Actors[i].Span,
				"duplicate actor declaration across blueprint files; only one actor is permitted",
			))
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
	for i := range sem.Entities {
		e := &sem.Entities[i]
		validateFields(e.Fields)
		for _, tr := range e.Traits {
			if tr.Name == "table" {
				tblName := strings.Trim(tr.Args["_"], `"'`)
				if tblName == "" {
					tblName = strings.Trim(tr.Args["name"], `"'`)
				}
				if tblName == "" {
					tblName = strings.Trim(tr.Args["_0"], `"'`)
				}
				if tblName == "" {
					diags = append(diags, diagnostics.NewWithDetail(
						"FDN0216",
						tr.Span,
						fmt.Sprintf("entity %q has @table trait with empty table name", e.Name),
					))
				}
			}
		}
		physTable := arch.EntityTableName(*e)
		e.TableName = physTable
		sym.entities[e.Name] = *e
		if prevEnt, exists := tablesSeen[physTable]; exists {
			diags = append(diags, diagnostics.NewWithDetail(
				"FDN0216",
				e.Span,
				fmt.Sprintf("duplicate physical table name %q declared on entity %q (conflicts with entity %q)", physTable, e.Name, prevEnt),
			))
		} else {
			tablesSeen[physTable] = e.Name
		}
		versionCount := 0
		for _, f := range e.Fields {
			for _, tr := range f.Traits {
				if tr.Name == "version" {
					versionCount++
					if f.Type != "int" {
						diags = append(diags, diagnostics.NewWithDetail(
							"FDN0205",
							f.Span,
							fmt.Sprintf("field %q with @version trait must be of type int, got %s", f.Name, f.Type),
						))
					}
					if versionCount > 1 {
						diags = append(diags, diagnostics.NewWithDetail(
							"FDN0215",
							tr.Span,
							fmt.Sprintf("entity %q declares multiple @version fields; at most one is allowed", e.Name),
						))
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
					diags = append(diags, diagnostics.NewWithDetail(
						"FDN0202",
						p.Span,
						fmt.Sprintf("unknown parameter type %q in slot %s.%s", p.Type, sl.Name, m.Name),
					))
					continue
				}
				p.Kind = kind
			}
			if m.ReturnType != "" {
				kind, ok := sym.classifyType(m.ReturnType)
				if !ok {
					diags = append(diags, diagnostics.NewWithDetail(
						"FDN0202",
						m.Span,
						fmt.Sprintf("unknown return type %q in slot %s.%s", m.ReturnType, sl.Name, m.Name),
					))
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
				diags = append(diags, diagnostics.NewWithDetail(
					"FDN0202",
					param.Span,
					fmt.Sprintf("unknown parameter type %q in policy %q", param.Type, pol.Name),
				))
			} else {
				param.Kind = kind
			}
			pScope[param.Name] = param.Type
		}
		bt, ok := inferExprType(&pol.Body, pScope, sym, &diags)
		if ok && bt != "bool" {
			diags = append(diags, diagnostics.NewWithDetail(
				"FDN0205",
				pol.Body.Span,
				fmt.Sprintf("policy %q body must evaluate to bool, got %s", pol.Name, bt),
			))
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
					diags = append(diags, diagnostics.NewWithDetail(
						"FDN0231",
						tr.Span,
						fmt.Sprintf("@event_handler on action %q requires an event name", a.Name),
					))
					continue
				}
				if _, ok := sym.events[evName]; !ok {
					diags = append(diags, diagnostics.New(
						"FDN0231",
						tr.Span,
						a.Name,
						evName,
					))
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
					diags = append(diags, diagnostics.NewWithDetail(
						"FDN0204",
						a.Span,
						fmt.Sprintf("@event_handler action %q must accept a parameter of event type %q", a.Name, evName),
					))
				}
			}
		}
	}

	return diags
}
