package addons

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// verifySignature is an INDEPENDENT implementation of the check an add-on
// performs. Deliberately not calling ComputeSignature: a verifier built from
// the producer proves the producer agrees with itself, which is the one thing
// that was never in doubt.
func verifySignature(header string, body, key []byte, tolerance time.Duration, now time.Time) error {
	var ts int64
	var sig []byte
	for _, pair := range strings.Split(header, ",") {
		k, v, ok := strings.Cut(pair, "=")
		if !ok {
			return fmt.Errorf("malformed pair %q", pair)
		}
		switch k {
		case "t":
			n, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				return fmt.Errorf("bad timestamp: %w", err)
			}
			ts = n
		case "v1":
			b, err := hex.DecodeString(v)
			if err != nil {
				return fmt.Errorf("bad signature hex: %w", err)
			}
			sig = b
		}
	}
	if ts == 0 || sig == nil {
		return errors.New("header missing t= or v1=")
	}
	skew := now.Sub(time.Unix(ts, 0))
	if skew < 0 {
		skew = -skew
	}
	if skew > tolerance {
		return errors.New("timestamp outside tolerance")
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(strconv.FormatInt(ts, 10) + "."))
	mac.Write(body)
	if !hmac.Equal(mac.Sum(nil), sig) {
		return errors.New("signature does not match")
	}
	return nil
}

// signedAddon wires a target at srv authenticated by a signing key on disk.
func signedAddon(t *testing.T, srvURL string, key []byte) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "sign.key")
	writeFile(t, path, key)
	installAddon(t, Registration{Target: "truenas", BaseURL: srvURL, SigningKeyPath: path}, goodManifest())
}

// passwordSet builds a dispatch that already satisfies backend policy's
// parameter schema, since almost no test here is about the schema. A nil
// argument means "valid, uninteresting" rather than "absent": password.set
// declares one required string, and an absent one is now a refusal.
func passwordSet(params map[string]any) CallRequest {
	if params == nil {
		params = map[string]any{"password": "a-value"}
	}
	return CallRequest{
		Target:    "truenas",
		Operation: "password.set",
		// Constructed directly because this file is in the package. No caller
		// outside it can do this, which is the entire point of the type.
		Record:      DispatchRecord{callID: "rec-0001", target: "truenas", operation: "password.set", subject: "user-42"},
		Subject:     "user-42",
		PlanID:      "plan-0001",
		Fingerprint: "sha256:abc",
		Params:      params,
	}
}

// withCallTimeout shortens the dispatch bound so a timeout test costs
// milliseconds.
func withCallTimeout(t *testing.T, d time.Duration) {
	t.Helper()
	saved := callTimeout
	callTimeout = d
	t.Cleanup(func() { callTimeout = saved })
}

func withBreaker(t *testing.T, threshold int, cooldown time.Duration) {
	t.Helper()
	st, sc := breakerThreshold, breakerCooldown
	breakerThreshold, breakerCooldown = threshold, cooldown
	t.Cleanup(func() { breakerThreshold, breakerCooldown = st, sc })
}

