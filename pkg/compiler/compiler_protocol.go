package compiler

import (
	"fmt"
	"io"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/arch"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/diagnostics"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/emit"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/platform"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/protocol"
)

// Wire protocol, semantic IR, and storage plan types exported for SDK and plugin authors.
type (
	Emission             = emit.Emission
	GenerationRequest    = protocol.GenerationRequest
	GenerationResponse   = protocol.GenerationResponse
	TargetInfo           = protocol.TargetInfo
	TargetConfig         = arch.TargetConfig
	PlatformInfo         = platform.Info
	PaginationDefaults   = platform.PaginationDefaults
	ReservedError        = platform.ReservedError
	DiagnosticDefinition = diagnostics.Definition

	// Root IR declarations.
	SemanticIR     = ir.SemanticIR
	ActorDecl      = ir.ActorDecl
	SlotDecl       = ir.SlotDecl
	SlotMethodDecl = ir.SlotMethodDecl
	GivenSlotDecl  = ir.GivenSlotDecl
	EnumDecl       = ir.EnumDecl
	EventDecl      = ir.EventDecl
	PolicyDecl     = ir.PolicyDecl

	// Field, value, error, action, and scenario declarations.
	FieldKind     = ir.FieldKind
	FieldDecl     = ir.FieldDecl
	TraitDecl     = ir.TraitDecl
	ValueDecl     = ir.ValueDecl
	EntityDecl    = ir.EntityDecl
	ErrorKind     = ir.ErrorKind
	ErrorPosture  = ir.ErrorPosture
	ErrorDecl     = ir.ErrorDecl
	ParamDecl     = ir.ParamDecl
	ActionDecl    = ir.ActionDecl
	GivenSeedDecl = ir.GivenSeedDecl
	ExpectDecl    = ir.ExpectDecl
	ScenarioDecl  = ir.ScenarioDecl

	// Step, expression, and diagnostic declarations.
	StepKind       = ir.StepKind
	ExprKind       = ir.ExprKind
	Expr           = ir.Expr
	FieldInitDecl  = ir.FieldInitDecl
	MatchArmDecl   = ir.MatchArmDecl
	WherePredicate = ir.WherePredicate
	StepDecl       = ir.StepDecl
	Span           = ir.Span
	Severity       = ir.Severity
	Diagnostic     = ir.Diagnostic

	// Relational storage plan declarations.
	StoragePlanIR    = arch.StoragePlanIR
	TableSchema      = arch.TableSchema
	ColumnSchema     = arch.ColumnSchema
	ForeignKey       = arch.ForeignKey
	IndexSchema      = arch.IndexSchema
	UniqueConstraint = arch.UniqueConstraint
	MigrationPlan    = arch.MigrationPlan
)

// Severity constants indicate diagnostic classification levels.
const (
	SeverityError   = ir.SeverityError
	SeverityWarning = ir.SeverityWarning
	SeverityInfo    = ir.SeverityInfo
)

// ProtocolVersion is the canonical integer version of the Foundation plugin generation wire protocol.
const ProtocolVersion = protocol.ProtocolVersion

// EncodeGenerationRequest serializes a GenerationRequest to deterministic JSON.
func EncodeGenerationRequest(req GenerationRequest) ([]byte, error) {
	return protocol.EncodeRequest(req)
}

// DecodeGenerationRequest deserializes a GenerationRequest from an io.Reader.
func DecodeGenerationRequest(r io.Reader) (GenerationRequest, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return GenerationRequest{}, err
	}
	return protocol.DecodeRequest(data)
}

// EncodeGenerationResponse serializes a GenerationResponse to deterministic JSON.
func EncodeGenerationResponse(resp GenerationResponse) ([]byte, error) {
	return protocol.EncodeResponse(resp)
}

// DecodeGenerationResponse deserializes a GenerationResponse from an io.Reader.
func DecodeGenerationResponse(r io.Reader) (GenerationResponse, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return GenerationResponse{}, err
	}
	return protocol.DecodeResponse(data)
}

// EmitBuiltin lowers and emits the built-in target for archIR with the compiler emitter registry.
func EmitBuiltin(archIR *ArchitectureIR) (Emission, []Diagnostic) {
	if archIR == nil {
		return Emission{}, []Diagnostic{NewDiagnostic(
			"FDN0300",
			Span{File: "foundation.build.yaml", Line: 1, Column: 1},
			"cannot emit nil architecture IR",
		)}
	}
	reg := defaultRegistry()
	key := arch.TargetKey{
		Kind:         archIR.Kind,
		Language:     archIR.Language,
		Architecture: archIR.Pattern,
	}
	emitter, ok := reg.emitters[key]
	if !ok {
		return Emission{}, []Diagnostic{NewDiagnostic(
			"FDN0300",
			Span{File: "foundation.build.yaml", Line: 1, Column: 1},
			fmt.Sprintf("no emitter registered for target: kind=%s language=%s architecture=%s", archIR.Kind, archIR.Language, archIR.Pattern),
		)}
	}
	return emitter.Emit(archIR)
}

// ExportIdent converts a field or entity name into an exported Go identifier with Go initialisms (ID, SKU, URL).
func ExportIdent(s string) string {
	return arch.ExportIdent(s)
}

// SafeRelPath confirms that rel is a clean relative path that stays within rootDir.
func SafeRelPath(rootDir, rel string) (string, error) {
	return emit.SafeRelPath(rootDir, rel)
}

