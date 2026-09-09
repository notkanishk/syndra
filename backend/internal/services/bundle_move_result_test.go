package services

import (
	"context"
	"testing"

	"syndra/internal/db"
	"syndra/internal/models"
)

// Applying a move must REPORT the move.
//
// `MoveHolders` built its plan into a variable shadowing the one it returns, so
// a move that repinned somebody handed the caller a zero-valued plan: no op, no
// outcomes, every count nought. The result step renders from exactly that, so
// the operator was told "Applied to 0 people" about a person who had just been
// moved — the one thing the whole rehearse-then-report shape exists to prevent.
//
// It stayed invisible because the shared dialog disables Apply without a
// plan_id and this endpoint issued none, so nobody had ever reached the result
// step. Found by applying a real move on the dev deployment: the holder went
// v3 → v4 and the response said nobody had.
func TestMoveHolders_ReportsWhatItMoved(t *testing.T) {
	resetCascadeDeps(t)

	const bundleID = "b1"
	const targetID = "version-4"

	// The closure inputs. Unstubbed they reach the real pool, and this test is
	// about what the apply REPORTS rather than about what it computes — the
	// person moves either way.
	svcGetDirectGrantsForUser = func(context.Context, string, bool) ([]models.DirectGrant, error) {
		return nil, nil
	}
	svcGetBundlesForUser = func(context.Context, string) ([]models.Bundle, error) { return nil, nil }

	svcVersionBelongsTo = func(context.Context, string, string) (bool, error) { return true, nil }
	svcGetRolesForVersion = func(context.Context, string) ([]models.BundleRole, error) {
		return []models.BundleRole{{BundleID: bundleID, ProjectID: "p1", RoleKey: "laser"}}, nil
	}
	svcGetActiveMappingRules = func(context.Context) ([]models.MappingRule, error) { return nil, nil }
	svcGetBundleHoldersByVersion = func(context.Context, string) ([]models.BundleHolder, error) {
		return []models.BundleHolder{{BundleID: bundleID, UserID: "u1", VersionID: "version-3", Version: 3}}, nil
	}
	svcListBundleVersions = func(context.Context, string) ([]models.BundleVersion, error) {
		return []models.BundleVersion{{ID: targetID, BundleID: bundleID, Version: 4}}, nil
	}
	svcGetBundleByID = func(context.Context, string) (models.Bundle, error) {
		return models.Bundle{ID: bundleID, ConfirmationMode: "manual"}, nil
	}
	svcMoveHoldersAndEnqueue = func(
		_ context.Context, _ string, _ string, _ string, _ []string, _ []db.EnqueueParams,
	) ([]string, error) {
		return []string{"outbox-1"}, nil
	}

	plan, err := MoveHolders(context.Background(), "operator", MoveHoldersRequest{
		BundleID:  bundleID,
		VersionID: targetID,
		UserIDs:   []string{"u1"},
	})
	if err != nil {
		t.Fatalf("MoveHolders: %v", err)
	}

	if plan.Op != "move_bundle_holders" {
		t.Errorf("the result must name the operation it performed, got %q", plan.Op)
	}
	if len(plan.Outcomes) != 1 {
		t.Fatalf("a move of one person must report one row, got %d (%+v)", len(plan.Outcomes), plan.Outcomes)
	}
	if got := plan.Outcomes[0].UserID; got != "u1" {
		t.Errorf("the row must name who moved, got %q", got)
	}
	if plan.Summary.Total != 1 {
		t.Errorf("the summary must count the person it moved, got total=%d", plan.Summary.Total)
	}
	if !plan.Applied {
		t.Error("a completed apply must say it was applied")
	}
}