// 2.5 — every mutating call carries the operation id, the record id, the plan
// id, and the fingerprint, and carries them INSIDE the signed body rather than
// beside it.
func TestCallCarriesPlanFingerprintAndOperationIdUnderSignature(t *testing.T) {
	key := []byte("a-real-signing-key")
	var (
		mu   sync.Mutex
		env  callEnvelope
		sigE error
		seen bool
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		defer mu.Unlock()
		seen = true
		sigE = verifySignature(r.Header.Get(SignatureHeader), body, key, time.Minute, time.Now())
		_ = json.Unmarshal(body, &env)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	signedAddon(t, srv.URL, key)
	resp := Call(context.Background(), passwordSet(map[string]any{"password": "hunter2"}))

	if resp.Outcome != OutcomeSucceeded {
		t.Fatalf("outcome = %s (err %v), want succeeded", resp.Outcome, resp.Err)
	}
	mu.Lock()
	defer mu.Unlock()
	if !seen {
		t.Fatal("add-on never received the call")
	}
	if sigE != nil {
		t.Fatalf("add-on could not verify the signature: %v", sigE)
	}
	if env.Operation != "password.set" || env.CallID != "rec-0001" ||
		env.PlanID != "plan-0001" || env.Fingerprint != "sha256:abc" || env.Subject != "user-42" {
		t.Fatalf("envelope lost binding fields: %+v", env)
	}
	if env.ContractVersion != ContractVersion {
		t.Fatalf("envelope contract version = %d, want %d", env.ContractVersion, ContractVersion)
	}
}

// 2.36 — the signature binds the body AND the timestamp. A signature that
// covered neither would be a shared secret with extra steps: it would prove who
// called and nothing about what they asked or when.
func TestSignatureBindsBodyAndTimestamp(t *testing.T) {
	key := []byte("a-real-signing-key")
	var (
		mu     sync.Mutex
		header string
		body   []byte
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		header, body = r.Header.Get(SignatureHeader), b
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	signedAddon(t, srv.URL, key)
	if resp := Call(context.Background(), passwordSet(map[string]any{"password": "hunter2"})); resp.Outcome != OutcomeSucceeded {
		t.Fatalf("setup call failed: %s %v", resp.Outcome, resp.Err)
	}

	mu.Lock()
	defer mu.Unlock()
	now := time.Now()
	if err := verifySignature(header, body, key, time.Minute, now); err != nil {
		t.Fatalf("captured signature should verify as sent: %v", err)
	}

	t.Run("a different body under the same signature is refused", func(t *testing.T) {
		tampered := append([]byte(nil), body...)
		tampered = []byte(strings.Replace(string(tampered), "user-42", "user-99", 1))
		if len(tampered) != len(body) {
			t.Fatal("test built a tampered body of a different length; the point is content, not length")
		}
		if err := verifySignature(header, tampered, key, time.Minute, now); err == nil {
			t.Fatal("a rewritten subject verified under the original signature")
		}
	})

	t.Run("a stretched timestamp is refused", func(t *testing.T) {
		// Take the same v1 and claim it was issued an hour later, the move that
		// extends a captured call's life.
		_, v1, _ := strings.Cut(header, ",")
		stretched := fmt.Sprintf("t=%d,%s", now.Add(time.Hour).Unix(), v1)
		if err := verifySignature(stretched, body, key, 2*time.Hour, now.Add(time.Hour)); err == nil {
			t.Fatal("a signature verified under a timestamp it was not computed over")
		}
	})

	t.Run("the header is not a reusable constant", func(t *testing.T) {
		// A bare shared secret would produce the same header every time and
		// replay forever. This one must not.
		second := ComputeSignature(now.Add(time.Second), body, key)
		if second == header {
			t.Fatal("two calls at different times produced an identical credential")
		}
	})
}

// 2.35, 2.36 — mutual TLS against a private CA: our client is admitted, and a
// caller without a certificate this deployment's CA issued is refused by the
// handshake, before any handler runs.
func TestMutualTLSAdmitsTheBackendAndRefusesEveryoneElse(t *testing.T) {
	pki := newTestPKI(t, time.Now().Add(365*24*time.Hour))
	var handled atomic.Int32
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handled.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	srv.TLS = &tls.Config{
		Certificates: []tls.Certificate{pki.serverCert},
		ClientCAs:    pki.caPool(t),
		ClientAuth:   tls.RequireAndVerifyClientCert,
		MinVersion:   tls.VersionTLS13,
	}
	srv.StartTLS()
	defer srv.Close()

	t.Run("the registered client is admitted", func(t *testing.T) {
		installAddon(t, pki.registration("truenas", srv.URL), goodManifest())
		resp := Call(context.Background(), passwordSet(map[string]any{"password": "hunter2"}))
		if resp.Outcome != OutcomeSucceeded {
			t.Fatalf("outcome = %s (err %v), want succeeded", resp.Outcome, resp.Err)
		}
	})

	t.Run("a certificate from another CA is refused", func(t *testing.T) {
		before := handled.Load()
		r := pki.registration("truenas", srv.URL)
		r.ClientCertPath, r.ClientKeyPath = pki.otherCertPath, pki.otherKeyPath
		installAddon(t, r, goodManifest())
		resp := Call(context.Background(), passwordSet(map[string]any{"password": "hunter2"}))
		if resp.Outcome == OutcomeSucceeded {
			t.Fatal("an untrusted client certificate was accepted")
		}
		if handled.Load() != before {
			t.Fatal("the handler ran for a caller the CA never issued")
		}
	})

	t.Run("no client certificate at all is refused", func(t *testing.T) {
		before := handled.Load()
		c := &http.Client{Transport: &http.Transport{
			TLSClientConfig: &tls.Config{RootCAs: pki.caPool(t), MinVersion: tls.VersionTLS13},
		}}
		if _, err := c.Get(srv.URL + "/capabilities"); err == nil {
			t.Fatal("a client presenting no certificate completed the handshake")
		}
		if handled.Load() != before {
			t.Fatal("the handler ran for an unauthenticated caller")
		}
	})
}

// 2.6 — each failure mode maps to exactly one outcome, and none of them maps to
// success. The table is the contract: a future status code added to
// classifyStatus has to declare which of the three non-success meanings it has.
func TestFailureModesMapToTheirOutcome(t *testing.T) {
	cases := []struct {
		name        string
		status      int
		want        Outcome
		terminal    bool
		retryable   bool
		description string
	}{
		{"applied", 200, OutcomeSucceeded, true, false, "the add-on acted and said so"},
		{"validation refusal", 400, OutcomeRejected, true, false, "it refused and did not act"},
		{"not found", 404, OutcomeRejected, true, false, "deterministic, retrying changes nothing"},
		{"forbidden", 403, OutcomeRejected, true, false, "deterministic"},
		{"backpressure", 429, OutcomeUnreached, false, true, "explicitly did not act, expects to be asked again"},
		{"request timeout", 408, OutcomeIndeterminate, false, false, "may have arrived"},
		{"server error", 500, OutcomeIndeterminate, false, false, "may have acted before failing"},
		{"bad gateway", 502, OutcomeIndeterminate, false, false, "may have acted"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
			}))
			defer srv.Close()
			signedAddon(t, srv.URL, []byte("k"))
			withBreaker(t, 1000, time.Minute) // not the subject here

			resp := Call(context.Background(), passwordSet(nil))
			if resp.Outcome != tc.want {
				t.Fatalf("%d -> %s, want %s (%s)", tc.status, resp.Outcome, tc.want, tc.description)
			}
			if resp.Outcome.Terminal() != tc.terminal {
				t.Fatalf("%s terminal = %t, want %t", resp.Outcome, resp.Outcome.Terminal(), tc.terminal)
			}
			if resp.Outcome.Retryable() != tc.retryable {
				t.Fatalf("%s retryable = %t, want %t", resp.Outcome, resp.Outcome.Retryable(), tc.retryable)
			}
			if tc.want != OutcomeSucceeded && resp.Err == nil {
				t.Fatal("a non-success outcome carried no error to explain it")
			}
		})
	}
}

