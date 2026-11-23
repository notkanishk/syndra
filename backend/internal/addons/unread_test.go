package addons

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// Design §13.1's table, one row per cause: Unread is the one mapping.
func TestUnreadNamesWhoIsDownAndWhy(t *testing.T) {
	problem := func(code string) []byte {
		return fmt.Appendf(nil, `{"code":%q,"detail":"TrueNAS is switched off. Turn it on."}`, code)
	}
	dial := &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("connection refused")}
	for _, tc := range []struct {
		name         string
		resp         CallResponse
		reason, why  string
		detailWanted bool
	}{
		{"read", CallResponse{Outcome: OutcomeSucceeded, Status: 200}, "", "", false},
		{"dial refused", CallResponse{Outcome: OutcomeUnreached, Err: dial}, ReasonAddonUnreachable, CauseNoAnswer, false},
		{"name not found", CallResponse{Outcome: OutcomeUnreached, Err: &net.OpError{Op: "dial", Err: &net.DNSError{Err: "no such host", IsNotFound: true}}}, ReasonAddonUnreachable, CauseNoAnswer, false},
		{"handshake timeout", CallResponse{Outcome: OutcomeUnreached, Err: errors.New("net/http: TLS handshake timeout")}, ReasonAddonUnreachable, CauseNoAnswer, false},
		{"pinned key mismatch", CallResponse{Outcome: OutcomeUnreached, Err: fmt.Errorf("wrap: %w", pinMismatchError{target: "truenas"})}, ReasonAddonUnreachable, CauseWrongHost, false},
		{"certificate error", CallResponse{Outcome: OutcomeUnreached, Err: &tls.CertificateVerificationError{Err: errors.New("x")}}, ReasonAddonUnreachable, CauseWrongHost, false},
		{"no key loaded", CallResponse{Outcome: OutcomeUnreached, Err: fmt.Errorf("addon truenas: %w: %w", errNoTransport, errors.New("secret file is empty"))}, ReasonAddonUnreachable, CauseNoKey, false},
		{"timeout after sending", CallResponse{Outcome: OutcomeIndeterminate, Err: context.DeadlineExceeded}, ReasonAddonUnreachable, CauseTimeout, false},
		{"reply cut off", CallResponse{Outcome: OutcomeIndeterminate, Status: 200, Err: io.ErrUnexpectedEOF}, ReasonAddonUnreachable, CauseTimeout, false},
		{"uncoded 500", CallResponse{Outcome: OutcomeIndeterminate, Status: 500}, ReasonAddonUnreachable, CauseFault, false},
		{"undecodable 2xx", CallResponse{Outcome: OutcomeIndeterminate, Status: 200, Err: fmt.Errorf("%w: x", ErrUnreadableReply)}, ReasonAddonUnreachable, CauseFault, false},
		{"redirect", CallResponse{Outcome: OutcomeRejected, Status: 302}, ReasonAddonUnreachable, CauseFault, false},
		{"breaker open", CallResponse{Outcome: OutcomeUnreached, Err: &CircuitOpenError{Opener: Why{Reason: ReasonAddonUnreachable, Cause: CauseTimeout}, RetryAfter: 12 * time.Second}}, ReasonAddonUnreachable, CauseTimeout, false},
		{"TARGET_UNREACHABLE", CallResponse{Outcome: OutcomeUnreached, Status: 503, Code: codeTargetUnreachable, Body: problem(codeTargetUnreachable)}, ReasonAppUnreachable, "", true},
		{"NO_CURRENT_READ", CallResponse{Outcome: OutcomeIndeterminate, Status: 503, Code: codeNoCurrentRead, Body: problem(codeNoCurrentRead)}, ReasonAppUnreachable, "", true},
		{"TARGET_UNREADABLE", CallResponse{Outcome: OutcomeIndeterminate, Status: 502, Code: codeTargetUnreadable, Body: problem(codeTargetUnreadable)}, ReasonAppUnreachable, "", true},
		{"STORE_UNREADABLE", CallResponse{Outcome: OutcomeIndeterminate, Status: 500, Code: CodeStoreUnreadable, Body: problem(CodeStoreUnreadable)}, ReasonReadRefused, "", true},
		{"cooldown", CallResponse{Outcome: OutcomeUnreached, Status: 429, Code: codeFreshReadCooldown, RetryAfter: 9}, ReasonCooldown, "", false},
		{"TARGET_BUSY", CallResponse{Outcome: OutcomeUnreached, Status: 503, Code: codeTargetBusy, Body: problem(codeTargetBusy), RetryAfter: 5}, ReasonBusy, "", true},
		{"shape refusal", CallResponse{Outcome: OutcomeUnreached, Status: 400, ShapeRefusal: true}, ReasonAddonOutdated, "", false},
		{"feature not declared", CallResponse{Outcome: OutcomeUnreached, Err: ErrFeatureNotDeclared}, ReasonAddonOutdated, "", false},
		{"Syndra's own refusal", CallResponse{Outcome: OutcomeUnreached, Err: ErrNoCallRecord}, ReasonReadRefused, "", false},
		{"TARGET_CHANGED", CallResponse{Outcome: OutcomeUnreached, Status: 503, Code: CodeTargetChanged, Body: problem(CodeTargetChanged), RetryAfter: 5}, ReasonReadRefused, "", true},
		{"TARGET_UNREADABLE as 429", CallResponse{Outcome: OutcomeUnreached, Status: 429, Code: codeTargetUnreadable, Body: problem(codeTargetUnreadable)}, ReasonAppUnreachable, "", true},
		{"coded 503", CallResponse{Outcome: OutcomeIndeterminate, Status: 503, Code: "APPLY_FAILED", Body: problem("APPLY_FAILED")}, ReasonReadRefused, "", true},
		{"an operation the add-on withdrew", CallResponse{Outcome: OutcomeUnreached, Err: &ErrOperationUnavailable{Target: "truenas", ID: "health.get", Reason: "the target version has not been read yet"}}, ReasonReadRefused, "", false},
		{"an operation the manifest does not declare", CallResponse{Outcome: OutcomeUnreached, Err: fmt.Errorf("%w: truenas/system.get", ErrUnknownOperation)}, ReasonAddonOutdated, "", false},
		{"a target no configuration names", CallResponse{Outcome: OutcomeUnreached, Err: fmt.Errorf("%w: truenas", ErrNotRegistered)}, ReasonReadRefused, "", true},
		{"call record not written", CallResponse{Outcome: OutcomeUnreached, Err: fmt.Errorf("%w: %w", errNotPrepared, errors.New("db down"))}, ReasonReadRefused, "", false},
		{"a 4xx about the request", CallResponse{Outcome: OutcomeRejected, Status: 404, Code: "NO_SUCH_SETTING"}, ReasonReadRefused, "", false},
	} {
		got := Unread(tc.resp)
		if got.Reason != tc.reason || got.Cause != tc.why {
			t.Errorf("%s: got %s/%s, want %s/%s", tc.name, got.Reason, got.Cause, tc.reason, tc.why)
		}
		if tc.detailWanted != (got.Detail != "") {
			t.Errorf("%s: the add-on's sentence carried = %t, want %t", tc.name, got.Detail != "", tc.detailWanted)
		}
		if got.Unreachable() != (tc.reason == ReasonAddonUnreachable || tc.reason == ReasonAppUnreachable) {
			t.Errorf("%s: Unreachable() = %t", tc.name, got.Unreachable())
		}
	}
	// No add-on to read the log of: the next step is Syndra's configuration.
	if w := Unread(CallResponse{Outcome: OutcomeUnreached, Err: fmt.Errorf("%w: truenas", ErrNotRegistered)}); !strings.Contains(w.Detail, "Syndra's add-on configuration") || strings.Contains(w.Detail, "log") {
		t.Errorf("a target no configuration names says %q, want Syndra's configuration as the next step", w.Detail)
	}
	if w := Unread(CallResponse{Outcome: OutcomeUnreached, Err: &CircuitOpenError{Opener: Why{Reason: ReasonAddonUnreachable, Cause: CauseTimeout}, RetryAfter: 11500 * time.Millisecond}}); w.RetryAfter != 12 {
		t.Errorf("an open breaker's time left, rounded up: %d", w.RetryAfter)
	}
}

