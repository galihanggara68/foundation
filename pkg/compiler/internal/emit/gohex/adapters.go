package gohex

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/arch"
)

func (lay goLayout) emitGoMemoryEventPublisher() string {
	return fmt.Sprintf(`package %s

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
`, lay.MemoryPkg)
}

func (lay goLayout) emitGoMemoryTransactionManager() string {
	return fmt.Sprintf(`package %s

import (
	"context"
	"sync"
)

type Snapshotter interface {
	snapshot() func()
}

type InMemoryTransactionManager struct {
	mu    sync.Mutex
	repos []Snapshotter
}

func NewInMemoryTransactionManager(repos ...Snapshotter) *InMemoryTransactionManager {
	return &InMemoryTransactionManager{repos: repos}
}

func (tm *InMemoryTransactionManager) WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	tm.mu.Lock()
	var restores []func()
	for _, r := range tm.repos {
		restores = append(restores, r.snapshot())
	}
	tm.mu.Unlock()

	err := fn(ctx)
	if err != nil {
		tm.mu.Lock()
		for _, restore := range restores {
			restore()
		}
		tm.mu.Unlock()
		return err
	}
	return nil
}
`, lay.MemoryPkg)
}

func (lay goLayout) emitGoSingleMemoryRepo(archIR *arch.ArchitectureIR, p arch.RepositoryPortIR) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`package %s

import (
	"context"
	"fmt"
	"sync"
	"%s/%s"
)

`, lay.MemoryPkg, archIR.Module, lay.DomainDir))

	repoStruct := "InMemory" + p.EntityName + "Repository"
	sb.WriteString(fmt.Sprintf("type %s struct {\n\tmu sync.RWMutex\n\tentities []%s.%s\n\tseq int\n}\n\n", repoStruct, lay.DomainPkg, p.EntityName))
	sb.WriteString(fmt.Sprintf("func New%s() *%s {\n\treturn &%s{}\n}\n\n", repoStruct, repoStruct, repoStruct))
	sb.WriteString(fmt.Sprintf("func (r *%s) Seed(entity %s.%s) {\n\tr.mu.Lock()\n\tdefer r.mu.Unlock()\n\tr.entities = append(r.entities, entity)\n}\n\n", repoStruct, lay.DomainPkg, p.EntityName))
	sb.WriteString(fmt.Sprintf("func (r *%s) snapshot() func() {\n\tr.mu.RLock()\n\tsaved := append([]%s.%s(nil), r.entities...)\n\tsavedSeq := r.seq\n\tr.mu.RUnlock()\n\treturn func() {\n\t\tr.mu.Lock()\n\t\tr.entities = saved\n\t\tr.seq = savedSeq\n\t\tr.mu.Unlock()\n\t}\n}\n\n", repoStruct, lay.DomainPkg, p.EntityName))

	ent := archIR.EntityByName(p.EntityName)
	hasID := false
	for _, f := range ent.Fields {
		if f.Name == "id" && f.Type == "uuid" {
			hasID = true
		}
	}

	for _, m := range p.Methods {
		switch m.Kind {
		case arch.RepoMethodCreate:
			sb.WriteString(fmt.Sprintf("func (r *%s) Create(_ context.Context, entity %s.%s) (%s.%s, error) {\n",
				repoStruct, lay.DomainPkg, p.EntityName, lay.DomainPkg, p.EntityName))
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
					sb.WriteString(fmt.Sprintf("\tif entity.%s == \"\" {\n\t\tentity.%s = %s.%s%s\n\t}\n",
						fExport, fExport, lay.DomainPkg, f.Type, arch.ExportIdent(variant)))
				}
			}
			sb.WriteString("\tr.entities = append(r.entities, entity)\n\treturn entity, nil\n}\n\n")

		case arch.RepoMethodUpdate:
			sb.WriteString(fmt.Sprintf("func (r *%s) Update(_ context.Context, entity %s.%s) error {\n", repoStruct, lay.DomainPkg, p.EntityName))
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
				repoStruct, m.Name, m.WhereField, lay.goType(m.WhereType, false)))
			sb.WriteString("\tr.mu.Lock()\n\tdefer r.mu.Unlock()\n")
			sb.WriteString(fmt.Sprintf("\tvar kept []%s.%s\n", lay.DomainPkg, p.EntityName))
			sb.WriteString("\tfor _, item := range r.entities {\n")
			sb.WriteString(fmt.Sprintf("\t\tif item.%s != %s {\n\t\t\tkept = append(kept, item)\n\t\t}\n", fieldExport, m.WhereField))
			sb.WriteString("\t}\n\tr.entities = kept\n\treturn nil\n}\n\n")

		case arch.RepoMethodFind:
			fieldExport := arch.ExportIdent(m.WhereField)
			sb.WriteString(fmt.Sprintf("func (r *%s) %s(_ context.Context, %s %s) (*%s.%s, error) {\n",
				repoStruct, m.Name, m.WhereField, lay.goType(m.WhereType, false), lay.DomainPkg, p.EntityName))
			sb.WriteString("\tr.mu.RLock()\n\tdefer r.mu.RUnlock()\n")
			sb.WriteString("\tfor _, item := range r.entities {\n")
			sb.WriteString(fmt.Sprintf("\t\tif item.%s == %s {\n\t\t\tcopyItem := item\n\t\t\treturn &copyItem, nil\n\t\t}\n", fieldExport, m.WhereField))
			sb.WriteString("\t}\n\treturn nil, nil\n}\n\n")
		}
	}

	return sb.String()
}

