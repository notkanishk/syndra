package drift

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"syndra/internal/addons"
	"syndra/internal/db"
	"syndra/internal/services"
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
	// The unbound half: mappings on the target, who holds each role, and who
	// already has a convergence waiting. Empty by default, so every earlier
	// test sees a target with nobody unbound.
	mappings []db.RoleMapping
	holders  map[string][]string
	waiting  map[string]bool
	// waitingFor is what a waiting row wants, as JSON; absent means the same
	// as this pass resolves.
	waitingFor map[string]string
	// askedFresh is each subjects read's fresh, in order.
	askedFresh []bool
	// reads answers the subjects reads in order, then read; waited is each
	// cooldown the pass waited out (never slept).
	reads  []addons.SubjectsResult
	waited []time.Duration
	// inFlight is who had an apply in flight since the read began, and
	// inFlightSince what the pass asked it from.
	inFlight       map[string]bool
	inFlightSince  time.Time
	inFlightWindow time.Duration
	// closedOutside is each whole-read close of the conflicts a read no longer
	// lists, as "outcome:kept,subjects".
	closedOutside []string
}

// dbClock is the database's clock in these tests, apart from this process's.
var dbClock = time.Date(2026, 11, 15, 9, 0, 0, 0, time.UTC)

func stubAddonReconcile(t *testing.T, h *addonReconcileHarness) {
	t.Helper()
	origSubjects, origPlan, origBindings := addonSubjects, addonPlan, listBindings
	origRecord, origResolve := recordConvergence, resolveIntent
	origUnrec, origRec := markUnreconciled, markReconciled
	origBases := listMergeBases
	origMappings, origHolders, origPending := listMappings, mappingHolders, pendingApply
	t.Cleanup(func() { listMappings, mappingHolders, pendingApply = origMappings, origHolders, origPending })
	origInFlight, origNow, origOutside := appliesInFlight, databaseNow, clearFindingsOutside
	t.Cleanup(func() { appliesInFlight, databaseNow, clearFindingsOutside = origInFlight, origNow, origOutside })
	databaseNow = func(context.Context) (time.Time, error) { return dbClock, nil }
	appliesInFlight = func(_ context.Context, _ string, since time.Time, window time.Duration) (map[string]bool, error) {
		h.inFlightSince, h.inFlightWindow = since, window
		return h.inFlight, nil
	}
	clearFindingsOutside = func(_ context.Context, _, outcome string, keep []string, _ string) (int64, error) {
		h.closedOutside = append(h.closedOutside, outcome+":"+strings.Join(keep, ","))
		return 0, nil
	}
	listMappings = func(context.Context, string) ([]db.RoleMapping, error) { return h.mappings, nil }
	mappingHolders = func(_ context.Context, project, role string) ([]string, error) {
		return h.holders[project+"/"+role], nil
	}
	pendingApply = func(_ context.Context, _, user string, desired map[string]json.RawMessage) (bool, error) {
		if want, ok := h.waitingFor[user]; ok {
			got, _ := json.Marshal(desired)
			return h.waiting[user] && string(got) == want, nil
		}
		return h.waiting[user], nil
	}
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

	addonSubjects = func(_ context.Context, _ string, fresh bool) addons.SubjectsResult {
		h.askedFresh = append(h.askedFresh, fresh)
		if len(h.reads) > 0 {
			r := h.reads[0]
			h.reads = h.reads[1:]
			return r
		}
		return h.read
	}
	origCooldown, origWait := freshCooldown, waitOut
	t.Cleanup(func() { freshCooldown, waitOut = origCooldown, origWait })
	freshCooldown = func(string) time.Duration { return 30 * time.Second }
	waitOut = func(_ context.Context, d time.Duration) bool {
		h.waited = append(h.waited, d)
		return true
	}
	listBindings = func(context.Context, string) ([]db.TargetBinding, error) { return h.bindings, nil }
	origRoles := fieldRoles
	t.Cleanup(func() { fieldRoles = origRoles })
	fieldRoles = func(context.Context, string) (addons.Roles, error) { return truenasRoles, nil }
	origSchema := entitlementSchema
	t.Cleanup(func() { entitlementSchema = origSchema })
	entitlementSchema = func(string) ([]addons.EntitlementField, error) { return truenasSchema, nil }
	addonPlan = func(_ context.Context, _ string, subjects []addons.PlanSubject) addons.PlanResult {
		h.planAsked = subjects
		return h.plan
	}
	resolveIntent = func(_ context.Context, subjectID, _ string) (services.EntitlementSet, error) {
		return services.EntitlementSet{Roles: truenasRoles, Lifecycle: services.LifecycleState{Account: true, Channels: map[string]bool{"smb_enabled": true}}}, nil
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

// truenasRoles is the TrueNAS add-on's role map, as targets.field_roles holds it.
var truenasRoles = addons.Roles{"group": "membership", "home": "flag", "home_quota": "limit", "enabled": "account", "smb_enabled": "channel"}

// truenasSchema is how its limit combines: max, "0" unlimited.
var truenasSchema = []addons.EntitlementField{
	{Name: "group", Role: addons.RoleMembership}, {Name: "home", Role: addons.RoleFlag},
	{Name: "home_quota", Role: addons.RoleLimit, Unit: "bytes", Combine: "max", Unlimited: "0"},
	{Name: "enabled", Role: addons.RoleAccount}, {Name: "smb_enabled", Role: addons.RoleChannel},
}

func uid(n int64) *string { s := strconv.FormatInt(n, 10); return &s }

// The scenario the requirement is written for: a target full of accounts nobody
// provisioned, seen for the first time.
func TestAFirstSweepRaisesNoDriftAndReportsInventory(t *testing.T) {
	h := &addonReconcileHarness{
		read: currentRead(
			addons.TargetAccount{Username: "root", AccountID: "0"},
			addons.TargetAccount{Username: "backup-svc", AccountID: "3100"},
			addons.TargetAccount{Username: "someone-elses", AccountID: "3101"},
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
			addons.TargetAccount{Username: "charlotte", AccountID: "3001", Subject: "u1"},
			addons.TargetAccount{Username: "root", AccountID: "0"},
		),
		bindings: []db.TargetBinding{{Target: "truenas", SubjectID: "u1", Username: "charlotte", AccountID: uid(3001)}},
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
		read:     currentRead(addons.TargetAccount{Username: "charlotte", AccountID: "3001", Subject: "u1"}),
		bindings: []db.TargetBinding{{Target: "truenas", SubjectID: "u1", Username: "charlotte", AccountID: uid(3001)}},
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
		read:     currentRead(addons.TargetAccount{Username: "charlotte-flair", AccountID: "3001", Subject: "u1"}),
		bindings: []db.TargetBinding{{Target: "truenas", SubjectID: "u1", Username: "charlotte", AccountID: uid(3001)}},
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
			Accounts: []addons.TargetAccount{{Username: "charlotte", AccountID: "3001"}},
			Current:  false, Outcome: addons.OutcomeSucceeded,
		},
		bindings: []db.TargetBinding{{Target: "truenas", SubjectID: "u1", Username: "charlotte"}},
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

// An inventory that halted before reading says nothing it did not read: no
// count, no lists, no date (holding is observed). Staging showed bound 0 with
// seven bindings recorded, and read_at 0001-01-01, rendered "1 Jan 1". A read
// that found nothing still says so: 0 and empty lists, with its time.
func TestAHaltedInventoryCarriesNoCountListOrDateItDidNotRead(t *testing.T) {
	keys := func(res AddonReconcileResult) map[string]json.RawMessage { return jsonKeys(t, res) }
	stubAddonReconcile(t, &addonReconcileHarness{
		read:     addons.SubjectsResult{Outcome: addons.OutcomeUnreached, Err: errors.New("connection refused")},
		bindings: []db.TargetBinding{{Target: "truenas", SubjectID: "u1", Username: "charlotte"}},
	})
	res, err := Inventory(context.Background(), "truenas", false)
	if err != nil || !res.Halted {
		t.Fatalf("want a halted inventory: %+v %v", res, err)
	}
	for k, v := range keys(res) {
		if k == "bound" || k == "accounts" || k == "unmanaged" || k == "read_at" {
			t.Errorf("a halted inventory carries %s=%s; it read nothing", k, v)
		}
	}

	stubAddonReconcile(t, &addonReconcileHarness{read: currentRead()})
	res, err = Inventory(context.Background(), "truenas", false)
	if err != nil || res.Halted {
		t.Fatalf("want a read inventory: %+v %v", res, err)
	}
	m := keys(res)
	for k, want := range map[string]string{"bound": "0", "accounts": "[]", "unmanaged": "[]"} {
		if string(m[k]) != want {
			t.Errorf("a read that found nothing: %s=%s, want %s", k, m[k], want)
		}
	}
	if _, ok := m["read_at"]; !ok {
		t.Error("a read inventory says when it was read")
	}
}

// jsonKeys is a result as the API sends it, by key: what is absent is absent.
func jsonKeys(t *testing.T, res AddonReconcileResult) map[string]json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(res)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

// A reconcile pass that halted (halt()) counted nothing and concluded nothing:
// no bound, no stale, no date, so the card never reads "0 people with an
// account missing" off a pass that never looked.
func TestAHaltedReconcileCarriesNoCountItDidNotTake(t *testing.T) {
	stubAddonReconcile(t, &addonReconcileHarness{
		read:     addons.SubjectsResult{Outcome: addons.OutcomeUnreached, Err: errors.New("connection refused")},
		bindings: []db.TargetBinding{{Target: "truenas", SubjectID: "u1", Username: "charlotte"}},
	})
	res, err := ReconcileAddon(context.Background(), "truenas")
	if err != nil || !res.Halted {
		t.Fatalf("want a halted pass: %+v %v", res, err)
	}
	for _, k := range []string{"bound", "stale", "unmanaged", "read_at"} {
		if v, ok := jsonKeys(t, res)[k]; ok {
			t.Errorf("a halted pass carries %s=%s; it counted nothing", k, v)
		}
	}
}

// Absence is concluded only from a whole read: a whole one that found nobody
// missing sends stale [], a truncated one sends no stale at all, so "none
// missing" and "not concluded" never look alike.
func TestStaleIsEmptyOnAWholeReadAndAbsentOnATruncatedOne(t *testing.T) {
	for _, truncated := range []bool{false, true} {
		read := currentRead(addons.TargetAccount{Username: "root", AccountID: "0"})
		read.Truncated = truncated
		stubAddonReconcile(t, &addonReconcileHarness{read: read})
		res, err := ReconcileAddon(context.Background(), "truenas")
		if err != nil || res.Halted {
			t.Fatalf("truncated=%v: want a pass: %+v %v", truncated, res, err)
		}
		got, ok := jsonKeys(t, res)["stale"]
		switch {
		case !truncated && string(got) != "[]":
			t.Errorf("a whole read that found none missing: stale=%s, want []", got)
		case truncated && ok:
			t.Errorf("a truncated read concluded an absence: stale=%s", got)
		}
	}
}

// A truncated read still supports the inventory — everything in it was seen —
// and the pass says the picture is incomplete rather than pretending otherwise.
func TestATruncatedReadStillReportsWhatItSaw(t *testing.T) {
	read := currentRead(addons.TargetAccount{Username: "root", AccountID: "0"})
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
		read:     currentRead(addons.TargetAccount{Username: "root", AccountID: "0"}),
		bindings: []db.TargetBinding{{Target: "truenas", SubjectID: "u1", Username: "charlotte"}},
	}
	stubAddonReconcile(t, h)

	res, err := Inventory(context.Background(), "truenas", false)
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

// Which accounts are managed is the add-on's to say (design 7t D3): a row it
// attributes to a subject is bound, whatever it is called now, and a row it
// attributes to nobody is unmanaged even where its name and number are ones a
// recorded binding holds. That second case is TrueNAS handing a freed number
// to a new account: matching on the backend's copy would hide a stranger's
// account behind a member's binding.
func TestOnlyTheAddOnsAttributionMakesAnAccountManaged(t *testing.T) {
	accounts := []addons.TargetAccount{
		{Username: "charlotte-renamed", AccountID: "3001", Subject: "subject-a"},
		{Username: "becky", AccountID: "3002"},
		{Username: "nobody-owns-this", AccountID: "4002"},
	}
	got := unmanaged(accounts)
	if len(got) != 2 || got[0].Username != "becky" || got[1].Username != "nobody-owns-this" {
		t.Fatalf("want exactly the rows attributed to nobody, got %+v", got)
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
	present := []addons.TargetAccount{{Username: "charlotte", AccountID: "3001", Subject: "sub-live"}}
	bindings := []db.TargetBinding{
		{SubjectID: "sub-live", Username: "charlotte", AccountID: uidPtr(3001)},
		{SubjectID: "sub-gone", Username: "becky", AccountID: uidPtr(3999)},
	}

	live, stale := partitionByPresence(bindings, present, nil, nil)
	if len(live) != 1 || live[0].SubjectID != "sub-live" {
		t.Fatalf("live = %+v, want only sub-live", live)
	}
	if len(stale) != 1 || stale[0].Username != "becky" || *stale[0].AccountID != "3999" {
		t.Fatalf("stale = %+v, want becky/3999", stale)
	}
}

// Presence is the add-on's attribution, never this side's match (AUTHORING
// ch. 2): an account renamed and renumbered is still the member's when the
// add-on says so, and one holding the recorded name and number is not theirs
// when it does not.
func TestPresenceIsBySubjectNeverByNameOrNumber(t *testing.T) {
	b := []db.TargetBinding{{SubjectID: "s", Username: "charlotte", AccountID: uidPtr(3001)}}

	moved := []addons.TargetAccount{{Username: "charlotte-smith", AccountID: "4242", Subject: "s"}}
	if live, stale := partitionByPresence(b, moved, nil, nil); len(live) != 1 || len(stale) != 0 {
		t.Error("the add-on attributes the account to the subject; it is present whatever it is called")
	}
	reused := []addons.TargetAccount{{Username: "charlotte", AccountID: "3001"}}
	if live, stale := partitionByPresence(b, reused, nil, nil); len(live) != 0 || len(stale) != 1 {
		t.Error("an account the add-on attributes to nobody is not the member's, whatever name and number it holds")
	}
	// The stale row carries the version of the record it was read from, for the page's release.
	if _, stale := partitionByPresence(b, reused, nil, nil); stale[0].Version != b[0].Version() || stale[0].Version == "" {
		t.Errorf("a stale binding names its record's version: %+v", stale)
	}
}

// A conflicted subject and one with an apply in flight are neither live nor
// stale: the first is its own finding, the second unknown to this read.
func TestAConflictOrAnApplyInFlightIsNeverLiveOrStale(t *testing.T) {
	b := []db.TargetBinding{{SubjectID: "held"}, {SubjectID: "landing"}, {SubjectID: "gone"}}
	live, stale := partitionByPresence(b, nil, map[string]bool{"held": true}, map[string]bool{"landing": true})
	if len(live) != 0 || len(stale) != 1 || stale[0].SubjectID != "gone" {
		t.Fatalf("live=%+v stale=%+v, want only gone stale", live, stale)
	}
}

func uidPtr(v int64) *string { return uid(v) }

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
			Username: "charlotte", AccountID: "3001", Subject: "sub-1", State: state(map[string]any{"enabled": false}),
		}),
		bindings: []db.TargetBinding{{Target: "truenas", SubjectID: "sub-1", Username: "charlotte", AccountID: uid(3001)}},
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
			Username: "charlotte", AccountID: "3001", Subject: "sub-1", State: state(map[string]any{"enabled": false}),
		}),
		bindings: []db.TargetBinding{{Target: "truenas", SubjectID: "sub-1", Username: "charlotte", AccountID: uid(3001)}},
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
			Username: "charlotte", AccountID: "3001", Subject: "sub-1", State: state(map[string]any{"enabled": false}),
		}),
		bindings: []db.TargetBinding{{Target: "truenas", SubjectID: "sub-1", Username: "charlotte", AccountID: uid(3001)}},
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
		read:     currentRead(addons.TargetAccount{Username: "someone-else", AccountID: "4000"}),
		bindings: []db.TargetBinding{{Target: "truenas", SubjectID: "sub-1", Username: "charlotte", AccountID: uid(3001)}},
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