// The breaker remembers what opened it, and a refused call carries it with
// the time left (design §13.2: an open breaker is a cause, not a second word).
func TestTheBreakerRemembersTheCauseThatOpenedIt(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	srv.Close() // a closed port: every dial is refused
	signedAddon(t, url, "a-breaker-secret")
	for range breakerThreshold {
		Health(context.Background(), "truenas")
	}
	h := Health(context.Background(), "truenas")
	if !errors.Is(h.Err, ErrCircuitOpen) {
		t.Fatalf("want the breaker open, got %v", h.Err)
	}
	if h.Why.Reason != ReasonAddonUnreachable || h.Why.Cause != CauseNoAnswer || h.Why.RetryAfter < 1 || h.Why.RetryAfter > int(breakerCooldown/time.Second) {
		t.Fatalf("an open breaker says the cause that opened it and its time left, got %+v", h.Why)
	}
}

// A dead address costs dialTimeout, and a listener that accepts and never
// speaks TLS costs tlsHandshakeTimeout: both surely unreached, long before the
// call's own 30 s (design §13.5).
func TestADeadAddressIsUnreachedInsideTheDialAndHandshakeTimeouts(t *testing.T) {
	if dialTimeout != 3*time.Second || tlsHandshakeTimeout != 5*time.Second {
		t.Fatalf("the decided timeouts are 3 s and 5 s, got %v and %v", dialTimeout, tlsHandshakeTimeout)
	}
	tr := newAddonClient(&tls.Config{}).Transport.(*http.Transport)
	if tr.TLSHandshakeTimeout != tlsHandshakeTimeout || tr.DialContext == nil {
		t.Fatalf("the client must carry the handshake timeout and the absolute dial: %+v", tr)
	}

	// A blackholed listener: accepts the connection and never answers.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var held []net.Conn
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			mu.Lock()
			held = append(held, c)
			mu.Unlock()
		}
	}()
	t.Cleanup(func() {
		ln.Close()
		mu.Lock()
		for _, c := range held {
			c.Close()
		}
		mu.Unlock()
	})
	hs := tlsHandshakeTimeout
	tlsHandshakeTimeout = 300 * time.Millisecond
	t.Cleanup(func() { tlsHandshakeTimeout = hs })
	cred := credFor(t, "https://"+ln.Addr().String())

	start := time.Now()
	resp := doAuthenticated(context.Background(), cred, http.MethodGet, "https://"+ln.Addr().String()+"/health", nil, 30*time.Second)
	if took := time.Since(start); took > 4*time.Second {
		t.Fatalf("a listener that never answers took %v; the handshake timeout bounds it", took)
	}
	if resp.Outcome != OutcomeUnreached || Unread(resp).Cause != CauseNoAnswer {
		t.Fatalf("a handshake that timed out sent nothing: want unreached no_answer, got %s %+v (%v)", resp.Outcome, Unread(resp), resp.Err)
	}

	// A blackholed address (TEST-NET-1, RFC 5737): the dial times out.
	dt := dialTimeout
	dialTimeout = 300 * time.Millisecond
	t.Cleanup(func() { dialTimeout = dt })
	start = time.Now()
	resp = doAuthenticated(context.Background(), cred, http.MethodGet, "https://192.0.2.1:8443/health", nil, 30*time.Second)
	if took := time.Since(start); took > 4*time.Second {
		t.Fatalf("a blackholed address took %v; the dial timeout bounds it", took)
	}
	if resp.Outcome != OutcomeUnreached || Unread(resp).Cause != CauseNoAnswer {
		t.Fatalf("want unreached no_answer, got %s %+v (%v)", resp.Outcome, Unread(resp), resp.Err)
	}
}

