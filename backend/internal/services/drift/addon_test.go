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
	"syndra/internal/services/merge"
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
	// bases is what the target last reported for each subject. Empty by
	// default, which is the ROLLOUT state — every binding made before the merge
	// base existed has none — so every test written before this mechanism keeps
	// asserting the behaviour a baseless subject must still have.
	bases map[string]db.MergeBase
	// What the pass wrote down: the findings it raised, the ones it closed
	// because their difference was over, and the observations it recorded.
	findingsWritten []db.MergeFinding
	findingsCleared []string
	basesWritten    []db.MergeBase
}

func stubAddonReconcile(t *testing.T, h *addonReconcileHarness) {
	t.Helper()
	origSubjects, origPlan, origBindings := addonSubjects, addonPlan, listBindings
	origRecord, origResolve := recordConvergence, resolveIntent
	origUnrec, origRec := markUnreconciled, markReconciled
	origBases := listMergeBases
	t.Cleanup(func() {
		addonSubjects, addonPlan, listBindings = origSubjects, origPlan, origBindings
		recordConvergence, resolveIntent = origRecord, origResolve
		markUnreconciled, markReconciled = origUnrec, origRec
		listMergeBases = origBases
	})
	origSave, origClear, origBase := saveMergeFinding, clearMergeFinding, saveMergeBase
	t.Cleanup(func() {
		saveMergeFinding, clearMergeFinding, saveMergeBase = origSave, origClear, origBase
	})
	saveMergeFinding = func(_ context.Context, f db.MergeFinding) error {
		h.findingsWritten = append(h.findingsWritten, f)
		return nil
	}
	clearMergeFinding = func(_ context.Context, _, subject, field, _ string) error {
		h.findingsCleared = append(h.findingsCleared, subject+"/"+field)
		return nil
	}
	saveMergeBase = func(_ context.Context, b db.MergeBase) error {
		h.basesWritten = append(h.basesWritten, b)
		return nil
	}
	listMergeBases = func(context.Context, string) (map[string]db.MergeBase, error) {
		if h.bases == nil {
			return map[string]db.MergeBase{}, nil
		}
		return h.bases, nil
	}

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

// §29 — the account behind a refused binding is never offered for adoption,
// and it is worth pinning why rather than adding a guard for it.
//
// `RecordTargetBinding` upserts on (target, subject_id), so the only unique
// indexes it can violate are (target, username) and (target, uid) — which means
// a conflict ALWAYS implies a binding to another subject already exists. That
// binding is what `unmanaged` filters on, so the account is excluded by the
// ordinary path and the adoption hazard §11 names is not reachable this way.
//
// Asserted rather than assumed, because the reasoning depends on the shape of
// the upsert: change it to conflict on something else and the exclusion stops
// following, silently.
func TestAnAccountBoundToAnotherSubjectIsNeverOfferedForAdoption(t *testing.T) {
	uid := int64(3001)
	bindings := []db.TargetBinding{
		{Target: "truenas", SubjectID: "subject-a", Username: "ada", AccountUID: &uid},
	}
	accounts := []addons.TargetAccount{
		{Username: "ada", UID: 3001},
		{Username: "nobody-owns-this", UID: 4002},
	}

	got := unmanaged(accounts, bindings)
	if len(got) != 1 || got[0].Username != "nobody-owns-this" {
		t.Fatalf("an account bound to another subject must not appear as adoptable: %+v", got)
	}

	// The uid arm too: a conflict can be raised by either index, and a binding
	// whose username has drifted still owns the account.
	renamed := []addons.TargetAccount{{Username: "ada-renamed", UID: 3001}}
	if got := unmanaged(renamed, bindings); len(got) != 0 {
		t.Errorf("a bound uid owns the account whatever it is currently called: %+v", got)
	}
}

// A binding whose account is gone from the target is a finding, never a
// convergence.
//
// The plan for one says "create", so queueing it recreates an account somebody
// deleted. This is not hypothetical: three stub-era bindings sat in a live
// deployment pointed at a production NAS, re-queueing every six hours, and the
// only thing that had kept them from landing was an unrelated bug in account
// creation. Fixing that bug turned them into three real accounts waiting for
// somebody to press a button.
func TestABindingWhoseAccountIsGoneIsReportedNotQueued(t *testing.T) {
	present := []addons.TargetAccount{{Username: "ada", UID: 3001}}
	bindings := []db.TargetBinding{
		{SubjectID: "sub-live", Username: "ada", AccountUID: uidPtr(3001)},
		{SubjectID: "sub-gone", Username: "alice", AccountUID: uidPtr(3999)},
	}

	live, stale := partitionByPresence(bindings, present)
	if len(live) != 1 || live[0].SubjectID != "sub-live" {
		t.Fatalf("live = %+v, want only sub-live", live)
	}
	if len(stale) != 1 || stale[0].Username != "alice" || stale[0].UID != 3999 {
		t.Fatalf("stale = %+v, want alice/3999", stale)
	}
}

// A rename keeps the uid; a recreated account keeps the name. Either is still
// the account, and neither is stale.
func TestAMatchOnEitherIdentityCountsAsPresent(t *testing.T) {
	renamed := []addons.TargetAccount{{Username: "ada-smith", UID: 3001}}
	recreated := []addons.TargetAccount{{Username: "ada", UID: 4242}}
	b := []db.TargetBinding{{SubjectID: "s", Username: "ada", AccountUID: uidPtr(3001)}}

	if live, stale := partitionByPresence(b, renamed); len(live) != 1 || len(stale) != 0 {
		t.Error("a renamed account was reported as gone; the uid still matches")
	}
	if live, stale := partitionByPresence(b, recreated); len(live) != 1 || len(stale) != 0 {
		t.Error("a recreated account was reported as gone; the name still matches")
	}
}

// A binding from before uids were recorded must not be condemned for it.
func TestABindingWithNoRecordedUIDMatchesOnName(t *testing.T) {
	b := []db.TargetBinding{{SubjectID: "s", Username: "ada"}}
	live, stale := partitionByPresence(b, []addons.TargetAccount{{Username: "ada", UID: 3001}})
	if len(live) != 1 || len(stale) != 0 {
		t.Fatalf("live=%d stale=%d; a uid-less binding whose name is present is present", len(live), len(stale))
	}
}

func uidPtr(v int64) *int64 { return &v }

// The merge, in the sweep (change `reconciliation-as-merge`).
//
// The classifier's own suite proves the six outcomes. What these prove is the
// consequence: which subjects this pass is willing to ASK the target about, and
// therefore which it can queue a write for. A subject whose difference the pass
// may not resolve is never planned, because planning it produces an `apply`
// effect and the queueing loop acts on those.

func state(fields map[string]any) map[string]json.RawMessage {
	out := map[string]json.RawMessage{}
	for k, v := range fields {
		encoded, err := json.Marshal(v)
		if err != nil {
			panic(err)
		}
		out[k] = encoded
	}
	return out
}

func baseFor(subject string, fields map[string]any) map[string]db.MergeBase {
	return map[string]db.MergeBase{
		subject: {Target: "truenas", SubjectID: subject, Base: state(fields)},
	}
}

// A hand edit on the target. Syndra has not moved, so there is nothing to apply
// — and the previous behaviour, applying anyway, is precisely the silent revert
// this change exists to stop.
func TestAHandEditOnTheTargetIsNotQueuedAndBecomesAFinding(t *testing.T) {
	h := &addonReconcileHarness{
		read: currentRead(addons.TargetAccount{
			Username: "ada", UID: 3001, State: state(map[string]any{"enabled": false}),
		}),
		bindings: []db.TargetBinding{{Target: "truenas", SubjectID: "sub-1", Username: "ada", AccountUID: uid(3001)}},
		// Syndra wants enabled=true (the harness's resolveIntent), and the last
		// thing the target reported was enabled=true. So the target moved.
		bases: baseFor("sub-1", map[string]any{"enabled": true}),
	}
	stubAddonReconcile(t, h)

	res, err := ReconcileAddon(context.Background(), "truenas")
	if err != nil {
		t.Fatal(err)
	}
	if len(h.planAsked) != 0 {
		t.Fatalf("a subject the pass may not resolve must not be planned: %+v", h.planAsked)
	}
	if res.Queued != 0 || len(h.converged) != 0 {
		t.Fatalf("nothing may be queued for a hand edit: queued=%d converged=%+v", res.Queued, h.converged)
	}
	if len(res.Findings) != 1 || res.Findings[0].Outcome != merge.TheirsOnly {
		t.Fatalf("want one theirs_only finding: %+v", res.Findings)
	}
	if res.Findings[0].SubjectID != "sub-1" || res.Findings[0].Field != "enabled" {
		t.Fatalf("the finding must name the subject and the field: %+v", res.Findings[0])
	}
}

// And the ordinary case still converges. Syndra moved, the target did not.
func TestAFastForwardIsStillQueued(t *testing.T) {
	h := &addonReconcileHarness{
		read: currentRead(addons.TargetAccount{
			Username: "ada", UID: 3001, State: state(map[string]any{"enabled": false}),
		}),
		bindings: []db.TargetBinding{{Target: "truenas", SubjectID: "sub-1", Username: "ada", AccountUID: uid(3001)}},
		// The target still holds what it last reported; the desired state moved.
		bases: baseFor("sub-1", map[string]any{"enabled": false}),
		plan: addons.PlanResult{Outcome: addons.OutcomeSucceeded, Outcomes: []addons.SubjectOutcome{
			{Subject: "sub-1", Effect: db.PlanEffectApply},
		}},
	}
	stubAddonReconcile(t, h)

	res, err := ReconcileAddon(context.Background(), "truenas")
	if err != nil {
		t.Fatal(err)
	}
	if len(h.planAsked) != 1 {
		t.Fatalf("a fast-forward must be planned: %+v", h.planAsked)
	}
	if res.Queued != 1 {
		t.Fatalf("a fast-forward must be queued, got %d", res.Queued)
	}
	if len(res.Findings) != 0 {
		t.Fatalf("a fast-forward is not a finding: %+v", res.Findings)
	}
}

// Both moved, differently. The finding carries all three values, because "what
// was it before" is the question an operator asks first.
func TestAConflictIsReportedWithAllThreeValues(t *testing.T) {
	h := &addonReconcileHarness{
		read: currentRead(addons.TargetAccount{
			Username: "ada", UID: 3001, State: state(map[string]any{"enabled": false}),
		}),
		bindings: []db.TargetBinding{{Target: "truenas", SubjectID: "sub-1", Username: "ada", AccountUID: uid(3001)}},
		// Base differs from both: Syndra now wants true, the target now has
		// false, and neither is what was last observed.
		bases: map[string]db.MergeBase{"sub-1": {
			Target: "truenas", SubjectID: "sub-1",
			Base: map[string]json.RawMessage{"enabled": json.RawMessage(`"unknown"`)},
		}},
	}
	stubAddonReconcile(t, h)

	res, err := ReconcileAddon(context.Background(), "truenas")
	if err != nil {
		t.Fatal(err)
	}
	if res.Queued != 0 || len(h.planAsked) != 0 {
		t.Fatal("a conflict is never resolved by an unattended pass")
	}
	if len(res.Findings) != 1 || res.Findings[0].Outcome != merge.Conflict {
		t.Fatalf("want one conflict: %+v", res.Findings)
	}
	f := res.Findings[0]
	if len(f.Base) == 0 || len(f.Ours) == 0 || len(f.Theirs) == 0 {
		t.Fatalf("a conflict must carry all three values: %+v", f)
	}
}

// The account is gone. Reported as an account-level finding beside the stale
// binding it already produced — same state, named in the merge's vocabulary.
func TestAnAbsentAccountIsReportedAsDeletedUpstream(t *testing.T) {
	h := &addonReconcileHarness{
		read:     currentRead(addons.TargetAccount{Username: "someone-else", UID: 4000}),
		bindings: []db.TargetBinding{{Target: "truenas", SubjectID: "sub-1", Username: "ada", AccountUID: uid(3001)}},
	}
	stubAddonReconcile(t, h)

	res, err := ReconcileAddon(context.Background(), "truenas")
	if err != nil {
		t.Fatal(err)
	}
	if res.Queued != 0 {
		t.Fatal("an absent account must never be queued — the plan for one says create")
	}
	if len(res.Findings) != 1 || res.Findings[0].Outcome != merge.DeletedUpstream {
		t.Fatalf("want one deleted_upstream finding: %+v", res.Findings)
	}
	if res.Findings[0].Field != "" {
		t.Errorf("deleted upstream is about the account, not a field: %+v", res.Findings[0])
	}
	// The existing surface keeps its row. Two vocabularies for one state is a
	// migration, not a rename.
	if len(res.Stale) != 1 {
		t.Errorf("the stale binding must still be reported: %+v", res.Stale)
	}
}

// The rollout rule, at the sweep level: a binding made before any base existed
// converges exactly as it did, and raises nothing.
func TestABaselessSubjectConvergesAndRaisesNothing(t *testing.T) {
	h := &addonReconcileHarness{
		read: currentRead(addons.TargetAccount{
			Username: "ada", UID: 3001, State: state(map[string]any{"enabled": false}),
		}),
		bindings: []db.TargetBinding{{Target: "truenas", SubjectID: "sub-1", Username: "ada", AccountUID: uid(3001)}},
		plan: addons.PlanResult{Outcome: addons.OutcomeSucceeded, Outcomes: []addons.SubjectOutcome{
			{Subject: "sub-1", Effect: db.PlanEffectApply},
		}},
	}
	stubAddonReconcile(t, h)

	res, err := ReconcileAddon(context.Background(), "truenas")
	if err != nil {
		t.Fatal(err)
	}
	if res.Queued != 1 {
		t.Fatalf("no base means converge as before, got %d queued", res.Queued)
	}
	if len(res.Findings) != 0 {
		t.Fatalf("no base means no cause, which is not a finding: %+v", res.Findings)
	}
}

// An add-on too old to report state leaves every subject with unknown current
// values. That must degrade to the pre-merge behaviour rather than to a target
// that looks emptied — the same failure as concluding an absence from a read
// that did not happen.
func TestAnAddOnThatReportsNoStateStillConverges(t *testing.T) {
	h := &addonReconcileHarness{
		read:     currentRead(addons.TargetAccount{Username: "ada", UID: 3001}),
		bindings: []db.TargetBinding{{Target: "truenas", SubjectID: "sub-1", Username: "ada", AccountUID: uid(3001)}},
		bases:    baseFor("sub-1", map[string]any{"enabled": true}),
		plan: addons.PlanResult{Outcome: addons.OutcomeSucceeded, Outcomes: []addons.SubjectOutcome{
			{Subject: "sub-1", Effect: db.PlanEffectApply},
		}},
	}
	stubAddonReconcile(t, h)

	res, err := ReconcileAddon(context.Background(), "truenas")
	if err != nil {
		t.Fatal(err)
	}
	if res.Queued != 1 {
		t.Fatalf("an add-on that reports no state must converge as before, got %d", res.Queued)
	}
}

// Durable, or it is not a finding.
//
// The state most likely to have been left as sweep output is the one that
// occurs most: `theirs_only` is what a hand edit on the target looks like, and
// as a return value it is visible to whoever ran the pass and to nobody else.
func TestAFindingOutlivesThePassThatFoundIt(t *testing.T) {
	h := &addonReconcileHarness{
		read: currentRead(addons.TargetAccount{
			Username: "ada", UID: 3001, State: state(map[string]any{"enabled": false}),
		}),
		bindings: []db.TargetBinding{{Target: "truenas", SubjectID: "sub-1", Username: "ada", AccountUID: uid(3001)}},
		bases:    baseFor("sub-1", map[string]any{"enabled": true}),
	}
	stubAddonReconcile(t, h)

	if _, err := ReconcileAddon(context.Background(), "truenas"); err != nil {
		t.Fatal(err)
	}
	if len(h.findingsWritten) != 1 {
		t.Fatalf("want one persisted finding: %+v", h.findingsWritten)
	}
	f := h.findingsWritten[0]
	if f.SubjectID != "sub-1" || f.Field != "enabled" || f.Outcome != string(merge.TheirsOnly) {
		t.Fatalf("the finding must carry what it is about: %+v", f)
	}
	if len(f.Base) == 0 || len(f.Ours) == 0 || len(f.Theirs) == 0 {
		t.Fatalf("all three values travel with it: %+v", f)
	}
	// And nothing is observed for a subject with an outstanding difference.
	// Advancing the base here would make the next pass read the target's
	// current state as the last agreed one and revert the edit.
	if len(h.basesWritten) != 0 {
		t.Fatalf("a base must not advance past an unresolved finding: %+v", h.basesWritten)
	}
}

// A difference that has stopped existing stops being a finding. Nothing is
// decided and nothing is written to the target — the disagreement is simply
// over, and a queue full of problems that are already finished is as unreadable
// as one full of noise.
func TestASettledDifferenceClosesItsFinding(t *testing.T) {
	h := &addonReconcileHarness{
		read: currentRead(addons.TargetAccount{
			Username: "ada", UID: 3001, State: state(map[string]any{"enabled": true}),
		}),
		bindings: []db.TargetBinding{{Target: "truenas", SubjectID: "sub-1", Username: "ada", AccountUID: uid(3001)}},
		bases:    baseFor("sub-1", map[string]any{"enabled": true}),
	}
	stubAddonReconcile(t, h)

	if _, err := ReconcileAddon(context.Background(), "truenas"); err != nil {
		t.Fatal(err)
	}
	if len(h.findingsWritten) != 0 {
		t.Fatalf("agreement is not a finding: %+v", h.findingsWritten)
	}
	// Two closes: the field whose difference is over, and the account-level slot
	// — the account is present, so anything that said it was gone is over too.
	if !contains(h.findingsCleared, "sub-1/enabled") {
		t.Fatalf("a settled difference must close whatever was standing: %v", h.findingsCleared)
	}
	if !contains(h.findingsCleared, "sub-1/") {
		t.Fatalf("a present account must close a standing deleted-upstream: %v", h.findingsCleared)
	}
}

// `already merged` writes nothing to the target, so nothing else would ever
// record it. Without the sweep observing it, a hand-made change that matched
// Syndra's intent would be re-detected as an agreement on every pass forever.
func TestAnAgreementIsObservedSoItIsNotRediscoveredForever(t *testing.T) {
	h := &addonReconcileHarness{
		read: currentRead(addons.TargetAccount{
			Username: "ada", UID: 3001, State: state(map[string]any{"enabled": true}),
		}),
		bindings: []db.TargetBinding{{Target: "truenas", SubjectID: "sub-1", Username: "ada", AccountUID: uid(3001)}},
		// The last observation disagrees with both sides, which now agree.
		bases: baseFor("sub-1", map[string]any{"enabled": false}),
	}
	stubAddonReconcile(t, h)

	if _, err := ReconcileAddon(context.Background(), "truenas"); err != nil {
		t.Fatal(err)
	}
	if len(h.findingsWritten) != 0 {
		t.Fatalf("somebody who made the change Syndra wanted has not drifted: %+v", h.findingsWritten)
	}
	if len(h.basesWritten) != 1 {
		t.Fatalf("the agreement must be recorded: %+v", h.basesWritten)
	}
	if string(h.basesWritten[0].Base["enabled"]) != "true" {
		t.Fatalf("the base must hold what the target reported: %v", h.basesWritten[0].Base)
	}
}

func contains(all []string, want string) bool {
	for _, v := range all {
		if v == want {
			return true
		}
	}
	return false
}

// `deleted_upstream` is a finding like the others, and it was the one left as
// sweep output: returned in the response and written nowhere, so it lived
// exactly as long as the request that carried it — gone on refresh, absent from
// the decision queue, uncounted by governance.
func TestAnAbsentAccountsFindingIsPersisted(t *testing.T) {
	h := &addonReconcileHarness{
		read:     currentRead(addons.TargetAccount{Username: "someone-else", UID: 4000}),
		bindings: []db.TargetBinding{{Target: "truenas", SubjectID: "sub-1", Username: "ada", AccountUID: uid(3001)}},
	}
	stubAddonReconcile(t, h)

	if _, err := ReconcileAddon(context.Background(), "truenas"); err != nil {
		t.Fatal(err)
	}
	if len(h.findingsWritten) != 1 {
		t.Fatalf("want one persisted finding: %+v", h.findingsWritten)
	}
	f := h.findingsWritten[0]
	if f.Outcome != string(merge.DeletedUpstream) || f.SubjectID != "sub-1" || f.Field != "" {
		t.Fatalf("want an account-level deleted_upstream row: %+v", f)
	}
}

// A pass that could not write down what it found has not reconciled the target,
// whatever its read managed. The failures were logged and the target was then
// marked reconciled — so the surface reported a clean pass over findings nobody
// would ever see.
func TestAFindingThatCouldNotBeWrittenLeavesTheTargetUnreconciled(t *testing.T) {
	h := &addonReconcileHarness{
		read: currentRead(addons.TargetAccount{
			Username: "ada", UID: 3001, State: state(map[string]any{"enabled": false}),
		}),
		bindings: []db.TargetBinding{{Target: "truenas", SubjectID: "sub-1", Username: "ada", AccountUID: uid(3001)}},
		bases:    baseFor("sub-1", map[string]any{"enabled": true}),
	}
	stubAddonReconcile(t, h)
	saveMergeFinding = func(context.Context, db.MergeFinding) error {
		return errors.New("the database went away")
	}

	res, err := ReconcileAddon(context.Background(), "truenas")
	if err != nil {
		t.Fatal(err)
	}
	if h.reconciled {
		t.Fatal("a pass that lost a finding must not claim it read the target cleanly")
	}
	if res.Reason != db.UnreconciledFindingsUnrecorded {
		t.Fatalf("want the findings-unrecorded reason, got %q", res.Reason)
	}
}