func (lay goLayout) emitGoChiRouter(archIR *arch.ArchitectureIR) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`package %s

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"%s/%s"
	"%s/%s"
)

type Server struct {
	router *chi.Mux
`, lay.HTTPPkg, archIR.Module, lay.DomainDir, archIR.Module, lay.UseCasesDir))

	for _, uc := range archIR.UseCases {
		sb.WriteString(fmt.Sprintf("\t%s *%s.%s\n", uc.ActionName, lay.UseCasesPkg, uc.Name))
	}
	sb.WriteString("}\n\n")

	var ctorArgs []string
	for _, uc := range archIR.UseCases {
		ctorArgs = append(ctorArgs, fmt.Sprintf("%s *%s.%s", strings.ToLower(uc.ActionName), lay.UseCasesPkg, uc.Name))
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
		sb.WriteString(fmt.Sprintf("\tctx := %s.WithActor(r.Context(), %s.Actor{\n\t\tID: r.Header.Get(\"X-Actor-ID\"),\n\t\tRole: r.Header.Get(\"X-Actor-Role\"),\n\t\tIsVerified: r.Header.Get(\"X-Actor-Verified\") == \"true\",\n\t})\n",
			lay.DomainPkg, lay.DomainPkg))
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
				sb.WriteString(fmt.Sprintf("\tvar %s %s\n", p.Name, lay.goType(p.Type, false)))
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

	sb.WriteString(fmt.Sprintf(`func writeError(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "application/json")
	var domErr *%s.DomainError
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
`, lay.DomainPkg))

	return sb.String()
}