// The registered host is resolved as an absolute name (RFC 1034 §3.1), so the
// search list is never tried; an IP literal is dialled as is.
func TestTheHostIsResolvedAsAnAbsoluteName(t *testing.T) {
	for in, want := range map[string]string{
		"truenas-addon:8443":  "truenas-addon.:8443",
		"addon.example.:8443": "addon.example.:8443",
		"127.0.0.1:8443":      "127.0.0.1:8443",
		"[::1]:8443":          "[::1]:8443",
		"[fe80::1%lo0]:8443":  "[fe80::1%lo0]:8443",
		"addon.internal:8443": "addon.internal.:8443",
	} {
		if got := absoluteAddr(in); got != want {
			t.Errorf("absoluteAddr(%q) = %q, want %q", in, got, want)
		}
	}

	// The dial is asked for "name.", and the request on the wire is the one
	// signed: Host without the dot, the signature verifying.
	type seenReq struct{ host, sig, uri string }
	got := make(chan seenReq, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got <- seenReq{r.Host, r.Header.Get(SignatureHeader), r.RequestURI}
	}))
	t.Cleanup(srv.Close)
	_, port, _ := net.SplitHostPort(strings.TrimPrefix(srv.URL, "http://"))
	var asked []string
	orig := addonDial
	addonDial = func(ctx context.Context, network, addr string) (net.Conn, error) {
		asked = append(asked, addr)
		return orig(ctx, network, net.JoinHostPort("127.0.0.1", port))
	}
	t.Cleanup(func() { addonDial = orig })
	cred := credFor(t, srv.URL)
	url := "http://truenas-addon:" + port + "/health?x=1"
	if resp := doAuthenticated(context.Background(), cred, http.MethodGet, url, nil, 5*time.Second); resp.Status != 200 {
		t.Fatalf("want 200, got %d %v", resp.Status, resp.Err)
	}
	if len(asked) != 1 || asked[0] != "truenas-addon.:"+port {
		t.Fatalf("the resolver must be asked for the absolute name, got %v", asked)
	}
	r := <-got
	if r.host != "truenas-addon:"+port || r.uri != "/health?x=1" {
		t.Fatalf("Host and target must be unchanged, got %+v", r)
	}
	var ts int64
	if _, err := fmt.Sscanf(r.sig, "t=%d,", &ts); err != nil {
		t.Fatalf("signature %q: %v", r.sig, err)
	}
	if want := ComputeSignature(time.Unix(ts, 0), http.MethodGet, "/health?x=1", nil, cred.signingKey); r.sig != want {
		t.Fatalf("the signed request changed: got %q want %q", r.sig, want)
	}
}

