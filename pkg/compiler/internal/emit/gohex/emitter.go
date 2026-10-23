package gohex

import (
	"fmt"
	"go/format"
	"strings"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/arch"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/emit"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
)

// Emitter implements emit.TargetEmitter for Go Hexagonal Architecture.
type Emitter struct{}

func (Emitter) Emit(archIR *arch.ArchitectureIR) (emit.Emission, []ir.Diagnostic) {
	tracked, stubs, diags := Emit(archIR)
	return emit.Emission{
		TrackedFiles: tracked,
		StubFiles:    stubs,
	}, diags
}

// Emit synthesizes a complete Go Hexagonal Architecture workspace from the lowered ArchitectureIR.
func Emit(archIR *arch.ArchitectureIR) (map[string][]byte, map[string][]byte, []ir.Diagnostic) {
	files := make(map[string][]byte)
	stubs := make(map[string][]byte)

	files["go.mod"] = []byte(fmt.Sprintf(`module %s

go 1.23

require github.com/go-chi/chi/v5 v5.3.0
`, archIR.Module))

	files["go.sum"] = []byte(`github.com/go-chi/chi/v5 v5.3.0 h1:halUjDxhshgXHMrao5bB8eNBXo/rnzwr8m5m36glehM=
github.com/go-chi/chi/v5 v5.3.0/go.mod h1:R+tYY2hNuVUUjxoPtqUdgBqevM9s9njzkTLutVsOCto=
`)

	if archIR.Storage.SchemaSQL != "" {
		files["schema.sql"] = []byte(archIR.Storage.SchemaSQL)
	}
	for _, mig := range archIR.Storage.Migrations {
		files[fmt.Sprintf("migrations/%s.up.sql", mig.Name)] = []byte(mig.UpSQL)
		files[fmt.Sprintf("migrations/%s.down.sql", mig.Name)] = []byte(mig.DownSQL)
	}

	if len(archIR.DomainEnums) > 0 {
		enumsCode, err := formatGo("internal/domain/enums.go", emitGoDomainEnums(archIR))
		if err != nil {
			return nil, nil, []ir.Diagnostic{errDiag("internal/domain/enums.go", err)}
		}
		files["internal/domain/enums.go"] = enumsCode
	}

	for _, e := range archIR.DomainEntities {
		relPath := fmt.Sprintf("internal/domain/%s_entity.go", strings.ToLower(e.Name))
		code, err := formatGo(relPath, "package domain\n\n"+emitGoStructDef(e.Name, e.Fields))
		if err != nil {
			return nil, nil, []ir.Diagnostic{errDiag(relPath, err)}
		}
		files[relPath] = code
	}

	domainValues, err := formatGo("internal/domain/values.go", emitGoDomainValues(archIR))
	if err != nil {
		return nil, nil, []ir.Diagnostic{errDiag("internal/domain/values.go", err)}
	}
	files["internal/domain/values.go"] = domainValues

	domainErrors, err := formatGo("internal/domain/errors.go", emitGoDomainErrors(archIR))
	if err != nil {
		return nil, nil, []ir.Diagnostic{errDiag("internal/domain/errors.go", err)}
	}
	files["internal/domain/errors.go"] = domainErrors

	if len(archIR.DomainEvents) > 0 {
		eventsCode, err := formatGo("internal/domain/events.go", emitGoDomainEvents(archIR))
		if err != nil {
			return nil, nil, []ir.Diagnostic{errDiag("internal/domain/events.go", err)}
		}
		files["internal/domain/events.go"] = eventsCode
	}

	if len(archIR.DomainPolicies) > 0 {
		policiesCode, err := formatGo("internal/domain/policies.go", emitGoDomainPolicies(archIR))
		if err != nil {
			return nil, nil, []ir.Diagnostic{errDiag("internal/domain/policies.go", err)}
		}
		files["internal/domain/policies.go"] = policiesCode
	}

	for _, p := range archIR.Ports {
		relPath := fmt.Sprintf("internal/ports/%s_repository.go", strings.ToLower(p.EntityName))
		code, err := formatGo(relPath, emitGoSinglePort(archIR.Module, p))
		if err != nil {
			return nil, nil, []ir.Diagnostic{errDiag(relPath, err)}
		}
		files[relPath] = code
	}

	for _, sl := range archIR.Slots {
		portPath := fmt.Sprintf("internal/ports/%s_slot.go", strings.ToLower(sl.Name))
		portCode, err := formatGo(portPath, emitGoSlotPort(archIR.Module, sl))
		if err != nil {
			return nil, nil, []ir.Diagnostic{errDiag(portPath, err)}
		}
		files[portPath] = portCode

		stubPath := fmt.Sprintf("internal/slots/%s_stub.go", strings.ToLower(sl.Name))
		stubCode, err := formatGo(stubPath, emitGoSlotStub(archIR.Module, sl))
		if err != nil {
			return nil, nil, []ir.Diagnostic{errDiag(stubPath, err)}
		}
		stubs[stubPath] = stubCode

		fakePath := fmt.Sprintf("internal/adapters/memory/%s_slot.go", strings.ToLower(sl.Name))
		fakeCode, err := formatGo(fakePath, emitGoMemorySlotFake(archIR.Module, sl))
		if err != nil {
			return nil, nil, []ir.Diagnostic{errDiag(fakePath, err)}
		}
		files[fakePath] = fakeCode
	}

	if len(archIR.Slots) > 0 {
		contractsCode, err := formatGo("contracts_test.go", emitGoContractsTest(archIR))
		if err != nil {
			return nil, nil, []ir.Diagnostic{errDiag("contracts_test.go", err)}
		}
		files["contracts_test.go"] = contractsCode
	}

	if len(archIR.DomainEvents) > 0 {
		evPortCode, err := formatGo("internal/ports/event_publisher.go", emitGoEventPublisherPort())
		if err != nil {
			return nil, nil, []ir.Diagnostic{errDiag("internal/ports/event_publisher.go", err)}
		}
		files["internal/ports/event_publisher.go"] = evPortCode
	}

	txPortCode, err := formatGo("internal/ports/transaction_manager.go", emitGoTransactionManagerPort())
	if err != nil {
		return nil, nil, []ir.Diagnostic{errDiag("internal/ports/transaction_manager.go", err)}
	}
	files["internal/ports/transaction_manager.go"] = txPortCode

	for _, uc := range archIR.UseCases {
		relPath := fmt.Sprintf("internal/usecases/%s_usecase.go", strings.ToLower(uc.ActionName))
		code, err := formatGo(relPath, emitGoSingleUseCase(archIR.Module, uc))
		if err != nil {
			return nil, nil, []ir.Diagnostic{errDiag(relPath, err)}
		}
		files[relPath] = code
	}

	for _, p := range archIR.Ports {
		relPath := fmt.Sprintf("internal/adapters/memory/%s_repository.go", strings.ToLower(p.EntityName))
		code, err := formatGo(relPath, emitGoSingleMemoryRepo(archIR, p))
		if err != nil {
			return nil, nil, []ir.Diagnostic{errDiag(relPath, err)}
		}
		files[relPath] = code

		pgPath := fmt.Sprintf("internal/adapters/postgres/%s_repository.go", strings.ToLower(p.EntityName))
		pgCode, err := formatGo(pgPath, emitGoSinglePostgresRepo(archIR, p))
		if err != nil {
			return nil, nil, []ir.Diagnostic{errDiag(pgPath, err)}
		}
		files[pgPath] = pgCode
	}

	txMemCode, err := formatGo("internal/adapters/memory/transaction_manager.go", emitGoMemoryTransactionManager())
	if err != nil {
		return nil, nil, []ir.Diagnostic{errDiag("internal/adapters/memory/transaction_manager.go", err)}
	}
	files["internal/adapters/memory/transaction_manager.go"] = txMemCode

	txPgCode, err := formatGo("internal/adapters/postgres/transaction_manager.go", emitGoPostgresTxManager())
	if err != nil {
		return nil, nil, []ir.Diagnostic{errDiag("internal/adapters/postgres/transaction_manager.go", err)}
	}
	files["internal/adapters/postgres/transaction_manager.go"] = txPgCode

	if len(archIR.DomainEvents) > 0 {
		evMemCode, err := formatGo("internal/adapters/memory/event_publisher.go", emitGoMemoryEventPublisher())
		if err != nil {
			return nil, nil, []ir.Diagnostic{errDiag("internal/adapters/memory/event_publisher.go", err)}
		}
		files["internal/adapters/memory/event_publisher.go"] = evMemCode
	}

	httpRouterCode, err := formatGo("internal/adapters/http/router.go", emitGoChiRouter(archIR))
	if err != nil {
		return nil, nil, []ir.Diagnostic{errDiag("internal/adapters/http/router.go", err)}
	}
	files["internal/adapters/http/router.go"] = httpRouterCode

	bootstrapCode, err := formatGo("internal/bootstrap/container.go", emitGoBootstrap(archIR))
	if err != nil {
		return nil, nil, []ir.Diagnostic{errDiag("internal/bootstrap/container.go", err)}
	}
	files["internal/bootstrap/container.go"] = bootstrapCode

	scenariosCode, err := formatGo("scenarios_test.go", emitGoScenariosTest(archIR))
	if err != nil {
		return nil, nil, []ir.Diagnostic{errDiag("scenarios_test.go", err)}
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

func errDiag(file string, err error) ir.Diagnostic {
	return ir.Diagnostic{
		Code:     "FDN0500",
		Severity: ir.SeverityError,
		Message:  err.Error(),
		Span:     ir.Span{File: file, Line: 1, Column: 1},
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
