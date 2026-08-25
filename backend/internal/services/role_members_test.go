package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"syndra/internal/db"

	"syndra/internal/models"
)

// stubRoleFixtures wires the injectables RoleMembers and GovernanceIndicators
// read, on top of the shared snapshot fixture directory.
func stubRoleFixtures(t *testing.T) {
	t.Helper()
	origGetRole, origAllGrants := svcDbGetRole, svcGetAllDirectGrants
	origTargets, origMappings, origAllowances := dbTargetsMappedToRole, dbMappingsForRoles, dbAllowancesOnTargets
	t.Cleanup(func() {
		svcDbGetRole = origGetRole
		svcGetAllDirectGrants = origAllGrants
		dbTargetsMappedToRole, dbMappingsForRoles, dbAllowancesOnTargets = origTargets, origMappings, origAllowances
	})
	svcDbGetRole = func(context.Context, string, string) (models.Role, error) {
		return models.Role{}, context.Canceled // no local row — best-effort metadata
	}
	// The carve-out read. Default is "this role reaches no target", which is
	// what most roles in this deployment do — a test about the source ordering
	// must not be made to invent a mapping, and the real read needs a database.
	dbTargetsMappedToRole = func(context.Context, string, string) ([]string, error) { return nil, nil }
	dbMappingsForRoles = func(context.Context, string, []db.RoleRef) ([]db.RoleMapping, error) { return nil, nil }
	dbAllowancesOnTargets = func(context.Context, []string) ([]db.Allowance, error) { return nil, nil }
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

// Holds due are counted apart from expiring grants, and the reason is that
// inaction means the opposite thing in each: an expiring grant lapses if nobody
// acts, and a hold stays in force. A badge that summed them would be counting
// "access about to end" together with "access still being withheld", and an
// operator reading one number could not tell which they were looking at.
func TestHoldsDueAreCountedApartFromExpiringGrants(t *testing.T) {
	resetGovernanceDeps(t)

	svcGetAccessRequests = func(context.Context, string) ([]models.AccessRequest, error) { return nil, nil }
	svcGetExpiringDirectGrants = func(context.Context, time.Duration) ([]models.DirectGrant, error) {
		return []models.DirectGrant{{ID: "g1"}, {ID: "g2"}}, nil
	}
	svcCountPendingPropagations = func(context.Context) (int, error) { return 0, nil }
	svcCountPendingDrift = func(context.Context) (int, error) { return 0, nil }
	svcAllowancesDueForReview = func(context.Context) ([]db.Allowance, error) {
		return []db.Allowance{{ID: "a1"}, {ID: "a2"}, {ID: "a3"}}, nil
	}

	got, err := GovernanceIndicators(context.Background())
	if err != nil {
		t.Fatalf("GovernanceIndicators: %v", err)
	}
	if got.HoldsDue != 3 {
		t.Errorf("holds due = %d, want 3", got.HoldsDue)
	}
	if got.ExpiringGrants != 2 {
		t.Errorf("expiring grants = %d, want 2 — the two counts must not merge", got.ExpiringGrants)
	}
}

// §6 on the holder list — the carve-out has to be visible EVERYWHERE the role
// appears, and this list is where an operator decides who to act on.
//
// Without it the screen says "these people hold this role" and means "most of
// them do". The member's own page had the carve-out; the operator's did not,
// which is the wrong way round.
func TestARoleHolderWithSomethingWithheldSaysSoOnTheList(t *testing.T) {
	setupSnapshotTestFixtures(t, 2, 0, 1)
	stubRoleFixtures(t)

	origCollect := collectUserRolesHook
	t.Cleanup(func() { collectUserRolesHook = origCollect })
	collectUserRolesHook = func(_ context.Context, userID string) (map[roleKey]*models.EffectiveRole, []models.Bundle, error) {
		return map[roleKey]*models.EffectiveRole{
			{projectID: "p0", roleKey: "trained"}: {
				Reasons: []models.RoleReason{{Kind: "direct"}},
			},
		}, nil, nil
	}

	dbTargetsMappedToRole = func(context.Context, string, string) ([]string, error) {
		return []string{"truenas"}, nil
	}
	dbMappingsForRoles = func(context.Context, string, []db.RoleRef) ([]db.RoleMapping, error) {
		return []db.RoleMapping{
			{Target: "truenas", ProjectID: "p0", RoleKey: "trained", Field: "group", Value: "lab_makers"},
		}, nil
	}
	dbAllowancesOnTargets = func(context.Context, []string) ([]db.Allowance, error) {
		return []db.Allowance{
			// Matches what this role binds — belongs on the row.
			{ID: "a1", SubjectID: "u0", Target: "truenas", Field: "group", Value: "lab_makers",
				Direction: db.AllowanceDeny, ActorID: "op_1", Reason: "safety review"},
			// A denial on something ANOTHER role grants. It must not appear
			// here: an operator would act on this row, and moving a carve-out
			// onto the wrong role is worse than omitting it.
			{ID: "a2", SubjectID: "u1", Target: "truenas", Field: "group", Value: "fabrication",
				Direction: db.AllowanceDeny, ActorID: "op_1", Reason: "unrelated"},
		}, nil
	}

	view, err := RoleMembers(context.Background(), "p0", "trained")
	if err != nil {
		t.Fatal(err)
	}
	if view.WithheldCount != 1 {
		t.Fatalf("exactly one holder has something withheld, got %d", view.WithheldCount)
	}
	var found bool
	for _, m := range view.Members {
		switch m.User.ID {
		case "u0":
			found = true
			if len(m.Withheld) != 1 || m.Withheld[0].Value != "lab_makers" {
				t.Errorf("the held value must be named on the row: %+v", m.Withheld)
			}
			if m.Withheld[0].Reason != "safety review" {
				t.Errorf("the reason travels, or the row is a mystery: %+v", m.Withheld[0])
			}
		case "u1":
			if len(m.Withheld) != 0 {
				t.Errorf("a denial on what a DIFFERENT role grants must not appear on this one: %+v", m.Withheld)
			}
		}
	}
	if !found {
		t.Fatal("the held holder is not in the list at all")
	}
}

// A lifecycle denial is bound to no mapping and belongs on every role reaching
// the target: `enabled = true` denied means the account is off, so nothing the
// role confers there is reachable. The intersection cannot find it, so it is
// matched on the target alone — and a test says so, because "matches nothing
// and is included anyway" reads as a bug in the filter.
func TestALifecycleDenialAppearsOnEveryRoleThatReachesTheTarget(t *testing.T) {
	setupSnapshotTestFixtures(t, 1, 0, 1)
	stubRoleFixtures(t)

	origCollect := collectUserRolesHook
	t.Cleanup(func() { collectUserRolesHook = origCollect })
	collectUserRolesHook = func(context.Context, string) (map[roleKey]*models.EffectiveRole, []models.Bundle, error) {
		return map[roleKey]*models.EffectiveRole{
			{projectID: "p0", roleKey: "trained"}: {Reasons: []models.RoleReason{{Kind: "direct"}}},
		}, nil, nil
	}
	dbTargetsMappedToRole = func(context.Context, string, string) ([]string, error) {
		return []string{"truenas"}, nil
	}
	// Deliberately binds something the denial does NOT name.
	dbMappingsForRoles = func(context.Context, string, []db.RoleRef) ([]db.RoleMapping, error) {
		return []db.RoleMapping{
			{Target: "truenas", ProjectID: "p0", RoleKey: "trained", Field: "group", Value: "lab_makers"},
		}, nil
	}
	dbAllowancesOnTargets = func(context.Context, []string) ([]db.Allowance, error) {
		return []db.Allowance{{
			ID: "a1", SubjectID: "u0", Target: "truenas", Field: "enabled", Value: "true",
			Direction: db.AllowanceDeny, ActorID: "op_1", Reason: "offboarding",
		}}, nil
	}

	view, err := RoleMembers(context.Background(), "p0", "trained")
	if err != nil {
		t.Fatal(err)
	}
	if view.WithheldCount != 1 {
		t.Fatalf("a disabled account withholds everything the role reaches, got %d", view.WithheldCount)
	}
}

// An additive allowance is not a carve-out. The write path refuses one, so
// reaching here means it grew a way to make one — and drawing it as access
// taken away would say the opposite of what happened.
func TestAnAdditiveAllowanceIsNotRenderedAsAWithholding(t *testing.T) {
	setupSnapshotTestFixtures(t, 1, 0, 1)
	stubRoleFixtures(t)

	origCollect := collectUserRolesHook
	t.Cleanup(func() { collectUserRolesHook = origCollect })
	collectUserRolesHook = func(context.Context, string) (map[roleKey]*models.EffectiveRole, []models.Bundle, error) {
		return map[roleKey]*models.EffectiveRole{
			{projectID: "p0", roleKey: "trained"}: {Reasons: []models.RoleReason{{Kind: "direct"}}},
		}, nil, nil
	}
	dbTargetsMappedToRole = func(context.Context, string, string) ([]string, error) {
		return []string{"truenas"}, nil
	}
	dbMappingsForRoles = func(context.Context, string, []db.RoleRef) ([]db.RoleMapping, error) {
		return []db.RoleMapping{
			{Target: "truenas", ProjectID: "p0", RoleKey: "trained", Field: "group", Value: "lab_makers"},
		}, nil
	}
	dbAllowancesOnTargets = func(context.Context, []string) ([]db.Allowance, error) {
		return []db.Allowance{{
			ID: "a1", SubjectID: "u0", Target: "truenas", Field: "group", Value: "lab_makers",
			Direction: db.AllowanceAllow, ActorID: "op_1", Reason: "granted by hand",
		}}, nil
	}

	view, err := RoleMembers(context.Background(), "p0", "trained")
	if err != nil {
		t.Fatal(err)
	}
	if view.WithheldCount != 0 {
		t.Fatalf("an allowance that GIVES something is not access taken away, got %d", view.WithheldCount)
	}
}

// A carve-out read that failed must not render as "nobody has one".
//
// The zero count and the empty rows are byte-identical to a cohort that
// genuinely has none, and a server log is not loud to the person reading the
// page. Without the flag the degradation reproduces, on this exact screen, the
// failure the field was added to close.
func TestAFailedCarveOutReadSaysSoRatherThanReadingAsClean(t *testing.T) {
	setupSnapshotTestFixtures(t, 1, 0, 1)
	stubRoleFixtures(t)

	origCollect := collectUserRolesHook
	t.Cleanup(func() { collectUserRolesHook = origCollect })
	collectUserRolesHook = func(context.Context, string) (map[roleKey]*models.EffectiveRole, []models.Bundle, error) {
		return map[roleKey]*models.EffectiveRole{
			{projectID: "p0", roleKey: "trained"}: {Reasons: []models.RoleReason{{Kind: "direct"}}},
		}, nil, nil
	}
	dbTargetsMappedToRole = func(context.Context, string, string) ([]string, error) {
		return nil, errors.New("the mapping table is unavailable")
	}

	view, err := RoleMembers(context.Background(), "p0", "trained")
	if err != nil {
		t.Fatalf("the holder list is still the answer to the question asked: %v", err)
	}
	if !view.WithheldUnavailable {
		t.Fatal("a list that could not read carve-outs must say so, or its zero reads as an answer")
	}
	if view.WithheldCount != 0 {
		t.Errorf("and it must not invent one: %d", view.WithheldCount)
	}
	if len(view.Members) != 1 {
		t.Errorf("the holders themselves are unaffected: %d", len(view.Members))
	}
}
