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

// Principal is the authenticated identity extracted from a validated Zitadel
// JWT. Subject is the Zitadel user ID; ProjectRoles is the set of role keys
// the principal carries in the urn:zitadel:iam:org:project:roles claim. The
// {orgId: orgName} value side of the Zitadel claim is intentionally discarded
// — handlers only need set-membership against role keys.
type Principal struct {
	Subject      string
	ProjectRoles map[string]struct{}
}

// HasProjectRole reports whether the principal carries the given role key.
// Safe on a nil receiver so callers can use it without first nil-checking
// the result of principalFromContext.
func (p *Principal) HasProjectRole(roleKey string) bool {
	if p == nil {
		return false
	}
	_, ok := p.ProjectRoles[roleKey]
	return ok
}

// Validate parses a Zitadel-issued RS256 JWT and returns the principal.
// Delegates signature, expiry, issuer, and audience verification to
// golang-jwt/jwt/v5; key material is fetched from the Zitadel JWKS endpoint
// and cached for one hour.
//
// The token is parsed exactly once — callers MUST stash the returned
// principal in request context for downstream readers rather than re-parsing
// the raw bearer token (audit ref C4).
func Validate(ctx context.Context, tokenStr, domain, audience string) (*Principal, error) {
	type zitadelClaims struct {
		jwt.RegisteredClaims
		ProjectRoles map[string]map[string]string `json:"urn:zitadel:iam:org:project:roles,omitempty"`
	}
	claims := &zitadelClaims{}
	_, err := jwt.ParseWithClaims(tokenStr, claims, store.keyFunc(ctx, domain),
		jwt.WithIssuer(fmt.Sprintf("https://%s", domain)),
		jwt.WithAudience(audience),
		jwt.WithExpirationRequired(),
		jwt.WithValidMethods([]string{"RS256"}),
	)
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	subject, err := claims.GetSubject()
	if err != nil || subject == "" {
		return nil, fmt.Errorf("token missing subject claim")
	}

	roles := make(map[string]struct{}, len(claims.ProjectRoles))
	for k := range claims.ProjectRoles {
		roles[k] = struct{}{}
	}
	return &Principal{Subject: subject, ProjectRoles: roles}, nil
}
