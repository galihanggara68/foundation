package compiler

import (
	"fmt"
	"go/format"
	"regexp"
	"strings"
)

type goHexagonalEmitter struct{}

func (goHexagonalEmitter) Emit(arch *ArchitectureIR) (Emission, []Diagnostic) {
	tracked, stubs, diags := EmitGoHexagonal(arch)
	return Emission{
		TrackedFiles: tracked,
		StubFiles:    stubs,
	}, diags
}

// EmitGoHexagonal emits a complete Go Hexagonal Architecture workspace from the lowered ArchitectureIR.
func EmitGoHexagonal(arch *ArchitectureIR) (map[string][]byte, map[string][]byte, []Diagnostic) {
	files := make(map[string][]byte)
	stubs := make(map[string][]byte)

	files["go.mod"] = []byte(fmt.Sprintf(`module %s

go 1.23

require github.com/go-chi/chi/v5 v5.3.0
`, arch.Module))

	files["go.sum"] = []byte(`github.com/go-chi/chi/v5 v5.3.0 h1:halUjDxhshgXHMrao5bB8eNBXo/rnzwr8m5m36glehM=
github.com/go-chi/chi/v5 v5.3.0/go.mod h1:R+tYY2hNuVUUjxoPtqUdgBqevM9s9njzkTLutVsOCto=
`)

	if len(arch.DomainEnums) > 0 {
		enumsCode, err := formatGo("internal/domain/enums.go", emitGoDomainEnums(arch))
		if err != nil {
			return nil, nil, []Diagnostic{errDiag("internal/domain/enums.go", err)}
		}
		files["internal/domain/enums.go"] = enumsCode
	}

	for _, e := range arch.DomainEntities {
		relPath := fmt.Sprintf("internal/domain/%s_entity.go", strings.ToLower(e.Name))
		code, err := formatGo(relPath, "package domain\n\n"+emitGoStructDef(e.Name, e.Fields))
		if err != nil {
			return nil, nil, []Diagnostic{errDiag(relPath, err)}
		}
		files[relPath] = code
	}

	domainValues, err := formatGo("internal/domain/values.go", emitGoDomainValues(arch))
	if err != nil {
		return nil, nil, []Diagnostic{errDiag("internal/domain/values.go", err)}
	}
	files["internal/domain/values.go"] = domainValues

	domainErrors, err := formatGo("internal/domain/errors.go", emitGoDomainErrors(arch))
	if err != nil {
		return nil, nil, []Diagnostic{errDiag("internal/domain/errors.go", err)}
	}
	files["internal/domain/errors.go"] = domainErrors

	if len(arch.DomainEvents) > 0 {
		eventsCode, err := formatGo("internal/domain/events.go", emitGoDomainEvents(arch))
		if err != nil {
			return nil, nil, []Diagnostic{errDiag("internal/domain/events.go", err)}
		}
		files["internal/domain/events.go"] = eventsCode
	}

	if len(arch.DomainPolicies) > 0 {
		policiesCode, err := formatGo("internal/domain/policies.go", emitGoDomainPolicies(arch))
		if err != nil {
			return nil, nil, []Diagnostic{errDiag("internal/domain/policies.go", err)}
		}
		files["internal/domain/policies.go"] = policiesCode
	}

	for _, p := range arch.Ports {
		relPath := fmt.Sprintf("internal/ports/%s_repository.go", strings.ToLower(p.EntityName))
		code, err := formatGo(relPath, emitGoSinglePort(arch.Module, p))
		if err != nil {
			return nil, nil, []Diagnostic{errDiag(relPath, err)}
		}
		files[relPath] = code
	}

	if len(arch.DomainEvents) > 0 {
		evPortCode, err := formatGo("internal/ports/event_publisher.go", emitGoEventPublisherPort())
		if err != nil {
			return nil, nil, []Diagnostic{errDiag("internal/ports/event_publisher.go", err)}
		}
		files["internal/ports/event_publisher.go"] = evPortCode
	}

	for _, uc := range arch.UseCases {
		relPath := fmt.Sprintf("internal/usecases/%s_usecase.go", strings.ToLower(uc.ActionName))
		code, err := formatGo(relPath, emitGoSingleUseCase(arch.Module, uc))
		if err != nil {
			return nil, nil, []Diagnostic{errDiag(relPath, err)}
		}
		files[relPath] = code
	}

	for _, p := range arch.Ports {
		relPath := fmt.Sprintf("internal/adapters/memory/%s_repository.go", strings.ToLower(p.EntityName))
		code, err := formatGo(relPath, emitGoSingleMemoryRepo(arch, p))
		if err != nil {
			return nil, nil, []Diagnostic{errDiag(relPath, err)}
		}
		files[relPath] = code
	}

	if len(arch.DomainEvents) > 0 {
		evMemCode, err := formatGo("internal/adapters/memory/event_publisher.go", emitGoMemoryEventPublisher())
		if err != nil {
			return nil, nil, []Diagnostic{errDiag("internal/adapters/memory/event_publisher.go", err)}
		}
		files["internal/adapters/memory/event_publisher.go"] = evMemCode
	}

	httpRouterCode, err := formatGo("internal/adapters/http/router.go", emitGoChiRouter(arch))
	if err != nil {
		return nil, nil, []Diagnostic{errDiag("internal/adapters/http/router.go", err)}
	}
	files["internal/adapters/http/router.go"] = httpRouterCode

	bootstrapCode, err := formatGo("internal/bootstrap/container.go", emitGoBootstrap(arch))
	if err != nil {
		return nil, nil, []Diagnostic{errDiag("internal/bootstrap/container.go", err)}
	}
	files["internal/bootstrap/container.go"] = bootstrapCode

	scenariosCode, err := formatGo("scenarios_test.go", emitGoScenariosTest(arch))
	if err != nil {
		return nil, nil, []Diagnostic{errDiag("scenarios_test.go", err)}
	}
	files["scenarios_test.go"] = scenariosCode

	return files, stubs, nil
}

