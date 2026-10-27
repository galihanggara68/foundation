package lsp

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
)

const (
	maxHeaderBytes  = 64 * 1024        // 64 KB header cap
	maxPayloadBytes = 16 * 1024 * 1024 // 16 MB message payload cap
)

var (
	errMalformedHeader = errors.New("malformed LSP message header")
	errMissingLength   = errors.New("missing Content-Length in LSP header")
	errNegativeLength  = errors.New("negative Content-Length in LSP header")
	errPayloadTooLarge = errors.New("LSP payload exceeds maximum size")
)

// Transport handles framed JSON-RPC 2.0 I/O over stream readers and writers.
type Transport struct {
	reader  *bufio.Reader
	writer  io.Writer
	writeMu sync.Mutex
}

// NewTransport constructs a thread-safe message transport around reader and writer.
func NewTransport(r io.Reader, w io.Writer) *Transport {
	return &Transport{
		reader: bufio.NewReader(r),
		writer: w,
	}
}

// ReadMessage reads a single framed JSON-RPC message from the input stream.
func (t *Transport) ReadMessage() ([]byte, error) {
	contentLength := -1
	headerBytesRead := 0

	for {
		line, err := t.reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		headerBytesRead += len(line)
		if headerBytesRead > maxHeaderBytes {
			return nil, errPayloadTooLarge
		}

		trimmed := strings.TrimRight(line, "\r\n")
		if trimmed == "" {
			// Empty line marks end of header section
			break
		}

		parts := strings.SplitN(trimmed, ":", 2)
		if len(parts) != 2 {
			return nil, errMalformedHeader
		}

		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])

		if strings.EqualFold(key, "Content-Length") {
			length, parseErr := strconv.Atoi(val)
			if parseErr != nil || length < 0 {
				return nil, errMalformedHeader
			}
			contentLength = length
		}
	}

	if contentLength < 0 {
		return nil, errMissingLength
	}
	if contentLength > maxPayloadBytes {
		return nil, errPayloadTooLarge
	}

	payload := make([]byte, contentLength)
	if _, err := io.ReadFull(t.reader, payload); err != nil {
		return nil, err
	}

	return payload, nil
}

// WriteResponse sends a structured JSON-RPC response with Content-Length framing.
func (t *Transport) WriteResponse(id json.RawMessage, result any, rpcErr *ResponseError) error {
	resp := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
	}
	if rpcErr != nil {
		resp["error"] = rpcErr
	} else {
		resp["result"] = result
	}

	data, err := json.Marshal(resp)
	if err != nil {
		return fmt.Errorf("failed to marshal LSP response: %w", err)
	}
	return t.writePayload(data)
}

// WriteNotification sends a structured JSON-RPC notification with Content-Length framing.
func (t *Transport) WriteNotification(method string, params any) error {
	notif := NotificationMessage{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}
	data, err := json.Marshal(notif)
	if err != nil {
		return fmt.Errorf("failed to marshal LSP notification: %w", err)
	}
	return t.writePayload(data)
}

func (t *Transport) writePayload(payload []byte) error {
	t.writeMu.Lock()
	defer t.writeMu.Unlock()

	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(payload))
	if _, err := io.WriteString(t.writer, header); err != nil {
		return err
	}
	_, err := t.writer.Write(payload)
	return err
}
