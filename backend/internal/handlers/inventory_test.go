package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"syndra/internal/addons"
	"syndra/internal/db"
	"syndra/internal/services"
	"syndra/internal/services/addonop"
)

// Adoption is the one action in the system that hands one person's data to
// another, and it has no undo. What it says it did therefore has to be what it
// did — found on the dev deployment, where a target that refused an adoption
// and a target that never answered both produced 200 "The account is now bound
// to that person."

type adoptHarness struct {
	outcome   addons.Outcome
	addonErr  error
	uid       int64
	dispatchd []addonop.Request
	recorded  []db.TargetBinding
}

func stubAdopt(t *testing.T, h *adoptHarness) {
	t.Helper()
	dispatch, record := svcDispatchOperation, dbRecordTargetBinding
	t.Cleanup(func() { svcDispatchOperation, dbRecordTargetBinding = dispatch, record })

	svcDispatchOperation = func(_ context.Context, req addonop.Request) (addonop.Result, error) {
		h.dispatchd = append(h.dispatchd, req)
		outcome := h.outcome
		if outcome == "" {
			outcome = addons.OutcomeSucceeded
		}
		return addonop.Result{OperationID: "op_1", Outcome: outcome, Err: h.addonErr, AccountUID: h.uid}, nil
	}
	dbRecordTargetBinding = func(_ context.Context, b db.TargetBinding) error {
		h.recorded = append(h.recorded, b)
		return nil
	}
}

func adopt(t *testing.T, body string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(http.MethodPost, "/api/v1/targets/truenas/inventory/ada/adopt", strings.NewReader(body))
	r.SetPathValue("target", "truenas")
	r.SetPathValue("username", "ada")
	rr := httptest.NewRecorder()
	handleAdoptAccount(rr, r)
	return rr
}