func formatGo(path, src string) ([]byte, error) {
	formatted, err := format.Source([]byte(src))
	if err != nil {
		return nil, fmt.Errorf("gofmt %s failed: %w\nsource:\n%s", path, err, src)
	}
	return formatted, nil
}

func errDiag(file string, err error) Diagnostic {
	return Diagnostic{
		Code:     "FDN0500",
		Severity: SeverityError,
		Message:  err.Error(),
		Span:     Span{File: file, Line: 1, Column: 1},
	}
}

func goType(fdnType string, inDomainPkg bool) string {
	if strings.HasPrefix(fdnType, "[]") {
		return "[]" + goType(strings.TrimPrefix(fdnType, "[]"), inDomainPkg)
	}
	switch fdnType {
	case "uuid", "string", "timestamp":
		return "string"
	case "int":
		return "int"
	case "decimal":
		return "float64"
	case "bool":
		return "bool"
	default:
		if inDomainPkg {
			return fdnType
		}
		return "domain." + fdnType
	}
}

func emitGoDomainEnums(arch *ArchitectureIR) string {
	var sb strings.Builder
	sb.WriteString("package domain\n\n")
	for _, en := range arch.DomainEnums {
		sb.WriteString(fmt.Sprintf("type %s string\n\nconst (\n", en.Name))
		for _, v := range en.Variants {
			sb.WriteString(fmt.Sprintf("\t%s%s %s = %q\n", en.Name, exportIdent(v), en.Name, v))
		}
		sb.WriteString(")\n\n")
	}
	return sb.String()
}

func emitGoStructDef(name string, fields []FieldDecl) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("type %s struct {\n", name))
	for _, f := range fields {
		sb.WriteString(fmt.Sprintf("\t%s %s `json:%q`\n", exportIdent(f.Name), goType(f.Type, true), f.Name))
	}
	sb.WriteString("}\n")
	return sb.String()
}

func emitGoFieldInits(sb *strings.Builder, fields []FieldInitDecl, indent string) {
	for _, fi := range fields {
		sb.WriteString(fmt.Sprintf("%s%s: %s,\n", indent, exportIdent(fi.Name), emitGoExpr(fi.Value, false)))
	}
}

func emitGoDomainValues(arch *ArchitectureIR) string {
	var sb strings.Builder
	sb.WriteString(`package domain

import "context"

type actorCtxKey struct{}

type Actor struct {
	ID         string ` + "`json:\"id\"`" + `
	Role       string ` + "`json:\"role\"`" + `
	IsVerified bool   ` + "`json:\"is_verified\"`" + `
	TenantID   string ` + "`json:\"tenant_id\"`" + `
}

func WithActor(ctx context.Context, actor Actor) context.Context {
	return context.WithValue(ctx, actorCtxKey{}, actor)
}

func ActorFromContext(ctx context.Context) Actor {
	if v, ok := ctx.Value(actorCtxKey{}).(Actor); ok {
		return v
	}
	return Actor{}
}

`)
	for _, v := range arch.DomainValues {
		sb.WriteString(emitGoStructDef(v.Name, v.Fields))
		sb.WriteString("\n")
	}
	return sb.String()
}

func emitGoDomainEvents(arch *ArchitectureIR) string {
	var sb strings.Builder
	sb.WriteString("package domain\n\n")
	for _, ev := range arch.DomainEvents {
		sb.WriteString(emitGoStructDef(ev.Name, ev.Fields))
		sb.WriteString("\n")
	}
	return sb.String()
}

func emitGoDomainPolicies(arch *ArchitectureIR) string {
	var sb strings.Builder
	sb.WriteString("package domain\n\n")
	for _, pol := range arch.DomainPolicies {
		var params []string
		for _, p := range pol.Params {
			params = append(params, fmt.Sprintf("%s %s", p.Name, goType(p.Type, true)))
		}
		sb.WriteString(fmt.Sprintf("func %s(%s) bool {\n\treturn %s\n}\n\n",
			pol.Name, strings.Join(params, ", "), emitGoExpr(pol.Body, true)))
	}
	return sb.String()
}

func emitGoDomainErrors(arch *ArchitectureIR) string {
	var sb strings.Builder
	sb.WriteString(`package domain

type DomainError struct {
	Code    string ` + "`json:\"code\"`" + `
	Kind    string ` + "`json:\"kind\"`" + `
	Message string ` + "`json:\"message\"`" + `
}

func (e *DomainError) Error() string {
	return e.Message
}

`)
	for _, errDecl := range arch.DomainErrors {
		sb.WriteString(fmt.Sprintf("var Err%s = &DomainError{\n\tCode: %q,\n\tKind: %q,\n\tMessage: %q,\n}\n\n",
			errDecl.Name, errDecl.Name, errDecl.Kind, errDecl.Message))
	}
	return sb.String()
}

func emitGoSinglePort(module string, p RepositoryPortIR) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`package ports

import (
	"context"
	"%s/internal/domain"
)

`, module))

	sb.WriteString(fmt.Sprintf("type %s interface {\n", p.InterfaceName))
	for _, m := range p.Methods {
		switch m.Kind {
		case RepoMethodCreate:
			sb.WriteString(fmt.Sprintf("\tCreate(ctx context.Context, entity domain.%s) (domain.%s, error)\n", m.EntityName, m.EntityName))
		case RepoMethodUpdate:
			sb.WriteString(fmt.Sprintf("\tUpdate(ctx context.Context, entity domain.%s) error\n", m.EntityName))
		case RepoMethodDelete:
			sb.WriteString(fmt.Sprintf("\t%s(ctx context.Context, %s %s) error\n",
				m.Name, m.WhereField, goType(m.WhereType, false)))
		case RepoMethodFind:
			sb.WriteString(fmt.Sprintf("\t%s(ctx context.Context, %s %s) (*domain.%s, error)\n",
				m.Name, m.WhereField, goType(m.WhereType, false), m.EntityName))
		}
	}
	sb.WriteString("}\n")

	return sb.String()
}