func (lay goLayout) emitGoBootstrap(archIR *arch.ArchitectureIR) string {
	var sb strings.Builder
	sb.WriteString("package bootstrap\n\nimport (\n\t\"database/sql\"\n")
	// Alias the HTTP import only when its package name cannot be inferred from the import path.
	if filepath.Base(lay.HTTPDir) != lay.HTTPPkg {
		sb.WriteString(fmt.Sprintf("\t%s \"%s/%s\"\n", lay.HTTPPkg, archIR.Module, lay.HTTPDir))
	} else {
		sb.WriteString(fmt.Sprintf("\t\"%s/%s\"\n", archIR.Module, lay.HTTPDir))
	}
	seenImports := map[string]bool{lay.HTTPDir: true}
	for _, dir := range []string{lay.MemoryDir, lay.PostgresDir, lay.PortsDir} {
		if !seenImports[dir] {
			seenImports[dir] = true
			sb.WriteString(fmt.Sprintf("\t\"%s/%s\"\n", archIR.Module, dir))
		}
	}
	if len(archIR.Slots) > 0 {
		sb.WriteString(fmt.Sprintf("\t\"%s/internal/slots\"\n", archIR.Module))
	}
	sb.WriteString(fmt.Sprintf("\t\"%s/%s\"\n", archIR.Module, lay.UseCasesDir))
	sb.WriteString(")\n\ntype Container struct {\n")

	for _, p := range archIR.Ports {
		sb.WriteString(fmt.Sprintf("\t%sRepo *%s.InMemory%sRepository\n", p.EntityName, lay.MemoryPkg, p.EntityName))
	}
	for _, sl := range archIR.Slots {
		sb.WriteString(fmt.Sprintf("\t%sSlot *%s.Fake%sSlot\n", sl.Name, lay.MemoryPkg, sl.Name))
	}
	if len(archIR.DomainEvents) > 0 {
		sb.WriteString(fmt.Sprintf("\tEvents *%s.InMemoryEventPublisher\n", lay.MemoryPkg))
	}
	sb.WriteString(fmt.Sprintf("\tTxManager *%s.InMemoryTransactionManager\n", lay.MemoryPkg))
	for _, uc := range archIR.UseCases {
		sb.WriteString(fmt.Sprintf("\t%s *%s.%s\n", uc.ActionName, lay.UseCasesPkg, uc.Name))
	}
	sb.WriteString(fmt.Sprintf("\tHTTPServer *%s.Server\n}\n\n", lay.HTTPPkg))

	sb.WriteString("type PostgresContainer struct {\n")
	for _, p := range archIR.Ports {
		sb.WriteString(fmt.Sprintf("\t%sRepo %s.%sRepository\n", p.EntityName, lay.PortsPkg, p.EntityName))
	}
	for _, sl := range archIR.Slots {
		sb.WriteString(fmt.Sprintf("\t%sSlot %s.%sSlot\n", sl.Name, lay.PortsPkg, sl.Name))
	}
	if len(archIR.DomainEvents) > 0 {
		sb.WriteString(fmt.Sprintf("\tEvents %s.EventPublisher\n", lay.PortsPkg))
	}
	sb.WriteString(fmt.Sprintf("\tTxManager %s.TransactionManager\n", lay.PortsPkg))
	for _, uc := range archIR.UseCases {
		sb.WriteString(fmt.Sprintf("\t%s *%s.%s\n", uc.ActionName, lay.UseCasesPkg, uc.Name))
	}
	sb.WriteString(fmt.Sprintf("\tHTTPServer *%s.Server\n}\n\n", lay.HTTPPkg))

	sb.WriteString("func NewTestContainer() *Container {\n")
	var snapRepos []string
	for _, p := range archIR.Ports {
		rVar := strings.ToLower(p.EntityName) + "Repo"
		sb.WriteString(fmt.Sprintf("\t%s := %s.NewInMemory%sRepository()\n", rVar, lay.MemoryPkg, p.EntityName))
		snapRepos = append(snapRepos, rVar)
	}
	for _, sl := range archIR.Slots {
		sb.WriteString(fmt.Sprintf("\t%sSlot := %s.NewFake%sSlot(slots.New%sSlot())\n",
			strings.ToLower(sl.Name), lay.MemoryPkg, sl.Name, sl.Name))
	}
	if len(archIR.DomainEvents) > 0 {
		sb.WriteString(fmt.Sprintf("\tevents := %s.NewInMemoryEventPublisher()\n", lay.MemoryPkg))
	}
	sb.WriteString(fmt.Sprintf("\ttxManager := %s.NewInMemoryTransactionManager(%s)\n\t_ = txManager\n",
		lay.MemoryPkg, strings.Join(snapRepos, ", ")))
	srvArgs := lay.emitGoUseCaseWiring(&sb, archIR)
	sb.WriteString(fmt.Sprintf("\tsrv := %s.NewServer(%s)\n", lay.HTTPPkg, strings.Join(srvArgs, ", ")))
	sb.WriteString("\treturn &Container{\n")
	emitGoContainerStructFields(&sb, archIR)
	sb.WriteString("\t}\n}\n\n")
	sb.WriteString(lay.emitGoPostgresBootstrapFunc(archIR))

	return sb.String()
}