// The uid the add-on read off the target is what gets recorded — not the name
// the operator clicked. A binding known only by name stops recognising its
// account the moment somebody renames it, and the account then reappears in the
// unmanaged inventory for a second person to adopt.
func TestASuccessfulAdoptionRecordsTheAccountsUID(t *testing.T) {
	h := &adoptHarness{uid: 3002}
	stubAdopt(t, h)

	rr := adopt(t, `{"subject_id":"u1","confirmed":true}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", rr.Code, rr.Body.String())
	}
	if len(h.recorded) != 1 {
		t.Fatalf("want one binding, got %d", len(h.recorded))
	}
	if h.recorded[0].AccountUID == nil || *h.recorded[0].AccountUID != 3002 {
		t.Errorf("the uid must travel into the binding: %+v", h.recorded[0].AccountUID)
	}
}

// And a uid nobody reported is absent rather than zero. uid 0 is root.
func TestAnUnreportedUIDIsNotRecordedAsRoot(t *testing.T) {
	h := &adoptHarness{uid: 0}
	stubAdopt(t, h)

	if rr := adopt(t, `{"subject_id":"u1","confirmed":true}`); rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rr.Code)
	}
	if len(h.recorded) != 1 || h.recorded[0].AccountUID != nil {
		t.Errorf("a missing uid must stay missing, got %+v", h.recorded[0].AccountUID)
	}
}

// A refusal is answered as a refusal, with what the target said.
func TestARefusedAdoptionIsNotReportedAsAdopted(t *testing.T) {
	h := &adoptHarness{
		outcome:  addons.OutcomeRejected,
		addonErr: errors.New("ada is already bound to another subject (leo)"),
	}
	stubAdopt(t, h)

	rr := adopt(t, `{"subject_id":"u1","confirmed":true}`)
	if rr.Code != http.StatusConflict {
		t.Fatalf("want 409, got %d (%s)", rr.Code, rr.Body.String())
	}
	if len(h.recorded) != 0 {
		t.Error("nothing may be recorded for an adoption the target refused")
	}
	if !strings.Contains(rr.Body.String(), "already bound to another subject") {
		t.Errorf("the target's own words are the useful half: %s", rr.Body.String())
	}
}

// An outcome nobody confirmed is neither success nor failure, and it says so.
func TestAnUnconfirmedAdoptionIsNotRecorded(t *testing.T) {
	for _, outcome := range []addons.Outcome{addons.OutcomeUnreached, addons.OutcomeIndeterminate} {
		t.Run(string(outcome), func(t *testing.T) {
			h := &adoptHarness{outcome: outcome}
			stubAdopt(t, h)

			rr := adopt(t, `{"subject_id":"u1","confirmed":true}`)
			if rr.Code != http.StatusAccepted {
				t.Fatalf("want 202, got %d (%s)", rr.Code, rr.Body.String())
			}
			var out map[string]any
			_ = json.Unmarshal(rr.Body.Bytes(), &out)
			if out["status"] == "adopted" {
				t.Error("an outcome the target did not confirm must not read as adopted")
			}
			if len(h.recorded) != 0 {
				t.Error("nothing may be recorded for a change nobody confirmed")
			}
		})
	}
}

// A detected violation has to reach a person.
//
// The sweep recorded truncation findings into a table with no reader: correct
// detection, correct row, one log line, and no surface in the system able to
// report it. Tamper-evidence nobody is told about has done half its job.
func TestACompromisedLogTravelsWithTheTargetsHealth(t *testing.T) {
	health, anchor, conflicts := addonsHealth, dbGetLogAnchor, dbStandingBindingConflicts
	t.Cleanup(func() {
		addonsHealth, dbGetLogAnchor, dbStandingBindingConflicts = health, anchor, conflicts
	})
	// The health payload now carries binding conflicts beside the anchor, and
	// the real read needs a database.
	dbStandingBindingConflicts = func(context.Context, string) ([]db.BindingConflict, error) {
		return nil, nil
	}

	addonsHealth = func(context.Context, string) addons.TargetHealth {
		return addons.TargetHealth{Reachable: true, Outcome: addons.OutcomeSucceeded}
	}
	dbGetLogAnchor = func(context.Context, string) (db.LogAnchor, bool, error) {
		return db.LogAnchor{Target: "truenas", Head: "abc", Records: 3, ViolationReason: db.AnchorRecordsDecreased}, true, nil
	}

	r := httptest.NewRequest(http.MethodGet, "/api/v1/targets/truenas/health", nil)
	r.SetPathValue("target", "truenas")
	rr := httptest.NewRecorder()
	handleTargetHealth(rr, r)

	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), db.AnchorRecordsDecreased) {
		t.Errorf("the finding must be in the answer an operator reads: %s", rr.Body.String())
	}
}

// And it survives the target being unreachable. An add-on that has gone away is
// exactly when somebody would like to know its record was edited.
func TestACompromisedLogIsReportedEvenWhenTheTargetIsDown(t *testing.T) {
	health, anchor, conflicts := addonsHealth, dbGetLogAnchor, dbStandingBindingConflicts
	t.Cleanup(func() {
		addonsHealth, dbGetLogAnchor, dbStandingBindingConflicts = health, anchor, conflicts
	})
	// The health payload now carries binding conflicts beside the anchor, and
	// the real read needs a database.
	dbStandingBindingConflicts = func(context.Context, string) ([]db.BindingConflict, error) {
		return nil, nil
	}

	addonsHealth = func(context.Context, string) addons.TargetHealth {
		return addons.TargetHealth{Outcome: addons.OutcomeUnreached, Err: errors.New("dial: no route to host")}
	}
	dbGetLogAnchor = func(context.Context, string) (db.LogAnchor, bool, error) {
		return db.LogAnchor{Target: "truenas", ViolationReason: db.AnchorHeadRewritten}, true, nil
	}

	r := httptest.NewRequest(http.MethodGet, "/api/v1/targets/truenas/health", nil)
	r.SetPathValue("target", "truenas")
	rr := httptest.NewRecorder()
	handleTargetHealth(rr, r)

	if !strings.Contains(rr.Body.String(), db.AnchorHeadRewritten) {
		t.Errorf("an unreachable target must still report its finding: %s", rr.Body.String())
	}
}

// §29's sweep — the only bulk action in the product.
//
// What makes it safe is not the ceremony, it is the re-check: the list an
// operator ticked was true when they read it, and this is the only thing that
// makes it true when it runs.

type sweepHarness struct {
	report    services.DormantReport
	reportErr error
	purged    []addonop.Request
	outcome   addons.Outcome
	forgotten []string
}

func stubSweep(t *testing.T, h *sweepHarness) {
	t.Helper()
	dormant, dispatch, forget := svcDormantAccounts, svcDispatchOperation, dbForgetTargetBinding
	t.Cleanup(func() {
		svcDormantAccounts, svcDispatchOperation, dbForgetTargetBinding = dormant, dispatch, forget
	})

	// The backend's half of a purge. Stubbed rather than left to the pool,
	// because it had no caller at all until §23 — and a nil-pool panic is how
	// that absence finally became visible.
	dbForgetTargetBinding = func(_ context.Context, _, subjectID string) error {
		h.forgotten = append(h.forgotten, subjectID)
		return nil
	}

	svcDormantAccounts = func(context.Context, string) (services.DormantReport, error) {
		return h.report, h.reportErr
	}
	svcDispatchOperation = func(_ context.Context, req addonop.Request) (addonop.Result, error) {
		h.purged = append(h.purged, req)
		outcome := h.outcome
		if outcome == "" {
			outcome = addons.OutcomeSucceeded
		}
		return addonop.Result{OperationID: "op_1", Outcome: outcome}, nil
	}
}

func sweep(t *testing.T, body string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(http.MethodPost, "/api/v1/targets/truenas/accounts/dormant/sweep",
		strings.NewReader(body))
	r.SetPathValue("target", "truenas")
	rr := httptest.NewRecorder()
	handleDormantSweep(rr, r)
	return rr
}

func TestASweepRemovesOnlyWhatIsStillDormant(t *testing.T) {
	h := &sweepHarness{report: services.DormantReport{
		Accounts: []services.DormantAccount{
			{Account: "gone", SubjectID: "u1", SubjectStillMember: false},
		},
	}}
	stubSweep(t, h)

	// "revived" was on the operator's list and is not on this one: somebody
	// gave them a role between the read and the click.
	rr := sweep(t, `{"accounts":["gone","revived"],"elevated_key":"k","confirmed":true}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", rr.Code, rr.Body.String())
	}
	if len(h.purged) != 1 || h.purged[0].SubjectID != "u1" {
		t.Fatalf("only the still-dormant account may be purged: %+v", h.purged)
	}
	if !strings.Contains(rr.Body.String(), "No longer dormant") {
		t.Errorf("the refusal must be reported per account: %s", rr.Body.String())
	}
}

