package chain_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/galihanggara68/foundation/pkg/emitterkit/chain"
)

func TestInsertBefore_SingleAnchor(t *testing.T) {
	content := `package router

func setup() {
	// foundation:anchor:middleware
	routes()
}
`
	snippet := "Use(OtelMiddleware())"
	result, err := chain.InsertBefore(content, "middleware", snippet)
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}

	expected := `package router

func setup() {
	Use(OtelMiddleware())
	// foundation:anchor:middleware
	routes()
}
`
	if result != expected {
		t.Errorf("mismatched result.\nExpected:\n%s\nGot:\n%s", expected, result)
	}
}

func TestInsertAfter_SingleAnchor(t *testing.T) {
	content := `package router

func setup() {
	// foundation:anchor:middleware
	routes()
}
`
	snippet := "Use(LoggingMiddleware())"
	result, err := chain.InsertAfter(content, "middleware", snippet)
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}

	expected := `package router

func setup() {
	// foundation:anchor:middleware
	Use(LoggingMiddleware())
	routes()
}
`
	if result != expected {
		t.Errorf("mismatched result.\nExpected:\n%s\nGot:\n%s", expected, result)
	}
}

func TestReplaceAnchor_SingleAnchor(t *testing.T) {
	t.Run("replace anchor with statement", func(t *testing.T) {
		content := `func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// foundation:anchor:http-router-serve
	s.router.ServeHTTP(w, r)
}
`
		snippet := "OpenTelemetryMiddleware(s.router).ServeHTTP(w, r)"
		result, err := chain.ReplaceAnchor(content, "http-router-serve", snippet)
		if err != nil {
			t.Fatalf("expected nil error, got: %v", err)
		}

		expected := `func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	OpenTelemetryMiddleware(s.router).ServeHTTP(w, r)
	s.router.ServeHTTP(w, r)
}
`
		if result != expected {
			t.Errorf("mismatched result.\nExpected:\n%s\nGot:\n%s", expected, result)
		}
	})

	t.Run("replace standalone anchor comment", func(t *testing.T) {
		content := `package main

// foundation:anchor:custom-hook

func main() {}
`
		snippet := "var ready = true"
		result, err := chain.ReplaceAnchor(content, "custom-hook", snippet)
		if err != nil {
			t.Fatalf("expected nil error, got: %v", err)
		}

		expected := `package main

var ready = true

func main() {}
`
		if result != expected {
			t.Errorf("mismatched result.\nExpected:\n%s\nGot:\n%s", expected, result)
		}
	})
}

func TestMissingAnchor_ReturnsErrAnchorNotFound(t *testing.T) {
	content := `package main

func run() {}
`
	tests := []struct {
		name string
		call func() (string, error)
	}{
		{
			name: "InsertBefore",
			call: func() (string, error) {
				return chain.InsertBefore(content, "missing", "code()")
			},
		},
		{
			name: "InsertAfter",
			call: func() (string, error) {
				return chain.InsertAfter(content, "missing", "code()")
			},
		},
		{
			name: "ReplaceAnchor",
			call: func() (string, error) {
				return chain.ReplaceAnchor(content, "missing", "code()")
			},
		},
		{
			name: "empty anchor name",
			call: func() (string, error) {
				return chain.ReplaceAnchor(content, "", "code()")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.call()
			if !errors.Is(err, chain.ErrAnchorNotFound) {
				t.Errorf("expected ErrAnchorNotFound, got: %v", err)
			}
		})
	}
}

func TestDuplicateAnchor_ReturnsErrDuplicateAnchor(t *testing.T) {
	content := `package main

// foundation:anchor:dup
// foundation:anchor:dup
func run() {}
`
	tests := []struct {
		name string
		call func() (string, error)
	}{
		{
			name: "InsertBefore",
			call: func() (string, error) {
				return chain.InsertBefore(content, "dup", "code()")
			},
		},
		{
			name: "InsertAfter",
			call: func() (string, error) {
				return chain.InsertAfter(content, "dup", "code()")
			},
		},
		{
			name: "ReplaceAnchor",
			call: func() (string, error) {
				return chain.ReplaceAnchor(content, "dup", "code()")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.call()
			if !errors.Is(err, chain.ErrDuplicateAnchor) {
				t.Errorf("expected ErrDuplicateAnchor, got: %v", err)
			}
		})
	}
}

func TestWhitespaceAndFormattingPreservation(t *testing.T) {
	t.Run("tab indentation preservation", func(t *testing.T) {
		content := "func nested() {\n\t\t// foundation:anchor:slot\n\t\treturn\n}\n"
		result, err := chain.InsertBefore(content, "slot", "call()")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected := "func nested() {\n\t\tcall()\n\t\t// foundation:anchor:slot\n\t\treturn\n}\n"
		if result != expected {
			t.Errorf("expected:\n%q\ngot:\n%q", expected, result)
		}
	})

	t.Run("space indentation preservation", func(t *testing.T) {
		content := "func nested() {\n    // foundation:anchor:slot\n    return\n}\n"
		result, err := chain.InsertAfter(content, "slot", "call()")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected := "func nested() {\n    // foundation:anchor:slot\n    call()\n    return\n}\n"
		if result != expected {
			t.Errorf("expected:\n%q\ngot:\n%q", expected, result)
		}
	})

	t.Run("pre-indented snippet does not double indent", func(t *testing.T) {
		content := "func nested() {\n\t// foundation:anchor:slot\n}\n"
		result, err := chain.InsertBefore(content, "slot", "\tcall()")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected := "func nested() {\n\tcall()\n\t// foundation:anchor:slot\n}\n"
		if result != expected {
			t.Errorf("expected:\n%q\ngot:\n%q", expected, result)
		}
	})

	t.Run("surrounding structure preserved", func(t *testing.T) {
		content := "package main\n\n// header\n\n// foundation:anchor:hook\n\n// footer\n"
		result, err := chain.ReplaceAnchor(content, "hook", "// new hook")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasPrefix(result, "package main\n\n// header\n\n") {
			t.Errorf("header corrupted: %q", result)
		}
		if !strings.HasSuffix(result, "\n\n// footer\n") {
			t.Errorf("footer corrupted: %q", result)
		}
	})
}
