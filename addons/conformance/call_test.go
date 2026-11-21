package conformance

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"

	"syndra/addons/reference"
)

// callServer is the reference over TLS, plus account.release (which the
// reference does not serve) answered 200 only for a request whose signature
// verifies under the derived key, so the release test proves the signing too.
// It returns the URL and every body it saw for a release or an adoption.
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
		if r.URL.Path != "/operations/account.release" && r.URL.Path != "/operations/account.adopt" {
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
		func() []map[string]any {
			mu.Lock()
			defer mu.Unlock()
			return append([]map[string]any(nil), releases...)
		},
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
		"release, a member":        {"POST", "/operations/account.release", `{"subject":"300000000000000001"}`},
		"release, empty subject":   {"POST", "/operations/account.release", `{"subject":""}`},
		"release, no subject":      {"POST", "/operations/account.release", `{}`},
		"release, another op body": {"POST", "/operations/account.release", `{"subject":"conformance-x","operation":"account.purge"}`},
		"settings, POST":           {"POST", "/settings/home_root", `{"value":""}`},
		"settings, DELETE":         {"DELETE", "/settings/home_root", ``},
		"settings, traversal":      {"PUT", "/settings/../operations/account.purge", `{"value":""}`},
		"settings, nested":         {"PUT", "/settings/a/b", `{"value":""}`},
		"settings, query":          {"PUT", "/settings/home_root?x=1", `{"value":""}`},
		"read an operation":        {"GET", "/operations/account.release", ``},
		"read a plan":              {"GET", "/plan", ``},
		"not an object":            {"POST", "/operations/account.release", `[1]`},
		"folded key (long s)":      {"POST", "/operations/account.release", `{"subject":"conformance-x","\u017fubject":"wwe-the-rock"}`},
		"folded key, first":        {"POST", "/operations/account.release", `{"\u017fubject":"wwe-the-rock","subject":"conformance-x"}`},
		"capitalised key":          {"POST", "/operations/account.release", `{"Subject":"conformance-x"}`},
		"capitalised beside":       {"POST", "/operations/account.release", `{"subject":"conformance-x","Subject":"wwe-the-rock"}`},
		"duplicate subject":        {"POST", "/operations/account.release", `{"subject":"conformance-x","subject":"wwe-the-rock"}`},
		"duplicate, member first":  {"POST", "/operations/account.release", `{"subject":"wwe-the-rock","subject":"conformance-x"}`},
		"trailing junk":            {"POST", "/operations/account.release", `{"subject":"conformance-x"} junk`},
		"second document":          {"POST", "/operations/account.release", `{"subject":"conformance-x"}{"subject":"wwe-the-rock"}`},
		"subject not a string":     {"POST", "/operations/account.release", `{"subject":null}`},
		"release with an actor":    {"POST", "/operations/account.release", `{"subject":"conformance-x","actor":"someone-else"}`},
		"release with params":      {"POST", "/operations/account.release", `{"subject":"conformance-x","params":{"a":1}}`},
		"setting with an actor":    {"PUT", "/settings/home_root", `{"value":"","actor":"someone-else"}`},
		"setting with a call_id":   {"PUT", "/settings/home_root", `{"value":"","call_id":"x"}`},
		"setting, folded value":    {"PUT", "/settings/home_root", `{"value":"","\u017ealue":"x"}`},
		"setting, capital Value":   {"PUT", "/settings/home_root", `{"Value":""}`},
		"setting, no body":         {"PUT", "/settings/home_root", ``},
		"setting, no value":        {"PUT", "/settings/home_root", `{}`},
		"setting, value not text":  {"PUT", "/settings/home_root", `{"value":1}`},
		"setting, trailing":        {"PUT", "/settings/home_root", `{"value":""}x`},
		"setting read by name":     {"GET", "/settings/home_root", ``},
		"read with a body":         {"GET", "/health", `{}`},
		"adopt, a member":          {"POST", "/operations/account.adopt", `{"subject":"300000000000000001","params":{"username":"conformance-home-1","account_id":"200007"}}`},
		"adopt, empty subject":     {"POST", "/operations/account.adopt", `{"subject":"","params":{"username":"conformance-home-1","account_id":"200007"}}`},
		"adopt, member's account":  {"POST", "/operations/account.adopt", `{"subject":"rey_student","params":{"username":"rey","account_id":"200007"}}`},
		"adopt, bare prefix":       {"POST", "/operations/account.adopt", `{"subject":"rey_student","params":{"username":"conformance-","account_id":"200007"}}`},
		"adopt, no account_id":     {"POST", "/operations/account.adopt", `{"subject":"rey_student","params":{"username":"conformance-home-1"}}`},
		"adopt, empty account_id":  {"POST", "/operations/account.adopt", `{"subject":"rey_student","params":{"username":"conformance-home-1","account_id":""}}`},
		"adopt, numeric id":        {"POST", "/operations/account.adopt", `{"subject":"rey_student","params":{"username":"conformance-home-1","account_id":200007}}`},
		"adopt, no params":         {"POST", "/operations/account.adopt", `{"subject":"rey_student"}`},
		"adopt, null params":       {"POST", "/operations/account.adopt", `{"subject":"rey_student","params":null}`},
		"adopt, subject not text":  {"POST", "/operations/account.adopt", `{"subject":7,"params":{"username":"conformance-home-1","account_id":"200007"}}`},
		"adopt, extra field":       {"POST", "/operations/account.adopt", `{"subject":"rey_student","params":{"username":"conformance-home-1","account_id":"200007"},"actor":"x"}`},
		"adopt, extra param":       {"POST", "/operations/account.adopt", `{"subject":"rey_student","params":{"username":"conformance-home-1","account_id":"200007","home":"/x"}}`},
		"adopt, another op":        {"POST", "/operations/account.adopt", `{"subject":"rey_student","operation":"account.purge","params":{"username":"conformance-home-1","account_id":"200007"}}`},
		"adopt, duplicate subject": {"POST", "/operations/account.adopt", `{"subject":"rey_student","subject":"300000000000000001","params":{"username":"conformance-home-1","account_id":"200007"}}`},
		"adopt, duplicate param":   {"POST", "/operations/account.adopt", `{"subject":"rey_student","params":{"username":"conformance-home-1","username":"rey","account_id":"200007"}}`},
		"adopt, folded param":      {"POST", "/operations/account.adopt", `{"subject":"rey_student","params":{"username":"conformance-home-1","USERNAME":"rey","account_id":"200007"}}`},
		"adopt, trailing junk":     {"POST", "/operations/account.adopt", `{"subject":"rey_student","params":{"username":"conformance-home-1","account_id":"200007"}} x`},
		"adopt, read":              {"GET", "/operations/account.adopt", ``},
		"adopt, PUT":               {"PUT", "/operations/account.adopt", `{"subject":"rey_student","params":{"username":"conformance-home-1","account_id":"200007"}}`},
		"adopt, near path":         {"POST", "/operations/account.adopt/", `{"subject":"rey_student","params":{"username":"conformance-home-1","account_id":"200007"}}`},
		"purge, a test subject":    {"POST", "/operations/account.purge", `{"subject":"rey_student"}`},
		"create":                   {"POST", "/operations/account.create", `{"subject":"rey_student"}`},
		"apply":                    {"POST", "/apply", `{"subject":"rey_student"}`},
		"plan":                     {"POST", "/plan", `{}`},
		"calls":                    {"GET", "/calls", ``},
		"stats":                    {"GET", "/stats", ``},
	} {
		if status, _, err := Call(context.Background(), callCfg(url), c.method, c.path, []byte(c.body)); err == nil || status != 0 {
			t.Errorf("%s: sent, answered %d, err %v", name, status, err)
		}
	}
	if s := seen(); len(s) != 0 {
		t.Fatalf("a refused call reached the add-on: %v", s)
	}
}

