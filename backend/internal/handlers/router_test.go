package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"syndra/internal/auth"
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

// stubMemberAuth puts the router in production mode with a jwtValidate stub
// that returns a plain member principal (no admin role) for any token.
func stubMemberAuth(t *testing.T, subject string) {
	t.Helper()
	t.Setenv("ZITADEL_DOMAIN", "example.zitadel.cloud")
	t.Setenv("ZITADEL_AUDIENCE", "test-aud")
	t.Setenv("ZITADEL_ADMIN_ROLE_KEY", "admin")
	origValidate := jwtValidate
	jwtValidate = func(context.Context, string, string, string) (*auth.Principal, error) {
		return &auth.Principal{
			Subject:      subject,
			ProjectRoles: map[string]struct{}{"member": {}},
		}, nil
	}
	t.Cleanup(func() { jwtValidate = origValidate })
}

// TestRouter_MemberDeniedOnOperatorRoutes is the SC1/SC3 regression: a plain
// authenticated member must get 403 — through the REAL router wiring, so a
// future withOperatorAuth→withUserAuth downgrade on any of these routes fails
// this test. Grant upsert and request decision are the privilege-escalation
// paths; audit and cross-user reads are the data-exposure paths. All 403
// before the handler runs, so no db stubs are needed.
func TestRouter_MemberDeniedOnOperatorRoutes(t *testing.T) {
	stubMemberAuth(t, "member-1")
	router := NewRouter()

	cases := []struct{ method, path string }{
		{http.MethodPost, "/api/v1/users/member-1/grants"}, // even a self-grant
		{http.MethodPost, "/api/v1/requests/req-1/decision"},
		{http.MethodGet, "/api/v1/audit"},
		{http.MethodGet, "/api/v1/users/someone-else/grants"},
		{http.MethodGet, "/api/v1/users/someone-else/access"},
		{http.MethodGet, "/api/v1/users/someone-else/bundles"},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(`{}`))
		req.Header.Set("Authorization", "Bearer member-token")
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		if rr.Code != http.StatusForbidden {
			t.Errorf("%s %s: expected 403 for member; got %d: %s", tc.method, tc.path, rr.Code, rr.Body.String())
		}
	}
}

// TestWithSelfOrOperatorAuth_AllowsSelfDeniesOther pins the self-scoping
// middleware: a member reaches the handler for their own {id}, never for
// another user's.
func TestWithSelfOrOperatorAuth_AllowsSelfDeniesOther(t *testing.T) {
	stubMemberAuth(t, "member-1")

	var innerCalls int
	inner := func(w http.ResponseWriter, _ *http.Request) {
		innerCalls++
		w.WriteHeader(http.StatusOK)
	}

	other := httptest.NewRequest(http.MethodGet, "/api/v1/users/other/grants", nil)
	other.SetPathValue("id", "other")
	other.Header.Set("Authorization", "Bearer t")
	rr := httptest.NewRecorder()
	withSelfOrOperatorAuth(inner)(rr, other)
	if rr.Code != http.StatusForbidden || innerCalls != 0 {
		t.Fatalf("cross-user read: expected 403 with handler untouched; got %d, calls=%d", rr.Code, innerCalls)
	}

	self := httptest.NewRequest(http.MethodGet, "/api/v1/users/member-1/grants", nil)
	self.SetPathValue("id", "member-1")
	self.Header.Set("Authorization", "Bearer t")
	rr = httptest.NewRecorder()
	withSelfOrOperatorAuth(inner)(rr, self)
	if rr.Code != http.StatusOK || innerCalls != 1 {
		t.Fatalf("self read: expected 200 with handler invoked; got %d, calls=%d", rr.Code, innerCalls)
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