func emitGoEventPublisherPort() string {
	return `package ports

import "context"

type EventPublisher interface {
	Publish(ctx context.Context, eventType string, payload any) error
}
`
}

func emitGoMemoryEventPublisher() string {
	return `package memory

import (
	"context"
	"sync"
)

type PublishedEvent struct {
	EventType string
	Payload   any
}

type InMemoryEventPublisher struct {
	mu     sync.RWMutex
	events []PublishedEvent
}

func NewInMemoryEventPublisher() *InMemoryEventPublisher {
	return &InMemoryEventPublisher{}
}

func (p *InMemoryEventPublisher) Publish(_ context.Context, eventType string, payload any) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.events = append(p.events, PublishedEvent{
		EventType: eventType,
		Payload:   payload,
	})
	return nil
}

func (p *InMemoryEventPublisher) ByType(eventType string) []any {
	p.mu.RLock()
	defer p.mu.RUnlock()
	var out []any
	for _, ev := range p.events {
		if ev.EventType == eventType {
			out = append(out, ev.Payload)
		}
	}
	return out
}
`
}

func emitGoSingleUseCase(module string, uc UseCaseIR) string {
	var sb strings.Builder
	sb.WriteString("package usecases\n\nimport (\n\t\"context\"\n")
	sb.WriteString(fmt.Sprintf("\t\"%s/internal/domain\"\n", module))
	if len(uc.RepoDeps) > 0 || uc.EmitsEvents {
		sb.WriteString(fmt.Sprintf("\t\"%s/internal/ports\"\n", module))
	}
	sb.WriteString(")\n\n")

	sb.WriteString(fmt.Sprintf("type %s struct {\n", uc.Name))
	for _, dep := range uc.RepoDeps {
		sb.WriteString(fmt.Sprintf("\t%sRepo ports.%sRepository\n", dep, dep))
	}
	if uc.EmitsEvents {
		sb.WriteString("\tEvents ports.EventPublisher\n")
	}
	sb.WriteString("}\n\n")

	var ctorParams []string
	for _, dep := range uc.RepoDeps {
		ctorParams = append(ctorParams, fmt.Sprintf("%sRepo ports.%sRepository", strings.ToLower(dep), dep))
	}
	if uc.EmitsEvents {
		ctorParams = append(ctorParams, "events ports.EventPublisher")
	}
	sb.WriteString(fmt.Sprintf("func New%s(%s) *%s {\n\treturn &%s{\n", uc.Name, strings.Join(ctorParams, ", "), uc.Name, uc.Name))
	for _, dep := range uc.RepoDeps {
		sb.WriteString(fmt.Sprintf("\t\t%sRepo: %sRepo,\n", dep, strings.ToLower(dep)))
	}
	if uc.EmitsEvents {
		sb.WriteString("\t\tEvents: events,\n")
	}
	sb.WriteString("\t}\n}\n\n")

	var execParams []string
	execParams = append(execParams, "ctx context.Context")
	for _, p := range uc.Params {
		execParams = append(execParams, fmt.Sprintf("%s %s", p.Name, goType(p.Type, false)))
	}
	retType := goType(uc.ReturnType, false)
	sb.WriteString(fmt.Sprintf("func (uc *%s) Execute(%s) (%s, error) {\n", uc.Name, strings.Join(execParams, ", "), retType))
	sb.WriteString(fmt.Sprintf("\tvar zero %s\n\t_ = zero\n\t_ = domain.Actor{}\n", retType))
	if uc.UsesActor {
		sb.WriteString("\tactor := domain.ActorFromContext(ctx)\n\t_ = actor\n")
	}

	declaredVars := make(map[string]bool)
	for _, p := range uc.Params {
		declaredVars[p.Name] = true
	}
	stepCounter := 0
	emitGoSteps(&sb, uc.Steps, declaredVars, &stepCounter, "\t")

	sb.WriteString("}\n")
	return sb.String()
}

