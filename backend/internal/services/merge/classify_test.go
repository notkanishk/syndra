package merge

import (
	"encoding/json"
	"testing"
)

// The six outcomes, and the pair that justifies the whole change.
//
// `theirs-only` and `conflict` are indistinguishable without a base: both are
// "the target differs from what Syndra wants", and a two-way diff resolves both
// by writing. One of them is a hand edit being reverted; the other is two
// decisions colliding. Telling them apart is the entire point.

func raw(v any) json.RawMessage {
	out, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return out
}

func fields(m map[string]any) map[string]json.RawMessage {
	out := map[string]json.RawMessage{}
	for k, v := range m {
		out[k] = raw(v)
	}
	return out
}

func outcomeOf(t *testing.T, s Subject, field string) Outcome {
	t.Helper()
	for _, f := range s.Fields {
		if f.Field == field {
			return f.Outcome
		}
	}
	t.Fatalf("no outcome for %s in %+v", field, s.Fields)
	return ""
}

func TestNobodyMovedIsUnchanged(t *testing.T) {
	s := Classify("sub-1",
		fields(map[string]any{"enabled": true}),
		fields(map[string]any{"enabled": true}),
		fields(map[string]any{"enabled": true}))
	if got := outcomeOf(t, s, "enabled"); got != Unchanged {
		t.Fatalf("want unchanged, got %s", got)
	}
	if s.NeedsWrite() {
		t.Error("nothing moved, so nothing needs writing")
	}
}

func TestOnlySyndraMovedIsAFastForward(t *testing.T) {
	s := Classify("sub-1",
		fields(map[string]any{"group": []string{"lab_makers", "fabrication"}}),
		fields(map[string]any{"group": []string{"lab_makers"}}),
		fields(map[string]any{"group": []string{"lab_makers"}}))
	if got := outcomeOf(t, s, "group"); got != FastForward {
		t.Fatalf("want fast_forward, got %s", got)
	}
	if !s.Convergeable() || !s.NeedsWrite() {
		t.Error("a fast-forward is exactly what an unattended pass may apply")
	}
}

// Somebody made the change Syndra was going to make. Checked BEFORE theirs-only,
// because telling them they have drifted is how a system trains people to
// ignore its findings.
func TestBothMovedToTheSameValueIsAlreadyMerged(t *testing.T) {
	s := Classify("sub-1",
		fields(map[string]any{"smb_enabled": true}),
		fields(map[string]any{"smb_enabled": true}),
		fields(map[string]any{"smb_enabled": false}))
	if got := outcomeOf(t, s, "smb_enabled"); got != AlreadyMerged {
		t.Fatalf("want already_merged, got %s", got)
	}
	if !s.Convergeable() {
		t.Error("an agreement is not a finding")
	}
	if s.NeedsWrite() {
		t.Error("already merged must issue no write — the target is already there")
	}
	if len(s.Findings()) != 0 {
		t.Error("agreeing with Syndra is not drift")
	}
}

// The half of the pair that a two-way diff silently reverts.
func TestOnlyTheTargetMovedIsTheirsOnly(t *testing.T) {
	s := Classify("sub-1",
		fields(map[string]any{"enabled": true}),
		fields(map[string]any{"enabled": false}),
		fields(map[string]any{"enabled": true}))
	if got := outcomeOf(t, s, "enabled"); got != TheirsOnly {
		t.Fatalf("want theirs_only, got %s", got)
	}
	if s.Convergeable() {
		t.Fatal("a hand edit must not be reverted by an unattended pass")
	}
	if len(s.Findings()) != 1 {
		t.Fatalf("it must become a finding: %+v", s.Findings())
	}
}

// And the other half. Same two-way difference, different cause, different
// answer — which is why the base exists.
func TestBothMovedDifferentlyIsAConflict(t *testing.T) {
	s := Classify("sub-1",
		fields(map[string]any{"group": []string{"fabrication"}}),
		fields(map[string]any{"group": []string{"electronics"}}),
		fields(map[string]any{"group": []string{"lab_makers"}}))
	if got := outcomeOf(t, s, "group"); got != Conflict {
		t.Fatalf("want conflict, got %s", got)
	}
	if s.Convergeable() {
		t.Fatal("a conflict is never resolved by an unattended pass")
	}
	f := s.Findings()[0]
	// All three values travel with the finding: "what was it before" is the
	// question an operator asks first and could not previously answer.
	if len(f.Base) == 0 || len(f.Ours) == 0 || len(f.Theirs) == 0 {
		t.Fatalf("a finding must carry all three values: %+v", f)
	}
}

// The pair, stated as one test: identical two-way differences, opposite
// classifications, decided only by the base.
func TestTheSameTwoWayDifferenceIsTwoDifferentThings(t *testing.T) {
	ours := fields(map[string]any{"enabled": true})
	theirs := fields(map[string]any{"enabled": false})

	drift := Classify("sub-1", ours, theirs, fields(map[string]any{"enabled": true}))
	collision := Classify("sub-1", ours, theirs, fields(map[string]any{"enabled": false}))

	if outcomeOf(t, drift, "enabled") != TheirsOnly {
		t.Error("base == ours means the target moved")
	}
	// base == theirs means the target never moved and Syndra did: a fast-forward.
	if outcomeOf(t, collision, "enabled") != FastForward {
		t.Error("base == theirs means Syndra moved")
	}
	// Without a base, both look identical — which is the state this replaces.
	if outcomeOf(t, Classify("sub-1", ours, theirs, nil), "enabled") != NoBase {
		t.Error("with no base, no cause can be determined")
	}
}

