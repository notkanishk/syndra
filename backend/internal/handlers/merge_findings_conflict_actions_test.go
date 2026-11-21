package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"syndra/internal/addons"
	"syndra/internal/db"
	"syndra/internal/services/addonop"
	"syndra/internal/services/drift"
)

// The three more answers to a binding conflict (addon-contract 7t-7, owner ruling 17 Nov 2026):
// adopt the holder when the add-on says it can, keep a twice-claimed account for one of the two,
// and Check again (the reconcile route, audited). Each goes through the one adopt or release path.

// conflictActions is findingHarness plus what these answers touch: per-finding reservations, the
// other claimant's finding, the adoption's binding write and the audit log.
type conflictActions struct {
	*findingHarness
	reservedOn []string // "id:decision"
	resolvedOn []string // "id:resolution"
	others     []db.MergeFinding
	recorded   []db.TargetBinding
	audits     []string
	accountID  string
}

func bindingConflict(id, subject, theirs string) db.MergeFinding {
	return db.MergeFinding{ID: id, Target: "truenas", SubjectID: subject, Outcome: "binding_conflict", Theirs: []byte(theirs)}
}

func stubConflictActions(t *testing.T, f db.MergeFinding) *conflictActions {
	t.Helper()
	h := &conflictActions{findingHarness: &findingHarness{finding: f, tracked: true}}
	stubFindings(t, h.findingHarness)
	list, record, decide, resolve, audit, dispatch := dbStandingMergeFindings, dbRecordTargetBinding,
		dbRecordMergeDecision, dbResolveMergeFinding, dbInsertAuditLogDetails, svcDispatchOperation
	t.Cleanup(func() {
		dbStandingMergeFindings, dbRecordTargetBinding = list, record
		dbRecordMergeDecision, dbResolveMergeFinding, dbInsertAuditLogDetails, svcDispatchOperation = decide, resolve, audit, dispatch
	})
	dbStandingMergeFindings = func(context.Context, string) ([]db.MergeFinding, error) {
		return append([]db.MergeFinding{f}, h.others...), nil
	}
	dbRecordMergeDecision = func(_ context.Context, id, actor, decision, reason string) (db.MergeFinding, error) {
		for _, o := range h.others {
			if o.ID == id && o.Decision != "" {
				return o, db.ErrMergeFindingDecided
			}
		}
		h.reservedOn = append(h.reservedOn, id+":"+decision)
		h.decided = append(h.decided, decision)
		return f, nil
	}
	dbResolveMergeFinding = func(_ context.Context, id, _, resolution string) (db.MergeFinding, error) {
		h.resolvedOn = append(h.resolvedOn, id+":"+resolution)
		return f, nil
	}
	dbRecordTargetBinding = func(_ context.Context, b db.TargetBinding) error {
		h.recorded = append(h.recorded, b)
		return nil
	}
	dbInsertAuditLogDetails = func(_ context.Context, actor, subject, action, resource string, _ any) error {
		h.audits = append(h.audits, actor+" "+action+" "+subject+" "+resource)
		return nil
	}
	svcDispatchOperation = func(_ context.Context, req addonop.Request) (addonop.Result, error) {
		h.dispatched = append(h.dispatched, req)
		outcome := h.dispatch
		if outcome == "" {
			outcome = addons.OutcomeSucceeded
		}
		res := addonop.Result{OperationID: "op_1", Outcome: outcome, AccountID: h.accountID}
		if outcome == addons.OutcomeRejected {
			res.Err = errors.New("eve can no longer be adopted for this person: TrueNAS changed since it was checked. Check again to see it as it is now.")
		}
		return res, nil
	}
	return h
}

const (
	nameHeldAdoptable   = `{"subject":"sub-1","reason":"name_held","username":"eve","account_id":"3051","adoptable":true}`
	numberHeldAdoptable = `{"subject":"sub-1","reason":"number_held","username":"eve","account_id":"3051","adoptable":true}`
	claimedTwice        = `{"subject":"sub-1","reason":"claimed_twice","username":"bob","account_id":"3051","adoptable":false,"bound_to":"sub-2"}`
)

