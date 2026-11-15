package propagation

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"syndra/internal/addons"
	"syndra/internal/db"
)

// 7t-3: a blocked apply carrying the add-on's binding conflict, for a subject
// with a binding on the target, raises the same standing finding the drift
// sweep raises from /subjects (same outcome, same (target, subject, "") slot,
// the add-on's report as theirs), so the operator sees it now rather than up
// to six hours later. A blocked create (no binding: the name it would be made
// under is taken) raises none: it is in no /subjects conflicts, nothing would
// close it, and `unbound` would release nothing. Both rows keep the add-on's
// words and next step.

// numberHeld is a bound subject's conflict: its recorded account is gone and
// the number is held by another.
func numberHeld() *addons.BindingConflict {
	return &addons.BindingConflict{
		Reason: "number_held", Username: "kofi_kingston", AccountID: "3001",
		Detail: "The account charlotte, recorded for this person, is gone, and its number is now held by a different account, kofi_kingston.",
		Next:   "Stop tracking this person's old account; the next change creates a new one for them. Adopt kofi_kingston instead only if it is theirs.",
	}
}

// nameTaken is a create's conflict: the name the new account would get is
// already an account's (unboundConflict in the TrueNAS add-on).
func nameTaken() *addons.BindingConflict {
	return &addons.BindingConflict{
		Reason: "name_held", Username: "seth_rollins", AccountID: "3077", Adoptable: true,
		Detail: "An account named seth_rollins already exists on TrueNAS and nobody in Syndra has it.",
		Next:   "Adopt it, or rename it on TrueNAS, then apply again.",
	}
}

func boundSub1(h *addonHarness) {
	h.bound = map[string]db.TargetBinding{"sub-1": {Target: "truenas", SubjectID: "sub-1", Username: "charlotte"}}
}

func assertConflictFinding(t *testing.T, findings []db.MergeFinding) {
	t.Helper()
	if len(findings) != 1 {
		t.Fatalf("want exactly one finding, got %+v", findings)
	}
	f := findings[0]
	if f.Target != "truenas" || f.SubjectID != "sub-1" || f.Field != "" || f.Outcome != "binding_conflict" {
		t.Fatalf("want binding_conflict in (truenas, sub-1, \"\"), got %+v", f)
	}
	var theirs addons.SubjectConflict
	if err := json.Unmarshal(f.Theirs, &theirs); err != nil || theirs.Subject != "sub-1" || theirs.Reason != "number_held" ||
		theirs.Username != "kofi_kingston" || theirs.Detail == "" || theirs.Next == "" {
		t.Fatalf("theirs must be the add-on's report, as /subjects carries it: %s (%v)", f.Theirs, err)
	}
}

func TestABlockedApplyWithAConflictRaisesTheSweepsFinding(t *testing.T) {
	h := stubAddonDrain(t, addonRow("o1"))
	boundSub1(h)
	c := numberHeld()
	h.resp = addons.ApplyResponse{Outcome: addons.OutcomeRejected, Status: 409, Code: "BINDING_CONFLICT",
		Effect: "blocked", Detail: c.Detail, Consequence: c.Next, Conflict: c}

	if _, err := DrainAddon(context.Background(), "truenas"); err != nil {
		t.Fatal(err)
	}
	if len(h.refused) != 1 || h.refused[0] != "o1" {
		t.Fatalf("a conflict is the target's refusal: %v", h.refused)
	}
	assertConflictFinding(t, h.findings)
	if got := h.failed["o1"]; got != c.Detail+" "+c.Next {
		t.Fatalf("the row must carry the add-on's words and next step as written, got %q", got)
	}
}

// A create blocked because its name is taken is no binding's conflict: no
// finding, and the row keeps the add-on's sentence and next step.
func TestABlockedCreateRaisesNoFindingAndKeepsItsWords(t *testing.T) {
	h := stubAddonDrain(t, addonRow("o1"))
	c := nameTaken()
	h.resp = addons.ApplyResponse{Outcome: addons.OutcomeRejected, Status: 409, Code: "BINDING_CONFLICT",
		Effect: "blocked", Detail: c.Detail, Consequence: c.Next, Conflict: c}

	if _, err := DrainAddon(context.Background(), "truenas"); err != nil {
		t.Fatal(err)
	}
	if len(h.findings) != 0 {
		t.Fatalf("a create's conflict must raise no finding: %+v", h.findings)
	}
	if len(h.refused) != 1 || h.failed["o1"] != c.Detail+" "+c.Next {
		t.Fatalf("the create is refused with the add-on's words and next step: %v %q", h.refused, h.failed["o1"])
	}
}

