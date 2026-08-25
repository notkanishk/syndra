package main

import (
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

// 4.x — the channel into the least trusted component in the system.
//
// Isolation is what running this add-on separately buys the deployment, and
// isolation is worth nothing if anything on the internal network can order a
// credential reset. Every case here is a way in that must not work.

const testKey = "a-signing-key"

func signedAuth(now time.Time) *authenticator {
	return &authenticator{signingKey: []byte(testKey), now: func() time.Time { return now }}
}

func signedRequest(t *testing.T, body string, ts time.Time, key string) *http.Request {
	t.Helper()
	return signedRequestTo(t, http.MethodPost, "/operations/password.set", body, ts, key)
}

func signedRequestTo(t *testing.T, method, path, body string, ts time.Time, key string) *http.Request {
	t.Helper()
	mac := computeMAC(ts.Unix(), method, path, []byte(body), []byte(key))
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	// Built independently of the header the producer writes, so a test that
	// agrees with the producer about a wrong format still fails.
	r.Header.Set(SignatureHeader, "t="+strconv.FormatInt(ts.Unix(), 10)+",v1="+hex.EncodeToString(mac))
	return r
}

func TestASignatureAuthenticatesTheBodyAndTheTimestamp(t *testing.T) {
	now := time.Now()
	a := signedAuth(now)

	body, err := a.verify(signedRequest(t, `{"call_id":"c1"}`, now, testKey))
	if err != nil {
		t.Fatalf("a correctly signed request must be accepted: %v", err)
	}
	if string(body) != `{"call_id":"c1"}` {
		t.Fatalf("the verified bytes must be the ones handed on, got %q", body)
	}
}

// The body is inside the MAC, so a signature authenticates WHAT was asked and
// not only who asked.
func TestAnEditedBodyIsRefused(t *testing.T) {
	now := time.Now()
	a := signedAuth(now)

	tampered := signedRequest(t, `{"call_id":"c1"}`, now, testKey)
	tampered.Body = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"call_id":"c2"}`)).Body

	if _, err := a.verify(tampered); !errors.Is(err, errBadSignature) {
		t.Fatalf("want errBadSignature, got %v", err)
	}
}

// The timestamp is inside the MAC too, so it cannot be edited to extend a
// captured signature's life — and it is checked before the MAC, so an expired
// one is refused whether or not it verifies.
func TestAStaleOrFutureSignatureIsRefused(t *testing.T) {
	now := time.Now()
	a := signedAuth(now)

	for _, tc := range []struct {
		name string
		skew time.Duration
	}{
		{"captured and replayed later", -signatureTolerance - time.Second},
		{"dated into the future", signatureTolerance + time.Second},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := a.verify(signedRequest(t, `{}`, now.Add(tc.skew), testKey)); !errors.Is(err, errStaleSignature) {
				t.Fatalf("want errStaleSignature, got %v", err)
			}
		})
	}

	// And ordinary clock skew between two containers on one host is absorbed.
	if _, err := a.verify(signedRequest(t, `{}`, now.Add(-signatureTolerance+time.Second), testKey)); err != nil {
		t.Errorf("a request inside the window must be accepted: %v", err)
	}
}

func TestAnUnsignedOrWrongKeyRequestIsRefused(t *testing.T) {
	now := time.Now()
	a := signedAuth(now)

	bare := httptest.NewRequest(http.MethodPost, "/operations/password.set", strings.NewReader(`{}`))
	if _, err := a.verify(bare); !errors.Is(err, errNoSignature) {
		t.Errorf("an unsigned request must be refused: %v", err)
	}

	if _, err := a.verify(signedRequest(t, `{}`, now, "not-the-key")); !errors.Is(err, errBadSignature) {
		t.Errorf("a signature under another key must be refused: %v", err)
	}

	malformed := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{}`))
	malformed.Header.Set(SignatureHeader, "just-some-text")
	if _, err := a.verify(malformed); !errors.Is(err, errNoSignature) {
		t.Errorf("a malformed header must be refused: %v", err)
	}
}

