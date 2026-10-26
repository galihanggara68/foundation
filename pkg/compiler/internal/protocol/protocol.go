package protocol

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/arch"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
)

// ProtocolVersion is the canonical integer version of the Foundation plugin generation wire protocol.
const ProtocolVersion = 1

// TargetInfo embeds arch.TargetConfig with custom options passed to external generators.
type TargetInfo struct {
	arch.TargetConfig
	Options map[string]string `json:"options,omitempty"`
}

// TargetInfoFromConfig constructs a TargetInfo from an arch.TargetConfig and custom options map.
func TargetInfoFromConfig(cfg arch.TargetConfig, options map[string]string) TargetInfo {
	opts := options
	if opts == nil {
		opts = make(map[string]string)
	}
	return TargetInfo{
		TargetConfig: cfg,
		Options:      opts,
	}
}

// GenerationRequest is the payload sent from the Foundation compiler to an external generator plugin.
type GenerationRequest struct {
	ProtocolVersion int                  `json:"protocol_version"`
	WorkspaceRoot   string               `json:"workspace_root"`
	Target          TargetInfo           `json:"target"`
	SemanticIR      *ir.SemanticIR       `json:"semantic_ir"`
	ArchitectureIR  *arch.ArchitectureIR `json:"architecture_ir,omitempty"`
	Files           map[string]string    `json:"files,omitempty"`
}

// Validate verifies structural completeness and protocol version compatibility.
func (r *GenerationRequest) Validate() error {
	if r.ProtocolVersion <= 0 {
		return fmt.Errorf("invalid protocol version: %d", r.ProtocolVersion)
	}
	if r.ProtocolVersion > ProtocolVersion {
		return fmt.Errorf("unsupported protocol version %d (supported up to %d)", r.ProtocolVersion, ProtocolVersion)
	}
	if r.WorkspaceRoot == "" {
		return fmt.Errorf("workspace_root must not be empty")
	}
	if r.Target.Name == "" {
		return fmt.Errorf("target.name must not be empty")
	}
	if r.Target.Kind == "" {
		return fmt.Errorf("target.kind must not be empty")
	}
	if r.Target.Language == "" {
		return fmt.Errorf("target.language must not be empty")
	}
	if r.Target.OutDir == "" {
		return fmt.Errorf("target.out_dir must not be empty")
	}
	if r.SemanticIR == nil {
		return fmt.Errorf("semantic_ir must not be nil")
	}
	return nil
}

// GetFile retrieves the content of a virtual file, returning false if not present.
func (r *GenerationRequest) GetFile(path string) (string, bool) {
	if r.Files == nil {
		return "", false
	}
	c, ok := r.Files[path]
	return c, ok
}

// SetFile sets or updates a virtual file entry.
func (r *GenerationRequest) SetFile(path, content string) {
	if r.Files == nil {
		r.Files = make(map[string]string)
	}
	r.Files[path] = content
}

// GenerationResponse is the payload returned from an external generator plugin to the Foundation compiler.
type GenerationResponse struct {
	Files       map[string]string `json:"files,omitempty"`
	Stubs       map[string]string `json:"stubs,omitempty"`
	Diagnostics []ir.Diagnostic   `json:"diagnostics,omitempty"`
	TestCommand string            `json:"test_command,omitempty"`
}

// HasErrors reports whether any diagnostic in the response has error severity.
func (r *GenerationResponse) HasErrors() bool {
	for _, d := range r.Diagnostics {
		if d.Severity == ir.SeverityError {
			return true
		}
	}
	return false
}

// SetFile sets or updates a tracked generated file in the response.
func (r *GenerationResponse) SetFile(path, content string) {
	if r.Files == nil {
		r.Files = make(map[string]string)
	}
	r.Files[path] = content
}

// AddFile appends or updates a tracked generated file in the response.
func (r *GenerationResponse) AddFile(path, content string) {
	r.SetFile(path, content)
}

// AddStub appends or updates a create-if-missing stub file in the response.
func (r *GenerationResponse) AddStub(path, content string) {
	if r.Stubs == nil {
		r.Stubs = make(map[string]string)
	}
	r.Stubs[path] = content
}

// AddDiagnostic appends a compiler diagnostic to the response.
func (r *GenerationResponse) AddDiagnostic(diag ir.Diagnostic) {
	r.Diagnostics = append(r.Diagnostics, diag)
}

// EncodeRequest serializes a GenerationRequest to formatted JSON.
func EncodeRequest(req GenerationRequest) ([]byte, error) {
	if req.ProtocolVersion == 0 {
		req.ProtocolVersion = ProtocolVersion
	}
	return json.MarshalIndent(req, "", "  ")
}

// DecodeRequest deserializes JSON bytes into a GenerationRequest, validating the protocol version
// and ensuring map fields are non-nil.
func DecodeRequest(data []byte) (GenerationRequest, error) {
	var req GenerationRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return GenerationRequest{}, fmt.Errorf("failed to decode generation request: %w", err)
	}
	if req.ProtocolVersion < 0 {
		return GenerationRequest{}, fmt.Errorf("invalid protocol version %d (must be >= 1)", req.ProtocolVersion)
	}
	if req.ProtocolVersion == 0 {
		req.ProtocolVersion = ProtocolVersion
	}
	if req.ProtocolVersion > ProtocolVersion {
		return GenerationRequest{}, fmt.Errorf("unsupported protocol version %d (supported up to %d)", req.ProtocolVersion, ProtocolVersion)
	}
	if req.Files == nil {
		req.Files = make(map[string]string)
	}
	if req.Target.Options == nil {
		req.Target.Options = make(map[string]string)
	}
	return req, nil
}

// EncodeResponse serializes a GenerationResponse to formatted JSON.
func EncodeResponse(resp GenerationResponse) ([]byte, error) {
	return json.MarshalIndent(resp, "", "  ")
}

// DecodeResponse deserializes JSON bytes into a GenerationResponse, ensuring map and slice fields are non-nil.
func DecodeResponse(data []byte) (GenerationResponse, error) {
	var resp GenerationResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return GenerationResponse{}, fmt.Errorf("failed to decode generation response: %w", err)
	}
	if resp.Files == nil {
		resp.Files = make(map[string]string)
	}
	if resp.Stubs == nil {
		resp.Stubs = make(map[string]string)
	}
	if resp.Diagnostics == nil {
		resp.Diagnostics = make([]ir.Diagnostic, 0)
	}
	return resp, nil
}

// WriteRequest serializes a GenerationRequest to the provided io.Writer.
func WriteRequest(w io.Writer, req GenerationRequest) error {
	data, err := EncodeRequest(req)
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

// ReadRequest reads and decodes a GenerationRequest from the provided io.Reader.
func ReadRequest(r io.Reader) (GenerationRequest, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return GenerationRequest{}, fmt.Errorf("failed to read generation request: %w", err)
	}
	return DecodeRequest(data)
}

// WriteResponse serializes a GenerationResponse to the provided io.Writer.
func WriteResponse(w io.Writer, resp GenerationResponse) error {
	data, err := EncodeResponse(resp)
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

// ReadResponse reads and decodes a GenerationResponse from the provided io.Reader.
func ReadResponse(r io.Reader) (GenerationResponse, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return GenerationResponse{}, fmt.Errorf("failed to read generation response: %w", err)
	}
	return DecodeResponse(data)
}