// The same missing account on a TRUNCATED read: it may be past the cap, so no
// absence finding is drawn and nothing is queued for it.
func TestATruncatedReadDrawsNoAbsenceFindings(t *testing.T) {
	read := currentRead(addons.TargetAccount{Username: "someone-else", AccountID: "4000"})
	read.Truncated = true
	h := &addonReconcileHarness{
		read:     read,
		bindings: []db.TargetBinding{{Target: "truenas", SubjectID: "sub-1", Username: "charlotte", AccountID: uid(3001)}},
	}
	stubAddonReconcile(t, h)

	res, err := ReconcileAddon(context.Background(), "truenas")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Findings) != 0 || len(res.Stale) != 0 || len(h.findingsWritten) != 0 {
		t.Fatalf("a binding missing from a capped read is not known to be absent: %+v %+v %+v",
			res.Findings, res.Stale, h.findingsWritten)
	}
	if res.Queued != 0 {
		t.Fatal("and nothing is queued for it")
	}
}

// The rollout rule, at the sweep level: a binding made before any base existed
// converges exactly as it did, and raises nothing.
func TestABaselessSubjectConvergesAndRaisesNothing(t *testing.T) {
	h := &addonReconcileHarness{
		read: currentRead(addons.TargetAccount{
			Username: "charlotte", AccountID: "3001", Subject: "sub-1", State: state(map[string]any{"enabled": false}),
		}),
		bindings: []db.TargetBinding{{Target: "truenas", SubjectID: "sub-1", Username: "charlotte", AccountID: uid(3001)}},
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
		read:     currentRead(addons.TargetAccount{Username: "charlotte", AccountID: "3001", Subject: "sub-1"}),
		bindings: []db.TargetBinding{{Target: "truenas", SubjectID: "sub-1", Username: "charlotte", AccountID: uid(3001)}},
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
			Username: "charlotte", AccountID: "3001", Subject: "sub-1", State: state(map[string]any{"enabled": false}),
		}),
		bindings: []db.TargetBinding{{Target: "truenas", SubjectID: "sub-1", Username: "charlotte", AccountID: uid(3001)}},
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
			Username: "charlotte", AccountID: "3001", Subject: "sub-1", State: state(map[string]any{"enabled": true}),
		}),
		bindings: []db.TargetBinding{{Target: "truenas", SubjectID: "sub-1", Username: "charlotte", AccountID: uid(3001)}},
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
			Username: "charlotte", AccountID: "3001", Subject: "sub-1", State: state(map[string]any{"enabled": true}),
		}),
		bindings: []db.TargetBinding{{Target: "truenas", SubjectID: "sub-1", Username: "charlotte", AccountID: uid(3001)}},
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
	if h.basesWritten[0].Partial {
		t.Fatal("an ordinary intent observes the whole managed set: its base replaces the last")
	}
}

