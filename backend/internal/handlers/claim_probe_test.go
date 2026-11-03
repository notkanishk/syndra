package handlers

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"syndra/internal/cache"
	"syndra/internal/claims"
	"syndra/internal/claimserve"
	"syndra/internal/claimserve/claimservetest"
	"syndra/internal/claimtel"
	"syndra/internal/services"
)

var (
	probeKey   = strings.Repeat("k", 32)
	otherKey   = strings.Repeat("x", 32)
	canaryJSON = func(roles ...string) string {
		f := claimserve.CanaryFacts()
		f.Roles = roles
		b, _ := json.Marshal(f)
		return string(b)
	}
)

// tlsDoor serves h over TLS with a certificate for 127.0.0.1 that ends at
// notAfter, and points the probe's client at it (trusting that certificate).
func tlsDoor(t *testing.T, h http.Handler, notAfter time.Time) *httptest.Server {
	t.Helper()
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "syndra.example.edu"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: notAfter,
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}, KeyUsage: x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cert, _ := x509.ParseCertificate(der)
	srv := httptest.NewUnstartedServer(h)
	srv.TLS = &tls.Config{Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: key}}}
	srv.StartTLS()
	t.Cleanup(srv.Close)
	pool := x509.NewCertPool()
	pool.AddCert(cert)
	useProbeClient(t, &http.Client{Timeout: 5 * time.Second, Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool}}})
	t.Setenv("SYNDRA_EXTERNAL_URL", srv.URL)
	return srv
}

func useProbeClient(t *testing.T, c *http.Client) {
	t.Helper()
	orig := probeClient
	t.Cleanup(func() { probeClient = orig })
	probeClient = func() *http.Client { return c }
}

func resetProbes(t *testing.T) {
	t.Helper()
	a, b := lastProbeL1.Load(), lastProbeL2.Load()
	lastProbeL1.Store(nil)
	lastProbeL2.Store(nil)
	t.Cleanup(func() { lastProbeL1.Store(a); lastProbeL2.Store(b) })
}

// canaryPlane is this process's snapshot with the canary (and p1) published
// and the canary's facts as given, in production mode with probeKey.
func canaryPlane(t *testing.T, factsJSON string) *claimservetest.Redis {
	t.Helper()
	stubHealthDeps(t, nil, &cache.WarmReport{At: time.Now(), Refreshed: 1, Of: 1}, 0)
	r := testPlane(t, map[string]claimserve.Project{
		"p1":                       project(services.ProjectShapes{}, failClosed),
		claimserve.CanaryProjectID: claimserve.CanaryProject(),
	})
	if factsJSON != "" {
		setFacts(r, claimserve.CanaryUserID, claimserve.CanaryProjectID, factsJSON)
	}
	t.Setenv("ZITADEL_DOMAIN", "zitadel.example.edu")
	t.Setenv(actionKeyEnv, probeKey)
	return r
}

