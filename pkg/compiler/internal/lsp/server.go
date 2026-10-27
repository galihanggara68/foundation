package lsp

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// Server is the Foundation Language Server implementation.
type Server struct {
	transport *Transport
	workspace *Workspace
	shutdown  bool
}

// NewServer creates an initialized Server instance reading from in and writing to out.
func NewServer(in io.Reader, out io.Writer, compileFn CompileFunc) *Server {
	return &Server{
		transport: NewTransport(in, out),
		workspace: NewWorkspace(compileFn),
	}
}

// Run starts the JSON-RPC message processing loop until EOF or exit notification.
func (s *Server) Run() error {
	for {
		payload, err := s.transport.ReadMessage()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return fmt.Errorf("LSP transport read error: %w", err)
		}

		if shouldExit := s.handleMessage(payload); shouldExit {
			if !s.shutdown {
				return errors.New("server exited without prior shutdown request")
			}
			return nil
		}
	}
}

func (s *Server) handleMessage(payload []byte) bool {
	var req RequestMessage
	if err := json.Unmarshal(payload, &req); err != nil {
		_ = s.transport.WriteResponse(nil, nil, &ResponseError{
			Code:    -32700,
			Message: "Parse error",
		})
		return false
	}

	isRequest := len(req.ID) > 0 && string(req.ID) != "null"

	switch req.Method {
	case "initialize":
		s.handleInitialize(req)
	case "initialized":
		// No-op ack
	case "shutdown":
		s.shutdown = true
		_ = s.transport.WriteResponse(req.ID, nil, nil)
	case "exit":
		return true
	case "textDocument/didOpen":
		s.handleDidOpen(req.Params)
	case "textDocument/didChange":
		s.handleDidChange(req.Params)
	case "textDocument/didSave":
		s.handleDidSave(req.Params)
	case "textDocument/didClose":
		s.handleDidClose(req.Params)
	case "textDocument/hover":
		s.handleHover(req)
	case "textDocument/completion":
		s.handleCompletion(req)
	default:
		if isRequest {
			_ = s.transport.WriteResponse(req.ID, nil, &ResponseError{
				Code:    -32601,
				Message: fmt.Sprintf("method not found: %s", req.Method),
			})
		}
	}

	return false
}

func (s *Server) handleInitialize(req RequestMessage) {
	var params InitializeParams
	if len(req.Params) > 0 {
		_ = json.Unmarshal(req.Params, &params)
	}

	if params.RootURI != "" {
		if rootPath, err := URIToPath(params.RootURI); err == nil {
			s.workspace.AddRoot(rootPath)
		}
	}
	for _, folder := range params.WorkspaceFolders {
		if rootPath, err := URIToPath(folder.URI); err == nil {
			s.workspace.AddRoot(rootPath)
		}
	}

	res := InitializeResult{
		Capabilities: ServerCapabilities{
			TextDocumentSync: 1, // Full document sync
			HoverProvider:    true,
			CompletionProvider: &CompletionOptions{
				TriggerCharacters: []string{"@", ":", "."},
			},
		},
		ServerInfo: &ServerInfo{
			Name:    "foundation-lsp",
			Version: "0.2.0",
		},
	}

	_ = s.transport.WriteResponse(req.ID, res, nil)
}

func (s *Server) handleDidOpen(paramsRaw json.RawMessage) {
	var params DidOpenTextDocumentParams
	if err := json.Unmarshal(paramsRaw, &params); err != nil {
		return
	}
	s.workspace.SetDocument(params.TextDocument.URI, params.TextDocument.Text)
	s.publishDiagnostics(params.TextDocument.URI)
}

func (s *Server) handleDidChange(paramsRaw json.RawMessage) {
	var params DidChangeTextDocumentParams
	if err := json.Unmarshal(paramsRaw, &params); err != nil {
		return
	}
	if len(params.ContentChanges) > 0 {
		// Full sync: last element represents latest text
		last := params.ContentChanges[len(params.ContentChanges)-1]
		s.workspace.SetDocument(params.TextDocument.URI, last.Text)
		s.publishDiagnostics(params.TextDocument.URI)
	}
}

func (s *Server) handleDidSave(paramsRaw json.RawMessage) {
	var params DidSaveTextDocumentParams
	if err := json.Unmarshal(paramsRaw, &params); err != nil {
		return
	}
	if params.Text != nil {
		s.workspace.SetDocument(params.TextDocument.URI, *params.Text)
	}
	s.publishDiagnostics(params.TextDocument.URI)
}

func (s *Server) handleDidClose(paramsRaw json.RawMessage) {
	var params DidCloseTextDocumentParams
	if err := json.Unmarshal(paramsRaw, &params); err != nil {
		return
	}
	s.workspace.RemoveDocument(params.TextDocument.URI)
}

func (s *Server) handleHover(req RequestMessage) {
	var params TextDocumentPositionParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		_ = s.transport.WriteResponse(req.ID, nil, nil)
		return
	}

	content, ok := s.workspace.GetDocument(params.TextDocument.URI)
	if !ok {
		_ = s.transport.WriteResponse(req.ID, nil, nil)
		return
	}

	hover := ComputeHover(content, params.Position)
	_ = s.transport.WriteResponse(req.ID, hover, nil)
}

func (s *Server) handleCompletion(req RequestMessage) {
	var params TextDocumentPositionParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		_ = s.transport.WriteResponse(req.ID, nil, nil)
		return
	}

	content, ok := s.workspace.GetDocument(params.TextDocument.URI)
	if !ok {
		_ = s.transport.WriteResponse(req.ID, nil, nil)
		return
	}

	list := ComputeCompletion(content, params.Position)
	_ = s.transport.WriteResponse(req.ID, list, nil)
}

func (s *Server) publishDiagnostics(targetURI string) {
	diagMap, err := s.workspace.CompileWorkspace(targetURI)
	if err != nil {
		return
	}

	for uri, diags := range diagMap {
		_ = s.transport.WriteNotification("textDocument/publishDiagnostics", PublishDiagnosticsParams{
			URI:         uri,
			Diagnostics: diags,
		})
	}
}
