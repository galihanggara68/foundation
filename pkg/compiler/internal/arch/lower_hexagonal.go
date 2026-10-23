package arch

import "github.com/galihanggara68/foundation/pkg/compiler/internal/ir"

// GoHexagonalLowerer lowers SemanticIR into a Hexagonal (Ports & Adapters) ArchitectureIR.
type GoHexagonalLowerer struct{}

func (GoHexagonalLowerer) Lower(sem *ir.SemanticIR, bp *BuildProfile, target TargetConfig) (*ArchitectureIR, []ir.Diagnostic) {
	layers := []ArchLayer{
		{Name: "domain", Package: "internal/domain"},
		{Name: "ports", Package: "internal/ports"},
		{Name: "usecases", Package: "internal/usecases"},
		{Name: "adapters_http", Package: "internal/adapters/http"},
		{Name: "adapters_memory", Package: "internal/adapters/memory"},
		{Name: "adapters_postgres", Package: "internal/adapters/postgres"},
		{Name: "slots", Package: "internal/slots"},
		{Name: "bootstrap", Package: "internal/bootstrap"},
	}
	return lowerBackendCommon(sem, bp, target, layers, "UseCase"), nil
}
