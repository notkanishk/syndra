package drift

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"regexp"
	"testing"
	"time"

	"syndra/internal/addons"
	"syndra/internal/db"
)

// 1.18/1.19 — the first sweep against a target that was already in use.
//
// The failure this prevents is a queue nobody trusts: a NAS holds `root`,
// service accounts and whatever an admin made by hand, and a reconcile that
// classified those as untraced access would fill the triage queue with findings
// that are not findings on day one.

type addonReconcileHarness struct {
	read       addons.SubjectsResult
	bindings   []db.TargetBinding
	plan       addons.PlanResult
	planAsked  []addons.PlanSubject
	converged  []db.SystemConvergence
	marked     []string
	reconciled bool
}

func stubAddonReconcile(t *testing.T, h *addonReconcileHarness) {
	t.Helper()
	origSubjects, origPlan, origBindings := addonSubjects, addonPlan, listBindings
	origRecord, origResolve := recordConvergence, resolveIntent
	origUnrec, origRec := markUnreconciled, markReconciled
	t.Cleanup(func() {
		addonSubjects, addonPlan, listBindings = origSubjects, origPlan, origBindings
		recordConvergence, resolveIntent = origRecord, origResolve
		markUnreconciled, markReconciled = origUnrec, origRec
	})

	addonSubjects = func(context.Context, string) addons.SubjectsResult { return h.read }
	listBindings = func(context.Context, string) ([]db.TargetBinding, error) { return h.bindings, nil }
	addonPlan = func(_ context.Context, _ string, subjects []addons.PlanSubject, _ bool) addons.PlanResult {
		h.planAsked = subjects
		return h.plan
	}
	resolveIntent = func(_ context.Context, subjectID, _ string) (map[string]json.RawMessage, error) {
		return map[string]json.RawMessage{"enabled": json.RawMessage(`true`)}, nil
	}
	recordConvergence = func(_ context.Context, c db.SystemConvergence) (string, string, error) {
		h.converged = append(h.converged, c)
		return "plan_1", "outbox_1", nil
	}
	markUnreconciled = func(_ context.Context, target, reason string) (db.TargetReconciliation, error) {
		h.marked = append(h.marked, reason)
		return db.TargetReconciliation{}, nil
	}
	markReconciled = func(context.Context, string) (db.TargetReconciliation, error) {
		h.reconciled = true
		return db.TargetReconciliation{}, nil
	}
}

func currentRead(accounts ...addons.TargetAccount) addons.SubjectsResult {
	return addons.SubjectsResult{
		Accounts: accounts, Current: true, Outcome: addons.OutcomeSucceeded,
		TakenAt: time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC),
	}
}

func uid(n int64) *int64 { return &n }

// The scenario the requirement is written for: a target full of accounts nobody
// provisioned, seen for the first time.
func TestAFirstSweepRaisesNoDriftAndReportsInventory(t *testing.T) {
	h := &addonReconcileHarness{
		read: currentRead(
			addons.TargetAccount{Username: "root", UID: 0},
			addons.TargetAccount{Username: "backup-svc", UID: 3100},
			addons.TargetAccount{Username: "someone-elses", UID: 3101},
		),
		// Nothing is bound: Syndra has provisioned nobody here yet.
		bindings: nil,
	}
	stubAddonReconcile(t, h)

	res, err := ReconcileAddon(context.Background(), "truenas")
	if err != nil {
		t.Fatalf("ReconcileAddon: %v", err)
	}
	if len(res.Unmanaged) != 3 {
		t.Fatalf("every unbound account must be reported as inventory, got %+v", res.Unmanaged)
	}
	if res.Queued != 0 {
		t.Errorf("an unbound account must not be converged: %d queued", res.Queued)
	}
	if len(h.converged) != 0 {
		t.Errorf("a sweep must not bind an account by inference: %+v", h.converged)
	}
	if len(h.planAsked) != 0 {
		t.Error("with nobody bound there is nothing to ask the target about")
	}
	if !h.reconciled {
		t.Error("a complete current read must record the target as reconciled")
	}
}

