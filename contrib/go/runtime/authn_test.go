package runtime_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

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

// assertRejects asserts a token is refused, and that it was refused for the expected reason so
// a test cannot pass because the token tripped some unrelated check.
func assertRejects(t *testing.T, resolver *runtime.JWTOIDCResolver, key *signingKey, claims map[string]any, want string) {
	t.Helper()
	if _, err := resolver.ResolveActor(context.Background(), bearerRequest(key.token(t, claims))); err == nil {
		t.Errorf("expected rejection containing %q, but the token was accepted", want)
	} else if !strings.Contains(err.Error(), want) {
		t.Errorf("rejected for the wrong reason: want %q, got %v", want, err)
	}
}

// TestJWTOIDCResolver covers the path that matters: a token genuinely signed by a key in the
// configured set, carrying claims the resolver accepts.
func TestJWTOIDCResolver(t *testing.T) {
	key := newSigningKey(t, "RS256")
	srv := serveJWKS(t, key)
	t.Setenv("AUTH_JWKS_URL", srv.URL)

	resolver := runtime.NewJWTOIDCResolver("https://auth.example.com", "my-api")

	claims := claimsFor(t, "https://auth.example.com", "my-api")
	claims["sub"] = "user-jwt-999"
	claims["role"] = "editor"

	resolved, err := resolver.ResolveActor(context.Background(), bearerRequest(key.token(t, claims)))
	if err != nil {
		t.Fatalf("a validly signed token must authenticate, got: %v", err)
	}
	if resolved["id"] != "user-jwt-999" {
		t.Errorf("expected id user-jwt-999, got %v", resolved["id"])
	}
	if resolved["role"] != "editor" {
		t.Errorf("expected role editor, got %v", resolved["role"])
	}

	// Missing Bearer token
	reqNoBearer := httptest.NewRequest("GET", "/v1/orders", nil)
	if _, err := resolver.ResolveActor(context.Background(), reqNoBearer); err == nil {
		t.Fatalf("expected unauthenticated error for missing bearer, got nil")
	}
}

// The key set is the only trust anchor. With none configured nothing can be verified, so
// nothing may be accepted.
func TestJWTOIDCResolver_NoKeySetConfigured_Rejects(t *testing.T) {
	key := newSigningKey(t, "RS256")
	resolver := runtime.NewJWTOIDCResolverWithConfig(runtime.JWTOIDCConfig{
		Issuer:   "https://auth.example.com",
		Audience: "my-api",
	})

	token := key.token(t, claimsFor(t, "https://auth.example.com", "my-api"))
	if _, err := resolver.ResolveActor(context.Background(), bearerRequest(token)); err == nil {
		t.Error("a resolver with no key set accepted a token")
	}
}

// ES256 is the other allowed algorithm, and its JWS signature is raw r‖s rather than a DER
// pair — an easy detail to get wrong.
func TestJWTOIDCResolver_ES256(t *testing.T) {
	key := newSigningKey(t, "ES256")
	srv := serveJWKS(t, key)
	resolver := runtime.NewJWTOIDCResolverWithConfig(runtime.JWTOIDCConfig{
		Issuer:   "https://auth.example.com",
		Audience: "my-api",
		JWKSURL:  srv.URL,
	})

	token := key.token(t, claimsFor(t, "https://auth.example.com", "my-api"))
	if _, err := resolver.ResolveActor(context.Background(), bearerRequest(token)); err != nil {
		t.Fatalf("a validly signed ES256 token must authenticate, got: %v", err)
	}
}

func TestJWTOIDCResolver_TimeClaims(t *testing.T) {
	key := newSigningKey(t, "RS256")
	srv := serveJWKS(t, key)
	resolver := runtime.NewJWTOIDCResolverWithConfig(runtime.JWTOIDCConfig{
		Issuer:   "https://auth.example.com",
		Audience: "my-api",
		JWKSURL:  srv.URL,
	})

	expired := claimsFor(t, "https://auth.example.com", "my-api")
	expired["exp"] = time.Now().Add(-time.Hour).Unix()
	assertRejects(t, resolver, key, expired, "expired")

	notYetValid := claimsFor(t, "https://auth.example.com", "my-api")
	notYetValid["nbf"] = time.Now().Add(time.Hour).Unix()
	assertRejects(t, resolver, key, notYetValid, "not valid yet")

	withoutExpiry := claimsFor(t, "https://auth.example.com", "my-api")
	delete(withoutExpiry, "exp")
	assertRejects(t, resolver, key, withoutExpiry, "no exp claim")
}

// A key set rotates. A token naming a key the cache does not hold must trigger a refresh
// rather than being rejected until the TTL happens to lapse.
func TestJWTOIDCResolver_UnknownKeyID_RefreshesKeySet(t *testing.T) {
	first := newSigningKey(t, "RS256")
	rotated := newSigningKey(t, "RS256")

	var mu sync.Mutex
	served := jwksDocument(first)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		_, _ = w.Write(served)
	}))
	t.Cleanup(srv.Close)

	resolver := runtime.NewJWTOIDCResolverWithConfig(runtime.JWTOIDCConfig{
		Issuer:          "https://auth.example.com",
		Audience:        "my-api",
		JWKSURL:         srv.URL,
		RefreshThrottle: -1, // no throttling, so a rotation is observable inside a test
	})

	claims := claimsFor(t, "https://auth.example.com", "my-api")
	if _, err := resolver.ResolveActor(context.Background(), bearerRequest(first.token(t, claims))); err != nil {
		t.Fatalf("the published key must verify: %v", err)
	}

	mu.Lock()
	served = jwksDocument(rotated)
	mu.Unlock()

	if _, err := resolver.ResolveActor(context.Background(), bearerRequest(rotated.token(t, claims))); err != nil {
		t.Fatalf("a rotated key must be picked up by refreshing the set, got: %v", err)
	}
}

// An unreachable key set fails closed: no token is trusted just because the endpoint is down.
func TestJWTOIDCResolver_KeySetUnreachable_FailsClosed(t *testing.T) {
	key := newSigningKey(t, "RS256")
	srv := serveJWKS(t, key)
	url := srv.URL
	srv.Close()

	resolver := runtime.NewJWTOIDCResolverWithConfig(runtime.JWTOIDCConfig{
		Issuer:   "https://auth.example.com",
		Audience: "my-api",
		JWKSURL:  url,
	})

	token := key.token(t, claimsFor(t, "https://auth.example.com", "my-api"))
	if _, err := resolver.ResolveActor(context.Background(), bearerRequest(token)); err == nil {
		t.Error("a token was accepted while the key set was unreachable")
	}
}
