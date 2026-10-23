package compiler

import (
	"fmt"
	"go/format"
	"regexp"
	"strings"
)

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

	for _, e := range arch.DomainEntities {
		relPath := fmt.Sprintf("internal/domain/%s_entity.go", strings.ToLower(e.Name))
		code, err := formatGo(relPath, emitGoSingleEntity(e))
		if err != nil {
			return nil, nil, []Diagnostic{errDiag(relPath, err)}
		}
		files[relPath] = code
	}

	if len(arch.DomainValues) > 0 {
		domainValues, err := formatGo("internal/domain/values.go", emitGoDomainValues(arch))
		if err != nil {
			return nil, nil, []Diagnostic{errDiag("internal/domain/values.go", err)}
		}
		files["internal/domain/values.go"] = domainValues
	}

	domainErrors, err := formatGo("internal/domain/errors.go", emitGoDomainErrors(arch))
	if err != nil {
		return nil, nil, []Diagnostic{errDiag("internal/domain/errors.go", err)}
	}
	files["internal/domain/errors.go"] = domainErrors

	for _, p := range arch.Ports {
		relPath := fmt.Sprintf("internal/ports/%s_repository.go", strings.ToLower(p.EntityName))
		code, err := formatGo(relPath, emitGoSinglePort(arch.Module, p))
		if err != nil {
			return nil, nil, []Diagnostic{errDiag(relPath, err)}
		}
		files[relPath] = code
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

func emitGoStructDef(name string, fields []FieldDecl) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("type %s struct {\n", name))
	for _, f := range fields {
		sb.WriteString(fmt.Sprintf("\t%s %s `json:%q`\n", exportIdent(f.Name), goType(f.Type, true), f.Name))
	}
	sb.WriteString("}\n")
	return sb.String()
}

func emitGoSingleEntity(e EntityDecl) string {
	return "package domain\n\n" + emitGoStructDef(e.Name, e.Fields)
}

func emitGoDomainValues(arch *ArchitectureIR) string {
	var sb strings.Builder
	sb.WriteString("package domain\n\n")
	for _, v := range arch.DomainValues {
		sb.WriteString(emitGoStructDef(v.Name, v.Fields))
		sb.WriteString("\n")
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
		if m.IsCreate {
			sb.WriteString(fmt.Sprintf("\tCreate(ctx context.Context, entity domain.%s) (domain.%s, error)\n", m.EntityName, m.EntityName))
		} else {
			sb.WriteString(fmt.Sprintf("\t%s(ctx context.Context, %s %s) (*domain.%s, error)\n",
				m.Name, m.WhereField, goType(m.WhereType, false), m.EntityName))
		}
	}
	sb.WriteString("}\n")

	return sb.String()
}

func emitGoSingleUseCase(module string, uc UseCaseIR) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`package usecases

import (
	"context"
	"%s/internal/domain"
	"%s/internal/ports"
)

`, module, module))

	sb.WriteString(fmt.Sprintf("type %s struct {\n", uc.Name))
	for _, dep := range uc.RepoDeps {
		sb.WriteString(fmt.Sprintf("\t%sRepo ports.%sRepository\n", dep, dep))
	}
	sb.WriteString("}\n\n")

	var ctorParams []string
	for _, dep := range uc.RepoDeps {
		ctorParams = append(ctorParams, fmt.Sprintf("%sRepo ports.%sRepository", strings.ToLower(dep), dep))
	}
	sb.WriteString(fmt.Sprintf("func New%s(%s) *%s {\n\treturn &%s{\n", uc.Name, strings.Join(ctorParams, ", "), uc.Name, uc.Name))
	for _, dep := range uc.RepoDeps {
		sb.WriteString(fmt.Sprintf("\t\t%sRepo: %sRepo,\n", dep, strings.ToLower(dep)))
	}
	sb.WriteString("\t}\n}\n\n")

	var execParams []string
	execParams = append(execParams, "ctx context.Context")
	for _, p := range uc.Params {
		execParams = append(execParams, fmt.Sprintf("%s %s", p.Name, goType(p.Type, false)))
	}
	retType := goType(uc.ReturnType, false)
	sb.WriteString(fmt.Sprintf("func (uc *%s) Execute(%s) (%s, error) {\n", uc.Name, strings.Join(execParams, ", "), retType))
	sb.WriteString(fmt.Sprintf("\tvar zero %s\n\t_ = zero\n", retType))

	for _, s := range uc.Steps {
		switch s.Kind {
		case StepLet:
			if s.Expr != nil {
				sb.WriteString(fmt.Sprintf("\t%s := %s\n", s.VarName, emitGoExpr(*s.Expr, false)))
			}
		case StepCreate:
			sb.WriteString(fmt.Sprintf("\t%s, err := uc.%sRepo.Create(ctx, domain.%s{\n", s.VarName, s.EntityName, s.EntityName))
			for _, fi := range s.Fields {
				sb.WriteString(fmt.Sprintf("\t\t%s: %s,\n", exportIdent(fi.Name), emitGoExpr(fi.Value, false)))
			}
			sb.WriteString("\t})\n")
			sb.WriteString("\tif err != nil {\n\t\treturn zero, err\n\t}\n")
		case StepQuery:
			mName := "FindBy" + exportIdent(s.WhereField)
			wExpr := emitGoExpr(*s.WhereValue, false)
			sb.WriteString(fmt.Sprintf("\t%sPtr, err := uc.%sRepo.%s(ctx, %s)\n", s.VarName, s.EntityName, mName, wExpr))
			sb.WriteString("\tif err != nil {\n\t\treturn zero, err\n\t}\n")
			if s.ElseError != "" {
				sb.WriteString(fmt.Sprintf("\tif %sPtr == nil {\n\t\treturn zero, domain.Err%s\n\t}\n", s.VarName, s.ElseError))
			}
			sb.WriteString(fmt.Sprintf("\t%s := *%sPtr\n", s.VarName, s.VarName))
		case StepReturn:
			if s.Expr != nil {
				sb.WriteString(fmt.Sprintf("\treturn %s, nil\n", emitGoExpr(*s.Expr, false)))
			}
		}
	}

	sb.WriteString("}\n")
	return sb.String()
}