// Still-a-member rows are refused by the BACKEND, not merely unselectable in
// the surface: removing one locks somebody out rather than tidying up, and a
// client-side filter is a suggestion.
func TestASweepRefusesAnAccountWhoseSubjectIsStillAMember(t *testing.T) {
	h := &sweepHarness{report: services.DormantReport{
		Accounts: []services.DormantAccount{
			{Account: "locked-out", SubjectID: "u2", SubjectStillMember: true},
		},
	}}
	stubSweep(t, h)

	sweep(t, `{"accounts":["locked-out"],"elevated_key":"k","confirmed":true}`)
	if len(h.purged) != 0 {
		t.Error("an account whose subject is still a member must never be purged here")
	}
}

func TestASweepRefusesWithoutAConfirmationOrACredential(t *testing.T) {
	h := &sweepHarness{report: services.DormantReport{
		Accounts: []services.DormantAccount{{Account: "gone", SubjectID: "u1"}},
	}}
	stubSweep(t, h)

	if code := sweep(t, `{"accounts":["gone"],"elevated_key":"k"}`).Code; code != http.StatusUnprocessableEntity {
		t.Errorf("want 422 without a confirmation, got %d", code)
	}
	if code := sweep(t, `{"accounts":["gone"],"confirmed":true}`).Code; code != http.StatusBadRequest {
		t.Errorf("want 400 without a credential, got %d", code)
	}
	if len(h.purged) != 0 {
		t.Error("nothing may be dispatched before both are present")
	}
}

