package lsp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"
)

func TestTransport_ReadMessage_Valid(t *testing.T) {
	body := `{"jsonrpc":"2.0","method":"initialized","params":{}}`
	frame := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(body), body)

	transport := NewTransport(strings.NewReader(frame), &bytes.Buffer{})
	payload, err := transport.ReadMessage()
	if err != nil {
		t.Fatalf("unexpected read error: %v", err)
	}

	if string(payload) != body {
		t.Fatalf("expected payload %q, got %q", body, string(payload))
	}
}

func TestTransport_ReadMessage_WithExtraHeaders(t *testing.T) {
	body := `{"jsonrpc":"2.0","id":1,"method":"shutdown"}`
	frame := fmt.Sprintf("Content-Type: application/vscode-jsonrpc; charset=utf-8\r\nContent-Length: %d\r\nCustom-Header: value\r\n\r\n%s", len(body), body)

	transport := NewTransport(strings.NewReader(frame), &bytes.Buffer{})
	payload, err := transport.ReadMessage()
	if err != nil {
		t.Fatalf("unexpected read error: %v", err)
	}

	if string(payload) != body {
		t.Fatalf("expected payload %q, got %q", body, string(payload))
	}
}

func TestTransport_ReadMessage_MultipleInStream(t *testing.T) {
	msg1 := `{"jsonrpc":"2.0","id":1}`
	msg2 := `{"jsonrpc":"2.0","id":2}`
	stream := fmt.Sprintf("Content-Length: %d\r\n\r\n%sContent-Length: %d\r\n\r\n%s", len(msg1), msg1, len(msg2), msg2)

	transport := NewTransport(strings.NewReader(stream), &bytes.Buffer{})

	p1, err := transport.ReadMessage()
	if err != nil {
		t.Fatalf("read message 1 failed: %v", err)
	}
	if string(p1) != msg1 {
		t.Fatalf("expected %q, got %q", msg1, string(p1))
	}

	p2, err := transport.ReadMessage()
	if err != nil {
		t.Fatalf("read message 2 failed: %v", err)
	}
	if string(p2) != msg2 {
		t.Fatalf("expected %q, got %q", msg2, string(p2))
	}

	// Next read should return EOF
	_, err = transport.ReadMessage()
	if err != io.EOF {
		t.Fatalf("expected EOF on third read, got: %v", err)
	}
}

func TestTransport_ReadMessage_MissingLength(t *testing.T) {
	frame := "Content-Type: application/json\r\n\r\n{}"
	transport := NewTransport(strings.NewReader(frame), &bytes.Buffer{})
	_, err := transport.ReadMessage()
	if err != errMissingLength {
		t.Fatalf("expected errMissingLength, got %v", err)
	}
}

func TestTransport_ReadMessage_MalformedHeader(t *testing.T) {
	frame := "MalformedHeaderWithoutColon\r\n\r\n{}"
	transport := NewTransport(strings.NewReader(frame), &bytes.Buffer{})
	_, err := transport.ReadMessage()
	if err != errMalformedHeader {
		t.Fatalf("expected errMalformedHeader, got %v", err)
	}
}

func TestTransport_ReadMessage_NegativeLength(t *testing.T) {
	frame := "Content-Length: -5\r\n\r\n{}"
	transport := NewTransport(strings.NewReader(frame), &bytes.Buffer{})
	_, err := transport.ReadMessage()
	if err != errMalformedHeader {
		t.Fatalf("expected errMalformedHeader for negative length, got %v", err)
	}
}

func TestTransport_WriteResponseAndNotification(t *testing.T) {
	var buf bytes.Buffer
	transport := NewTransport(&bytes.Buffer{}, &buf)

	id := json.RawMessage(`"req-42"`)
	if err := transport.WriteResponse(id, map[string]string{"status": "ok"}, nil); err != nil {
		t.Fatalf("WriteResponse failed: %v", err)
	}

	respPayload, err := transport.ReadMessage()
	// Create another transport reading from buf
	readerTransport := NewTransport(&buf, &bytes.Buffer{})
	respPayload, err = readerTransport.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read response back: %v", err)
	}

	var resp ResponseMessage
	if err := json.Unmarshal(respPayload, &resp); err != nil {
		t.Fatalf("failed to parse response JSON: %v", err)
	}
	if string(resp.ID) != `"req-42"` {
		t.Fatalf("expected ID %q, got %q", `"req-42"`, string(resp.ID))
	}

	buf.Reset()
	if err := transport.WriteNotification("test/event", map[string]int{"count": 7}); err != nil {
		t.Fatalf("WriteNotification failed: %v", err)
	}

	notifPayload, err := readerTransport.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read notification back: %v", err)
	}
	var notif NotificationMessage
	if err := json.Unmarshal(notifPayload, &notif); err != nil {
		t.Fatalf("failed to parse notification JSON: %v", err)
	}
	if notif.Method != "test/event" {
		t.Fatalf("expected method 'test/event', got %q", notif.Method)
	}
}

// chunkedReader returns 1 byte per Read call to simulate chunked/fragmented stream I/O.
type chunkedReader struct {
	data []byte
	pos  int
}

func (cr *chunkedReader) Read(p []byte) (int, error) {
	if cr.pos >= len(cr.data) {
		return 0, io.EOF
	}
	if len(p) == 0 {
		return 0, nil
	}
	p[0] = cr.data[cr.pos]
	cr.pos++
	return 1, nil
}

func TestTransport_ReadMessage_ChunkedStream(t *testing.T) {
	body := `{"jsonrpc":"2.0","method":"test/chunked"}`
	frame := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(body), body)

	cr := &chunkedReader{data: []byte(frame)}
	transport := NewTransport(cr, &bytes.Buffer{})

	payload, err := transport.ReadMessage()
	if err != nil {
		t.Fatalf("unexpected error on chunked read: %v", err)
	}
	if string(payload) != body {
		t.Fatalf("expected payload %q, got %q", body, string(payload))
	}
}

func TestTransport_ReadMessage_InvalidUTF8(t *testing.T) {
	// Raw invalid UTF-8 byte sequence 0xFF 0xFE
	invalidPayload := []byte{'{', '"', 'd', '"', ':', ' ', 0xff, 0xfe, '}'}
	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(invalidPayload))
	data := append([]byte(header), invalidPayload...)

	transport := NewTransport(bytes.NewReader(data), &bytes.Buffer{})
	payload, err := transport.ReadMessage()
	if err != nil {
		t.Fatalf("transport should read raw bytes without crashing, got: %v", err)
	}
	if !bytes.Equal(payload, invalidPayload) {
		t.Fatalf("expected exact raw bytes to be preserved")
	}
}
