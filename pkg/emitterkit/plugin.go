package emitterkit

import (
	"fmt"
	"io"
	"os"

	"github.com/galihanggara68/foundation/pkg/compiler"
)

// Wire protocol, semantic IR, and storage plan types re-exported for SDK and plugin authors.
type (
	GenerationRequest  = compiler.GenerationRequest
	GenerationResponse = compiler.GenerationResponse
	TargetInfo         = compiler.TargetInfo
	TargetConfig       = compiler.TargetConfig
	PlatformInfo       = compiler.PlatformInfo
	PaginationDefaults = compiler.PaginationDefaults
	Diagnostic         = compiler.Diagnostic
	Emission           = compiler.Emission
	SemanticIR         = compiler.SemanticIR
	ArchitectureIR     = compiler.ArchitectureIR
	EntityDecl         = compiler.EntityDecl
	FieldDecl          = compiler.FieldDecl
	ActionDecl         = compiler.ActionDecl

	// Root IR declarations.
	ActorDecl      = compiler.ActorDecl
	SlotDecl       = compiler.SlotDecl
	SlotMethodDecl = compiler.SlotMethodDecl
	GivenSlotDecl  = compiler.GivenSlotDecl
	EnumDecl       = compiler.EnumDecl
	EventDecl      = compiler.EventDecl
	PolicyDecl     = compiler.PolicyDecl

	// Field, value, error, action, and scenario declarations.
	FieldKind     = compiler.FieldKind
	TraitDecl     = compiler.TraitDecl
	ValueDecl     = compiler.ValueDecl
	ErrorKind     = compiler.ErrorKind
	ErrorPosture  = compiler.ErrorPosture
	ErrorDecl     = compiler.ErrorDecl
	ParamDecl     = compiler.ParamDecl
	GivenSeedDecl = compiler.GivenSeedDecl
	ExpectDecl    = compiler.ExpectDecl
	ScenarioDecl  = compiler.ScenarioDecl

	// Step, expression, and diagnostic declarations.
	StepKind       = compiler.StepKind
	ExprKind       = compiler.ExprKind
	Expr           = compiler.Expr
	FieldInitDecl  = compiler.FieldInitDecl
	MatchArmDecl   = compiler.MatchArmDecl
	WherePredicate = compiler.WherePredicate
	StepDecl       = compiler.StepDecl
	Span           = compiler.Span
	Severity       = compiler.Severity

	// Relational storage plan declarations.
	StoragePlanIR    = compiler.StoragePlanIR
	TableSchema      = compiler.TableSchema
	ColumnSchema     = compiler.ColumnSchema
	ForeignKey       = compiler.ForeignKey
	IndexSchema      = compiler.IndexSchema
	UniqueConstraint = compiler.UniqueConstraint
	MigrationPlan    = compiler.MigrationPlan
)

// Severity constants indicate diagnostic classification levels.
const (
	SeverityError   = compiler.SeverityError
	SeverityWarning = compiler.SeverityWarning
	SeverityInfo    = compiler.SeverityInfo
)

// ProtocolVersion is the canonical integer version of the Foundation plugin wire protocol.
const ProtocolVersion = compiler.ProtocolVersion

// HandlerFunc processes a GenerationRequest and produces a GenerationResponse or an error.
type HandlerFunc func(req GenerationRequest) (GenerationResponse, error)

// RunPlugin executes the plugin using standard input (request) and standard output (response).
// If unhandled errors occur during execution, it logs to stderr and terminates with exit code 1.
func RunPlugin(handler HandlerFunc) {
	if err := RunPluginStream(os.Stdin, os.Stdout, handler); err != nil {
		fmt.Fprintf(os.Stderr, "plugin execution failed: %v\n", err)
		os.Exit(1)
	}
}

// RunPluginStream reads a GenerationRequest from in, calls handler, and writes the GenerationResponse to out.
func RunPluginStream(in io.Reader, out io.Writer, handler HandlerFunc) error {
	req, err := compiler.DecodeGenerationRequest(in)
	if err != nil {
		errResp := GenerationResponse{
			Diagnostics: []Diagnostic{{
				Code:     "FDN0701",
				Severity: compiler.SeverityError,
				Message:  fmt.Sprintf("failed to decode generation request: %v", err),
				Span:     compiler.Span{File: "protocol", Line: 1, Column: 1},
			}},
		}
		data, encErr := compiler.EncodeGenerationResponse(errResp)
		if encErr != nil {
			return encErr
		}
		_, writeErr := out.Write(data)
		return writeErr
	}

	resp, handlerErr := handler(req)
	if handlerErr != nil {
		resp.Diagnostics = append(resp.Diagnostics, Diagnostic{
			Code:     "FDN0701",
			Severity: compiler.SeverityError,
			Message:  handlerErr.Error(),
			Span:     compiler.Span{File: "plugin", Line: 1, Column: 1},
		})
	}

	if resp.Files == nil {
		resp.Files = make(map[string]string)
	}
	if resp.Stubs == nil {
		resp.Stubs = make(map[string]string)
	}

	data, err := compiler.EncodeGenerationResponse(resp)
	if err != nil {
		return err
	}
	_, err = out.Write(data)
	return err
}