// Bound subjects are converged; unbound accounts beside them are still only
// inventory. The two halves must not contaminate each other.
func TestOnlyBoundSubjectsAreConverged(t *testing.T) {
	h := &addonReconcileHarness{
		read: currentRead(
			addons.TargetAccount{Username: "ada", UID: 3001},
			addons.TargetAccount{Username: "root", UID: 0},
		),
		bindings: []db.TargetBinding{{Target: "truenas", SubjectID: "u1", Username: "ada", AccountUID: uid(3001)}},
		plan: addons.PlanResult{
			Outcome: addons.OutcomeSucceeded, Current: true,
			Outcomes: []addons.SubjectOutcome{{Subject: "u1", Effect: db.PlanEffectApply}},
		},
	}
	stubAddonReconcile(t, h)

	res, err := ReconcileAddon(context.Background(), "truenas")
	if err != nil {
		t.Fatalf("ReconcileAddon: %v", err)
	}
	if len(h.planAsked) != 1 || h.planAsked[0].Subject != "u1" {
		t.Fatalf("only bound subjects may be asked about: %+v", h.planAsked)
	}
	if res.Queued != 1 || len(h.converged) != 1 {
		t.Fatalf("a bound subject out of step must be converged: %d", res.Queued)
	}
	if len(res.Unmanaged) != 1 || res.Unmanaged[0].Username != "root" {
		t.Errorf("the unbound account must still be inventory: %+v", res.Unmanaged)
	}
}

// A blocked outcome is an operator decision — a binding conflict, most often —
// and queueing a convergence for it would be the sweep inferring exactly the
// decision it is forbidden from inferring.
func TestABlockedSubjectIsNotConverged(t *testing.T) {
	h := &addonReconcileHarness{
		read:     currentRead(addons.TargetAccount{Username: "ada", UID: 3001}),
		bindings: []db.TargetBinding{{Target: "truenas", SubjectID: "u1", Username: "ada", AccountUID: uid(3001)}},
		plan: addons.PlanResult{
			Outcome: addons.OutcomeSucceeded, Current: true,
			Outcomes: []addons.SubjectOutcome{{Subject: "u1", Effect: db.PlanEffectBlocked}},
		},
	}
	stubAddonReconcile(t, h)

	res, err := ReconcileAddon(context.Background(), "truenas")
	if err != nil {
		t.Fatalf("ReconcileAddon: %v", err)
	}
	if res.Queued != 0 || len(h.converged) != 0 {
		t.Fatalf("a blocked row must wait for an operator: %+v", h.converged)
	}
}

// A rename moves the name and leaves the uid. Matching by name alone would
// report a member's own account as unmanaged and invite an operator to adopt it
// for somebody else.
func TestARenamedAccountIsStillRecognisedAsManaged(t *testing.T) {
	h := &addonReconcileHarness{
		read:     currentRead(addons.TargetAccount{Username: "ada-rivera", UID: 3001}),
		bindings: []db.TargetBinding{{Target: "truenas", SubjectID: "u1", Username: "ada", AccountUID: uid(3001)}},
		plan: addons.PlanResult{
			Outcome: addons.OutcomeSucceeded, Current: true,
			Outcomes: []addons.SubjectOutcome{{Subject: "u1", Effect: db.PlanEffectNoChange}},
		},
	}
	stubAddonReconcile(t, h)

	res, err := ReconcileAddon(context.Background(), "truenas")
	if err != nil {
		t.Fatalf("ReconcileAddon: %v", err)
	}
	if len(res.Unmanaged) != 0 {
		t.Fatalf("a renamed account is not an unmanaged one: %+v", res.Unmanaged)
	}
}

// A stale read is not evidence. Every conclusion drawn from the mirror would be
// about an earlier moment, and the convergences queued from one would be
// computed against a world that has moved.
func TestAStaleReadConvergesNothingAndSaysWhy(t *testing.T) {
	h := &addonReconcileHarness{
		read: addons.SubjectsResult{
			Accounts: []addons.TargetAccount{{Username: "ada", UID: 3001}},
			Current:  false, Outcome: addons.OutcomeSucceeded,
		},
		bindings: []db.TargetBinding{{Target: "truenas", SubjectID: "u1", Username: "ada"}},
	}
	stubAddonReconcile(t, h)

	res, err := ReconcileAddon(context.Background(), "truenas")
	if err != nil {
		t.Fatalf("ReconcileAddon: %v", err)
	}
	if !res.Halted || res.Reason != db.UnreconciledStaleRead {
		t.Fatalf("a mirrored read must halt the pass: %+v", res)
	}
	if len(h.converged) != 0 || len(h.planAsked) != 0 {
		t.Error("nothing may be concluded from a read of an earlier moment")
	}
	if h.reconciled {
		t.Error("a target served from its mirror has not been reconciled")
	}
	if len(h.marked) != 1 || h.marked[0] != db.UnreconciledStaleRead {
		t.Errorf("the outage must be recorded with its reason: %v", h.marked)
	}
}

