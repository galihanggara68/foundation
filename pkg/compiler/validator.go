package compiler

import (
	"fmt"
	"sort"
	"strings"
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
	enums       map[string]EnumDecl
	entities    map[string]EntityDecl
	values      map[string]ValueDecl
	errors      map[string]ErrorDecl
	events      map[string]EventDecl
	policies    map[string]PolicyDecl
	actions     map[string]ActionDecl
	actorFields map[string]FieldDecl
}

func newSymbolTable() *symbolTable {
	return &symbolTable{
		enums:    make(map[string]EnumDecl),
		entities: make(map[string]EntityDecl),
		values:   make(map[string]ValueDecl),
		errors:   make(map[string]ErrorDecl),
		events:   make(map[string]EventDecl),
		policies: make(map[string]PolicyDecl),
		actions:  make(map[string]ActionDecl),
		actorFields: map[string]FieldDecl{
			"id":          {Name: "id", Type: "uuid"},
			"role":        {Name: "role", Type: "string"},
			"is_verified": {Name: "is_verified", Type: "bool"},
			"tenant_id":   {Name: "tenant_id", Type: "uuid"},
		},
	}
}

func (s *symbolTable) isKnownType(t string) bool {
	if strings.HasPrefix(t, "[]") {
		return s.isKnownType(strings.TrimPrefix(t, "[]"))
	}
	if scalarTypes[t] || t == "Actor" {
		return true
	}
	if _, ok := s.enums[t]; ok {
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
		return m
	}
	if v, ok := s.values[typeName]; ok {
		for _, f := range v.Fields {
			m[f.Name] = f
		}
		return m
	}
	if ev, ok := s.events[typeName]; ok {
		for _, f := range ev.Fields {
			m[f.Name] = f
		}
		return m
	}
	if typeName == "Actor" {
		for k, f := range s.actorFields {
			m[k] = f
		}
	}
	return m
}

func (s *symbolTable) enumHasVariant(enumName, variant string) bool {
	en, ok := s.enums[enumName]
	if !ok {
		return false
	}
	for _, v := range en.Variants {
		if v == variant {
			return true
		}
	}
	return false
}

func (s *symbolTable) findEnumByVariant(variant string) (string, bool) {
	var names []string
	for name := range s.enums {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if s.enumHasVariant(name, variant) {
			return name, true
		}
	}
	return "", false
}

func (s *symbolTable) resolveEnumLiteral(e *Expr, expectedType string) {
	if e != nil && e.Kind == ExprEnumLit && s.enumHasVariant(expectedType, e.Value) {
		e.InferredType = expectedType
	}
}

func isAssignableType(expected, actual string) bool {
	return expected == actual || (expected == "uuid" && actual == "string") || (strings.HasPrefix(expected, "[]") && actual == "[]")
}

func isNumericType(t string) bool {
	return t == "int" || t == "decimal"
}

