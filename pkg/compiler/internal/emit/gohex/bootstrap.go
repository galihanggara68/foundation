package gohex

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/arch"
)

func (lay goLayout) emitGoBootstrap(archIR *arch.ArchitectureIR) string {
	var sb strings.Builder
	sb.WriteString("package bootstrap\n\nimport (\n")
	if len(archIR.DomainEvents) > 0 && len(archIR.EventHandlers) > 0 {
		sb.WriteString("\t\"context\"\n")
	}
	sb.WriteString("\t\"database/sql\"\n")
	sb.WriteString("\t\"github.com/galihanggara68/foundation/contrib/go/runtime\"\n")
	if len(archIR.DomainEvents) > 0 && len(archIR.EventHandlers) > 0 {
		sb.WriteString(fmt.Sprintf("\t\"%s/%s\"\n", archIR.Module, lay.DomainDir))
	}
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
	sb.WriteString("\truntimeDriver := runtime.NewMemoryDriver()\n\t_ = runtimeDriver\n")
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
		sb.WriteString("\t_ = events\n")
	}
	sb.WriteString(fmt.Sprintf("\ttxManager := %s.NewInMemoryTransactionManager(%s)\n\t_ = txManager\n",
		lay.MemoryPkg, strings.Join(snapRepos, ", ")))
	srvArgs := lay.emitGoUseCaseWiring(&sb, archIR)
	if len(archIR.DomainEvents) > 0 && len(archIR.EventHandlers) > 0 {
		for _, eh := range archIR.EventHandlers {
			actionVar := strings.ToLower(eh.ActionName) + "UC"
			sb.WriteString(fmt.Sprintf("\tevents.Subscribe(%q, func(ctx context.Context, payload any) error {\n", eh.EventName))
			sb.WriteString(fmt.Sprintf("\t\tev, ok := payload.(%s.%s)\n\t\tif !ok {\n\t\t\treturn nil\n\t\t}\n", lay.DomainPkg, eh.EventName))
			sb.WriteString(fmt.Sprintf("\t\t_, err := %s.Execute(ctx, ev)\n\t\treturn err\n", actionVar))
			sb.WriteString("\t})\n")
		}
	}
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
		if _, isIdem := uc.HasTrait("idempotent"); isIdem {
			depArgs = append(depArgs, "runtimeDriver.Idempotency()")
		}
		if _, isCache := uc.HasTrait("cache"); isCache {
			depArgs = append(depArgs, "runtimeDriver.Cache()")
		}
		if _, isOutbox := uc.HasTrait("outbox"); isOutbox {
			depArgs = append(depArgs, "runtimeDriver.Outbox()")
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
	sb.WriteString("\truntimeDriver := runtime.NewMemoryDriver()\n\t_ = runtimeDriver\n")
	for _, p := range archIR.Ports {
		rVar := strings.ToLower(p.EntityName) + "Repo"
		sb.WriteString(fmt.Sprintf("\t%s := %s.NewPostgres%sRepository(db)\n\t_ = %s\n", rVar, lay.PostgresPkg, p.EntityName, rVar))
	}
	for _, sl := range archIR.Slots {
		sb.WriteString(fmt.Sprintf("\t%sSlot := slots.New%sSlot()\n\t_ = %sSlot\n", strings.ToLower(sl.Name), sl.Name, strings.ToLower(sl.Name)))
	}
	if len(archIR.DomainEvents) > 0 {
		sb.WriteString(fmt.Sprintf("\tevents := %s.NewInMemoryEventPublisher()\n", lay.MemoryPkg))
		sb.WriteString("\t_ = events\n")
	}
	sb.WriteString(fmt.Sprintf("\ttxManager := %s.NewPostgresTransactionManager(db)\n\t_ = txManager\n", lay.PostgresPkg))
	srvArgs := lay.emitGoUseCaseWiring(&sb, archIR)
	if len(archIR.DomainEvents) > 0 && len(archIR.EventHandlers) > 0 {
		for _, eh := range archIR.EventHandlers {
			actionVar := strings.ToLower(eh.ActionName) + "UC"
			sb.WriteString(fmt.Sprintf("\tevents.Subscribe(%q, func(ctx context.Context, payload any) error {\n", eh.EventName))
			sb.WriteString(fmt.Sprintf("\t\tev, ok := payload.(%s.%s)\n\t\tif !ok {\n\t\t\treturn nil\n\t\t}\n", lay.DomainPkg, eh.EventName))
			sb.WriteString(fmt.Sprintf("\t\t_, err := %s.Execute(ctx, ev)\n\t\treturn err\n", actionVar))
			sb.WriteString("\t})\n")
		}
	}
	sb.WriteString(fmt.Sprintf("\tsrv := %s.NewServer(%s)\n", lay.HTTPPkg, strings.Join(srvArgs, ", ")))
	sb.WriteString("\treturn &PostgresContainer{\n")
	emitGoContainerStructFields(&sb, archIR)
	sb.WriteString("\t}\n}\n")
	return sb.String()
}