// 2.6 — unreachable means nothing happened. That is the only outcome safe to
// retry, and getting it wrong in either direction is a real failure: too
// generous duplicates a mutation, too strict abandons a queued row.
func TestUnreachableAddonIsUnreachedAndRetryable(t *testing.T) {
	// A port nothing is listening on: closed immediately at dial.
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := srv.URL
	srv.Close()

	signedAddon(t, url, []byte("k"))
	resp := Call(context.Background(), passwordSet(nil))
	if resp.Outcome != OutcomeUnreached {
		t.Fatalf("outcome = %s (err %v), want unreached", resp.Outcome, resp.Err)
	}
	if !resp.Outcome.Retryable() {
		t.Fatal("a call that never reached the add-on must be safe to retry")
	}
}

// 2.6 — a timeout is NOT a failure. The add-on may have applied the change and
// lost the answer, so reporting this as failed would invite a retry that
// duplicates a mutation, and reporting it as succeeded would be a lie.
func TestTimeoutIsIndeterminateNotFailed(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-release
		w.WriteHeader(http.StatusOK)
	}))
	defer func() { close(release); srv.Close() }()

	signedAddon(t, srv.URL, []byte("k"))
	withCallTimeout(t, 50*time.Millisecond)

	resp := Call(context.Background(), passwordSet(nil))
	if resp.Outcome != OutcomeIndeterminate {
		t.Fatalf("outcome = %s, want indeterminate", resp.Outcome)
	}
	if resp.Outcome.Terminal() {
		t.Fatal("indeterminate must not settle the call")
	}
	if resp.Outcome.Retryable() {
		t.Fatal("a call that may have been applied must never be auto-retried")
	}
}

