package arch

import (
	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
)

// GoSDKLowerer lowers a SemanticIR and TargetConfig into an ArchitectureIR for Go client SDK emission.
type GoSDKLowerer struct{}

func (GoSDKLowerer) Lower(sem *ir.SemanticIR, bp *BuildProfile, target TargetConfig) (*ArchitectureIR, []ir.Diagnostic) {
	return lowerSDKCommon(sem, bp, target)
}

// TSSDKLowerer lowers a SemanticIR and TargetConfig into an ArchitectureIR for TypeScript client SDK emission.
type TSSDKLowerer struct{}

func (TSSDKLowerer) Lower(sem *ir.SemanticIR, bp *BuildProfile, target TargetConfig) (*ArchitectureIR, []ir.Diagnostic) {
	return lowerSDKCommon(sem, bp, target)
}

func lowerSDKCommon(sem *ir.SemanticIR, bp *BuildProfile, target TargetConfig) (*ArchitectureIR, []ir.Diagnostic) {
	arch := &ArchitectureIR{
		TargetName:     target.Name,
		Kind:           target.Kind,
		Language:       target.Language,
		Pattern:        target.Architecture,
		Module:         bp.Project.Module,
		OutDir:         target.OutDir,
		DomainEnums:    sem.Enums,
		DomainValues:   sem.Values,
		DomainEntities: sem.Entities,
		DomainErrors:   sem.Errors,
	}

	for _, a := range sem.Actions {
		if h, ok := LowerActionHTTP(a); ok {
			arch.HTTPHandlers = append(arch.HTTPHandlers, h)
		}
	}

	return arch, nil
}