// A departed person already locked by hand: the sweep records the agreement,
// and since the intent is the switch alone, Partial, so the groups the base
// held before they left stand for their return (db.MergeBase.Partial).
func TestADepartedPersonsObservationKeepsTheRestOfTheBase(t *testing.T) {
	h := &addonReconcileHarness{
		read: currentRead(addons.TargetAccount{
			Username: "charlotte", AccountID: "3001", Subject: "sub-1", State: state(map[string]any{"enabled": false}),
		}),
		bindings: []db.TargetBinding{{Target: "truenas", SubjectID: "sub-1", Username: "charlotte", AccountID: uid(3001)}},
		bases:    baseFor("sub-1", map[string]any{"enabled": true, "group": []string{"lab_makers"}}),
	}
	stubAddonReconcile(t, h)
	resolveIntent = func(context.Context, string, string) (services.EntitlementSet, error) {
		return services.EntitlementSet{Roles: truenasRoles,
			Fields:    map[string][]string{"group": {"lab_makers"}},
			Lifecycle: services.LifecycleState{NotAMember: true},
		}, nil
	}

	if _, err := ReconcileAddon(context.Background(), "truenas"); err != nil {
		t.Fatal(err)
	}
	if len(h.basesWritten) != 1 || !h.basesWritten[0].Partial {
		t.Fatalf("want one Partial observation, got %+v", h.basesWritten)
	}
	if _, has := h.basesWritten[0].Base["group"]; has {
		t.Fatalf("a withdraw-only intent manages the lifecycle fields alone: %v", h.basesWritten[0].Base)
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
		read:     currentRead(addons.TargetAccount{Username: "someone-else", AccountID: "4000"}),
		bindings: []db.TargetBinding{{Target: "truenas", SubjectID: "sub-1", Username: "charlotte", AccountID: uid(3001)}},
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
			Username: "charlotte", AccountID: "3001", Subject: "sub-1", State: state(map[string]any{"enabled": false}),
		}),
		bindings: []db.TargetBinding{{Target: "truenas", SubjectID: "sub-1", Username: "charlotte", AccountID: uid(3001)}},
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