// 2.5 — the breaker stops the backend spending a timeout per queued row against
// a target that is down, and an open circuit dispatches nothing at all.
func TestCircuitOpensAndRefusesWithoutDispatching(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	signedAddon(t, srv.URL, []byte("k"))
	withBreaker(t, 3, 30*time.Second)
	clock := withTestClock(t, time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC))

	for i := 0; i < 3; i++ {
		if resp := Call(context.Background(), passwordSet(nil)); resp.Outcome != OutcomeIndeterminate {
			t.Fatalf("call %d outcome = %s, want indeterminate", i, resp.Outcome)
		}
	}
	if hits.Load() != 3 {
		t.Fatalf("dispatched %d times before the threshold, want 3", hits.Load())
	}

	resp := Call(context.Background(), passwordSet(nil))
	if resp.Outcome != OutcomeUnreached {
		t.Fatalf("open circuit outcome = %s, want unreached", resp.Outcome)
	}
	if !errors.Is(resp.Err, ErrCircuitOpen) {
		t.Fatalf("err = %v, want ErrCircuitOpen", resp.Err)
	}
	if hits.Load() != 3 {
		t.Fatalf("the open circuit still dispatched (%d hits)", hits.Load())
	}

	a, _ := Get("truenas")
	if !a.CircuitOpen() {
		t.Fatal("CircuitOpen must report what the dispatch path just enforced")
	}

	clock.advance(31 * time.Second)
	if a.CircuitOpen() {
		t.Fatal("the circuit must reopen for a probe once the cooldown elapses")
	}
	if resp := Call(context.Background(), passwordSet(nil)); resp.Outcome != OutcomeIndeterminate {
		t.Fatalf("probe outcome = %s, want indeterminate", resp.Outcome)
	}
	if hits.Load() != 4 {
		t.Fatalf("the cooldown did not let exactly one probe through: %d hits", hits.Load())
	}
	if !a.CircuitOpen() {
		t.Fatal("a failed probe must re-open the circuit immediately, not restart the count")
	}
}

// 2.5 — a rejection is a HEALTHY add-on saying no. If 4xx tripped the breaker,
// one operator submitting one malformed request repeatedly would take the whole
// target offline for everybody.
func TestRejectionsDoNotOpenTheCircuit(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	signedAddon(t, srv.URL, []byte("k"))
	withBreaker(t, 3, 30*time.Second)

	for i := 0; i < 10; i++ {
		if resp := Call(context.Background(), passwordSet(nil)); resp.Outcome != OutcomeRejected {
			t.Fatalf("call %d outcome = %s, want rejected", i, resp.Outcome)
		}
	}
	if hits.Load() != 10 {
		t.Fatalf("dispatched %d of 10 — a rejection opened the circuit", hits.Load())
	}
	a, _ := Get("truenas")
	if a.CircuitOpen() {
		t.Fatal("a target answering 400 is up, and the circuit must say so")
	}
}