func emitGoSteps(sb *strings.Builder, steps []StepDecl, declaredVars map[string]bool, stepCounter *int, indent string) {
	for _, s := range steps {
		*stepCounter++
		idx := *stepCounter
		switch s.Kind {
		case StepRequire:
			var args []string
			for _, arg := range s.PolicyArgs {
				args = append(args, emitGoExpr(arg, false))
			}
			sb.WriteString(fmt.Sprintf("%sif !domain.%s(%s) {\n%s\treturn zero, domain.Err%s\n%s}\n",
				indent, s.PolicyName, strings.Join(args, ", "), indent, s.ElseError, indent))

		case StepGuard:
			if s.Expr != nil {
				sb.WriteString(fmt.Sprintf("%sif !(%s) {\n%s\treturn zero, domain.Err%s\n%s}\n",
					indent, emitGoExpr(*s.Expr, false), indent, s.ElseError, indent))
			}

		case StepLet:
			if s.Expr != nil {
				assignOp := ":="
				if declaredVars[s.VarName] {
					assignOp = "="
				} else {
					declaredVars[s.VarName] = true
				}
				sb.WriteString(fmt.Sprintf("%s%s %s %s\n", indent, s.VarName, assignOp, emitGoExpr(*s.Expr, false)))
			}

		case StepCreate:
			declaredVars[s.VarName] = true
			sb.WriteString(fmt.Sprintf("%s%s, err := uc.%sRepo.Create(ctx, domain.%s{\n", indent, s.VarName, s.EntityName, s.EntityName))
			emitGoFieldInits(sb, s.Fields, indent+"\t")
			sb.WriteString(fmt.Sprintf("%s})\n", indent))
			sb.WriteString(fmt.Sprintf("%sif err != nil {\n%s\treturn zero, err\n%s}\n", indent, indent, indent))

		case StepQuery:
			declaredVars[s.VarName] = true
			mName := "FindBy" + exportIdent(s.WhereField)
			wExpr := emitGoExpr(*s.WhereValue, false)
			sb.WriteString(fmt.Sprintf("%s%sPtr, err := uc.%sRepo.%s(ctx, %s)\n", indent, s.VarName, s.EntityName, mName, wExpr))
			sb.WriteString(fmt.Sprintf("%sif err != nil {\n%s\treturn zero, err\n%s}\n", indent, indent, indent))
			if s.ElseError != "" {
				sb.WriteString(fmt.Sprintf("%sif %sPtr == nil {\n%s\treturn zero, domain.Err%s\n%s}\n", indent, s.VarName, indent, s.ElseError, indent))
			}
			sb.WriteString(fmt.Sprintf("%s%s := *%sPtr\n", indent, s.VarName, s.VarName))

		case StepUpdate:
			mName := "FindBy" + exportIdent(s.WhereField)
			wExpr := emitGoExpr(*s.WhereValue, false)
			updVar := fmt.Sprintf("updTarget%d", idx)
			sb.WriteString(fmt.Sprintf("%s%s, err := uc.%sRepo.%s(ctx, %s)\n", indent, updVar, s.EntityName, mName, wExpr))
			sb.WriteString(fmt.Sprintf("%sif err != nil {\n%s\treturn zero, err\n%s}\n", indent, indent, indent))
			sb.WriteString(fmt.Sprintf("%sif %s != nil {\n", indent, updVar))
			for _, fi := range s.Fields {
				sb.WriteString(fmt.Sprintf("%s\t%s.%s = %s\n", indent, updVar, exportIdent(fi.Name), emitGoExpr(fi.Value, false)))
			}
			sb.WriteString(fmt.Sprintf("%s\tif err := uc.%sRepo.Update(ctx, *%s); err != nil {\n%s\t\treturn zero, err\n%s\t}\n",
				indent, s.EntityName, updVar, indent, indent))
			sb.WriteString(fmt.Sprintf("%s}\n", indent))

		case StepDelete:
			mName := "DeleteBy" + exportIdent(s.WhereField)
			wExpr := emitGoExpr(*s.WhereValue, false)
			sb.WriteString(fmt.Sprintf("%sif err := uc.%sRepo.%s(ctx, %s); err != nil {\n%s\treturn zero, err\n%s}\n",
				indent, s.EntityName, mName, wExpr, indent, indent))

		case StepIf:
			if s.Expr != nil {
				sb.WriteString(fmt.Sprintf("%sif %s {\n", indent, emitGoExpr(*s.Expr, false)))
				emitGoSteps(sb, s.ThenSteps, cloneBoolMap(declaredVars), stepCounter, indent+"\t")
				if len(s.ElseSteps) > 0 {
					sb.WriteString(fmt.Sprintf("%s} else {\n", indent))
					emitGoSteps(sb, s.ElseSteps, cloneBoolMap(declaredVars), stepCounter, indent+"\t")
				}
				sb.WriteString(fmt.Sprintf("%s}\n", indent))
			}

		case StepMatch:
			if s.Expr != nil {
				enumType := s.Expr.InferredType
				sb.WriteString(fmt.Sprintf("%sswitch %s {\n", indent, emitGoExpr(*s.Expr, false)))
				for _, arm := range s.MatchArms {
					sb.WriteString(fmt.Sprintf("%scase domain.%s%s:\n", indent, enumType, exportIdent(arm.Variant)))
					emitGoSteps(sb, arm.Steps, cloneBoolMap(declaredVars), stepCounter, indent+"\t")
				}
				sb.WriteString(fmt.Sprintf("%s}\n", indent))
			}

		case StepFor:
			if s.Expr != nil {
				sb.WriteString(fmt.Sprintf("%sfor _, %s := range %s {\n%s\t_ = %s\n",
					indent, s.IterVar, emitGoExpr(*s.Expr, false), indent, s.IterVar))
				loopVars := cloneBoolMap(declaredVars)
				loopVars[s.IterVar] = true
				emitGoSteps(sb, s.ThenSteps, loopVars, stepCounter, indent+"\t")
				sb.WriteString(fmt.Sprintf("%s}\n", indent))
			}

		case StepEmit:
			sb.WriteString(fmt.Sprintf("%sif uc.Events != nil {\n%s\t_ = uc.Events.Publish(ctx, %q, domain.%s{\n", indent, indent, s.EventName, s.EventName))
			emitGoFieldInits(sb, s.Fields, indent+"\t\t")
			sb.WriteString(fmt.Sprintf("%s\t})\n%s}\n", indent, indent))

		case StepReturn:
			if s.Expr != nil {
				sb.WriteString(fmt.Sprintf("%sreturn %s, nil\n", indent, emitGoExpr(*s.Expr, false)))
			}
		}
	}
}

