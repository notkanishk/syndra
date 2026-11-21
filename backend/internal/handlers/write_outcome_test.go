package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"syndra/internal/addons"
	"syndra/internal/db"
	"syndra/internal/services"
	"syndra/internal/services/addonop"
)

// Design §13.4: every write path says one of two things when it does not
// land. Each path below is driven against an unreached add-on (the monitor
// holds it addon_unreachable; the dispatch refuses it undialed) and an
// indeterminate one (sent, the answer lost).

// heldWhy is the monitor's addon_unreachable, as addons.Held returns it.
var heldWhy = addons.Why{Reason: addons.ReasonAddonUnreachable, Cause: addons.CauseNoAnswer, RetryAfter: 17}

func assertNothingSent(t *testing.T, rr *httptest.ResponseRecorder, reason string) {
	t.Helper()
	var out ErrorResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &out)
	if rr.Code != http.StatusServiceUnavailable || out.Error != "NOTHING_SENT" {
		t.Fatalf("want 503 NOTHING_SENT, got %d %s", rr.Code, rr.Body.String())
	}
	if rr.Header().Get("Retry-After") == "" || out.RetryAfter == 0 {
		t.Errorf("a 503 says when to try again (RFC 9110 §10.2.3): %s", rr.Body.String())
	}
	if out.Details["reason"] != reason {
		t.Errorf("details.reason = %q, want %q", out.Details["reason"], reason)
	}
	if !strings.HasPrefix(out.Message, "Nothing went out.") || !strings.Contains(out.Message, "Try again when") {
		t.Errorf("the sentence says what happened and the next step: %q", out.Message)
	}
	if strings.Contains(rr.Body.String(), "TARGET_UNREACHABLE") {
		t.Error("TARGET_UNREACHABLE is the add-on's wire code, never Syndra's (§13.3)")
	}
}

func assertUnconfirmed(t *testing.T, rr *httptest.ResponseRecorder) {
	t.Helper()
	var out map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &out)
	if rr.Code != http.StatusAccepted || out["status"] != "unconfirmed" {
		t.Fatalf("want 202 unconfirmed, got %d %s", rr.Code, rr.Body.String())
	}
	if d, _ := out["detail"].(string); !strings.Contains(d, "don't send it again") {
		t.Errorf("an unconfirmed write says not to send it again: %q", d)
	}
}

// answerDispatch makes every dispatch answer outcome, as the real one would.
func answerDispatch(outcome addons.Outcome) {
	svcDispatchOperation = func(context.Context, addonop.Request) (addonop.Result, error) {
		res := addonop.Result{OperationID: "op_1", Outcome: outcome}
		if outcome == addons.OutcomeUnreached {
			res.Why = heldWhy
		}
		return res, nil
	}
}

func TestEveryWritePathSaysOneOfTwoThings(t *testing.T) {
	type path struct {
		name string
		// run sets the path up, makes its add-on call answer outcome, and
		// sends the write.
		run func(t *testing.T, outcome addons.Outcome) *httptest.ResponseRecorder
	}
	paths := []path{
		{"adopt", func(t *testing.T, o addons.Outcome) *httptest.ResponseRecorder {
			stubAdopt(t, &adoptHarness{})
			answerDispatch(o)
			return adopt(t, `{"subject_id":"u1","confirmed":true}`)
		}},
		{"release", func(t *testing.T, o addons.Outcome) *httptest.ResponseRecorder {
			stubRelease(t, &releaseHarness{})
			answerDispatch(o)
			return release(t, `{"confirmed":true}`)
		}},
		{"setting", func(t *testing.T, o addons.Outcome) *httptest.ResponseRecorder {
			put := addonsPutSetting
			t.Cleanup(func() { addonsPutSetting = put })
			addonsPutSetting = func(context.Context, string, string, string, string) (addons.CallResponse, bool) {
				if o == addons.OutcomeUnreached {
					return addons.CallResponse{Outcome: o, Err: &addons.HeldError{Target: "truenas", Why: heldWhy}}, false
				}
				return addons.CallResponse{Outcome: o}, false
			}
			return putTargetSetting(t, `{"value":"/mnt/p/main/home_dir"}`)
		}},
		{"lifecycle", func(t *testing.T, o addons.Outcome) *httptest.ResponseRecorder {
			set := addonsSetLifecycle
			t.Cleanup(func() { addonsSetLifecycle = set })
			addonsSetLifecycle = func(context.Context, string, string, string, string) (map[string]any, error) {
				why := heldWhy
				if o != addons.OutcomeUnreached {
					why = addons.Why{Reason: addons.ReasonAddonUnreachable, Cause: addons.CauseTimeout}
				}
				return nil, &addons.NotLandedError{Target: "truenas", Outcome: o, Why: why}
			}
			r := httptest.NewRequest(http.MethodPost, "/api/v1/targets/truenas/lifecycle", strings.NewReader(`{"state":"draining","reason":"disk swap"}`))
			r.SetPathValue("target", "truenas")
			rr := httptest.NewRecorder()
			handleSetTargetLifecycle(rr, r)
			return rr
		}},
		{"password", func(t *testing.T, o addons.Outcome) *httptest.ResponseRecorder {
			stubMyStorage(t, &storageHarness{entitled: true, bound: true})
			answerDispatch(o)
			rr, _ := setCredential(t, `{"password":"`+memberSecret+`"}`)
			return rr
		}},
		{"physical action", func(t *testing.T, o addons.Outcome) *httptest.ResponseRecorder {
			stubMemberAction(t)
			answerDispatch(o)
			rr, _ := act(t, "item.checkout", confirmBody)
			return rr
		}},
		{"resolving a binding finding (release)", func(t *testing.T, o addons.Outcome) *httptest.ResponseRecorder {
			return resolveUnboundAgainst(t, o)
		}},
	}
	for _, p := range paths {
		t.Run(p.name+"/unreached", func(t *testing.T) {
			stubNames(t)
			assertNothingSent(t, p.run(t, addons.OutcomeUnreached), addons.ReasonAddonUnreachable)
		})
		t.Run(p.name+"/indeterminate", func(t *testing.T) {
			stubNames(t)
			assertUnconfirmed(t, p.run(t, addons.OutcomeIndeterminate))
		})
	}
}

