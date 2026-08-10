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
	health, anchor := addonsHealth, dbGetLogAnchor
	t.Cleanup(func() { addonsHealth, dbGetLogAnchor = health, anchor })

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
	health, anchor := addonsHealth, dbGetLogAnchor
	t.Cleanup(func() { addonsHealth, dbGetLogAnchor = health, anchor })

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
