package drift

import (
	"context"
	"encoding/json"
	"testing"

	"syndra/internal/addons"
	"syndra/internal/db"
	"syndra/internal/services/merge"
)

// 7t-3: the backend reads attribution off the add-on's /subjects (design 7t
// §2.1, §3). A subject is bound (a row carries it), in conflict (the add-on's
// conflicts), or absent; never matched here by name or number.

// The add-on's conflict for one subject, as /subjects carries it.
func heldBy(subject string) addons.SubjectConflict {
	return addons.SubjectConflict{Subject: subject, BindingConflict: addons.BindingConflict{
		Reason: "number_held", Username: "kofi_kingston", AccountID: "3001",
		Detail: "The account charlotte, recorded for this person, is gone, and its number is now held by a different account, kofi_kingston.",
		Next:   "Stop tracking this person's old account; the next change creates a new one for them. Adopt kofi_kingston instead only if it is theirs.",
	}}
}

// The known wrong finding, fixed: a conflicted subject's row is missing, so it
// used to read as deleted_upstream and be offered a re-create the add-on then
// refused. Now it is one binding_conflict in the account slot, carrying the
// add-on's report, and it is never planned, queued or called gone.
func TestAConflictIsRaisedAndNeverPlannedOrCalledGone(t *testing.T) {
	read := currentRead(addons.TargetAccount{Username: "kofi_kingston", AccountID: "3001"})
	read.Conflicts = []addons.SubjectConflict{heldBy("sub-1")}
	h := &addonReconcileHarness{
		read:     read,
		bindings: []db.TargetBinding{{Target: "truenas", SubjectID: "sub-1", Username: "charlotte", AccountID: uid(3001)}},
		plan: addons.PlanResult{Outcome: addons.OutcomeSucceeded, Current: true,
			Outcomes: []addons.SubjectOutcome{{Subject: "sub-1", Effect: db.PlanEffectApply}}},
	}
	stubAddonReconcile(t, h)

	res, err := ReconcileAddon(context.Background(), "truenas")
	if err != nil {
		t.Fatal(err)
	}
	if len(h.findingsWritten) != 1 {
		t.Fatalf("want exactly one finding, the conflict: %+v", h.findingsWritten)
	}
	f := h.findingsWritten[0]
	if f.Outcome != string(merge.BindingConflict) || f.SubjectID != "sub-1" || f.Field != "" {
		t.Fatalf("want binding_conflict in sub-1's account slot, got %+v", f)
	}
	var theirs addons.SubjectConflict
	if err := json.Unmarshal(f.Theirs, &theirs); err != nil || theirs.Reason != "number_held" ||
		theirs.Username != "kofi_kingston" || theirs.Detail == "" || theirs.Next == "" {
		t.Fatalf("the finding must carry the add-on's reason, holder and words: %s (%v)", f.Theirs, err)
	}
	if len(h.planAsked) != 0 || len(h.converged) != 0 || res.Queued != 0 {
		t.Fatalf("a conflicted subject is never planned or queued: asked %+v, queued %d", h.planAsked, res.Queued)
	}
	if len(res.Stale) != 0 {
		t.Fatalf("a conflicted subject is not gone: %+v", res.Stale)
	}
	for _, c := range h.findingsCleared {
		if c == "sub-1/" {
			t.Fatal("the pass that raised the conflict must not close its slot")
		}
	}
	if len(res.Unmanaged) != 1 || res.Unmanaged[0].Username != "kofi_kingston" {
		t.Fatalf("the holder is attributed to nobody, so it is unmanaged: %+v", res.Unmanaged)
	}
}

// A conflict for a subject the backend has no binding for is raised too
// (which subject a binding is for is the add-on's to say), and the unbound half
// of the pass does not queue the create the add-on would refuse.
func TestAConflictForAnUnrecordedSubjectIsRaisedNotCreated(t *testing.T) {
	read := currentRead()
	read.Conflicts = []addons.SubjectConflict{heldBy("u_held")}
	h := &addonReconcileHarness{
		read:     read,
		mappings: []db.RoleMapping{{ProjectID: "pNet", RoleKey: "community"}},
		holders:  map[string][]string{"pNet/community": {"u_held"}},
	}
	stubAddonReconcile(t, h)

	if _, err := ReconcileAddon(context.Background(), "truenas"); err != nil {
		t.Fatal(err)
	}
	if len(h.converged) != 0 {
		t.Fatalf("no create may be queued for a subject the add-on reports in conflict: %+v", h.converged)
	}
	if len(h.findingsWritten) != 1 || h.findingsWritten[0].Outcome != string(merge.BindingConflict) {
		t.Fatalf("want the conflict raised: %+v", h.findingsWritten)
	}
}