func cloneBoolMap(in map[string]bool) map[string]bool {
	out := make(map[string]bool, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func emitGoExpr(e Expr, inDomainPkg bool) string {
	switch e.Kind {
	case ExprString:
		return fmt.Sprintf("%q", e.Value)
	case ExprInt, ExprFloat, ExprBool:
		return e.Value
	case ExprEnumLit:
		prefix := "domain."
		if inDomainPkg {
			prefix = ""
		}
		if e.InferredType != "" {
			return fmt.Sprintf("%s%s%s", prefix, e.InferredType, exportIdent(e.Value))
		}
		return fmt.Sprintf("%q", e.Value)
	case ExprList:
		sliceGoType := goType(e.InferredType, inDomainPkg)
		var items []string
		for _, it := range e.Items {
			items = append(items, emitGoExpr(it, inDomainPkg))
		}
		return fmt.Sprintf("%s{%s}", sliceGoType, strings.Join(items, ", "))
	case ExprIdent:
		return e.Value
	case ExprField:
		if e.Left != nil {
			return fmt.Sprintf("%s.%s", emitGoExpr(*e.Left, inDomainPkg), exportIdent(e.Field))
		}
		return fmt.Sprintf("%s.%s", e.Target, exportIdent(e.Field))
	case ExprCombinator:
		return emitGoCombinatorExpr(e, inDomainPkg)
	case ExprBinary:
		leftStr := emitGoExpr(*e.Left, inDomainPkg)
		rightStr := emitGoExpr(*e.Right, inDomainPkg)
		if (e.Left.InferredType == "decimal" && e.Right.InferredType == "int") ||
			(e.Left.InferredType == "int" && e.Right.InferredType == "decimal") {
			if e.Left.InferredType == "int" {
				leftStr = fmt.Sprintf("float64(%s)", leftStr)
			}
			if e.Right.InferredType == "int" {
				rightStr = fmt.Sprintf("float64(%s)", rightStr)
			}
		}
		return fmt.Sprintf("(%s %s %s)", leftStr, e.Op, rightStr)
	case ExprConstruct:
		prefix := "domain."
		if inDomainPkg {
			prefix = ""
		}
		var parts []string
		for _, f := range e.Fields {
			parts = append(parts, fmt.Sprintf("%s: %s", exportIdent(f.Name), emitGoExpr(f.Value, inDomainPkg)))
		}
		return fmt.Sprintf("%s%s{%s}", prefix, e.TypeName, strings.Join(parts, ", "))
	default:
		return e.Value
	}
}

func emitGoCombinatorExpr(e Expr, inDomainPkg bool) string {
	recvStr := emitGoExpr(*e.Left, inDomainPkg)
	retGoType := goType(e.InferredType, inDomainPkg)
	param := e.Target
	if param == "" {
		param = "_item"
	}
	switch e.Op {
	case "filter":
		bodyStr := emitGoExpr(*e.Right, inDomainPkg)
		return fmt.Sprintf("func() %s { var _out %s; for _, %s := range %s { if %s { _out = append(_out, %s) } }; return _out }()",
			retGoType, retGoType, param, recvStr, bodyStr, param)
	case "map":
		bodyStr := emitGoExpr(*e.Right, inDomainPkg)
		return fmt.Sprintf("func() %s { var _out %s; for _, %s := range %s { _out = append(_out, %s) }; return _out }()",
			retGoType, retGoType, param, recvStr, bodyStr)
	case "sum":
		bodyStr := param
		if e.Right != nil {
			bodyStr = emitGoExpr(*e.Right, inDomainPkg)
		}
		return fmt.Sprintf("func() %s { var _sum %s; for _, %s := range %s { _sum += %s }; return _sum }()",
			retGoType, retGoType, param, recvStr, bodyStr)
	case "count":
		if e.Right == nil {
			return fmt.Sprintf("len(%s)", recvStr)
		}
		bodyStr := emitGoExpr(*e.Right, inDomainPkg)
		return fmt.Sprintf("func() int { var _cnt int; for _, %s := range %s { if %s { _cnt++ } }; return _cnt }()",
			param, recvStr, bodyStr)
	case "any":
		bodyStr := emitGoExpr(*e.Right, inDomainPkg)
		return fmt.Sprintf("func() bool { for _, %s := range %s { if %s { return true } }; return false }()",
			param, recvStr, bodyStr)
	case "all":
		bodyStr := emitGoExpr(*e.Right, inDomainPkg)
		return fmt.Sprintf("func() bool { for _, %s := range %s { if !(%s) { return false } }; return true }()",
			param, recvStr, bodyStr)
	}
	return recvStr
}

func emitGoSingleMemoryRepo(arch *ArchitectureIR, p RepositoryPortIR) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`package memory

import (
	"context"
	"fmt"
	"sync"
	"%s/internal/domain"
)

`, arch.Module))

	entityMap := make(map[string]EntityDecl)
	for _, e := range arch.DomainEntities {
		entityMap[e.Name] = e
	}

	repoStruct := "InMemory" + p.EntityName + "Repository"
	sb.WriteString(fmt.Sprintf("type %s struct {\n\tmu sync.RWMutex\n\tentities []domain.%s\n\tseq int\n}\n\n", repoStruct, p.EntityName))
	sb.WriteString(fmt.Sprintf("func New%s() *%s {\n\treturn &%s{}\n}\n\n", repoStruct, repoStruct, repoStruct))
	sb.WriteString(fmt.Sprintf("func (r *%s) Seed(entity domain.%s) {\n\tr.mu.Lock()\n\tdefer r.mu.Unlock()\n\tr.entities = append(r.entities, entity)\n}\n\n", repoStruct, p.EntityName))

	ent := entityMap[p.EntityName]
	hasID := false
	for _, f := range ent.Fields {
		if f.Name == "id" && f.Type == "uuid" {
			hasID = true
		}
	}

	for _, m := range p.Methods {
		switch m.Kind {
		case RepoMethodCreate:
			sb.WriteString(fmt.Sprintf("func (r *%s) Create(_ context.Context, entity domain.%s) (domain.%s, error) {\n", repoStruct, p.EntityName, p.EntityName))
			sb.WriteString("\tr.mu.Lock()\n\tdefer r.mu.Unlock()\n\tr.seq++\n")
			if hasID {
				sb.WriteString("\tif entity.ID == \"\" {\n\t\tentity.ID = fmt.Sprintf(\"00000000-0000-0000-0000-%012d\", r.seq)\n\t}\n")
			} else {
				sb.WriteString("\t_ = fmt.Sprintf\n")
			}
			for _, f := range ent.Fields {
				if strings.HasPrefix(f.DefaultVal, ".") {
					variant := strings.TrimPrefix(f.DefaultVal, ".")
					fExport := exportIdent(f.Name)
					sb.WriteString(fmt.Sprintf("\tif entity.%s == \"\" {\n\t\tentity.%s = domain.%s%s\n\t}\n",
						fExport, fExport, f.Type, exportIdent(variant)))
				}
			}
			sb.WriteString("\tr.entities = append(r.entities, entity)\n\treturn entity, nil\n}\n\n")

		case RepoMethodUpdate:
			sb.WriteString(fmt.Sprintf("func (r *%s) Update(_ context.Context, entity domain.%s) error {\n", repoStruct, p.EntityName))
			sb.WriteString("\tr.mu.Lock()\n\tdefer r.mu.Unlock()\n")
			if hasID {
				sb.WriteString("\tfor i := range r.entities {\n\t\tif r.entities[i].ID == entity.ID {\n\t\t\tr.entities[i] = entity\n\t\t\treturn nil\n\t\t}\n\t}\n")
			} else if len(ent.Fields) > 0 {
				firstExport := exportIdent(ent.Fields[0].Name)
				sb.WriteString(fmt.Sprintf("\tfor i := range r.entities {\n\t\tif r.entities[i].%s == entity.%s {\n\t\t\tr.entities[i] = entity\n\t\t\treturn nil\n\t\t}\n\t}\n", firstExport, firstExport))
			}
			sb.WriteString("\treturn nil\n}\n\n")

		case RepoMethodDelete:
			fieldExport := exportIdent(m.WhereField)
			sb.WriteString(fmt.Sprintf("func (r *%s) %s(_ context.Context, %s %s) error {\n",
				repoStruct, m.Name, m.WhereField, goType(m.WhereType, false)))
			sb.WriteString("\tr.mu.Lock()\n\tdefer r.mu.Unlock()\n")
			sb.WriteString(fmt.Sprintf("\tvar kept []domain.%s\n", p.EntityName))
			sb.WriteString("\tfor _, item := range r.entities {\n")
			sb.WriteString(fmt.Sprintf("\t\tif item.%s != %s {\n\t\t\tkept = append(kept, item)\n\t\t}\n", fieldExport, m.WhereField))
			sb.WriteString("\t}\n\tr.entities = kept\n\treturn nil\n}\n\n")

		case RepoMethodFind:
			fieldExport := exportIdent(m.WhereField)
			sb.WriteString(fmt.Sprintf("func (r *%s) %s(_ context.Context, %s %s) (*domain.%s, error) {\n",
				repoStruct, m.Name, m.WhereField, goType(m.WhereType, false), p.EntityName))
			sb.WriteString("\tr.mu.RLock()\n\tdefer r.mu.RUnlock()\n")
			sb.WriteString("\tfor _, item := range r.entities {\n")
			sb.WriteString(fmt.Sprintf("\t\tif item.%s == %s {\n\t\t\tcopyItem := item\n\t\t\treturn &copyItem, nil\n\t\t}\n", fieldExport, m.WhereField))
			sb.WriteString("\t}\n\treturn nil, nil\n}\n\n")
		}
	}

	return sb.String()
}

