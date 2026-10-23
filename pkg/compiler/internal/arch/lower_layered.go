package arch

import "github.com/galihanggara68/foundation/pkg/compiler/internal/ir"

// GoLayeredLowerer lowers SemanticIR into a flattened Layered (handlers -> services -> repositories -> models) ArchitectureIR.
type GoLayeredLowerer struct{}

func (GoLayeredLowerer) Lower(sem *ir.SemanticIR, bp *BuildProfile, target TargetConfig) (*ArchitectureIR, []ir.Diagnostic) {
	layers := []ArchLayer{
		{Name: "models", Package: "internal/models"},
		{Name: "repositories", Package: "internal/repositories"},
		{Name: "services", Package: "internal/services"},
		{Name: "handlers", Package: "internal/handlers"},
		{Name: "slots", Package: "internal/slots"},
		{Name: "bootstrap", Package: "internal/bootstrap"},
	}
	return lowerBackendCommon(sem, bp, target, layers, "Service"), nil
}