// A person the directory reports gone: the sweep's disable is declared
// withdraw-only, so the background runner sends it instead of leaving it for
// an operator. A lock because no mapped role reaches them is not: it waits.
func TestADepartedPersonsDisableDrainsOnItsOwn(t *testing.T) {
	// Twice: TrueNAS, and a target whose account is `active` (addon-contract
	// 2.1): the rule is by role, so the drain cannot depend on a name.
	ruby := addons.Roles{"group": addons.RoleMembership, "active": addons.RoleAccount}
	for _, shape := range []struct {
		roles   addons.Roles
		account string
	}{{truenasRoles, "enabled"}, {ruby, "active"}} {
		for _, tc := range []struct {
			name string
			set  services.EntitlementSet
			want bool
		}{
			{"the directory reports them gone", services.EntitlementSet{Roles: shape.roles,
				Fields: map[string][]string{"group": {"lab_makers"}}, Lifecycle: services.LifecycleState{NotAMember: true}}, true},
			{"no mapped role reaches them", services.EntitlementSet{Roles: shape.roles, Fields: map[string][]string{"group": nil}}, false},
		} {
			t.Run(shape.account+"/"+tc.name, func(t *testing.T) {
				h := &addonReconcileHarness{
					read:     currentRead(addons.TargetAccount{Username: "charlotte", AccountID: "3001", Subject: "u1"}),
					bindings: []db.TargetBinding{{Target: "truenas", SubjectID: "u1", Username: "charlotte", AccountID: uid(3001)}},
					plan: addons.PlanResult{
						Outcome: addons.OutcomeSucceeded, Current: true,
						Outcomes: []addons.SubjectOutcome{{Subject: "u1", Effect: db.PlanEffectApply}},
					},
				}
				stubAddonReconcile(t, h)
				fieldRoles = func(context.Context, string) (addons.Roles, error) { return shape.roles, nil }
				resolveIntent = func(context.Context, string, string) (services.EntitlementSet, error) { return tc.set, nil }

				if _, err := ReconcileAddon(context.Background(), "truenas"); err != nil {
					t.Fatal(err)
				}
				if len(h.converged) != 1 {
					t.Fatalf("want one convergence, got %+v", h.converged)
				}
				c := h.converged[0]
				if c.WithdrawsOnly != tc.want || string(c.Desired[shape.account]) != "false" {
					t.Fatalf("queued WithdrawsOnly=%v %s=%s, want %v and false", c.WithdrawsOnly, shape.account, c.Desired[shape.account], tc.want)
				}
				// A withdrawal goes unreviewed, so it carries the switch alone; an
				// ordinary lock still says what to do with the groups.
				if _, carriesGroups := c.Desired["group"]; carriesGroups == tc.want {
					t.Fatalf("withdraw-only=%v but the row carries group=%v: %v", tc.want, carriesGroups, c.Desired)
				}
				// And what the target was asked to plan is the row it will run.
				if len(h.planAsked) != 1 || len(h.planAsked[0].Desired) != len(c.Desired) {
					t.Fatalf("planned %v, queued %v: the plan must be of the instruction queued", h.planAsked, c.Desired)
				}
			})
		}
	}
}

