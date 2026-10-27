package emitterkit

import (
	"fmt"
	"io"
	"os"

	"github.com/galihanggara68/foundation/pkg/compiler"
)

// Generation wire protocol types re-exported for SDK and plugin authors.
type (
	GenerationRequest  = compiler.GenerationRequest
	GenerationResponse = compiler.GenerationResponse
	TargetInfo         = compiler.TargetInfo
	TargetConfig       = compiler.TargetConfig
	Diagnostic         = compiler.Diagnostic
	Emission           = compiler.Emission
	SemanticIR         = compiler.SemanticIR
	ArchitectureIR     = compiler.ArchitectureIR
	EntityDecl         = compiler.EntityDecl
	FieldDecl          = compiler.FieldDecl
	ActionDecl         = compiler.ActionDecl
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
