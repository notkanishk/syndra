package handlers

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"reflect"
	"strings"
	"sync/atomic"
	"time"

	"syndra/internal/claims"
	"syndra/internal/claimserve"
	"syndra/internal/claimtel"
)

// The synthetic probes (claims-delivery-integrity S6, assessment §4d).
//
// L1, every minute: the canary's preaccesstoken call, signed with the key the
// data plane holds, POSTed to SYNDRA_EXTERNAL_URL: public DNS, Caddy, TLS, the
// LXC, whichever door Caddy routes to, the snapshot and the shaper. It must
// get CanaryExpected exactly. It does not prove anything on Zitadel's side.
//
// L2, every 15 minutes, only when the owner has made the canary in Zitadel
// (SYNDRA_CANARY_*): a real token for a machine user, which must carry the
// claims Syndra's snapshot gives that user, no ":log" claim (a key dropped),
// a call the telemetry saw, and the same on userinfo. When L2 fails and L1
// passes, the fault is on Zitadel's side.

const (
	probeL1Every   = time.Minute
	probeL2Every   = 15 * time.Minute
	certWarnWithin = 14 * 24 * time.Hour
	// probeStaleAfter: three missed runs and the probe itself has stopped.
	probeStaleAfter = 3 * probeL1Every
	logClaimL2      = "urn:zitadel:iam:action:function/preaccesstoken:log"

	canaryKeyPathEnv   = "SYNDRA_CANARY_KEY_PATH"
	canaryProjectIDEnv = "SYNDRA_CANARY_PROJECT_ID"
	canaryClientIDEnv  = "SYNDRA_CANARY_CLIENT_ID"
)

// The classes a probe fails with: its finding's subject and its reason.
var (
	l1Classes = []string{"dns", "tls", "timeout", "unreachable", "signature", "status", "claims"}
	l2Classes = []string{"token", "no_facts", "claims", "log_claim", "not_called", "userinfo"}
)

// probeResult is one probe run, as /healthz/claims and the operator see it.
// Detail never carries a key, a token or a URL.
type probeResult struct {
	At           time.Time  `json:"at"`
	Pass         bool       `json:"pass"`
	Class        string     `json:"class,omitempty"`
	Detail       string     `json:"detail,omitempty"`
	TookMS       float64    `json:"took_ms"`
	CertNotAfter *time.Time `json:"cert_not_after,omitempty"`
}

var lastProbeL1, lastProbeL2 atomic.Pointer[probeResult]

// ProbeL1Enabled is whether there is a public URL to probe.
func ProbeL1Enabled() bool { return os.Getenv("SYNDRA_EXTERNAL_URL") != "" }

// ProbeL2Enabled is whether the owner has set up the canary in Zitadel.
func ProbeL2Enabled() bool {
	return os.Getenv(canaryKeyPathEnv) != "" && os.Getenv(canaryProjectIDEnv) != "" && os.Getenv(canaryClientIDEnv) != ""
}

// RunProbeL1 runs L1 once, records it, and raises or settles its findings.
func RunProbeL1(ctx context.Context) error {
	if !ProbeL1Enabled() {
		return nil
	}
	res := probeL1Once(ctx, os.Getenv("SYNDRA_EXTERNAL_URL"))
	lastProbeL1.Store(&res)
	obs := []claimtel.Observation{}
	if res.CertNotAfter != nil {
		obs = append(obs, claimtel.Observation{Kind: claimtel.KindCertExpiring, Subject: "L1", At: res.At,
			Fail:   res.CertNotAfter.Sub(res.At) < certWarnWithin,
			Sample: map[string]any{"not_after": res.CertNotAfter.UTC(), "days_left": daysLeft(res.At, *res.CertNotAfter)}})
	}
	return recordProbe(ctx, claimtel.KindProbeL1, l1Classes, res, obs)
}

// RunProbeL2 runs L2 once, records it, and raises or settles its findings.
func RunProbeL2(ctx context.Context) error {
	if !ProbeL2Enabled() {
		return nil
	}
	res := probeL2Once(ctx, os.Getenv(canaryProjectIDEnv), os.Getenv(canaryClientIDEnv))
	lastProbeL2.Store(&res)
	return recordProbe(ctx, claimtel.KindProbeL2, l2Classes, res, nil)
}

func recordProbe(ctx context.Context, kind string, classes []string, res probeResult, obs []claimtel.Observation) error {
	var seen []string
	if !res.Pass {
		seen = []string{res.Class}
		obs = append(obs, claimtel.Observation{Kind: kind, Subject: res.Class, Fail: true, At: res.At,
			Sample: map[string]any{"detail": res.Detail, "took_ms": res.TookMS}})
		probeLog(kind, res)
	}
	var err error
	if len(obs) > 0 {
		err = applyClaimFindings(ctx, obs, claimtel.ResolveAfterReads)
	}
	// A pass settles every class; a failure settles the classes it is not.
	// seen is never empty: NOT (subject = ANY(NULL)) would settle nothing.
	return errors.Join(err, dbSettleUnseenClaimFindings(ctx, kind, classes, append(seen, "")), refreshClaimFindings(ctx))
}

