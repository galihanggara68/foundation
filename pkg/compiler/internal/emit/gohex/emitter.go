package gohex

import (
	"fmt"
	"go/format"
	"strings"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/arch"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/emit"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
)

type goLayout struct {
	DomainPkg       string
	DomainDir       string
	PortsPkg        string
	PortsDir        string
	UseCasesPkg     string
	UseCasesDir     string
	UseCaseFileSuf  string
	MemoryPkg       string
	MemoryDir       string
	MemoryRepoSuf   string
	MemorySlotSuf   string
	MemoryEventFile string
	MemoryTxFile    string
	PostgresPkg     string
	PostgresDir     string
	PostgresRepoSuf string
	PostgresTxFile  string
	HTTPPkg         string
	HTTPDir         string
	AuthnPkg        string
	AuthnDir        string
	// UnifiedRepositories marks layouts whose memory/postgres adapters live in the
	// same package as the repository ports (layered); hexagonal keeps them separate.
	UnifiedRepositories bool
}

func hexagonalLayout() goLayout {
	return goLayout{
		DomainPkg:       "domain",
		DomainDir:       "internal/domain",
		PortsPkg:        "ports",
		PortsDir:        "internal/ports",
		UseCasesPkg:     "usecases",
		UseCasesDir:     "internal/usecases",
		UseCaseFileSuf:  "_usecase.go",
		MemoryPkg:       "memory",
		MemoryDir:       "internal/adapters/memory",
		MemoryRepoSuf:   "_repository.go",
		MemorySlotSuf:   "_slot.go",
		MemoryEventFile: "event_publisher.go",
		MemoryTxFile:    "transaction_manager.go",
		PostgresPkg:     "postgres",
		PostgresDir:     "internal/adapters/postgres",
		PostgresRepoSuf: "_repository.go",
		PostgresTxFile:  "transaction_manager.go",
		HTTPPkg:         "httpadapter",
		HTTPDir:         "internal/adapters/http",
		AuthnPkg:        "authn",
		AuthnDir:        "internal/adapters/authn",
	}
}

func layeredLayout() goLayout {
	return goLayout{
		DomainPkg:       "models",
		DomainDir:       "internal/models",
		PortsPkg:        "repositories",
		PortsDir:        "internal/repositories",
		UseCasesPkg:     "services",
		UseCasesDir:     "internal/services",
		UseCaseFileSuf:  "_service.go",
		MemoryPkg:       "repositories",
		MemoryDir:       "internal/repositories",
		MemoryRepoSuf:   "_memory.go",
		MemorySlotSuf:   "_fake.go",
		MemoryEventFile: "event_publisher_memory.go",
		MemoryTxFile:    "transaction_manager_memory.go",
		PostgresPkg:     "repositories",
		PostgresDir:     "internal/repositories",
		PostgresRepoSuf: "_postgres.go",
		PostgresTxFile:  "postgres_tx.go",
		HTTPPkg:         "handlers",
		HTTPDir:         "internal/handlers",
		AuthnPkg:        "authn",
		AuthnDir:        "internal/authn",

		UnifiedRepositories: true,
	}
}

// Emitter implements emit.TargetEmitter for Go Hexagonal Architecture.
type Emitter struct{}

func (Emitter) Emit(archIR *arch.ArchitectureIR) (emit.Emission, []ir.Diagnostic) {
	tracked, stubs, diags := emitWithLayout(archIR, hexagonalLayout())
	return emit.Emission{
		TrackedFiles: tracked,
		StubFiles:    stubs,
	}, diags
}

// LayeredEmitter implements emit.TargetEmitter for Go Layered Architecture.
type LayeredEmitter struct{}

func (LayeredEmitter) Emit(archIR *arch.ArchitectureIR) (emit.Emission, []ir.Diagnostic) {
	tracked, stubs, diags := emitWithLayout(archIR, layeredLayout())
	return emit.Emission{
		TrackedFiles: tracked,
		StubFiles:    stubs,
	}, diags
}

