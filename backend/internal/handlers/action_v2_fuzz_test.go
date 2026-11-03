package handlers

import (
	"bytes"
	"context"
	"crypto/hmac"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"syndra/internal/claimserve"
	"syndra/internal/claimserve/claimservetest"
	"syndra/internal/claimtel"
	"syndra/internal/services"
)

// fuzzRoute is the real route with fast, always-answering dependencies, and
// the outcome of the last call.
func fuzzRoute(_ *testing.F) (func(body []byte) (int, claimtel.Outcome), func()) {
	plane, rec := claimPlane, recordClaimCall
	restore := func() { claimPlane, recordClaimCall = plane, rec }
	r := claimservetest.New()
	_, _ = claimserve.Publish(context.Background(), r, map[string]claimserve.Project{"p1": project(services.ProjectShapes{}, failClosed)})
	claimPlane = claimserve.New(r, nil)
	_ = claimPlane.Poll(context.Background())
	setFacts(r, "u1", "p1", factsJSON("u1", "p1", "admin"))
	var last claimtel.Outcome
	recordClaimCall = func(c claimtel.Call) { last = c.Outcome }
	router := NewRouter()
	return func(body []byte) (int, claimtel.Outcome) {
		last = ""
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/action/inject", bytes.NewReader(body)))
		return rr.Code, last
	}, restore
}

// FuzzActionV2Decode (assessment §4h). Invariants, for any body:
//   - it never panics (no 500, no outcome panic);
//   - the outcome is one of the enumerated classes;
//   - absent user_grants next to an unknown top-level list of objects (a
//     renamed grants list) is contract_fault, never no_grants; absent with
//     only Zitadel's known keys is no_grants;
//   - and, for the second input, a value of any type in a field Syndra does
//     not need never costs the token its claims (no 400, served).
func FuzzActionV2Decode(f *testing.F) {
	for _, seed := range []string{
		canonicalV2Payload,
		`{"function":"function/preaccesstoken","user":{"id":"u1"},"user_grants":[{"project_id":"p1","roles":["a"]}]}`,
		`{"function":"function/preuserinfo","user":{"id":"u1"}}`,
		`{"function":"function/preaccesstoken","user":{"id":"u1"},"grants_v5":[{"project_id":"p1"}]}`,
		`{"user":{"id":"u1"},"user_grants":[{"projectId":"p1"}],"application":[]}`,
		`{"user":{"id":" "},"user_grants":null}`,
		`{"user":"u1"}`, `[]`, `null`, `{}`, `not json`, `{"a":1} {"b":2}`,
	} {
		f.Add([]byte(seed), []byte(`{"nested":[1,"x",null]}`))
	}
	f.Add([]byte(`{}`), []byte(`"a string"`))
	f.Add([]byte(`{}`), []byte(`[[{"x":1}]]`))
	f.Add([]byte(`{}`), []byte(`12`))

	f.Setenv("ZITADEL_DOMAIN", "") // dev mode: no signature on the fuzzed bodies
	f.Setenv("ZITADEL_ACTION_SIGNING_KEY", "")
	serve, restore := fuzzRoute(f)
	defer restore()
	log.SetOutput(io.Discard) // a degraded path logs; a million lines is noise
	defer log.SetOutput(os.Stderr)

	f.Fuzz(func(t *testing.T, raw, filler []byte) {
		code, got := serve(raw)
		if code == http.StatusInternalServerError || got == claimtel.Panic {
			t.Fatalf("panicked on %q", raw)
		}
		if !claimtel.Valid(got) {
			t.Fatalf("outcome %q is not a class (body %q)", got, raw)
		}
		if want, ok := expectedGrantsOutcome(raw); ok && got != want {
			t.Fatalf("outcome %s, want %s for %q", got, want, raw)
		}

		if !json.Valid(filler) {
			return
		}
		fill := string(filler)
		body := `{"function":"function/preaccesstoken","user":{"id":"u1","human":` + fill + `,"x":` + fill + `},` +
			`"userinfo":` + fill + `,"org":` + fill + `,"user_metadata":` + fill + `,"actor":` + fill + `,` +
			`"application":{"client_id":"c1","extra":` + fill + `},` +
			`"user_grants":[{"project_id":"p1","roles":` + fill + `,"state":` + fill + `}]}`
		if code, got := serve([]byte(body)); code != http.StatusOK || got != claimtel.Served {
			t.Fatalf("an unused field holding %s gave HTTP %d / %s", fill, code, got)
		}
	})
}