// DiagnosticRegistry stores all registered compiler diagnostic definitions.
var DiagnosticRegistry = diagnostics.Registry

// LookupDiagnostic returns the diagnostic definition for the given code.
func LookupDiagnostic(code string) (DiagnosticDefinition, bool) {
	return diagnostics.Lookup(code)
}

// NewDiagnostic creates a Diagnostic for the code, span, and optional message arguments.
func NewDiagnostic(code string, span Span, args ...any) Diagnostic {
	return diagnostics.New(code, span, args...)
}

// NewDiagnosticWithDetail creates a Diagnostic for the code, span, and custom message.
func NewDiagnosticWithDetail(code string, span Span, msg string) Diagnostic {
	return diagnostics.NewWithDetail(code, span, msg)
}

// CaptureRequest compiles the blueprints in dir and returns an encoded GenerationRequest envelope
// for the selected target. The envelope is the wire payload that the compiler sends to a generator
// plugin. Plugin authors use the envelope to build authoritative test fixtures.
// An empty targetName selects the first configured target.
func CaptureRequest(dir string, targetName string) ([]byte, []Diagnostic) {
	sem, diags := Check(dir)
	if HasErrors(diags) {
		return nil, diags
	}

	bp, bpDiags := arch.LoadBuildProfile(dir)
	if len(bpDiags) > 0 {
		return nil, bpDiags
	}

	if len(bp.Targets) == 0 {
		return nil, []Diagnostic{diagnostics.New(
			"FDN0301",
			Span{File: "foundation.build.yaml", Line: 1, Column: 1},
			"no targets configured in foundation.build.yaml",
		)}
	}

	target, found := selectCaptureTarget(bp.Targets, targetName)
	if !found {
		return nil, []Diagnostic{diagnostics.NewWithDetail(
			"FDN0302",
			Span{File: "foundation.build.yaml", Line: 1, Column: 1},
			fmt.Sprintf("target %q not found in foundation.build.yaml", targetName),
		)}
	}

	warnDiags := diags
	hasStorage := bp.Storage.Dialect != "" || bp.Storage.Mode != "" || target.Kind == "backend"
	var storagePlan arch.StoragePlanIR
	if hasStorage {
		var storageDiags []Diagnostic
		storagePlan, _, storageDiags = arch.BuildStoragePlan(dir, sem, bp, false)
		if HasErrors(storageDiags) {
			return nil, storageDiags
		}
		warnDiags = append(warnDiags, storageDiags...)
	}

	archIR, reqFiles, captureDiags := lowerCaptureTarget(sem, bp, target, storagePlan, hasStorage)
	if len(captureDiags) > 0 {
		return nil, captureDiags
	}
	warnDiags = append(warnDiags, captureDiags...)

	platformInfo := platform.DefaultInfo()
	req := protocol.GenerationRequest{
		ProtocolVersion: protocol.ProtocolVersion,
		WorkspaceRoot:   dir,
		Target:          protocol.TargetInfoFromConfig(target, target.Options),
		SemanticIR:      sem,
		ArchitectureIR:  archIR,
		Platform:        &platformInfo,
		Files:           reqFiles,
	}

	payload, err := protocol.EncodeRequest(req)
	if err != nil {
		return nil, []Diagnostic{diagnostics.New(
			"FDN0303",
			Span{File: dir, Line: 1, Column: 1},
			err,
		)}
	}
	return payload, warnDiags
}

// selectCaptureTarget resolves a target by name, or falls back to the first target when name is empty.
func selectCaptureTarget(targets []arch.TargetConfig, name string) (arch.TargetConfig, bool) {
	if name == "" {
		return targets[0], true
	}
	for _, t := range targets {
		if t.Name == name {
			return t, true
		}
	}
	return arch.TargetConfig{}, false
}

// lowerCaptureTarget lowers and emits the selected target the same way Build does, and returns the
// architecture IR with the virtual upstream files that a chained pipeline plugin receives.
func lowerCaptureTarget(sem *ir.SemanticIR, bp *arch.BuildProfile, target arch.TargetConfig, storagePlan arch.StoragePlanIR, hasStorage bool) (*arch.ArchitectureIR, map[string]string, []Diagnostic) {
	reqFiles := make(map[string]string)

	if arch.IsBuiltinTarget(target) {
		archIR, emission, diags := lowerAndEmitBuiltin(sem, bp, target, storagePlan, defaultRegistry())
		if len(diags) > 0 {
			return nil, nil, diags
		}
		if emission != nil {
			for p, content := range emission.TrackedFiles {
				reqFiles[p] = string(content)
			}
			for p, content := range emission.StubFiles {
				if _, exists := reqFiles[p]; !exists {
					reqFiles[p] = string(content)
				}
			}
		}
		return archIR, reqFiles, nil
	}

	if target.EffectiveScaffold() == arch.ScaffoldBuiltin {
		return nil, nil, []Diagnostic{diagnostics.New(
			"FDN0300",
			Span{File: "foundation.build.yaml", Line: 1, Column: 1},
			fmt.Sprintf("no emitter registered for target: kind=%s language=%s architecture=%s", target.Kind, target.Language, target.Architecture),
		)}
	}

	archIR := arch.NewCustomArchitectureIR(target, bp.Project.Module)
	if archIR != nil && hasStorage {
		archIR.Storage = storagePlan
	}
	return archIR, reqFiles, nil
}