// L1 through TLS to this process's door: the exact canary claims, the
// certificate's end recorded, a certificate two days from its end a soft
// finding, and /healthz/claims saying pass.
func TestProbeL1_PassesAndWatchesTheCertificate(t *testing.T) {
	sink := stubClaimFindings(t)
	freshRecorder(t)
	resetProbes(t)
	canaryPlane(t, canaryJSON("probe"))
	notAfter := time.Now().Add(3 * 24 * time.Hour)
	tlsDoor(t, NewClaimsRouter(), notAfter)

	if err := RunProbeL1(context.Background()); err != nil {
		t.Fatal(err)
	}
	res := lastProbeL1.Load()
	if res == nil || !res.Pass || res.CertNotAfter == nil || !res.CertNotAfter.Equal(notAfter.Truncate(time.Second)) {
		t.Fatalf("L1: %+v", res)
	}
	if k := sink.kinds(); k[claimtel.KindCertExpiring+" L1"] != 1 || k[claimtel.KindProbeL1+" claims"] != 0 {
		t.Fatalf("findings %v, want CERT_EXPIRING only", k)
	}
	// Labelled probe: counted apart, never in the SLIs or the correlator.
	if claimDelivery.Probes()[claimtel.Served] != 1 || len(claimDelivery.Recent(10)) != 0 || claimDelivery.ActivePairs() != 0 {
		t.Fatalf("probe telemetry: probes %v recent %v", claimDelivery.Probes(), claimDelivery.Recent(10))
	}
	code, h, _ := getHealth(t)
	if code != 200 || h.ProbeL1 != "pass" || h.ProbeL1AgeS == nil || h.CertDaysLeft == nil || *h.CertDaysLeft != 2 ||
		strings.Join(h.Reasons, ",") != claimtel.KindCertExpiring || h.ProbeL2 != "off" {
		t.Fatalf("health: HTTP %d %+v", code, h)
	}

	// A certificate with months left: clean, and the finding settles.
	sink.ups = nil
	tlsDoor(t, NewClaimsRouter(), time.Now().Add(80*24*time.Hour))
	_ = RunProbeL1(context.Background())
	for _, u := range sink.ups {
		if u.Kind == claimtel.KindCertExpiring && (u.Fails != 0 || u.TrailingClean != 1) {
			t.Fatalf("a renewed certificate: %+v", u)
		}
	}
	if _, h, _ := getHealth(t); h.Status != "ok" {
		t.Fatalf("after renewal: %+v", h)
	}
}

// Each way L1 fails names its own class: on the finding, and on
// /healthz/claims as a hard reason with probe_l1 fail.
func TestProbeL1_FailuresEachNameTheirClass(t *testing.T) {
	cases := []struct {
		name  string
		class string
		setup func(t *testing.T)
	}{
		{"the door holds another key", "signature", func(t *testing.T) {
			canaryPlane(t, canaryJSON("probe"))
			r := claimservetest.New()
			tlsDoor(t, claimserve.NewProcess(r, otherKey, false).Router(), time.Now().Add(80*24*time.Hour))
		}},
		{"the snapshot answers other claims", "claims", func(t *testing.T) {
			canaryPlane(t, canaryJSON("someone-else"))
			tlsDoor(t, NewClaimsRouter(), time.Now().Add(80*24*time.Hour))
		}},
		{"the canary's facts are gone", "claims", func(t *testing.T) {
			canaryPlane(t, "")
			tlsDoor(t, NewClaimsRouter(), time.Now().Add(80*24*time.Hour))
		}},
		{"the host does not resolve", "dns", func(t *testing.T) {
			canaryPlane(t, canaryJSON("probe"))
			t.Setenv("SYNDRA_EXTERNAL_URL", "https://syndra.example.edu")
			useProbeClient(t, &http.Client{Transport: &http.Transport{DialContext: func(context.Context, string, string) (net.Conn, error) {
				return nil, &net.DNSError{Err: "no such host", Name: "syndra.example.edu", IsNotFound: true}
			}}})
		}},
		{"the certificate does not verify", "tls", func(t *testing.T) {
			canaryPlane(t, canaryJSON("probe"))
			srv := tlsDoor(t, NewClaimsRouter(), time.Now().Add(80*24*time.Hour))
			useProbeClient(t, &http.Client{Timeout: 5 * time.Second}) // the system roots, which do not hold it
			t.Setenv("SYNDRA_EXTERNAL_URL", srv.URL)
		}},
		{"a route that is not the claims door", "status", func(t *testing.T) {
			canaryPlane(t, canaryJSON("probe"))
			tlsDoor(t, http.NotFoundHandler(), time.Now().Add(80*24*time.Hour))
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sink := stubClaimFindings(t)
			freshRecorder(t)
			resetProbes(t)
			tc.setup(t)
			_ = RunProbeL1(context.Background())
			res := lastProbeL1.Load()
			if res == nil || res.Pass || res.Class != tc.class {
				t.Fatalf("L1: %+v, want class %s", res, tc.class)
			}
			if strings.Contains(res.Detail, probeKey) || strings.Contains(res.Detail, "127.0.0.1") {
				t.Fatalf("the detail carries a key or the URL: %q", res.Detail)
			}
			if n := sink.kinds()[claimtel.KindProbeL1+" "+tc.class]; n != 1 {
				t.Fatalf("findings %v", sink.kinds())
			}
			code, h, _ := getHealth(t)
			if code != 503 || h.ProbeL1 != "fail" || !strings.Contains(strings.Join(h.Reasons, ","), "PROBE_L1:"+tc.class) {
				t.Fatalf("health: HTTP %d %+v", code, h)
			}
		})
	}
}

