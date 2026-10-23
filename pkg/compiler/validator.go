package compiler

import (
	"fmt"
)

var scalarTypes = map[string]bool{
	"uuid":      true,
	"string":    true,
	"int":       true,
	"decimal":   true,
	"bool":      true,
	"timestamp": true,
}

var validErrorKinds = map[string]bool{
	"not_found":    true,
	"validation":   true,
	"conflict":     true,
	"unauthorized": true,
	"forbidden":    true,
	"internal":     true,
}

func validateIR(ir *SemanticIR) []Diagnostic {
	var diags []Diagnostic

	entities := make(map[string]EntityDecl)
	values := make(map[string]ValueDecl)
	errorsMap := make(map[string]ErrorDecl)
	actions := make(map[string]ActionDecl)

	for _, v := range ir.Values {
		if _, exists := values[v.Name]; exists {
			diags = append(diags, Diagnostic{
				Code:     "FDN0200",
				Severity: SeverityError,
				Message:  fmt.Sprintf("duplicate value declaration %q", v.Name),
				Span:     v.Span,
			})
		}
		values[v.Name] = v
	}

	for _, e := range ir.Entities {
		if _, exists := entities[e.Name]; exists {
			diags = append(diags, Diagnostic{
				Code:     "FDN0200",
				Severity: SeverityError,
				Message:  fmt.Sprintf("duplicate entity declaration %q", e.Name),
				Span:     e.Span,
			})
		}
		entities[e.Name] = e
	}

	for _, errDecl := range ir.Errors {
		if !validErrorKinds[errDecl.Kind] {
			diags = append(diags, Diagnostic{
				Code:     "FDN0201",
				Severity: SeverityError,
				Message:  fmt.Sprintf("invalid error kind %q in error %q", errDecl.Kind, errDecl.Name),
				Span:     errDecl.Span,
			})
		}
		errorsMap[errDecl.Name] = errDecl
	}

	isKnownType := func(t string) bool {
		if scalarTypes[t] {
			return true
		}
		if _, ok := entities[t]; ok {
			return true
		}
		if _, ok := values[t]; ok {
			return true
		}
		return false
	}

	for _, v := range ir.Values {
		for _, f := range v.Fields {
			if !isKnownType(f.Type) {
				diags = append(diags, Diagnostic{
					Code:     "FDN0202",
					Severity: SeverityError,
					Message:  fmt.Sprintf("unknown type %q for field %q", f.Type, f.Name),
					Span:     f.Span,
				})
			}
		}
	}

	for _, e := range ir.Entities {
		for _, f := range e.Fields {
			if !isKnownType(f.Type) {
				diags = append(diags, Diagnostic{
					Code:     "FDN0202",
					Severity: SeverityError,
					Message:  fmt.Sprintf("unknown type %q for field %q", f.Type, f.Name),
					Span:     f.Span,
				})
			}
		}
	}

	fieldMap := func(typeName string) map[string]FieldDecl {
		m := make(map[string]FieldDecl)
		if e, ok := entities[typeName]; ok {
			for _, f := range e.Fields {
				m[f.Name] = f
			}
		}
		if v, ok := values[typeName]; ok {
			for _, f := range v.Fields {
				m[f.Name] = f
			}
		}
		return m
	}

	for _, a := range ir.Actions {
		actions[a.Name] = a
		scope := make(map[string]string)
		for _, p := range a.Params {
			if !isKnownType(p.Type) {
				diags = append(diags, Diagnostic{
					Code:     "FDN0202",
					Severity: SeverityError,
					Message:  fmt.Sprintf("unknown parameter type %q", p.Type),
					Span:     p.Span,
				})
			}
			scope[p.Name] = p.Type
		}
		if a.ReturnType != "" && !isKnownType(a.ReturnType) {
			diags = append(diags, Diagnostic{
				Code:     "FDN0202",
				Severity: SeverityError,
				Message:  fmt.Sprintf("unknown return type %q", a.ReturnType),
				Span:     a.Span,
			})
		}

		for _, s := range a.Steps {
			switch s.Kind {
			case StepQuery:
				ent, ok := entities[s.EntityName]
				if !ok {
					diags = append(diags, Diagnostic{
						Code:     "FDN0203",
						Severity: SeverityError,
						Message:  fmt.Sprintf("unknown entity %q in query", s.EntityName),
						Span:     s.Span,
					})
					continue
				}
				fields := fieldMap(ent.Name)
				wf, fOk := fields[s.WhereField]
				if !fOk {
					diags = append(diags, Diagnostic{
						Code:     "FDN0204",
						Severity: SeverityError,
						Message:  fmt.Sprintf("entity %q has no field %q", ent.Name, s.WhereField),
						Span:     s.Span,
					})
				} else if s.WhereValue != nil {
					valType, ok := inferExprType(*s.WhereValue, scope, fieldMap, &diags)
					if ok && valType != wf.Type {
						diags = append(diags, Diagnostic{
							Code:     "FDN0205",
							Severity: SeverityError,
							Message:  fmt.Sprintf("type mismatch in query where clause: field %q is %s, got %s", s.WhereField, wf.Type, valType),
							Span:     s.WhereValue.Span,
						})
					}
				}
				if s.ElseError != "" {
					if _, errOk := errorsMap[s.ElseError]; !errOk {
						diags = append(diags, Diagnostic{
							Code:     "FDN0206",
							Severity: SeverityError,
							Message:  fmt.Sprintf("undeclared error %q", s.ElseError),
							Span:     s.Span,
						})
					}
				}
				scope[s.VarName] = ent.Name

			case StepCreate:
				ent, ok := entities[s.EntityName]
				if !ok {
					diags = append(diags, Diagnostic{
						Code:     "FDN0203",
						Severity: SeverityError,
						Message:  fmt.Sprintf("unknown entity %q in create", s.EntityName),
						Span:     s.Span,
					})
					continue
				}
				fields := fieldMap(ent.Name)
				for _, fi := range s.Fields {
					ef, fOk := fields[fi.Name]
					if !fOk {
						diags = append(diags, Diagnostic{
							Code:     "FDN0204",
							Severity: SeverityError,
							Message:  fmt.Sprintf("entity %q has no field %q", ent.Name, fi.Name),
							Span:     fi.Span,
						})
						continue
					}
					valType, ok := inferExprType(fi.Value, scope, fieldMap, &diags)
					if ok && valType != ef.Type {
						diags = append(diags, Diagnostic{
							Code:     "FDN0205",
							Severity: SeverityError,
							Message:  fmt.Sprintf("type mismatch for field %q: expected %s, got %s", fi.Name, ef.Type, valType),
							Span:     fi.Value.Span,
						})
					}
				}
				scope[s.VarName] = ent.Name

			case StepLet:
				if s.Expr != nil {
					t, ok := inferExprType(*s.Expr, scope, fieldMap, &diags)
					if ok {
						scope[s.VarName] = t
					}
				}

			case StepReturn:
				if s.Expr != nil {
					t, ok := inferExprType(*s.Expr, scope, fieldMap, &diags)
					if ok && a.ReturnType != "" && t != a.ReturnType {
						diags = append(diags, Diagnostic{
							Code:     "FDN0205",
							Severity: SeverityError,
							Message:  fmt.Sprintf("return type mismatch: action %q returns %s, got %s", a.Name, a.ReturnType, t),
							Span:     s.Expr.Span,
						})
					}
				}
			}
		}
	}

	for _, sc := range ir.Scenarios {
		for _, g := range sc.Givens {
			if g.EntityName == "actor" {
				continue
			}
			if _, ok := entities[g.EntityName]; !ok {
				diags = append(diags, Diagnostic{
					Code:     "FDN0203",
					Severity: SeverityError,
					Message:  fmt.Sprintf("unknown entity %q in scenario given", g.EntityName),
					Span:     g.Span,
				})
			}
		}
		if _, ok := actions[sc.WhenAction]; !ok {
			diags = append(diags, Diagnostic{
				Code:     "FDN0207",
				Severity: SeverityError,
				Message:  fmt.Sprintf("unknown action %q in scenario when", sc.WhenAction),
				Span:     sc.Span,
			})
		}
	}

	return diags
}

