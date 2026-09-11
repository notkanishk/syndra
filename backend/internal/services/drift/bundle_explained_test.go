package drift

import (
	"context"
	"testing"

	"syndra/internal/db"
	"syndra/internal/models"
)

// A role a bundle grants is not drift.
//
// Found on the production deployment. An operator granted somebody three roles
// in Zitadel by hand, so the webhook raised three findings — correctly, Syndra
// had no intent behind them. They then built a bundle carrying the same three
// roles and assigned it, which gave Syndra an intent, projected it, and drained
// it successfully. The findings stayed, and every sweep re-affirmed them.
//
// The cause was that the sweep's classification asked three questions — direct
// grant, mapping rule, exclusion — and bundles were not among them. The
// webhook's own check never had this hole: it goes through
// services.UserExpectsRole, which counts direct grants, bundles AND rules. So
// the two detectors disagreed, and the sweep's opinion was the one written to
// the table.
//
// Left alone it would have flagged every projected bundle role for every holder
// for ever.

func bundleHolderGrant(userID, projectID, roleKey string) db.BundleDerivedGrant {
	return db.BundleDerivedGrant{UserID: userID, ProjectID: projectID, RoleKey: roleKey}
}

func TestSweep_ABundleRoleIsNotDrift(t *testing.T) {
	stubSweep(t)

	t.Cleanup(swap(&allObservedGrants, func(context.Context) ([]db.ObservedGrant, error) {
		return []db.ObservedGrant{observedGrant("g1", "shikha", "p-admin", "admin-staff")}, nil
	}))
	t.Cleanup(swap(&svcAllBundleDerivedGrants, func(context.Context) ([]db.BundleDerivedGrant, error) {
		return []db.BundleDerivedGrant{bundleHolderGrant("shikha", "p-admin", "admin-staff")}, nil
	}))

	var raised []string
	t.Cleanup(swap(&upsertDriftItem, func(_ context.Context, _, userID, projectID string, roles []string, _, _, _ string, _ db.DriftEvidence) (string, bool, error) {
		raised = append(raised, userID+"/"+projectID+"/"+roles[0])
		return "d1", true, nil
	}))

	res, err := Sweep(context.Background())
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if len(raised) != 0 {
		t.Fatalf("a grant the bundle accounts for was raised as drift: %v", raised)
	}
	if res.DriftItemsCreated != 0 {
		t.Errorf("expected no findings, got %d", res.DriftItemsCreated)
	}
}

// The bundle has to explain it through the version the person is PINNED to.
// Reading the working copy would explain a live grant on the strength of an
// unpublished edit — the same confusion that put unpublished roles into an
// assignment preview. `GetAllBundleDerivedGrants` joins on `a.version_id`, so
// a role only present in a later version explains nothing here.
func TestSweep_ARoleTheHoldersVersionDoesNotCarryIsStillDrift(t *testing.T) {
	stubSweep(t)

	t.Cleanup(swap(&allObservedGrants, func(context.Context) ([]db.ObservedGrant, error) {
		return []db.ObservedGrant{observedGrant("g1", "shikha", "p-admin", "admin-staff")}, nil
	}))
	// Their pin carries a DIFFERENT role. Nothing accounts for admin-staff.
	t.Cleanup(swap(&svcAllBundleDerivedGrants, func(context.Context) ([]db.BundleDerivedGrant, error) {
		return []db.BundleDerivedGrant{bundleHolderGrant("shikha", "p-admin", "something-else")}, nil
	}))

	var raised []string
	t.Cleanup(swap(&upsertDriftItem, func(_ context.Context, _, userID, projectID string, roles []string, _, _, _ string, _ db.DriftEvidence) (string, bool, error) {
		raised = append(raised, userID+"/"+projectID+"/"+roles[0])
		return "d1", true, nil
	}))

	if _, err := Sweep(context.Background()); err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if len(raised) != 1 {
		t.Fatalf("expected the unexplained grant to be raised, got %v", raised)
	}
}