// 2.5 — a success clears the count, so a target that flaps and recovers does
// not accumulate its way to an outage over a day.
func TestSuccessClosesTheCircuit(t *testing.T) {
	var fail atomic.Bool
	fail.Store(true)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if fail.Load() {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	signedAddon(t, srv.URL, []byte("k"))
	withBreaker(t, 3, 30*time.Second)
	clock := withTestClock(t, time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC))

	for i := 0; i < 3; i++ {
		Call(context.Background(), passwordSet(nil))
	}
	a, _ := Get("truenas")
	if !a.CircuitOpen() {
		t.Fatal("three failures should have opened the circuit")
	}

	fail.Store(false)
	clock.advance(31 * time.Second)
	if resp := Call(context.Background(), passwordSet(nil)); resp.Outcome != OutcomeSucceeded {
		t.Fatalf("recovery probe outcome = %s", resp.Outcome)
	}
	if a.CircuitOpen() {
		t.Fatal("a successful probe must close the circuit outright")
	}

	// And the count is genuinely zero, not merely below the threshold: two more
	// failures must not be enough to re-open it.
	fail.Store(true)
	for i := 0; i < 2; i++ {
		Call(context.Background(), passwordSet(nil))
	}
	if a.CircuitOpen() {
		t.Fatal("the failure count survived a success")
	}
}

// 2.13 — a dispatch without an operation record is a mutation nothing in the
// database knows was attempted. The transport refuses it structurally rather
// than trusting every future caller to remember.
func TestDispatchWithoutAnOperationRecordIsRefused(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	signedAddon(t, srv.URL, []byte("k"))
	req := passwordSet(nil)
	req.Record = DispatchRecord{}

	resp := Call(context.Background(), req)
	if !errors.Is(resp.Err, ErrNoCallRecord) {
		t.Fatalf("err = %v, want ErrNoCallRecord", resp.Err)
	}
	if resp.Outcome != OutcomeUnreached {
		t.Fatalf("outcome = %s, want unreached — nothing was dispatched", resp.Outcome)
	}
	if hits.Load() != 0 {
		t.Fatal("a call with no operation record reached the add-on")
	}
}

// 2.4, 2.5 — the callability gate runs before the network, so an operation the
// effective set does not offer costs nothing and tells the add-on nothing.
func TestUncallableOperationNeverReachesTheNetwork(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	signedAddon(t, srv.URL, []byte("k"))

	req := passwordSet(nil)
	req.Operation = "account.purge" // in policy, absent from this manifest
	resp := Call(context.Background(), req)
	if !errors.Is(resp.Err, ErrUnknownOperation) {
		t.Fatalf("err = %v, want ErrUnknownOperation", resp.Err)
	}
	if hits.Load() != 0 {
		t.Fatal("an uncallable operation was dispatched anyway")
	}

	req.Target = "nosuchtarget"
	if resp := Call(context.Background(), req); !errors.Is(resp.Err, ErrNotRegistered) {
		t.Fatalf("err = %v, want ErrNotRegistered", resp.Err)
	}
	if hits.Load() != 0 {
		t.Fatal("an unregistered target was dispatched to")
	}
}