// The gate takes each answer only where the stored report says it can work, and a refused answer
// does nothing at all: no reservation, no call to the add-on, no write.
func TestEachConflictAnswerIsTakenOnlyWhereItCanWork(t *testing.T) {
	for _, tc := range []struct {
		theirs, resolution string
		ok                 bool
	}{
		{nameHeldAdoptable, "adopted", true},
		{numberHeldAdoptable, "adopted", true},
		{`{"reason":"name_held","username":"eve","account_id":"3051","adoptable":false}`, "adopted", false},
		{`{"reason":"number_held","username":"eve","account_id":"3051"}`, "adopted", false},
		{`{"reason":"ambiguous","adoptable":true,"holders":[{"username":"eve","account_id":"1"},{"username":"eve","account_id":"2"}]}`, "adopted", false},
		{`{"reason":"claimed_twice","username":"bob","account_id":"3051","adoptable":true,"bound_to":"sub-2"}`, "adopted", false},
		{`{"reason":"name_held","account_id":"3051","adoptable":true}`, "adopted", false},
		{claimedTwice, "kept", true},
		{`{"reason":"claimed_twice","username":"bob","account_id":"3051"}`, "kept", false},
		{`{"reason":"claimed_twice","username":"bob","account_id":"3051","bound_to":"sub-1"}`, "kept", false},
		{nameHeldAdoptable, "kept", false},
		// Another person holds the name: not the same account claimed twice, so not theirs to keep.
		{`{"reason":"name_held","username":"eve","account_id":"3051","bound_to":"sub-2"}`, "kept", false},
		{`{"reason":"ambiguous"}`, "kept", false},
		{nameHeldAdoptable, "unbound", true},
		{claimedTwice, "unbound", true},
		{`{"reason":"ambiguous"}`, "unbound", true},
		{claimedTwice, "agreed", false},
	} {
		h := stubConflictActions(t, bindingConflict("f1", "sub-1", tc.theirs))
		rr := resolveFinding(t, `{"resolution":"`+tc.resolution+`","reason":"checked with them"}`)
		if tc.ok && rr.Code != http.StatusOK {
			t.Errorf("%s on %s: want 200, got %d %s", tc.resolution, tc.theirs, rr.Code, rr.Body.String())
		}
		if !tc.ok && (rr.Code != http.StatusUnprocessableEntity || len(h.dispatched)+len(h.reservedOn)+len(h.recorded)+len(h.forgotten) != 0) {
			t.Errorf("%s on %s: want 422 and nothing done, got %d %s (dispatched %v)", tc.resolution, tc.theirs, rr.Code, rr.Body.String(), h.dispatched)
		}
	}
}

// "It's theirs: adopt it" is the one account.adopt, for this person, naming the holder and the
// account it was shown as; this side's half is recordAdoptedBinding with what the add-on bound.
// A decision, not a settlement: the sweep closes it when the add-on stops reporting the conflict.
func TestAdoptingTheHolderGoesThroughTheOneAdoption(t *testing.T) {
	h := stubConflictActions(t, bindingConflict("f1", "sub-1", numberHeldAdoptable))
	h.accountID = "3051"
	rr := resolveFinding(t, `{"resolution":"adopted","reason":"eve is the same person, renamed"}`)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"resolved":false`) || !strings.Contains(rr.Body.String(), `"decided":true`) {
		t.Fatalf("want 200 decided and waiting, got %d %s", rr.Code, rr.Body.String())
	}
	if len(h.dispatched) != 1 {
		t.Fatalf("one call, got %+v", h.dispatched)
	}
	d := h.dispatched[0]
	if d.Operation != "account.adopt" || d.SubjectID != "sub-1" || !d.Confirmed ||
		d.Params["username"] != "eve" || d.Params["account_id"] != "3051" {
		t.Fatalf("the adoption names this person, the holder and the account shown: %+v", d)
	}
	if len(h.recorded) != 1 || h.recorded[0].SubjectID != "sub-1" || h.recorded[0].Username != "eve" ||
		h.recorded[0].AccountID == nil || *h.recorded[0].AccountID != "3051" {
		t.Fatalf("this side records the binding the add-on wrote: %+v", h.recorded)
	}
	if strings.Join(h.reservedOn, ",") != "f1:adopted" || len(h.resolvedOn) != 0 {
		t.Fatalf("reserved first, closed by the sweep: reserved %v resolved %v", h.reservedOn, h.resolvedOn)
	}
	if len(h.audits) != 1 || h.audits[0] != "system merge_finding.decided sub-1 f1" {
		t.Fatalf("audited with who decided: %v", h.audits)
	}
}

// A stale adoptable is the add-on's to refuse when it runs: its sentence reaches the operator,
// the reservation is given back, and nothing is recorded here.
func TestARefusedAdoptionChangesNothingHere(t *testing.T) {
	h := stubConflictActions(t, bindingConflict("f1", "sub-1", nameHeldAdoptable))
	h.dispatch = addons.OutcomeRejected
	rr := resolveFinding(t, `{"resolution":"adopted","reason":"it is theirs"}`)
	if rr.Code != http.StatusConflict || !strings.Contains(rr.Body.String(), "ADOPTION_REFUSED") ||
		!strings.Contains(rr.Body.String(), "Check again") {
		t.Fatalf("want 409 in the add-on's words, got %d %s", rr.Code, rr.Body.String())
	}
	if len(h.recorded) != 0 || len(h.forgotten) != 0 || len(h.reservationsCleared) != 1 || len(h.audits) != 0 {
		t.Fatalf("nothing kept: recorded %v forgotten %v cleared %v audits %v", h.recorded, h.forgotten, h.reservationsCleared, h.audits)
	}
}

// "Keep it for sub-1" on sub-1's own row releases sub-2 and never sub-1, forgets sub-2's copy,
// and settles sub-2's finding (stop tracking, from their side); sub-1's stays decided until the
// add-on reports one binding.
func TestKeepingItForThisPersonReleasesTheOtherNeverThisOne(t *testing.T) {
	h := stubConflictActions(t, bindingConflict("f1", "sub-1", claimedTwice))
	h.others = []db.MergeFinding{bindingConflict("f2", "sub-2", `{"subject":"sub-2","reason":"claimed_twice","username":"bob","account_id":"3051","bound_to":"sub-1"}`)}
	rr := resolveFinding(t, `{"resolution":"kept","reason":"bob is sub-1's"}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d %s", rr.Code, rr.Body.String())
	}
	if len(h.dispatched) != 1 || h.dispatched[0].Operation != "account.release" || h.dispatched[0].SubjectID != "sub-2" {
		t.Fatalf("release the other person, only: %+v", h.dispatched)
	}
	if strings.Join(h.forgotten, ",") != "sub-2" {
		t.Fatalf("forget the other person's copy, never this one's: %v", h.forgotten)
	}
	if strings.Join(h.reservedOn, ",") != "f1:kept,f2:unbound" || strings.Join(h.resolvedOn, ",") != "f2:unbound" {
		t.Fatalf("both rows answered, the other's settled: reserved %v resolved %v", h.reservedOn, h.resolvedOn)
	}
}

