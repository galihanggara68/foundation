package main

import (
	"strings"
	"testing"

	"github.com/galihanggara68/foundation/pkg/emitterkit"
)

func TestGenerateOpenTelemetry_MiddlewareAnchor(t *testing.T) {
	req := emitterkit.GenerationRequest{
		Files: map[string]string{
			"internal/adapters/http/router.go": `package httpadapter

func NewServer() *Server {
	s := &Server{router: chi.NewRouter()}
	// foundation:anchor:http-router-middleware
	s.routes()
	return s
}
`,
		},
	}

	resp, err := generateOpenTelemetry(req)
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}

	routerContent, ok := resp.Files["internal/adapters/http/router.go"]
	if !ok {
		t.Fatal("expected router.go in response files")
	}

	if !strings.Contains(routerContent, "s.router.Use(OpenTelemetryMiddleware)") {
		t.Errorf("expected router to inject middleware, got:\n%s", routerContent)
	}

	if _, ok := resp.Files["internal/adapters/http/otel_middleware.go"]; !ok {
		t.Error("expected otel_middleware.go to exist in response files")
	}
}

func TestGenerateOpenTelemetry_ServeAnchorFallback(t *testing.T) {
	req := emitterkit.GenerationRequest{
		Files: map[string]string{
			"internal/adapters/http/router.go": `package httpadapter

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// foundation:anchor:http-router-serve
	s.router.ServeHTTP(w, r)
}
`,
		},
	}

	resp, err := generateOpenTelemetry(req)
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}

	routerContent, ok := resp.Files["internal/adapters/http/router.go"]
	if !ok {
		t.Fatal("expected router.go in response files")
	}

	if !strings.Contains(routerContent, "OpenTelemetryMiddleware(s.router).ServeHTTP(w, r)") {
		t.Errorf("expected router to wrap ServeHTTP on fallback, got:\n%s", routerContent)
	}
}
