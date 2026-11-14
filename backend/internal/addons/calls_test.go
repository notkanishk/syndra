package addons

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"syndra/internal/db"
)

// Slice 6c: physical operations, GET /calls and the no_change outcome, as the
// backend sends and reads them.

// inventoryAddon stands up an add-on whose manifest is the inventory fixture
// (features operations and calls; item.checkout declared), with
// features overriding the fixture's when given, answering every request with
// h. It returns how many requests arrived.
func inventoryAddon(t *testing.T, h http.HandlerFunc, features ...string) *atomic.Int64 {
	t.Helper()
	var m Manifest
	if err := json.Unmarshal(contractFixture(t, "manifest_inventory.json"), &m); err != nil {
		t.Fatal(err)
	}
	if features != nil {
		m.Features = features
	}
	var hits atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		h(w, r)
	}))
	t.Cleanup(srv.Close)
	installAddon(t, Registration{Target: "inventory", BaseURL: srv.URL, Secret: []byte("a-test-secret")}, m)
	return &hits
}

func reply(status int, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}
}

func checkout(id string) CallRequest {
	return CallRequest{
		Target: "inventory", Operation: "item.checkout",
		Record:  OperationRecord(id, "inventory", "item.checkout", "289471021834760193"),
		Subject: "289471021834760193", Actor: "289471021834760193",
		Params: map[string]any{"item_id": "item-2207"},
	}
}

// constants.in_doubt_bound is the backend's InDoubtBound and
// constants.call_memory its CallMemory: the registry is the one statement of
// how long a physical call is the add-on's to settle, and how long its
// unknown can be believed.
func TestInDoubtBoundIsTheContracts(t *testing.T) {
	var reg struct {
		Constants map[string]struct {
			Value any `json:"value"`
		} `json:"constants"`
	}
	if err := json.Unmarshal(mustRead(t, "registry", "constants.json"), &reg); err != nil {
		t.Fatal(err)
	}
	if got := reg.Constants["in_doubt_bound"].Value; isoCooldown(got.(string)) != InDoubtBound {
		t.Errorf("constants.in_doubt_bound is %q and InDoubtBound %v; one of them moved", got, InDoubtBound)
	}
	if got := reg.Constants["call_memory"].Value; isoCooldown(got.(string)) != CallMemory {
		t.Errorf("constants.call_memory is %q and CallMemory %v; one of them moved", got, CallMemory)
	}
	if h := PhysicalHold(); h.For != InDoubtBound || strings.Join(h.Operations, ",") != "item.checkout" {
		t.Errorf("PhysicalHold is %+v", h)
	}
}

// 6.3a: Syndra reads the reply's own outcome. succeeded is done; no_change is
// done-nothing only from an add-on declaring calls (the feature it is listed
// under); a missing or unknown outcome on a 2xx is indeterminate, because
// absence is never a value and an answer Syndra cannot read may have acted.
func TestTheWireOutcomeIsReadNotTheStatus(t *testing.T) {
	for _, tc := range []struct {
		name     string
		body     string
		features []string
		want     Outcome
		noChange bool
		err      error // for an indeterminate one: missing or unknown, or unreadable
	}{
		{"succeeded", `{"operation":"item.checkout","subject":"s","outcome":"succeeded"}`, nil, OutcomeSucceeded, false, nil},
		{"no_change from a calls add-on", string(contractFixture(t, "operation_response_no_change.json")), nil, OutcomeSucceeded, true, nil},
		{"no_change from an add-on without calls", `{"outcome":"no_change"}`, []string{"operations"}, OutcomeIndeterminate, false, ErrUnknownOutcome},
		{"no outcome", `{"operation":"item.checkout","subject":"s"}`, nil, OutcomeIndeterminate, false, ErrUnknownOutcome},
		{"an outcome outside the vocabulary", `{"outcome":"conflict"}`, nil, OutcomeIndeterminate, false, ErrUnknownOutcome},
		{"Outcome is not outcome", `{"Outcome":"succeeded"}`, nil, OutcomeIndeterminate, false, ErrUnknownOutcome},
		{"not JSON", `ok`, nil, OutcomeIndeterminate, false, ErrUnreadableReply},
		{"the outcome twice", `{"outcome":"no_change","outcome":"succeeded"}`, nil, OutcomeIndeterminate, false, ErrUnreadableReply},
	} {
		t.Run(tc.name, func(t *testing.T) {
			op := "item.checkout"
			if tc.features != nil {
				op = "password.set" // item.checkout is unavailable without calls
			}
			inventoryAddon(t, reply(200, tc.body), tc.features...)
			req := checkout("rec-" + tc.name)
			if tc.features != nil {
				a, _ := Get("inventory")
				a.ops, _ = resolveOperations(Manifest{Features: tc.features, Operations: []Operation{{ID: op, Scope: ScopeMember, Available: true}}})
				req.Operation, req.Params = op, map[string]any{"password": "x"}
				req.Record = OperationRecord("rec-"+tc.name, "inventory", op, req.Subject)
			}
			resp := Call(context.Background(), req)
			if resp.Outcome != tc.want || resp.NoChange != tc.noChange {
				t.Fatalf("outcome %s no_change %t (err %v), want %s %t", resp.Outcome, resp.NoChange, resp.Err, tc.want, tc.noChange)
			}
			if tc.err != nil && !errors.Is(resp.Err, tc.err) {
				t.Errorf("err %v, want %v", resp.Err, tc.err)
			}
			want := map[bool]string{true: db.AddonOpNoChange, false: StatusFor(tc.want)}[tc.noChange]
			if got := StatusOf(resp); got != want {
				t.Errorf("recorded as %q, want %q", got, want)
			}
		})
	}
}

