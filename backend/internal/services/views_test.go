package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"syndra/internal/db"
	"syndra/internal/directory"
	"syndra/internal/models"
)

// resetGovernanceDeps captures and restores all governance/lineage injectable vars.
func resetGovernanceDeps(t *testing.T) {
	t.Helper()
	// The access view carries a third band now, so it makes a third read. Left
	// unstubbed it reaches a nil pool, which would make every test in this file
	// fail for a reason none of them is about.
	defer func() {
		svcAllowancesForSubject = func(context.Context, string) ([]db.Allowance, error) { return nil, nil }
	}()
	// And the revocation count, for the same reason: the indicators now carry
	// it, and left unstubbed it reaches a nil pool.
	origRevocations := svcCountUnconfirmedRevocations
	t.Cleanup(func() { svcCountUnconfirmedRevocations = origRevocations })
	svcCountUnconfirmedRevocations = func(context.Context) (db.UnconfirmedRevocationSummary, error) {
		return db.UnconfirmedRevocationSummary{}, nil
	}
	// And the holds-due count, for the same reason again: the indicators grew a
	// sixth read and an unstubbed one reaches a nil pool, failing every test in
	// this file for a reason none of them is about.
	origHoldsDue := svcAllowancesDueForReview
	t.Cleanup(func() { svcAllowancesDueForReview = origHoldsDue })
	svcAllowancesDueForReview = func(context.Context) ([]db.Allowance, error) { return nil, nil }
	origGetRequests := svcGetAccessRequests
	origGetExpiring := svcGetExpiringDirectGrants
	origGetAllBundles := svcGetAllBundles
	origGetBundlesForUser := svcGetBundlesForUser
	origGetRolesForBundle := svcGetRolesForBundle
	origUserBundleRoles := svcGetUserBundleRolesGrouped
	origGetDirectGrants := svcGetDirectGrantsForUser
	origGetRules := svcGetActiveMappingRules
	origCount := svcCountPendingPropagations
	origReachable := svcZitadelReachable
	origCountDrift := svcCountPendingDrift
	origTopDrift := svcGetTopDrift
	origAllowances := svcAllowancesForSubject
	t.Cleanup(func() {
		svcAllowancesForSubject = origAllowances
		svcGetAccessRequests = origGetRequests
		svcGetExpiringDirectGrants = origGetExpiring
		svcGetAllBundles = origGetAllBundles
		svcGetBundlesForUser = origGetBundlesForUser
		svcGetRolesForBundle = origGetRolesForBundle
		svcGetUserBundleRolesGrouped = origUserBundleRoles
		svcGetDirectGrantsForUser = origGetDirectGrants
		svcGetActiveMappingRules = origGetRules
		svcCountPendingPropagations = origCount
		svcZitadelReachable = origReachable
		svcCountPendingDrift = origCountDrift
		svcGetTopDrift = origTopDrift
	})

	// Default: this person holds nothing through any bundle. Version-aware
	// resolution is the path every closure now takes, so a harness that left it
	// pointing at the real database would fail on the pool, not on the case.
	svcGetUserBundleRolesGrouped = func(context.Context, string) (map[string][]models.BundleRole, error) {
		return nil, nil
	}
	// Safe baseline so Governance() tests don't hit the nil PG pool / MgmtClient
	// via the pending-propagation/drift summary blocks. Tests override as needed.
	svcCountPendingPropagations = func(context.Context) (int, error) { return 0, nil }
	svcZitadelReachable = func(context.Context) bool { return false }
	svcCountPendingDrift = func(context.Context) (int, error) { return 0, nil }
	svcGetTopDrift = func(context.Context, int) ([]models.DriftItem, error) { return nil, nil }
}

// --- Governance tests ---

