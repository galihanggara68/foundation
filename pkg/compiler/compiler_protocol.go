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
