package compiler

import (
	"github.com/galihanggara68/foundation/pkg/compiler/internal/arch"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/emit"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/emit/gohex"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/validate"
)

type compilerRegistry struct {
	passes   []validate.Pass
	lowerers map[arch.TargetKey]arch.PatternLowerer
	emitters map[arch.TargetKey]emit.TargetEmitter
}

func defaultRegistry() *compilerRegistry {
	goHexKey := arch.TargetKey{Kind: "backend", Language: "go", Architecture: "hexagonal"}
	return &compilerRegistry{
		passes: []validate.Pass{
			validate.DeclarationPass{},
			validate.ActionTypeCheckPass{},
			validate.TerminationPass{},
			validate.InvariantPass{},
			validate.ScenarioTypeCheckPass{},
		},
		lowerers: map[arch.TargetKey]arch.PatternLowerer{
			goHexKey: arch.GoHexagonalLowerer{},
		},
		emitters: map[arch.TargetKey]emit.TargetEmitter{
			goHexKey: gohex.Emitter{},
		},
	}
}

func validateSemanticIR(sem *ir.SemanticIR) []ir.Diagnostic {
	sym := validate.NewSymbolTable()
	reg := defaultRegistry()
	var diags []ir.Diagnostic
	for _, pass := range reg.passes {
		diags = append(diags, pass.Validate(sem, sym)...)
	}
	return diags
}