// A failed read of the bundle inventory must abort, not degrade to an empty
// set. An empty set explains nothing, so every projected bundle role in the
// deployment would be written down as drift — the exact defect, arrived at from
// the other direction. Same rule the rules and exclusions reads follow.
func TestSweep_AFailedBundleReadAbortsRatherThanFlaggingEverything(t *testing.T) {
	stubSweep(t)

	t.Cleanup(swap(&allObservedGrants, func(context.Context) ([]db.ObservedGrant, error) {
		return []db.ObservedGrant{observedGrant("g1", "shikha", "p-admin", "admin-staff")}, nil
	}))
	t.Cleanup(swap(&svcAllBundleDerivedGrants, func(context.Context) ([]db.BundleDerivedGrant, error) {
		return nil, context.DeadlineExceeded
	}))

	wrote := false
	t.Cleanup(swap(&upsertDriftItem, func(context.Context, string, string, string, []string, string, string, string, db.DriftEvidence) (string, bool, error) {
		wrote = true
		return "d1", true, nil
	}))

	if _, err := Sweep(context.Background()); err == nil {
		t.Fatal("expected the sweep to abort when it cannot read what the bundles account for")
	}
	if wrote {
		t.Fatal("a finding was written from an inventory that failed to load")
	}
}

// The findings the bug already wrote have to be closable by the sweep.
//
// They do not age out — a pending_triage row waits for a human — and the only
// resolution the UI offers for an unexplained grant is Adopt, which writes a
// direct-grant row. On a finding a bundle already explains that is worse than
// leaving it: the redundant grant means removing the bundle would revoke
// nothing, so an operator takes a bundle away and the person keeps the access.
func TestSweep_RetractsAFindingItCanNowExplain(t *testing.T) {
	stubSweep(t)

	t.Cleanup(swap(&allObservedGrants, func(context.Context) ([]db.ObservedGrant, error) {
		return []db.ObservedGrant{observedGrant("g1", "shikha", "p-admin", "admin-staff")}, nil
	}))
	t.Cleanup(swap(&svcAllBundleDerivedGrants, func(context.Context) ([]db.BundleDerivedGrant, error) {
		return []db.BundleDerivedGrant{bundleHolderGrant("shikha", "p-admin", "admin-staff")}, nil
	}))
	t.Cleanup(swap(&svcPendingDriftItems, func(context.Context, string) ([]models.DriftItem, error) {
		return []models.DriftItem{{
			ID: "d-stale", Target: db.TargetZitadel, DriftType: db.DriftTargetOnly,
			UserID: "shikha", ProjectID: "p-admin", RoleKeys: []string{"admin-staff"},
		}}, nil
	}))

	var retracted []string
	t.Cleanup(swap(&retractExplainedDrift, func(_ context.Context, id, _, _, because string) error {
		retracted = append(retracted, id+" because "+because)
		return nil
	}))

	res, err := Sweep(context.Background())
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if len(retracted) != 1 {
		t.Fatalf("expected the stale finding to be retracted, got %v", retracted)
	}
	if res.DriftItemsRetracted != 1 {
		t.Errorf("the result must report the retraction, got %d", res.DriftItemsRetracted)
	}
}

// And it must NOT retract one that is still unexplained. This is the assertion
// that keeps the retraction pass from becoming a way to empty the queue.
func TestSweep_LeavesAFindingItCannotExplain(t *testing.T) {
	stubSweep(t)

	t.Cleanup(swap(&svcPendingDriftItems, func(context.Context, string) ([]models.DriftItem, error) {
		return []models.DriftItem{{
			ID: "d-real", Target: db.TargetZitadel, DriftType: db.DriftTargetOnly,
			UserID: "someone", ProjectID: "p-admin", RoleKeys: []string{"admin-staff"},
		}}, nil
	}))

	retracted := 0
	t.Cleanup(swap(&retractExplainedDrift, func(context.Context, string, string, string, string) error {
		retracted++
		return nil
	}))

	if _, err := Sweep(context.Background()); err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if retracted != 0 {
		t.Fatal("a finding nothing accounts for was retracted")
	}
}