// Probes run in parallel, each bounded by probeTimeout: two add-ons that hang
// cost one probe timeout, not two.
func TestProbeAllProbesInParallelEachBounded(t *testing.T) {
	m := monitor
	monitor = newHealthMonitor()
	t.Cleanup(func() { monitor = m })
	pt := probeTimeout
	probeTimeout = 400 * time.Millisecond
	t.Cleanup(func() { probeTimeout = pt })

	release := make(chan struct{})
	hang := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-release }))
	t.Cleanup(hang.Close)
	t.Cleanup(func() { close(release) }) // runs first: Close waits for the handlers
	signedAddon(t, hang.URL, "a-parallel-secret")
	a, _ := Get("truenas")
	second := a.Registration
	second.Target = "printers"
	registryMu.Lock()
	registry["printers"] = &Addon{Registration: second}
	registryMu.Unlock()
	t.Cleanup(func() { registryMu.Lock(); delete(registry, "printers"); registryMu.Unlock() })

	start := time.Now()
	_ = ProbeAll(context.Background())
	if took := time.Since(start); took > 700*time.Millisecond {
		t.Fatalf("two hanging add-ons took %v; in parallel they cost one probe timeout", took)
	}
	for _, target := range []string{"truenas", "printers"} {
		if state, cause := missOf(LastHealth(target)); state != StateAddonUnreachable || cause != CauseTimeout {
			t.Errorf("%s: a hung probe is a miss as addon_unreachable timeout, got %s %q", target, state, cause)
		}
	}
}

