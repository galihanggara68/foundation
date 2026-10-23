package validate

import (
	"fmt"
	"strings"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
)

func validateFieldInitializers(kindLabel, typeName string, inits []ir.FieldInitDecl, scope map[string]string, sym *SymbolTable, diags *[]ir.Diagnostic) {
	fields := sym.fieldsOf(typeName)
	for i := range inits {
		fi := &inits[i]
		f, fOk := fields[fi.Name]
		if !fOk {
			*diags = append(*diags, ir.Diagnostic{
				Code:     "FDN0204",
				Severity: ir.SeverityError,
				Message:  fmt.Sprintf("%s %q has no field %q", kindLabel, typeName, fi.Name),
				Span:     fi.Span,
			})
			continue
		}
		sym.resolveEnumLiteral(&fi.Value, f.Type)
		valType, ok := inferExprType(&fi.Value, scope, sym, diags)
		if ok && !isAssignableType(f.Type, valType) {
			*diags = append(*diags, ir.Diagnostic{
				Code:     "FDN0205",
				Severity: ir.SeverityError,
				Message:  fmt.Sprintf("type mismatch for field %q: expected %s, got %s", fi.Name, f.Type, valType),
				Span:     fi.Value.Span,
			})
		} else if ok && fi.Value.Kind == ir.ExprList && fi.Value.InferredType == "[]" {
			fi.Value.InferredType = f.Type
		}
	}
}

func inferExprType(e *ir.Expr, scope map[string]string, sym *SymbolTable, diags *[]ir.Diagnostic) (string, bool) {
	switch e.Kind {
	case ir.ExprString:
		e.InferredType = "string"
		return "string", true
	case ir.ExprInt:
		e.InferredType = "int"
		return "int", true
	case ir.ExprFloat:
		e.InferredType = "decimal"
		return "decimal", true
	case ir.ExprBool:
		e.InferredType = "bool"
		return "bool", true
	case ir.ExprEnumLit:
		if e.InferredType != "" && sym.enumHasVariant(e.InferredType, e.Value) {
			return e.InferredType, true
		}
		enName, ok := sym.findEnumByVariant(e.Value)
		if !ok {
			*diags = append(*diags, ir.Diagnostic{
				Code:     "FDN0208",
				Severity: ir.SeverityError,
				Message:  fmt.Sprintf("unknown enum variant .%s", e.Value),
				Span:     e.Span,
			})
			return "", false
		}
		e.InferredType = enName
		return enName, true
	case ir.ExprList:
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
				*diags = append(*diags, ir.Diagnostic{
					Code:     "FDN0205",
					Severity: ir.SeverityError,
					Message:  fmt.Sprintf("list item %d has type %s, expected %s", i+1, itType, firstType),
					Span:     e.Items[i].Span,
				})
			}
		}
		e.InferredType = "[]" + firstType
		return e.InferredType, true
	case ir.ExprIdent:
		t, ok := scope[e.Value]
		if !ok {
			*diags = append(*diags, ir.Diagnostic{
				Code:     "FDN0208",
				Severity: ir.SeverityError,
				Message:  fmt.Sprintf("undeclared identifier %q", e.Value),
				Span:     e.Span,
			})
			return "", false
		}
		e.InferredType = t
		return t, true
	case ir.ExprField:
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
				*diags = append(*diags, ir.Diagnostic{
					Code:     "FDN0208",
					Severity: ir.SeverityError,
					Message:  fmt.Sprintf("undeclared identifier %q", e.Target),
					Span:     e.Span,
				})
				return "", false
			}
		}
		fields := sym.fieldsOf(targetType)
		f, fOk := fields[e.Field]
		if !fOk {
			*diags = append(*diags, ir.Diagnostic{
				Code:     "FDN0204",
				Severity: ir.SeverityError,
				Message:  fmt.Sprintf("type %q has no field %q", targetType, e.Field),
				Span:     e.Span,
			})
			return "", false
		}
		e.InferredType = f.Type
		return f.Type, true
	case ir.ExprCombinator:
		return inferCombinatorType(e, scope, sym, diags)
	case ir.ExprBinary:
		return inferBinaryExprType(e, scope, sym, diags)
	case ir.ExprConstruct:
		fields := sym.fieldsOf(e.TypeName)
		if len(fields) == 0 {
			*diags = append(*diags, ir.Diagnostic{
				Code:     "FDN0202",
				Severity: ir.SeverityError,
				Message:  fmt.Sprintf("unknown type %q in struct construction", e.TypeName),
				Span:     e.Span,
			})
			return "", false
		}
		validateFieldInitializers("type", e.TypeName, e.Fields, scope, sym, diags)
		e.InferredType = e.TypeName
		return e.TypeName, true
	case ir.ExprCall:
		for i := range e.Items {
			inferExprType(&e.Items[i], scope, sym, diags)
		}
		if act, ok := sym.actions[e.Value]; ok {
			e.InferredType = act.ReturnType
			return act.ReturnType, true
		}
		*diags = append(*diags, ir.Diagnostic{
			Code:     "FDN0208",
			Severity: ir.SeverityError,
			Message:  fmt.Sprintf("undeclared function %q", e.Value),
			Span:     e.Span,
		})
		return "", false
	default:
		return "", false
	}
}

