package gohex

import (
	"fmt"
	"strings"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/arch"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/platform"
)

func (lay goLayout) emitGoChiRouter(archIR *arch.ArchitectureIR) string {
	var sb strings.Builder
	portsImport := ""
	if archIR.HasActorPort {
		portsImport = fmt.Sprintf("\t\"%s/%s\"\n", archIR.Module, lay.PortsDir)
	}
	sb.WriteString(fmt.Sprintf(`package %s

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"%s/%s"
%s	"%s/%s"
)

var (
	_ = fmt.Sprintf
	_ = strings.Trim
	_ = strconv.Atoi
)

type Server struct {
	router *chi.Mux
`, lay.HTTPPkg, archIR.Module, lay.DomainDir, portsImport, archIR.Module, lay.UseCasesDir))

	if archIR.HasActorPort {
		sb.WriteString(fmt.Sprintf("\tactorResolver %s.ActorResolver\n", lay.PortsPkg))
	}
	for _, uc := range archIR.UseCases {
		sb.WriteString(fmt.Sprintf("\t%s *%s.%s\n", uc.ActionName, lay.UseCasesPkg, uc.Name))
	}
	sb.WriteString("}\n\n")

	var ctorArgs []string
	if archIR.HasActorPort {
		ctorArgs = append(ctorArgs, fmt.Sprintf("actorResolver %s.ActorResolver", lay.PortsPkg))
	}
	for _, uc := range archIR.UseCases {
		ctorArgs = append(ctorArgs, fmt.Sprintf("%s *%s.%s", strings.ToLower(uc.ActionName), lay.UseCasesPkg, uc.Name))
	}
	sb.WriteString(fmt.Sprintf("func NewServer(%s) *Server {\n\ts := &Server{\n\t\trouter: chi.NewRouter(),\n", strings.Join(ctorArgs, ", ")))
	if archIR.HasActorPort {
		sb.WriteString("\t\tactorResolver: actorResolver,\n")
	}
	for _, uc := range archIR.UseCases {
		sb.WriteString(fmt.Sprintf("\t\t%s: %s,\n", uc.ActionName, strings.ToLower(uc.ActionName)))
	}
	sb.WriteString("\t}\n\ts.routes()\n\treturn s\n}\n\n")

	sb.WriteString("func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {\n\ts.router.ServeHTTP(w, r)\n}\n\n")

	sb.WriteString("func (s *Server) routes() {\n")
	for _, h := range archIR.HTTPHandlers {
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

	for _, h := range archIR.HTTPHandlers {
		var targetUC *arch.UseCaseIR
		for i := range archIR.UseCases {
			if archIR.UseCases[i].ActionName == h.ActionName {
				targetUC = &archIR.UseCases[i]
				break
			}
		}
		sb.WriteString(fmt.Sprintf("func (s *Server) handle%s(w http.ResponseWriter, r *http.Request) {\n", h.ActionName))
		if archIR.HasActorPort {
			sb.WriteString(fmt.Sprintf("\tactor, err := s.actorResolver.ResolveActor(r.Context(), r)\n\tif err != nil {\n\t\twriteError(w, err)\n\t\treturn\n\t}\n\tctx := %s.WithActor(r.Context(), actor)\n", lay.DomainPkg))
		} else {
			sb.WriteString("\tctx := r.Context()\n")
		}
		var callArgs []string
		callArgs = append(callArgs, "ctx")

		pathParamSet := make(map[string]bool)
		for _, pp := range h.PathParams {
			pathParamSet[pp] = true
		}

		var bodyParams []ir.ParamDecl
		for _, p := range h.Params {
			if pathParamSet[p.Name] {
				sb.WriteString(fmt.Sprintf("\t%s := chi.URLParam(r, %q)\n", p.Name, p.Name))
				callArgs = append(callArgs, p.Name)
			} else {
				bodyParams = append(bodyParams, p)
			}
		}

		if h.Method == "GET" {
			for _, p := range bodyParams {
				if isScalarParam(p) && p.Kind == ir.KindInt {
					sb.WriteString(fmt.Sprintf("\t%s, _ := strconv.Atoi(r.URL.Query().Get(%q))\n", p.Name, p.Name))
				} else if isScalarParam(p) && p.Kind == ir.KindBool {
					sb.WriteString(fmt.Sprintf("\t%s := r.URL.Query().Get(%q) == \"true\"\n", p.Name, p.Name))
				} else {
					sb.WriteString(fmt.Sprintf("\t%s := r.URL.Query().Get(%q)\n", p.Name, p.Name))
				}
				callArgs = append(callArgs, p.Name)
			}
		} else if len(bodyParams) == 1 && !isScalarParam(bodyParams[0]) {
			p := bodyParams[0]
			sb.WriteString(fmt.Sprintf("\tvar %s %s\n", p.Name, lay.goKindType(p.Kind, p.Type, false)))
			sb.WriteString(fmt.Sprintf("\tif err := json.NewDecoder(r.Body).Decode(&%s); err != nil {\n\t\twriteError(w, err)\n\t\treturn\n\t}\n", p.Name))
			callArgs = append(callArgs, p.Name)
		} else if len(bodyParams) > 0 {
			sb.WriteString("\tvar reqBody struct {\n")
			for _, p := range bodyParams {
				sb.WriteString(fmt.Sprintf("\t\t%s %s `json:%q`\n", arch.ExportIdent(p.Name), lay.goKindType(p.Kind, p.Type, false), p.Name))
			}
			sb.WriteString("\t}\n")
			sb.WriteString("\tif err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {\n\t\twriteError(w, err)\n\t\treturn\n\t}\n")
			for _, p := range bodyParams {
				sb.WriteString(fmt.Sprintf("\t%s := reqBody.%s\n", p.Name, arch.ExportIdent(p.Name)))
				callArgs = append(callArgs, p.Name)
			}
		}

		if h.Method != "GET" && h.ExpectedVersionParam != "" {
			sb.WriteString(fmt.Sprintf("\tif ifMatch := r.Header.Get(\"If-Match\"); ifMatch != \"\" && ifMatch != \"*\" {\n\t\tcleanMatch := strings.Trim(ifMatch, \"\\\"\")\n\t\tif fmt.Sprintf(\"%%d\", %s) != cleanMatch {\n\t\t\twriteError(w, %s.ErrConflict)\n\t\t\treturn\n\t\t}\n\t}\n", h.ExpectedVersionParam, lay.DomainPkg))
		}

		sb.WriteString(fmt.Sprintf("\tres, err := s.%s.Execute(%s)\n", h.ActionName, strings.Join(callArgs, ", ")))
		sb.WriteString("\tif err != nil {\n\t\twriteError(w, err)\n\t\treturn\n\t}\n")
		if h.Method == "GET" && targetUC != nil {
			retEnt := archIR.EntityByName(targetUC.ReturnType)
			if _, vExp, hasV := arch.EntityVersionField(retEnt); hasV {
				sb.WriteString(fmt.Sprintf("\tw.Header().Set(\"ETag\", fmt.Sprintf(\"\\\"%%d\\\"\", res.%s))\n", vExp))
			}
		}
		status := "http.StatusOK"
		if h.Method == "POST" {
			status = "http.StatusCreated"
		}
		sb.WriteString(fmt.Sprintf("\tw.Header().Set(\"Content-Type\", \"application/json\")\n\tw.WriteHeader(%s)\n\t_ = json.NewEncoder(w).Encode(res)\n}\n\n", status))
	}

	var standardPolicy strings.Builder
	for _, kind := range platform.StandardErrorKinds() {
		policy, _ := platform.ErrorPolicyFor(kind)
		fmt.Fprintf(&standardPolicy, "\t%q: {status: %d, posture: %s.%s},\n",
			kind, policy.Status, lay.DomainPkg, emitGoPostureConst(policy.Posture))
	}

	sb.WriteString(fmt.Sprintf(`
type errorPolicy struct {
	status  int
	posture string
}

// standardErrorPolicy is the platform's standard treatment of each error kind,
// generated from the compiler's vocabulary.
var standardErrorPolicy = map[string]errorPolicy{
%s}

// policyFor resolves the platform's standard treatment of an error the compiler did
// not stamp — one built by application code. A compiler-stamped value always wins,
// exactly as a declared value wins over the kind default at build time, and anything
// unrecognised stays diagnostic so an unknown message never reaches a caller.
func policyFor(domErr *%s.DomainError) errorPolicy {
	policy := errorPolicy{status: http.StatusInternalServerError, posture: %s.PostureTechnical}
	if standard, ok := standardErrorPolicy[string(domErr.Kind)]; ok {
		policy = standard
	}
	if domErr.Status >= %d && domErr.Status <= %d {
		policy.status = domErr.Status
	}
	if domErr.Posture == %s.PostureBusiness || domErr.Posture == %s.PostureTechnical {
		policy.posture = domErr.Posture
	}
	return policy
}

func writeError(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "application/json")
	var domErr *%s.DomainError
	if errors.As(err, &domErr) {
		policy := policyFor(domErr)
		w.WriteHeader(policy.status)
		if policy.posture == %s.PostureTechnical {
			log.Printf("error %%s (%%s): %%s", domErr.Code, domErr.Kind, domErr.Message)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"code": domErr.Code,
				"kind": domErr.Kind,
			})
			return
		}
		_ = json.NewEncoder(w).Encode(domErr)
		return
	}
	w.WriteHeader(http.StatusBadRequest)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}
`, standardPolicy.String(), lay.DomainPkg, lay.DomainPkg, platform.MinHTTPStatus, platform.MaxHTTPStatus, lay.DomainPkg, lay.DomainPkg, lay.DomainPkg, lay.DomainPkg))

	return sb.String()
}
