package compiler

import (
	"fmt"
	"io"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/arch"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/emit"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/protocol"
)

// Generation wire protocol public types exported for SDK and plugin authors.
type (
	Emission           = emit.Emission
	GenerationRequest  = protocol.GenerationRequest
	GenerationResponse = protocol.GenerationResponse
	TargetInfo         = protocol.TargetInfo
	TargetConfig       = arch.TargetConfig
	EntityDecl         = ir.EntityDecl
	FieldDecl          = ir.FieldDecl
	ActionDecl         = ir.ActionDecl
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

// EmitBuiltin lowers and emits the built-in target for archIR using the compiler's emitter registry.
func EmitBuiltin(archIR *ArchitectureIR) (Emission, []Diagnostic) {
	if archIR == nil {
		return Emission{}, []Diagnostic{{
			Code:     "FDN0300",
			Severity: SeverityError,
			Message:  "cannot emit nil architecture IR",
			Span:     Span{File: "foundation.build.yaml", Line: 1, Column: 1},
		}}
	}
	reg := defaultRegistry()
	key := arch.TargetKey{
		Kind:         archIR.Kind,
		Language:     archIR.Language,
		Architecture: archIR.Pattern,
	}
	emitter, ok := reg.emitters[key]
	if !ok {
		return Emission{}, []Diagnostic{{
			Code:     "FDN0300",
			Severity: SeverityError,
			Message:  fmt.Sprintf("no emitter registered for target: kind=%s language=%s architecture=%s", archIR.Kind, archIR.Language, archIR.Pattern),
			Span:     Span{File: "foundation.build.yaml", Line: 1, Column: 1},
		}}
	}
	return emitter.Emit(archIR)
}

// ExportIdent converts a field or entity name into an exported Go identifier honoring Go initialisms (ID, SKU, URL).
func ExportIdent(s string) string {
	return arch.ExportIdent(s)
}