// The registered refusals a physical operation meets are the add-on checking
// the request and doing nothing (rejected, never a shape refusal: a 404
// UNKNOWN_RESOURCE is about the item, not the route), and what each names
// rides through for the page's words, never a value.
func TestThePhysicalRefusalsAreRejectedWithWhatTheyName(t *testing.T) {
	for fixture, want := range map[string]struct {
		code    string
		refusal Refusal
	}{
		"error_unknown_resource.json": {"UNKNOWN_RESOURCE", Refusal{Param: "item_id"}},
		"error_not_entitled.json":     {"NOT_ENTITLED", Refusal{Param: "item_id"}},
		"error_resource_taken.json":   {"RESOURCE_TAKEN", Refusal{Param: "item_id"}},
		"error_limit_reached.json":    {"LIMIT_REACHED", Refusal{Limit: ptr(2), Unit: "items", Count: ptr(2)}},
		"error_call_unsettled.json":   {"CALL_UNSETTLED", Refusal{}},
	} {
		t.Run(fixture, func(t *testing.T) {
			body := contractFixture(t, fixture)
			var p struct {
				Status int `json:"status"`
			}
			_ = json.Unmarshal(body, &p)
			hits := inventoryAddon(t, reply(p.Status, string(body)))
			resp := Call(context.Background(), checkout("rec-"+fixture))
			if resp.Outcome != OutcomeRejected || resp.ShapeRefusal || resp.Code != want.code {
				t.Fatalf("%d %s: outcome %s shape %t code %q", p.Status, fixture, resp.Outcome, resp.ShapeRefusal, resp.Code)
			}
			got, _ := json.Marshal(resp.Refusal)
			w, _ := json.Marshal(want.refusal)
			if string(got) != string(w) {
				t.Errorf("refusal %s, want %s", got, w)
			}
			if hits.Load() != 1 {
				t.Errorf("%d requests", hits.Load())
			}
		})
	}
}

func ptr(n int) *int { return &n }

// Physical rule 2: an indeterminate physical call is executed again by no
// one, the transport included. Whatever the answer (a 5xx that may have
// acted, a 429 that did not, a reset), one request reaches the add-on.
func TestAnIndeterminatePhysicalCallIsSentOnce(t *testing.T) {
	for _, tc := range []struct {
		name string
		h    http.HandlerFunc
	}{
		{"503", reply(503, `{"code":"OPERATION_FAILED"}`)},
		{"500", reply(500, ``)},
		{"429", reply(429, `{"code":"OPERATION_FAILED"}`)},
		{"reset", func(w http.ResponseWriter, r *http.Request) {
			conn, _, _ := w.(http.Hijacker).Hijack()
			_ = conn.Close()
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			saved := addonBackoff
			addonBackoff = time.Millisecond
			t.Cleanup(func() { addonBackoff = saved })
			hits := inventoryAddon(t, tc.h)
			_ = Call(context.Background(), checkout("rec-once-"+tc.name))
			if n := hits.Load(); n != 1 {
				t.Fatalf("%d requests reached the add-on; a physical call is sent once", n)
			}
		})
	}
}

// Physical rule 1: the preview carries preview: true and no call_id, takes no
// claim and writes no record, and its reply reads through the read path's
// decoder (the would-be record, and no_change when it would change nothing).
func TestAPreviewCarriesNoCallIDAndLeavesNoRow(t *testing.T) {
	var body []byte
	inventoryAddon(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ = io.ReadAll(r.Body)
		reply(200, string(contractFixture(t, "operation_response_no_change.json")))(w, r)
	})
	saved, savedClaim := dbBeginAddonOperation, dbClaimAddonOperation
	t.Cleanup(func() { dbBeginAddonOperation, dbClaimAddonOperation = saved, savedClaim })
	dbBeginAddonOperation = func(context.Context, db.AddonOperationParams) (string, error) {
		t.Error("a preview began a record")
		return "", errors.New("no")
	}
	dbClaimAddonOperation = func(context.Context, string, string, string, string) (db.AddonOperation, error) {
		t.Error("a preview took a claim")
		return db.AddonOperation{}, errors.New("no")
	}

	r := Preview(context.Background(), "inventory", "item.checkout", "289471021834760193", "289471021834760193",
		map[string]any{"item_id": "item-2207"})
	if r.Outcome != OutcomeSucceeded || !r.NoChange {
		t.Fatalf("outcome %s no_change %t err %v", r.Outcome, r.NoChange, r.Err)
	}
	sameJSON(t, body, contractFixture(t, "operation_request_preview.json"), "the preview request")
	if strings.Contains(string(body), "call_id") {
		t.Errorf("the preview carried a call_id: %s", body)
	}
	v := r.View()
	if !v.Readable || v.Shape != "records" {
		t.Errorf("the preview's record is not readable through the read path: %+v", v)
	}
}