func emitGoChiRouter(arch *ArchitectureIR) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`package httpadapter

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"%s/internal/domain"
	"%s/internal/usecases"
)

type Server struct {
	router *chi.Mux
`, arch.Module, arch.Module))

	for _, uc := range arch.UseCases {
		sb.WriteString(fmt.Sprintf("\t%s *usecases.%s\n", uc.ActionName, uc.Name))
	}
	sb.WriteString("}\n\n")

	var ctorArgs []string
	for _, uc := range arch.UseCases {
		ctorArgs = append(ctorArgs, fmt.Sprintf("%s *usecases.%s", strings.ToLower(uc.ActionName), uc.Name))
	}
	sb.WriteString(fmt.Sprintf("func NewServer(%s) *Server {\n\ts := &Server{\n\t\trouter: chi.NewRouter(),\n", strings.Join(ctorArgs, ", ")))
	for _, uc := range arch.UseCases {
		sb.WriteString(fmt.Sprintf("\t\t%s: %s,\n", uc.ActionName, strings.ToLower(uc.ActionName)))
	}
	sb.WriteString("\t}\n\ts.routes()\n\treturn s\n}\n\n")

	sb.WriteString("func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {\n\ts.router.ServeHTTP(w, r)\n}\n\n")

	sb.WriteString("func (s *Server) routes() {\n")
	for _, h := range arch.HTTPHandlers {
		methodFunc := "Post"
		switch h.Method {
		case "GET":
			methodFunc = "Get"
		case "POST":
			methodFunc = "Post"
		case "PUT":
			methodFunc = "Put"
		case "DELETE":
			methodFunc = "Delete"
		}
		sb.WriteString(fmt.Sprintf("\ts.router.%s(%q, s.handle%s)\n", methodFunc, h.Path, h.ActionName))
	}
	sb.WriteString("}\n\n")

	for _, h := range arch.HTTPHandlers {
		sb.WriteString(fmt.Sprintf("func (s *Server) handle%s(w http.ResponseWriter, r *http.Request) {\n", h.ActionName))
		sb.WriteString("\tctx := domain.WithActor(r.Context(), domain.Actor{\n\t\tID: r.Header.Get(\"X-Actor-ID\"),\n\t\tRole: r.Header.Get(\"X-Actor-Role\"),\n\t\tIsVerified: r.Header.Get(\"X-Actor-Verified\") == \"true\",\n\t})\n")
		var callArgs []string
		callArgs = append(callArgs, "ctx")

		pathParamSet := make(map[string]bool)
		for _, pp := range h.PathParams {
			pathParamSet[pp] = true
		}

		for _, p := range h.Params {
			if pathParamSet[p.Name] {
				sb.WriteString(fmt.Sprintf("\t%s := chi.URLParam(r, %q)\n", p.Name, p.Name))
				callArgs = append(callArgs, p.Name)
			} else {
				sb.WriteString(fmt.Sprintf("\tvar %s %s\n", p.Name, goType(p.Type, false)))
				sb.WriteString(fmt.Sprintf("\tif err := json.NewDecoder(r.Body).Decode(&%s); err != nil {\n\t\twriteError(w, err)\n\t\treturn\n\t}\n", p.Name))
				callArgs = append(callArgs, p.Name)
			}
		}

		sb.WriteString(fmt.Sprintf("\tres, err := s.%s.Execute(%s)\n", h.ActionName, strings.Join(callArgs, ", ")))
		sb.WriteString("\tif err != nil {\n\t\twriteError(w, err)\n\t\treturn\n\t}\n")
		status := "http.StatusOK"
		if h.Method == "POST" {
			status = "http.StatusCreated"
		}
		sb.WriteString(fmt.Sprintf("\tw.Header().Set(\"Content-Type\", \"application/json\")\n\tw.WriteHeader(%s)\n\t_ = json.NewEncoder(w).Encode(res)\n}\n\n", status))
	}

	sb.WriteString(`func writeError(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "application/json")
	var domErr *domain.DomainError
	if errors.As(err, &domErr) {
		status := http.StatusInternalServerError
		switch domErr.Kind {
		case "not_found":
			status = http.StatusNotFound
		case "validation":
			status = http.StatusBadRequest
		case "conflict":
			status = http.StatusConflict
		case "unauthorized":
			status = http.StatusUnauthorized
		case "forbidden":
			status = http.StatusForbidden
		}
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(domErr)
		return
	}
	w.WriteHeader(http.StatusBadRequest)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}
`)

	return sb.String()
}