// A limit is compared the way the add-on combines it (task 2.7): the intent's
// values become one effective limit (max, "0" unlimited and dominant) beside
// the effective limit the target reports. A limit nothing sets, one the target
// did not report (a quota table it could not read) and one whose rule is not
// known are not compared at all: unknown, never theirs_only.
func TestALimitIsComparedAsTheAddOnCombinesIt(t *testing.T) {
	raw := func(v any) json.RawMessage { b, _ := json.Marshal(v); return b }
	intent := map[string]json.RawMessage{"enabled": raw(true), "group": raw([]string{"staff"})}
	reported := map[string]json.RawMessage{"home_quota": raw([]string{"0"})}
	for _, tc := range []struct {
		name   string
		values []string
		theirs map[string]json.RawMessage
		schema []addons.EntitlementField
		want   string // "" is not compared
	}{
		{"the largest of two limits", []string{"1073741824", "5368709120"}, reported, truenasSchema, `["5368709120"]`},
		{"unlimited dominates", []string{"5368709120", "0"}, reported, truenasSchema, `["0"]`},
		{"one person's own limit", []string{"10737418240"}, reported, truenasSchema, `["10737418240"]`},
		{"nothing sets a limit", []string{}, reported, truenasSchema, ""},
		{"the target could not read it", []string{"5368709120"}, map[string]json.RawMessage{}, truenasSchema, ""},
		{"no accepted schema says how it combines", []string{"5368709120"}, reported, nil, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := maps.Clone(intent)
			in["home_quota"] = raw(tc.values)
			got := comparable(in, tc.theirs, truenasRoles, tc.schema)
			if string(got["enabled"]) != "true" || string(got["group"]) != `["staff"]` {
				t.Fatalf("every other field is the intent's own: %v", got)
			}
			if limit, compared := got["home_quota"]; string(limit) != tc.want || compared != (tc.want != "") {
				t.Fatalf("home_quota compared as %s (compared=%v), want %q", limit, compared, tc.want)
			}
		})
	}
}

