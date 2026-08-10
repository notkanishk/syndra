package services

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"syndra/internal/addons"
	"syndra/internal/db"
	"syndra/internal/models"
)

// The rehearsal that connects the resolver, the add-on and the plan store.
//
// What it asserts is mostly about DISAGREEMENT: that the set asked about is the
// set that would be applied, that a subject the add-on skipped is reported
// rather than dropped, and that an answer from the mirror produces a plan
// labelled provisional rather than one that looks current.

// planStub replaces the add-on read leg. It records what it was asked so a test
// can check the question, not only the answer.
type planStub struct {
	asked  []addons.PlanSubject
	target string
	ack    bool
	result addons.PlanResult
}

func (p *planStub) install(t *testing.T) {
	t.Helper()
	prev := addonsPlan
	addonsPlan = func(_ context.Context, target string, subjects []addons.PlanSubject, ack bool) addons.PlanResult {
		p.target, p.asked, p.ack = target, subjects, ack
		return p.result
	}
	t.Cleanup(func() { addonsPlan = prev })
}

func stubDirectory(t *testing.T, email string) {
	t.Helper()
	prev := directoryFindUser
	directoryFindUser = func(_ context.Context, id string) (models.UserProfile, bool, error) {
		return models.UserProfile{ID: id, Name: "Ada", Email: email}, true, nil
	}
	t.Cleanup(func() { directoryFindUser = prev })
}

func okPlan(outcomes ...addons.SubjectOutcome) addons.PlanResult {
	return addons.PlanResult{
		Outcomes: outcomes, Outcome: addons.OutcomeSucceeded,
		Current: true, TakenAt: time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC),
	}
}

func TestTheRehearsalAsksAboutTheSetTheApplyWouldSend(t *testing.T) {
	f := &resolverFixture{
		roles:    []db.RoleRef{{ProjectID: "pLab", RoleKey: "trained"}},
		mappings: []db.RoleMapping{mapping("trained", "group", "lab_makers")},
	}
	f.install(t)
	stubDirectory(t, "ada@example.edu")
	stub := &planStub{result: okPlan(addons.SubjectOutcome{
		Subject: "u1", Effect: EffectApply, Detail: "Joins lab_makers.", Fingerprint: "fp-1",
	})}
	stub.install(t)

	plan, err := RehearseEntitlements(context.Background(), EntitlementRehearsal{
		Target: "truenas", SubjectIDs: []string{"u1"}, Actor: "op_1",
	})
	if err != nil {
		t.Fatalf("RehearseEntitlements: %v", err)
	}

	if stub.target != "truenas" || len(stub.asked) != 1 {
		t.Fatalf("the add-on was asked the wrong question: target=%s subjects=%d", stub.target, len(stub.asked))
	}
	if stub.asked[0].Email != "ada@example.edu" {
		t.Errorf("the email a username is derived from did not travel: %q", stub.asked[0].Email)
	}
	// The set asked about IS the resolved set. A rehearsal that asked about
	// anything else would review a diff the apply does not produce.
	resolvedSet := plan.Desired["u1"].Desired()
	if !sameRaw(stub.asked[0].Desired, resolvedSet) {
		t.Errorf("asked about %v, would apply %v", stub.asked[0].Desired, resolvedSet)
	}
	if len(plan.Outcomes) != 1 || plan.Outcomes[0].Fingerprint != "fp-1" {
		t.Fatalf("the add-on's fingerprint must reach the plan: %+v", plan.Outcomes)
	}
	if plan.Provisional {
		t.Error("a live read must not produce a provisional plan")
	}
}

// §8 — an unreachable TARGET produces a plan against last-known state, labelled
// with its age. That is the fail-open rule: the entitlement decision is not
// blocked by a NAS being down.
func TestAnAnswerFromTheMirrorIsAProvisionalPlan(t *testing.T) {
	f := &resolverFixture{
		roles:    []db.RoleRef{{ProjectID: "pLab", RoleKey: "trained"}},
		mappings: []db.RoleMapping{mapping("trained", "group", "lab_makers")},
	}
	f.install(t)
	stubDirectory(t, "ada@example.edu")

	taken := time.Date(2026, 8, 8, 9, 30, 0, 0, time.UTC)
	stub := &planStub{result: addons.PlanResult{
		Outcome: addons.OutcomeSucceeded, Current: false, TakenAt: taken,
		Outcomes: []addons.SubjectOutcome{{Subject: "u1", Effect: EffectApply, Fingerprint: "fp-stale"}},
	}}
	stub.install(t)

	plan, err := RehearseEntitlements(context.Background(), EntitlementRehearsal{
		Target: "truenas", SubjectIDs: []string{"u1"}, Actor: "op_1",
	})
	if err != nil {
		t.Fatalf("RehearseEntitlements: %v", err)
	}
	if !plan.Provisional {
		t.Error("a plan computed from the mirror must be marked provisional")
	}
	if !plan.StateReadAt.Equal(taken) {
		// Without the age, "computed against last-known state" is a label with
		// no number attached, which is not something an operator can act on.
		t.Errorf("state read time = %v, want %v", plan.StateReadAt, taken)
	}
}

