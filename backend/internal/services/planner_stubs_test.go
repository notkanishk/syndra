package services

import (
	"context"
	"testing"

	"syndra/internal/db"
	"syndra/internal/models"
)

// plannerFromStubs feeds the planner's one snapshot from the per-person
// stubs the cascade tests already set (direct grants, bundle roles, rules,
// queued revocations), read when the snapshot is: a test that changes a stub
// after this still drives the plan. Everyone is an active person the
// directory knows, and Zitadel offers every role the stubs mention.
func plannerFromStubs(t *testing.T) {
	t.Helper()
	origSnap, origUsers, origFind, origCat, origClaim := svcReadAccessSnapshot, svcDirectoryUsers, svcFindUser, PlannerCatalogue, svcClaimPlanJoined
	t.Cleanup(func() {
		svcReadAccessSnapshot, svcDirectoryUsers, svcFindUser, PlannerCatalogue, svcClaimPlanJoined = origSnap, origUsers, origFind, origCat, origClaim
	})
	svcReadAccessSnapshot = func(ctx context.Context, ids []string) (db.AccessSnapshot, error) {
		rules, err := svcGetActiveMappingRules(ctx)
		if err != nil {
			return db.AccessSnapshot{}, err
		}
		snap := db.AccessSnapshot{People: map[string]db.PersonAccess{}, Rules: rules}
		for _, id := range ids {
			direct, err := svcGetDirectGrantsForUser(ctx, id, false)
			if err != nil {
				return snap, err
			}
			byBundle, err := svcGetUserBundleRolesGrouped(ctx, id)
			if err != nil {
				return snap, err
			}
			revoking, err := svcQueuedRevocations(ctx, id)
			if err != nil {
				return snap, err
			}
			p := db.PersonAccess{Direct: direct, ByBundle: byBundle, Bundles: map[string]bool{}, Revoking: revoking}
			for b := range byBundle {
				p.Bundles[b] = true
			}
			snap.People[id] = p
		}
		return snap, nil
	}
	svcDirectoryUsers = func(context.Context) ([]models.UserProfile, error) { return nil, nil }
	svcFindUser = func(_ context.Context, id string) (models.UserProfile, bool, error) {
		return models.UserProfile{ID: id, Name: id, Status: "active"}, true, nil
	}
	PlannerCatalogue = func(ctx context.Context) ([]models.ProjectCatalog, error) {
		offered := map[string][]models.ProjectRole{}
		add := func(project, role string) {
			offered[project] = append(offered[project], models.ProjectRole{Key: role, Label: role})
		}
		for _, read := range []func() ([]models.BundleRole, error){
			func() ([]models.BundleRole, error) { return svcCascGetRolesForBundle(ctx, "b1") },
			func() ([]models.BundleRole, error) { return svcGetRolesForVersion(ctx, "v-old") },
		} {
			func() {
				// A stub the test didn't set reaches for the database: offer
				// nothing from it.
				defer func() { _ = recover() }()
				if roles, err := read(); err == nil {
					for _, r := range roles {
						add(r.ProjectID, r.RoleKey)
					}
				}
			}()
		}
		func() {
			defer func() { _ = recover() }()
			if includes, err := svcGetWorkingIncludes(ctx, "b1"); err == nil {
				for _, in := range includes {
					if roles, err := svcGetRolesForVersion(ctx, in.ChildVersionID); err == nil {
						for _, r := range roles {
							add(r.ProjectID, r.RoleKey)
						}
					}
				}
			}
		}()
		func() {
			defer func() { _ = recover() }()
			if rules, err := svcGetActiveMappingRules(ctx); err == nil {
				for _, r := range rules {
					add(r.SourceProject, r.SourceRole)
					add(r.TargetProject, r.TargetRole)
				}
			}
		}()
		var out []models.ProjectCatalog
		for p, roles := range offered {
			out = append(out, models.ProjectCatalog{ID: p, Name: p, Roles: roles})
		}
		return out, nil
	}
}

// publishNow is a publish reviewed and applied, as the publish handler does.
func publishNow(t *testing.T, req PublishRequest) (BulkPlan, models.BundleVersion, error) {
	t.Helper()
	plan, _, err := applyNow(t, BulkRequest{Op: OpPublish, BundleID: req.BundleID, Migrate: req.Migrate,
		Includers: req.Includers, Note: req.Note, Actor: "admin"})
	return plan, plan.Version, err
}

// moveNow is a move reviewed and applied, as the move handler does.
func moveNow(t *testing.T, req MoveHoldersRequest) (BulkPlan, error) {
	t.Helper()
	plan, _, err := applyNow(t, BulkRequest{Op: OpMove, BundleID: req.BundleID, VersionID: req.VersionID,
		UserIDs: req.UserIDs, Actor: "operator"})
	return plan, err
}

// applyNow plans a bulk op and applies the plan it issued, as the handler
// does: the stored rows are the preview's, so the apply's fresh read must
// agree with them. A publish nobody holds cites no plan.
func applyNow(t *testing.T, req BulkRequest) (BulkPlan, map[string]Change, error) {
	t.Helper()
	ctx := context.Background()
	preview, err := rehearsePlanned(ctx, req)
	if err != nil {
		return preview, nil, err
	}
	var subjects []db.PlanSubject
	for _, o := range preview.Outcomes {
		subjects = append(subjects, db.PlanSubject{SubjectID: o.UserID, Fingerprint: o.Fingerprint,
			Outcome: db.PlanOutcome{Effect: o.Effect, CancelIDs: o.CancelIDs}})
	}
	svcClaimPlanJoined = func(_ context.Context, cite db.PlanCitation) (db.Plan, []db.PlanSubject, error) {
		if cite.RequestFingerprint != preview.RequestFingerprint {
			return db.Plan{}, nil, db.ErrPlanRequestMismatch
		}
		return db.Plan{}, subjects, nil
	}
	cite := db.PlanCitation{PlanID: "plan-1"}
	if len(subjects) == 0 {
		cite.PlanID = ""
	}
	return ApplyPlannedBulk(ctx, req, cite)
}
