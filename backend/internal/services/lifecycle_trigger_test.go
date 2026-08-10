package services

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"syndra/internal/db"
)

// 7.9/7.10 — a role change reaches a target when somebody mapped it there.
//
// The symmetry is the property: gaining a first mapped role creates the account
// through the apply itself, and losing the last one disables it, both through
// the same convergence with nothing special-cased in either direction.

type triggerHarness struct {
	mapped    map[string][]string
	queued    []db.SystemConvergence
	recordErr error
	targetErr error
}

func stubTrigger(t *testing.T, f *resolverFixture, mapped map[string][]string) *triggerHarness {
	t.Helper()
	f.install(t)
	h := &triggerHarness{mapped: mapped}

	origTargets, origRecord := dbTargetsMappedToRole, dbRecordSystemConvergence
	t.Cleanup(func() { dbTargetsMappedToRole, dbRecordSystemConvergence = origTargets, origRecord })

	dbTargetsMappedToRole = func(_ context.Context, project, role string) ([]string, error) {
		return h.mapped[project+"/"+role], h.targetErr
	}
	dbRecordSystemConvergence = func(_ context.Context, c db.SystemConvergence) (string, string, error) {
		h.queued = append(h.queued, c)
		return "plan_1", "outbox_1", h.recordErr
	}
	return h
}

// Gaining a first mapped role queues a convergence whose intent enables the
// account. Creation is part of that convergence rather than a call sequenced
// before it — which is what dissolves the ordering problem instead of answering
// it.
func TestGainingAMappedRoleQueuesAConvergenceThatEnablesTheAccount(t *testing.T) {
	f := &resolverFixture{
		roles:    []db.RoleRef{{ProjectID: "pLab", RoleKey: "trained"}},
		mappings: []db.RoleMapping{mapping("trained", "group", "lab_makers")},
	}
	h := stubTrigger(t, f, map[string][]string{"pLab/trained": {"truenas"}})

	err := triggerTargetConvergence(context.Background(), "op_1", "u1",
		[]roleKey{{projectID: "pLab", roleKey: "trained"}})
	if err != nil {
		t.Fatalf("triggerTargetConvergence: %v", err)
	}
	if len(h.queued) != 1 {
		t.Fatalf("want one convergence, got %d", len(h.queued))
	}
	got := h.queued[0]
	if got.Target != "truenas" || got.SubjectID != "u1" || got.Actor != "op_1" {
		t.Errorf("convergence queued for the wrong thing: %+v", got)
	}
	if string(got.Desired["enabled"]) != "true" {
		t.Errorf("a subject holding a mapped role must be enabled: %s", got.Desired["enabled"])
	}
	if string(got.Desired["group"]) != `["lab_makers"]` {
		t.Errorf("the mapped value did not reach the intent: %s", got.Desired["group"])
	}
}

// Losing the last mapped role disables and never deletes. Deprovisioning is
// reversible by construction (design §12): the account and its home data stay,
// and regaining a role restores it through the same apply.
func TestLosingTheLastMappedRoleDisablesRatherThanDeleting(t *testing.T) {
	// The resolver sees no roles left — this is the state AFTER the removal,
	// which is what the trigger resolves against.
	f := &resolverFixture{}
	h := stubTrigger(t, f, map[string][]string{"pLab/trained": {"truenas"}})

	err := triggerTargetConvergence(context.Background(), "op_1", "u1",
		[]roleKey{{projectID: "pLab", roleKey: "trained"}})
	if err != nil {
		t.Fatalf("triggerTargetConvergence: %v", err)
	}
	if len(h.queued) != 1 {
		t.Fatalf("want one convergence, got %d", len(h.queued))
	}
	got := h.queued[0]
	if string(got.Desired["enabled"]) != "false" || string(got.Desired["smb_enabled"]) != "false" {
		t.Errorf("losing the last mapped role must disable: %v", got.Desired)
	}
	// Nothing in the intent says delete, and there is nowhere for it to: the
	// entitlement schema has no such field. Asserted anyway, because the reason
	// it is absent is a design decision and not an oversight.
	for field := range got.Desired {
		if strings.Contains(field, "delete") || strings.Contains(field, "purge") {
			t.Errorf("an automatic convergence must never carry %q", field)
		}
	}
}

// A role nobody mapped reaches nothing. This is the common case and it has to
// stay free — one indexed read, no convergence, no plan row.
func TestAnUnmappedRoleTriggersNothing(t *testing.T) {
	f := &resolverFixture{}
	h := stubTrigger(t, f, nil)

	err := triggerTargetConvergence(context.Background(), "op_1", "u1",
		[]roleKey{{projectID: "pLab", roleKey: "unmapped"}})
	if err != nil {
		t.Fatalf("triggerTargetConvergence: %v", err)
	}
	if len(h.queued) != 0 {
		t.Fatalf("an unmapped role must queue nothing, got %+v", h.queued)
	}
}