func inferExprType(e Expr, scope map[string]string, fieldMap func(string) map[string]FieldDecl, diags *[]Diagnostic) (string, bool) {
	switch e.Kind {
	case ExprString:
		return "string", true
	case ExprInt:
		return "int", true
	case ExprFloat:
		return "decimal", true
	case ExprBool:
		return "bool", true
	case ExprIdent:
		t, ok := scope[e.Value]
		if !ok {
			*diags = append(*diags, Diagnostic{
				Code:     "FDN0208",
				Severity: SeverityError,
				Message:  fmt.Sprintf("undeclared identifier %q", e.Value),
				Span:     e.Span,
			})
			return "", false
		}
		return t, true
	case ExprField:
		targetType, ok := scope[e.Target]
		if !ok {
			*diags = append(*diags, Diagnostic{
				Code:     "FDN0208",
				Severity: SeverityError,
				Message:  fmt.Sprintf("undeclared identifier %q", e.Target),
				Span:     e.Span,
			})
			return "", false
		}
		fields := fieldMap(targetType)
		f, fOk := fields[e.Field]
		if !fOk {
			*diags = append(*diags, Diagnostic{
				Code:     "FDN0204",
				Severity: SeverityError,
				Message:  fmt.Sprintf("type %q has no field %q", targetType, e.Field),
				Span:     e.Span,
			})
			return "", false
		}
		return f.Type, true
	case ExprBinary:
		if e.Left == nil || e.Right == nil {
			return "", false
		}
		lt, lok := inferExprType(*e.Left, scope, fieldMap, diags)
		rt, rok := inferExprType(*e.Right, scope, fieldMap, diags)
		if !lok || !rok {
			return "", false
		}
		if e.Op == "==" || e.Op == "!=" || e.Op == ">=" || e.Op == "<=" || e.Op == ">" || e.Op == "<" || e.Op == "&&" || e.Op == "||" {
			return "bool", true
		}
		if lt == rt {
			return lt, true
		}
		if (lt == "decimal" && rt == "int") || (lt == "int" && rt == "decimal") {
			return "decimal", true
		}
		*diags = append(*diags, Diagnostic{
			Code:     "FDN0205",
			Severity: SeverityError,
			Message:  fmt.Sprintf("incompatible operand types %s and %s for %s", lt, rt, e.Op),
			Span:     e.Span,
		})
		return "", false
	case ExprConstruct:
		return e.TypeName, true
	default:
		return "", false
	}
}