// A quota edited by hand on the target is a finding, never a silent revert
// (task 2.7): the base saw the roles' 5 GiB, the target now holds 10 GiB.
// And a quota the add-on could not read raises nothing, whatever the base.
func TestAHandEditedLimitIsAFinding(t *testing.T) {
	for _, tc := range []struct {
		name    string
		theirs  map[string]any
		finding bool
	}{
		{"edited by hand", map[string]any{"enabled": true, "home_quota": []string{"10737418240"}}, true},
		{"unchanged", map[string]any{"enabled": true, "home_quota": []string{"5368709120"}}, false},
		{"the quota table could not be read", map[string]any{"enabled": true}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := &addonReconcileHarness{
				read: currentRead(addons.TargetAccount{
					Username: "charlotte", AccountID: "3001", Subject: "sub-1", State: state(tc.theirs),
				}),
				bindings: []db.TargetBinding{{Target: "truenas", SubjectID: "sub-1", Username: "charlotte", AccountID: uid(3001)}},
				bases:    baseFor("sub-1", map[string]any{"enabled": true, "home_quota": []string{"5368709120"}}),
				plan:     addons.PlanResult{Outcome: addons.OutcomeSucceeded, Current: true},
			}
			stubAddonReconcile(t, h)
			resolveIntent = func(context.Context, string, string) (services.EntitlementSet, error) {
				return services.EntitlementSet{Roles: truenasRoles,
					Fields:    map[string][]string{"home_quota": {"1073741824", "5368709120"}},
					Lifecycle: services.LifecycleState{Account: true, Channels: map[string]bool{"smb_enabled": true}}}, nil
			}
			res, err := ReconcileAddon(context.Background(), "truenas")
			if err != nil {
				t.Fatal(err)
			}
			if !tc.finding {
				if len(res.Findings) != 0 {
					t.Fatalf("want no finding, got %+v", res.Findings)
				}
				return
			}
			if len(res.Findings) != 1 || res.Findings[0].Field != "home_quota" || res.Findings[0].Outcome != merge.TheirsOnly {
				t.Fatalf("want one theirs_only finding on home_quota, got %+v", res.Findings)
			}
			if f := res.Findings[0]; string(f.Ours) != `["5368709120"]` || string(f.Theirs) != `["10737418240"]` {
				t.Fatalf("the finding must carry the combined limit and the target's: ours %s theirs %s", f.Ours, f.Theirs)
			}
			if len(h.converged) != 0 || len(h.planAsked) != 0 {
				t.Fatalf("a hand edit is not reverted: planned %v, queued %v", h.planAsked, h.converged)
			}
		})
	}
}