// Without calls the operation is unavailable, so a preview is never sent (the
// request field would reach an add-on that does not know it).
func TestAPreviewIsNeverSentToAnAddonWithoutCalls(t *testing.T) {
	hits := inventoryAddon(t, reply(200, `{}`), "operations")
	r := Preview(context.Background(), "inventory", "item.checkout", "s", "s", map[string]any{"item_id": "item-2207"})
	var unavailable *ErrOperationUnavailable
	if !errors.As(r.Err, &unavailable) || hits.Load() != 0 {
		t.Fatalf("err %v, %d requests", r.Err, hits.Load())
	}
	// And a read is never previewed.
	inventoryAddon(t, reply(200, `{}`))
	if r := Preview(context.Background(), "inventory", "holdings.list", "s", "s", nil); !errors.Is(r.Err, ErrNotPhysical) {
		t.Errorf("a read previewed: %v", r.Err)
	}
}

// GET /calls: the three answers, read strictly. A reply only with recorded,
// about this call only; a secret reply is dropped unread; an add-on without
// calls is never asked.
func TestCallAnswerReadsTheThreeAnswersStrictly(t *testing.T) {
	const id = "0b7d4c1e-6a5f-4e2b-9c3d-8f1a2e4b6c70"
	recorded := `{"call_id":"` + id + `","state":"recorded","reply":{"operation":"item.checkout","outcome":"succeeded"}}`
	for _, tc := range []struct {
		name, body, state string
		reply, fails      bool
	}{
		{"recorded", recorded, CallRecorded, true, false},
		{"in_doubt", `{"call_id":"` + id + `","state":"in_doubt"}`, CallInDoubt, false, false},
		{"unknown", `{"call_id":"` + id + `","state":"unknown"}`, CallUnknown, false, false},
		{"in_doubt with a reply", `{"call_id":"` + id + `","state":"in_doubt","reply":{}}`, "", false, true},
		{"unknown with a reply", `{"call_id":"` + id + `","state":"unknown","reply":{}}`, "", false, true},
		{"another call", `{"call_id":"other","state":"unknown"}`, "", false, true},
		{"no call_id", `{"state":"unknown"}`, "", false, true},
		{"a state outside the three", `{"call_id":"` + id + `","state":"done"}`, "", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var path string
			inventoryAddon(t, func(w http.ResponseWriter, r *http.Request) {
				path = r.Method + " " + r.URL.EscapedPath()
				reply(200, tc.body)(w, r)
			})
			s, err := CallAnswer(context.Background(), "inventory", "item.checkout", id)
			if path != "GET /calls/"+id {
				t.Errorf("asked %q", path)
			}
			if tc.fails {
				if !errors.Is(err, ErrNotACallAnswer) {
					t.Fatalf("read %+v %v, want ErrNotACallAnswer", s, err)
				}
				return
			}
			if err != nil || s.State != tc.state || (len(s.Reply) > 0) != tc.reply {
				t.Fatalf("%+v %v", s, err)
			}
		})
	}

	t.Run("a secret reply is dropped", func(t *testing.T) {
		p := operationPolicy["item.checkout"]
		p.SecretResult = true
		operationPolicy["item.checkout"] = p
		t.Cleanup(func() { p.SecretResult = false; operationPolicy["item.checkout"] = p })
		inventoryAddon(t, reply(200, recorded))
		s, err := CallAnswer(context.Background(), "inventory", "item.checkout", id)
		if err != nil || s.State != CallRecorded || s.Reply != nil {
			t.Fatalf("%+v %v: a secret reply was kept", s, err)
		}
		if v := ViewOf("inventory", "item.checkout", []byte(`{"result":{"records":[],"taken_at":"2026-11-10T09:30:00Z","current":true}}`), Clock{}); v.Readable {
			t.Errorf("a secret reply was shown: %+v", v)
		}
	})

	t.Run("an add-on without calls is never asked", func(t *testing.T) {
		hits := inventoryAddon(t, reply(200, recorded), "operations")
		if _, err := CallAnswer(context.Background(), "inventory", "item.checkout", id); !errors.Is(err, ErrFeatureNotDeclared) || hits.Load() != 0 {
			t.Fatalf("err %v, %d requests", err, hits.Load())
		}
	})
}
