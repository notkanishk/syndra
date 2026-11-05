package claimserve

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"syndra/internal/claimtel"
	"syndra/internal/ratelimit"
)

// limitedDoor is the claims process with a signing key and a limiter of
// `failures` failed calls a minute per source, on store.
func limitedDoor(t *testing.T, failures int, store ratelimit.Store) (*Process, http.Handler, string) {
	t.Helper()
	_, r := process(t)
	key := strings.Repeat("k", 32)
	p := NewProcess(r, key, false)
	if err := p.Plane.Poll(context.Background()); err != nil {
		t.Fatal(err)
	}
	if store != nil {
		p.Limit = ratelimit.New(store, []ratelimit.Rule{{N: failures, Per: time.Minute}}, nil)
	}
	return p, p.Router(), key
}

func signed(key string) http.Header {
	return http.Header{SignatureHeader: {SignatureHeaderValue(time.Now(), []byte(u1Call), key)}}
}

// Zitadel calls on every sign-in from the same few addresses. A thousand
// signed calls from one source in a burst all pass, and none is counted.
func TestRateLimit_SignedCallsAreNeverLimited(t *testing.T) {
	p, h, key := limitedDoor(t, 5, ratelimit.Memory())
	for i := 0; i < 1000; i++ {
		if rr := post(t, h, u1Call, signed(key)); rr.Code != http.StatusOK {
			t.Fatalf("signed call %d: %d %s", i+1, rr.Code, rr.Body)
		}
	}
	if s := p.Limit.Stats(); len(s.Limited) != 0 {
		t.Fatalf("signed calls were counted: %+v", s)
	}
}

// Unsigned or wrongly signed calls from one source are refused 401 up to the
// limit and 429 with Retry-After past it, while a signed call from that same
// source still passes.
func TestRateLimit_FailuresAreLimitedAndSignedStillPass(t *testing.T) {
	p, h, key := limitedDoor(t, 5, ratelimit.Memory())
	forged := signed(strings.Repeat("x", 32))
	for i := 0; i < 5; i++ {
		hdr := forged
		if i%2 == 0 {
			hdr = nil // no signature at all
		}
		if rr := post(t, h, u1Call, hdr); rr.Code != http.StatusUnauthorized {
			t.Fatalf("failure %d: %d, want 401 under the limit", i+1, rr.Code)
		}
	}
	rr := post(t, h, u1Call, forged)
	var body map[string]string
	_ = json.Unmarshal(rr.Body.Bytes(), &body)
	if rr.Code != http.StatusTooManyRequests || rr.Header().Get("Retry-After") == "" || body["message"] != ratelimit.Message {
		t.Fatalf("past the limit: %d Retry-After=%q %v", rr.Code, rr.Header().Get("Retry-After"), body)
	}
	if rr := post(t, h, u1Call, signed(key)); rr.Code != http.StatusOK {
		t.Fatalf("a signed call from the limited source: %d %s", rr.Code, rr.Body)
	}
	if rr := post(t, h, u1Call, nil); rr.Code != http.StatusTooManyRequests {
		t.Fatalf("the signed call reset the bucket: %d", rr.Code)
	}

	// The refused calls still land in their outcome class, and health shows
	// the count per endpoint.
	if got := p.Rec.Recent(HealthWindowMinutes); got[claimtel.SigMismatch]+got[claimtel.SigMissing] != 7 {
		t.Fatalf("outcomes %v, want 7 signature failures", got)
	}
	hr := httptest.NewRecorder()
	h.ServeHTTP(hr, httptest.NewRequest(http.MethodGet, "/healthz/claims", nil))
	var health Health
	_ = json.Unmarshal(hr.Body.Bytes(), &health)
	if health.RateLimits == nil || health.RateLimits.Limited["POST /api/action/inject"] != 2 {
		t.Fatalf("health rate_limits = %+v", health.RateLimits)
	}
}

// Redis down or hung: nothing is limited, every failure is a plain 401 and
// every signed call passes.
func TestRateLimit_RedisDownAllowsEverything(t *testing.T) {
	for name, store := range map[string]ratelimit.Store{
		"down": func(context.Context, []string, []ratelimit.Rule, time.Time) (time.Duration, error) {
			return 0, errors.New("dial tcp: connection refused")
		},
		"hung": func(ctx context.Context, _ []string, _ []ratelimit.Rule, _ time.Time) (time.Duration, error) {
			<-ctx.Done()
			return 0, ctx.Err()
		},
	} {
		p, h, key := limitedDoor(t, 1, store)
		for i := 0; i < 20; i++ {
			if rr := post(t, h, u1Call, nil); rr.Code != http.StatusUnauthorized {
				t.Fatalf("%s: failure %d answered %d", name, i+1, rr.Code)
			}
			if rr := post(t, h, u1Call, signed(key)); rr.Code != http.StatusOK {
				t.Fatalf("%s: signed call answered %d", name, rr.Code)
			}
		}
		if s := p.Limit.Stats(); s.FailOpen != 20 {
			t.Fatalf("%s: fail_open = %d, want 20", name, s.FailOpen)
		}
	}
}

// SYNDRA_RATE_LIMITS=off is a nil limiter: no failure is ever 429.
func TestRateLimit_KillSwitch(t *testing.T) {
	p, h, _ := limitedDoor(t, 0, nil)
	p.Limit = ratelimit.FromEnv(nil, func(k string) string {
		if k == ratelimit.EnvSwitch {
			return "off"
		}
		return ""
	})
	for i := 0; i < 100; i++ {
		if rr := post(t, h, u1Call, nil); rr.Code != http.StatusUnauthorized {
			t.Fatalf("failure %d with the limits off: %d", i+1, rr.Code)
		}
	}
}