// Two changed roles mapped to one target produce ONE convergence, not two. The
// apply is level-triggered and carries the whole resolved set, so a second row
// would dispatch the same state twice.
func TestTwoChangedRolesOnOneTargetConvergeOnce(t *testing.T) {
	f := &resolverFixture{
		roles: []db.RoleRef{{ProjectID: "pLab", RoleKey: "trained"}},
		mappings: []db.RoleMapping{
			mapping("trained", "group", "lab_makers"),
		},
	}
	h := stubTrigger(t, f, map[string][]string{
		"pLab/trained": {"truenas"},
		"pLab/safety":  {"truenas"},
	})

	err := triggerTargetConvergence(context.Background(), "op_1", "u1", []roleKey{
		{projectID: "pLab", roleKey: "trained"},
		{projectID: "pLab", roleKey: "safety"},
	})
	if err != nil {
		t.Fatalf("triggerTargetConvergence: %v", err)
	}
	if len(h.queued) != 1 {
		t.Fatalf("want one convergence for one target, got %d", len(h.queued))
	}
}

// A convergence that cannot be queued fails the transaction rather than being
// logged past. The role change and the convergence that follows from it commit
// together, or the person's access changes in Zitadel and nowhere else with
// nothing on any surface disagreeing.
func TestAConvergenceThatCannotBeQueuedFailsTheChange(t *testing.T) {
	f := &resolverFixture{roles: []db.RoleRef{{ProjectID: "pLab", RoleKey: "trained"}}}
	h := stubTrigger(t, f, map[string][]string{"pLab/trained": {"truenas"}})
	h.recordErr = errors.New("the outbox is unavailable")

	err := triggerTargetConvergence(context.Background(), "op_1", "u1",
		[]roleKey{{projectID: "pLab", roleKey: "trained"}})
	if err == nil {
		t.Fatal("a convergence that failed to queue must fail the role change with it")
	}
}

// The delta is what decides WHETHER to converge; the resolved set is what is
// converged TO. Sending the delta would be a level-triggered apply carrying only
// what moved, which removes everything that did not.
func TestTheIntentIsTheWholeResolvedSetAndNotTheDelta(t *testing.T) {
	f := &resolverFixture{
		roles: []db.RoleRef{
			{ProjectID: "pLab", RoleKey: "trained"},
			{ProjectID: "pLab", RoleKey: "safety"},
		},
		mappings: []db.RoleMapping{
			mapping("trained", "group", "lab_makers"),
			mapping("safety", "group", "fabrication"),
		},
	}
	h := stubTrigger(t, f, map[string][]string{"pLab/safety": {"truenas"}})

	// Only `safety` changed, and the intent must still carry both groups.
	err := triggerTargetConvergence(context.Background(), "op_1", "u1",
		[]roleKey{{projectID: "pLab", roleKey: "safety"}})
	if err != nil {
		t.Fatalf("triggerTargetConvergence: %v", err)
	}
	if got := string(h.queued[0].Desired["group"]); got != `["fabrication","lab_makers"]` {
		t.Errorf("the intent must be the whole set the subject should hold, got %s", got)
	}
}

// The trigger is wired to `deltaParams` and nowhere else, because that is the
// one function every closure delta in this package passes through. A source
// guard, since the alternative — nine hooks — fails by omission, and an omission
// is invisible to every behavioural test that does not know to look for it.
func TestTheTriggerHangsOffTheOneChokepoint(t *testing.T) {
	src := readServiceSource(t, "cascade.go")
	body := sliceBetween(t, src, "func deltaParams(", "\nfunc ")
	if !strings.Contains(body, "triggerTargetConvergence(") {
		t.Fatal("deltaParams no longer fires the lifecycle trigger, so a cascade reaches Zitadel and no mapped target")
	}
	if strings.Index(body, "triggerTargetConvergence(") > strings.Index(body, "params := make(") {
		t.Error("the trigger must run before the params are built, so a failure to queue it stops the change")
	}

	// And every caller goes through it: a closure delta turned into enqueue
	// params by hand would bypass the trigger entirely.
	for _, file := range []string{"bundle_publish.go", "role_members.go"} {
		other := readServiceSource(t, file)
		if strings.Contains(other, "db.EnqueueParams{") {
			t.Errorf("%s builds enqueue params directly instead of going through deltaParams", file)
		}
	}
}

func readServiceSource(t *testing.T, name string) string {
	t.Helper()
	raw, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(raw)
}

func sliceBetween(t *testing.T, src, from, to string) string {
	t.Helper()
	i := strings.Index(src, from)
	if i < 0 {
		t.Fatalf("source does not contain %q", from)
	}
	rest := src[i+len(from):]
	if j := strings.Index(rest, to); j >= 0 {
		return rest[:j]
	}
	return rest
}