// A row naming two roles where only one is explained is still a finding about
// the other. Half-retracting it would erase the half nobody has looked at.
func TestSweep_DoesNotRetractAPartlyExplainedFinding(t *testing.T) {
	stubSweep(t)

	t.Cleanup(swap(&svcAllBundleDerivedGrants, func(context.Context) ([]db.BundleDerivedGrant, error) {
		return []db.BundleDerivedGrant{bundleHolderGrant("shikha", "p-admin", "admin-staff")}, nil
	}))
	t.Cleanup(swap(&svcPendingDriftItems, func(context.Context, string) ([]models.DriftItem, error) {
		return []models.DriftItem{{
			ID: "d-mixed", Target: db.TargetZitadel, DriftType: db.DriftTargetOnly,
			UserID: "shikha", ProjectID: "p-admin",
			RoleKeys: []string{"admin-staff", "not-accounted-for"},
		}}, nil
	}))

	retracted := 0
	t.Cleanup(swap(&retractExplainedDrift, func(context.Context, string, string, string, string) error {
		retracted++
		return nil
	}))

	if _, err := Sweep(context.Background()); err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if retracted != 0 {
		t.Fatal("a finding was retracted while one of its roles was still unexplained")
	}
}

// A finding an operator's EXCLUSION accounts for must not be closed as
// Syndra's own.
//
// `attributed` means Syndra owns this access. An exclusion means the opposite:
// the operator said the grant belongs to somebody else and Syndra should stop
// asking. Closing it as attributed would write a claim of ownership over access
// that has been explicitly disclaimed — onto the governance record, which is
// the one place that has to read back as what actually happened.
func TestSweep_RetractsAnExcludedFindingAsExternalNotAsOwned(t *testing.T) {
	stubSweep(t)

	t.Cleanup(swap(&svcGetExclusions, func(context.Context, string) ([]models.ExternalGrantExclusion, error) {
		return []models.ExternalGrantExclusion{{
			Target: db.TargetZitadel, UserID: "shikha", ProjectID: "p-admin", RoleKey: "admin-staff",
		}}, nil
	}))
	t.Cleanup(swap(&svcPendingDriftItems, func(context.Context, string) ([]models.DriftItem, error) {
		return []models.DriftItem{{
			ID: "d-external", Target: db.TargetZitadel, DriftType: db.DriftTargetOnly,
			UserID: "shikha", ProjectID: "p-admin", RoleKeys: []string{"admin-staff"},
		}}, nil
	}))

	var gotStatus string
	t.Cleanup(swap(&retractExplainedDrift, func(_ context.Context, _, _, status, _ string) error {
		gotStatus = status
		return nil
	}))

	if _, err := Sweep(context.Background()); err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if gotStatus != db.DriftMarkedExternal {
		t.Fatalf("an excluded finding must close as %q, got %q", db.DriftMarkedExternal, gotStatus)
	}
}

// A row explained by two DIFFERENT sources has no single honest status, so it
// is left for a human rather than closed under whichever came first.
// Prod: a webhook recorded zitadel.grant_removed for a user/project/role, so
// the pending target_only row nothing ever explains is now also a row
// nothing needs to explain — the grant itself is gone. Only retractExplained
// closed rows before this; a row for access that vanished, rather than
// access Syndra came to own, sat forever ("47 items" where Zitadel held 46
// unexplained). closeGoneDrift is the other half: it closes what the sweep's
// COMPLETE read no longer contains at all.
func TestSweep_ClosesAFindingWhoseGrantHasVanished(t *testing.T) {
	stubSweep(t)

	// Zitadel holds nothing for this triple any more.
	t.Cleanup(swap(&allObservedGrants, func(context.Context) ([]db.ObservedGrant, error) {
		return nil, nil
	}))
	t.Cleanup(swap(&svcPendingDriftItems, func(context.Context, string) ([]models.DriftItem, error) {
		return []models.DriftItem{{
			ID: "d-gone", Target: db.TargetZitadel, DriftType: db.DriftTargetOnly,
			UserID: "shikha", ProjectID: "p-admin", RoleKeys: []string{"admin-staff"},
		}}, nil
	}))

	var closed []string
	t.Cleanup(swap(&closeGoneDriftItem, func(_ context.Context, id, target string) error {
		closed = append(closed, id+"@"+target)
		return nil
	}))

	res, err := Sweep(context.Background())
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if len(closed) != 1 || closed[0] != "d-gone@"+db.TargetZitadel {
		t.Fatalf("expected the vanished finding to be closed, got %v", closed)
	}
	if res.DriftItemsClosedGone != 1 {
		t.Errorf("the result must report the closure, got %d", res.DriftItemsClosedGone)
	}
}

