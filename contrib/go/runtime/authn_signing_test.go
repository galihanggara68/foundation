package runtime_test

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// signingKey is a key pair plus the JWK that publishes its public half, so tests can mint
// real tokens and serve a real JWKS without a network, an issuer, or a third-party library.
type signingKey struct {
	kid  string
	alg  string
	priv any
	jwk  map[string]any
}

func b64url(b []byte) string {
	return base64.RawURLEncoding.EncodeToString(b)
}

// keyIDFor derives a key id from the public key, so two generated keys never share one and a
// rotation is distinguishable from a re-fetch of the same set.
func keyIDFor(material []byte) string {
	digest := sha256.Sum256(material)
	return "test-" + b64url(digest[:6])
}

// newSigningKey generates a key pair for a supported algorithm: RS256 → RSA-2048,
// ES256 → P-256.
func newSigningKey(t *testing.T, alg string) *signingKey {
	t.Helper()

	switch alg {
	case "RS256":
		priv, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			t.Fatalf("generate RSA key: %v", err)
		}
		pub := priv.Public().(*rsa.PublicKey)
		kid := keyIDFor(pub.N.Bytes())
		return &signingKey{
			kid:  kid,
			alg:  alg,
			priv: priv,
			jwk: map[string]any{
				"kty": "RSA",
				"kid": kid,
				"alg": alg,
				"use": "sig",
				"n":   b64url(pub.N.Bytes()),
				"e":   b64url(big.NewInt(int64(pub.E)).Bytes()),
			},
		}
	case "ES256":
		priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			t.Fatalf("generate ECDSA key: %v", err)
		}
		pub := priv.Public().(*ecdsa.PublicKey)
		size := (pub.Curve.Params().BitSize + 7) / 8
		x := pub.X.FillBytes(make([]byte, size))
		y := pub.Y.FillBytes(make([]byte, size))
		kid := keyIDFor(append(append([]byte{}, x...), y...))
		return &signingKey{
			kid:  kid,
			alg:  alg,
			priv: priv,
			jwk: map[string]any{
				"kty": "EC",
				"kid": kid,
				"alg": alg,
				"use": "sig",
				"crv": "P-256",
				"x":   b64url(x),
				"y":   b64url(y),
			},
		}
	default:
		t.Fatalf("unsupported test algorithm %q", alg)
		return nil
	}
}

// jwksDocument renders the keys as a JWKS body.
func jwksDocument(keys ...*signingKey) []byte {
	entries := make([]map[string]any, 0, len(keys))
	for _, k := range keys {
		entries = append(entries, k.jwk)
	}
	doc, _ := json.Marshal(map[string]any{"keys": entries})
	return doc
}

// serveJWKS publishes keys at a local endpoint and cleans the server up with the test.
func serveJWKS(t *testing.T, keys ...*signingKey) *httptest.Server {
	t.Helper()
	body := jwksDocument(keys...)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// claimsFor builds a claim set that passes every check the resolver makes, so a rejection
// can be attributed to the thing the test is probing.
func claimsFor(t *testing.T, iss, aud string) map[string]any {
	t.Helper()
	return map[string]any{
		"sub":  "user-1",
		"role": "admin",
		"iss":  iss,
		"aud":  aud,
		"exp":  time.Now().Add(time.Hour).Unix(),
		"iat":  time.Now().Add(-time.Minute).Unix(),
	}
}

// token signs claims with this key into a compact JWS.
func (k *signingKey) token(t *testing.T, claims map[string]any) string {
	t.Helper()
	return k.tokenWithHeader(t, map[string]any{"alg": k.alg, "typ": "JWT", "kid": k.kid}, claims)
}

// tokenWithHeader signs arbitrary claims under an arbitrary header, for the cases that probe
// header handling (`alg: none`, a missing kid, a mismatched alg).
func (k *signingKey) tokenWithHeader(t *testing.T, header, claims map[string]any) string {
	t.Helper()
	headerJSON, err := json.Marshal(header)
	if err != nil {
		t.Fatalf("marshal header: %v", err)
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("marshal claims: %v", err)
	}
	signingInput := b64url(headerJSON) + "." + b64url(claimsJSON)
	return signingInput + "." + b64url(k.signature(t, signingInput))
}

func (k *signingKey) signature(t *testing.T, signingInput string) []byte {
	t.Helper()
	digest := sha256.Sum256([]byte(signingInput))

	switch priv := k.priv.(type) {
	case *rsa.PrivateKey:
		sig, err := rsa.SignPKCS1v15(rand.Reader, priv, crypto.SHA256, digest[:])
		if err != nil {
			t.Fatalf("sign RS256: %v", err)
		}
		return sig
	case *ecdsa.PrivateKey:
		r, s, err := ecdsa.Sign(rand.Reader, priv, digest[:])
		if err != nil {
			t.Fatalf("sign ES256: %v", err)
		}
		size := (priv.Curve.Params().BitSize + 7) / 8
		sig := append(r.FillBytes(make([]byte, size)), s.FillBytes(make([]byte, size))...)
		return sig
	default:
		t.Fatalf("unsupported signing key %T", k.priv)
		return nil
	}
}

// tamper rewrites the payload of a token while keeping its signature, which is what an
// attacker does when the signature is not checked.
func tamper(t *testing.T, token string, claims map[string]any) string {
	t.Helper()
	parts := splitToken(t, token)
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("marshal claims: %v", err)
	}
	return parts[0] + "." + b64url(claimsJSON) + "." + parts[2]
}

func splitToken(t *testing.T, token string) []string {
	t.Helper()
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("token has %d segments, want 3: %q", len(parts), token)
	}
	return parts
}