// Off without a public URL: no run, no finding, no reason.
func TestProbeL1_OffWithoutAURL(t *testing.T) {
	stubClaimFindings(t)
	resetProbes(t)
	stubHealthDeps(t, nil, &cache.WarmReport{At: time.Now(), Refreshed: 1, Of: 1}, 0)
	t.Setenv("SYNDRA_EXTERNAL_URL", "")
	if err := RunProbeL1(context.Background()); err != nil || lastProbeL1.Load() != nil {
		t.Fatal("L1 ran with no URL")
	}
	if _, h, _ := getHealth(t); h.ProbeL1 != "off" || h.Status != "ok" {
		t.Fatalf("%+v", h)
	}
}

func jwtFor(claims map[string]any) string {
	enc := func(v any) string { b, _ := json.Marshal(v); return base64.RawURLEncoding.EncodeToString(b) }
	return enc(map[string]string{"alg": "RS256"}) + "." + enc(claims) + ".sig"
}

// L2 with fakes for Zitadel: a token carrying the canary's claims, a call the
// telemetry saw, and userinfo; each missing piece is its own class.
func TestProbeL2(t *testing.T) {
	setup := func(t *testing.T) (*claimservetest.Redis, *map[string]any) {
		t.Helper()
		stubClaimFindings(t)
		freshRecorder(t)
		resetProbes(t)
		stubHealthDeps(t, nil, &cache.WarmReport{At: time.Now(), Refreshed: 1, Of: 1}, 0)
		r := builtinPlane(t, "pC")
		setFacts(r, "uC", "pC", factsJSON("uC", "pC", "canary"))
		t.Setenv("ZITADEL_DOMAIN", "zitadel.example.edu")
		t.Setenv(canaryKeyPathEnv, "/etc/syndra/canary-key.json")
		t.Setenv(canaryProjectIDEnv, "pC")
		t.Setenv(canaryClientIDEnv, "cC")
		ct, cu, cw := canaryToken, canaryUserinfo, canaryCallWait
		t.Cleanup(func() { canaryToken, canaryUserinfo, canaryCallWait = ct, cu, cw })
		canaryCallWait = 50 * time.Millisecond
		tok := map[string]any{"sub": "uC", "roles": []string{"canary"}}
		canaryToken = func(_ context.Context, scope string) (string, error) {
			if !strings.Contains(scope, "urn:zitadel:iam:org:project:id:pC:aud") {
				return "", errors.New("no audience scope")
			}
			return jwtFor(tok), nil
		}
		canaryUserinfo = func(context.Context, string) (map[string]any, error) {
			return map[string]any{"sub": "uC", "roles": []any{"canary"}}, nil
		}
		return r, &tok
	}
	served := func() {
		claimDelivery.Record(claimtel.Call{UserID: "uC", ClientID: "cC", Function: claims.FunctionPreAccessToken, Outcome: claimtel.Served, At: time.Now()})
	}

	// Owner, 3 Nov 2026: L2 reuses Syndra's own service account. With only the
	// project set, the key is ZITADEL_MACHINE_KEY_PATH and the client id is the
	// token's own.
	t.Run("on Syndra's own service account", func(t *testing.T) {
		_, tok := setup(t)
		t.Setenv(canaryKeyPathEnv, "")
		t.Setenv(canaryClientIDEnv, "")
		t.Setenv("ZITADEL_MACHINE_KEY_PATH", "/etc/syndra/zitadel-sa-key.json")
		(*tok)["client_id"] = "cC"
		if !ProbeL2Enabled() || canaryKeyPath() != "/etc/syndra/zitadel-sa-key.json" {
			t.Fatalf("enabled=%v key=%q", ProbeL2Enabled(), canaryKeyPath())
		}
		served()
		if err := RunProbeL2(context.Background()); err != nil {
			t.Fatal(err)
		}
		if res := lastProbeL2.Load(); res == nil || !res.Pass {
			t.Fatalf("L2: %+v", res)
		}
	})
	t.Run("pass", func(t *testing.T) {
		setup(t)
		served()
		if err := RunProbeL2(context.Background()); err != nil {
			t.Fatal(err)
		}
		if res := lastProbeL2.Load(); res == nil || !res.Pass {
			t.Fatalf("L2: %+v", res)
		}
		if _, h, _ := getHealth(t); h.ProbeL2 != "pass" {
			t.Fatalf("%+v", h)
		}
	})
	for _, tc := range []struct {
		name, class string
		mutate      func(r *claimservetest.Redis, tok *map[string]any)
		call        bool
	}{
		{"the token lacks Syndra's claim", "claims", func(_ *claimservetest.Redis, tok *map[string]any) { delete(*tok, "roles") }, true},
		{"the token holds another value", "claims", func(_ *claimservetest.Redis, tok *map[string]any) { (*tok)["roles"] = []string{"x"} }, true},
		{"Zitadel dropped a key", "log_claim", func(_ *claimservetest.Redis, tok *map[string]any) { (*tok)[logClaimL2] = "roles collides" }, true},
		{"no call reached either door", "not_called", func(*claimservetest.Redis, *map[string]any) {}, false},
		{"Syndra holds no facts for the canary", "no_facts", func(r *claimservetest.Redis, _ *map[string]any) {
			r.HDel(claimserve.FactsKey("uC"), "pC")
		}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, tok := setup(t)
			tc.mutate(r, tok)
			if tc.call {
				served()
			}
			_ = RunProbeL2(context.Background())
			if res := lastProbeL2.Load(); res == nil || res.Pass || res.Class != tc.class {
				t.Fatalf("L2: %+v, want %s", res, tc.class)
			}
			if code, h, _ := getHealth(t); code != 503 || !strings.Contains(strings.Join(h.Reasons, ","), "PROBE_L2:"+tc.class) {
				t.Fatalf("health: %d %+v", code, h)
			}
		})
	}
	t.Run("no token", func(t *testing.T) {
		setup(t)
		canaryToken = func(context.Context, string) (string, error) { return "", errors.New("invalid_grant") }
		_ = RunProbeL2(context.Background())
		if res := lastProbeL2.Load(); res.Class != "token" {
			t.Fatalf("%+v", res)
		}
	})
	t.Run("userinfo lacks the claim", func(t *testing.T) {
		setup(t)
		served()
		canaryUserinfo = func(context.Context, string) (map[string]any, error) { return map[string]any{"sub": "uC"}, nil }
		_ = RunProbeL2(context.Background())
		if res := lastProbeL2.Load(); res.Class != "userinfo" {
			t.Fatalf("%+v", res)
		}
	})
	t.Run("off until the owner sets it", func(t *testing.T) {
		setup(t)
		t.Setenv(canaryProjectIDEnv, "")
		if err := RunProbeL2(context.Background()); err != nil || lastProbeL2.Load() != nil {
			t.Fatal("L2 ran with no project set")
		}
	})
}