// 2.6 — the sweep the whole outcome type exists for: across every failure mode
// there is exactly one path to "succeeded", and it requires a 2xx the backend
// actually read.
func TestNoFailureModeIsSilentSuccess(t *testing.T) {
	withBreaker(t, 1000, time.Minute)

	t.Run("a truncated 2xx body is not a success", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Length", "128")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"partial":`))
			// Hijack and close mid-body so the read fails after the status line.
			if hj, ok := w.(http.Hijacker); ok {
				conn, _, err := hj.Hijack()
				if err == nil {
					_ = conn.Close()
				}
			}
		}))
		defer srv.Close()
		signedAddon(t, srv.URL, []byte("k"))
		resp := Call(context.Background(), passwordSet(nil))
		if resp.Outcome == OutcomeSucceeded {
			t.Fatal("a 2xx whose body was cut off was reported as success")
		}
	})

	t.Run("an already-cancelled context dispatches nothing", func(t *testing.T) {
		var hits atomic.Int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hits.Add(1)
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()
		signedAddon(t, srv.URL, []byte("k"))

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		resp := Call(ctx, passwordSet(nil))
		if resp.Outcome != OutcomeUnreached {
			t.Fatalf("outcome = %s, want unreached", resp.Outcome)
		}
		if hits.Load() != 0 {
			t.Fatal("a cancelled context still dispatched")
		}
	})
}

// 2.5, 2.35 — the manifest read is authenticated too. A capability set read
// over an unauthenticated channel is one an on-path attacker can edit, and
// capability is what the backend then decides against.
func TestManifestReadIsAuthenticated(t *testing.T) {
	key := []byte("manifest-signing-key")
	var (
		mu     sync.Mutex
		sigErr error
		saw    bool
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		saw = true
		sigErr = verifySignature(r.Header.Get(SignatureHeader), body, key, time.Minute, time.Now())
		mu.Unlock()
		_ = json.NewEncoder(w).Encode(goodManifest())
	}))
	defer srv.Close()

	path := filepath.Join(t.TempDir(), "sign.key")
	writeFile(t, path, key)
	resetRegistry(t)
	registryMu.Lock()
	registry = map[string]*Addon{"truenas": {Registration: Registration{
		Target: "truenas", BaseURL: srv.URL, SigningKeyPath: path,
	}}}
	registryMu.Unlock()

	if err := Refresh(context.Background(), "truenas"); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if !saw {
		t.Fatal("the manifest was never requested")
	}
	if sigErr != nil {
		t.Fatalf("the manifest read carried no valid signature: %v", sigErr)
	}
}

// 2.35 — an http:// base URL means no handshake, which means the client
// certificate is never presented and the private CA is never consulted. The
// registration would report auth=mtls with nothing authenticating the
// connection: the same wrong mental model an incomplete certificate triple
// creates, reached through a URL scheme instead.
func TestNonHTTPSBaseURLDoesNotRegister(t *testing.T) {
	cases := []struct {
		name, url string
	}{
		{"plain http", "http://addon-truenas:8090"},
		{"no scheme at all", "addon-truenas:8090"},
		{"scheme with no host", "https://"},
		{"not a URL", "://nonsense"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resetRegistry(t)
			withEnv(t, map[string]string{
				"ADDON_TARGETS":             "truenas",
				"ADDON_TRUENAS_BASE_URL":    tc.url,
				"ADDON_TRUENAS_CLIENT_CERT": "/run/secrets/c.crt",
				"ADDON_TRUENAS_CLIENT_KEY":  "/run/secrets/c.key",
				"ADDON_TRUENAS_CA_CERT":     "/run/secrets/ca.crt",
			})
			withTargetRegistry(t, &fakeTargetRegistry{})
			if err := Init(context.Background()); err != nil {
				t.Fatalf("Init: %v", err)
			}
			if got := Registered(); len(got) != 0 {
				t.Fatalf("%q registered despite not being https: %+v", tc.url, got)
			}
		})
	}

	t.Run("https registers", func(t *testing.T) {
		registerTrueNAS(t)
		if len(Registered()) != 1 {
			t.Fatal("the https case must still register, or this guard rejects everything")
		}
	})
}

