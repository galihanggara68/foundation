package gohex

import (
	"fmt"
	"strings"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/arch"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
)

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

func emitGoSingleMemoryRepo(archIR *arch.ArchitectureIR, p arch.RepositoryPortIR) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`package memory

import (
	"context"
	"fmt"
	"sync"
	"%s/internal/domain"
)

`, archIR.Module))

	entityMap := make(map[string]ir.EntityDecl)
	for _, e := range archIR.DomainEntities {
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
		case arch.RepoMethodCreate:
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
					fExport := arch.ExportIdent(f.Name)
					sb.WriteString(fmt.Sprintf("\tif entity.%s == \"\" {\n\t\tentity.%s = domain.%s%s\n\t}\n",
						fExport, fExport, f.Type, arch.ExportIdent(variant)))
				}
			}
			sb.WriteString("\tr.entities = append(r.entities, entity)\n\treturn entity, nil\n}\n\n")

		case arch.RepoMethodUpdate:
			sb.WriteString(fmt.Sprintf("func (r *%s) Update(_ context.Context, entity domain.%s) error {\n", repoStruct, p.EntityName))
			sb.WriteString("\tr.mu.Lock()\n\tdefer r.mu.Unlock()\n")
			if hasID {
				sb.WriteString("\tfor i := range r.entities {\n\t\tif r.entities[i].ID == entity.ID {\n\t\t\tr.entities[i] = entity\n\t\t\treturn nil\n\t\t}\n\t}\n")
			} else if len(ent.Fields) > 0 {
				firstExport := arch.ExportIdent(ent.Fields[0].Name)
				sb.WriteString(fmt.Sprintf("\tfor i := range r.entities {\n\t\tif r.entities[i].%s == entity.%s {\n\t\t\tr.entities[i] = entity\n\t\t\treturn nil\n\t\t}\n\t}\n", firstExport, firstExport))
			}
			sb.WriteString("\treturn nil\n}\n\n")

		case arch.RepoMethodDelete:
			fieldExport := arch.ExportIdent(m.WhereField)
			sb.WriteString(fmt.Sprintf("func (r *%s) %s(_ context.Context, %s %s) error {\n",
				repoStruct, m.Name, m.WhereField, goType(m.WhereType, false)))
			sb.WriteString("\tr.mu.Lock()\n\tdefer r.mu.Unlock()\n")
			sb.WriteString(fmt.Sprintf("\tvar kept []domain.%s\n", p.EntityName))
			sb.WriteString("\tfor _, item := range r.entities {\n")
			sb.WriteString(fmt.Sprintf("\t\tif item.%s != %s {\n\t\t\tkept = append(kept, item)\n\t\t}\n", fieldExport, m.WhereField))
			sb.WriteString("\t}\n\tr.entities = kept\n\treturn nil\n}\n\n")

		case arch.RepoMethodFind:
			fieldExport := arch.ExportIdent(m.WhereField)
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

func emitGoChiRouter(archIR *arch.ArchitectureIR) string {
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
`, archIR.Module, archIR.Module))

	for _, uc := range archIR.UseCases {
		sb.WriteString(fmt.Sprintf("\t%s *usecases.%s\n", uc.ActionName, uc.Name))
	}
	sb.WriteString("}\n\n")

	var ctorArgs []string
	for _, uc := range archIR.UseCases {
		ctorArgs = append(ctorArgs, fmt.Sprintf("%s *usecases.%s", strings.ToLower(uc.ActionName), uc.Name))
	}
	sb.WriteString(fmt.Sprintf("func NewServer(%s) *Server {\n\ts := &Server{\n\t\trouter: chi.NewRouter(),\n", strings.Join(ctorArgs, ", ")))
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

func emitGoBootstrap(archIR *arch.ArchitectureIR) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`package bootstrap

import (
	httpadapter "%s/internal/adapters/http"
	"%s/internal/adapters/memory"
	"%s/internal/usecases"
)

type Container struct {
`, archIR.Module, archIR.Module, archIR.Module))

	for _, p := range archIR.Ports {
		sb.WriteString(fmt.Sprintf("\t%sRepo *memory.InMemory%sRepository\n", p.EntityName, p.EntityName))
	}
	if len(archIR.DomainEvents) > 0 {
		sb.WriteString("\tEvents *memory.InMemoryEventPublisher\n")
	}
	for _, uc := range archIR.UseCases {
		sb.WriteString(fmt.Sprintf("\t%s *usecases.%s\n", uc.ActionName, uc.Name))
	}
	sb.WriteString("\tHTTPServer *httpadapter.Server\n}\n\n")

	sb.WriteString("func NewTestContainer() *Container {\n")
	for _, p := range archIR.Ports {
		sb.WriteString(fmt.Sprintf("\t%sRepo := memory.NewInMemory%sRepository()\n", strings.ToLower(p.EntityName), p.EntityName))
	}
	if len(archIR.DomainEvents) > 0 {
		sb.WriteString("\tevents := memory.NewInMemoryEventPublisher()\n")
	}
	var srvArgs []string
	for _, uc := range archIR.UseCases {
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
	for _, p := range archIR.Ports {
		sb.WriteString(fmt.Sprintf("\t\t%sRepo: %sRepo,\n", p.EntityName, strings.ToLower(p.EntityName)))
	}
	if len(archIR.DomainEvents) > 0 {
		sb.WriteString("\t\tEvents: events,\n")
	}
	for _, uc := range archIR.UseCases {
		sb.WriteString(fmt.Sprintf("\t\t%s: %s,\n", uc.ActionName, strings.ToLower(uc.ActionName)+"UC"))
	}
	sb.WriteString("\t\tHTTPServer: srv,\n\t}\n}\n")

	return sb.String()
}