// expectedGrantsOutcome is the user_grants rule restated independently of
// decodeActionV2: ok is false when the body does not reach it.
func expectedGrantsOutcome(raw []byte) (claimtel.Outcome, bool) {
	var top map[string]json.RawMessage
	if json.Unmarshal(raw, &top) != nil || top == nil {
		return "", false
	}
	var user struct {
		ID json.RawMessage `json:"id"`
	}
	_ = json.Unmarshal(top["user"], &user)
	var id string
	if json.Unmarshal(user.ID, &id) != nil || strings.TrimSpace(id) == "" {
		return claimtel.ContractFault, true
	}
	if _, present := top["user_grants"]; present {
		return "", false
	}
	for k, v := range top {
		if knownTopLevel[k] {
			continue
		}
		var rows []any
		if json.Unmarshal(v, &rows) != nil || len(rows) == 0 {
			continue
		}
		objects := true
		for _, r := range rows {
			if _, isObj := r.(map[string]any); !isObj && r != nil {
				objects = false
			}
		}
		if objects {
			return claimtel.ContractFault, true
		}
	}
	return claimtel.NoGrants, true
}

// FuzzZitadelSignature (assessment §4h): header permutations, whitespace,
// duplicate t=, many v1=, non-hex. It never panics, and it never accepts a
// request without a v1 that is the HMAC of the body under the key, for a
// timestamp the header carries.
func FuzzZitadelSignature(f *testing.F) {
	const secret = "fuzz-secret-fuzz-secret-fuzz-sec"
	now := time.Unix(1_800_000_000, 0)
	body := []byte(`{"user":{"id":"u1"}}`)
	good := hex.EncodeToString(computeZitadelActionSignature(now, body, secret))
	ts := strconv.FormatInt(now.Unix(), 10)
	for _, h := range []string{
		"t=" + ts + ",v1=" + good,
		" t = " + ts + " , v1 = " + good + " ",
		"t=1,t=" + ts + ",v1=" + good,
		"t=" + ts + ",t=1,v1=" + good,
		"t=" + ts + ",v1=zz,v1=" + good,
		"t=" + ts + ",v1=" + good[:10],
		"v1=" + good, "t=" + ts, "", ",,,", "t=" + ts + ",v1=",
		"t=" + ts + strings.Repeat(",v1=00", 50),
	} {
		f.Add(h, body)
	}
	f.Fuzz(func(t *testing.T, header string, b []byte) {
		if verifyZitadelActionSignature(b, header, secret, zitadelSignatureTolerance, now) != nil {
			return
		}
		var stamps []time.Time
		var sigs [][]byte
		for _, pair := range strings.Split(header, ",") {
			k, v, _ := strings.Cut(pair, "=")
			switch strings.TrimSpace(k) {
			case "t":
				if n, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64); err == nil {
					stamps = append(stamps, time.Unix(n, 0))
				}
			case "v1":
				if s, err := hex.DecodeString(strings.TrimSpace(v)); err == nil {
					sigs = append(sigs, s)
				}
			}
		}
		for _, at := range stamps {
			want := computeZitadelActionSignature(at, b, secret)
			for _, s := range sigs {
				if hmac.Equal(want, s) {
					return
				}
			}
		}
		t.Fatalf("accepted %q without a matching HMAC", header)
	})
}
