package runtime_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/galihanggara68/foundation/contrib/go/runtime"
)

// TestAuthnSecurity_JWTOIDC_ForgedSignatureRejected asserts that a token whose signature
// does not verify never authenticates. This test previously *logged* the acceptance as the
// current state ("CURRENT STATE: ... accepted forged signature"); an authentication bypass
// that a test only narrates is still an authentication bypass.
func TestAuthnSecurity_JWTOIDC_ForgedSignatureRejected(t *testing.T) {
	key := newSigningKey(t, "RS256")
	srv := serveJWKS(t, key)
	t.Setenv("AUTH_JWKS_URL", srv.URL)

	resolver := runtime.NewJWTOIDCResolver("https://issuer.example.com", "audience-api")

	valid := key.token(t, claimsFor(t, "https://issuer.example.com", "audience-api"))
	if _, err := resolver.ResolveActor(context.Background(), bearerRequest(valid)); err != nil {
		t.Fatalf("the fixture must authenticate before tampering, got: %v", err)
	}

	// Same key id, same signature, but the claims an attacker would want: the payload changed
	// after signing, which is exactly what signature verification exists to catch.
	escalated := claimsFor(t, "https://issuer.example.com", "audience-api")
	escalated["role"] = "superadmin"
	forged := tamper(t, valid, escalated)
	if forged == valid {
		t.Fatal("the fixture did not change the token, so nothing is being tested")
	}
	if _, err := resolver.ResolveActor(context.Background(), bearerRequest(forged)); err == nil {
		t.Error("SECURITY FAILURE: JWTOIDCResolver accepted a token whose signature does not verify")
	}
}

// TestAuthnSecurity_JWTOIDC_UnverifiedAlgorithmsRejected covers the two classic JWT
// confusion attacks: an unsigned token, and a token that claims an HMAC algorithm so a
// public key would be used as the HMAC secret.
func TestAuthnSecurity_JWTOIDC_UnverifiedAlgorithmsRejected(t *testing.T) {
	key := newSigningKey(t, "RS256")
	srv := serveJWKS(t, key)
	t.Setenv("AUTH_JWKS_URL", srv.URL)

	resolver := runtime.NewJWTOIDCResolver("https://issuer.example.com", "audience-api")
	claims := claimsFor(t, "https://issuer.example.com", "audience-api")

	noneHeader := map[string]any{"alg": "none", "typ": "JWT", "kid": key.kid}
	// An unsigned token: alg "none" with an empty signature segment.
	segments := strings.Split(key.tokenWithHeader(t, noneHeader, claims), ".")
	noneToken := segments[0] + "." + segments[1] + "."
	if _, err := resolver.ResolveActor(context.Background(), bearerRequest(noneToken)); err == nil {
		t.Error("SECURITY FAILURE: JWTOIDCResolver accepted an unsigned alg:none token")
	}

	hmacHeader := map[string]any{"alg": "HS256", "typ": "JWT", "kid": key.kid}
	if _, err := resolver.ResolveActor(context.Background(), bearerRequest(key.tokenWithHeader(t, hmacHeader, claims))); err == nil {
		t.Error("SECURITY FAILURE: JWTOIDCResolver accepted an HS256 token, letting the public key stand in as an HMAC secret")
	}
}

func bearerRequest(token string) *http.Request {
	req := httptest.NewRequest("GET", "/v1/secure-action", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	return req
}

// TestAuthnSecurity_JWTOIDC_TamperedClaimsBoundary asserts the claim checks hold for a token
// that is *validly signed*: the rejection has to come from the claim, not from the signature,
// or the test would pass for the wrong reason.
func TestAuthnSecurity_JWTOIDC_TamperedClaimsBoundary(t *testing.T) {
	key := newSigningKey(t, "RS256")
	srv := serveJWKS(t, key)
	resolver := runtime.NewJWTOIDCResolverWithConfig(runtime.JWTOIDCConfig{
		Issuer:          "https://trusted-auth.com",
		Audience:        "my-target-service",
		JWKSURL:         srv.URL,
		RefreshThrottle: -1, // no throttling, so the unpublished-key case reports the real reason
	})

	assertRejects(t, resolver, key, claimsFor(t, "https://evil-auth.com", "my-target-service"), "invalid jwt issuer")
	assertRejects(t, resolver, key, claimsFor(t, "https://trusted-auth.com", "wrong-service"), "invalid jwt audience")

	noSubject := claimsFor(t, "https://trusted-auth.com", "my-target-service")
	delete(noSubject, "sub")
	assertRejects(t, resolver, key, noSubject, "missing sub claim")

	// Validly signed by a key the published set does not contain.
	unpublished := newSigningKey(t, "RS256")
	assertRejects(t, resolver, unpublished, claimsFor(t, "https://trusted-auth.com", "my-target-service"),
		"not in the configured key set")
}

// TestAuthnSecurity_DevHeaders_ArbitraryImpersonation probes DevHeadersResolver
// to document exact header injection behavior.
func TestAuthnSecurity_DevHeaders_ArbitraryImpersonation(t *testing.T) {
	resolver := runtime.NewDevHeadersResolver()

	req := httptest.NewRequest("POST", "/admin/delete-database", nil)
	req.Header.Set("X-Actor-ID", "impersonated-root-admin")
	req.Header.Set("X-Actor-Role", "system-administrator")
	req.Header.Set("X-Actor-Is-Superuser", "true")

	claims, err := resolver.ResolveActor(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Logf("CURRENT STATE [Dev Headers]: Plain header injection succeeded as designed for dev mode:")
	t.Logf("  id = %v", claims["id"])
	t.Logf("  role = %v", claims["role"])
	t.Logf("  is_superuser = %v", claims["is_superuser"])

	if claims["id"] != "impersonated-root-admin" || claims["role"] != "system-administrator" {
		t.Errorf("DevHeadersResolver failed to extract expected claims")
	}
}