// An unreachable target records the outage and concludes nothing — an absence
// of evidence reported as such, rather than fabricated evidence of absence.
func TestAnUnreachableTargetRecordsTheOutage(t *testing.T) {
	h := &addonReconcileHarness{
		read: addons.SubjectsResult{Outcome: addons.OutcomeUnreached, Err: errors.New("connection refused")},
	}
	stubAddonReconcile(t, h)

	res, err := ReconcileAddon(context.Background(), "truenas")
	if err != nil {
		t.Fatalf("ReconcileAddon: %v", err)
	}
	if !res.Halted || res.Reason != db.UnreconciledUnreachable {
		t.Fatalf("want the outage recorded: %+v", res)
	}
	if len(res.Unmanaged) != 0 {
		t.Error("an unreachable target must not report an empty inventory as a finding")
	}
}

// A truncated read still supports the inventory — everything in it was seen —
// and the pass says the picture is incomplete rather than pretending otherwise.
func TestATruncatedReadStillReportsWhatItSaw(t *testing.T) {
	read := currentRead(addons.TargetAccount{Username: "root", UID: 0})
	read.Truncated = true
	h := &addonReconcileHarness{read: read}
	stubAddonReconcile(t, h)

	res, err := ReconcileAddon(context.Background(), "truenas")
	if err != nil {
		t.Fatalf("ReconcileAddon: %v", err)
	}
	if len(res.Unmanaged) != 1 {
		t.Fatalf("what was seen is still real: %+v", res.Unmanaged)
	}
	if res.Reason != db.UnreconciledTruncated {
		t.Errorf("the cap must be recorded, got %q", res.Reason)
	}
	if h.reconciled {
		t.Error("a capped read is not a reconciliation")
	}
}

// The inventory is a read. Opening the page must not queue anything.
func TestTheInventoryListingQueuesNothing(t *testing.T) {
	h := &addonReconcileHarness{
		read:     currentRead(addons.TargetAccount{Username: "root", UID: 0}),
		bindings: []db.TargetBinding{{Target: "truenas", SubjectID: "u1", Username: "ada"}},
	}
	stubAddonReconcile(t, h)

	res, err := Inventory(context.Background(), "truenas")
	if err != nil {
		t.Fatalf("Inventory: %v", err)
	}
	if len(res.Unmanaged) != 1 {
		t.Fatalf("want the unmanaged account listed: %+v", res.Unmanaged)
	}
	if len(h.converged) != 0 || len(h.planAsked) != 0 {
		t.Error("a listing must neither converge nor ask the target what would change")
	}
	if h.reconciled || len(h.marked) != 0 {
		t.Error("a listing must not record a reconciliation it did not perform")
	}
}

// The anchoring call must skip on a failed READ and on nothing else.
//
// It used to skip when the add-on reported an empty log head as well, which
// meant a log deleted outright — the cheapest tampering there is — produced no
// finding at all. A source guard because the condition is one line and the
// failure it causes is silent for as long as nobody looks.
func TestAnchoringSkipsOnlyWhenTheHealthReadFailed(t *testing.T) {
	src, err := os.ReadFile("addon.go")
	if err != nil {
		t.Fatalf("read source: %v", err)
	}
	guard := regexp.MustCompile(`(?s)health := addonHealth\(ctx, target\)(.*?)\n\t_, verdict, err := anchorLogHead`).
		FindStringSubmatch(string(src))
	if guard == nil {
		t.Fatal("anchorLog's shape changed; if it moved, move this guard with it")
	}
	if regexp.MustCompile(`health\.LogHead\s*==\s*""`).MatchString(guard[1]) {
		t.Error("an empty log head must reach the classifier — against an existing anchor it is a truncation, not a silence")
	}
}
