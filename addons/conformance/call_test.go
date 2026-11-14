package conformance

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"syndra/addons/reference"
)

// callServer is the reference over TLS, plus account.release (which the
// reference does not serve) answered 200 only for a request whose signature
// verifies under the derived key, so the release test proves the signing too.
// It returns the URL and every body it saw for the release.
func callServer(t *testing.T) (string, func() []map[string]any, func() []string) {
	t.Helper()
	addon, err := reference.New(reference.Config{Target: "reference", Secret: []byte(testSecret)})
	if err != nil {
		t.Fatal(err)
	}
	_, key, _ := deriveKeys([]byte(testSecret), "reference")
	var mu sync.Mutex
	var releases []map[string]any
	var seen []string
	h := addon.Handler()
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seen = append(seen, r.Method+" "+r.URL.Path)
		mu.Unlock()
		if r.URL.Path != "/operations/account.release" {
			h.ServeHTTP(w, r)
			return
		}
		body, _ := io.ReadAll(r.Body)
		var ts int64
		var sig string
		fmt.Sscanf(strings.ReplaceAll(strings.ReplaceAll(r.Header.Get(signatureHeader), "t=", ""), ",v1=", " "), "%d %s", &ts, &sig)
		if sig != mac(ts, r.Method, signedTarget(r.URL), body, key) {
			http.Error(w, `{"code":"BAD_SIGNATURE"}`, http.StatusUnauthorized)
			return
		}
		var doc map[string]any
		_ = json.Unmarshal(body, &doc)
		mu.Lock()
		releases = append(releases, doc)
		mu.Unlock()
		_, _ = w.Write([]byte(`{"outcome":"succeeded"}`))
	}))
	srv.TLS = &tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{addon.Certificate()}}
	srv.StartTLS()
	t.Cleanup(srv.Close)
	return srv.URL,
		func() []map[string]any { mu.Lock(); defer mu.Unlock(); return append([]map[string]any(nil), releases...) },
		func() []string { mu.Lock(); defer mu.Unlock(); return append([]string(nil), seen...) }
}

func callCfg(url string) CallConfig {
	return CallConfig{URL: url, Target: "reference", Secret: []byte(testSecret)}
}

func TestCallSettingRoundTrips(t *testing.T) {
	url, _, _ := callServer(t)
	ctx := context.Background()
	status, reply, err := Call(ctx, callCfg(url), "PUT", "/settings/home_printer", []byte(`{"value":"mk4-a"}`))
	if err != nil || status != 200 {
		t.Fatalf("PUT answered %d %s, %v", status, reply, err)
	}
	status, reply, err = Call(ctx, callCfg(url), "GET", "/settings", nil)
	if err != nil || status != 200 || !strings.Contains(string(reply), `"value":"mk4-a"`) || !strings.Contains(string(reply), `"set_by":"syndra-addon-call"`) {
		t.Fatalf("GET /settings answered %d %s, %v", status, reply, err)
	}
	// A clear is a value of "", sent the same way.
	if status, reply, err = Call(ctx, callCfg(url), "PUT", "/settings/home_printer", []byte(`{"value":""}`)); err != nil || status != 200 {
		t.Fatalf("clear answered %d %s, %v", status, reply, err)
	}
}

func TestCallReleasesAConformanceSubject(t *testing.T) {
	url, releases, _ := callServer(t)
	status, reply, err := Call(context.Background(), callCfg(url), "POST", "/operations/account.release", []byte(`{"subject":"conformance-wwe-rey"}`))
	if err != nil || status != 200 {
		t.Fatalf("release answered %d %s, %v", status, reply, err)
	}
	got := releases()
	if len(got) != 1 || got[0]["subject"] != "conformance-wwe-rey" || got[0]["operation"] != "account.release" ||
		got[0]["contract_version"] != float64(Major) || got[0]["call_id"] == "" || got[0]["params"] == nil {
		t.Fatalf("the add-on saw %v", got)
	}
}

func TestCallRefusalsSendNothing(t *testing.T) {
	url, _, seen := callServer(t)
	for name, c := range map[string]struct{ method, path, body string }{
		"purge":                    {"POST", "/operations/account.purge", `{"subject":"conformance-x"}`},
		"another operation":        {"POST", "/operations/password.set", `{"subject":"conformance-x"}`},
		"release, a member":        {"POST", "/operations/account.release", `{"subject":"wwe-the-rock"}`},
		"release, no subject":      {"POST", "/operations/account.release", `{}`},
		"release, bare prefix":     {"POST", "/operations/account.release", `{"subject":"conformance-"}`},
		"release, another op body": {"POST", "/operations/account.release", `{"subject":"conformance-x","operation":"account.purge"}`},
		"settings, POST":           {"POST", "/settings/home_root", `{"value":""}`},
		"settings, DELETE":         {"DELETE", "/settings/home_root", ``},
		"settings, traversal":      {"PUT", "/settings/../operations/account.purge", `{"value":""}`},
		"settings, nested":         {"PUT", "/settings/a/b", `{"value":""}`},
		"settings, query":          {"PUT", "/settings/home_root?x=1", `{"value":""}`},
		"read an operation":        {"GET", "/operations/account.release", ``},
		"read a plan":              {"GET", "/plan", ``},
		"not an object":            {"POST", "/operations/account.release", `[1]`},
	} {
		if status, _, err := Call(context.Background(), callCfg(url), c.method, c.path, []byte(c.body)); err == nil || status != 0 {
			t.Errorf("%s: sent, answered %d, err %v", name, status, err)
		}
	}
	if s := seen(); len(s) != 0 {
		t.Fatalf("a refused call reached the add-on: %v", s)
	}
}

func TestCallNeverPrintsTheSecret(t *testing.T) {
	_, _, err := Call(context.Background(), CallConfig{URL: "https://127.0.0.1:1", Target: "reference", Secret: []byte(testSecret)}, "GET", "/health", nil)
	if err == nil || strings.Contains(err.Error(), testSecret) {
		t.Fatalf("want a connection error without the secret, got %v", err)
	}
}
