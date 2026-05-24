package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"mkauth/internal/auth"
)

// TestWithOperatorAuth_ParsesJWTExactlyOnce asserts the C4 contract by
// invoking the real withOperatorAuth(handler) chain and counting calls to
// jwtValidate. A regression where withOperatorAuth re-extracts the bearer
// token and re-parses (the pre-C4 behaviour) would bump the count to 2.
func TestWithOperatorAuth_ParsesJWTExactlyOnce(t *testing.T) {
	t.Setenv("ZITADEL_DOMAIN", "example.zitadel.cloud")
	t.Setenv("ZITADEL_AUDIENCE", "test-aud")
	t.Setenv("ZITADEL_ADMIN_ROLE_KEY", "admin")

	var validateCalls int
	origValidate := jwtValidate
	jwtValidate = func(ctx context.Context, tokenStr, domain, audience string) (*auth.Principal, error) {
		validateCalls++
		if tokenStr != "fake-jwt-fixture" {
			t.Errorf("jwtValidate received unexpected token %q", tokenStr)
		}
		if domain != "example.zitadel.cloud" || audience != "test-aud" {
			t.Errorf("jwtValidate received unexpected domain/audience: %q / %q", domain, audience)
		}
		return &auth.Principal{
			Subject:      "operator-1",
			ProjectRoles: map[string]struct{}{"admin": {}},
		}, nil
	}
	t.Cleanup(func() { jwtValidate = origValidate })

	var innerCalls int
	var observedSubject string
	inner := func(w http.ResponseWriter, r *http.Request) {
		innerCalls++
		if p := principalFromContext(r.Context()); p != nil {
			observedSubject = p.Subject
		}
		w.WriteHeader(http.StatusOK)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/zitadel/users", nil)
	req.Header.Set("Authorization", "Bearer fake-jwt-fixture")
	rr := httptest.NewRecorder()

	withOperatorAuth(inner)(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for admin principal; got %d: %s", rr.Code, rr.Body.String())
	}
	if innerCalls != 1 {
		t.Fatalf("expected inner handler called once; got %d", innerCalls)
	}
	if validateCalls != 1 {
		t.Fatalf("C4 contract violated: jwtValidate called %d times; expected exactly 1 (withOperatorAuth must NOT re-parse the bearer token)", validateCalls)
	}
	if observedSubject != "operator-1" {
		t.Fatalf("expected inner handler to read principal.Subject=operator-1; got %q", observedSubject)
	}
}

// TestWithOperatorAuth_DeniesWhenAdminRoleMissing asserts the role gate on
// the real withOperatorAuth, again going through jwtValidate so the JWT
// parse path is exercised end-to-end.
func TestWithOperatorAuth_DeniesWhenAdminRoleMissing(t *testing.T) {
	t.Setenv("ZITADEL_DOMAIN", "example.zitadel.cloud")
	t.Setenv("ZITADEL_AUDIENCE", "test-aud")
	t.Setenv("ZITADEL_ADMIN_ROLE_KEY", "admin")

	origValidate := jwtValidate
	jwtValidate = func(ctx context.Context, tokenStr, domain, audience string) (*auth.Principal, error) {
		return &auth.Principal{
			Subject:      "viewer-1",
			ProjectRoles: map[string]struct{}{"viewer": {}},
		}, nil
	}
	t.Cleanup(func() { jwtValidate = origValidate })

	var innerCalls int
	inner := func(w http.ResponseWriter, _ *http.Request) {
		innerCalls++
		w.WriteHeader(http.StatusOK)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/zitadel/users", nil)
	req.Header.Set("Authorization", "Bearer any")
	rr := httptest.NewRecorder()

	withOperatorAuth(inner)(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403 when admin role missing; got %d: %s", rr.Code, rr.Body.String())
	}
	if innerCalls != 0 {
		t.Fatalf("inner handler must not be invoked when role check fails; got %d calls", innerCalls)
	}
}

// TestWithOperatorAuth_RejectsMissingBearerToken keeps the existing
// "missing token → 401" contract guarded after the refactor.
func TestWithOperatorAuth_RejectsMissingBearerToken(t *testing.T) {
	t.Setenv("ZITADEL_DOMAIN", "example.zitadel.cloud")
	t.Setenv("ZITADEL_AUDIENCE", "test-aud")

	origValidate := jwtValidate
	jwtValidate = func(context.Context, string, string, string) (*auth.Principal, error) {
		t.Error("jwtValidate must not be called when bearer token is missing")
		return nil, nil
	}
	t.Cleanup(func() { jwtValidate = origValidate })

	req := httptest.NewRequest(http.MethodGet, "/api/v1/zitadel/users", nil)
	rr := httptest.NewRecorder()
	withOperatorAuth(func(http.ResponseWriter, *http.Request) {})(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 when bearer token missing; got %d", rr.Code)
	}
}