// The drain reads Why off the apply: a closed port is addon_unreachable
// no_answer, the reason it spends no attempt.
func TestAnApplyToAClosedPortSaysNoAnswer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	srv.Close()
	signedAddon(t, url, "an-apply-closed-secret")
	resp := Apply(context.Background(), ApplyRequest{Target: "truenas", CallID: "call-1", Fingerprint: "fp-1", Subject: "sub-1"})
	if resp.Outcome != OutcomeUnreached || !resp.Why.Unreachable() || resp.Why.Cause != CauseNoAnswer {
		t.Fatalf("want unreached addon_unreachable no_answer, got %s %+v (%v)", resp.Outcome, resp.Why, resp.Err)
	}
}

// Syndra failing before it sends (its call record did not write) is its own
// refusal, read_refused: never no_answer, which would spend no attempt and
// count as a monitor miss.
func TestAClaimThatFailsIsNotAnOutage(t *testing.T) {
	recorder(t, http.StatusOK)
	a, _ := Get("truenas")
	resp, _ := a.send(context.Background(), sendSpec{Method: http.MethodPut, Path: "/settings/home_root", Write: true,
		Claim: func(context.Context) error { return errors.New("db down") }, Body: []byte(`{}`)})
	if w := Unread(resp); w.Reason != ReasonReadRefused || w.Unreachable() {
		t.Fatalf("a failed claim is Syndra's own refusal, got %+v (%v)", w, resp.Err)
	}
}

// TARGET_CHANGED is the add-on working and its app moving under it: a busy app
// never opens the breaker and takes the target away from everybody.
func TestTargetChangedNeverOpensTheBreaker(t *testing.T) {
	withBreaker(t, 3, breakerCooldown)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = fmt.Fprintf(w, `{"code":%q,"detail":"TrueNAS changed while it was read."}`, CodeTargetChanged)
	}))
	t.Cleanup(srv.Close)
	signedAddon(t, srv.URL, "a-changed-secret")
	a, _ := Get("truenas")
	for range breakerThreshold + 2 {
		_, _ = a.send(context.Background(), sendSpec{Method: http.MethodGet, Path: "/subjects"})
	}
	if a.CircuitOpen() {
		t.Error("TARGET_CHANGED answers opened the breaker")
	}
}

// TARGET_BUSY is the add-on's queue to its app, or the app, busy: never an
// outage, so it never opens the breaker (8g fix round 2, 22 Nov 2026).
func TestTargetBusyNeverOpensTheBreaker(t *testing.T) {
	withBreaker(t, 3, breakerCooldown)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.Header().Set("Retry-After", "5")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = fmt.Fprintf(w, `{"code":%q,"detail":"The NAS is busy right now."}`, codeTargetBusy)
	}))
	t.Cleanup(srv.Close)
	signedAddon(t, srv.URL, "a-busy-secret")
	a, _ := Get("truenas")
	var last CallResponse
	for range breakerThreshold + 2 {
		last, _ = a.send(context.Background(), sendSpec{Method: http.MethodGet, Path: "/subjects"})
	}
	if a.CircuitOpen() {
		t.Error("TARGET_BUSY answers opened the breaker")
	}
	if w := Unread(last); w.Reason != ReasonBusy || w.Unreachable() || w.RetryAfter != 5 || last.Outcome != OutcomeUnreached {
		t.Errorf("TARGET_BUSY read as %+v, outcome %s; want busy, not an outage, 5 s, unreached", w, last.Outcome)
	}
}

// An add-on answering that its app is down (TARGET_UNREACHABLE,
// NO_CURRENT_READ, TARGET_UNREADABLE: app_unreachable) is the add-on working,
// so it never opens the breaker; the monitor owns app_unreachable. Opened, a
// page read during an app outage named a stale opener instead (health.get,
// addon_unreachable/no_answer, on the 8g staging run, 22 Nov 2026).
func TestAnAppOutageAnswerNeverOpensTheBreaker(t *testing.T) {
	for _, code := range []string{codeTargetUnreachable, codeNoCurrentRead, codeTargetUnreadable} {
		withBreaker(t, 3, breakerCooldown)
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/problem+json")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = fmt.Fprintf(w, `{"code":%q,"detail":"TrueNAS did not answer."}`, code)
		}))
		signedAddon(t, srv.URL, "an-app-down-secret")
		a, _ := Get("truenas")
		var sys SystemReport
		for range breakerThreshold + 2 {
			sys = SystemHealth(context.Background(), "truenas", "operator-1")
		}
		srv.Close()
		if a.CircuitOpen() {
			t.Errorf("%s answers opened the breaker", code)
		}
		if sys.Read.Reason != ReasonAppUnreachable {
			t.Errorf("%s: system health read as %+v, want app_unreachable", code, sys.Read)
		}
	}
}