// What is planned is still the wire map: the roles' limits as they are, for
// the add-on to combine; only the comparison combines them.
func TestTheLimitIsPlannedUncombined(t *testing.T) {
	h := &addonReconcileHarness{
		read: currentRead(addons.TargetAccount{Username: "charlotte", AccountID: "3001", Subject: "sub-1",
			State: state(map[string]any{"enabled": true, "home_quota": []string{"0"}})}),
		bindings: []db.TargetBinding{{Target: "truenas", SubjectID: "sub-1", Username: "charlotte", AccountID: uid(3001)}},
		plan:     addons.PlanResult{Outcome: addons.OutcomeSucceeded, Current: true},
	}
	stubAddonReconcile(t, h)
	resolveIntent = func(context.Context, string, string) (services.EntitlementSet, error) {
		return services.EntitlementSet{Roles: truenasRoles, Fields: map[string][]string{"home_quota": {"1073741824", "5368709120"}},
			Lifecycle: services.LifecycleState{Account: true}}, nil
	}
	if _, err := ReconcileAddon(context.Background(), "truenas"); err != nil {
		t.Fatal(err)
	}
	if len(h.planAsked) != 1 || string(h.planAsked[0].Desired["home_quota"]) != `["1073741824","5368709120"]` {
		t.Fatalf("the plan must carry the roles' limits uncombined: %v", h.planAsked)
	}
	// The observation recorded the target's limit, so the next pass can tell
	// who moved it.
	if len(h.basesWritten) != 1 || string(h.basesWritten[0].Base["home_quota"]) != `["0"]` {
		t.Fatalf("the base must record the observed limit: %+v", h.basesWritten)
	}
}

