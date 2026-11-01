package runtime

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

var (
	// ErrUnauthenticated is returned when authentication credentials are missing or invalid.
	ErrUnauthenticated = errors.New("unauthenticated: missing or invalid credentials")
)

// DevHeadersResolver authenticates requests by reading plain HTTP headers.
// It is intended for local development and hermetic testing only.
type DevHeadersResolver struct {
	warnOnce sync.Once
}

// NewDevHeadersResolver creates a new DevHeadersResolver and logs a security warning.
func NewDevHeadersResolver() *DevHeadersResolver {
	r := &DevHeadersResolver{}
	r.warnOnce.Do(r.logWarningBanner)
	return r
}

func (r *DevHeadersResolver) logWarningBanner() {
	banner := `
================================================================================
[SECURITY WARNING] dev-headers Authn Adapter is active!
Plain HTTP headers (X-Actor-ID, etc.) are trusted without cryptographic verification.
DO NOT RUN IN PRODUCTION!
================================================================================`
	fmt.Println(banner)
	log.Println("[WARN] authn: dev-headers adapter active; header spoofing permitted")
}

// ResolveActor extracts actor claims from HTTP headers.
func (r *DevHeadersResolver) ResolveActor(ctx context.Context, req *http.Request) (map[string]any, error) {
	actorID := req.Header.Get("X-Actor-ID")
	if actorID == "" {
		return nil, ErrUnauthenticated
	}
	claims := make(map[string]any)
	claims["id"] = actorID
	claims["role"] = req.Header.Get("X-Actor-Role")

	for k, v := range req.Header {
		lowerK := strings.ToLower(k)
		if strings.HasPrefix(lowerK, "x-actor-") && len(v) > 0 {
			key := strings.TrimPrefix(lowerK, "x-actor-")
			if strings.HasPrefix(key, "claim-") {
				key = strings.TrimPrefix(key, "claim-")
			}
			key = strings.ReplaceAll(key, "-", "_")
			if v[0] == "true" {
				claims[key] = true
			} else if v[0] == "false" {
				claims[key] = false
			} else {
				claims[key] = v[0]
			}
		}
	}
	if v, ok := claims["verified"].(bool); ok {
		claims["is_verified"] = v
	} else if v, ok := claims["is_verified"].(bool); ok {
		claims["verified"] = v
	}
	return claims, nil
}

// JWTOIDCConfig configures JWT verification.
//
// Every field has a sensible default, and the key set URL falls back to AUTH_JWKS_URL, so an
// application generated before verification existed gains it by setting that one variable —
// the promise ADR 0006 makes for a security fix published in a contrib adapter rather than in
// the compiler.
type JWTOIDCConfig struct {
	Issuer   string
	Audience string
	JWKSURL  string
	// Algorithms is the accepted "alg" allow-list; it defaults to RS256 and ES256.
	Algorithms []string
	// ClockSkew tolerates small clock differences when checking exp and nbf.
	ClockSkew time.Duration
	// CacheTTL is how long a fetched key set is trusted before it is fetched again.
	CacheTTL time.Duration
	// RefreshThrottle is the shortest gap between two fetches of the key set. Zero means the
	// default; the throttle is what stops a flood of tokens with unknown key ids from turning
	// every request into a round trip to the identity provider.
	RefreshThrottle time.Duration
}

// defaultJWTOIDCAlgorithms is deliberately an allow-list of the two asymmetric algorithms
// identity providers publish key sets for. Nothing HMAC is ever accepted — a public key must
// never be usable as an HMAC secret — and "none" is not representable at all.
var defaultJWTOIDCAlgorithms = []string{"RS256", "ES256"}

// JWTOIDCResolver verifies a JWT's signature against a configured key set before trusting any
// claim. The key set never comes from the token itself (no jku, x5u or embedded jwk), so a
// token cannot point this verifier at an attacker's keys.
type JWTOIDCResolver struct {
	Issuer   string
	Audience string

	algs map[string]bool
	skew time.Duration
	jwks *jwksCache
}

