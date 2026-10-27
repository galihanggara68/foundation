package protocol_test

import (
	"testing"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/protocol"
)

// FuzzDecodeResponse tests wire protocol resilience against corrupted or hostile JSON responses.
func FuzzDecodeResponse(f *testing.F) {
	// Seed 1: Valid minimal response
	f.Add([]byte(`{"protocol_version":1,"diagnostics":[],"files":{},"stubs":{}}`))

	// Seed 2: Empty byte array
	f.Add([]byte{})

	// Seed 3: Corrupt JSON
	f.Add([]byte(`{"protocol_version": 1, "files": {"foo": `))
	f.Add([]byte(`{"protocol_version": -999, "diagnostics": "not-an-array"}`))
	f.Add([]byte(`{"files": {"\u0000\ufffd": 12345}}`))

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 64*1024 {
			return
		}

		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("PANIC during DecodeResponse on input: %v", r)
			}
		}()

		resp, err := protocol.DecodeResponse(data)
		_ = resp
		_ = err
	})
}

// FuzzDecodeRequest tests wire protocol resilience against corrupted generation requests.
func FuzzDecodeRequest(f *testing.F) {
	// Seed 1: Minimal request
	f.Add([]byte(`{"protocol_version":1,"workspace_root":".","target":{"name":"backend","language":"go"}}`))

	// Seed 2: Malformed payloads
	f.Add([]byte(`{"semantic_ir": {"entities": [{"fields": null}]}}`))
	f.Add([]byte(`{"files": {"a/b/c": "\x00\xff"}}`))

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 64*1024 {
			return
		}

		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("PANIC during DecodeRequest on input: %v", r)
			}
		}()

		req, err := protocol.DecodeRequest(data)
		_ = req
		_ = err
	})
}
