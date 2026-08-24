package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// Demo mode had no way to say who was asking.
//
// The UI proxy sends the shared API key and, without a token, nothing else — so
// `resolveActor` answered "system" for every `/me/*` route: a subject with no
// entitlement, no binding and no account. The member storage page therefore
// could not be exercised on any deployment without a live Zitadel, which is why
// nobody noticed that middleware was redirecting members away from it. A path
// that cannot be tested is a path that is broken quietly.

func demoRequest(t *testing.T, subject string) string {
	t.Helper()
	t.Setenv("SYNDRA_API_KEY", "test-key")

	var seen string
	handler := withAPIKeyAuth(func(_ http.ResponseWriter, r *http.Request) {
		seen = resolveActor(r, "")
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me/targets", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	if subject != "" {
		req.Header.Set(demoSubjectHeader, subject)
	}
	handler(httptest.NewRecorder(), req)
	return seen
}

func TestDemoSubjectNamesTheActor(t *testing.T) {
	if got := demoRequest(t, "sam_student"); got != "sam_student" {
		t.Fatalf("want the named subject, got %q", got)
	}
}

func TestWithoutTheHeaderNothingChanges(t *testing.T) {
	if got := demoRequest(t, ""); got != "system" {
		t.Fatalf("the previous behaviour must survive an absent header, got %q", got)
	}
}

// The property that makes this safe rather than a back door.
//
// `withUserAuth` reaches `withAPIKeyAuth` only when ZITADEL_DOMAIN is unset.
// With a domain set, the shared key is refused outright and a validated token
// is the only way in — so a header claiming to be somebody is never read, and
// the request never reaches the handler at all.
func TestTheDemoHeaderIsInertInProduction(t *testing.T) {
	t.Setenv("ZITADEL_DOMAIN", "zitadel.example.org")
	t.Setenv("ZITADEL_AUDIENCE", "syndra")
	t.Setenv("SYNDRA_API_KEY", "test-key")

	reached := false
	handler := withUserAuth(func(http.ResponseWriter, *http.Request) { reached = true })

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me/targets", nil)
	// The shared key AND a subject claim: everything an attacker holding the
	// key would send.
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set(demoSubjectHeader, "dev_admin")
	rr := httptest.NewRecorder()
	handler(rr, req)

	if reached {
		t.Fatal("the shared key must not authenticate anything once a domain is configured")
	}
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", rr.Code)
	}
}

// It identifies; it does not grant. Demo mode has no role source, and inventing
// roles here would turn a header into a privilege rather than a name.
func TestTheDemoSubjectCarriesNoRoles(t *testing.T) {
	t.Setenv("SYNDRA_API_KEY", "test-key")

	var hasRole bool
	handler := withAPIKeyAuth(func(_ http.ResponseWriter, r *http.Request) {
		hasRole = principalFromContext(r.Context()).HasProjectRole("admin")
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me/targets", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	req.Header.Set(demoSubjectHeader, "sam_student")
	handler(httptest.NewRecorder(), req)

	if hasRole {
		t.Fatal("naming a subject must not confer a role on them")
	}
}