func (lay goLayout) emitGoUseCaseWiring(sb *strings.Builder, archIR *arch.ArchitectureIR) []string {
	var srvArgs []string
	for _, uc := range archIR.UseCases {
		var depArgs []string
		for _, dep := range uc.RepoDeps {
			depArgs = append(depArgs, strings.ToLower(dep)+"Repo")
		}
		for _, sDep := range uc.SlotDeps {
			depArgs = append(depArgs, strings.ToLower(sDep)+"Slot")
		}
		if uc.EmitsEvents {
			depArgs = append(depArgs, "events")
		}
		if _, isTx := uc.HasTrait("transactional"); isTx {
			depArgs = append(depArgs, "txManager")
		}
		varName := strings.ToLower(uc.ActionName) + "UC"
		sb.WriteString(fmt.Sprintf("\t%s := %s.New%s(%s)\n", varName, lay.UseCasesPkg, uc.Name, strings.Join(depArgs, ", ")))
		srvArgs = append(srvArgs, varName)
	}
	return srvArgs
}

func emitGoContainerStructFields(sb *strings.Builder, archIR *arch.ArchitectureIR) {
	for _, p := range archIR.Ports {
		sb.WriteString(fmt.Sprintf("\t\t%sRepo: %sRepo,\n", p.EntityName, strings.ToLower(p.EntityName)))
	}
	for _, sl := range archIR.Slots {
		sb.WriteString(fmt.Sprintf("\t\t%sSlot: %sSlot,\n", sl.Name, strings.ToLower(sl.Name)))
	}
	if len(archIR.DomainEvents) > 0 {
		sb.WriteString("\t\tEvents: events,\n")
	}
	sb.WriteString("\t\tTxManager: txManager,\n")
	for _, uc := range archIR.UseCases {
		sb.WriteString(fmt.Sprintf("\t\t%s: %s,\n", uc.ActionName, strings.ToLower(uc.ActionName)+"UC"))
	}
	sb.WriteString("\t\tHTTPServer: srv,\n")
}

func (lay goLayout) emitGoPostgresBootstrapFunc(archIR *arch.ArchitectureIR) string {
	var sb strings.Builder
	sb.WriteString("func NewPostgresContainer(db *sql.DB) *PostgresContainer {\n")
	for _, p := range archIR.Ports {
		rVar := strings.ToLower(p.EntityName) + "Repo"
		sb.WriteString(fmt.Sprintf("\t%s := %s.NewPostgres%sRepository(db)\n\t_ = %s\n", rVar, lay.PostgresPkg, p.EntityName, rVar))
	}
	for _, sl := range archIR.Slots {
		sb.WriteString(fmt.Sprintf("\t%sSlot := slots.New%sSlot()\n\t_ = %sSlot\n", strings.ToLower(sl.Name), sl.Name, strings.ToLower(sl.Name)))
	}
	if len(archIR.DomainEvents) > 0 {
		sb.WriteString(fmt.Sprintf("\tevents := %s.NewInMemoryEventPublisher()\n", lay.MemoryPkg))
	}
	sb.WriteString(fmt.Sprintf("\ttxManager := %s.NewPostgresTransactionManager(db)\n\t_ = txManager\n", lay.PostgresPkg))
	srvArgs := lay.emitGoUseCaseWiring(&sb, archIR)
	sb.WriteString(fmt.Sprintf("\tsrv := %s.NewServer(%s)\n", lay.HTTPPkg, strings.Join(srvArgs, ", ")))
	sb.WriteString("\treturn &PostgresContainer{\n")
	emitGoContainerStructFields(&sb, archIR)
	sb.WriteString("\t}\n}\n")
	return sb.String()
}
