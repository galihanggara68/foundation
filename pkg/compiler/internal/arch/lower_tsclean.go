package arch

import "github.com/galihanggara68/foundation/pkg/compiler/internal/ir"

// TSCleanLowerer lowers SemanticIR into a TypeScript Clean Architecture (entities, usecases, controllers, gateways, slots, main) ArchitectureIR.
type TSCleanLowerer struct{}

func (TSCleanLowerer) Lower(sem *ir.SemanticIR, bp *BuildProfile, target TargetConfig) (*ArchitectureIR, []ir.Diagnostic) {
	layers := []ArchLayer{
		{Name: "entities", Package: "src/entities"},
		{Name: "usecases", Package: "src/usecases"},
		{Name: "controllers", Package: "src/controllers"},
		{Name: "gateways", Package: "src/gateways"},
		{Name: "slots", Package: "src/slots"},
		{Name: "main", Package: "src/main"},
	}
	return lowerBackendCommon(sem, bp, target, layers, "UseCase"), nil
}