// An authenticator with no signing key authenticates nothing, so it must refuse
// everything rather than admit everything.
//
// `loadConfig` cannot produce this state — it refuses to start without a
// secret — so this guards the constructor, not the configuration. The mTLS mode
// this replaced had the mirror-image hazard: it treated an empty signing key as
// "the handshake did it", which was correct only for as long as a handshake
// really had. An open door reached by an unreachable path is still an open
// door, and the path stops being unreachable one refactor later.
func TestAnAuthenticatorWithNoKeyRefusesEverything(t *testing.T) {
	a := &authenticator{now: time.Now}

	plain := httptest.NewRequest(http.MethodPost, "/operations/password.set", strings.NewReader(`{}`))
	if _, err := a.verify(plain); !errors.Is(err, errNoSigningKey) {
		t.Fatalf("want errNoSigningKey, got %v", err)
	}
	// And a validly-signed request is refused too: with no key there is nothing
	// to verify it against, so accepting it would be accepting the signature's
	// mere presence.
	signed := signedRequest(t, `{}`, time.Now(), testKey)
	if _, err := a.verify(signed); !errors.Is(err, errNoSigningKey) {
		t.Fatalf("a signature must not be trusted by an authenticator holding no key: %v", err)
	}
}

// A body is bounded before it is read. "Read it all and then check" is how a
// service with one caller becomes a memory-exhaustion target.
func TestAnOversizedBodyIsRefusedRatherThanBuffered(t *testing.T) {
	now := time.Now()
	a := signedAuth(now)

	huge := strings.Repeat("x", maxRequestBytes+1)
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(huge))
	r.Header.Set(SignatureHeader, "t=1,v1=00")
	if _, err := a.verify(r); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("want a size refusal, got %v", err)
	}
}

// The refusal says nothing. A caller that cannot authenticate has no business
// learning which half of the check it failed, and the operator debugging a real
// misconfiguration reads the add-on's log.
func TestAnUnauthenticatedResponseExplainsNothing(t *testing.T) {
	a := signedAuth(time.Now())
	handler := a.authenticated(func(http.ResponseWriter, *http.Request, []byte) {
		t.Fatal("the handler must not run")
	})

	rr := httptest.NewRecorder()
	handler(rr, httptest.NewRequest(http.MethodPost, "/operations/password.set", strings.NewReader(`{"password":"hunter2"}`)))

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", rr.Code)
	}
	for _, leak := range []string{"signature", "timestamp", "hunter2"} {
		if strings.Contains(strings.ToLower(rr.Body.String()), leak) {
			t.Errorf("the response must not mention %q: %s", leak, rr.Body.String())
		}
	}
}

// The operation name is in the PATH and in nothing else. `GET /capabilities`
// carries an empty body, so a MAC over the timestamp and body alone was a
// function of the timestamp — valid for any zero-body request in the window,
// whichever path it was replayed at. The call-id dedup blocked the sequential
// version of that, which is a mitigation standing where a check should be.
func TestASignatureIsBoundToTheMethodAndPath(t *testing.T) {
	now := time.Now()
	a := signedAuth(now)

	// Signed for the capabilities read, replayed at the credential reset.
	captured := signedRequestTo(t, http.MethodGet, "/capabilities", "", now, testKey)
	replayed := httptest.NewRequest(http.MethodPost, "/operations/password.set", strings.NewReader(""))
	replayed.Header.Set(SignatureHeader, captured.Header.Get(SignatureHeader))

	if _, err := a.verify(replayed); !errors.Is(err, errBadSignature) {
		t.Fatalf("a signature must not travel between paths: %v", err)
	}

	// The same body and timestamp under a different method is equally not it.
	sameBody := signedRequestTo(t, http.MethodPost, "/apply", `{"call_id":"c1"}`, now, testKey)
	otherMethod := httptest.NewRequest(http.MethodGet, "/apply", strings.NewReader(`{"call_id":"c1"}`))
	otherMethod.Header.Set(SignatureHeader, sameBody.Header.Get(SignatureHeader))
	if _, err := a.verify(otherMethod); !errors.Is(err, errBadSignature) {
		t.Fatalf("a signature must not travel between methods: %v", err)
	}

	// And the request it was actually made for still verifies.
	if _, err := a.verify(signedRequestTo(t, http.MethodGet, "/capabilities", "", now, testKey)); err != nil {
		t.Fatalf("the signed request must still be accepted: %v", err)
	}
}