// The add-on being down is not the target being down, and the two must not
// collapse: one has a mirror to plan against and the other has nothing.
func TestAnUnreachableAddonRefusesRatherThanPlanningFromNothing(t *testing.T) {
	f := &resolverFixture{roles: []db.RoleRef{{ProjectID: "pLab", RoleKey: "trained"}}}
	f.install(t)
	stubDirectory(t, "ada@example.edu")
	stub := &planStub{result: addons.PlanResult{
		Outcome: addons.OutcomeUnreached, Err: errors.New("connection refused"),
	}}
	stub.install(t)

	_, err := RehearseEntitlements(context.Background(), EntitlementRehearsal{
		Target: "truenas", SubjectIDs: []string{"u1"}, Actor: "op_1",
	})
	if !errors.Is(err, ErrTargetUnplannable) {
		t.Fatalf("want ErrTargetUnplannable, got %v", err)
	}
}

// A subject the add-on skipped is reported blocked, not dropped. Dropping would
// silently shrink the cohort the operator selected, and the plan is the record
// of what they reviewed.
func TestASubjectTheTargetDidNotAnswerForIsReported(t *testing.T) {
	f := &resolverFixture{roles: []db.RoleRef{{ProjectID: "pLab", RoleKey: "trained"}}}
	f.install(t)
	stubDirectory(t, "ada@example.edu")
	stub := &planStub{result: okPlan(addons.SubjectOutcome{
		Subject: "u1", Effect: EffectNoChange, Fingerprint: "fp-1",
	})}
	stub.install(t)

	plan, err := RehearseEntitlements(context.Background(), EntitlementRehearsal{
		Target: "truenas", SubjectIDs: []string{"u1", "u2"}, Actor: "op_1",
	})
	if err != nil {
		t.Fatalf("RehearseEntitlements: %v", err)
	}
	if len(plan.Outcomes) != 2 {
		t.Fatalf("both selected subjects must appear: %+v", plan.Outcomes)
	}
	var missing BulkOutcome
	for _, o := range plan.Outcomes {
		if o.UserID == "u2" {
			missing = o
		}
	}
	if missing.Effect != EffectBlocked {
		t.Errorf("an unanswered subject must be blocked, got %q", missing.Effect)
	}
	if missing.Fingerprint == "fp-1" || missing.Fingerprint == "" {
		// It must carry a fingerprint of its own that cannot match a live read,
		// or a row nobody evaluated would verify at apply time.
		t.Errorf("an unanswered subject must not borrow another row's fingerprint: %q", missing.Fingerprint)
	}
}

func TestTheBuiltInTargetIsRefused(t *testing.T) {
	_, err := RehearseEntitlements(context.Background(), EntitlementRehearsal{
		Target: db.TargetZitadel, SubjectIDs: []string{"u1"},
	})
	if !errors.Is(err, ErrTargetIsBuiltIn) {
		t.Fatalf("want ErrTargetIsBuiltIn, got %v", err)
	}
}

func TestARehearsalForNobodyIsRefused(t *testing.T) {
	_, err := RehearseEntitlements(context.Background(), EntitlementRehearsal{
		Target: "truenas", SubjectIDs: []string{"  ", ""},
	})
	if !errors.Is(err, ErrNoSubjects) {
		t.Fatalf("want ErrNoSubjects, got %v", err)
	}
}

// The desired-state encoding is the instruction, so the three ways it can be
// wrong are each a way a subject ends up with the wrong access.
func TestDesiredEncodesTheInstructionAndNotAnApproximationOfIt(t *testing.T) {
	t.Run("an empty field is present and empty, never absent", func(t *testing.T) {
		set := EntitlementSet{Fields: map[string][]string{"group": {}}}
		got := set.Desired()
		raw, present := got["group"]
		if !present {
			t.Fatal("a fully suppressed field must still be managed, or the groups stay exactly as they were")
		}
		if string(raw) != "[]" {
			t.Errorf("group = %s, want []", raw)
		}
	})

	t.Run("lifecycle is always present", func(t *testing.T) {
		none := EntitlementSet{Fields: map[string][]string{}}
		got := none.Desired()
		for _, field := range []string{FieldEnabled, FieldSMBEnabled} {
			if _, present := got[field]; !present {
				t.Errorf("%s absent: an omitted lifecycle field leaves a deprovisioned account usable", field)
			}
		}
		if string(got[FieldEnabled]) != "false" {
			t.Errorf("enabled = %s, want false", got[FieldEnabled])
		}
	})

	t.Run("a lifecycle field arriving through Fields does not race the resolver's", func(t *testing.T) {
		set := EntitlementSet{
			Fields:    map[string][]string{FieldEnabled: {"true"}},
			Lifecycle: LifecycleState{Enabled: false},
		}
		if string(set.Desired()[FieldEnabled]) != "false" {
			t.Error("the resolver's lifecycle value must win over anything in Fields")
		}
	})

	t.Run("never nil", func(t *testing.T) {
		empty := EntitlementSet{}
		if empty.Desired() == nil {
			t.Error("nil encodes as JSON null, which the drain reads as no approved desired state")
		}
	})
}

func sameRaw(a, b map[string]json.RawMessage) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if string(b[k]) != string(v) {
			return false
		}
	}
	return true
}