func inferCombinatorType(e *ir.Expr, scope map[string]string, sym *SymbolTable, diags *[]ir.Diagnostic) (string, bool) {
	if e.Left == nil {
		return "", false
	}
	recvType, ok := inferExprType(e.Left, scope, sym, diags)
	if !ok {
		return "", false
	}
	if !strings.HasPrefix(recvType, "[]") {
		*diags = append(*diags, ir.Diagnostic{
			Code:     "FDN0205",
			Severity: ir.SeverityError,
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
				*diags = append(*diags, ir.Diagnostic{
					Code:     "FDN0205",
					Severity: ir.SeverityError,
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
			*diags = append(*diags, ir.Diagnostic{
				Code:     "FDN0205",
				Severity: ir.SeverityError,
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
		*diags = append(*diags, ir.Diagnostic{
			Code:     "FDN0204",
			Severity: ir.SeverityError,
			Message:  fmt.Sprintf("unknown collection combinator %q", e.Op),
			Span:     e.Span,
		})
		return "", false
	}
}

func validateBoolLambda(opName string, body *ir.Expr, lambdaScope map[string]string, sym *SymbolTable, diags *[]ir.Diagnostic) {
	bodyType, bOk := inferExprType(body, lambdaScope, sym, diags)
	if bOk && bodyType != "bool" {
		*diags = append(*diags, ir.Diagnostic{
			Code:     "FDN0205",
			Severity: ir.SeverityError,
			Message:  fmt.Sprintf("%s predicate must return bool, got %s", opName, bodyType),
			Span:     body.Span,
		})
	}
}

func inferBinaryExprType(e *ir.Expr, scope map[string]string, sym *SymbolTable, diags *[]ir.Diagnostic) (string, bool) {
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
		if e.Left.Kind == ir.ExprEnumLit && e.Left.InferredType == rt {
			lt = rt
		}
	}
	if !lok || !rok {
		return "", false
	}

	switch e.Op {
	case "&&", "||":
		if lt != "bool" || rt != "bool" {
			*diags = append(*diags, ir.Diagnostic{
				Code:     "FDN0205",
				Severity: ir.SeverityError,
				Message:  fmt.Sprintf("logical operator %s requires bool operands, got %s and %s", e.Op, lt, rt),
				Span:     e.Span,
			})
			return "", false
		}
		e.InferredType = "bool"
		return "bool", true

	case "==", "!=":
		if !isAssignableType(lt, rt) && !isAssignableType(rt, lt) && !(isNumericType(lt) && isNumericType(rt)) {
			*diags = append(*diags, ir.Diagnostic{
				Code:     "FDN0205",
				Severity: ir.SeverityError,
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
			*diags = append(*diags, ir.Diagnostic{
				Code:     "FDN0205",
				Severity: ir.SeverityError,
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
		*diags = append(*diags, ir.Diagnostic{
			Code:     "FDN0205",
			Severity: ir.SeverityError,
			Message:  fmt.Sprintf("arithmetic operator %s requires numeric operands, got %s and %s", e.Op, lt, rt),
			Span:     e.Span,
		})
		return "", false
	}

	return "", false
}