// A release or an adoption goes out for a test subject (any that is not all
// digits, as every person's is), built from the typed values alone.
func TestCallSendsAdoptAndReleaseForATestSubject(t *testing.T) {
	for name, c := range map[string]struct {
		path, body string
		want       map[string]any
	}{
		"adopt": {"/operations/account.adopt", `{"subject":"rey_student","params":{"username":"conformance-home-20261114123616","account_id":"200007"}}`,
			map[string]any{"subject": "rey_student", "operation": "account.adopt",
				"params": map[string]any{"username": "conformance-home-20261114123616", "account_id": "200007"}}},
		"release, a ring name": {"/operations/account.release", `{"subject":"rey_student"}`,
			map[string]any{"subject": "rey_student", "operation": "account.release", "params": map[string]any{}}},
		"release, the kit's": {"/operations/account.release", `{"subject":"conformance-wwe-rey"}`,
			map[string]any{"subject": "conformance-wwe-rey", "operation": "account.release", "params": map[string]any{}}},
	} {
		url, sent, _ := callServer(t)
		if status, reply, err := Call(context.Background(), callCfg(url), "POST", c.path, []byte(c.body)); err != nil || status != 200 {
			t.Fatalf("%s: answered %d %s, %v", name, status, reply, err)
		}
		got := sent()
		if len(got) != 1 {
			t.Fatalf("%s: the add-on saw %v", name, got)
		}
		for _, k := range []string{"contract_version", "call_id", "actor"} {
			delete(got[0], k)
		}
		if !reflect.DeepEqual(got[0], c.want) {
			t.Errorf("%s: sent %v, want %v", name, got[0], c.want)
		}
	}
}

// The tool sets the actor, the call_id and the contract version itself.
func TestCallActorIsTheToolsOwn(t *testing.T) {
	url, releases, _ := callServer(t)
	if _, _, err := Call(context.Background(), callCfg(url), "POST", "/operations/account.release", []byte(`{"subject":"conformance-wwe-rey"}`)); err != nil {
		t.Fatal(err)
	}
	if got := releases(); len(got) != 1 || got[0]["actor"] != "syndra-addon-call" || len(got[0]) != 6 {
		t.Fatalf("the add-on saw %v", got)
	}
}

func TestCallRefusesABaseURLThatIsMoreThanAnOrigin(t *testing.T) {
	url, _, seen := callServer(t)
	for _, base := range []string{url + "/x", url + "?a=1", url + "/?a=1", url + "#f", url + "?", "https://user:pw@" + strings.TrimPrefix(url, "https://")} {
		if status, _, err := Call(context.Background(), callCfg(base), "GET", "/health", nil); err == nil || status != 0 {
			t.Errorf("%q: sent, answered %d, err %v", base, status, err)
		}
	}
	if s := seen(); len(s) != 0 {
		t.Fatalf("a refused call reached the add-on: %v", s)
	}
	if status, _, err := Call(context.Background(), callCfg(url+"/"), "GET", "/health", nil); err != nil || status != 200 {
		t.Fatalf("a trailing slash is the origin: %d %v", status, err)
	}
}

func TestCallNeverPrintsTheSecret(t *testing.T) {
	_, _, err := Call(context.Background(), CallConfig{URL: "https://127.0.0.1:1", Target: "reference", Secret: []byte(testSecret)}, "GET", "/health", nil)
	if err == nil || strings.Contains(err.Error(), testSecret) {
		t.Fatalf("want a connection error without the secret, got %v", err)
	}
}
