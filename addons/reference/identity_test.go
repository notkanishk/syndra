package reference

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The reference's request identity reproduces the contract's four JCS vectors
// byte for byte, and two spellings of one JSON value are one identity.
func TestIdentityDigestReproducesTheContractVectors(t *testing.T) {
	paths, _ := filepath.Glob(filepath.Join("..", "contract", "v2", "fixtures", "identity_*.json"))
	if len(paths) != 4 {
		t.Fatalf("found %d identity vectors, want 4", len(paths))
	}
	for _, p := range paths {
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		var vec struct {
			Input     any    `json:"input"`
			Canonical string `json:"canonical"`
			SHA256    string `json:"sha256"`
		}
		if err := json.Unmarshal(raw, &vec); err != nil {
			t.Fatal(err)
		}
		if got := string(canonical(vec.Input)); got != vec.Canonical {
			t.Errorf("%s: canonical\n got  %s\n want %s", filepath.Base(p), got, vec.Canonical)
		}
		if got := identityDigest(vec.Input); got != vec.SHA256 {
			t.Errorf("%s: sha256 %s, want %s", filepath.Base(p), got, vec.SHA256)
		}
	}
	a := map[string]json.RawMessage{"printers": json.RawMessage(`[ "mk4-a" ]`), "enabled": json.RawMessage(`true`)}
	b := map[string]json.RawMessage{"enabled": json.RawMessage(`true`), "printers": json.RawMessage(`["mk4-a"]`)}
	if identityDigest(plain(a)) != identityDigest(plain(b)) {
		t.Error("two spellings of the same desired state are different identities")
	}
}

// Through the wire: the same call_id with the same desired state spelled
// another way is a replay, answered byte for byte, never CALL_ID_REUSED.
func TestASameCallSpelledDifferentlyIsAReplay(t *testing.T) {
	a, err := New(Config{Target: "reference", Secret: []byte("reference-test-secret")})
	if err != nil {
		t.Fatal(err)
	}
	h := a.Handler()
	post := func(target, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPost, target, strings.NewReader(body))
		now := time.Now().Unix()
		r.Header.Set(SignatureHeader, fmt.Sprintf("t=%d,v1=%x", now, computeMAC(now, http.MethodPost, target, []byte(body), a.auth.key)))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	plan := post("/plan", `{"contract_version":2,"subjects":[{"subject":"s-1","email":"seth.rollins@example.org","desired":{"enabled":true}}]}`)
	var p struct {
		Outcomes []struct{ Fingerprint string }
	}
	if err := json.Unmarshal(plan.Body.Bytes(), &p); err != nil || len(p.Outcomes) != 1 {
		t.Fatalf("plan: %d %s", plan.Code, plan.Body)
	}
	apply := `{"contract_version":2,"call_id":"c-1","subject":"s-1","email":"seth.rollins@example.org","fingerprint":"` + p.Outcomes[0].Fingerprint + `","desired":%s}`
	first := post("/apply", fmt.Sprintf(apply, `{"enabled":true,"printers":["mk4-a"]}`))
	again := post("/apply", fmt.Sprintf(apply, `{ "printers" : [ "mk4-a" ], "enabled" : true }`))
	if first.Code != 200 || again.Code != first.Code || again.Body.String() != first.Body.String() {
		t.Errorf("first %d %s\nagain %d %s", first.Code, first.Body, again.Code, again.Body)
	}
	if got := string(canonical(map[string]any{"reason": "<lab & co>"})); got != `{"reason":"<lab & co>"}` {
		t.Errorf("canonical escapes what RFC 8785 writes raw: %s", got)
	}
}