// The inventory's managed rows carry the standing the add-on reports for each
// account in the same read, by the subject the add-on attributes the row to (a
// renamed account keeps its own standing, and an account that took the
// recorded name is not the member's), and none when no row is theirs
// (addon-contract slice 2, for task 3.7; 7t-3). Unmanaged rows already do.
func TestTheInventorysBoundAccountsCarryTheirStanding(t *testing.T) {
	waiting := &addons.Standing{WaitingOn: "member", Note: "Waiting for a password."}
	usable := &addons.Standing{Usable: true, WaitingOn: "none"}
	h := &addonReconcileHarness{
		read: currentRead(
			addons.TargetAccount{Username: "charlotte.flair", AccountID: "3001", Subject: "sub-1", Standing: waiting},
			addons.TargetAccount{Username: "charlotte", AccountID: "3009", Standing: usable},
			addons.TargetAccount{Username: "stray", AccountID: "4000", Standing: usable},
		),
		bindings: []db.TargetBinding{
			{Target: "truenas", SubjectID: "sub-1", Username: "charlotte", AccountID: uid(3001)}, // renamed on the target
			{Target: "truenas", SubjectID: "sub-2", Username: "gone", AccountID: uid(5000)},
		},
	}
	stubAddonReconcile(t, h)

	res, err := Inventory(context.Background(), "truenas", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Accounts) != 2 || res.Accounts[0].Standing != waiting || res.Accounts[1].Standing != nil {
		t.Fatalf("standing by the add-on's attribution, none for a subject on no row: %+v", res.Accounts)
	}
	raw, _ := json.Marshal(res.Accounts[0])
	if !strings.Contains(string(raw), `"subject_id":"sub-1"`) || !strings.Contains(string(raw), `"standing":{`) {
		t.Fatalf("the row is the binding plus its standing: %s", raw)
	}
	if len(res.Unmanaged) != 2 || res.Unmanaged[0].Username != "charlotte" || res.Unmanaged[1].Standing != usable {
		t.Fatalf("unmanaged rows keep theirs, and the account now on the recorded name is one: %+v", res.Unmanaged)
	}
}

// After a channel is retired (its queued rows superseded, db.SaveTargetFieldRoles),
// the next sweep queues the departure lock again from the current roles: the
// account switch and the channels still declared, nothing retired, and
// withdraw-only, so it sends on its own (addon-contract 2.1).
func TestTheSweepRequeuesALockWithoutARetiredChannel(t *testing.T) {
	current := addons.Roles{"group": addons.RoleMembership, "enabled": addons.RoleAccount, "web": addons.RoleChannel} // smb_enabled retired
	h := &addonReconcileHarness{
		read:     currentRead(addons.TargetAccount{Username: "charlotte", AccountID: "3001", Subject: "u1"}),
		bindings: []db.TargetBinding{{Target: "truenas", SubjectID: "u1", Username: "charlotte", AccountID: uid(3001)}},
		plan: addons.PlanResult{Outcome: addons.OutcomeSucceeded, Current: true,
			Outcomes: []addons.SubjectOutcome{{Subject: "u1", Effect: db.PlanEffectApply}}},
	}
	stubAddonReconcile(t, h)
	fieldRoles = func(context.Context, string) (addons.Roles, error) { return current, nil }
	resolveIntent = func(context.Context, string, string) (services.EntitlementSet, error) {
		return services.EntitlementSet{Roles: current, Fields: map[string][]string{"group": {"lab_makers"}},
			Lifecycle: services.LifecycleState{NotAMember: true}}, nil
	}
	if _, err := ReconcileAddon(context.Background(), "truenas"); err != nil {
		t.Fatal(err)
	}
	if len(h.converged) != 1 {
		t.Fatalf("want the lock queued again, got %+v", h.converged)
	}
	c := h.converged[0]
	got, _ := json.Marshal(c.Desired)
	if !c.WithdrawsOnly || string(got) != `{"enabled":false,"web":false}` {
		t.Fatalf("want a withdraw-only {enabled:false, web:false}, got %v %s", c.WithdrawsOnly, got)
	}
}