var probeLogged atomic.Pointer[string]

// probeLog says a failure once per class, not every minute.
func probeLog(kind string, res probeResult) {
	k := kind + ":" + res.Class
	if p := probeLogged.Load(); p != nil && *p == k {
		return
	}
	probeLogged.Store(&k)
	log.Printf("[DATA PLANE] %s failed (%s): %s", kind, res.Class, res.Detail)
}

func daysLeft(now, t time.Time) int { return int(t.Sub(now).Hours() / 24) }

func probeL1Once(ctx context.Context, base string) probeResult {
	start := time.Now()
	res := probeResult{At: start}
	done := func(class, detail string) probeResult {
		res.Pass, res.Class, res.Detail = class == "", class, detail
		res.TookMS = float64(time.Since(start).Microseconds()) / 1000
		return res
	}
	body := claimserve.CanaryPayload()
	pctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(pctx, http.MethodPost, strings.TrimRight(base, "/")+"/api/action/inject", bytes.NewReader(body))
	if err != nil {
		return done("unreachable", "SYNDRA_EXTERNAL_URL is not a URL")
	}
	req.Header.Set("Content-Type", "application/json")
	if keys := actionSigningKeys(); len(keys) > 0 {
		req.Header.Set(claimserve.SignatureHeader, claimserve.SignatureHeaderValue(time.Now(), body, keys[0].Key))
	}
	resp, err := probeClient().Do(req)
	if err != nil {
		class, detail := netFailure(err)
		return done(class, detail)
	}
	defer resp.Body.Close()
	if resp.TLS != nil && len(resp.TLS.PeerCertificates) > 0 {
		na := resp.TLS.PeerCertificates[0].NotAfter
		res.CertNotAfter = &na
	}
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	switch {
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return done("signature", fmt.Sprintf("HTTP %d: the door refused the key Syndra holds", resp.StatusCode))
	case resp.StatusCode != http.StatusOK:
		return done("status", fmt.Sprintf("HTTP %d", resp.StatusCode))
	}
	var out claimserve.Response
	if err := json.Unmarshal(raw, &out); err != nil {
		return done("claims", "the answer is not an Actions v2 response")
	}
	got, _ := json.Marshal(out.AppendClaims)
	if string(got) != claimserve.CanaryExpected {
		return done("claims", "append_claims "+truncate(string(got), 300)+", want "+claimserve.CanaryExpected)
	}
	return done("", "")
}

// netFailure names a failed request without its URL.
func netFailure(err error) (class, detail string) {
	var dnsErr *net.DNSError
	var certErr *tls.CertificateVerificationError
	var unknownCA x509.UnknownAuthorityError
	var hostErr x509.HostnameError
	var invalid x509.CertificateInvalidError
	var ne net.Error
	switch {
	case errors.As(err, &dnsErr):
		return "dns", "the host does not resolve from this LXC: " + dnsErr.Err
	case errors.As(err, &certErr), errors.As(err, &unknownCA), errors.As(err, &hostErr), errors.As(err, &invalid):
		return "tls", "the certificate does not verify"
	case errors.Is(err, context.DeadlineExceeded), errors.As(err, &ne) && ne.Timeout():
		return "timeout", "no answer within 5s"
	}
	var ue interface{ Unwrap() error }
	if errors.As(err, &ue) && ue.Unwrap() != nil {
		err = ue.Unwrap()
	}
	return "unreachable", truncate(err.Error(), 200)
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}

func probeL2Once(ctx context.Context, projectID, clientID string) probeResult {
	start := time.Now()
	res := probeResult{At: start}
	done := func(class, detail string) probeResult {
		res.Pass, res.Class, res.Detail = class == "", class, detail
		res.TookMS = float64(time.Since(start).Microseconds()) / 1000
		return res
	}
	pctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	// Client-credentials and JWT-profile grants pass no project: without the
	// audience scope Zitadel lists no grants (assessment Z-d, Z-e).
	tok, err := canaryToken(pctx, "openid urn:zitadel:iam:org:project:id:"+projectID+":aud")
	if err != nil {
		return done("token", "no token for the canary: "+truncate(err.Error(), 200))
	}
	payload, ok := jwtPayload(tok)
	if !ok {
		return done("token", "the canary's access token is not a JWT: set its token type to JWT in Zitadel")
	}
	userID, _ := payload["sub"].(string)
	if userID == "" {
		return done("token", "the canary's token names no subject")
	}

	want := claimPlane.Answer(pctx, userID, projectID, clientID, claims.FunctionPreAccessToken)
	if want.Outcome != claimtel.Served && want.Outcome != claimtel.ServedLKG {
		return done("no_facts", "Syndra's snapshot answers the canary "+string(want.Outcome))
	}
	if key, ok := missingClaim(payload, want.Claims); !ok {
		return done("claims", "the access token lacks Syndra's "+key+" (or carries another value)")
	}
	if _, ok := payload[logClaimL2]; ok {
		return done("log_claim", "Zitadel logged a dropped claim on the canary's token: a key Syndra sent collides")
	}
	if !sawCanaryCall(pctx, userID, clientID, start.Add(-5*time.Second)) {
		// The token's own client_id is what SYNDRA_CANARY_CLIENT_ID must be
		// when the two differ; it is no secret.
		tc, _ := payload["client_id"].(string)
		return done("not_called", fmt.Sprintf("the token was issued, but neither door counted a served preaccesstoken call for client %q (the token says client_id=%q)", clientID, tc))
	}

	info, err := canaryUserinfo(pctx, tok)
	if err != nil {
		return done("userinfo", "userinfo: "+truncate(err.Error(), 200))
	}
	if ui := claimPlane.Answer(pctx, userID, projectID, clientID, claims.FunctionPreUserinfo); ui.Outcome != claimtel.NotLanding {
		if key, ok := missingClaim(info, ui.Claims); !ok {
			return done("userinfo", "userinfo lacks Syndra's "+key)
		}
	}
	return done("", "")
}