func emitGoBootstrap(arch *ArchitectureIR) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`package bootstrap

import (
	httpadapter "%s/internal/adapters/http"
	"%s/internal/adapters/memory"
	"%s/internal/usecases"
)

type Container struct {
`, arch.Module, arch.Module, arch.Module))

	for _, p := range arch.Ports {
		sb.WriteString(fmt.Sprintf("\t%sRepo *memory.InMemory%sRepository\n", p.EntityName, p.EntityName))
	}
	if len(arch.DomainEvents) > 0 {
		sb.WriteString("\tEvents *memory.InMemoryEventPublisher\n")
	}
	for _, uc := range arch.UseCases {
		sb.WriteString(fmt.Sprintf("\t%s *usecases.%s\n", uc.ActionName, uc.Name))
	}
	sb.WriteString("\tHTTPServer *httpadapter.Server\n}\n\n")

	sb.WriteString("func NewTestContainer() *Container {\n")
	for _, p := range arch.Ports {
		sb.WriteString(fmt.Sprintf("\t%sRepo := memory.NewInMemory%sRepository()\n", strings.ToLower(p.EntityName), p.EntityName))
	}
	if len(arch.DomainEvents) > 0 {
		sb.WriteString("\tevents := memory.NewInMemoryEventPublisher()\n")
	}
	var srvArgs []string
	for _, uc := range arch.UseCases {
		var depArgs []string
		for _, dep := range uc.RepoDeps {
			depArgs = append(depArgs, strings.ToLower(dep)+"Repo")
		}
		if uc.EmitsEvents {
			depArgs = append(depArgs, "events")
		}
		varName := strings.ToLower(uc.ActionName) + "UC"
		sb.WriteString(fmt.Sprintf("\t%s := usecases.New%s(%s)\n", varName, uc.Name, strings.Join(depArgs, ", ")))
		srvArgs = append(srvArgs, varName)
	}
	sb.WriteString(fmt.Sprintf("\tsrv := httpadapter.NewServer(%s)\n", strings.Join(srvArgs, ", ")))
	sb.WriteString("\treturn &Container{\n")
	for _, p := range arch.Ports {
		sb.WriteString(fmt.Sprintf("\t\t%sRepo: %sRepo,\n", p.EntityName, strings.ToLower(p.EntityName)))
	}
	if len(arch.DomainEvents) > 0 {
		sb.WriteString("\t\tEvents: events,\n")
	}
	for _, uc := range arch.UseCases {
		sb.WriteString(fmt.Sprintf("\t\t%s: %s,\n", uc.ActionName, strings.ToLower(uc.ActionName)+"UC"))
	}
	sb.WriteString("\t\tHTTPServer: srv,\n\t}\n}\n")

	return sb.String()
}

var nonAlphaNum = regexp.MustCompile(`[^a-zA-Z0-9]+`)

func sanitizeTestFuncName(name string) string {
	parts := nonAlphaNum.Split(name, -1)
	var sb strings.Builder
	sb.WriteString("TestScenario_")
	for _, p := range parts {
		if p == "" {
			continue
		}
		sb.WriteString(strings.ToUpper(p[:1]) + p[1:])
	}
	return sb.String()
}