// A refusal with no conflict raises nothing: only the add-on's conflict is a
// standing decision; any other refusal is this row's reason alone.
func TestARefusalWithoutAConflictRaisesNoFinding(t *testing.T) {
	h := stubAddonDrain(t, addonRow("o1"))
	boundSub1(h)
	h.resp = addons.ApplyResponse{Outcome: addons.OutcomeRejected, Status: 422, Code: "APPLY_BLOCKED",
		Detail: "lab_makers does not exist.", Consequence: "Create the group on TrueNAS, then apply again."}

	if _, err := DrainAddon(context.Background(), "truenas"); err != nil {
		t.Fatal(err)
	}
	if len(h.findings) != 0 {
		t.Fatalf("no conflict, no finding: %+v", h.findings)
	}
	if got := h.failed["o1"]; got != "lab_makers does not exist. Create the group on TrueNAS, then apply again." {
		t.Fatalf("the consequence is the next step and must not be lost: %q", got)
	}
}

// A refusal with no words says so, by the system's name and never a status
// (copy guide §8).
func TestARefusalWithNoWordsNamesNoStatus(t *testing.T) {
	h := stubAddonDrain(t, addonRow("o1"))
	h.resp = addons.ApplyResponse{Outcome: addons.OutcomeRejected, Status: 422}
	if _, err := DrainAddon(context.Background(), "truenas"); err != nil {
		t.Fatal(err)
	}
	// Stored as none, as a refusal by the system: the page words the absence with the
	// system's current name, and no text is read to tell it from anything else.
	if got, ok := h.failed["o1"]; !ok || got != "" || len(h.refused) != 1 {
		t.Fatalf("want an empty reason on a refused row, got %q refused %v", got, h.refused)
	}
}

// A system-initiated row is re-planned before it is sent; a plan that blocks on
// a bound subject's conflict raises the same finding and never sends.
func TestAReplanBlockedOnAConflictRaisesTheSameFinding(t *testing.T) {
	h := stubAddonDrain(t, addonRow("o1"))
	boundSub1(h)
	h.intent.OutboxID, h.intent.Surface = "o1", db.SystemConvergenceSurface
	c := numberHeld()
	t.Cleanup(swap(&subjectEmail, func(context.Context, string) string { return "" }))
	t.Cleanup(swap(&planOne, func(context.Context, string, []addons.PlanSubject) addons.PlanResult {
		return addons.PlanResult{Outcome: addons.OutcomeSucceeded, Current: true, Outcomes: []addons.SubjectOutcome{{
			Subject: "sub-1", Effect: db.PlanEffectBlocked, Detail: c.Detail, Consequence: c.Next, Conflict: c,
		}}}
	}))

	if _, err := DrainAddon(context.Background(), "truenas"); err != nil {
		t.Fatal(err)
	}
	if len(h.dispatched) != 0 {
		t.Fatalf("a blocked plan must never be sent: %+v", h.dispatched)
	}
	assertConflictFinding(t, h.findings)
	if got := h.failed["o1"]; !strings.HasPrefix(got, c.Detail) || !strings.HasSuffix(got, c.Next) {
		t.Fatalf("the row must carry the add-on's words and next step: %q", got)
	}
}

// Each row of a claimed batch is stamped as sent immediately before its own
// call, never only when the batch was claimed: the drift sweep's in-flight
// window measures from started_at, and row k goes out k calls after the claim.
// A row the stamp could not mark (settled meanwhile) is not sent.
func TestEachRowIsStampedAsItIsSent(t *testing.T) {
	h := stubAddonDrain(t, addonRow("o1"), addonRow("o2"), addonRow("o3"))
	var events []string
	t.Cleanup(swap(&markSending, func(_ context.Context, id string) error {
		events = append(events, "stamp "+id)
		if id == "o3" {
			return db.ErrPropagationNotInFlight
		}
		return nil
	}))
	t.Cleanup(swap(&applyEntitlement, func(_ context.Context, req addons.ApplyRequest) addons.ApplyResponse {
		events = append(events, "send "+req.CallID)
		return h.resp
	}))
	if _, err := DrainAddon(context.Background(), "truenas"); err != nil {
		t.Fatal(err)
	}
	want := "stamp o1,send o1,stamp o2,send o2,stamp o3"
	if got := strings.Join(events, ","); got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}

// The requeue reasons reach last_error, which the pages show: in the copy
// guide's voice, the system by its name, never "the target".
func TestUnreachedReasonsNameTheSystem(t *testing.T) {
	for code, want := range map[string]string{
		"":                       "TrueNAS could not be reached",
		addons.CodeTargetChanged: "TrueNAS changed while Syndra was reading it",
	} {
		if got := unreachedWhy("TrueNAS", addons.ApplyResponse{Code: code}); got != want {
			t.Errorf("%q: got %q, want %q", code, got, want)
		}
	}
}
