// Package auth handles Zitadel-issued JWT validation for the MkAuth backend.
package auth

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// jwk is a JSON Web Key entry from a JWKS endpoint.
type jwk struct {
	Kty string `json:"kty"`
	Use string `json:"use"`
	Kid string `json:"kid"`
	N   string `json:"n"`
	E   string `json:"e"`
}

type jwksResponse struct {
	Keys []jwk `json:"keys"`
}

// keyStore caches RSA public keys fetched from the Zitadel JWKS endpoint.
type keyStore struct {
	mu        sync.RWMutex
	keys      map[string]*rsa.PublicKey
	fetchedAt time.Time
	ttl       time.Duration
}

var store = &keyStore{ttl: time.Hour}

// SetKeysForTesting injects RSA public keys directly into the key store cache.
// Intended for use in tests only — bypasses the JWKS network fetch.
func SetKeysForTesting(keys map[string]*rsa.PublicKey) {
	store.mu.Lock()
	store.keys = keys
	store.fetchedAt = time.Now()
	store.mu.Unlock()
}

func jwkToRSA(k jwk) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(k.N)
	if err != nil {
		return nil, fmt.Errorf("decode modulus: %w", err)
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(k.E)
	if err != nil {
		return nil, fmt.Errorf("decode exponent: %w", err)
	}
	e := int(new(big.Int).SetBytes(eBytes).Int64())
	if e == 0 {
		return nil, fmt.Errorf("invalid RSA exponent")
	}
	return &rsa.PublicKey{N: new(big.Int).SetBytes(nBytes), E: e}, nil
}

func (s *keyStore) refresh(ctx context.Context, domain string) error {
	url := fmt.Sprintf("https://%s/oauth/v2/keys", domain)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("build jwks request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("fetch jwks: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("jwks endpoint returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return fmt.Errorf("read jwks body: %w", err)
	}
	var set jwksResponse
	if err := json.Unmarshal(body, &set); err != nil {
		return fmt.Errorf("decode jwks: %w", err)
	}

	keys := make(map[string]*rsa.PublicKey, len(set.Keys))
	for _, k := range set.Keys {
		if k.Kty != "RSA" || k.Use != "sig" {
			continue
		}
		pub, err := jwkToRSA(k)
		if err != nil {
			return fmt.Errorf("parse key kid=%s: %w", k.Kid, err)
		}
		keys[k.Kid] = pub
	}

	s.mu.Lock()
	s.keys = keys
	s.fetchedAt = time.Now()
	s.mu.Unlock()
	return nil
}

func (s *keyStore) keyFunc(ctx context.Context, domain string) jwt.Keyfunc {
	return func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		kid, _ := t.Header["kid"].(string)

		s.mu.RLock()
		fresh := time.Since(s.fetchedAt) < s.ttl
		var k *rsa.PublicKey
		if fresh {
			k = s.keys[kid]
		}
		s.mu.RUnlock()

		if k != nil {
			return k, nil
		}

		// Cache miss or stale — refresh then retry
		if err := s.refresh(ctx, domain); err != nil {
			return nil, err
		}

		s.mu.RLock()
		k = s.keys[kid]
		s.mu.RUnlock()

		if k == nil {
			return nil, fmt.Errorf("unknown key id %q; check Zitadel key rotation", kid)
		}
		return k, nil
	}
}

// ValidateToken validates a Zitadel-issued RS256 JWT.
// Returns the subject (Zitadel user ID) on success.
// Delegates signature verification, expiry, issuer, and audience checks to
// golang-jwt/jwt/v5; key material is fetched from the Zitadel JWKS endpoint
// and cached for one hour.
func ValidateToken(ctx context.Context, tokenStr, domain, audience string) (string, error) {
	claims := &jwt.RegisteredClaims{}
	_, err := jwt.ParseWithClaims(tokenStr, claims, store.keyFunc(ctx, domain),
		jwt.WithIssuer(fmt.Sprintf("https://%s", domain)),
		jwt.WithAudience(audience),
		jwt.WithExpirationRequired(),
		jwt.WithValidMethods([]string{"RS256"}),
	)
	if err != nil {
		return "", fmt.Errorf("invalid token: %w", err)
	}

	subject, err := claims.GetSubject()
	if err != nil || subject == "" {
		return "", fmt.Errorf("token missing subject claim")
	}

	return subject, nil
}
