package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func newTestRSAKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}
	return key
}

// signToken builds a signed RS256 JWT using the provided key and claims.
func signToken(t *testing.T, key *rsa.PrivateKey, kid string, claims jwt.RegisteredClaims) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tok.Header["kid"] = kid
	signed, err := tok.SignedString(key)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return signed
}

// injectKey populates the key store with a test RSA public key.
func injectKey(t *testing.T, kid string, key *rsa.PrivateKey) {
	t.Helper()
	pub := key.Public().(*rsa.PublicKey)
	SetKeysForTesting(map[string]*rsa.PublicKey{kid: pub})
	t.Cleanup(func() { SetKeysForTesting(map[string]*rsa.PublicKey{}) })
}

const (
	testDomain   = "auth.example.com"
	testAudience = "mkauth-backend"
	testSubject  = "user-abc-123"
)

func validClaims(issuer, audience string) jwt.RegisteredClaims {
	return jwt.RegisteredClaims{
		Issuer:    issuer,
		Audience:  jwt.ClaimStrings{audience},
		Subject:   testSubject,
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
	}
}

func TestValidate_ValidToken(t *testing.T) {
	key := newTestRSAKey(t)
	kid := "kid-valid"
	injectKey(t, kid, key)

	token := signToken(t, key, kid, validClaims(fmt.Sprintf("https://%s", testDomain), testAudience))

	got, err := Validate(context.Background(), token, testDomain, testAudience)
	if err != nil {
		t.Fatalf("expected valid token, got error: %v", err)
	}
	if got.Subject != testSubject {
		t.Fatalf("expected subject %q, got %q", testSubject, got.Subject)
	}
}

func TestValidate_AudienceArray(t *testing.T) {
	key := newTestRSAKey(t)
	kid := "kid-audarray"
	injectKey(t, kid, key)

	// Zitadel sometimes sends multiple audiences
	claims := validClaims(fmt.Sprintf("https://%s", testDomain), testAudience)
	claims.Audience = jwt.ClaimStrings{"other-service", testAudience}
	token := signToken(t, key, kid, claims)

	_, err := Validate(context.Background(), token, testDomain, testAudience)
	if err != nil {
		t.Fatalf("expected valid token with aud array, got error: %v", err)
	}
}

func TestValidate_ExpiredToken(t *testing.T) {
	key := newTestRSAKey(t)
	kid := "kid-exp"
	injectKey(t, kid, key)

	claims := validClaims(fmt.Sprintf("https://%s", testDomain), testAudience)
	claims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-time.Minute))
	token := signToken(t, key, kid, claims)

	_, err := Validate(context.Background(), token, testDomain, testAudience)
	if err == nil {
		t.Fatal("expected error for expired token, got nil")
	}
}

func TestValidate_WrongAudience(t *testing.T) {
	key := newTestRSAKey(t)
	kid := "kid-aud"
	injectKey(t, kid, key)

	token := signToken(t, key, kid, validClaims(fmt.Sprintf("https://%s", testDomain), "wrong-service"))

	_, err := Validate(context.Background(), token, testDomain, testAudience)
	if err == nil {
		t.Fatal("expected error for wrong audience, got nil")
	}
}

func TestValidate_WrongIssuer(t *testing.T) {
	key := newTestRSAKey(t)
	kid := "kid-iss"
	injectKey(t, kid, key)

	token := signToken(t, key, kid, validClaims("https://evil.example.com", testAudience))

	_, err := Validate(context.Background(), token, testDomain, testAudience)
	if err == nil {
		t.Fatal("expected error for wrong issuer, got nil")
	}
}

func TestValidate_TamperedSignature(t *testing.T) {
	key := newTestRSAKey(t)
	kid := "kid-sig"
	injectKey(t, kid, key)

	token := signToken(t, key, kid, validClaims(fmt.Sprintf("https://%s", testDomain), testAudience))
	tampered := token[:len(token)-4] + "AAAA"

	_, err := Validate(context.Background(), tampered, testDomain, testAudience)
	if err == nil {
		t.Fatal("expected error for tampered signature, got nil")
	}
}

func TestValidate_MalformedJWT(t *testing.T) {
	tests := []struct{ name, token string }{
		{"two_parts", "header.payload"},
		{"empty", ""},
		{"garbage", "not!a!jwt"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Validate(context.Background(), tc.token, testDomain, testAudience)
			if err == nil {
				t.Fatalf("expected error for %q, got nil", tc.token)
			}
		})
	}
}

func TestValidate_UnknownKid(t *testing.T) {
	key := newTestRSAKey(t)
	other := newTestRSAKey(t)
	// Inject key under a different kid than the token will use
	injectKey(t, "different-kid", other)

	token := signToken(t, key, "unknown-kid", validClaims(fmt.Sprintf("https://%s", testDomain), testAudience))

	// Store is fresh so no refresh is attempted; kid lookup fails
	_, err := Validate(context.Background(), token, testDomain, testAudience)
	if err == nil {
		t.Fatal("expected error for unknown kid, got nil")
	}
}

// signTokenWithRoles builds an RS256 JWT carrying the Zitadel project-roles
// claim (urn:zitadel:iam:org:project:roles) in addition to the standard
// registered claims. Used to exercise Principal.ProjectRoles population.
func signTokenWithRoles(t *testing.T, key *rsa.PrivateKey, kid string, claims jwt.RegisteredClaims, roles map[string]any) string {
	t.Helper()
	mapClaims := jwt.MapClaims{
		"iss":                               claims.Issuer,
		"aud":                               []string(claims.Audience),
		"sub":                               claims.Subject,
		"exp":                               claims.ExpiresAt.Unix(),
		"iat":                               claims.IssuedAt.Unix(),
		"urn:zitadel:iam:org:project:roles": roles,
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, mapClaims)
	tok.Header["kid"] = kid
	signed, err := tok.SignedString(key)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return signed
}

func TestValidate_PopulatesProjectRoles(t *testing.T) {
	key := newTestRSAKey(t)
	kid := "kid-roles"
	injectKey(t, kid, key)

	token := signTokenWithRoles(t, key, kid,
		validClaims(fmt.Sprintf("https://%s", testDomain), testAudience),
		map[string]any{
			"admin":  map[string]any{"org-1": "Demo Org"},
			"viewer": map[string]any{"org-1": "Demo Org"},
		},
	)

	p, err := Validate(context.Background(), token, testDomain, testAudience)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if p.Subject != testSubject {
		t.Errorf("expected Subject=%q; got %q", testSubject, p.Subject)
	}
	if !p.HasProjectRole("admin") {
		t.Error("expected HasProjectRole(admin)=true")
	}
	if !p.HasProjectRole("viewer") {
		t.Error("expected HasProjectRole(viewer)=true")
	}
	if p.HasProjectRole("nonexistent") {
		t.Error("expected HasProjectRole(nonexistent)=false")
	}
}
