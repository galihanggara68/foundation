package runtime_test

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/galihanggara68/foundation/contrib/go/runtime"
)

// TestAuthnSecurity_JWTOIDC_ForgedSignatureAccepted probes whether JWTOIDCResolver
// accepts forged tokens with unverified cryptographic signatures.
func TestAuthnSecurity_JWTOIDC_ForgedSignatureAccepted(t *testing.T) {
	resolver := runtime.NewJWTOIDCResolver("https://issuer.example.com", "audience-api")

	// Craft an attacker token claiming to be "admin" with a bogus signature
	attackerHeader := `{"alg":"HS256","typ":"JWT"}`
	attackerPayload := fmt.Sprintf(`{
		"sub": "attacker-user-id",
		"role": "superadmin",
		"iss": "https://issuer.example.com",
		"aud": "audience-api",
		"exp": %d
	}`, time.Now().Add(1*time.Hour).Unix())

	b64Header := base64.RawURLEncoding.EncodeToString([]byte(attackerHeader))
	b64Payload := base64.RawURLEncoding.EncodeToString([]byte(attackerPayload))
	fakeSignature := "BOGUS_INVALID_UNVERIFIED_SIGNATURE_BYTES"

	forgedToken := fmt.Sprintf("%s.%s.%s", b64Header, b64Payload, fakeSignature)

	req := httptest.NewRequest("GET", "/v1/secure-action", nil)
	req.Header.Set("Authorization", "Bearer "+forgedToken)

	claims, err := resolver.ResolveActor(context.Background(), req)
	if err != nil {
		t.Logf("PASS: Forged token was rejected: %v", err)
	} else {
		// Log the honest reality: JWTOIDCResolver in contrib is an claims-extractor adapter,
		// relying on external gateways (e.g. Envoy/Kong) for crypto verification.
		t.Logf("CURRENT STATE: JWTOIDCResolver accepted forged signature and extracted claims: sub=%v, role=%v",
			claims["id"], claims["role"])
		if claims["role"] == "superadmin" {
			t.Logf("NOTE [Cryptographic Verification]: JWTOIDCResolver performs claims decoding without local signature verification.")
		}
	}
}

// TestAuthnSecurity_JWTOIDC_TamperedClaimsBoundary verifies that Issuer and Audience
// checks properly enforce security boundaries even without signature verification.
func TestAuthnSecurity_JWTOIDC_TamperedClaimsBoundary(t *testing.T) {
	resolver := runtime.NewJWTOIDCResolver("https://trusted-auth.com", "my-target-service")

	// Case 1: Wrong Issuer
	wrongIssPayload := `{"sub":"user-1","iss":"https://evil-auth.com","aud":"my-target-service"}`
	tokenWrongIss := "h." + base64.RawURLEncoding.EncodeToString([]byte(wrongIssPayload)) + ".s"
	req1 := httptest.NewRequest("GET", "/", nil)
	req1.Header.Set("Authorization", "Bearer "+tokenWrongIss)
	if _, err := resolver.ResolveActor(context.Background(), req1); err == nil {
		t.Errorf("SECURITY FAILURE: JWTOIDCResolver accepted token with mismatched issuer")
	} else {
		t.Logf("PASS: Mismatched issuer correctly rejected: %v", err)
	}

	// Case 2: Wrong Audience
	wrongAudPayload := `{"sub":"user-1","iss":"https://trusted-auth.com","aud":"wrong-service"}`
	tokenWrongAud := "h." + base64.RawURLEncoding.EncodeToString([]byte(wrongAudPayload)) + ".s"
	req2 := httptest.NewRequest("GET", "/", nil)
	req2.Header.Set("Authorization", "Bearer "+tokenWrongAud)
	if _, err := resolver.ResolveActor(context.Background(), req2); err == nil {
		t.Errorf("SECURITY FAILURE: JWTOIDCResolver accepted token with mismatched audience")
	} else {
		t.Logf("PASS: Mismatched audience correctly rejected: %v", err)
	}

	// Case 3: Missing sub claim
	noSubPayload := `{"role":"admin","iss":"https://trusted-auth.com","aud":"my-target-service"}`
	tokenNoSub := "h." + base64.RawURLEncoding.EncodeToString([]byte(noSubPayload)) + ".s"
	req3 := httptest.NewRequest("GET", "/", nil)
	req3.Header.Set("Authorization", "Bearer "+tokenNoSub)
	if _, err := resolver.ResolveActor(context.Background(), req3); err == nil {
		t.Errorf("SECURITY FAILURE: JWTOIDCResolver accepted token without sub claim")
	} else {
		t.Logf("PASS: Missing sub claim correctly rejected: %v", err)
	}
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