// "Keep it for sub-2" on sub-1's row is Stop tracking for sub-1, and holds sub-2's row as kept, so
// nobody can then keep it for sub-1 from sub-2's row: both releases would leave it to neither.
func TestStopTrackingATwiceClaimedAccountHoldsTheOtherRowAsKept(t *testing.T) {
	h := stubConflictActions(t, bindingConflict("f1", "sub-1", claimedTwice))
	h.others = []db.MergeFinding{bindingConflict("f2", "sub-2", `{"subject":"sub-2","reason":"claimed_twice","bound_to":"sub-1"}`)}
	rr := resolveFinding(t, `{"resolution":"unbound","reason":"bob is sub-2's"}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d %s", rr.Code, rr.Body.String())
	}
	if len(h.dispatched) != 1 || h.dispatched[0].SubjectID != "sub-1" || strings.Join(h.forgotten, ",") != "sub-1" {
		t.Fatalf("release this person: %+v %v", h.dispatched, h.forgotten)
	}
	if strings.Join(h.reservedOn, ",") != "f1:unbound,f2:kept" || strings.Join(h.resolvedOn, ",") != "f1:unbound" {
		t.Fatalf("reserved %v resolved %v", h.reservedOn, h.resolvedOn)
	}
}

// The opposite answer already given on the other person's row refuses this one before the add-on
// is called, and gives this row's reservation back.
func TestTheOtherRowsOppositeAnswerRefusesBeforeTheTarget(t *testing.T) {
	h := stubConflictActions(t, bindingConflict("f1", "sub-1", claimedTwice))
	other := bindingConflict("f2", "sub-2", `{"subject":"sub-2","reason":"claimed_twice","bound_to":"sub-1"}`)
	other.Decision, other.DecidedBy, other.DecisionReason = "kept", "op-bayley", "bob is sub-2's"
	h.others = []db.MergeFinding{other}
	rr := resolveFinding(t, `{"resolution":"kept","reason":"bob is sub-1's"}`)
	if rr.Code != http.StatusConflict || !strings.Contains(rr.Body.String(), "op-bayley") {
		t.Fatalf("want 409 naming who decided, got %d %s", rr.Code, rr.Body.String())
	}
	if len(h.dispatched) != 0 || len(h.forgotten) != 0 || strings.Join(h.reservationsCleared, ",") != "kept" {
		t.Fatalf("nothing called, this row's reservation given back: %+v %v %v", h.dispatched, h.forgotten, h.reservationsCleared)
	}
}

func reconcileTruenas(t *testing.T) *httptest.ResponseRecorder {
	t.Helper()
	get, rec := addonsGet, svcReconcileAddon
	t.Cleanup(func() { addonsGet, svcReconcileAddon = get, rec })
	addonsGet = func(string) (*addons.Addon, error) { return &addons.Addon{}, nil }
	svcReconcileAddon = func(_ context.Context, target string) (drift.AddonReconcileResult, error) {
		return drift.AddonReconcileResult{Target: target, Read: readNow()}, nil
	}
	rr := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/v1/targets/truenas/reconcile", nil)
	r.SetPathValue("target", "truenas")
	handleReconcileTarget(rr, r)
	return rr
}

// Check again is the reconcile route, and who pressed it is audited.
func TestACheckIsAuditedWithWhoAskedForIt(t *testing.T) {
	stubConflictActions(t, bindingConflict("f1", "sub-1", claimedTwice))
	var audits []string
	dbInsertAuditLogDetails = func(_ context.Context, actor, _, action, resource string, _ any) error {
		audits = append(audits, actor+" "+action+" "+resource)
		return nil
	}
	rec := reconcileTruenas(t)
	if rec.Code != http.StatusOK || strings.Join(audits, ",") != "system target.checked truenas" {
		t.Fatalf("want the check audited, got %d %v", rec.Code, audits)
	}
}
