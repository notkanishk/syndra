package services

import (
	"context"
	"strings"
	"testing"
	"time"

	"syndra/internal/db"
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

// A project the map references (here through a rule) and the directory doesn't
// list has no label: the id never stands in for its name, and the map's one
// fallback says "Unknown project" (ui/src/components/insights/model.ts).
func TestTopology_AnUnlistedProjectHasNoLabel(t *testing.T) {
	setupSnapshotTestFixtures(t, 1, 0, 1)
	svcGetActiveMappingRules = func(context.Context) ([]models.MappingRule, error) {
		return []models.MappingRule{{ID: "r1", SourceProject: "pGone", SourceRole: "old", TargetProject: "p0", TargetRole: "role0"}}, nil
	}
	snap, err := newAccessSnapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	graph, err := topologyFromSnapshot(snap)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range graph.Nodes {
		if n.ID == "project:pGone" {
			if n.Label != "" || n.ProjectID != "pGone" {
				t.Fatalf("placeholder %+v, want no label and its project id", n)
			}
			return
		}
	}
	t.Fatalf("no placeholder node for the unlisted project in %+v", graph.Nodes)
}

// Projects without a name come back in one order, by id, every time: the
// response never depends on map iteration.
func TestExplainUserAccess_UnnamedProjectsAreOrderedById(t *testing.T) {
	noRecordsFor(t, nil)
	observedStore(t, db.Observation{ObservedAt: time.Now(), Complete: true}, nil, []db.ObservedGrant{
		{GrantID: "g1", ProjectID: "pZulu", RoleKeys: []string{"a"}},
		{GrantID: "g2", ProjectID: "pAlpha", RoleKeys: []string{"a"}},
		{GrantID: "g3", ProjectID: "pMike", RoleKeys: []string{"a"}},
	})
	for range 5 {
		view, err := ExplainUserAccess(context.Background(), "dev_admin")
		if err != nil {
			t.Fatal(err)
		}
		var got []string
		for _, p := range view.Projects {
			if p.ProjectName != "" {
				t.Fatalf("%s has a name %q; the test wants unlisted projects", p.ProjectID, p.ProjectName)
			}
			got = append(got, p.ProjectID)
		}
		if strings.Join(got, ",") != "pAlpha,pMike,pZulu" {
			t.Fatalf("order %v, want by id", got)
		}
	}
}