func emitWithLayout(archIR *arch.ArchitectureIR, lay goLayout) (map[string][]byte, map[string][]byte, []ir.Diagnostic) {
	files := make(map[string][]byte)
	stubs := make(map[string][]byte)

	contribGoPath := emit.FindContribGoRuntime()
	replaceClause := ""
	if contribGoPath != "" {
		replaceClause = fmt.Sprintf("\nreplace github.com/galihanggara68/foundation/contrib/go/runtime => %s\n", contribGoPath)
	}

	files["go.mod"] = []byte(fmt.Sprintf(`module %s

go 1.23

require (
	github.com/go-chi/chi/v5 v5.3.0
	github.com/galihanggara68/foundation/contrib/go/runtime v0.1.0
	github.com/lib/pq v1.10.9
)
%s`, archIR.Module, replaceClause))

	files["go.sum"] = []byte(`github.com/go-chi/chi/v5 v5.3.0 h1:halUjDxhshgXHMrao5bB8eNBXo/rnzwr8m5m36glehM=
github.com/go-chi/chi/v5 v5.3.0/go.mod h1:R+tYY2hNuVUUjxoPtqUdgBqevM9s9njzkTLutVsOCto=
github.com/lib/pq v1.10.9 h1:YXG7RB+JIjhP29X+OtkiDnYaXQwpS4JEWq7dtCCRUEw=
github.com/lib/pq v1.10.9/go.mod h1:AlVN5x4E4T544tWzH6hKfbfQvm3HdbOxrmggDNAPY9o=
`)

	if archIR.Storage.SchemaSQL != "" {
		files["schema.sql"] = []byte(archIR.Storage.SchemaSQL)
	}
	for _, mig := range archIR.Storage.Migrations {
		files[fmt.Sprintf("migrations/%s.up.sql", mig.Name)] = []byte(mig.UpSQL)
		files[fmt.Sprintf("migrations/%s.down.sql", mig.Name)] = []byte(mig.DownSQL)
	}

	addGoFile := func(relPath, src string) *ir.Diagnostic {
		code, err := formatGo(relPath, src)
		if err != nil {
			d := errDiag(relPath, err)
			return &d
		}
		files[relPath] = code
		return nil
	}

	if len(archIR.DomainEnums) > 0 {
		if d := addGoFile(lay.DomainDir+"/enums.go", lay.emitGoDomainEnums(archIR)); d != nil {
			return nil, nil, []ir.Diagnostic{*d}
		}
	}

	for _, e := range archIR.DomainEntities {
		relPath := fmt.Sprintf("%s/%s_entity.go", lay.DomainDir, strings.ToLower(e.Name))
		if d := addGoFile(relPath, lay.emitGoEntityFile(e)); d != nil {
			return nil, nil, []ir.Diagnostic{*d}
		}
	}

	if d := addGoFile(lay.DomainDir+"/decimal.go", emitGoDecimalDef(lay.DomainPkg)); d != nil {
		return nil, nil, []ir.Diagnostic{*d}
	}
	if d := addGoFile(lay.DomainDir+"/values.go", lay.emitGoDomainValues(archIR)); d != nil {
		return nil, nil, []ir.Diagnostic{*d}
	}
	if d := addGoFile(lay.DomainDir+"/errors.go", lay.emitGoDomainErrors(archIR)); d != nil {
		return nil, nil, []ir.Diagnostic{*d}
	}
	if len(archIR.DomainEvents) > 0 {
		if d := addGoFile(lay.DomainDir+"/events.go", lay.emitGoDomainEvents(archIR)); d != nil {
			return nil, nil, []ir.Diagnostic{*d}
		}
	}
	if len(archIR.DomainPolicies) > 0 {
		if d := addGoFile(lay.DomainDir+"/policies.go", lay.emitGoDomainPolicies(archIR)); d != nil {
			return nil, nil, []ir.Diagnostic{*d}
		}
	}

	for _, p := range archIR.Ports {
		relPath := fmt.Sprintf("%s/%s_repository.go", lay.PortsDir, strings.ToLower(p.EntityName))
		if d := addGoFile(relPath, lay.emitGoSinglePort(archIR.Module, p)); d != nil {
			return nil, nil, []ir.Diagnostic{*d}
		}
	}

	if archIR.HasActorPort {
		if d := addGoFile(lay.PortsDir+"/actor_resolver.go", lay.emitGoActorResolverPort(archIR.Module)); d != nil {
			return nil, nil, []ir.Diagnostic{*d}
		}
		if d := addGoFile(lay.AuthnDir+"/authn_adapter.go", lay.emitGoAuthnAdapters(archIR)); d != nil {
			return nil, nil, []ir.Diagnostic{*d}
		}
	}

	for _, sl := range archIR.Slots {
		portPath := fmt.Sprintf("%s/%s_slot.go", lay.PortsDir, strings.ToLower(sl.Name))
		if d := addGoFile(portPath, lay.emitGoSlotPort(archIR.Module, sl)); d != nil {
			return nil, nil, []ir.Diagnostic{*d}
		}

		stubPath := fmt.Sprintf("internal/slots/%s_stub.go", strings.ToLower(sl.Name))
		stubCode, err := formatGo(stubPath, lay.emitGoSlotStub(sl))
		if err != nil {
			return nil, nil, []ir.Diagnostic{errDiag(stubPath, err)}
		}
		stubs[stubPath] = stubCode

		fakePath := fmt.Sprintf("%s/%s%s", lay.MemoryDir, strings.ToLower(sl.Name), lay.MemorySlotSuf)
		if d := addGoFile(fakePath, lay.emitGoMemorySlotFake(archIR.Module, sl)); d != nil {
			return nil, nil, []ir.Diagnostic{*d}
		}
	}

	if len(archIR.Slots) > 0 {
		if d := addGoFile("internal/slots/contracts.go", lay.emitGoSlotTypeAliases(archIR)); d != nil {
			return nil, nil, []ir.Diagnostic{*d}
		}
		if d := addGoFile("contracts_test.go", lay.emitGoContractsTest(archIR)); d != nil {
			return nil, nil, []ir.Diagnostic{*d}
		}
	}

	if len(archIR.DomainEvents) > 0 {
		if d := addGoFile(lay.PortsDir+"/event_publisher.go", lay.emitGoEventPublisherPort()); d != nil {
			return nil, nil, []ir.Diagnostic{*d}
		}
		if d := addGoFile(lay.MemoryDir+"/"+lay.MemoryEventFile, lay.emitGoMemoryEventPublisher()); d != nil {
			return nil, nil, []ir.Diagnostic{*d}
		}
	}

	if d := addGoFile(lay.PortsDir+"/transaction_manager.go", lay.emitGoTransactionManagerPort()); d != nil {
		return nil, nil, []ir.Diagnostic{*d}
	}
	if d := addGoFile(lay.MemoryDir+"/"+lay.MemoryTxFile, lay.emitGoMemoryTransactionManager()); d != nil {
		return nil, nil, []ir.Diagnostic{*d}
	}
	if d := addGoFile(lay.PostgresDir+"/"+lay.PostgresTxFile, lay.emitGoPostgresTxManager()); d != nil {
		return nil, nil, []ir.Diagnostic{*d}
	}

	for _, uc := range archIR.UseCases {
		relPath := fmt.Sprintf("%s/%s%s", lay.UseCasesDir, strings.ToLower(uc.ActionName), lay.UseCaseFileSuf)
		if d := addGoFile(relPath, lay.emitGoSingleUseCase(archIR, uc)); d != nil {
			return nil, nil, []ir.Diagnostic{*d}
		}
	}

	hasFindMany := false
	for _, p := range archIR.Ports {
		for _, m := range p.Methods {
			if m.Kind == arch.RepoMethodFindMany {
				hasFindMany = true
				break
			}
		}
	}
	if hasFindMany {
		if d := addGoFile(lay.MemoryDir+"/cursor.go", lay.emitGoCursorHelpers(lay.MemoryPkg)); d != nil {
			return nil, nil, []ir.Diagnostic{*d}
		}
		if d := addGoFile(lay.PostgresDir+"/cursor.go", lay.emitGoCursorHelpers(lay.PostgresPkg)); d != nil {
			return nil, nil, []ir.Diagnostic{*d}
		}
	}

	for _, p := range archIR.Ports {
		memPath := fmt.Sprintf("%s/%s%s", lay.MemoryDir, strings.ToLower(p.EntityName), lay.MemoryRepoSuf)
		if d := addGoFile(memPath, lay.emitGoSingleMemoryRepo(archIR, p)); d != nil {
			return nil, nil, []ir.Diagnostic{*d}
		}
		pgPath := fmt.Sprintf("%s/%s%s", lay.PostgresDir, strings.ToLower(p.EntityName), lay.PostgresRepoSuf)
		if d := addGoFile(pgPath, lay.emitGoSinglePostgresRepo(archIR, p)); d != nil {
			return nil, nil, []ir.Diagnostic{*d}
		}
	}

	if d := addGoFile(lay.HTTPDir+"/router.go", lay.emitGoChiRouter(archIR)); d != nil {
		return nil, nil, []ir.Diagnostic{*d}
	}
	if d := addGoFile("internal/bootstrap/container.go", lay.emitGoBootstrap(archIR)); d != nil {
		return nil, nil, []ir.Diagnostic{*d}
	}
	if d := addGoFile("cmd/server/main.go", emitGoServer(archIR)); d != nil {
		return nil, nil, []ir.Diagnostic{*d}
	}
	if d := addGoFile("scenarios_test.go", lay.emitGoScenariosTest(archIR)); d != nil {
		return nil, nil, []ir.Diagnostic{*d}
	}

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

func (lay goLayout) goType(fdnType string, inDomainPkg bool) string {
	if strings.HasPrefix(fdnType, "[]") {
		return "[]" + lay.goType(strings.TrimPrefix(fdnType, "[]"), inDomainPkg)
	}
	switch fdnType {
	case "uuid", "string":
		return "string"
	case "timestamp":
		return "time.Time"
	case "int":
		return "int"
	case "decimal":
		if inDomainPkg {
			return "Decimal"
		}
		return lay.DomainPkg + ".Decimal"
	case "bool":
		return "bool"
	default:
		return lay.goNamedType(fdnType, inDomainPkg)
	}
}

// goKindType maps a compiler-validated FieldKind to its Go type. Array-ness is read
// from the declared type string; a field's Kind is the kind of its element type.
func (lay goLayout) goKindType(kind ir.FieldKind, declaredType string, inDomainPkg bool) string {
	if strings.HasPrefix(declaredType, "[]") {
		return "[]" + lay.goKindType(kind, strings.TrimPrefix(declaredType, "[]"), inDomainPkg)
	}
	switch kind {
	case ir.KindUUID, ir.KindString:
		return "string"
	case ir.KindTimestamp:
		return "time.Time"
	case ir.KindInt:
		return "int"
	case ir.KindDecimal:
		if inDomainPkg {
			return "Decimal"
		}
		return lay.DomainPkg + ".Decimal"
	case ir.KindBool:
		return "bool"
	default:
		return lay.goNamedType(declaredType, inDomainPkg)
	}
}

// isScalarKind reports whether a compiler-validated FieldKind is a primitive
// scalar, i.e. one that maps to a built-in Go type rather than a domain type.
func isScalarKind(kind ir.FieldKind) bool {
	switch kind {
	case ir.KindUUID, ir.KindString, ir.KindInt, ir.KindDecimal, ir.KindBool, ir.KindTimestamp:
		return true
	default:
		return false
	}
}

// isArrayType reports whether a declared type is an array. Array-ness is carried on
// the declared type string; a FieldKind is only the element kind.
func isArrayType(declaredType string) bool {
	return strings.HasPrefix(declaredType, "[]")
}

// isScalarParam reports whether a param is a single built-in scalar. Arrays, enums,
// and declared types are not scalars.
func isScalarParam(p ir.ParamDecl) bool {
	return !isArrayType(p.Type) && isScalarKind(p.Kind)
}

// goNamedType renders a declared type as a reference to a type in the domain package.
func (lay goLayout) goNamedType(name string, inDomainPkg bool) string {
	if inDomainPkg {
		return name
	}
	return lay.DomainPkg + "." + name
}
