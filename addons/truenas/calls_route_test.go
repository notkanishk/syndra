package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// GET /calls/{call_id} (feature calls) answers from the three places that
// know: the call store, the lease, the mutation log. "unknown" (safe to try
// again) only when the log shows nothing that acted or may have.

func getCall(t *testing.T, s *server, id string) (int, map[string]json.RawMessage) {
	t.Helper()
	rr := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/calls/"+id, nil)
	r.SetPathValue("call_id", id)
	s.handleCall(rr, r, nil)
	var out map[string]json.RawMessage
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatalf("%d %s", rr.Code, rr.Body)
	}
	if rr.Code == http.StatusOK {
		if err := v2Validate(t, "call.json", json.RawMessage(rr.Body.Bytes())); err != nil {
			t.Errorf("the answer is not schema/call.json: %v\n%s", err, rr.Body)
		}
	}
	return rr.Code, out
}

func stateOf(out map[string]json.RawMessage) string {
	var s string
	_ = json.Unmarshal(out["state"], &s)
	return s
}

func TestCallsAnswersFromTheStoreTheLeaseAndTheLog(t *testing.T) {
	s, _ := opServer(t)

	// Recorded: the reply, byte for byte what the call answered.
	rr := postOperation(t, s, "account.release", `{"call_id":"c-recorded","operation":"account.release","subject":"sub-1","actor":"op-1"}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("release %d %s", rr.Code, rr.Body)
	}
	code, out := getCall(t, s, "c-recorded")
	var first, replayed any
	_ = json.Unmarshal(rr.Body.Bytes(), &first)
	_ = json.Unmarshal(out["reply"], &replayed)
	if code != 200 || stateOf(out) != "recorded" || string(mustJSON(t, first)) != string(mustJSON(t, replayed)) {
		t.Fatalf("%d %s: want recorded with the first reply %s", code, out["reply"], rr.Body)
	}

	// Never seen: unknown, with no reply.
	if code, out := getCall(t, s, "c-never"); code != 200 || stateOf(out) != "unknown" || out["reply"] != nil {
		t.Errorf("never seen: %d %v", code, out)
	}

	// Executing now (the lease): in_doubt.
	release, _, _ := s.calls.take("c-leased", "x")
	if _, out := getCall(t, s, "c-leased"); stateOf(out) != "in_doubt" {
		t.Errorf("leased: %v", out)
	}
	release()

	log := func(r Record) Record {
		t.Helper()
		got, err := s.log.Append(r)
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	// An intent with no result (the add-on stopped between the two): in_doubt.
	log(Record{Operation: "account.adopt", Subject: "sub-2", Actor: "op-1", CallID: "c-open", Outcome: outcomeIntent, Write: "store.bind"})
	if _, out := getCall(t, s, "c-open"); stateOf(out) != "in_doubt" {
		t.Errorf("an open intent: %v", out)
	}
	// A write that acted and whose reply was never recorded: in_doubt.
	in := log(Record{Operation: "account.adopt", Subject: "sub-2", Actor: "op-1", CallID: "c-acted", Outcome: outcomeIntent, Write: "store.bind"})
	log(Record{Operation: "account.adopt", Subject: "sub-2", Actor: "op-1", CallID: "c-acted", Outcome: outcomeSucceeded, Write: "store.bind", Intent: in.Seq})
	if _, out := getCall(t, s, "c-acted"); stateOf(out) != "in_doubt" {
		t.Errorf("acted, reply not recorded: %v", out)
	}
	// Settled unknown at a restart (Recover): in_doubt.
	in = log(Record{Operation: "account.adopt", Subject: "sub-2", Actor: "op-1", CallID: "c-recovered", Outcome: outcomeIntent, Write: "store.bind"})
	log(Record{Operation: "account.adopt", Subject: "sub-2", Actor: "op-1", CallID: "c-recovered", Outcome: outcomeUnknown, Write: "store.bind", Intent: in.Seq})
	if _, out := getCall(t, s, "c-recovered"); stateOf(out) != "in_doubt" {
		t.Errorf("recovered unknown: %v", out)
	}
	// Writes that changed nothing (refused, never sent): unknown, safe to try again.
	in = log(Record{Operation: "password.rotate", Subject: "sub-1", Actor: "op-1", CallID: "c-failed", Outcome: outcomeIntent, Write: "user.update"})
	log(Record{Operation: "password.rotate", Subject: "sub-1", Actor: "op-1", CallID: "c-failed", Outcome: outcomeFailed, Write: "user.update", Intent: in.Seq})
	in = log(Record{Operation: "password.rotate", Subject: "sub-1", Actor: "op-1", CallID: "c-failed", Outcome: outcomeIntent, Write: "user.update"})
	log(Record{Operation: "password.rotate", Subject: "sub-1", Actor: "op-1", CallID: "c-failed", Outcome: outcomeNotSent, Write: "user.update", Intent: in.Seq})
	if _, out := getCall(t, s, "c-failed"); stateOf(out) != "unknown" {
		t.Errorf("nothing changed: %v", out)
	}
}

// Asking changes nothing and is never gated: answered in read_only too, and
// the log's head stays where it was.
func TestCallsIsAReadInEveryLifecycle(t *testing.T) {
	s, _ := opServer(t)
	if err := s.life.Set("read_only", "maintenance"); err != nil {
		t.Skipf("cannot set the lifecycle here: %v", err)
	}
	head, count := s.log.Head()
	if code, out := getCall(t, s, "c-never"); code != 200 || stateOf(out) != "unknown" {
		t.Errorf("read_only: %d %v", code, out)
	}
	if h, c := s.log.Head(); h != head || c != count {
		t.Error("asking wrote to the mutation log")
	}
}

// This add-on has no physical operation, so a preview is refused before
// anything runs, never executed as the write it names.
func TestAPreviewIsRefusedHere(t *testing.T) {
	s, m := opServer(t)
	rr := postOperation(t, s, "password.rotate", `{"call_id":"c-preview","operation":"password.rotate","subject":"sub-1","preview":true}`)
	if rr.Code != http.StatusBadRequest || problemCode(t, rr) != CodeBadRequest {
		t.Fatalf("%d %s", rr.Code, rr.Body)
	}
	if len(m.updates) != 0 {
		t.Errorf("a preview reached the NAS: %v", m.updates)
	}
	if _, count := s.log.Head(); count != 0 {
		t.Errorf("a preview wrote %d log records", count)
	}
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