func TestGovernance_NilSafeCollections(t *testing.T) {
	resetGovernanceDeps(t)

	svcGetAccessRequests = func(ctx context.Context, status string) ([]models.AccessRequest, error) {
		return nil, nil
	}
	svcGetExpiringDirectGrants = func(ctx context.Context, within time.Duration) ([]models.DirectGrant, error) {
		return nil, nil
	}
	svcGetAllBundles = func(ctx context.Context) ([]models.Bundle, error) {
		return []models.Bundle{}, nil
	}
	svcGetBundlesForUser = func(ctx context.Context, userID string) ([]models.Bundle, error) {
		return nil, nil
	}
	svcGetUserBundleRolesGrouped = func(ctx context.Context, userID string) (map[string][]models.BundleRole, error) {
		return nil, nil
	}
	svcGetRolesForBundle = func(ctx context.Context, bundleID string) ([]models.BundleRole, error) {
		return nil, nil
	}

	summary, err := Governance(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify nil-safety by marshaling to JSON — nil slices serialize as "null"
	data, err := json.Marshal(summary)
	if err != nil {
		t.Fatalf("failed to marshal summary: %v", err)
	}
	body := string(data)
	if strings.Contains(body, `"pending_requests":null`) {
		t.Fatalf("PendingRequests serialized as null: %s", body)
	}
	if strings.Contains(body, `"expiring_grants":null`) {
		t.Fatalf("ExpiringGrants serialized as null: %s", body)
	}
}

func TestGovernance_PendingCountMatchesSliceLength(t *testing.T) {
	resetGovernanceDeps(t)

	svcGetAccessRequests = func(ctx context.Context, status string) ([]models.AccessRequest, error) {
		return []models.AccessRequest{
			{ID: "r1", Status: "pending"},
			{ID: "r2", Status: "pending"},
			{ID: "r3", Status: "pending"},
		}, nil
	}
	svcGetExpiringDirectGrants = func(ctx context.Context, within time.Duration) ([]models.DirectGrant, error) {
		return []models.DirectGrant{}, nil
	}
	svcGetAllBundles = func(ctx context.Context) ([]models.Bundle, error) {
		return []models.Bundle{}, nil
	}

	summary, err := Governance(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(summary.PendingRequests) != 3 {
		t.Fatalf("expected 3 pending requests, got %d", len(summary.PendingRequests))
	}
}

func TestGovernance_PendingPropagationBlock(t *testing.T) {
	resetGovernanceDeps(t)

	svcGetAccessRequests = func(context.Context, string) ([]models.AccessRequest, error) { return nil, nil }
	svcGetExpiringDirectGrants = func(context.Context, time.Duration) ([]models.DirectGrant, error) { return nil, nil }
	svcGetAllBundles = func(context.Context) ([]models.Bundle, error) { return []models.Bundle{}, nil }
	svcCountPendingPropagations = func(context.Context) (int, error) { return 4, nil }
	svcZitadelReachable = func(context.Context) bool { return true }

	summary, err := Governance(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary.PendingPropagation.Count != 4 || !summary.PendingPropagation.ZitadelReachable {
		t.Fatalf("unexpected pending_propagation block: %+v", summary.PendingPropagation)
	}
}

func TestGovernance_PendingPropagationCountErrorDegradesToZero(t *testing.T) {
	resetGovernanceDeps(t)

	svcGetAccessRequests = func(context.Context, string) ([]models.AccessRequest, error) { return nil, nil }
	svcGetExpiringDirectGrants = func(context.Context, time.Duration) ([]models.DirectGrant, error) { return nil, nil }
	svcGetAllBundles = func(context.Context) ([]models.Bundle, error) { return []models.Bundle{}, nil }
	svcCountPendingPropagations = func(context.Context) (int, error) { return 0, context.DeadlineExceeded }
	svcZitadelReachable = func(context.Context) bool { return false }

	summary, err := Governance(context.Background())
	if err != nil {
		t.Fatalf("a count error must NOT fail the whole summary: %v", err)
	}
	if summary.PendingPropagation.Count != 0 {
		t.Fatalf("count error must degrade to 0, got %d", summary.PendingPropagation.Count)
	}
}

func TestGovernance_UnusedBundleHintGenerated(t *testing.T) {
	resetGovernanceDeps(t)

	svcGetAccessRequests = func(ctx context.Context, status string) ([]models.AccessRequest, error) {
		return []models.AccessRequest{}, nil
	}
	svcGetExpiringDirectGrants = func(ctx context.Context, within time.Duration) ([]models.DirectGrant, error) {
		return []models.DirectGrant{}, nil
	}
	svcGetAllBundles = func(ctx context.Context) ([]models.Bundle, error) {
		return []models.Bundle{{ID: "b1", Name: "Orphan Bundle"}}, nil
	}
	svcGetBundlesForUser = func(ctx context.Context, userID string) ([]models.Bundle, error) {
		// No users are assigned to any bundle.
		return []models.Bundle{}, nil
	}
	svcGetRolesForBundle = func(ctx context.Context, bundleID string) ([]models.BundleRole, error) {
		return []models.BundleRole{}, nil
	}
	svcGetUserBundleRolesGrouped = func(ctx context.Context, userID string) (map[string][]models.BundleRole, error) {
		grouped := map[string][]models.BundleRole{}
		for _, r := range []models.BundleRole{} {
			grouped[r.BundleID] = append(grouped[r.BundleID], r)
		}
		return grouped, nil
	}

	summary, err := Governance(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	hasUnusedHint := false
	for _, hint := range summary.CleanupHints {
		if strings.Contains(hint, "Orphan Bundle") {
			hasUnusedHint = true
			break
		}
	}
	if !hasUnusedHint {
		t.Fatalf("expected cleanup hint for unused bundle, got hints: %v", summary.CleanupHints)
	}
}

// --- ExplainUserAccess / lineage tests ---

func TestExplainUserAccess_NilSafeCollections(t *testing.T) {
	resetGovernanceDeps(t)

	svcGetDirectGrantsForUser = func(ctx context.Context, userID string, includeExpired bool) ([]models.DirectGrant, error) {
		return nil, nil
	}
	svcGetBundlesForUser = func(ctx context.Context, userID string) ([]models.Bundle, error) {
		return nil, nil
	}
	svcGetActiveMappingRules = func(ctx context.Context) ([]models.MappingRule, error) {
		return nil, nil
	}
	svcGetUserBundleRolesGrouped = func(ctx context.Context, userID string) (map[string][]models.BundleRole, error) {
		return nil, nil
	}
	svcGetRolesForBundle = func(ctx context.Context, bundleID string) ([]models.BundleRole, error) {
		return nil, nil
	}

	view, err := ExplainUserAccess(context.Background(), "dev_admin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if view.Bundles == nil {
		t.Fatalf("Bundles should not be nil")
	}
	for _, proj := range view.Projects {
		if proj.SourceRoles == nil {
			t.Fatalf("SourceRoles should not be nil for project %s", proj.ProjectID)
		}
		if proj.DerivedRoles == nil {
			t.Fatalf("DerivedRoles should not be nil for project %s", proj.ProjectID)
		}
	}
}

func TestExplainUserAccess_SourceVsDerivedLabeling(t *testing.T) {
	resetGovernanceDeps(t)

	svcGetDirectGrantsForUser = func(ctx context.Context, userID string, includeExpired bool) ([]models.DirectGrant, error) {
		return []models.DirectGrant{
			{UserID: userID, ProjectID: "p1", RoleKey: "r1"},
		}, nil
	}
	svcGetBundlesForUser = func(ctx context.Context, userID string) ([]models.Bundle, error) {
		return []models.Bundle{}, nil
	}
	svcGetActiveMappingRules = func(ctx context.Context) ([]models.MappingRule, error) {
		return []models.MappingRule{
			{ID: "rule-1", SourceProject: "p1", SourceRole: "r1", TargetProject: "p2", TargetRole: "r2"},
		}, nil
	}
	svcGetUserBundleRolesGrouped = func(ctx context.Context, userID string) (map[string][]models.BundleRole, error) {
		return nil, nil
	}
	svcGetRolesForBundle = func(ctx context.Context, bundleID string) ([]models.BundleRole, error) {
		return nil, nil
	}

	view, err := ExplainUserAccess(context.Background(), "dev_admin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var p1Bucket, p2Bucket *models.ProjectAccessView
	for i := range view.Projects {
		switch view.Projects[i].ProjectID {
		case "p1":
			p1Bucket = &view.Projects[i]
		case "p2":
			p2Bucket = &view.Projects[i]
		}
	}

	if p1Bucket == nil {
		t.Fatalf("expected project p1 in view")
	}
	if p2Bucket == nil {
		t.Fatalf("expected project p2 in view (from derived rule)")
	}

	if len(p1Bucket.SourceRoles) == 0 || p1Bucket.SourceRoles[0].RoleKey != "r1" {
		t.Fatalf("expected p1:r1 in SourceRoles, got %v", p1Bucket.SourceRoles)
	}
	if !p1Bucket.SourceRoles[0].IsSource {
		t.Fatalf("expected p1:r1 IsSource=true")
	}

	if len(p2Bucket.DerivedRoles) == 0 || p2Bucket.DerivedRoles[0].RoleKey != "r2" {
		t.Fatalf("expected p2:r2 in DerivedRoles, got %v", p2Bucket.DerivedRoles)
	}
	if p2Bucket.DerivedRoles[0].IsSource {
		t.Fatalf("expected p2:r2 IsSource=false")
	}
}

func TestExplainUserAccess_BundleGrantLabeled(t *testing.T) {
	resetGovernanceDeps(t)

	svcGetDirectGrantsForUser = func(ctx context.Context, userID string, includeExpired bool) ([]models.DirectGrant, error) {
		return []models.DirectGrant{}, nil
	}
	svcGetBundlesForUser = func(ctx context.Context, userID string) ([]models.Bundle, error) {
		return []models.Bundle{{ID: "b1", Name: "Engineering", PinnedVersion: 2}}, nil
	}
	// What this person gets from b1 comes from the version they are pinned to,
	// not from the bundle's working copy.
	svcGetUserBundleRolesGrouped = func(ctx context.Context, userID string) (map[string][]models.BundleRole, error) {
		return map[string][]models.BundleRole{
			"b1": {{BundleID: "b1", ProjectID: "p1", RoleKey: "r1"}},
		}, nil
	}
	svcGetActiveMappingRules = func(ctx context.Context) ([]models.MappingRule, error) {
		return []models.MappingRule{}, nil
	}

	view, err := ExplainUserAccess(context.Background(), "dev_admin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var p1Bucket *models.ProjectAccessView
	for i := range view.Projects {
		if view.Projects[i].ProjectID == "p1" {
			p1Bucket = &view.Projects[i]
			break
		}
	}
	if p1Bucket == nil {
		t.Fatalf("expected project p1 in view")
	}
	if len(p1Bucket.SourceRoles) == 0 {
		t.Fatalf("expected bundle-assigned role in SourceRoles")
	}
	role := p1Bucket.SourceRoles[0]
	if role.RoleKey != "r1" {
		t.Fatalf("expected r1, got %s", role.RoleKey)
	}
	if len(role.Reasons) == 0 || role.Reasons[0].Kind != "bundle" {
		t.Fatalf("expected reason kind=bundle, got %v", role.Reasons)
	}
}

func TestExplainUserAccess_UnknownUser_ReturnsError(t *testing.T) {
	resetGovernanceDeps(t)

	_, err := ExplainUserAccess(context.Background(), "user-does-not-exist-xyz")
	if err == nil {
		t.Fatalf("expected error for unknown user, got nil")
	}
}

func TestExplainUserAccess_MultiHopDerivation(t *testing.T) {
	resetGovernanceDeps(t)

	svcGetDirectGrantsForUser = func(ctx context.Context, userID string, includeExpired bool) ([]models.DirectGrant, error) {
		return []models.DirectGrant{
			{UserID: userID, ProjectID: "p1", RoleKey: "r1"},
		}, nil
	}
	svcGetBundlesForUser = func(ctx context.Context, userID string) ([]models.Bundle, error) {
		return []models.Bundle{}, nil
	}
	svcGetActiveMappingRules = func(ctx context.Context) ([]models.MappingRule, error) {
		return []models.MappingRule{
			{ID: "rule-1", SourceProject: "p1", SourceRole: "r1", TargetProject: "p2", TargetRole: "r2"},
			{ID: "rule-2", SourceProject: "p2", SourceRole: "r2", TargetProject: "p3", TargetRole: "r3"},
		}, nil
	}
	svcGetRolesForBundle = func(ctx context.Context, bundleID string) ([]models.BundleRole, error) {
		return nil, nil
	}

	view, err := ExplainUserAccess(context.Background(), "dev_admin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	foundP3 := false
	for _, proj := range view.Projects {
		if proj.ProjectID == "p3" {
			for _, role := range proj.DerivedRoles {
				if role.RoleKey == "r3" {
					foundP3 = true
				}
			}
		}
	}
	if !foundP3 {
		t.Fatalf("expected p3:r3 in derived roles after multi-hop, got projects: %v", view.Projects)
	}
}

func TestUpsertRoleDeduplicatesReason(t *testing.T) {
	roleMap := map[roleKey]*models.EffectiveRole{}
	key := roleKey{projectID: "p1", roleKey: "admin"}
	reason := models.RoleReason{
		Kind:        "bundle",
		Description: "Granted by bundle Engineering",
		BundleID:    "b1",
		BundleName:  "Engineering",
	}

	ctx := context.Background()
	firstAdded := upsertRole(ctx, roleMap, key, true, reason)
	secondAdded := upsertRole(ctx, roleMap, key, true, reason)

	if !firstAdded {
		t.Fatalf("expected first insert to add role")
	}
	if secondAdded {
		t.Fatalf("expected duplicate reason to be ignored")
	}

	current := roleMap[key]
	if current == nil {
		t.Fatalf("expected role to exist after insert")
	}
	if len(current.Reasons) != 1 {
		t.Fatalf("expected exactly one reason, got %d", len(current.Reasons))
	}
	if !current.IsSource {
		t.Fatalf("expected source flag to remain true")
	}
}

// --- accessSnapshot tests (B3) -------------------------------------------
//
// snapshotFixtureDirectory is a Source implementation that returns a fixed
// number of users, applications, and projects (and minimal role/app metadata)
// so call-count assertions on collectUserRoles are deterministic regardless
// of the live or demo seed catalog.
type snapshotFixtureDirectory struct {
	users []models.UserProfile
	apps  []models.ApplicationCatalog
	projs []models.ProjectCatalog
}

func (d *snapshotFixtureDirectory) Users(context.Context) ([]models.UserProfile, error) {
	return d.users, nil
}
func (d *snapshotFixtureDirectory) FindUser(_ context.Context, id string) (models.UserProfile, bool, error) {
	for _, u := range d.users {
		if u.ID == id {
			return u, true, nil
		}
	}
	return models.UserProfile{}, false, nil
}
func (d *snapshotFixtureDirectory) Projects(context.Context) ([]models.ProjectCatalog, error) {
	return d.projs, nil
}
func (d *snapshotFixtureDirectory) FindProject(_ context.Context, id string) (models.ProjectCatalog, bool, error) {
	for _, p := range d.projs {
		if p.ID == id {
			return p, true, nil
		}
	}
	return models.ProjectCatalog{}, false, nil
}
func (d *snapshotFixtureDirectory) Applications(context.Context) ([]models.ApplicationCatalog, error) {
	return d.apps, nil
}
func (d *snapshotFixtureDirectory) FindApplication(_ context.Context, id string) (models.ApplicationCatalog, bool, error) {
	for _, a := range d.apps {
		if a.ID == id {
			return a, true, nil
		}
	}
	return models.ApplicationCatalog{}, false, nil
}
func (d *snapshotFixtureDirectory) RoleKeysForProject(context.Context, string) ([]string, error) {
	return nil, nil
}
func (d *snapshotFixtureDirectory) ProjectName(_ context.Context, id string) (string, error) {
	return id, nil
}
func (d *snapshotFixtureDirectory) Tag() string              { return "snapshot-fixture" }
func (d *snapshotFixtureDirectory) InvalidateAll()           {}
func (d *snapshotFixtureDirectory) InvalidateProject(string) {}
func (d *snapshotFixtureDirectory) InvalidateUsers()         {}

// setupSnapshotTestFixtures swaps directory.Default for a fixture source
// seeded with the requested number of users, apps, and projects; neutralises
// every service-layer DB injectable that views.go consults; and restores
// originals via t.Cleanup. Returns nothing — tests configure collectUserRolesHook
// themselves after calling this helper.
func setupSnapshotTestFixtures(t *testing.T, numUsers, numApps, numProjects int) {
	t.Helper()
	resetGovernanceDeps(t)

	users := make([]models.UserProfile, 0, numUsers)
	for i := 0; i < numUsers; i++ {
		users = append(users, models.UserProfile{ID: fmt.Sprintf("u%d", i), Name: fmt.Sprintf("User %d", i)})
	}
	projs := make([]models.ProjectCatalog, 0, numProjects)
	for i := 0; i < numProjects; i++ {
		projs = append(projs, models.ProjectCatalog{ID: fmt.Sprintf("p%d", i), Name: fmt.Sprintf("Project %d", i)})
	}
	apps := make([]models.ApplicationCatalog, 0, numApps)
	for i := 0; i < numApps; i++ {
		apps = append(apps, models.ApplicationCatalog{
			ID:        fmt.Sprintf("a%d", i),
			Name:      fmt.Sprintf("App %d", i),
			ProjectID: fmt.Sprintf("p%d", i%max1(numProjects)),
		})
	}

	origDir := directory.Default
	directory.Default = &snapshotFixtureDirectory{users: users, apps: apps, projs: projs}
	t.Cleanup(func() { directory.Default = origDir })

	svcGetDirectGrantsForUser = func(context.Context, string, bool) ([]models.DirectGrant, error) {
		return nil, nil
	}
	svcGetBundlesForUser = func(context.Context, string) ([]models.Bundle, error) {
		return nil, nil
	}
	svcGetRolesForBundle = func(context.Context, string) ([]models.BundleRole, error) {
		return nil, nil
	}
	svcGetActiveMappingRules = func(context.Context) ([]models.MappingRule, error) {
		return nil, nil
	}
	svcGetAllBundles = func(context.Context) ([]models.Bundle, error) {
		return nil, nil
	}
	svcGetAllDirectGrants = func(context.Context, bool) ([]models.DirectGrant, error) {
		return nil, nil
	}
	svcGetAccessRequests = func(context.Context, string) ([]models.AccessRequest, error) {
		return nil, nil
	}
	svcGetExpiringDirectGrants = func(context.Context, time.Duration) ([]models.DirectGrant, error) {
		return nil, nil
	}
}

func max1(n int) int {
	if n < 1 {
		return 1
	}
	return n
}

func TestListApplications_CollectsUserRolesExactlyOncePerUser(t *testing.T) {
	setupSnapshotTestFixtures(t, 3, 5, 1)

	var calls int
	origCollect := collectUserRolesHook
	collectUserRolesHook = func(ctx context.Context, userID string) (map[roleKey]*models.EffectiveRole, []models.Bundle, error) {
		calls++
		return origCollect(ctx, userID)
	}
	t.Cleanup(func() { collectUserRolesHook = origCollect })

	if _, err := ListApplications(context.Background()); err != nil {
		t.Fatalf("ListApplications: %v", err)
	}
	if calls != 3 {
		t.Fatalf("expected collectUserRoles called exactly 3 times (once per user); got %d", calls)
	}
}

func TestListProjects_CollectsUserRolesExactlyOncePerUser(t *testing.T) {
	setupSnapshotTestFixtures(t, 3, 5, 4)

	var calls int
	origCollect := collectUserRolesHook
	collectUserRolesHook = func(ctx context.Context, userID string) (map[roleKey]*models.EffectiveRole, []models.Bundle, error) {
		calls++
		return origCollect(ctx, userID)
	}
	t.Cleanup(func() { collectUserRolesHook = origCollect })

	if _, err := ListProjects(context.Background()); err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
	if calls != 3 {
		t.Fatalf("expected 3 calls (once per user); got %d", calls)
	}
}

// 8.12 — an unread band must not look like "no carve-outs". Empty and unread
// are identical to a surface, and one of them means this person is suspended
// from something the view cannot show.
func TestAnUnreadableAllowanceBandIsSaidOutLoudInTheView(t *testing.T) {
	resetGovernanceDeps(t)

	svcGetDirectGrantsForUser = func(context.Context, string, bool) ([]models.DirectGrant, error) { return nil, nil }
	svcGetBundlesForUser = func(context.Context, string) ([]models.Bundle, error) { return nil, nil }
	svcGetActiveMappingRules = func(context.Context) ([]models.MappingRule, error) { return nil, nil }
	svcGetUserBundleRolesGrouped = func(context.Context, string) (map[string][]models.BundleRole, error) { return nil, nil }
	svcAllowancesForSubject = func(context.Context, string) ([]db.Allowance, error) {
		return nil, fmt.Errorf("db down")
	}

	svcGetRolesForBundle = func(context.Context, string) ([]models.BundleRole, error) { return nil, nil }

	view, err := ExplainUserAccess(context.Background(), "dev_admin")
	if err != nil {
		// The view still renders: one unreadable band must not deny an operator
		// the rest of somebody's access.
		t.Fatalf("the view must still be produced: %v", err)
	}
	var said bool
	for _, hint := range view.CleanupHints {
		if strings.Contains(hint, "Carve-outs could not be read") {
			said = true
		}
	}
	if !said {
		t.Fatalf("the view must say the band is incomplete: %v", view.CleanupHints)
	}
	if view.Allowances == nil {
		t.Error("and the band must be a list rather than nil, so a surface renders empty rather than crashing")
	}
}
