package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"syndra/internal/claimserve"
	"syndra/internal/ratelimit"
)

func withLimiter(t *testing.T, l *ratelimit.Limiter) {
	t.Helper()
	old := rateLimiter
	t.Cleanup(func() { rateLimiter = old })
	rateLimiter = l
}

// A member's own password and storage writes, through the real router: ten
// a minute per person pass, the eleventh is 429 with Retry-After, and another
// person is untouched. The handlers refuse these bodies before any store is
// read, so every non-429 answer is the handler's own.
func TestRateLimit_SelfServiceWritesPerPerson(t *testing.T) {
	t.Setenv("ZITADEL_DOMAIN", "")
	t.Setenv("SYNDRA_API_KEY", "test-key")
	withLimiter(t, ratelimit.New(ratelimit.Memory(), nil, ratelimit.DefaultSelfService))
	h := NewRouter()

	send := func(method, path, who, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer test-key")
		r.Header.Set(demoSubjectHeader, who)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, r)
		return rr
	}
	for _, route := range []struct{ method, path, body string }{
		{http.MethodPost, "/api/v1/me/targets/nas/credential", "{"},             // 400, malformed
		{http.MethodDelete, "/api/v1/users/someone-else/shadow-credential", ""}, // 403, not theirs
	} {
		for i := 0; i < 10; i++ {
			if rr := send(route.method, route.path, "u1", route.body); rr.Code != http.StatusBadRequest && rr.Code != http.StatusForbidden {
				t.Fatalf("%s %s: write %d answered %d, want the handler's own refusal", route.method, route.path, i+1, rr.Code)
			}
		}
		rr := send(route.method, route.path, "u1", route.body)
		if rr.Code != http.StatusTooManyRequests || rr.Header().Get("Retry-After") == "" || !strings.Contains(rr.Body.String(), ratelimit.Message) {
			t.Fatalf("%s %s: 11th write %d Retry-After=%q %s", route.method, route.path, rr.Code, rr.Header().Get("Retry-After"), rr.Body)
		}
		if rr := send(route.method, route.path, "u2", route.body); rr.Code == http.StatusTooManyRequests {
			t.Fatalf("%s %s: another person was limited", route.method, route.path)
		}
	}
	if s := rateLimiter.Stats(); s.Limited["POST /api/v1/me/targets/{target}/credential"] != 1 || s.Limited["DELETE /api/v1/users/{uid}/shadow-credential"] != 1 {
		t.Fatalf("stats %+v", s)
	}
}

// The event webhook shares the claims door's check: failures from one source
// are 429 past the limit, and the kill switch puts them back to 401.
func TestRateLimit_WebhookFailures(t *testing.T) {
	t.Setenv("ZITADEL_DOMAIN", "zitadel.example.org")
	t.Setenv("ZITADEL_EVENT_SIGNING_KEY", strings.Repeat("e", 32))
	h := NewRouter()
	call := func() int {
		r := httptest.NewRequest(http.MethodPost, "/api/webhooks/zitadel", strings.NewReader(`{}`))
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, r)
		return rr.Code
	}

	withLimiter(t, ratelimit.New(ratelimit.Memory(), []ratelimit.Rule{{N: 3, Per: time.Minute}}, nil))
	for i := 0; i < 3; i++ {
		if c := call(); c != http.StatusUnauthorized {
			t.Fatalf("failure %d: %d", i+1, c)
		}
	}
	if c := call(); c != http.StatusTooManyRequests {
		t.Fatalf("past the limit: %d", c)
	}

	rateLimiter = nil // SYNDRA_RATE_LIMITS=off
	for i := 0; i < 10; i++ {
		if c := call(); c != http.StatusUnauthorized {
			t.Fatalf("limits off, failure %d: %d", i+1, c)
		}
	}
}

// A webhook call that verifies is never counted or refused, however many come
// from one source, and it never spends the bucket a forged call from that
// source draws on. The limit sits only after verification has failed.
func TestRateLimit_VerifiedWebhookCallsAreNeverCounted(t *testing.T) {
	t.Setenv("ZITADEL_DOMAIN", "zitadel.example.org")
	key := strings.Repeat("e", 32)
	t.Setenv("ZITADEL_EVENT_SIGNING_KEY", key)
	h := NewRouter()
	body := []byte(`{}`)
	call := func(signed bool) int {
		r := httptest.NewRequest(http.MethodPost, "/api/webhooks/zitadel", strings.NewReader(string(body)))
		if signed {
			r.Header.Set(claimserve.SignatureHeader, claimserve.SignatureHeaderValue(time.Now(), body, key))
		}
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, r)
		return rr.Code
	}

	l := ratelimit.New(ratelimit.Memory(), []ratelimit.Rule{{N: 2, Per: time.Minute}}, nil)
	withLimiter(t, l)
	for i := 0; i < 200; i++ {
		if c := call(true); c == http.StatusUnauthorized || c == http.StatusTooManyRequests {
			t.Fatalf("verified call %d refused: %d", i+1, c)
		}
	}
	if s := l.Stats(); len(s.Limited) != 0 {
		t.Fatalf("verified calls were counted: %+v", s)
	}
	// The bucket is untouched: two failures still answer 401, the third 429,
	// and a verified call from that same source still passes.
	for i := 0; i < 2; i++ {
		if c := call(false); c != http.StatusUnauthorized {
			t.Fatalf("failure %d: %d", i+1, c)
		}
	}
	if c := call(false); c != http.StatusTooManyRequests {
		t.Fatalf("past the limit: %d", c)
	}
	if c := call(true); c == http.StatusUnauthorized || c == http.StatusTooManyRequests {
		t.Fatalf("verified call from a limited source: %d", c)
	}
}
