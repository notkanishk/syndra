package services

import (
	"context"
	"testing"

	"syndra/internal/db"
)

// An approved request or a direct grant converges the one system its project
// serves. They never pass the cascade, so before this a member whose first
// storage role came that way got no account until an operator applied by hand.
func TestARoleChangeOutsideACascadeConvergesItsProjectsSystem(t *testing.T) {
	f := &resolverFixture{
		roles:    []db.RoleRef{{ProjectID: "pNet", RoleKey: "community"}},
		mappings: []db.RoleMapping{mapping("community", "group", "Community")},
	}
	h := stubTrigger(t, f, nil)
	lock, serving := svcInTxLockingAccess, dbTargetServingProject
	t.Cleanup(func() { svcInTxLockingAccess, dbTargetServingProject = lock, serving })
	svcInTxLockingAccess = func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }
	dbTargetServingProject = func(_ context.Context, project string) (string, bool, error) {
		return "truenas", project == "pNet", nil
	}

	if err := ConvergeProjectChange(context.Background(), "op_1", "u1", "pNet"); err != nil {
		t.Fatal(err)
	}
	if len(h.queued) != 1 || h.queued[0].Target != "truenas" || string(h.queued[0].Desired["group"]) != `["Community"]` {
		t.Fatalf("want one convergence on truenas carrying Community, got %+v", h.queued)
	}

	// A project no system serves queues nothing: most grants touch no target.
	h.queued = nil
	if err := ConvergeProjectChange(context.Background(), "op_1", "u1", "pLaser"); err != nil || len(h.queued) != 0 {
		t.Fatalf("a project serving no system must queue nothing, got %v %+v", err, h.queued)
	}
}
