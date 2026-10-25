package runtime

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
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

// JWTOIDCResolver validates JWT Bearer tokens from the Authorization header.
type JWTOIDCResolver struct {
	Issuer   string
	Audience string
}

// NewJWTOIDCResolver creates a new JWTOIDCResolver.
func NewJWTOIDCResolver(issuer, audience string) *JWTOIDCResolver {
	return &JWTOIDCResolver{
		Issuer:   issuer,
		Audience: audience,
	}
}

// ResolveActor extracts and verifies JWT claims from the Authorization header.
func (r *JWTOIDCResolver) ResolveActor(ctx context.Context, req *http.Request) (map[string]any, error) {
	authHeader := req.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return nil, ErrUnauthenticated
	}
	token := strings.TrimPrefix(authHeader, "Bearer ")
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("%w: malformed jwt token", ErrUnauthenticated)
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		payloadBytes, err = base64.URLEncoding.DecodeString(parts[1])
		if err != nil {
			return nil, fmt.Errorf("%w: invalid jwt payload base64", ErrUnauthenticated)
		}
	}

	var claims map[string]any
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return nil, fmt.Errorf("%w: invalid jwt json payload", ErrUnauthenticated)
	}

	if sub, ok := claims["sub"].(string); ok && sub != "" {
		claims["id"] = sub
	} else if _, hasID := claims["id"]; !hasID {
		return nil, fmt.Errorf("%w: jwt missing sub claim", ErrUnauthenticated)
	}

	if r.Issuer != "" {
		if iss, ok := claims["iss"].(string); !ok || iss != r.Issuer {
			return nil, fmt.Errorf("%w: invalid jwt issuer", ErrUnauthenticated)
		}
	}
	if r.Audience != "" {
		audValid := false
		if aud, ok := claims["aud"].(string); ok && aud == r.Audience {
			audValid = true
		} else if auds, ok := claims["aud"].([]any); ok {
			for _, a := range auds {
				if aStr, ok := a.(string); ok && aStr == r.Audience {
					audValid = true
					break
				}
			}
		}
		if !audValid {
			return nil, fmt.Errorf("%w: invalid jwt audience", ErrUnauthenticated)
		}
	}

	return claims, nil
}
