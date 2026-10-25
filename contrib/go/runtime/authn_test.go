package runtime_test

import (
	"context"
	"encoding/base64"
	"net/http/httptest"
	"testing"

	"github.com/galihanggara68/foundation/contrib/go/runtime"
)

func TestDevHeadersResolver(t *testing.T) {
	resolver := runtime.NewDevHeadersResolver()

	req := httptest.NewRequest("GET", "/v1/orders", nil)
	req.Header.Set("X-Actor-ID", "user-123")
	req.Header.Set("X-Actor-Role", "admin")
	req.Header.Set("X-Actor-Verified", "true")
	req.Header.Set("X-Actor-Tenant-ID", "tenant-456")

	claims, err := resolver.ResolveActor(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if claims["id"] != "user-123" {
		t.Errorf("expected id user-123, got %v", claims["id"])
	}
	if claims["role"] != "admin" {
		t.Errorf("expected role admin, got %v", claims["role"])
	}
	if claims["verified"] != true {
		t.Errorf("expected verified true, got %v", claims["verified"])
	}
	if claims["tenant_id"] != "tenant-456" {
		t.Errorf("expected tenant_id tenant-456, got %v", claims["tenant_id"])
	}

	// Missing header -> unauthenticated
	reqNoAuth := httptest.NewRequest("GET", "/v1/orders", nil)
	_, err = resolver.ResolveActor(context.Background(), reqNoAuth)
	if err == nil {
		t.Fatalf("expected unauthenticated error, got nil")
	}
}

func TestJWTOIDCResolver(t *testing.T) {
	resolver := runtime.NewJWTOIDCResolver("https://auth.example.com", "my-api")

	// Payload with sub, role, iss, aud
	payloadJSON := `{"sub":"user-jwt-999","role":"editor","iss":"https://auth.example.com","aud":"my-api"}`
	b64Payload := base64.RawURLEncoding.EncodeToString([]byte(payloadJSON))
	token := "header." + b64Payload + ".signature"

	req := httptest.NewRequest("GET", "/v1/orders", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	claims, err := resolver.ResolveActor(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if claims["id"] != "user-jwt-999" {
		t.Errorf("expected id user-jwt-999, got %v", claims["id"])
	}
	if claims["role"] != "editor" {
		t.Errorf("expected role editor, got %v", claims["role"])
	}

	// Missing Bearer token
	reqNoBearer := httptest.NewRequest("GET", "/v1/orders", nil)
	_, err = resolver.ResolveActor(context.Background(), reqNoBearer)
	if err == nil {
		t.Fatalf("expected unauthenticated error for missing bearer, got nil")
	}
}