// 2.6 — io.LimitReader signals its limit with EOF, not an error, so a body
// larger than the bound reads back as a clean short body. Without an explicit
// overflow check an oversized 2xx is reported as success, which is exactly the
// silent-success this outcome type exists to prevent.
func TestOversizedResponseIsNotASuccess(t *testing.T) {
	oversized := bytes.Repeat([]byte("a"), maxResponseBytes+64)

	t.Run("an oversized 2xx is indeterminate", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(oversized)
		}))
		defer srv.Close()
		signedAddon(t, srv.URL, []byte("k"))
		withBreaker(t, 1000, time.Minute)

		resp := Call(context.Background(), passwordSet(nil))
		if resp.Outcome != OutcomeIndeterminate {
			t.Fatalf("outcome = %s, want indeterminate — the body was not read whole", resp.Outcome)
		}
		if len(resp.Body) > maxResponseBytes {
			t.Fatalf("body of %d bytes exceeds the bound", len(resp.Body))
		}
	})

	t.Run("an oversized refusal stays a refusal", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write(oversized)
		}))
		defer srv.Close()
		signedAddon(t, srv.URL, []byte("k"))
		withBreaker(t, 1000, time.Minute)

		// A 4xx is decided by its status. Reclassifying it because the
		// diagnostic body was long would turn a deterministic refusal into a
		// row that never settles.
		resp := Call(context.Background(), passwordSet(nil))
		if resp.Outcome != OutcomeRejected {
			t.Fatalf("outcome = %s, want rejected", resp.Outcome)
		}
		if resp.Err == nil || !strings.Contains(resp.Err.Error(), "exceeding") {
			t.Fatalf("the truncation went unrecorded: %v", resp.Err)
		}
	})

	t.Run("an oversized manifest is refused rather than parsed", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write(oversized)
		}))
		defer srv.Close()
		path := filepath.Join(t.TempDir(), "sign.key")
		writeFile(t, path, []byte("k"))
		resetRegistry(t)
		registryMu.Lock()
		registry = map[string]*Addon{"truenas": {Registration: Registration{
			Target: "truenas", BaseURL: srv.URL, SigningKeyPath: path,
		}}}
		registryMu.Unlock()

		if err := Refresh(context.Background(), "truenas"); err == nil {
			t.Fatal("an oversized capability response was accepted")
		}
	})
}

// 2.35 — signing a request says nothing about the response. Signed mode still
// verifies the add-on's server certificate, against the private CA when one is
// configured, or a forged 2xx is recorded as a completed mutation.
func TestSignedModeVerifiesTheAddonsServerCertificate(t *testing.T) {
	pki := newTestPKI(t, time.Now().Add(365*24*time.Hour))
	keyPath := filepath.Join(t.TempDir(), "sign.key")
	writeFile(t, keyPath, []byte("a-real-signing-key"))

	serve := func(cert tls.Certificate) *httptest.Server {
		s := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		s.TLS = &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS13}
		s.StartTLS()
		return s
	}

	t.Run("a server the private CA issued is accepted", func(t *testing.T) {
		srv := serve(pki.serverCert)
		defer srv.Close()
		installAddon(t, Registration{
			Target: "truenas", BaseURL: srv.URL, CAPath: pki.caPath, SigningKeyPath: keyPath,
		}, goodManifest())

		if resp := Call(context.Background(), passwordSet(nil)); resp.Outcome != OutcomeSucceeded {
			t.Fatalf("outcome = %s (err %v), want succeeded", resp.Outcome, resp.Err)
		}
	})

	t.Run("a server from another CA is refused", func(t *testing.T) {
		impostor := newTestPKI(t, time.Now().Add(365*24*time.Hour))
		srv := serve(impostor.serverCert)
		defer srv.Close()
		installAddon(t, Registration{
			Target: "truenas", BaseURL: srv.URL, CAPath: pki.caPath, SigningKeyPath: keyPath,
		}, goodManifest())

		resp := Call(context.Background(), passwordSet(nil))
		if resp.Outcome == OutcomeSucceeded {
			t.Fatal("a signed-mode call trusted a server the configured CA never issued")
		}
	})
}