func TestAnAbsentAccountIsItsOwnStateAndNeverConverged(t *testing.T) {
	s := Absent("sub-1")
	if !s.Absent {
		t.Fatal("want absent")
	}
	if s.Convergeable() {
		t.Fatal("re-provisioning an account somebody deleted is not a sweep's decision")
	}
	if s.NeedsWrite() {
		t.Fatal("an absent account must not be queued for a write")
	}
}

// The rollout rule. A subject with no base converges as it did before this
// mechanism existed — inventing one would either fabricate agreement or raise a
// conflict for every managed subject on the first pass.
func TestASubjectWithNoBaseConvergesAsItDidBefore(t *testing.T) {
	s := Classify("sub-1",
		fields(map[string]any{"group": []string{"fabrication"}, "enabled": true}),
		fields(map[string]any{"group": []string{"lab_makers"}, "enabled": true}),
		nil)
	if !s.Convergeable() || !s.NeedsWrite() {
		t.Fatal("a baseless subject must converge exactly as it did before")
	}
	if len(s.Findings()) != 0 {
		t.Fatalf("no base means no cause, which is not a finding: %+v", s.Findings())
	}
	// The field that already agrees is not a write, even with no base.
	if outcomeOf(t, s, "enabled") != Unchanged {
		t.Error("agreement needs no cause")
	}
}

// The subject-level rule, and the one most likely to be got wrong: an apply
// carries the WHOLE managed set, so a fast-forward cannot be applied beside a
// conflict without overwriting it.
func TestAFastForwardBesideAConflictIsNotConverged(t *testing.T) {
	s := Classify("sub-1",
		fields(map[string]any{"group": []string{"fabrication"}, "enabled": true}),
		fields(map[string]any{"group": []string{"lab_makers"}, "enabled": false}),
		fields(map[string]any{"group": []string{"lab_makers"}, "enabled": true}))

	if outcomeOf(t, s, "group") != FastForward {
		t.Error("group moved on Syndra's side only")
	}
	if outcomeOf(t, s, "enabled") != TheirsOnly {
		t.Error("enabled moved on the target's side only")
	}
	if s.Convergeable() {
		t.Fatal("converging would apply the whole set and revert the hand edit")
	}
}

// Group membership is a SET. A target returning it in its own order must not
// produce a conflict on every pass — that is the shape of false finding that
// empties a queue of its credibility.
func TestGroupOrderIsNotADifference(t *testing.T) {
	s := Classify("sub-1",
		fields(map[string]any{"group": []string{"a", "b"}}),
		fields(map[string]any{"group": []string{"b", "a"}}),
		fields(map[string]any{"group": []string{"a", "b"}}))
	if got := outcomeOf(t, s, "group"); got != Unchanged {
		t.Fatalf("membership is a set, not a sequence: got %s", got)
	}
}

// Only what Syndra manages participates. A field the target holds and no policy
// names is the target's own business, and reporting it would fill the queue
// with values nobody here decided.
func TestUnmanagedFieldsDoNotParticipate(t *testing.T) {
	s := Classify("sub-1",
		fields(map[string]any{"enabled": true}),
		fields(map[string]any{"enabled": true, "shell": "/bin/zsh"}),
		fields(map[string]any{"enabled": true, "shell": "/bin/sh"}))
	if len(s.Fields) != 1 || s.Fields[0].Field != "enabled" {
		t.Fatalf("only managed fields are classified: %+v", s.Fields)
	}
}

// A field the target has stopped reporting changed on their side, whatever the
// reason. It is not an absence to be ignored.
func TestAFieldTheTargetNoLongerReportsIsTheirs(t *testing.T) {
	s := Classify("sub-1",
		fields(map[string]any{"smb_enabled": true}),
		fields(map[string]any{}),
		fields(map[string]any{"smb_enabled": true}))
	if got := outcomeOf(t, s, "smb_enabled"); got != TheirsOnly {
		t.Fatalf("want theirs_only, got %s", got)
	}
}

// An absent account is a finding in its own right, at the account level rather
// than about any field. It is the state that already bit: stub-era bindings
// queueing a create every six hours against a production NAS.
func TestAnAbsentAccountIsAFindingWithNoField(t *testing.T) {
	found := Absent("sub-1").Findings()
	if len(found) != 1 {
		t.Fatalf("want one finding, got %+v", found)
	}
	if found[0].Outcome != DeletedUpstream || found[0].Field != "" {
		t.Fatalf("want an account-level deleted_upstream: %+v", found[0])
	}
	if found[0].SubjectID != "sub-1" {
		t.Fatalf("a finding must name its subject: %+v", found[0])
	}
}

// Every finding names its subject, or it cannot be persisted against one.
func TestEveryFindingNamesItsSubject(t *testing.T) {
	s := Classify("sub-9",
		fields(map[string]any{"enabled": true}),
		fields(map[string]any{"enabled": false}),
		fields(map[string]any{"enabled": true}))
	for _, f := range s.Findings() {
		if f.SubjectID != "sub-9" {
			t.Fatalf("finding lost its subject: %+v", f)
		}
	}
}