func cloneScope(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func registerUniqueDecl[T any](m map[string]T, kindLabel, name string, item T, span Span, diags *[]Diagnostic) {
	if _, exists := m[name]; exists {
		*diags = append(*diags, Diagnostic{
			Code:     "FDN0200",
			Severity: SeverityError,
			Message:  fmt.Sprintf("duplicate %s declaration %q", kindLabel, name),
			Span:     span,
		})
	}
	m[name] = item
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

// declarationPass populates the symbolTable and validates top-level declarations, error kinds, field types, and policies.
type declarationPass struct{}

func (declarationPass) Validate(ir *SemanticIR, sym *symbolTable) []Diagnostic {
	var diags []Diagnostic

	for _, en := range ir.Enums {
		registerUniqueDecl(sym.enums, "enum", en.Name, en, en.Span, &diags)
	}
	for _, v := range ir.Values {
		registerUniqueDecl(sym.values, "value", v.Name, v, v.Span, &diags)
	}
	for _, e := range ir.Entities {
		registerUniqueDecl(sym.entities, "entity", e.Name, e, e.Span, &diags)
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
		registerUniqueDecl(sym.errors, "error", errDecl.Name, errDecl, errDecl.Span, &diags)
	}
	for _, ev := range ir.Events {
		registerUniqueDecl(sym.events, "event", ev.Name, ev, ev.Span, &diags)
	}
	for _, pol := range ir.Policies {
		registerUniqueDecl(sym.policies, "policy", pol.Name, pol, pol.Span, &diags)
	}
	for _, a := range ir.Actions {
		registerUniqueDecl(sym.actions, "action", a.Name, a, a.Span, &diags)
	}

	validateFields := func(fields []FieldDecl) {
		for _, f := range fields {
			if !sym.isKnownType(f.Type) {
				diags = append(diags, Diagnostic{
					Code:     "FDN0202",
					Severity: SeverityError,
					Message:  fmt.Sprintf("unknown type %q for field %q", f.Type, f.Name),
					Span:     f.Span,
				})
				continue
			}
			if strings.HasPrefix(f.DefaultVal, ".") {
				variant := strings.TrimPrefix(f.DefaultVal, ".")
				if !sym.enumHasVariant(f.Type, variant) {
					diags = append(diags, Diagnostic{
						Code:     "FDN0205",
						Severity: SeverityError,
						Message:  fmt.Sprintf("invalid default enum variant %s for field %q of type %s", f.DefaultVal, f.Name, f.Type),
						Span:     f.Span,
					})
				}
			}
		}
	}

	for _, v := range ir.Values {
		validateFields(v.Fields)
	}
	for _, e := range ir.Entities {
		validateFields(e.Fields)
	}
	for _, ev := range ir.Events {
		validateFields(ev.Fields)
	}

	for i := range ir.Policies {
		pol := &ir.Policies[i]
		pScope := make(map[string]string)
		for _, param := range pol.Params {
			if !sym.isKnownType(param.Type) {
				diags = append(diags, Diagnostic{
					Code:     "FDN0202",
					Severity: SeverityError,
					Message:  fmt.Sprintf("unknown parameter type %q in policy %q", param.Type, pol.Name),
					Span:     param.Span,
				})
			}
			pScope[param.Name] = param.Type
		}
		bt, ok := inferExprType(&pol.Body, pScope, sym, &diags)
		if ok && bt != "bool" {
			diags = append(diags, Diagnostic{
				Code:     "FDN0205",
				Severity: SeverityError,
				Message:  fmt.Sprintf("policy %q body must evaluate to bool, got %s", pol.Name, bt),
				Span:     pol.Body.Span,
			})
		}
		sym.policies[pol.Name] = *pol
	}

	return diags
}

// actionTypeCheckPass validates Action parameters, return types, and Action Pipeline step types.
type actionTypeCheckPass struct{}

func (actionTypeCheckPass) Validate(ir *SemanticIR, sym *symbolTable) []Diagnostic {
	var diags []Diagnostic

	for i := range ir.Actions {
		a := &ir.Actions[i]
		scope := map[string]string{
			"actor": "Actor",
		}
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

		for j := range a.Steps {
			validateActionStep(*a, &a.Steps[j], scope, sym, &diags)
		}
		sym.actions[a.Name] = *a
	}

	return diags
}

func validateActionStep(a ActionDecl, s *StepDecl, scope map[string]string, sym *symbolTable, diags *[]Diagnostic) {
	switch s.Kind {
	case StepRequire:
		validateRequireStep(s, scope, sym, diags)

	case StepGuard:
		if s.Expr != nil {
			condType, ok := inferExprType(s.Expr, scope, sym, diags)
			if ok && condType != "bool" {
				*diags = append(*diags, Diagnostic{
					Code:     "FDN0205",
					Severity: SeverityError,
					Message:  fmt.Sprintf("guard condition must be bool, got %s", condType),
					Span:     s.Expr.Span,
				})
			}
		}
		validateErrorRef(s.ElseError, "guard else", s.Span, sym, diags)

	case StepEmit:
		ev, ok := sym.events[s.EventName]
		if !ok {
			*diags = append(*diags, Diagnostic{
				Code:     "FDN0203",
				Severity: SeverityError,
				Message:  fmt.Sprintf("undeclared event %q in emit", s.EventName),
				Span:     s.Span,
			})
			return
		}
		validateFieldInitializers("event", ev.Name, s.Fields, scope, sym, diags)

	case StepQuery:
		ent, ok := validateEntityWhereClause("query", s.EntityName, s.WhereField, s.WhereValue, s.Span, scope, sym, diags)
		if !ok {
			return
		}
		if s.ElseError == "" {
			*diags = append(*diags, Diagnostic{
				Code:     "FDN0206",
				Severity: SeverityError,
				Message:  "query one requires an 'else <Error>' clause",
				Span:     s.Span,
			})
		} else {
			validateErrorRef(s.ElseError, "query else", s.Span, sym, diags)
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

	case StepUpdate:
		ent, ok := validateEntityWhereClause("update", s.EntityName, s.WhereField, s.WhereValue, s.Span, scope, sym, diags)
		if !ok {
			return
		}
		validateFieldInitializers("entity", ent.Name, s.Fields, scope, sym, diags)

	case StepDelete:
		validateEntityWhereClause("delete", s.EntityName, s.WhereField, s.WhereValue, s.Span, scope, sym, diags)

	case StepIf:
		if s.Expr != nil {
			condType, ok := inferExprType(s.Expr, scope, sym, diags)
			if ok && condType != "bool" {
				*diags = append(*diags, Diagnostic{
					Code:     "FDN0205",
					Severity: SeverityError,
					Message:  fmt.Sprintf("if condition must be bool, got %s", condType),
					Span:     s.Expr.Span,
				})
			}
		}
		thenScope := cloneScope(scope)
		for i := range s.ThenSteps {
			validateActionStep(a, &s.ThenSteps[i], thenScope, sym, diags)
		}
		elseScope := cloneScope(scope)
		for i := range s.ElseSteps {
			validateActionStep(a, &s.ElseSteps[i], elseScope, sym, diags)
		}

	case StepMatch:
		validateMatchStep(a, s, scope, sym, diags)

	case StepFor:
		var elemType string
		if s.Expr != nil {
			collType, ok := inferExprType(s.Expr, scope, sym, diags)
			if ok {
				if !strings.HasPrefix(collType, "[]") {
					*diags = append(*diags, Diagnostic{
						Code:     "FDN0205",
						Severity: SeverityError,
						Message:  fmt.Sprintf("for loop collection must be a list type, got %s", collType),
						Span:     s.Expr.Span,
					})
				} else {
					elemType = strings.TrimPrefix(collType, "[]")
				}
			}
		}
		bodyScope := cloneScope(scope)
		if elemType != "" {
			bodyScope[s.IterVar] = elemType
		}
		for i := range s.ThenSteps {
			validateActionStep(a, &s.ThenSteps[i], bodyScope, sym, diags)
		}

	case StepLet:
		if s.Expr != nil {
			if t, ok := inferExprType(s.Expr, scope, sym, diags); ok {
				scope[s.VarName] = t
			}
		}

	case StepReturn:
		if s.Expr != nil {
			sym.resolveEnumLiteral(s.Expr, a.ReturnType)
			t, ok := inferExprType(s.Expr, scope, sym, diags)
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

func validateErrorRef(errName, contextLabel string, span Span, sym *symbolTable, diags *[]Diagnostic) {
	if _, ok := sym.errors[errName]; !ok {
		*diags = append(*diags, Diagnostic{
			Code:     "FDN0206",
			Severity: SeverityError,
			Message:  fmt.Sprintf("undeclared error %q in %s", errName, contextLabel),
			Span:     span,
		})
	}
}

func validateEntityWhereClause(opName, entityName, whereField string, whereVal *Expr, span Span, scope map[string]string, sym *symbolTable, diags *[]Diagnostic) (EntityDecl, bool) {
	ent, ok := sym.entities[entityName]
	if !ok {
		*diags = append(*diags, Diagnostic{
			Code:     "FDN0203",
			Severity: SeverityError,
			Message:  fmt.Sprintf("unknown entity %q in %s", entityName, opName),
			Span:     span,
		})
		return EntityDecl{}, false
	}
	fields := sym.fieldsOf(ent.Name)
	wf, fOk := fields[whereField]
	if !fOk {
		*diags = append(*diags, Diagnostic{
			Code:     "FDN0204",
			Severity: SeverityError,
			Message:  fmt.Sprintf("entity %q has no field %q", ent.Name, whereField),
			Span:     span,
		})
	} else if whereVal != nil {
		sym.resolveEnumLiteral(whereVal, wf.Type)
		valType, ok := inferExprType(whereVal, scope, sym, diags)
		if ok && !isAssignableType(wf.Type, valType) {
			*diags = append(*diags, Diagnostic{
				Code:     "FDN0205",
				Severity: SeverityError,
				Message:  fmt.Sprintf("type mismatch in %s where clause: field %q is %s, got %s", opName, whereField, wf.Type, valType),
				Span:     whereVal.Span,
			})
		}
	}
	return ent, true
}

func validateRequireStep(s *StepDecl, scope map[string]string, sym *symbolTable, diags *[]Diagnostic) {
	pol, ok := sym.policies[s.PolicyName]
	if !ok {
		*diags = append(*diags, Diagnostic{
			Code:     "FDN0209",
			Severity: SeverityError,
			Message:  fmt.Sprintf("undeclared policy %q in require", s.PolicyName),
			Span:     s.Span,
		})
	} else if len(s.PolicyArgs) != len(pol.Params) {
		*diags = append(*diags, Diagnostic{
			Code:     "FDN0205",
			Severity: SeverityError,
			Message:  fmt.Sprintf("policy %q expects %d arguments, got %d", pol.Name, len(pol.Params), len(s.PolicyArgs)),
			Span:     s.Span,
		})
	} else {
		for idx := range s.PolicyArgs {
			expectedType := pol.Params[idx].Type
			sym.resolveEnumLiteral(&s.PolicyArgs[idx], expectedType)
			argType, ok := inferExprType(&s.PolicyArgs[idx], scope, sym, diags)
			if ok && !isAssignableType(expectedType, argType) {
				*diags = append(*diags, Diagnostic{
					Code:     "FDN0205",
					Severity: SeverityError,
					Message:  fmt.Sprintf("argument %d to policy %q has type %s, expected %s", idx+1, pol.Name, argType, expectedType),
					Span:     s.PolicyArgs[idx].Span,
				})
			}
		}
	}
	validateErrorRef(s.ElseError, "require else", s.Span, sym, diags)
}

func validateMatchStep(a ActionDecl, s *StepDecl, scope map[string]string, sym *symbolTable, diags *[]Diagnostic) {
	if s.Expr == nil {
		return
	}
	targetType, ok := inferExprType(s.Expr, scope, sym, diags)
	if !ok {
		return
	}
	en, isEnum := sym.enums[targetType]
	if !isEnum {
		*diags = append(*diags, Diagnostic{
			Code:     "FDN0205",
			Severity: SeverityError,
			Message:  fmt.Sprintf("match target must be an enum type, got %s", targetType),
			Span:     s.Expr.Span,
		})
		return
	}
	validVariants := make(map[string]bool, len(en.Variants))
	for _, v := range en.Variants {
		validVariants[v] = true
	}
	covered := make(map[string]bool, len(s.MatchArms))
	for i := range s.MatchArms {
		arm := &s.MatchArms[i]
		if !validVariants[arm.Variant] {
			*diags = append(*diags, Diagnostic{
				Code:     "FDN0208",
				Severity: SeverityError,
				Message:  fmt.Sprintf("enum %q has no variant .%s", en.Name, arm.Variant),
				Span:     arm.Span,
			})
		}
		covered[arm.Variant] = true
		armScope := cloneScope(scope)
		for j := range arm.Steps {
			validateActionStep(a, &arm.Steps[j], armScope, sym, diags)
		}
	}
	var missing []string
	for _, v := range en.Variants {
		if !covered[v] {
			missing = append(missing, "."+v)
		}
	}
	if len(missing) > 0 {
		*diags = append(*diags, Diagnostic{
			Code:     "FDN0212",
			Severity: SeverityError,
			Message:  fmt.Sprintf("non-exhaustive match over enum %q: missing variants %s", en.Name, strings.Join(missing, ", ")),
			Span:     s.Span,
		})
	}
}

// scenarioTypeCheckPass validates Scenario given seeds, when action invocations, and expect assertions.
type scenarioTypeCheckPass struct{}

func (scenarioTypeCheckPass) Validate(ir *SemanticIR, sym *symbolTable) []Diagnostic {
	var diags []Diagnostic

	for i := range ir.Scenarios {
		sc := &ir.Scenarios[i]
		for j := range sc.Givens {
			g := &sc.Givens[j]
			if g.EntityName == "actor" {
				validateFieldInitializers("type", "Actor", g.Fields, nil, sym, &diags)
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
			for k := range sc.WhenArgs {
				paramType := act.Params[k].Type
				sym.resolveEnumLiteral(&sc.WhenArgs[k], paramType)
				argType, ok := inferExprType(&sc.WhenArgs[k], nil, sym, &diags)
				if ok && !isAssignableType(paramType, argType) {
					diags = append(diags, Diagnostic{
						Code:     "FDN0205",
						Severity: SeverityError,
						Message:  fmt.Sprintf("argument %d to %q has type %s, expected %s", k+1, act.Name, argType, paramType),
						Span:     sc.WhenArgs[k].Span,
					})
				}
			}
		}

		expectScope := map[string]string{
			"result": act.ReturnType,
		}
		for k := range sc.Expects {
			ex := &sc.Expects[k]
			if ex.ErrorRaised != "" {
				validateErrorRef(ex.ErrorRaised, "scenario expect", ex.Span, sym, &diags)
				continue
			}
			if ex.EmittedEvent != "" {
				ev, evOk := sym.events[ex.EmittedEvent]
				if !evOk {
					diags = append(diags, Diagnostic{
						Code:     "FDN0203",
						Severity: SeverityError,
						Message:  fmt.Sprintf("undeclared event %q in scenario expect emitted", ex.EmittedEvent),
						Span:     ex.Span,
					})
					continue
				}
				validateFieldInitializers("event", ev.Name, ex.EmittedFields, expectScope, sym, &diags)
				continue
			}
			if ex.StoredEntity != "" {
				ent, sOk := validateEntityWhereClause("scenario expect stored", ex.StoredEntity, ex.WhereField, ex.WhereValue, ex.Span, expectScope, sym, &diags)
				if sOk && len(ex.StoredFields) > 0 {
					validateFieldInitializers("entity", ent.Name, ex.StoredFields, expectScope, sym, &diags)
				}
				continue
			}
			lt, lok := inferExprType(&ex.Left, expectScope, sym, &diags)
			if lok {
				sym.resolveEnumLiteral(&ex.Right, lt)
			}
			rt, rok := inferExprType(&ex.Right, expectScope, sym, &diags)
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
	for i := range inits {
		fi := &inits[i]
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
		sym.resolveEnumLiteral(&fi.Value, f.Type)
		valType, ok := inferExprType(&fi.Value, scope, sym, diags)
		if ok && !isAssignableType(f.Type, valType) {
			*diags = append(*diags, Diagnostic{
				Code:     "FDN0205",
				Severity: SeverityError,
				Message:  fmt.Sprintf("type mismatch for field %q: expected %s, got %s", fi.Name, f.Type, valType),
				Span:     fi.Value.Span,
			})
		} else if ok && fi.Value.Kind == ExprList && fi.Value.InferredType == "[]" {
			fi.Value.InferredType = f.Type
		}
	}
}

func inferExprType(e *Expr, scope map[string]string, sym *symbolTable, diags *[]Diagnostic) (string, bool) {
	switch e.Kind {
	case ExprString:
		e.InferredType = "string"
		return "string", true
	case ExprInt:
		e.InferredType = "int"
		return "int", true
	case ExprFloat:
		e.InferredType = "decimal"
		return "decimal", true
	case ExprBool:
		e.InferredType = "bool"
		return "bool", true
	case ExprEnumLit:
		if e.InferredType != "" && sym.enumHasVariant(e.InferredType, e.Value) {
			return e.InferredType, true
		}
		enName, ok := sym.findEnumByVariant(e.Value)
		if !ok {
			*diags = append(*diags, Diagnostic{
				Code:     "FDN0208",
				Severity: SeverityError,
				Message:  fmt.Sprintf("unknown enum variant .%s", e.Value),
				Span:     e.Span,
			})
			return "", false
		}
		e.InferredType = enName
		return enName, true
	case ExprList:
		if len(e.Items) == 0 {
			e.InferredType = "[]"
			return "[]", true
		}
		firstType, ok := inferExprType(&e.Items[0], scope, sym, diags)
		if !ok {
			return "", false
		}
		for i := 1; i < len(e.Items); i++ {
			sym.resolveEnumLiteral(&e.Items[i], firstType)
			itType, iOk := inferExprType(&e.Items[i], scope, sym, diags)
			if iOk && !isAssignableType(firstType, itType) {
				*diags = append(*diags, Diagnostic{
					Code:     "FDN0205",
					Severity: SeverityError,
					Message:  fmt.Sprintf("list item %d has type %s, expected %s", i+1, itType, firstType),
					Span:     e.Items[i].Span,
				})
			}
		}
		e.InferredType = "[]" + firstType
		return e.InferredType, true
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
		e.InferredType = t
		return t, true
	case ExprField:
		var targetType string
		if e.Left != nil {
			var ok bool
			targetType, ok = inferExprType(e.Left, scope, sym, diags)
			if !ok {
				return "", false
			}
		} else {
			var ok bool
			targetType, ok = scope[e.Target]
			if !ok {
				*diags = append(*diags, Diagnostic{
					Code:     "FDN0208",
					Severity: SeverityError,
					Message:  fmt.Sprintf("undeclared identifier %q", e.Target),
					Span:     e.Span,
				})
				return "", false
			}
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
		e.InferredType = f.Type
		return f.Type, true
	case ExprCombinator:
		return inferCombinatorType(e, scope, sym, diags)
	case ExprBinary:
		return inferBinaryExprType(e, scope, sym, diags)
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
		e.InferredType = e.TypeName
		return e.TypeName, true
	case ExprCall:
		for i := range e.Items {
			inferExprType(&e.Items[i], scope, sym, diags)
		}
		if act, ok := sym.actions[e.Value]; ok {
			e.InferredType = act.ReturnType
			return act.ReturnType, true
		}
		*diags = append(*diags, Diagnostic{
			Code:     "FDN0208",
			Severity: SeverityError,
			Message:  fmt.Sprintf("undeclared function %q", e.Value),
			Span:     e.Span,
		})
		return "", false
	default:
		return "", false
	}
}

func inferCombinatorType(e *Expr, scope map[string]string, sym *symbolTable, diags *[]Diagnostic) (string, bool) {
	if e.Left == nil {
		return "", false
	}
	recvType, ok := inferExprType(e.Left, scope, sym, diags)
	if !ok {
		return "", false
	}
	if !strings.HasPrefix(recvType, "[]") {
		*diags = append(*diags, Diagnostic{
			Code:     "FDN0205",
			Severity: SeverityError,
			Message:  fmt.Sprintf("combinator %q requires a collection receiver, got %s", e.Op, recvType),
			Span:     e.Span,
		})
		return "", false
	}
	elemType := strings.TrimPrefix(recvType, "[]")
	lambdaScope := cloneScope(scope)
	if e.Target != "" {
		lambdaScope[e.Target] = elemType
	}
	switch e.Op {
	case "map":
		if e.Right == nil {
			return "", false
		}
		bodyType, bOk := inferExprType(e.Right, lambdaScope, sym, diags)
		if !bOk {
			return "", false
		}
		e.InferredType = "[]" + bodyType
		return e.InferredType, true
	case "filter":
		if e.Right == nil {
			return "", false
		}
		validateBoolLambda(e.Op, e.Right, lambdaScope, sym, diags)
		e.InferredType = recvType
		return recvType, true
	case "sum":
		if e.Right == nil {
			if !isNumericType(elemType) {
				*diags = append(*diags, Diagnostic{
					Code:     "FDN0205",
					Severity: SeverityError,
					Message:  fmt.Sprintf("sum() without selector requires []int or []decimal, got %s", recvType),
					Span:     e.Span,
				})
				return "", false
			}
			e.InferredType = elemType
			return elemType, true
		}
		bodyType, bOk := inferExprType(e.Right, lambdaScope, sym, diags)
		if !bOk {
			return "", false
		}
		if !isNumericType(bodyType) {
			*diags = append(*diags, Diagnostic{
				Code:     "FDN0205",
				Severity: SeverityError,
				Message:  fmt.Sprintf("sum selector must return int or decimal, got %s", bodyType),
				Span:     e.Right.Span,
			})
			return "", false
		}
		e.InferredType = bodyType
		return bodyType, true
	case "count":
		if e.Right != nil {
			validateBoolLambda(e.Op, e.Right, lambdaScope, sym, diags)
		}
		e.InferredType = "int"
		return "int", true
	case "any", "all":
		if e.Right == nil {
			return "", false
		}
		validateBoolLambda(e.Op, e.Right, lambdaScope, sym, diags)
		e.InferredType = "bool"
		return "bool", true
	default:
		*diags = append(*diags, Diagnostic{
			Code:     "FDN0204",
			Severity: SeverityError,
			Message:  fmt.Sprintf("unknown collection combinator %q", e.Op),
			Span:     e.Span,
		})
		return "", false
	}
}

func validateBoolLambda(opName string, body *Expr, lambdaScope map[string]string, sym *symbolTable, diags *[]Diagnostic) {
	bodyType, bOk := inferExprType(body, lambdaScope, sym, diags)
	if bOk && bodyType != "bool" {
		*diags = append(*diags, Diagnostic{
			Code:     "FDN0205",
			Severity: SeverityError,
			Message:  fmt.Sprintf("%s predicate must return bool, got %s", opName, bodyType),
			Span:     body.Span,
		})
	}
}

func inferBinaryExprType(e *Expr, scope map[string]string, sym *symbolTable, diags *[]Diagnostic) (string, bool) {
	if e.Left == nil || e.Right == nil {
		return "", false
	}
	lt, lok := inferExprType(e.Left, scope, sym, diags)
	if lok {
		sym.resolveEnumLiteral(e.Right, lt)
	}
	rt, rok := inferExprType(e.Right, scope, sym, diags)
	if rok {
		sym.resolveEnumLiteral(e.Left, rt)
		if e.Left.Kind == ExprEnumLit && e.Left.InferredType == rt {
			lt = rt
		}
	}
	if !lok || !rok {
		return "", false
	}

	switch e.Op {
	case "&&", "||":
		if lt != "bool" || rt != "bool" {
			*diags = append(*diags, Diagnostic{
				Code:     "FDN0205",
				Severity: SeverityError,
				Message:  fmt.Sprintf("logical operator %s requires bool operands, got %s and %s", e.Op, lt, rt),
				Span:     e.Span,
			})
			return "", false
		}
		e.InferredType = "bool"
		return "bool", true

	case "==", "!=":
		if !isAssignableType(lt, rt) && !isAssignableType(rt, lt) && !(isNumericType(lt) && isNumericType(rt)) {
			*diags = append(*diags, Diagnostic{
				Code:     "FDN0205",
				Severity: SeverityError,
				Message:  fmt.Sprintf("incompatible operand types %s and %s for %s", lt, rt, e.Op),
				Span:     e.Span,
			})
			return "", false
		}
		e.InferredType = "bool"
		return "bool", true

	case "<", "<=", ">", ">=":
		bothNumeric := isNumericType(lt) && isNumericType(rt)
		bothOrderedStrings := (lt == "string" && rt == "string") || (lt == "timestamp" && rt == "timestamp")
		if !bothNumeric && !bothOrderedStrings {
			*diags = append(*diags, Diagnostic{
				Code:     "FDN0205",
				Severity: SeverityError,
				Message:  fmt.Sprintf("comparison operator %s requires numeric, string, or timestamp operands, got %s and %s", e.Op, lt, rt),
				Span:     e.Span,
			})
			return "", false
		}
		e.InferredType = "bool"
		return "bool", true

	case "+", "-", "*", "/":
		if e.Op == "+" && lt == "string" && rt == "string" {
			e.InferredType = "string"
			return "string", true
		}
		if isNumericType(lt) && isNumericType(rt) {
			if lt == "decimal" || rt == "decimal" {
				e.InferredType = "decimal"
				return "decimal", true
			}
			e.InferredType = "int"
			return "int", true
		}
		*diags = append(*diags, Diagnostic{
			Code:     "FDN0205",
			Severity: SeverityError,
			Message:  fmt.Sprintf("arithmetic operator %s requires numeric operands, got %s and %s", e.Op, lt, rt),
			Span:     e.Span,
		})
		return "", false
	}

	return "", false
}

// terminationPass enforces Action Pipeline termination invariants (no unbounded while loops or recursive action calls).
type terminationPass struct{}

func (terminationPass) Validate(ir *SemanticIR, sym *symbolTable) []Diagnostic {
	var diags []Diagnostic

	for _, a := range ir.Actions {
		walkStepsAndExprs(a.Steps,
			func(s StepDecl) {
				if s.Kind == StepWhile {
					diags = append(diags, Diagnostic{
						Code:     "FDN0210",
						Severity: SeverityError,
						Message:  "unbounded 'while' iteration is forbidden in Action Pipelines; use bounded 'for item in collection'",
						Span:     s.Span,
					})
				}
			},
			func(e Expr) {
				if e.Kind == ExprCall {
					if _, isAction := sym.actions[e.Value]; isAction {
						diags = append(diags, Diagnostic{
							Code:     "FDN0211",
							Severity: SeverityError,
							Message:  fmt.Sprintf("recursive or direct action invocation %q is forbidden in Action Pipelines", e.Value),
							Span:     e.Span,
						})
					}
				}
			},
		)
	}

	return diags
}