// 2.2 — a private CA alone is a deliberate signed-mode anchor, not half-built
// mutual TLS. Warning about it would train an operator to ignore the warning
// that does matter.
func TestPrivateCAAloneIsASignedModeAnchorNotAWarning(t *testing.T) {
	r := Registration{
		Target: "truenas", BaseURL: "https://addon:8090",
		CAPath: "/run/secrets/ca.crt", SigningKeyPath: "/run/secrets/s.key",
	}
	if r.AuthMode() != "signed" {
		t.Fatalf("auth mode = %q, want signed", r.AuthMode())
	}
	if r.partialMTLS() {
		t.Fatal("a CA with no client certificate was reported as incomplete mTLS")
	}

	// A certificate without its key still is.
	half := Registration{Target: "truenas", ClientCertPath: "/c.crt", SigningKeyPath: "/s.key"}
	if !half.partialMTLS() {
		t.Fatal("a client certificate with no key must still be flagged")
	}
}

// 2.35 — an add-on's response never redirects the backend. Go follows redirects
// by default and re-sends the body on 307/308, and it strips Authorization and
// Cookie across hosts but not a custom header — so a compromised add-on could
// have the whole secret-bearing POST replayed to a host of its choosing, signed
// and therefore authenticated to that host. The final 2xx would then classify as
// success while the registered target never acted.
func TestARedirectIsRefusedAndNeverReplaysTheBody(t *testing.T) {
	var (
		secondHits atomic.Int32
		secondBody atomic.Value
	)
	second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		secondHits.Add(1)
		b, _ := io.ReadAll(r.Body)
		secondBody.Store(string(b))
		w.WriteHeader(http.StatusOK)
	}))
	defer second.Close()

	for _, code := range []int{http.StatusTemporaryRedirect, http.StatusPermanentRedirect, http.StatusFound} {
		t.Run(http.StatusText(code), func(t *testing.T) {
			secondHits.Store(0)
			first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, second.URL+r.URL.Path, code)
			}))
			defer first.Close()

			signedAddon(t, first.URL, []byte("k"))
			withBreaker(t, 1000, time.Minute)

			resp := Call(context.Background(), passwordSet(map[string]any{"password": theSecret}))
			if resp.Outcome == OutcomeSucceeded {
				t.Fatal("a redirect was followed and its 2xx reported as a completed mutation")
			}
			if resp.Outcome != OutcomeRejected {
				t.Fatalf("outcome = %s, want rejected — the registered target did not act", resp.Outcome)
			}
			if resp.Status != code {
				t.Fatalf("status = %d, want the redirect %d surfaced rather than swallowed", resp.Status, code)
			}
			if secondHits.Load() != 0 {
				t.Fatalf("the redirect target received %d request(s)", secondHits.Load())
			}
			if body, _ := secondBody.Load().(string); strings.Contains(body, theSecret) {
				t.Fatal("the secret-bearing body was replayed to a second host")
			}
			if resp.Err == nil || !strings.Contains(resp.Err.Error(), "redirect") {
				t.Fatalf("the refusal did not name its cause: %v", resp.Err)
			}
		})
	}

	t.Run("the capability read does not follow one either", func(t *testing.T) {
		secondHits.Store(0)
		first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, second.URL+r.URL.Path, http.StatusTemporaryRedirect)
		}))
		defer first.Close()

		path := filepath.Join(t.TempDir(), "sign.key")
		writeFile(t, path, []byte("k"))
		resetRegistry(t)
		registryMu.Lock()
		registry = map[string]*Addon{"truenas": {Registration: Registration{
			Target: "truenas", BaseURL: first.URL, SigningKeyPath: path,
		}}}
		registryMu.Unlock()

		if err := Refresh(context.Background(), "truenas"); err == nil {
			t.Fatal("a redirected capability read was accepted")
		}
		if secondHits.Load() != 0 {
			t.Fatal("the capability read followed a redirect to another host")
		}
	})
}
