package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"syndra/internal/addons"
	"syndra/internal/auth"
	"syndra/internal/db"
)

// A-13 §4: passwords that settle themselves, on the server's side of the page.

// The page's resend names the first send by finding it, never by trusting the
// page: the page may never have learnt the first send's id.
func TestAResendRepeatsTheFirstSendItFinds(t *testing.T) {
	h := &storageHarness{entitled: true, bound: true, resendable: "op_first"}
	stubMyStorage(t, h)

	rr, _ := setCredential(t, `{"password":"`+memberSecret+`","resend":true}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", rr.Code, rr.Body.String())
	}
	if len(h.dispatched) != 1 || h.dispatched[0].RetryOf != "op_first" {
		t.Fatalf("the resend must be dispatched as a retry of the first send: %+v", h.dispatched)
	}
}

// A first send is never a retry, even when an earlier one could be resent: only
// the page's own automatic resend says so.
func TestAnOrdinaryChangeIsNeverARetry(t *testing.T) {
	h := &storageHarness{entitled: true, bound: true, resendable: "op_first"}
	stubMyStorage(t, h)

	setCredential(t, `{"password":"`+memberSecret+`"}`)
	if len(h.dispatched) != 1 || h.dispatched[0].RetryOf != "" {
		t.Fatalf("a member's own change is a new change, not a retry: %+v", h.dispatched)
	}
}

// A resend with nothing to repeat (the first never got as far as a record) is
// an ordinary first send: still sent, and still counted by the limiter.
func TestAResendWithNothingToRepeatIsAFirstSend(t *testing.T) {
	h := &storageHarness{entitled: true, bound: true}
	stubMyStorage(t, h)

	setCredential(t, `{"password":"`+memberSecret+`","resend":true}`)
	if len(h.dispatched) != 1 || h.dispatched[0].RetryOf != "" {
		t.Fatalf("with nothing to repeat the send is a first send: %+v", h.dispatched)
	}
}

// Only a confirmed change settles the earlier ones, and only it says "set".
func TestOnlyAConfirmedChangeSettlesTheEarlierOnes(t *testing.T) {
	for _, outcome := range []addons.Outcome{addons.OutcomeIndeterminate, addons.OutcomeRejected, addons.OutcomeUnreached} {
		h := &storageHarness{entitled: true, bound: true, outcome: outcome}
		stubMyStorage(t, h)
		rr, _ := setCredential(t, `{"password":"`+memberSecret+`"}`)
		var out map[string]any
		_ = json.Unmarshal(rr.Body.Bytes(), &out)
		if rr.Code != http.StatusAccepted || out["status"] != string(outcome) {
			t.Errorf("%s: want 202 with status %q, got %d %v", outcome, outcome, rr.Code, out)
		}
		if len(h.superseded) != 0 {
			t.Errorf("%s: an unconfirmed change must not settle anything", outcome)
		}
	}

	h := &storageHarness{entitled: true, bound: true}
	stubMyStorage(t, h)
	rr, _ := setCredential(t, `{"password":"`+memberSecret+`"}`)
	if rr.Code != http.StatusOK || len(h.superseded) != 1 || h.superseded[0] != "op_1" {
		t.Fatalf("a confirmed change settles the earlier ones: %d %v", rr.Code, h.superseded)
	}
}

// Sent, with its outcome unwritten: as unknown as a lost reply. The page must
// be told to treat it as one, never "Nothing was changed".
func TestAnUnrecordedOutcomeIsALostReplyNotARefusal(t *testing.T) {
	h := &storageHarness{entitled: true, bound: true, dispatchErr: errors.New("outcome not recorded")}
	stubMyStorage(t, h)

	rr, _ := setCredential(t, `{"password":"`+memberSecret+`"}`)
	var out map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &out)
	if rr.Code != http.StatusAccepted || out["status"] != "indeterminate" {
		t.Fatalf("want 202 indeterminate, got %d %v", rr.Code, out)
	}
	if strings.Contains(rr.Body.String(), "Nothing was changed") {
		t.Fatal("a send that may have applied must never read as nothing changed")
	}
}

// The member's line: present while the latest change is open, and gone after.
func TestTheStorageReadCarriesAnUnconfirmedChange(t *testing.T) {
	at := time.Date(2026, 11, 3, 9, 41, 2, 0, time.UTC)
	h := &storageHarness{entitled: true, bound: true, unconfirmed: at}
	stubMyStorage(t, h)

	view := readMyTargets(t)
	if view.PasswordChange == nil || view.PasswordChange.State != "unconfirmed" || !view.PasswordChange.At.Equal(at) {
		t.Fatalf("want an unconfirmed change at %v, got %+v", at, view.PasswordChange)
	}

	h.unconfirmed = time.Time{}
	if view := readMyTargets(t); view.PasswordChange != nil {
		t.Fatalf("a settled change leaves no line: %+v", view.PasswordChange)
	}
}

func readMyTargets(t *testing.T) myTargetView {
	t.Helper()
	saved, manifest := addonsRegistered, addonsAcceptedShape
	t.Cleanup(func() { addonsRegistered, addonsAcceptedShape = saved, manifest })
	addonsRegistered = func() []addons.Registration { return []addons.Registration{{Target: "truenas"}} }
	// Read, so a member meets it (memberMeets).
	addonsAcceptedShape = func(context.Context, string) (addons.Shape, error) {
		return addons.Shape{Product: "TrueNAS", Kind: "storage", EntitlementSchema: truenasSchema}, nil
	}
	r := httptest.NewRequest(http.MethodGet, "/api/v1/me/targets", nil)
	r = r.WithContext(withPrincipal(r.Context(), &auth.Principal{Subject: "u1"}))
	rr := httptest.NewRecorder()
	handleMyTargets(rr, r)
	var out struct {
		Targets []myTargetView `json:"targets"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil || len(out.Targets) != 1 {
		t.Fatalf("read /me/targets: %v (%s)", err, rr.Body.String())
	}
	return out.Targets[0]
}

// A-09 §4: the operator's answer.

const answeredID = "0b0b0b0b-0000-4000-8000-000000000009"

type answerHarness struct {
	resolved []string
	happened []bool
	by       []string
	err      error
}

func stubAnswer(t *testing.T, h *answerHarness) {
	t.Helper()
	saved := dbResolveAddonOperation
	t.Cleanup(func() { dbResolveAddonOperation = saved })
	dbResolveAddonOperation = func(_ context.Context, target, id string, happened bool, by string, _ time.Duration) (db.AddonOperation, error) {
		if h.err != nil {
			return db.AddonOperation{}, h.err
		}
		h.resolved = append(h.resolved, target+"/"+id)
		h.happened = append(h.happened, happened)
		h.by = append(h.by, by)
		return db.AddonOperation{ID: id, Target: target}, nil
	}
}

func answer(t *testing.T, actor, body string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(http.MethodPost, "/api/v1/targets/truenas/unconfirmed/0b0b0b0b-0000-4000-8000-000000000009/answer", strings.NewReader(body))
	r.SetPathValue("target", "truenas")
	r.SetPathValue("id", answeredID)
	if actor != "" {
		r = r.WithContext(withPrincipal(r.Context(), &auth.Principal{Subject: actor}))
	}
	rr := httptest.NewRecorder()
	handleSettleUnconfirmed(rr, r)
	return rr
}

func TestAnAnswerIsRecordedWithTheOperatorsName(t *testing.T) {
	h := &answerHarness{}
	stubAnswer(t, h)

	rr := answer(t, "op-kanishk", `{"happened":false}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", rr.Code, rr.Body.String())
	}
	if len(h.resolved) != 1 || h.resolved[0] != "truenas/"+answeredID || h.happened[0] || h.by[0] != "op-kanishk" {
		t.Fatalf("the answer must be recorded as given, with who gave it: %+v", h)
	}
}

// Strict, and explicit: a body without an answer, or with anything else in
// it, records nothing. "No answer" must never read as "it didn't happen".
func TestAnAnswerMustSayWhatHappened(t *testing.T) {
	for _, body := range []string{`{}`, `{"happened":true,"status":"succeeded"}`, `not json`} {
		h := &answerHarness{}
		stubAnswer(t, h)
		if rr := answer(t, "op-kanishk", body); rr.Code != http.StatusBadRequest {
			t.Errorf("%s: want 400, got %d", body, rr.Code)
		}
		if len(h.resolved) != 0 {
			t.Errorf("%s: nothing may be recorded", body)
		}
	}
}

// Syndra's own name is a read's, never a person's answer, and nobody may
// answer anonymously.
func TestAnAnswerNeedsAPerson(t *testing.T) {
	for _, actor := range []string{"", db.SyndraReader} {
		h := &answerHarness{}
		stubAnswer(t, h)
		if rr := answer(t, actor, `{"happened":true}`); rr.Code != http.StatusUnauthorized {
			t.Errorf("actor %q: want 401, got %d", actor, rr.Code)
		}
		if len(h.resolved) != 0 {
			t.Errorf("actor %q: nothing may be recorded", actor)
		}
	}
}

// Already answered, still in flight, a password, or another target's: one
// refusal, and nothing recorded.
func TestAnAnswerToSomethingNotWaitingIsRefused(t *testing.T) {
	h := &answerHarness{err: db.ErrAddonOperationNotUnresolved}
	stubAnswer(t, h)
	if rr := answer(t, "op-kanishk", `{"happened":true}`); rr.Code != http.StatusConflict {
		t.Fatalf("want 409, got %d", rr.Code)
	}
}

// An id that names no record is not a database error.
func TestAnAnswerToAMalformedIDIsNotWaiting(t *testing.T) {
	h := &answerHarness{}
	stubAnswer(t, h)
	r := httptest.NewRequest(http.MethodPost, "/api/v1/targets/truenas/unconfirmed/x/answer", strings.NewReader(`{"happened":true}`))
	r.SetPathValue("target", "truenas")
	r.SetPathValue("id", "not-an-id")
	r = r.WithContext(withPrincipal(r.Context(), &auth.Principal{Subject: "op-kanishk"}))
	rr := httptest.NewRecorder()
	handleSettleUnconfirmed(rr, r)
	if rr.Code != http.StatusConflict || len(h.resolved) != 0 {
		t.Fatalf("want 409 and nothing recorded, got %d %v", rr.Code, h.resolved)
	}
}
