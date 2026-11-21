package handlers

import (
	"os"
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

// A call that was sent and whose outcome Syndra could not record
// (addonop.Dispatch returns its OperationID with the error) is as unknown as
// a lost answer: 202 unconfirmed on every dispatching path, never a refusal
// or Syndra's 500 that invites a second send.
func sentUnrecorded() {
	svcDispatchOperation = func(context.Context, addonop.Request) (addonop.Result, error) {
		return addonop.Result{OperationID: "op_1", Outcome: addons.OutcomeSucceeded},
			errors.New("addonop: operation op_1 was dispatched but its outcome was not recorded")
	}
}

func TestASentCallWithItsOutcomeUnrecordedIsUnconfirmed(t *testing.T) {
	for name, run := range map[string]func(t *testing.T) *httptest.ResponseRecorder{
		"adopt": func(t *testing.T) *httptest.ResponseRecorder {
			stubAdopt(t, &adoptHarness{})
			sentUnrecorded()
			return adopt(t, `{"subject_id":"u1","confirmed":true}`)
		},
		"release": func(t *testing.T) *httptest.ResponseRecorder {
			stubRelease(t, &releaseHarness{})
			sentUnrecorded()
			return release(t, `{"confirmed":true}`)
		},
		"password": func(t *testing.T) *httptest.ResponseRecorder {
			stubMyStorage(t, &storageHarness{entitled: true, bound: true})
			sentUnrecorded()
			rr, _ := setCredential(t, `{"password":"`+memberSecret+`"}`)
			return rr
		},
		"physical action": func(t *testing.T) *httptest.ResponseRecorder {
			stubMemberAction(t)
			sentUnrecorded()
			rr, _ := act(t, "item.checkout", confirmBody)
			return rr
		},
		"resolving a binding finding (release)": func(t *testing.T) *httptest.ResponseRecorder {
			stubFindings(t, &findingHarness{finding: db.MergeFinding{ID: "f1", Target: "truenas", SubjectID: "sub-1", Outcome: "deleted_upstream"}})
			sentUnrecorded()
			return resolveFinding(t, `{"resolution":"unbound","reason":"the account was removed on purpose"}`)
		},
		"resolving a binding finding (adopt)": func(t *testing.T) *httptest.ResponseRecorder {
			stubConflictActions(t, bindingConflict("f1", "sub-1", numberHeldAdoptable))
			sentUnrecorded()
			return resolveFinding(t, `{"resolution":"adopted","reason":"it is theirs"}`)
		},
	} {
		t.Run(name, func(t *testing.T) {
			stubNames(t)
			assertUnconfirmed(t, run(t))
		})
	}
}

// The two paths whose reply lists the write inside a larger one say it the
// same way: the dormant removal's account and the revocation's rotation.
func TestASentCallWithItsOutcomeUnrecordedIsUnconfirmedInsideALargerReply(t *testing.T) {
	stubNames(t)
	stubSweep(t, &sweepHarness{report: services.DormantReport{Accounts: []services.DormantAccount{{Account: "gone", SubjectID: "u1"}}}})
	sentUnrecorded()
	if body := sweep(t, `{"accounts":["gone"],"elevated_key":"k","confirmed":true}`).Body.String(); !strings.Contains(body, `"outcome":"unconfirmed"`) {
		t.Errorf("dormant removal: %s", body)
	}

	stubRevocation(t)
	sentUnrecorded()
	if body := revoke(`{"reason":"offboarding","confirmed":true}`).Body.String(); !strings.Contains(body, `"status":"unconfirmed"`) {
		t.Errorf("rotation: %s", body)
	}
}

// Nothing went out is nothing went out, even when Syndra could not record it:
// an unreached outcome is never turned into "may have been applied".
func TestAnUnreachedCallWithItsOutcomeUnrecordedStaysUnreached(t *testing.T) {
	res, err := dispatched(addonop.Result{OperationID: "op_1", Outcome: addons.OutcomeUnreached}, errors.New("not recorded"))
	if err != nil || res.Outcome != addons.OutcomeUnreached {
		t.Fatalf("got %v %v", res.Outcome, err)
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
	if got := whySentence(n.System, n.Product, n.Why, n.Member); got != "Ringside NAS's add-on is running but can't reach TrueNAS. Check that TrueNAS is on." {
		t.Errorf("fallback: %q", got)
	}
	n.Why.Detail = "The NAS's API refused the session key"
	if got := whySentence(n.System, n.Product, n.Why, n.Member); got != "The NAS's API refused the session key." {
		t.Errorf("add-on's words: %q", got)
	}
}

// Every reason against design §13.1's table, operator and member. The app's
// two unusable answers come through addons.Unread, the one mapping.
func TestEveryReasonSaysItsSection13Sentence(t *testing.T) {
	unusable := addons.Unread(addons.CallResponse{Outcome: addons.OutcomeRejected, Status: 503, Code: "NO_CURRENT_READ"})
	unreadable := addons.Unread(addons.CallResponse{Outcome: addons.OutcomeRejected, Status: 503, Code: "TARGET_UNREADABLE"})
	for _, c := range []struct {
		name     string
		why      addons.Why
		operator string
		member   string
	}{
		{"no_answer", addons.Why{Reason: addons.ReasonAddonUnreachable, Cause: addons.CauseNoAnswer},
			"Syndra can't reach Ringside NAS's add-on. Check that its container is running. Syndra keeps trying.", "Ringside NAS isn't answering right now."},
		{"wrong_host", addons.Why{Reason: addons.ReasonAddonUnreachable, Cause: addons.CauseWrongHost},
			"Something answered at Ringside NAS's add-on address, but it isn't the add-on registered here. Check the address and the shared secret, then restart the add-on.", "Ringside NAS isn't answering right now."},
		{"timeout", addons.Why{Reason: addons.ReasonAddonUnreachable, Cause: addons.CauseTimeout},
			"Ringside NAS's add-on started answering and stopped. Syndra keeps trying. If it lasts, read the add-on's log.", "Ringside NAS isn't answering right now."},
		{"fault", addons.Why{Reason: addons.ReasonAddonUnreachable, Cause: addons.CauseFault},
			"Ringside NAS's add-on answered with an error it didn't explain. Read the add-on's log.", "Ringside NAS isn't answering right now."},
		{"breaker", addons.Why{Reason: addons.ReasonAddonUnreachable, Cause: addons.CauseNoAnswer, RetryAfter: 9},
			"Syndra can't reach Ringside NAS's add-on. Check that its container is running. Syndra keeps trying. Syndra tries again in 9 s.", "Ringside NAS isn't answering right now."},
		{"no_key", addons.Why{Reason: addons.ReasonAddonUnreachable, Cause: addons.CauseNoKey},
			"Syndra can't load Ringside NAS's shared secret. Check Syndra's secret file.", "Ringside NAS isn't answering right now."},
		{"app", addons.Why{Reason: addons.ReasonAppUnreachable},
			"Ringside NAS's add-on is running but can't reach TrueNAS. Check that TrueNAS is on.", "Ringside NAS isn't answering right now."},
		{"NO_CURRENT_READ", unusable,
			"TrueNAS didn't give its add-on an answer it could use. Check TrueNAS, then read again.", "Ringside NAS isn't answering right now."},
		{"TARGET_UNREADABLE", unreadable,
			"TrueNAS didn't give its add-on an answer it could use. Check TrueNAS, then read again.", "Ringside NAS isn't answering right now."},
		{"read_refused", addons.Why{Reason: addons.ReasonReadRefused},
			"Ringside NAS's add-on didn't take this. Read the add-on's log.", "Ringside NAS can't do this right now. Ask makerspace staff."},
		{"addon_outdated", addons.Why{Reason: addons.ReasonAddonOutdated},
			"Ringside NAS's add-on doesn't offer this. Update the add-on.", "Ringside NAS can't do this right now. Ask makerspace staff."},
		{"cooldown", addons.Why{Reason: addons.ReasonCooldown, RetryAfter: 5},
			"Ringside NAS was read moments ago. Read it again in 5 s.", "Ringside NAS was read moments ago. Read it again in 5 s."},
	} {
		if got := whySentence("Ringside NAS", "TrueNAS", c.why, false); got != c.operator {
			t.Errorf("%s operator: %q", c.name, got)
		}
		n := &notLanded{System: "Ringside NAS", Product: "TrueNAS", Outcome: addons.OutcomeUnreached, Why: c.why, Member: true}
		got := nothingSent(n)
		if got != "Nothing went out. "+c.member+map[bool]string{true: " Try again when Ringside NAS answers.", false: ""}[c.why.Unreachable()] {
			t.Errorf("%s member: %q", c.name, got)
		}
		if !c.why.Unreachable() && strings.Contains(got, "Try again when") {
			t.Errorf("%s is not an outage, so it is not waited out: %q", c.name, got)
		}
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

// A re-check that did not read says why it did not, the read's own why (8c's
// NotRead), never an empty one; any other failure is Syndra's own, 500.
func TestADormantRemovalsFailedReCheckCarriesItsWhy(t *testing.T) {
	stubNames(t)
	h := &sweepHarness{reportErr: fmt.Errorf("dormant: %w", addons.NotRead{Why: addons.Why{Reason: addons.ReasonAddonUnreachable, Cause: addons.CauseWrongHost}})}
	stubSweep(t, h)
	rr := sweep(t, `{"accounts":["gone"],"elevated_key":"k","confirmed":true}`)
	assertNothingSent(t, rr, addons.ReasonAddonUnreachable)
	if !strings.Contains(rr.Body.String(), `"cause":"wrong_host"`) {
		t.Errorf("the read's cause travels: %s", rr.Body)
	}
	h.reportErr = errors.New("read bindings: connection refused")
	if rr := sweep(t, `{"accounts":["gone"],"elevated_key":"k","confirmed":true}`); rr.Code != http.StatusInternalServerError {
		t.Errorf("Syndra's own fault is 500, got %d %s", rr.Code, rr.Body)
	}
}

// A re-check cut at the add-on's cap says so, in the list's own sentence,
// never read_refused's generic one.
func TestADormantRemovalsTruncatedReCheckSaysItWasCut(t *testing.T) {
	stubNames(t)
	stubSweep(t, &sweepHarness{reportErr: fmt.Errorf("dormant: %w: %w", services.ErrReadTruncated, addons.NotRead{Why: addons.Why{Reason: addons.ReasonReadRefused}})})
	rr := sweep(t, `{"accounts":["gone"],"elevated_key":"k","confirmed":true}`)
	if body := rr.Body.String(); rr.Code != http.StatusServiceUnavailable || !strings.Contains(body, "NOTHING_SENT") ||
		!strings.Contains(body, "listed only part of its accounts") || strings.Contains(body, "didn't take this") {
		t.Fatalf("got %d %s", rr.Code, body)
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

// Waiting does not update an add-on: an outdated one gets no Retry-After.
func TestAnOutdatedAddonIsNotToldToRetryLater(t *testing.T) {
	rr := httptest.NewRecorder()
	writeOutcome(rr, &notLanded{System: "Ringside NAS", Outcome: addons.OutcomeUnreached, Why: addons.Why{Reason: addons.ReasonAddonOutdated}}, nil)
	if rr.Code != http.StatusServiceUnavailable || rr.Header().Get("Retry-After") != "" {
		t.Fatalf("got %d Retry-After %q", rr.Code, rr.Header().Get("Retry-After"))
	}
}

// The copy guide's section 19 is the one source of §13.1's words: every
// operator and member-write row of its table is what whySentence and
// nothingSent say. The UI's whyLine is held to the same table
// (whyCopyGuide.test.ts), so the two ends cannot drift apart.
func TestTheCopyGuideSaysEverySection13Sentence(t *testing.T) {
	raw, err := os.ReadFile("../../../openspec/changes/plain-language-copy/design.md")
	if err != nil {
		t.Fatal(err)
	}
	guide := string(raw)
	section := guide[strings.Index(guide, "## 19. When a system is not answering"):]
	fill := strings.NewReplacer("{system}", "Ringside NAS", "{product}", "TrueNAS", "{n}", "9")
	checked := 0
	for _, line := range strings.Split(section, "\n") {
		if !strings.HasPrefix(line, "| operator |") && !strings.HasPrefix(line, "| member write |") {
			continue
		}
		cells := strings.Split(line, "|")
		reader, cause, wait, says := strings.TrimSpace(cells[1]), strings.TrimSpace(cells[3]), strings.TrimSpace(cells[4]), fill.Replace(strings.TrimSpace(cells[5]))
		for _, reason := range strings.Split(strings.TrimSpace(cells[2]), ", ") {
			why := addons.Why{Reason: reason}
			switch cause {
			case "—":
			case "unusable":
				why = addons.Unread(addons.CallResponse{Outcome: addons.OutcomeRejected, Status: 503, Code: "NO_CURRENT_READ"})
			default:
				why.Cause = cause
			}
			if wait != "—" {
				why.RetryAfter = 9
			}
			var got string
			if reader == "operator" {
				got = whySentence("Ringside NAS", "TrueNAS", why, false)
			} else {
				got = nothingSent(&notLanded{System: "Ringside NAS", Product: "TrueNAS", Outcome: addons.OutcomeUnreached, Why: why, Member: true})
			}
			if got != says {
				t.Errorf("%s %s %s: the guide says %q, the backend %q", reader, reason, cause, says, got)
			}
			checked++
		}
	}
	if checked < 10 {
		t.Fatalf("read only %d rows of the guide's table", checked)
	}
}
