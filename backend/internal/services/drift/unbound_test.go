package drift

import (
	"context"
	"encoding/json"
	"testing"

	"syndra/internal/addons"
	"syndra/internal/db"
)

// A person who holds a mapped role and has no account is converged by the
// pass, so any grant path that skipped the cascade heals within one pass.
// One row per person, never a second while one waits, and none for someone
// whose access resolves to disabled.
func TestThePassCreatesAccountsForUnboundHolders(t *testing.T) {
	h := &addonReconcileHarness{
		read:     addons.SubjectsResult{Outcome: addons.OutcomeSucceeded, Current: true},
		mappings: []db.RoleMapping{{ProjectID: "pNet", RoleKey: "community"}, {ProjectID: "pNet", RoleKey: "staff"}},
		holders: map[string][]string{
			"pNet/community": {"u_new", "u_bound", "u_waiting", "u_held"},
			"pNet/staff":     {"u_new"},
		},
		bindings: []db.TargetBinding{{SubjectID: "u_bound", Username: "bound"}},
		waiting:  map[string]bool{"u_waiting": true},
	}
	stubAddonReconcile(t, h)
	resolveIntent = func(_ context.Context, subject, _ string) (map[string]json.RawMessage, error) {
		if subject == "u_held" {
			return map[string]json.RawMessage{"enabled": json.RawMessage(`false`)}, nil
		}
		return map[string]json.RawMessage{"enabled": json.RawMessage(`true`), "group": json.RawMessage(`["Community"]`)}, nil
	}

	n, err := convergeUnbound(context.Background(), "truenas", h.bindings)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 || len(h.converged) != 1 || h.converged[0].SubjectID != "u_new" {
		t.Fatalf("want exactly u_new converged once, got %d %+v", n, h.converged)
	}
}

// The same rule for someone already bound: a pass that finds them out of step
// while a convergence for them is still waiting adds nothing. Two passes over
// one unapplied difference were two rows in Pending changes for one person.
func TestAPassDoesNotQueueASecondRowForABoundSubjectAlreadyWaiting(t *testing.T) {
	h := &addonReconcileHarness{
		read: currentRead(
			addons.TargetAccount{Username: "ada", UID: 3001},
			addons.TargetAccount{Username: "bea", UID: 3002},
		),
		bindings: []db.TargetBinding{
			{Target: "truenas", SubjectID: "u_waiting", Username: "ada", AccountUID: uid(3001)},
			{Target: "truenas", SubjectID: "u_new", Username: "bea", AccountUID: uid(3002)},
		},
		plan: addons.PlanResult{
			Outcome: addons.OutcomeSucceeded, Current: true,
			Outcomes: []addons.SubjectOutcome{
				{Subject: "u_waiting", Effect: db.PlanEffectApply},
				{Subject: "u_new", Effect: db.PlanEffectApply},
			},
		},
		waiting: map[string]bool{"u_waiting": true},
	}
	stubAddonReconcile(t, h)

	res, err := ReconcileAddon(context.Background(), "truenas")
	if err != nil {
		t.Fatal(err)
	}
	if res.Queued != 1 || len(h.converged) != 1 || h.converged[0].SubjectID != "u_new" {
		t.Fatalf("want only u_new queued, got %d %+v", res.Queued, h.converged)
	}
}

// A waiting row that wants something else is no reason to skip: the drain
// applies the snapshot a row was queued with, so skipping the newer one would
// apply the older state (an account enabled for someone deactivated since).
func TestAPassQueuesANewRowWhenTheWaitingOneWantsSomethingElse(t *testing.T) {
	h := &addonReconcileHarness{
		read:     currentRead(addons.TargetAccount{Username: "ada", UID: 3001}),
		bindings: []db.TargetBinding{{Target: "truenas", SubjectID: "u1", Username: "ada", AccountUID: uid(3001)}},
		plan: addons.PlanResult{
			Outcome: addons.OutcomeSucceeded, Current: true,
			Outcomes: []addons.SubjectOutcome{{Subject: "u1", Effect: db.PlanEffectApply}},
		},
		waiting:    map[string]bool{"u1": true},
		waitingFor: map[string]string{"u1": `{"enabled":false}`},
	}
	stubAddonReconcile(t, h)

	res, err := ReconcileAddon(context.Background(), "truenas")
	if err != nil {
		t.Fatal(err)
	}
	if res.Queued != 1 {
		t.Fatalf("the newer state must be queued behind the stale row, got %d", res.Queued)
	}
}
