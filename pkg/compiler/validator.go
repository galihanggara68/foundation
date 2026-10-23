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

// symbolTable indexes declared domain constructs for fast, DRY lookup across validation passes.
type symbolTable struct {
	entities map[string]EntityDecl
	values   map[string]ValueDecl
	errors   map[string]ErrorDecl
	actions  map[string]ActionDecl
}

func newSymbolTable() *symbolTable {
	return &symbolTable{
		entities: make(map[string]EntityDecl),
		values:   make(map[string]ValueDecl),
		errors:   make(map[string]ErrorDecl),
		actions:  make(map[string]ActionDecl),
	}
}

func (s *symbolTable) isKnownType(t string) bool {
	if scalarTypes[t] {
		return true
	}
	if _, ok := s.entities[t]; ok {
		return true
	}
	if _, ok := s.values[t]; ok {
		return true
	}
	return false
}

func (s *symbolTable) fieldsOf(typeName string) map[string]FieldDecl {
	m := make(map[string]FieldDecl)
	if e, ok := s.entities[typeName]; ok {
		for _, f := range e.Fields {
			m[f.Name] = f
		}
	}
	if v, ok := s.values[typeName]; ok {
		for _, f := range v.Fields {
			m[f.Name] = f
		}
	}
	return m
}

func isAssignableType(expected, actual string) bool {
	return expected == actual || (expected == "uuid" && actual == "string")
}

func validateIR(ir *SemanticIR) []Diagnostic {
	sym := newSymbolTable()
	reg := defaultRegistry()
	var diags []Diagnostic
	for _, pass := range reg.passes {
		diags = append(diags, pass.Validate(ir, sym)...)
	}
	return diags
}

// declarationPass populates the symbolTable and validates top-level declarations, error kinds, and field types.
type declarationPass struct{}

func (declarationPass) Validate(ir *SemanticIR, sym *symbolTable) []Diagnostic {
	var diags []Diagnostic

	for _, v := range ir.Values {
		if _, exists := sym.values[v.Name]; exists {
			diags = append(diags, Diagnostic{
				Code:     "FDN0200",
				Severity: SeverityError,
				Message:  fmt.Sprintf("duplicate value declaration %q", v.Name),
				Span:     v.Span,
			})
		}
		sym.values[v.Name] = v
	}

	for _, e := range ir.Entities {
		if _, exists := sym.entities[e.Name]; exists {
			diags = append(diags, Diagnostic{
				Code:     "FDN0200",
				Severity: SeverityError,
				Message:  fmt.Sprintf("duplicate entity declaration %q", e.Name),
				Span:     e.Span,
			})
		}
		sym.entities[e.Name] = e
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
		sym.errors[errDecl.Name] = errDecl
	}

	for _, a := range ir.Actions {
		sym.actions[a.Name] = a
	}

	validateFieldTypes := func(fields []FieldDecl) {
		for _, f := range fields {
			if !sym.isKnownType(f.Type) {
				diags = append(diags, Diagnostic{
					Code:     "FDN0202",
					Severity: SeverityError,
					Message:  fmt.Sprintf("unknown type %q for field %q", f.Type, f.Name),
					Span:     f.Span,
				})
			}
		}
	}

	for _, v := range ir.Values {
		validateFieldTypes(v.Fields)
	}
	for _, e := range ir.Entities {
		validateFieldTypes(e.Fields)
	}

	return diags
}

// actionTypeCheckPass validates Action parameters, return types, and Action Pipeline step types.
type actionTypeCheckPass struct{}

func (actionTypeCheckPass) Validate(ir *SemanticIR, sym *symbolTable) []Diagnostic {
	var diags []Diagnostic

	for _, a := range ir.Actions {
		scope := make(map[string]string)
		for _, p := range a.Params {
			if !sym.isKnownType(p.Type) {
				diags = append(diags, Diagnostic{
					Code:     "FDN0202",
					Severity: SeverityError,
					Message:  fmt.Sprintf("unknown parameter type %q", p.Type),
					Span:     p.Span,
				})
			}
			scope[p.Name] = p.Type
		}
		if a.ReturnType != "" && !sym.isKnownType(a.ReturnType) {
			diags = append(diags, Diagnostic{
				Code:     "FDN0202",
				Severity: SeverityError,
				Message:  fmt.Sprintf("unknown return type %q", a.ReturnType),
				Span:     a.Span,
			})
		}

		for _, s := range a.Steps {
			validateActionStep(a, s, scope, sym, &diags)
		}
	}

	return diags
}

