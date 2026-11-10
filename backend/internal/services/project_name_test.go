package services

import (
	"context"
	"testing"

	"syndra/internal/models"
)

// A project the directory does not list has no name: "" and
// ProjectNameResolved=false, so the UI's one fallback ("Unknown project")
// renders it. The id is never passed off as a name (one-path W2d).

func TestRoleMembers_UnlistedProjectHasNoName(t *testing.T) {
	setupSnapshotTestFixtures(t, 1, 0, 1)
	stubRoleFixtures(t)
	view, err := RoleMembers(context.Background(), "gone-project", "trained")
	if err != nil {
		t.Fatalf("RoleMembers: %v", err)
	}
	if view.ProjectID != "gone-project" || view.ProjectName != "" {
		t.Errorf("unlisted project: id=%q name=%q, want the id and no name", view.ProjectID, view.ProjectName)
	}
}

func TestProjectClaimShape_UnlistedProjectHasNoName(t *testing.T) {
	stubClaimShapeDeps(t)
	view, err := GetProjectClaimShape(context.Background(), "gone-project")
	if err != nil {
		t.Fatal(err)
	}
	if view.ProjectName != "" {
		t.Errorf("project name = %q, want none", view.ProjectName)
	}
	listed, err := GetProjectClaimShape(context.Background(), "pNet")
	if err != nil || listed.ProjectName != "pNet" {
		t.Errorf("a listed project keeps its name: %+v err=%v", listed, err)
	}
}

func TestExplainUserAccess_ObservedUnlistedProjectIsUnresolved(t *testing.T) {
	// upsertRole carries the same convention for the recorded path; the
	// observed-only bucket in ExplainUserAccess shares the directory call.
	roleMap := map[roleKey]*models.EffectiveRole{}
	setupSnapshotTestFixtures(t, 1, 0, 1)
	upsertRole(context.Background(), roleMap, roleKey{projectID: "gone-project", roleKey: "x"}, true, models.RoleReason{Kind: "direct"})
	got := roleMap[roleKey{projectID: "gone-project", roleKey: "x"}]
	if got.ProjectName != "" || got.ProjectNameResolved {
		t.Errorf("got name=%q resolved=%v", got.ProjectName, got.ProjectNameResolved)
	}
}
