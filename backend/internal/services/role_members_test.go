package services

import (
	"context"
	"testing"
	"time"

	"mkauth/internal/models"
)

// stubRoleFixtures wires the injectables RoleMembers and GovernanceIndicators
// read, on top of the shared snapshot fixture directory.
func stubRoleFixtures(t *testing.T) {
	t.Helper()
	origGetRole, origAllGrants := svcDbGetRole, svcGetAllDirectGrants
	t.Cleanup(func() {
		svcDbGetRole = origGetRole
		svcGetAllDirectGrants = origAllGrants
	})
	svcDbGetRole = func(context.Context, string, string) (models.Role, error) {
		return models.Role{}, context.Canceled // no local row — best-effort metadata
	}
}

// A person can hold one role through several sources at once. The row must
// carry all of them, ordered Direct → Via bundle → Automatic, because the UI
// names each removal after its own source and renders the strongest first.
func TestRoleMembers_ReturnsEverySourceInFixedOrder(t *testing.T) {
	setupSnapshotTestFixtures(t, 2, 0, 1)
	stubRoleFixtures(t)

	origCollect := collectUserRolesHook
	t.Cleanup(func() { collectUserRolesHook = origCollect })
	collectUserRolesHook = func(_ context.Context, userID string) (map[roleKey]*models.EffectiveRole, []models.Bundle, error) {
		key := roleKey{projectID: "p0", roleKey: "trained"}
		if userID == "u0" {
			// Held three ways, deliberately supplied out of order.
			return map[roleKey]*models.EffectiveRole{
				key: {ProjectID: "p0", RoleKey: "trained", IsSource: true, Reasons: []models.RoleReason{
					{Kind: "mapping", TriggerProject: "p1", TriggerRole: "operator"},
					{Kind: "bundle", BundleName: "Lab Tech"},
					{Kind: "direct"},
				}},
			}, nil, nil
		}
		// u1 holds an unrelated role and must not appear.
		return map[roleKey]*models.EffectiveRole{
			{projectID: "p0", roleKey: "maintainer"}: {ProjectID: "p0", RoleKey: "maintainer"},
		}, nil, nil
	}

	view, err := RoleMembers(context.Background(), "p0", "trained")
	if err != nil {
		t.Fatalf("RoleMembers: %v", err)
	}
	if len(view.Members) != 1 {
		t.Fatalf("expected only holders of p0/trained, got %d members", len(view.Members))
	}

	kinds := []string{}
	for _, r := range view.Members[0].Reasons {
		kinds = append(kinds, r.Kind)
	}
	want := []string{"direct", "bundle", "mapping"}
	for i := range want {
		if i >= len(kinds) || kinds[i] != want[i] {
			t.Fatalf("sources must read Direct → Via bundle → Automatic, got %v", kinds)
		}
	}
	if view.DirectCount != 1 || view.BundleCount != 1 || view.AutomaticCount != 1 {
		t.Errorf("filter pill counts wrong: direct=%d bundle=%d automatic=%d",
			view.DirectCount, view.BundleCount, view.AutomaticCount)
	}
}

// A direct source must carry its grant id: without it the row can only offer a
// removal it cannot perform.
func TestRoleMembers_DirectRowCarriesGrantIDAndExpiry(t *testing.T) {
	setupSnapshotTestFixtures(t, 1, 0, 1)
	stubRoleFixtures(t)

	expires := time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC)
	svcGetAllDirectGrants = func(context.Context, bool) ([]models.DirectGrant, error) {
		return []models.DirectGrant{{
			ID: "g_88", UserID: "u0", ProjectID: "p0", RoleKey: "trained",
			CreatedAt: time.Date(2026, 7, 14, 0, 0, 0, 0, time.UTC), ExpiresAt: &expires,
		}}, nil
	}

	origCollect := collectUserRolesHook
	t.Cleanup(func() { collectUserRolesHook = origCollect })
	collectUserRolesHook = func(context.Context, string) (map[roleKey]*models.EffectiveRole, []models.Bundle, error) {
		return map[roleKey]*models.EffectiveRole{
			{projectID: "p0", roleKey: "trained"}: {
				ProjectID: "p0", RoleKey: "trained", IsSource: true,
				Reasons: []models.RoleReason{{Kind: "direct"}},
			},
		}, nil, nil
	}

	view, err := RoleMembers(context.Background(), "p0", "trained")
	if err != nil {
		t.Fatalf("RoleMembers: %v", err)
	}
	if len(view.Members) != 1 {
		t.Fatalf("expected 1 member, got %d", len(view.Members))
	}
	m := view.Members[0]
	if m.GrantID != "g_88" {
		t.Errorf("expected the grant id the removal endpoint takes, got %q", m.GrantID)
	}
	if m.Since != "2026-07-14" || m.Expires != "2026-08-02" {
		t.Errorf("since/expires wrong: since=%q expires=%q", m.Since, m.Expires)
	}
}