func validateActionStep(a ActionDecl, s StepDecl, scope map[string]string, sym *symbolTable, diags *[]Diagnostic) {
	switch s.Kind {
	case StepQuery:
		ent, ok := sym.entities[s.EntityName]
		if !ok {
			*diags = append(*diags, Diagnostic{
				Code:     "FDN0203",
				Severity: SeverityError,
				Message:  fmt.Sprintf("unknown entity %q in query", s.EntityName),
				Span:     s.Span,
			})
			return
		}
		fields := sym.fieldsOf(ent.Name)
		wf, fOk := fields[s.WhereField]
		if !fOk {
			*diags = append(*diags, Diagnostic{
				Code:     "FDN0204",
				Severity: SeverityError,
				Message:  fmt.Sprintf("entity %q has no field %q", ent.Name, s.WhereField),
				Span:     s.Span,
			})
		} else if s.WhereValue != nil {
			valType, ok := inferExprType(*s.WhereValue, scope, sym, diags)
			if ok && !isAssignableType(wf.Type, valType) {
				*diags = append(*diags, Diagnostic{
					Code:     "FDN0205",
					Severity: SeverityError,
					Message:  fmt.Sprintf("type mismatch in query where clause: field %q is %s, got %s", s.WhereField, wf.Type, valType),
					Span:     s.WhereValue.Span,
				})
			}
		}
		if s.ElseError == "" {
			*diags = append(*diags, Diagnostic{
				Code:     "FDN0206",
				Severity: SeverityError,
				Message:  "query one requires an 'else <Error>' clause",
				Span:     s.Span,
			})
		} else if _, errOk := sym.errors[s.ElseError]; !errOk {
			*diags = append(*diags, Diagnostic{
				Code:     "FDN0206",
				Severity: SeverityError,
				Message:  fmt.Sprintf("undeclared error %q", s.ElseError),
				Span:     s.Span,
			})
		}
		scope[s.VarName] = ent.Name

	case StepCreate:
		ent, ok := sym.entities[s.EntityName]
		if !ok {
			*diags = append(*diags, Diagnostic{
				Code:     "FDN0203",
				Severity: SeverityError,
				Message:  fmt.Sprintf("unknown entity %q in create", s.EntityName),
				Span:     s.Span,
			})
			return
		}
		validateFieldInitializers("entity", ent.Name, s.Fields, scope, sym, diags)
		scope[s.VarName] = ent.Name

	case StepLet:
		if s.Expr != nil {
			if t, ok := inferExprType(*s.Expr, scope, sym, diags); ok {
				scope[s.VarName] = t
			}
		}

	case StepReturn:
		if s.Expr != nil {
			t, ok := inferExprType(*s.Expr, scope, sym, diags)
			if ok && a.ReturnType != "" && !isAssignableType(a.ReturnType, t) {
				*diags = append(*diags, Diagnostic{
					Code:     "FDN0205",
					Severity: SeverityError,
					Message:  fmt.Sprintf("return type mismatch: action %q returns %s, got %s", a.Name, a.ReturnType, t),
					Span:     s.Expr.Span,
				})
			}
		}
	}
}

// scenarioTypeCheckPass validates Scenario given seeds, when action invocations, and expect assertions.
type scenarioTypeCheckPass struct{}

