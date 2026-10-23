package compiler

// ValidationPass represents a single semantic or architectural invariant verification pass
// over the SemanticIR (Composite / Pipeline pattern for SRP & OCP).
type ValidationPass interface {
	Validate(ir *SemanticIR, sym *symbolTable) []Diagnostic
}

// TargetKey uniquely identifies a compilation target strategy by kind, language, and architecture.
type TargetKey struct {
	Kind         string
	Language     string
	Architecture string
}

// PatternLowerer lowers a validated SemanticIR + TargetConfig into an ArchitectureIR (Strategy pattern).
type PatternLowerer interface {
	Lower(sem *SemanticIR, bp *BuildProfile, target TargetConfig) (*ArchitectureIR, []Diagnostic)
}

// Emission holds the deterministic virtual file trees produced by a TargetEmitter.
type Emission struct {
	TrackedFiles map[string][]byte
	StubFiles    map[string][]byte
}

// TargetEmitter translates a lowered ArchitectureIR into target source files (Strategy pattern).
type TargetEmitter interface {
	Emit(arch *ArchitectureIR) (Emission, []Diagnostic)
}

type compilerRegistry struct {
	passes   []ValidationPass
	lowerers map[TargetKey]PatternLowerer
	emitters map[TargetKey]TargetEmitter
}

func defaultRegistry() *compilerRegistry {
	goHexKey := TargetKey{Kind: "backend", Language: "go", Architecture: "hexagonal"}
	return &compilerRegistry{
		passes: []ValidationPass{
			declarationPass{},
			actionTypeCheckPass{},
			scenarioTypeCheckPass{},
		},
		lowerers: map[TargetKey]PatternLowerer{
			goHexKey: goHexagonalLowerer{},
		},
		emitters: map[TargetKey]TargetEmitter{
			goHexKey: goHexagonalEmitter{},
		},
	}
}