func emitGoExpr(e Expr, inDomainPkg bool) string {
	switch e.Kind {
	case ExprString:
		return fmt.Sprintf("%q", e.Value)
	case ExprInt, ExprFloat, ExprBool:
		return e.Value
	case ExprIdent:
		return e.Value
	case ExprField:
		return fmt.Sprintf("%s.%s", e.Target, exportIdent(e.Field))
	case ExprBinary:
		return fmt.Sprintf("(%s %s %s)", emitGoExpr(*e.Left, inDomainPkg), e.Op, emitGoExpr(*e.Right, inDomainPkg))
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
		if m.IsCreate {
			sb.WriteString(fmt.Sprintf("func (r *%s) Create(_ context.Context, entity domain.%s) (domain.%s, error) {\n", repoStruct, p.EntityName, p.EntityName))
			sb.WriteString("\tr.mu.Lock()\n\tdefer r.mu.Unlock()\n\tr.seq++\n")
			if hasID {
				sb.WriteString("\tif entity.ID == \"\" {\n\t\tentity.ID = fmt.Sprintf(\"00000000-0000-0000-0000-%012d\", r.seq)\n\t}\n")
			} else {
				sb.WriteString("\t_ = fmt.Sprintf\n")
			}
			sb.WriteString("\tr.entities = append(r.entities, entity)\n\treturn entity, nil\n}\n\n")
		} else {
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
		var callArgs []string
		callArgs = append(callArgs, "r.Context()")

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
	for _, uc := range arch.UseCases {
		sb.WriteString(fmt.Sprintf("\t%s *usecases.%s\n", uc.ActionName, uc.Name))
	}
	sb.WriteString("\tHTTPServer *httpadapter.Server\n}\n\n")

	sb.WriteString("func NewTestContainer() *Container {\n")
	for _, p := range arch.Ports {
		sb.WriteString(fmt.Sprintf("\t%sRepo := memory.NewInMemory%sRepository()\n", strings.ToLower(p.EntityName), p.EntityName))
	}
	var srvArgs []string
	for _, uc := range arch.UseCases {
		var depArgs []string
		for _, dep := range uc.RepoDeps {
			depArgs = append(depArgs, strings.ToLower(dep)+"Repo")
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
				continue
			}
			sb.WriteString(fmt.Sprintf("\tc.%sRepo.Seed(domain.%s{\n", g.EntityName, g.EntityName))
			for _, fi := range g.Fields {
				sb.WriteString(fmt.Sprintf("\t\t%s: %s,\n", exportIdent(fi.Name), emitGoExpr(fi.Value, false)))
			}
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
			for _, ex := range sc.Expects {
				leftStr := emitGoExpr(ex.Left, false)
				rightStr := emitGoExpr(ex.Right, false)
				sb.WriteString(fmt.Sprintf("\tif !(%s %s %s) {\n\t\tt.Fatalf(\"assertion failed: expected %%v %s %%v\", %s, %s)\n\t}\n",
					leftStr, ex.Op, rightStr, ex.Op, leftStr, rightStr))
			}
		}

		sb.WriteString("}\n\n")
	}

	// Emit an HTTP handler integration test exercising the chi router
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
`)

	if len(arch.Scenarios) > 0 && len(arch.HTTPHandlers) > 0 {
		sc := arch.Scenarios[0]
		var handler *HTTPHandlerIR
		for i := range arch.HTTPHandlers {
			if arch.HTTPHandlers[i].ActionName == sc.WhenAction {
				handler = &arch.HTTPHandlers[i]
				break
			}
		}
		if handler != nil && handler.Method == "POST" && len(sc.WhenArgs) == 1 {
			sb.WriteString(fmt.Sprintf("\tbodyBytes, err := json.Marshal(%s)\n", emitGoExpr(sc.WhenArgs[0], false)))
			sb.WriteString("\tif err != nil {\n\t\tt.Fatalf(\"marshal input: %v\", err)\n\t}\n")
			sb.WriteString(fmt.Sprintf("\tpostReq := httptest.NewRequest(http.MethodPost, %q, bytes.NewReader(bodyBytes))\n", handler.Path))
			sb.WriteString("\tpostRec := httptest.NewRecorder()\n")
			sb.WriteString("\tc.HTTPServer.ServeHTTP(postRec, postReq)\n")
			sb.WriteString("\tif postRec.Code != http.StatusCreated {\n\t\tt.Fatalf(\"expected 201 Created from HTTP handler, got %d: %s\", postRec.Code, postRec.Body.String())\n\t}\n")
		}
	}

	sb.WriteString(`	_ = json.Marshal
	_ = errors.Is
	_ = domain.DomainError{}
}
`)

	return sb.String()
}
