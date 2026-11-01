package runtime

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"sync"
	"time"
)

const (
	// maxJWKSBytes caps how much of a key set we will read, so a broken or hostile endpoint
	// cannot exhaust memory.
	maxJWKSBytes = 1 << 20
	// minRefreshInterval stops a flood of tokens with unknown key ids from turning every
	// request into a fetch against the identity provider.
	minRefreshInterval = 10 * time.Second
	// defaultJWKSCacheTTL is how long a fetched key set is trusted before it is re-fetched.
	defaultJWKSCacheTTL = 5 * time.Minute
)

// jwksCache holds a key set fetched from one endpoint and refreshes it when a token names a
// key it does not have. It is safe for concurrent use.
type jwksCache struct {
	url       string
	client    *http.Client
	ttl       time.Duration
	throttle  time.Duration
	mu        sync.RWMutex
	keys      map[string]crypto.PublicKey
	fetchedAt time.Time
	attemptAt time.Time
}

func newJWKSCache(url string, ttl, throttle time.Duration) *jwksCache {
	if ttl <= 0 {
		ttl = defaultJWKSCacheTTL
	}
	if throttle < 0 {
		throttle = 0
	}
	return &jwksCache{
		url:      url,
		client:   &http.Client{Timeout: 10 * time.Second},
		ttl:      ttl,
		throttle: throttle,
	}
}

// key returns the public key that keyID names, fetching or refreshing the set as needed.
// An unknown key id triggers one refresh, because key sets rotate.
func (c *jwksCache) key(keyID string) (crypto.PublicKey, error) {
	if key, ok := c.fresh(keyID); ok {
		return key, nil
	}
	if err := c.refresh(); err != nil {
		return nil, err
	}
	if key, ok := c.fresh(keyID); ok {
		return key, nil
	}
	return nil, fmt.Errorf("%w: key %q is not in the configured key set", ErrUnauthenticated, keyID)
}

func (c *jwksCache) fresh(keyID string) (crypto.PublicKey, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.fetchedAt.IsZero() || time.Since(c.fetchedAt) > c.ttl {
		return nil, false
	}
	key, ok := c.keys[keyID]
	return key, ok
}

func (c *jwksCache) refresh() error {
	c.mu.Lock()
	if c.throttle > 0 && !c.attemptAt.IsZero() && time.Since(c.attemptAt) < c.throttle {
		// We asked recently and it did not give us this key. Refusing rather than fetching
		// again keeps a flood of tokens with unknown key ids from turning every request into a
		// round trip to the identity provider.
		c.mu.Unlock()
		return fmt.Errorf("%w: key set was refreshed too recently", ErrUnauthenticated)
	}
	c.attemptAt = time.Now()
	c.mu.Unlock()

	resp, err := c.client.Get(c.url)
	if err != nil {
		return fmt.Errorf("%w: fetching key set: %v", ErrUnauthenticated, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: key set endpoint answered %d", ErrUnauthenticated, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxJWKSBytes))
	if err != nil {
		return fmt.Errorf("%w: reading key set: %v", ErrUnauthenticated, err)
	}
	keys, err := parseJWKS(body)
	if err != nil {
		return err
	}

	c.mu.Lock()
	c.keys = keys
	c.fetchedAt = time.Now()
	c.mu.Unlock()
	return nil
}

// parseJWKS turns a JWKS document into public keys by key id. Entries it cannot represent are
// skipped rather than failing the whole set, so one unknown key type cannot lock out the rest.
func parseJWKS(body []byte) (map[string]crypto.PublicKey, error) {
	var doc struct {
		Keys []struct {
			Kty string `json:"kty"`
			Kid string `json:"kid"`
			N   string `json:"n"`
			E   string `json:"e"`
			Crv string `json:"crv"`
			X   string `json:"x"`
			Y   string `json:"y"`
		} `json:"keys"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("%w: key set is not valid JSON: %v", ErrUnauthenticated, err)
	}

	keys := make(map[string]crypto.PublicKey, len(doc.Keys))
	for _, entry := range doc.Keys {
		if entry.Kid == "" {
			continue
		}
		switch entry.Kty {
		case "RSA":
			n, errN := base64.RawURLEncoding.DecodeString(entry.N)
			e, errE := base64.RawURLEncoding.DecodeString(entry.E)
			if errN != nil || errE != nil || len(n) == 0 || len(e) == 0 {
				continue
			}
			keys[entry.Kid] = &rsa.PublicKey{
				N: new(big.Int).SetBytes(n),
				E: int(new(big.Int).SetBytes(e).Int64()),
			}
		case "EC":
			if entry.Crv != "P-256" {
				continue
			}
			x, errX := base64.RawURLEncoding.DecodeString(entry.X)
			y, errY := base64.RawURLEncoding.DecodeString(entry.Y)
			if errX != nil || errY != nil || len(x) == 0 || len(y) == 0 {
				continue
			}
			keys[entry.Kid] = &ecdsa.PublicKey{
				Curve: elliptic.P256(),
				X:     new(big.Int).SetBytes(x),
				Y:     new(big.Int).SetBytes(y),
			}
		}
	}
	return keys, nil
}