// NewJWTOIDCResolver creates a resolver whose key set comes from AUTH_JWKS_URL. With no key
// set configured every request is rejected: accepting unverified tokens is what this replaces.
func NewJWTOIDCResolver(issuer, audience string) *JWTOIDCResolver {
	return NewJWTOIDCResolverWithConfig(JWTOIDCConfig{
		Issuer:   issuer,
		Audience: audience,
		JWKSURL:  os.Getenv("AUTH_JWKS_URL"),
	})
}

// NewJWTOIDCResolverWithConfig creates a resolver from explicit configuration.
func NewJWTOIDCResolverWithConfig(cfg JWTOIDCConfig) *JWTOIDCResolver {
	algorithms := cfg.Algorithms
	if len(algorithms) == 0 {
		algorithms = defaultJWTOIDCAlgorithms
	}
	algs := make(map[string]bool, len(algorithms))
	for _, alg := range algorithms {
		algs[alg] = true
	}

	resolver := &JWTOIDCResolver{
		Issuer:   cfg.Issuer,
		Audience: cfg.Audience,
		algs:     algs,
		skew:     cfg.ClockSkew,
	}
	if cfg.JWKSURL == "" {
		log.Println("[WARN] authn: jwt-oidc adapter has no AUTH_JWKS_URL configured; every request will be rejected")
		return resolver
	}
	resolver.jwks = newJWKSCache(cfg.JWKSURL, cfg.CacheTTL, refreshThrottle(cfg.RefreshThrottle))
	return resolver
}

// refreshThrottle resolves the configured throttle, treating zero as the default and a
// negative value as "no throttling" (which is only useful in tests).
func refreshThrottle(configured time.Duration) time.Duration {
	if configured == 0 {
		return minRefreshInterval
	}
	return configured
}

// ResolveActor verifies the bearer token and returns its claims.
func (r *JWTOIDCResolver) ResolveActor(ctx context.Context, req *http.Request) (map[string]any, error) {
	authHeader := req.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return nil, ErrUnauthenticated
	}
	token := strings.TrimPrefix(authHeader, "Bearer ")

	claims, err := r.verifyToken(token)
	if err != nil {
		return nil, err
	}

	if sub, ok := claims["sub"].(string); ok && sub != "" {
		claims["id"] = sub
	}
	return claims, nil
}

// verifyToken checks the algorithm, verifies the signature against the key set, and only then
// reads the claims: a claim is never trusted before the signature is.
func (r *JWTOIDCResolver) verifyToken(token string) (map[string]any, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("%w: malformed jwt token", ErrUnauthenticated)
	}

	headerBytes, err := decodeSegment(parts[0])
	if err != nil {
		return nil, fmt.Errorf("%w: invalid jwt header base64", ErrUnauthenticated)
	}
	var header struct {
		Alg string `json:"alg"`
		Kid string `json:"kid"`
	}
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return nil, fmt.Errorf("%w: invalid jwt header", ErrUnauthenticated)
	}
	if !r.algs[header.Alg] {
		return nil, fmt.Errorf("%w: unsupported jwt alg %q", ErrUnauthenticated, header.Alg)
	}
	if r.jwks == nil {
		return nil, fmt.Errorf("%w: no key set configured", ErrUnauthenticated)
	}
	publicKey, err := r.jwks.key(header.Kid)
	if err != nil {
		return nil, err
	}
	signature, err := decodeSegment(parts[2])
	if err != nil {
		return nil, fmt.Errorf("%w: invalid jwt signature base64", ErrUnauthenticated)
	}
	if err := verifyJWTSignature(header.Alg, publicKey, []byte(parts[0]+"."+parts[1]), signature); err != nil {
		return nil, err
	}

	payloadBytes, err := decodeSegment(parts[1])
	if err != nil {
		return nil, fmt.Errorf("%w: invalid jwt payload base64", ErrUnauthenticated)
	}
	var claims map[string]any
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return nil, fmt.Errorf("%w: invalid jwt json payload", ErrUnauthenticated)
	}
	if err := r.checkClaims(claims); err != nil {
		return nil, err
	}
	return claims, nil
}