// The credential exists for the length of one request and appears in nothing
// this handler emits — the same property the member's password path is held to.
func TestTheElevatedCredentialIsNotEchoed(t *testing.T) {
	const key = "delete-capable-9!"
	h := &sweepHarness{report: services.DormantReport{
		Accounts: []services.DormantAccount{{Account: "gone", SubjectID: "u1"}},
	}}
	stubSweep(t, h)

	rr := sweep(t, `{"accounts":["gone"],"elevated_key":"`+key+`","confirmed":true}`)
	if strings.Contains(rr.Body.String(), key) {
		t.Errorf("the response carries the credential: %s", rr.Body.String())
	}
	if got, _ := h.purged[0].Params["elevated_key"].(string); got != key {
		t.Error("it must reach the add-on unchanged, or this test proves nothing")
	}
}

// An outcome the target did not confirm is never retried on this path: a purge
// that may have happened is the one operation where trying again is not free.
func TestAnUnconfirmedPurgeIsReportedRatherThanRetried(t *testing.T) {
	h := &sweepHarness{
		report: services.DormantReport{
			Accounts: []services.DormantAccount{{Account: "gone", SubjectID: "u1"}},
		},
		outcome: addons.OutcomeIndeterminate,
	}
	stubSweep(t, h)

	rr := sweep(t, `{"accounts":["gone"],"elevated_key":"k","confirmed":true}`)
	if len(h.purged) != 1 {
		t.Fatalf("want exactly one attempt, got %d", len(h.purged))
	}
	body := rr.Body.String()
	if !strings.Contains(body, "did not confirm") {
		t.Errorf("an unconfirmed purge must say so: %s", body)
	}
	if strings.Contains(body, `"removed":1`) {
		t.Error("it must not be counted as removed")
	}
}

// §23 — the binding goes with the account.
//
// `ForgetTargetBinding` had no caller, and its own docblock said what that
// costs: the apply path reads bound-but-absent as an out-of-band deletion and
// RECREATES the account under the recorded name. Right for one somebody else
// deleted; exactly wrong for one this sweep just purged. The add-on drops its
// own binding as part of the purge, so the two stores disagreed.
func TestAPurgeForgetsTheBindingItLeavesBehind(t *testing.T) {
	h := &sweepHarness{report: services.DormantReport{
		Accounts: []services.DormantAccount{
			{Account: "gone", SubjectID: "u1", SubjectStillMember: false},
		},
	}}
	stubSweep(t, h)

	rr := sweep(t, `{"accounts":["gone"],"elevated_key":"k","confirmed":true}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if len(h.forgotten) != 1 || h.forgotten[0] != "u1" {
		t.Fatalf("a purged account must not leave its binding: %v", h.forgotten)
	}
}

// And an outcome the target did not confirm leaves it alone. Forgetting the
// binding for an account that may still exist is the mirror failure: the next
// convergence would derive a fresh name and make a second account beside it.
func TestAnUnconfirmedPurgeKeepsTheBinding(t *testing.T) {
	h := &sweepHarness{
		outcome: addons.OutcomeIndeterminate,
		report: services.DormantReport{
			Accounts: []services.DormantAccount{
				{Account: "maybe-gone", SubjectID: "u1", SubjectStillMember: false},
			},
		},
	}
	stubSweep(t, h)

	sweep(t, `{"accounts":["maybe-gone"],"elevated_key":"k","confirmed":true}`)
	if len(h.forgotten) != 0 {
		t.Fatalf("an unconfirmed purge must leave the binding in place: %v", h.forgotten)
	}
}
