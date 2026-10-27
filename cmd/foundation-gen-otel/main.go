package main

import (
	"strings"

	"github.com/galihanggara68/foundation/pkg/emitterkit"
)

func main() {
	emitterkit.RunPlugin(generateOpenTelemetry)
}

func generateOpenTelemetry(req emitterkit.GenerationRequest) (emitterkit.GenerationResponse, error) {
	resp := emitterkit.GenerationResponse{
		Files:       make(map[string]string),
		TestCommand: "go test ./...",
	}

	// Forward all upstream files (virtual file chaining)
	for path, content := range req.Files {
		resp.Files[path] = content
	}

	// Inject OpenTelemetry HTTP tracing middleware and unit test
	resp.Files["internal/adapters/http/otel_middleware.go"] = emitOtelMiddleware()
	resp.Files["internal/adapters/http/otel_middleware_test.go"] = emitOtelMiddlewareTest()

	// Wrap the HTTP router with OpenTelemetry tracing headers
	if routerContent, ok := resp.Files["internal/adapters/http/router.go"]; ok {
		target := "s.router.ServeHTTP(w, r)"
		replacement := "OpenTelemetryMiddleware(s.router).ServeHTTP(w, r)"
		if strings.Contains(routerContent, target) {
			resp.Files["internal/adapters/http/router.go"] = strings.Replace(routerContent, target, replacement, 1)
		}
	}

	return resp, nil
}

func emitOtelMiddleware() string {
	return `package httpadapter

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
)

// OpenTelemetryMiddleware injects W3C Trace Context and distributed tracing headers.
func OpenTelemetryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		traceID := r.Header.Get("traceparent")
		if traceID == "" {
			buf := make([]byte, 16)
			_, _ = rand.Read(buf)
			traceID = "00-" + hex.EncodeToString(buf) + "-0000000000000001-01"
		}
		w.Header().Set("traceparent", traceID)
		w.Header().Set("X-Trace-ID", traceID)
		next.ServeHTTP(w, r)
	})
}
`
}

func emitOtelMiddlewareTest() string {
	return `package httpadapter

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOpenTelemetryMiddleware(t *testing.T) {
	handler := OpenTelemetryMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest("GET", "/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Header().Get("X-Trace-ID") == "" {
		t.Error("expected X-Trace-ID header in response")
	}
	if rec.Header().Get("traceparent") == "" {
		t.Error("expected traceparent header in response")
	}
}
`
}
