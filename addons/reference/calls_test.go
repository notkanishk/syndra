package reference

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// The physical operation's rules a black-box kit cannot prove: a reply lost
// mid-call, two checkouts racing, the defects that break each. Every request
// goes through deviate, so a defect acts as it would on the wire.

// lender is the reference serving calls, with two members who may
// use mk4-a (so items 0001 and 0002) and a lost-reply hook.
func lender(t *testing.T, down func() bool, lose func(string) bool, defects ...Defect) *Addon {
	t.Helper()
	set := map[Defect]bool{}
	for _, d := range defects {
		set[d] = true
	}
	a, err := New(Config{Target: "reference", Secret: []byte("s"), Features: []string{"calls"},
		Defects: set, Down: down, LoseReply: lose})
	if err != nil {
		t.Fatal(err)
	}
	for i, subj := range []string{"rey-mysterio", "kofi-kingston"} {
		id := fmt.Sprint(1001 + i)
		a.accounts[id] = &account{id: id, username: subj, state: map[string]any{
			"enabled": true, "remote": true, "printers": []string{lobby, "mk4-a"}, "after_hours": []string{}, "job_seconds": []string{"0"}}}
		a.bindings[subj] = id
	}
	return a
}

type answer struct {
	status int
	body   []byte
	doc    map[string]any
}

func (r answer) code() string    { s, _ := r.doc["code"].(string); return s }
func (r answer) outcome() string { s, _ := r.doc["outcome"].(string); return s }

// checkoutAs sends item.checkout for subject: a preview when callID is "".
func checkoutAs(a *Addon, subject, callID, itemID string) answer {
	body := map[string]any{"contract_version": 2, "operation": "item.checkout", "subject": subject, "actor": subject,
		"params": map[string]any{"item_id": itemID}}
	if callID == "" {
		body["preview"] = true
	} else {
		body["call_id"] = callID
	}
	raw, _ := json.Marshal(body)
	return send(a, "POST /operations/{name}", "item.checkout", raw)
}

func send(a *Addon, route, name string, raw []byte) answer {
	r := httptest.NewRequest(http.MethodPost, "/operations/"+name, nil)
	r.SetPathValue("name", name)
	h := a.operation
	if route == "GET /calls/{call_id}" {
		r = httptest.NewRequest(http.MethodGet, "/calls/"+name, nil)
		r.SetPathValue("call_id", name)
		h = a.callState
	}
	rep := a.deviate(route, h, r, raw)
	var doc map[string]any
	_ = json.Unmarshal(rep.body, &doc)
	return answer{rep.status, rep.body, doc}
}

func callOf(a *Addon, callID string) answer { return send(a, "GET /calls/{call_id}", callID, nil) }

func (a *Addon) logged() int              { a.mu.Lock(); defer a.mu.Unlock(); return a.logCount }
func (a *Addon) lent() int                { a.mu.Lock(); defer a.mu.Unlock(); return len(a.loans) }
func (r answer) state() string            { s, _ := r.doc["state"].(string); return s }
func checkedOut(r answer) bool            { return r.status == 200 && r.outcome() == "succeeded" }
func refusedWith(r answer, c string) bool { return r.status >= 400 && r.code() == c }

// Rule 4: an item the subject already holds is no_change, with the record, and
// writes nothing; the replay of the first is the first, byte for byte.
// no-change-as-succeeded answers succeeded and logs, which this catches.
func TestASecondIdenticalCheckoutIsNoChange(t *testing.T) {
	for _, d := range []Defect{"", NoChangeAsSucceeded} {
		t.Run(string(d), func(t *testing.T) {
			a := lender(t, nil, nil, d)
			first := checkoutAs(a, "rey-mysterio", "c-1", "item-0001")
			if !checkedOut(first) || a.logged() != 1 {
				t.Fatalf("the first checkout answered %d %s, log %d", first.status, first.body, a.logged())
			}
			again := checkoutAs(a, "rey-mysterio", "c-2", "item-0001")
			clean := again.status == 200 && again.outcome() == "no_change" && a.logged() == 1 && a.lent() == 1
			if clean != (d == "") {
				t.Errorf("with defect %q the second checkout answered %d %s, log %d", d, again.status, again.body, a.logged())
			}
			if replay := checkoutAs(a, "rey-mysterio", "c-1", "item-0001"); !bytes.Equal(replay.body, first.body) {
				t.Errorf("the replay of c-1 answered %s, not the first reply %s", replay.body, first.body)
			}
		})
	}
}