// Closed `agreed` (the account-slot clear) the first pass the add-on stops
// reporting the conflict and names the subject's own row again.
func TestAConflictTheAddOnStopsReportingIsClosed(t *testing.T) {
	h := &addonReconcileHarness{
		read:     currentRead(addons.TargetAccount{Username: "charlotte", AccountID: "3001", Subject: "sub-1"}),
		bindings: []db.TargetBinding{{Target: "truenas", SubjectID: "sub-1", Username: "charlotte", AccountID: uid(3001)}},
		plan: addons.PlanResult{Outcome: addons.OutcomeSucceeded, Current: true,
			Outcomes: []addons.SubjectOutcome{{Subject: "sub-1", Effect: db.PlanEffectNoChange}}},
	}
	stubAddonReconcile(t, h)

	if _, err := ReconcileAddon(context.Background(), "truenas"); err != nil {
		t.Fatal(err)
	}
	closed := false
	for _, c := range h.findingsCleared {
		closed = closed || c == "sub-1/"
	}
	if !closed || len(h.findingsWritten) != 0 {
		t.Fatalf("the account slot must close and nothing be raised: cleared %v, written %+v", h.findingsCleared, h.findingsWritten)
	}
}

// A conflict ends when the add-on stops listing it, whether or not the
// subject has a binding here: on a whole read, every standing
// binding_conflict outside this read's conflicts is closed, keeping the ones
// it lists. Not on a truncated read, which may have cut a conflict's holder.
func TestAWholeReadClosesTheConflictsItNoLongerLists(t *testing.T) {
	read := currentRead()
	read.Conflicts = []addons.SubjectConflict{heldBy("sub-1")}
	h := &addonReconcileHarness{read: read}
	stubAddonReconcile(t, h)
	if _, err := ReconcileAddon(context.Background(), "truenas"); err != nil {
		t.Fatal(err)
	}
	if len(h.closedOutside) != 1 || h.closedOutside[0] != "binding_conflict:sub-1" {
		t.Fatalf("want the conflicts outside {sub-1} closed, got %v", h.closedOutside)
	}

	truncated := currentRead()
	truncated.Truncated = true
	h2 := &addonReconcileHarness{read: truncated}
	stubAddonReconcile(t, h2)
	if _, err := ReconcileAddon(context.Background(), "truenas"); err != nil {
		t.Fatal(err)
	}
	if len(h2.closedOutside) != 0 {
		t.Fatalf("a truncated read closes no conflict: %v", h2.closedOutside)
	}
}

// A reply that both attributes a row to a subject and lists it in conflict
// contradicts itself; the conflict wins, so the subject is never planned.
func TestAConflictWinsOverARowForTheSameSubject(t *testing.T) {
	b := []db.TargetBinding{{SubjectID: "sub-1"}}
	accounts := []addons.TargetAccount{{Username: "kofi_kingston", AccountID: "3001", Subject: "sub-1"}}
	if live, stale := partitionByPresence(b, accounts, map[string]bool{"sub-1": true}, nil); len(live) != 0 || len(stale) != 0 {
		t.Fatalf("live=%+v stale=%+v, want neither", live, stale)
	}
}