// And it must NOT close one that is still present in Zitadel — closing it
// would discard a live finding nobody has triaged.
func TestSweep_LeavesAFindingWhoseGrantIsStillPresent(t *testing.T) {
	stubSweep(t)

	t.Cleanup(swap(&allObservedGrants, func(context.Context) ([]db.ObservedGrant, error) {
		return []db.ObservedGrant{observedGrant("g1", "shikha", "p-admin", "admin-staff")}, nil
	}))
	t.Cleanup(swap(&svcPendingDriftItems, func(context.Context, string) ([]models.DriftItem, error) {
		return []models.DriftItem{{
			ID: "d-live", Target: db.TargetZitadel, DriftType: db.DriftTargetOnly,
			UserID: "shikha", ProjectID: "p-admin", RoleKeys: []string{"admin-staff"},
		}}, nil
	}))

	closed := 0
	t.Cleanup(swap(&closeGoneDriftItem, func(context.Context, string, string) error {
		closed++
		return nil
	}))

	res, err := Sweep(context.Background())
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if closed != 0 {
		t.Fatal("a finding was closed while its grant is still present in Zitadel")
	}
	if res.DriftItemsClosedGone != 0 {
		t.Errorf("expected no closures reported, got %d", res.DriftItemsClosedGone)
	}
}

// Mutation-sensitive: a truncated (or failed) observation cannot tell "gone"
// from "unseen past the cap", so it must close NOTHING. This fails if the
// completeness guard around closeGoneDrift's call site is ever removed —
// everything here is set up exactly as TestSweep_ClosesAFindingWhoseGrantHasVanished
// is, except the observation is incomplete.
func TestSweep_TruncatedObservationClosesNothing(t *testing.T) {
	stubSweep(t)

	t.Cleanup(swap(&latestOrgObservation, func(context.Context) (db.Observation, error) {
		return db.Observation{Scope: "org", ObservedAt: testObservedAt, Complete: false}, nil
	}))
	t.Cleanup(swap(&allObservedGrants, func(context.Context) ([]db.ObservedGrant, error) {
		return nil, nil // Zitadel appears to hold nothing — but the read was incomplete
	}))
	t.Cleanup(swap(&svcPendingDriftItems, func(context.Context, string) ([]models.DriftItem, error) {
		return []models.DriftItem{{
			ID: "d-gone", Target: db.TargetZitadel, DriftType: db.DriftTargetOnly,
			UserID: "shikha", ProjectID: "p-admin", RoleKeys: []string{"admin-staff"},
		}}, nil
	}))

	closed := 0
	t.Cleanup(swap(&closeGoneDriftItem, func(context.Context, string, string) error {
		closed++
		return nil
	}))

	res, err := Sweep(context.Background())
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if !res.Truncated {
		t.Fatal("expected the sweep to report truncated")
	}
	if closed != 0 {
		t.Fatal("a finding was closed from a truncated read — absence cannot be concluded from an incomplete observation")
	}
	if res.DriftItemsClosedGone != 0 {
		t.Errorf("expected no closures reported on a truncated read, got %d", res.DriftItemsClosedGone)
	}
}

func TestSweep_LeavesAFindingWithMixedExplanations(t *testing.T) {
	stubSweep(t)

	t.Cleanup(swap(&svcAllBundleDerivedGrants, func(context.Context) ([]db.BundleDerivedGrant, error) {
		return []db.BundleDerivedGrant{bundleHolderGrant("shikha", "p-admin", "by-bundle")}, nil
	}))
	t.Cleanup(swap(&svcGetExclusions, func(context.Context, string) ([]models.ExternalGrantExclusion, error) {
		return []models.ExternalGrantExclusion{{
			Target: db.TargetZitadel, UserID: "shikha", ProjectID: "p-admin", RoleKey: "by-exclusion",
		}}, nil
	}))
	t.Cleanup(swap(&svcPendingDriftItems, func(context.Context, string) ([]models.DriftItem, error) {
		return []models.DriftItem{{
			ID: "d-mixed-source", Target: db.TargetZitadel, DriftType: db.DriftTargetOnly,
			UserID: "shikha", ProjectID: "p-admin",
			RoleKeys: []string{"by-bundle", "by-exclusion"},
		}}, nil
	}))

	retracted := 0
	t.Cleanup(swap(&retractExplainedDrift, func(context.Context, string, string, string, string) error {
		retracted++
		return nil
	}))

	if _, err := Sweep(context.Background()); err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if retracted != 0 {
		t.Fatal("a finding explained two different ways was closed under one of them")
	}
}