// Rule 5: two checkouts by one member, of two items, at once, with a limit of
// one: exactly one loan, the other LIMIT_REACHED, every time. Unserialised,
// both pass the limit at least once in the run.
func TestAConcurrentPairPassesTheLimitOnce(t *testing.T) {
	for _, d := range []Defect{"", CheckoutUnserialised} {
		t.Run(string(d), func(t *testing.T) {
			doubled := 0
			for i := range 200 {
				// The app call takes a moment, as a real one does: the
				// window in which the first checkout's intent is open.
				a := lender(t, nil, func(string) bool { time.Sleep(200 * time.Microsecond); return false }, d)
				var wg sync.WaitGroup
				got := make([]answer, 2)
				for j, it := range []string{"item-0001", "item-0002"} {
					wg.Go(func() { got[j] = checkoutAs(a, "rey-mysterio", fmt.Sprintf("c-%d-%d", i, j), it) })
				}
				wg.Wait()
				if a.lent() > 1 {
					doubled++
					continue
				}
				ok, limited := 0, 0
				for _, g := range got {
					switch {
					case checkedOut(g):
						ok++
					case refusedWith(g, "LIMIT_REACHED") && g.doc["count"] == float64(1) && g.doc["unit"] == "items":
						limited++
					}
				}
				if ok != 1 || limited != 1 {
					t.Fatalf("iteration %d answered %d %s and %d %s", i, got[0].status, got[0].body, got[1].status, got[1].body)
				}
			}
			if (doubled > 0) != (d == CheckoutUnserialised) {
				t.Errorf("with defect %q, %d of 200 pairs both passed the limit", d, doubled)
			}
		})
	}
}

// Rule 2: a reply lost before the loan leaves an intent with no result. Until
// it settles, nothing runs for that member, under its call_id or a new one;
// /calls says in_doubt while the target cannot be read, then unknown (no loan),
// after which a checkout runs. reexecutes-in-doubt runs the new one.
func TestACallInDoubtIsNeverExecutedAgain(t *testing.T) {
	for _, d := range []Defect{"", ReexecutesInDoubt} {
		t.Run(string(d), func(t *testing.T) {
			var down, armed atomic.Bool
			armed.Store(true)
			a := lender(t, down.Load, func(stage string) bool { return stage == "before" && armed.Swap(false) }, d)
			if lost := checkoutAs(a, "rey-mysterio", "c-1", "item-0001"); lost.status != 500 || a.lent() != 0 || a.logged() != 0 {
				t.Fatalf("the lost checkout answered %d, loans %d, log %d", lost.status, a.lent(), a.logged())
			}
			same := checkoutAs(a, "rey-mysterio", "c-1", "item-0001")
			fresh := checkoutAs(a, "rey-mysterio", "c-2", "item-0002")
			held := refusedWith(same, "CALL_UNSETTLED") && refusedWith(fresh, "CALL_UNSETTLED") && a.lent() == 0 && a.logged() == 0
			if held != (d == "") {
				t.Fatalf("with defect %q, retries answered %d %s and %d %s; loans %d, log %d",
					d, same.status, same.code(), fresh.status, fresh.code(), a.lent(), a.logged())
			}
			if d != "" {
				return
			}
			down.Store(true)
			if s := callOf(a, "c-1").state(); s != "in_doubt" {
				t.Errorf("/calls with the target unreadable answered %s, want in_doubt", s)
			}
			down.Store(false)
			if s := callOf(a, "c-1"); s.state() != "unknown" || s.doc["reply"] != nil {
				t.Errorf("/calls once settled answered %s, want unknown: no loan was made", s.body)
			}
			if next := checkoutAs(a, "rey-mysterio", "c-3", "item-0002"); !checkedOut(next) {
				t.Errorf("a checkout after the call settled unknown answered %d %s", next.status, next.body)
			}
		})
	}
}