// missingClaim is the first claim of want that got lacks or holds differently.
func missingClaim(got map[string]any, want []claimserve.Claim) (string, bool) {
	for _, c := range want {
		v, ok := got[c.Key]
		if !ok || !jsonEqual(v, c.Value) {
			return c.Key, false
		}
	}
	return "", true
}

func jsonEqual(a, b any) bool {
	ja, _ := json.Marshal(a)
	jb, _ := json.Marshal(b)
	var va, vb any
	_ = json.Unmarshal(ja, &va)
	_ = json.Unmarshal(jb, &vb)
	return reflect.DeepEqual(va, vb)
}

// jwtPayload reads a JWT's claims without verifying it: Syndra asked for this
// token itself, over TLS, from its own issuer.
func jwtPayload(tok string) (map[string]any, bool) {
	parts := strings.Split(tok, ".")
	if len(parts) != 3 {
		return nil, false
	}
	b, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, false
	}
	var out map[string]any
	return out, json.Unmarshal(b, &out) == nil && out != nil
}

// sawCanaryCall waits up to 10s (the claims container relays every 2s) for a
// served preaccesstoken call for the canary on either door.
func sawCanaryCall(ctx context.Context, userID, clientID string, since time.Time) bool {
	deadline := time.Now().Add(canaryCallWait)
	for {
		worst, _ := claimCalls{}.Window(userID, clientID, since, time.Now().Add(time.Second))
		if o, ok := worst[claimtel.FnPreAccessToken]; ok && (o == claimtel.Served || o == claimtel.ServedLKG) {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		select {
		case <-ctx.Done():
			return false
		case <-time.After(500 * time.Millisecond):
		}
	}
}

var canaryCallWait = 10 * time.Second

func fetchCanaryUserinfo(ctx context.Context, tok string) (map[string]any, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, canaryUserinfoURL(), nil)
	if err != nil {
		return nil, errors.New("ZITADEL_DOMAIN does not make a URL")
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := probeClient().Do(req)
	if err != nil {
		_, detail := netFailure(err)
		return nil, errors.New(detail)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	var out map[string]any
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&out); err != nil {
		return nil, errors.New("not JSON")
	}
	return out, nil
}

// probeHealth adds the probes to /healthz/claims: their state, and a reason
// for a failure (the same one its finding gives, so an outage of Postgres
// cannot hide it) or for a probe that has stopped running.
func probeHealth(now time.Time, h *claimsHealth, hard, soft []string) ([]string, []string) {
	state := func(enabled bool, p *probeResult, kind string, every time.Duration) (string, *int64) {
		switch {
		case !enabled:
			return "off", nil
		case p == nil:
			if now.Sub(claimCorrelator.Since()) > every+probeStaleAfter {
				soft = append(soft, kind+"_STALE")
			}
			return "not_run", nil
		}
		if now.Sub(p.At) > every+probeStaleAfter {
			soft = append(soft, kind+"_STALE")
		}
		if !p.Pass {
			hard = append(hard, claimtel.FindingReason(kind, p.Class))
			return "fail", ageS(now, p.At)
		}
		return "pass", ageS(now, p.At)
	}
	l1 := lastProbeL1.Load()
	h.ProbeL1, h.ProbeL1AgeS = state(ProbeL1Enabled(), l1, claimtel.KindProbeL1, probeL1Every)
	if l1 != nil && l1.CertNotAfter != nil {
		d := daysLeft(now, *l1.CertNotAfter)
		h.CertDaysLeft = &d
		if l1.CertNotAfter.Sub(now) < certWarnWithin {
			soft = append(soft, claimtel.KindCertExpiring)
		}
	}
	h.ProbeL2, h.ProbeL2AgeS = state(ProbeL2Enabled(), lastProbeL2.Load(), claimtel.KindProbeL2, probeL2Every)
	return hard, soft
}

// probesSnapshot is the probes for the operator view.
func probesSnapshot() map[string]any {
	return map[string]any{
		"l1": map[string]any{"enabled": ProbeL1Enabled(), "last": lastProbeL1.Load()},
		"l2": map[string]any{"enabled": ProbeL2Enabled(), "last": lastProbeL2.Load()},
	}
}