func emitGoScenariosTest(arch *ArchitectureIR) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`package scenarios_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"%s/internal/bootstrap"
	"%s/internal/domain"
)

`, arch.Module, arch.Module))

	for _, sc := range arch.Scenarios {
		fnName := sanitizeTestFuncName(sc.Name)
		sb.WriteString(fmt.Sprintf("func %s(t *testing.T) {\n", fnName))
		sb.WriteString("\tctx := context.Background()\n")
		sb.WriteString("\tc := bootstrap.NewTestContainer()\n")

		for _, g := range sc.Givens {
			if g.EntityName == "actor" {
				sb.WriteString("\tctx = domain.WithActor(ctx, domain.Actor{\n")
				emitGoFieldInits(&sb, g.Fields, "\t\t")
				sb.WriteString("\t})\n")
				continue
			}
			sb.WriteString(fmt.Sprintf("\tc.%sRepo.Seed(domain.%s{\n", g.EntityName, g.EntityName))
			emitGoFieldInits(&sb, g.Fields, "\t\t")
			sb.WriteString("\t})\n")
		}

		var callArgs []string
		callArgs = append(callArgs, "ctx")
		for _, arg := range sc.WhenArgs {
			callArgs = append(callArgs, emitGoExpr(arg, false))
		}
		sb.WriteString(fmt.Sprintf("\tresult, err := c.%s.Execute(%s)\n", sc.WhenAction, strings.Join(callArgs, ", ")))

		expectsError := false
		for _, ex := range sc.Expects {
			if ex.ErrorRaised != "" {
				expectsError = true
				sb.WriteString("\t_ = result\n")
				sb.WriteString(fmt.Sprintf("\tif !errors.Is(err, domain.Err%s) {\n\t\tt.Fatalf(\"expected error %s, got %%v\", err)\n\t}\n",
					ex.ErrorRaised, ex.ErrorRaised))
			}
		}

		if !expectsError {
			sb.WriteString("\tif err != nil {\n\t\tt.Fatalf(\"unexpected error: %v\", err)\n\t}\n")
			sb.WriteString("\t_ = result\n")
			for idx, ex := range sc.Expects {
				emitGoScenarioExpect(&sb, idx, ex)
			}
		}

		sb.WriteString("}\n\n")
	}

	sb.WriteString(`func TestHTTPAdapter_Integration(t *testing.T) {
	c := bootstrap.NewTestContainer()
	if c.HTTPServer == nil {
		t.Fatal("expected non-nil HTTPServer")
	}
	req := httptest.NewRequest(http.MethodGet, "/__healthz_not_registered", bytes.NewReader(nil))
	rec := httptest.NewRecorder()
	c.HTTPServer.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown route, got %d", rec.Code)
	}
	_ = json.Marshal
	_ = errors.Is
	_ = domain.DomainError{}
}
`)

	return sb.String()
}

func emitGoScenarioExpect(sb *strings.Builder, idx int, ex ExpectDecl) {
	if ex.EmittedEvent != "" {
		evVar := fmt.Sprintf("emitted%d", idx)
		sb.WriteString(fmt.Sprintf("\t%s := c.Events.ByType(%q)\n", evVar, ex.EmittedEvent))
		sb.WriteString(fmt.Sprintf("\tif len(%s) == 0 {\n\t\tt.Fatalf(\"expected event %s to be emitted, but none were published\")\n\t}\n",
			evVar, ex.EmittedEvent))
		if len(ex.EmittedFields) > 0 {
			sb.WriteString(fmt.Sprintf("\tevPayload%d, ok := %s[len(%s)-1].(domain.%s)\n", idx, evVar, evVar, ex.EmittedEvent))
			sb.WriteString(fmt.Sprintf("\tif !ok {\n\t\tt.Fatalf(\"unexpected event payload type for %s\")\n\t}\n", ex.EmittedEvent))
			for _, fi := range ex.EmittedFields {
				fieldAccess := fmt.Sprintf("evPayload%d.%s", idx, exportIdent(fi.Name))
				expectedVal := emitGoExpr(fi.Value, false)
				sb.WriteString(fmt.Sprintf("\tif %s != %s {\n\t\tt.Fatalf(\"event %s.%s mismatch: got %%v, want %%v\", %s, %s)\n\t}\n",
					fieldAccess, expectedVal, ex.EmittedEvent, fi.Name, fieldAccess, expectedVal))
			}
		}
		return
	}

	if ex.StoredEntity != "" && ex.WhereValue != nil {
		findMethod := "FindBy" + exportIdent(ex.WhereField)
		whereExpr := emitGoExpr(*ex.WhereValue, false)
		storedVar := fmt.Sprintf("stored%d", idx)
		sb.WriteString(fmt.Sprintf("\t%s, err := c.%sRepo.%s(ctx, %s)\n", storedVar, ex.StoredEntity, findMethod, whereExpr))
		sb.WriteString("\tif err != nil {\n\t\tt.Fatalf(\"unexpected repo error: %v\", err)\n\t}\n")
		if ex.NotStored {
			sb.WriteString(fmt.Sprintf("\tif %s != nil {\n\t\tt.Fatalf(\"expected %s where %s == %%v to be deleted, but found %%+v\", %s, *%s)\n\t}\n",
				storedVar, ex.StoredEntity, ex.WhereField, whereExpr, storedVar))
			return
		}
		sb.WriteString(fmt.Sprintf("\tif %s == nil {\n\t\tt.Fatalf(\"expected %s where %s == %%v to be stored, but found nil\", %s)\n\t}\n",
			storedVar, ex.StoredEntity, ex.WhereField, whereExpr))
		for _, fi := range ex.StoredFields {
			fieldAccess := fmt.Sprintf("%s.%s", storedVar, exportIdent(fi.Name))
			expectedVal := emitGoExpr(fi.Value, false)
			sb.WriteString(fmt.Sprintf("\tif %s != %s {\n\t\tt.Fatalf(\"stored %s.%s mismatch: got %%v, want %%v\", %s, %s)\n\t}\n",
				fieldAccess, expectedVal, ex.StoredEntity, fi.Name, fieldAccess, expectedVal))
		}
		return
	}

	leftStr := emitGoExpr(ex.Left, false)
	rightStr := emitGoExpr(ex.Right, false)
	sb.WriteString(fmt.Sprintf("\tif !(%s %s %s) {\n\t\tt.Fatalf(\"assertion failed: expected %%v %s %%v\", %s, %s)\n\t}\n",
		leftStr, ex.Op, rightStr, ex.Op, leftStr, rightStr))
}