// A reply lost after the loan: the call happened. Retries are still refused
// until /calls settles it, which answers recorded with the reply, and a
// replay of its call_id is that reply.
func TestACallLostAfterTheLoanSettlesRecorded(t *testing.T) {
	var armed atomic.Bool
	armed.Store(true)
	a := lender(t, nil, func(stage string) bool { return stage == "after" && armed.Swap(false) })
	if lost := checkoutAs(a, "rey-mysterio", "c-1", "item-0001"); lost.status != 500 || a.lent() != 1 || a.logged() != 1 {
		t.Fatalf("the lost checkout answered %d, loans %d, log %d", lost.status, a.lent(), a.logged())
	}
	if again := checkoutAs(a, "rey-mysterio", "c-2", "item-0001"); !refusedWith(again, "CALL_UNSETTLED") {
		t.Errorf("a retry before settling answered %d %s, want CALL_UNSETTLED", again.status, again.body)
	}
	got := callOf(a, "c-1")
	reply, _ := json.Marshal(got.doc["reply"])
	if got.state() != "recorded" || !bytes.Contains(reply, []byte(`"outcome":"succeeded"`)) || !bytes.Contains(reply, []byte("item-0001")) {
		t.Fatalf("/calls answered %s, want recorded with the checkout's reply", got.body)
	}
	if replay := checkoutAs(a, "rey-mysterio", "c-1", "item-0001"); replay.status != 200 || !sameJSON(replay.body, reply) {
		t.Errorf("the replay answered %d %s, not the recorded reply %s", replay.status, replay.body, reply)
	}
	if a.logged() != 1 || a.lent() != 1 {
		t.Errorf("settling wrote: log %d, loans %d", a.logged(), a.lent())
	}
}

// Rule 1: a preview answers the would-be record and leaves nothing: no log
// record, no loan, no call entry; /calls on a never-used id is unknown. A
// preview carries no call_id, and only an add-on declaring calls takes one.
// preview-recorded logs one, which this catches.
func TestAPreviewLeavesNothingBehind(t *testing.T) {
	for _, d := range []Defect{"", PreviewRecorded} {
		t.Run(string(d), func(t *testing.T) {
			a := lender(t, nil, nil, d)
			p := checkoutAs(a, "rey-mysterio", "", "item-0001")
			records, _ := p.doc["result"].(map[string]any)["records"].([]any)
			if p.status != 200 || p.outcome() != "succeeded" || len(records) != 1 || records[0].(map[string]any)["id"] != "item-0001" {
				t.Fatalf("the preview answered %d %s", p.status, p.body)
			}
			left := a.logged() == 0 && a.lent() == 0 && len(a.calls) == 0 && callOf(a, "never-used").state() == "unknown"
			if left != (d == "") {
				t.Errorf("with defect %q the preview left log %d, loans %d, calls %d", d, a.logged(), a.lent(), len(a.calls))
			}
		})
	}
	a := lender(t, nil, nil)
	raw := []byte(`{"contract_version":2,"call_id":"c-1","operation":"item.checkout","subject":"rey-mysterio","params":{"item_id":"item-0001"},"preview":true}`)
	if r := send(a, "POST /operations/{name}", "item.checkout", raw); !refusedWith(r, "BAD_REQUEST") || a.lent() != 0 {
		t.Errorf("a preview with a call_id answered %d %s", r.status, r.body)
	}
	plain, _ := New(Config{Target: "reference", Secret: []byte("s")})
	raw = []byte(`{"contract_version":2,"call_id":"c-1","operation":"password.set","subject":"s","params":{"password":"x"},"preview":true}`)
	if r := send(plain, "POST /operations/{name}", "password.set", raw); !refusedWith(r, "UNKNOWN_REQUEST_FIELD") {
		t.Errorf("preview to an add-on without calls answered %d %s, want UNKNOWN_REQUEST_FIELD", r.status, r.body)
	}
	if r := callOf(plain, "c-1"); !refusedWith(r, "UNKNOWN_ROUTE") {
		t.Errorf("/calls without the feature answered %d %s, want 404 UNKNOWN_ROUTE", r.status, r.body)
	}
}