// decodeSegment reads one base64url segment, accepting the padded form too because not every
// issuer emits unpadded segments.
func decodeSegment(segment string) ([]byte, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(segment)
	if err == nil {
		return decoded, nil
	}
	return base64.URLEncoding.DecodeString(segment)
}

// verifyJWTSignature checks a signature over the signing input with the algorithm the header
// named. The key type has to match that algorithm, so a key set entry cannot be reinterpreted
// under a different one.
func verifyJWTSignature(alg string, publicKey crypto.PublicKey, signingInput, signature []byte) error {
	digest := sha256.Sum256(signingInput)

	switch alg {
	case "RS256":
		rsaKey, ok := publicKey.(*rsa.PublicKey)
		if !ok {
			return fmt.Errorf("%w: key set entry is not an RSA key", ErrUnauthenticated)
		}
		if err := rsa.VerifyPKCS1v15(rsaKey, crypto.SHA256, digest[:], signature); err != nil {
			return fmt.Errorf("%w: jwt signature does not verify", ErrUnauthenticated)
		}
		return nil
	case "ES256":
		ecKey, ok := publicKey.(*ecdsa.PublicKey)
		if !ok {
			return fmt.Errorf("%w: key set entry is not an EC key", ErrUnauthenticated)
		}
		size := (ecKey.Curve.Params().BitSize + 7) / 8
		if len(signature) != 2*size {
			return fmt.Errorf("%w: jwt signature has the wrong length", ErrUnauthenticated)
		}
		rInt := new(big.Int).SetBytes(signature[:size])
		sInt := new(big.Int).SetBytes(signature[size:])
		if !ecdsa.Verify(ecKey, digest[:], rInt, sInt) {
			return fmt.Errorf("%w: jwt signature does not verify", ErrUnauthenticated)
		}
		return nil
	default:
		return fmt.Errorf("%w: unsupported jwt alg %q", ErrUnauthenticated, alg)
	}
}

// checkClaims enforces exp and nbf, requires a subject, and validates iss and aud when they
// are configured.
func (r *JWTOIDCResolver) checkClaims(claims map[string]any) error {
	now := time.Now()

	exp, hasExp := numericClaim(claims, "exp")
	if !hasExp {
		return fmt.Errorf("%w: jwt has no exp claim", ErrUnauthenticated)
	}
	if now.Add(-r.skew).After(time.Unix(exp, 0)) {
		return fmt.Errorf("%w: jwt is expired", ErrUnauthenticated)
	}
	if nbf, hasNbf := numericClaim(claims, "nbf"); hasNbf && now.Add(r.skew).Before(time.Unix(nbf, 0)) {
		return fmt.Errorf("%w: jwt is not valid yet", ErrUnauthenticated)
	}
	if _, hasSub := claims["sub"].(string); !hasSub {
		if _, hasID := claims["id"]; !hasID {
			return fmt.Errorf("%w: jwt missing sub claim", ErrUnauthenticated)
		}
	}
	if r.Issuer != "" {
		if iss, ok := claims["iss"].(string); !ok || iss != r.Issuer {
			return fmt.Errorf("%w: invalid jwt issuer", ErrUnauthenticated)
		}
	}
	if r.Audience != "" && !audienceMatches(claims["aud"], r.Audience) {
		return fmt.Errorf("%w: invalid jwt audience", ErrUnauthenticated)
	}
	return nil
}

func numericClaim(claims map[string]any, name string) (int64, bool) {
	switch value := claims[name].(type) {
	case float64:
		return int64(value), true
	case json.Number:
		parsed, err := value.Int64()
		return parsed, err == nil
	case int64:
		return value, true
	case int:
		return int64(value), true
	default:
		return 0, false
	}
}

func audienceMatches(raw any, want string) bool {
	switch aud := raw.(type) {
	case string:
		return aud == want
	case []any:
		for _, entry := range aud {
			if value, ok := entry.(string); ok && value == want {
				return true
			}
		}
	}
	return false
}