func (scenarioTypeCheckPass) Validate(ir *SemanticIR, sym *symbolTable) []Diagnostic {
	var diags []Diagnostic

	for _, sc := range ir.Scenarios {
		for _, g := range sc.Givens {
			if g.EntityName == "actor" {
				continue
			}
			ent, ok := sym.entities[g.EntityName]
			if !ok {
				diags = append(diags, Diagnostic{
					Code:     "FDN0203",
					Severity: SeverityError,
					Message:  fmt.Sprintf("unknown entity %q in scenario given", g.EntityName),
					Span:     g.Span,
				})
				continue
			}
			validateFieldInitializers("entity", ent.Name, g.Fields, nil, sym, &diags)
		}

		act, ok := sym.actions[sc.WhenAction]
		if !ok {
			diags = append(diags, Diagnostic{
				Code:     "FDN0207",
				Severity: SeverityError,
				Message:  fmt.Sprintf("unknown action %q in scenario when", sc.WhenAction),
				Span:     sc.Span,
			})
			continue
		}

		if len(sc.WhenArgs) != len(act.Params) {
			diags = append(diags, Diagnostic{
				Code:     "FDN0205",
				Severity: SeverityError,
				Message:  fmt.Sprintf("action %q expects %d arguments, got %d", act.Name, len(act.Params), len(sc.WhenArgs)),
				Span:     sc.Span,
			})
		} else {
			for i, arg := range sc.WhenArgs {
				argType, ok := inferExprType(arg, nil, sym, &diags)
				paramType := act.Params[i].Type
				if ok && !isAssignableType(paramType, argType) {
					diags = append(diags, Diagnostic{
						Code:     "FDN0205",
						Severity: SeverityError,
						Message:  fmt.Sprintf("argument %d to %q has type %s, expected %s", i+1, act.Name, argType, paramType),
						Span:     arg.Span,
					})
				}
			}
		}

		expectScope := map[string]string{
			"result": act.ReturnType,
		}
		for _, ex := range sc.Expects {
			if ex.ErrorRaised != "" {
				if _, errOk := sym.errors[ex.ErrorRaised]; !errOk {
					diags = append(diags, Diagnostic{
						Code:     "FDN0206",
						Severity: SeverityError,
						Message:  fmt.Sprintf("undeclared error %q in scenario expect", ex.ErrorRaised),
						Span:     ex.Span,
					})
				}
				continue
			}
			lt, lok := inferExprType(ex.Left, expectScope, sym, &diags)
			rt, rok := inferExprType(ex.Right, expectScope, sym, &diags)
			if lok && rok && !isAssignableType(lt, rt) {
				diags = append(diags, Diagnostic{
					Code:     "FDN0205",
					Severity: SeverityError,
					Message:  fmt.Sprintf("scenario expect type mismatch: %s vs %s", lt, rt),
					Span:     ex.Span,
				})
			}
		}
	}

	return diags
}

func validateFieldInitializers(kindLabel, typeName string, inits []FieldInitDecl, scope map[string]string, sym *symbolTable, diags *[]Diagnostic) {
	fields := sym.fieldsOf(typeName)
	for _, fi := range inits {
		f, fOk := fields[fi.Name]
		if !fOk {
			*diags = append(*diags, Diagnostic{
				Code:     "FDN0204",
				Severity: SeverityError,
				Message:  fmt.Sprintf("%s %q has no field %q", kindLabel, typeName, fi.Name),
				Span:     fi.Span,
			})
			continue
		}
		valType, ok := inferExprType(fi.Value, scope, sym, diags)
		if ok && !isAssignableType(f.Type, valType) {
			*diags = append(*diags, Diagnostic{
				Code:     "FDN0205",
				Severity: SeverityError,
				Message:  fmt.Sprintf("type mismatch for field %q: expected %s, got %s", fi.Name, f.Type, valType),
				Span:     fi.Value.Span,
			})
		}
	}
}

func inferExprType(e Expr, scope map[string]string, sym *symbolTable, diags *[]Diagnostic) (string, bool) {
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
		fields := sym.fieldsOf(targetType)
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
		lt, lok := inferExprType(*e.Left, scope, sym, diags)
		rt, rok := inferExprType(*e.Right, scope, sym, diags)
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
		fields := sym.fieldsOf(e.TypeName)
		if len(fields) == 0 {
			*diags = append(*diags, Diagnostic{
				Code:     "FDN0202",
				Severity: SeverityError,
				Message:  fmt.Sprintf("unknown type %q in struct construction", e.TypeName),
				Span:     e.Span,
			})
			return "", false
		}
		validateFieldInitializers("type", e.TypeName, e.Fields, scope, sym, diags)
		return e.TypeName, true
	default:
		return "", false
	}
}