// The confirm with its own fresh id executes, and /calls answers recorded
// with its reply, byte for byte; two confirms carry two ids and each is its
// own. (Sent under one id, the second is CALL_ID_REUSED and this fails.)
func TestTheConfirmWithItsFreshIDExecutes(t *testing.T) {
	a := lender(t, nil, nil)
	ids := []string{"c-rey", "c-kofi"}
	for i, who := range []string{"rey-mysterio", "kofi-kingston"} {
		it := fmt.Sprintf("item-000%d", i+1)
		got := checkoutAs(a, who, ids[i], it)
		if !checkedOut(got) {
			t.Fatalf("%s's confirm answered %d %s", who, got.status, got.body)
		}
		c := callOf(a, ids[i])
		raw := c.doc["reply"]
		reply, _ := json.Marshal(raw)
		if c.state() != "recorded" || !sameJSON(got.body, reply) || !bytes.Contains(c.body, got.body) {
			t.Errorf("/calls/%s answered %s, want recorded with %s byte for byte", ids[i], c.body, got.body)
		}
	}
	if a.lent() != 2 || a.logged() != 2 {
		t.Errorf("two confirms left loans %d, log %d", a.lent(), a.logged())
	}
}

// What a member may not do is refused before anything is written, each with
// the param it is about, and never queued.
func TestACheckoutIsRefusedBeforeAnyWrite(t *testing.T) {
	var down atomic.Bool
	a := lender(t, down.Load, nil)
	if !checkedOut(checkoutAs(a, "kofi-kingston", "c-0", "item-0002")) {
		t.Fatal("kofi's checkout did not run")
	}
	for _, tc := range []struct{ item, code string }{
		{"item-9999", "UNKNOWN_RESOURCE"}, {"item-0003", "NOT_ENTITLED"}, {"item-0002", "RESOURCE_TAKEN"},
	} {
		for _, id := range []string{"", "c-" + tc.item} {
			r := checkoutAs(a, "rey-mysterio", id, tc.item)
			if !refusedWith(r, tc.code) || r.doc["param"] != "item_id" {
				t.Errorf("%s (call_id %q) answered %d %s, want %s with param item_id", tc.item, id, r.status, r.body, tc.code)
			}
		}
	}
	down.Store(true)
	if r := checkoutAs(a, "rey-mysterio", "c-down", "item-0001"); !refusedWith(r, "TARGET_UNREACHABLE") || r.status != 503 {
		t.Errorf("a checkout with the ledger down answered %d %s", r.status, r.body)
	}
	down.Store(false)
	a.lifecycle = "read_only"
	if r := checkoutAs(a, "rey-mysterio", "", "item-0001"); !refusedWith(r, "LIFECYCLE_REFUSAL") {
		t.Errorf("a preview while read_only answered %d %s", r.status, r.body)
	}
	if a.lent() != 1 || a.logged() != 1 {
		t.Errorf("the refusals wrote: loans %d, log %d", a.lent(), a.logged())
	}
}

// /calls answers for every write, a password.set's included, and never with a
// parameter. calls-echo-params keeps them and answers them, which this catches.
func TestCallsAnswersWithoutTheParams(t *testing.T) {
	for _, d := range []Defect{"", CallsEchoParams} {
		t.Run(string(d), func(t *testing.T) {
			a := lender(t, nil, nil, d)
			raw := []byte(`{"contract_version":2,"call_id":"p-1","operation":"password.set","subject":"rey-mysterio","params":{"password":"canary-619"}}`)
			if r := send(a, "POST /operations/{name}", "password.set", raw); r.status != 200 {
				t.Fatalf("password.set answered %d %s", r.status, r.body)
			}
			c := callOf(a, "p-1")
			leaked := bytes.Contains(c.body, []byte("canary-619"))
			if c.state() != "recorded" || leaked != (d == CallsEchoParams) {
				t.Errorf("with defect %q /calls answered %s", d, c.body)
			}
		})
	}
}

func sameJSON(a, b []byte) bool {
	var x, y any
	if json.Unmarshal(a, &x) != nil || json.Unmarshal(b, &y) != nil {
		return false
	}
	p, _ := json.Marshal(x)
	q, _ := json.Marshal(y)
	return bytes.Equal(p, q)
}