func TestRoleMembers_EmptyWhenNobodyHoldsIt(t *testing.T) {
	setupSnapshotTestFixtures(t, 2, 0, 1)
	stubRoleFixtures(t)

	origCollect := collectUserRolesHook
	t.Cleanup(func() { collectUserRolesHook = origCollect })
	collectUserRolesHook = func(context.Context, string) (map[roleKey]*models.EffectiveRole, []models.Bundle, error) {
		return map[roleKey]*models.EffectiveRole{}, nil, nil
	}

	view, err := RoleMembers(context.Background(), "p0", "trained")
	if err != nil {
		t.Fatalf("RoleMembers: %v", err)
	}
	// Must be an empty list, never nil: the UI's empty state renders from
	// length, and a null array would decode as "failed to load" instead.
	if view.Members == nil || len(view.Members) != 0 {
		t.Fatalf("expected an empty members list, got %#v", view.Members)
	}
}

// The sidebar badge endpoint must count without materialising the payload —
// and must agree with Today about what "expiring soon" means.
func TestGovernanceIndicators_CountsWithoutBuildingSummary(t *testing.T) {
	resetGovernanceDeps(t)

	var horizon time.Duration
	svcGetAccessRequests = func(_ context.Context, status string) ([]models.AccessRequest, error) {
		if status != "pending" {
			t.Errorf("indicators should count pending requests only, asked for %q", status)
		}
		return []models.AccessRequest{{ID: "r1"}, {ID: "r2"}, {ID: "r3"}}, nil
	}
	svcGetExpiringDirectGrants = func(_ context.Context, within time.Duration) ([]models.DirectGrant, error) {
		horizon = within
		return []models.DirectGrant{{ID: "g1"}}, nil
	}
	svcCountPendingPropagations = func(context.Context) (int, error) { return 2, nil }
	svcCountPendingDrift = func(context.Context) (int, error) { return 12, nil }
	svcZitadelReachable = func(context.Context) bool { return false }

	got, err := GovernanceIndicators(context.Background())
	if err != nil {
		t.Fatalf("GovernanceIndicators: %v", err)
	}
	if got.PendingRequests != 3 || got.ExpiringGrants != 1 || got.PendingPropagation != 2 || got.Drift != 12 {
		t.Fatalf("indicator counts wrong: %#v", got)
	}
	if horizon != expiryHorizon {
		t.Errorf("badge and Today must share one expiry horizon; got %v want %v", horizon, expiryHorizon)
	}
	// Reachability is what explains a disabled "Resume now", so it must be
	// reported whenever there is something queued.
	if got.ZitadelReachable {
		t.Error("expected zitadel_reachable=false to be reported with a non-empty outbox")
	}
}

// With nothing queued there is nothing to resume, so the probe is skipped
// rather than paying for a Zitadel round trip on every sidebar poll.
func TestGovernanceIndicators_SkipsReachabilityProbeWhenOutboxEmpty(t *testing.T) {
	resetGovernanceDeps(t)

	svcGetAccessRequests = func(context.Context, string) ([]models.AccessRequest, error) { return nil, nil }
	svcGetExpiringDirectGrants = func(context.Context, time.Duration) ([]models.DirectGrant, error) { return nil, nil }
	svcCountPendingPropagations = func(context.Context) (int, error) { return 0, nil }
	svcCountPendingDrift = func(context.Context) (int, error) { return 0, nil }
	svcZitadelReachable = func(context.Context) bool {
		t.Error("reachability probed with an empty outbox")
		return false
	}

	got, err := GovernanceIndicators(context.Background())
	if err != nil {
		t.Fatalf("GovernanceIndicators: %v", err)
	}
	if !got.ZitadelReachable {
		t.Error("with nothing queued the flag should stay true, not report a probe that never ran")
	}
}