// A preview has no effect, so a lost answer to one is nothing sent, never
// "may have applied"; and a held system is refused with the monitor's why.
func TestAPreviewThatDidNotLandIsNothingSent(t *testing.T) {
	for _, reply := range []addons.ReadReply{
		{Outcome: addons.OutcomeUnreached, Err: &addons.HeldError{Target: "truenas", Why: heldWhy}},
		{Outcome: addons.OutcomeIndeterminate},
	} {
		stubNames(t)
		h := stubMemberAction(t)
		h.preview = reply
		rr, _ := act(t, "item.checkout", previewBody)
		var out ErrorResponse
		_ = json.Unmarshal(rr.Body.Bytes(), &out)
		if rr.Code != http.StatusServiceUnavailable || out.Error != "NOTHING_SENT" {
			t.Fatalf("%s: %d %s", reply.Outcome, rr.Code, rr.Body.String())
		}
	}
}

// A member reads one sentence for both outages; an operator reads which side
// is down and what to check (§13.1).
func TestAMemberIsNotToldWhichBoxIsDown(t *testing.T) {
	stubNames(t)
	stubMyStorage(t, &storageHarness{entitled: true, bound: true})
	answerDispatch(addons.OutcomeUnreached)
	rr, _ := setCredential(t, `{"password":"`+memberSecret+`"}`)
	if body := rr.Body.String(); strings.Contains(body, "container") || !strings.Contains(body, "TrueNAS isn't answering") {
		t.Fatalf("member sentence: %s", body)
	}

	stubAdopt(t, &adoptHarness{})
	answerDispatch(addons.OutcomeUnreached)
	if body := adopt(t, `{"subject_id":"u1","confirmed":true}`).Body.String(); !strings.Contains(body, "Check that its container is running") {
		t.Fatalf("operator sentence: %s", body)
	}
}

// The add-on's own sentence about its app comes first; ours is the fallback,
// naming the product it fronts.
func TestAnAppOutageSaysTheAddOnsWordsFirst(t *testing.T) {
	n := &notLanded{System: "Ringside NAS", Product: "TrueNAS", Why: addons.Why{Reason: addons.ReasonAppUnreachable}}
	if got := whySentence(n); got != "Ringside NAS's add-on is running but can't reach TrueNAS. Check that TrueNAS is on." {
		t.Errorf("fallback: %q", got)
	}
	n.Why.Detail = "The NAS's API refused the session key"
	if got := whySentence(n); got != "The NAS's API refused the session key." {
		t.Errorf("add-on's words: %q", got)
	}
}

// The dormant removal is refused at once when the monitor holds the system
// down: no re-read, nothing dispatched.
func TestADormantRemovalToAHeldSystemIsRefusedUndialed(t *testing.T) {
	stubNames(t)
	h := &sweepHarness{report: services.DormantReport{Accounts: []services.DormantAccount{{Account: "gone", SubjectID: "u1"}}}}
	stubSweep(t, h)
	read := 0
	svcDormantAccounts = func(context.Context, string) (services.DormantReport, error) { read++; return h.report, nil }
	held := addonsHeld
	t.Cleanup(func() { addonsHeld = held })
	addonsHeld = func(string) (addons.Why, bool) { return heldWhy, true }

	assertNothingSent(t, sweep(t, `{"accounts":["gone"],"elevated_key":"k","confirmed":true}`), addons.ReasonAddonUnreachable)
	if read != 0 || len(h.purged) != 0 {
		t.Fatalf("a held system is neither read nor sent to: read %d, purged %d", read, len(h.purged))
	}
}

// Per account, the dormant removal says the same two words.
func TestADormantRemovalSaysEachAccountsOutcome(t *testing.T) {
	stubNames(t)
	h := &sweepHarness{report: services.DormantReport{Accounts: []services.DormantAccount{{Account: "gone", SubjectID: "u1"}}}}
	stubSweep(t, h)
	for o, word := range map[addons.Outcome]string{addons.OutcomeUnreached: "nothing_sent", addons.OutcomeIndeterminate: "unconfirmed"} {
		answerDispatch(o)
		body := sweep(t, `{"accounts":["gone"],"elevated_key":"k","confirmed":true}`).Body.String()
		if !strings.Contains(body, `"outcome":"`+word+`"`) || !strings.Contains(body, `"removed":0`) {
			t.Errorf("%s: %s", o, body)
		}
	}
}

// resolveUnboundAgainst resolves a finding as "stop managing it", its
// release answering outcome.
func resolveUnboundAgainst(t *testing.T, o addons.Outcome) *httptest.ResponseRecorder {
	t.Helper()
	h := &findingHarness{finding: db.MergeFinding{ID: "f1", Target: "truenas", SubjectID: "sub-1", Outcome: "deleted_upstream"}}
	stubFindings(t, h)
	answerDispatch(o)
	return resolveFinding(t, `{"resolution":"unbound","reason":"the account was removed on purpose"}`)
}
