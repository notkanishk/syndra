package propagation

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"syndra/internal/addons"
	"syndra/internal/db"
)

// 7t-3: a blocked apply carrying the add-on's binding conflict raises the same
// standing finding the drift sweep raises from /subjects (same outcome, same
// (target, subject, "") slot, the add-on's report as theirs), so the operator
// sees it now rather than up to six hours later; and the row keeps the add-on's
// words and its next step.

func sethHeld() *addons.BindingConflict {
	return &addons.BindingConflict{
		Reason: "name_held", Username: "seth_rollins", AccountID: "3077", BoundTo: "sub-2",
		Detail: "An account named seth_rollins already exists and is bound to another subject.",
		Next:   "If it is not theirs, release it from that subject, then adopt it here.",
	}
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
	if err := json.Unmarshal(f.Theirs, &theirs); err != nil || theirs.Subject != "sub-1" || theirs.Reason != "name_held" ||
		theirs.Username != "seth_rollins" || theirs.Detail == "" || theirs.Next == "" {
		t.Fatalf("theirs must be the add-on's report, as /subjects carries it: %s (%v)", f.Theirs, err)
	}
}

func TestABlockedApplyWithAConflictRaisesTheSweepsFinding(t *testing.T) {
	h := stubAddonDrain(t, addonRow("o1"))
	c := sethHeld()
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

// A refusal with no conflict raises nothing: only the add-on's conflict is a
// standing decision; any other refusal is this row's reason alone.
func TestARefusalWithoutAConflictRaisesNoFinding(t *testing.T) {
	h := stubAddonDrain(t, addonRow("o1"))
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

// A system-initiated row is re-planned before it is sent; a plan that blocks on
// a conflict raises the same finding and never sends.
func TestAReplanBlockedOnAConflictRaisesTheSameFinding(t *testing.T) {
	h := stubAddonDrain(t, addonRow("o1"))
	h.intent.OutboxID, h.intent.Surface = "o1", db.SystemConvergenceSurface
	c := sethHeld()
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