// Nor does it clear the count, as TARGET_BUSY does not: the code is the
// add-on's to send, so a broken add-on alternating 5xx with it would keep the
// breaker shut for ever. The answer pauses the count; the probe's 2xx clears.
func TestAnAppOutageAnswerDoesNotClearTheBreaker(t *testing.T) {
	b := &breaker{}
	now := time.Unix(1_700_000_000, 0)
	for range breakerThreshold - 1 {
		b.record(now, CallResponse{Outcome: OutcomeIndeterminate, Status: 500})
		b.record(now, CallResponse{Outcome: OutcomeUnreached, Status: 503, Code: codeTargetUnreachable})
	}
	b.record(now, CallResponse{Outcome: OutcomeIndeterminate, Status: 500})
	if b.allow(now) {
		t.Fatal("app-outage answers reset the count a failing add-on is accumulating")
	}
}

// The staging sequence: app-outage answers, then the monitor's one probe
// refused while the add-on restarts. That one refusal alone must not open the
// breaker, so the next read still says app_unreachable, never the refused
// dial's no_answer for the cooldown.
func TestOneRefusedProbeDuringAnAppOutageDoesNotOpenTheBreaker(t *testing.T) {
	withBreaker(t, 3, breakerCooldown)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = fmt.Fprintf(w, `{"code":%q,"detail":"TrueNAS did not answer."}`, codeTargetUnreachable)
	}))
	t.Cleanup(srv.Close)
	signedAddon(t, srv.URL, "a-restart-secret")
	a, _ := Get("truenas")
	for range breakerThreshold - 1 {
		SystemHealth(context.Background(), "truenas", "operator-1")
	}
	a.br.record(timeNow(), CallResponse{Outcome: OutcomeUnreached, Err: errors.New("dial tcp: connection refused")})
	if sys := SystemHealth(context.Background(), "truenas", "operator-1"); sys.Read.Reason != ReasonAppUnreachable {
		t.Errorf("after one refused probe, system health read as %+v, want app_unreachable", sys.Read)
	}
}

// Over TLS the dot is the dial's alone: the server is asked for
// "truenas-addon" (SNI, RFC 6066 §3: no trailing dot) while the resolver was
// asked for "truenas-addon.".
func TestTLSNamesTheHostWithoutTheDot(t *testing.T) {
	sni := make(chan string, 1)
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	srv.TLS = &tls.Config{GetConfigForClient: func(hi *tls.ClientHelloInfo) (*tls.Config, error) {
		sni <- hi.ServerName
		return nil, nil
	}}
	srv.StartTLS()
	t.Cleanup(srv.Close)
	_, port, _ := net.SplitHostPort(strings.TrimPrefix(srv.URL, "https://"))
	var asked string
	orig := addonDial
	addonDial = func(ctx context.Context, network, addr string) (net.Conn, error) {
		asked = addr
		return orig(ctx, network, net.JoinHostPort("127.0.0.1", port))
	}
	t.Cleanup(func() { addonDial = orig })
	client := newAddonClient(&tls.Config{InsecureSkipVerify: true}) //nolint:gosec // the test server's own certificate
	resp, err := client.Get("https://truenas-addon:" + port + "/health")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if got := <-sni; got != "truenas-addon" || asked != "truenas-addon.:"+port {
		t.Fatalf("SNI %q (want truenas-addon), dial %q (want truenas-addon.:%s)", got, asked, port)
	}
}
