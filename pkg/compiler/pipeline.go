package compiler

import (
	"github.com/galihanggara68/foundation/pkg/compiler/internal/arch"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/emit"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/emit/gohex"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/emit/tsclean"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/validate"
)

type compilerRegistry struct {
	emitters map[arch.TargetKey]emit.TargetEmitter
}

func defaultRegistry() *compilerRegistry {
	goHexKey := arch.TargetKey{Kind: "backend", Language: "go", Architecture: "hexagonal"}
	goLayeredKey := arch.TargetKey{Kind: "backend", Language: "go", Architecture: "layered"}
	tsCleanKey := arch.TargetKey{Kind: "backend", Language: "typescript", Architecture: "clean"}
	return &compilerRegistry{
		emitters: map[arch.TargetKey]emit.TargetEmitter{
			goHexKey:     gohex.Emitter{},
			goLayeredKey: gohex.LayeredEmitter{},
			tsCleanKey:   tsclean.Emitter{},
		},
	}
}

func validateSemanticIR(sem *ir.SemanticIR) []ir.Diagnostic {
	return validate.Validate(sem)
}