// An apply landing between the add-on's read of the app and its read of its
// bindings leaves the subject on no row and in no conflict for one read. With
// an apply in flight since the read began, the pass concludes nothing about
// them: no deleted_upstream, no stale row, nothing queued.
func TestASubjectWithAnApplyInFlightIsNeverConcludedDeleted(t *testing.T) {
	h := &addonReconcileHarness{
		read:     currentRead(),
		bindings: []db.TargetBinding{{Target: "truenas", SubjectID: "sub-1", Username: "charlotte", AccountID: uid(3001)}},
		inFlight: map[string]bool{"sub-1": true},
	}
	stubAddonReconcile(t, h)

	res, err := ReconcileAddon(context.Background(), "truenas")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Stale) != 0 || len(h.findingsWritten) != 0 || len(h.converged) != 0 {
		t.Fatalf("nothing may be concluded about a subject whose apply was in flight: stale %+v, findings %+v, queued %+v",
			res.Stale, h.findingsWritten, h.converged)
	}
	// Asked on the database's clock, which completed_at and started_at are on,
	// and bounded by how long a claimed apply's calls can run, so a row a
	// crashed drain left in_flight never hides a deletion for ever.
	if !h.inFlightSince.Equal(dbClock) || h.inFlightWindow != addons.ApplyWindow() || h.inFlightWindow <= 0 {
		t.Fatalf("in flight is asked from the database's clock with the apply window, got %v / %v", h.inFlightSince, h.inFlightWindow)
	}

	// The same subject with nothing in flight is concluded gone: the guard is
	// the in-flight answer, not a blanket silence.
	h2 := &addonReconcileHarness{read: currentRead(), bindings: h.bindings}
	stubAddonReconcile(t, h2)
	if _, err := ReconcileAddon(context.Background(), "truenas"); err != nil {
		t.Fatal(err)
	}
	if len(h2.findingsWritten) != 1 || h2.findingsWritten[0].Outcome != string(merge.DeletedUpstream) {
		t.Fatalf("want deleted_upstream without an apply in flight: %+v", h2.findingsWritten)
	}
}

// A /subjects the add-on refused because its own records failed
// (STORE_UNREADABLE) is not TrueNAS silent: the pass and the inventory say
// read_refused, with the add-on's sentence and its next step, never
// target_unreachable ("It did not answer").
func TestAnUnreadableStoreIsARefusedReadNeverAnOutage(t *testing.T) {
	const detail = "Syndra's records of who has which account on TrueNAS can't be read. Check the add-on's storage, then try again."
	refused := addons.SubjectsResult{Outcome: addons.OutcomeIndeterminate, Status: 500, Code: "STORE_UNREADABLE", Detail: detail}

	h := &addonReconcileHarness{read: refused}
	stubAddonReconcile(t, h)
	res, err := ReconcileAddon(context.Background(), "truenas")
	if err != nil {
		t.Fatal(err)
	}
	if !res.Halted || res.Reason != db.UnreconciledReadRefused || res.Detail != detail {
		t.Fatalf("want a halted pass, read_refused, with the add-on's words: %+v", res)
	}
	if len(h.marked) != 1 || h.marked[0] != db.UnreconciledReadRefused {
		t.Fatalf("the target's record must say read_refused: %v", h.marked)
	}

	inv, err := Inventory(context.Background(), "truenas", true)
	if err != nil {
		t.Fatal(err)
	}
	if !inv.Halted || inv.Reason != db.UnreconciledReadRefused || inv.Detail != detail {
		t.Fatalf("the inventory must say the same: %+v", inv)
	}

	// Every other failure is TrueNAS not read, answered in a code or not
	// (NO_CURRENT_READ: it could not be read and the add-on holds no copy):
	// target_unreachable, which the pages word with the system's name and their
	// own next step; never read_refused, whose words are about the add-on's
	// records.
	for _, read := range []addons.SubjectsResult{
		{Outcome: addons.OutcomeUnreached},
		{Outcome: addons.OutcomeUnreached, Status: 503, Code: "TARGET_UNREACHABLE", Detail: "TrueNAS did not answer."},
		{Outcome: addons.OutcomeIndeterminate, Status: 503, Code: "NO_CURRENT_READ", Detail: "TrueNAS could not be read."},
		{Outcome: addons.OutcomeIndeterminate, Status: 502, Code: "TARGET_UNREADABLE"},
		{Outcome: addons.OutcomeIndeterminate, Status: 502},
	} {
		if reason, words := readFailure(read); reason != db.UnreconciledUnreachable || words != "" {
			t.Errorf("%+v: got %q %q, want target_unreachable", read, reason, words)
		}
	}
}
