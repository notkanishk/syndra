package services

import (
	"context"
	"strings"
	"testing"
	"time"

	"syndra/internal/db"
	"syndra/internal/directory"
	"syndra/internal/models"
)

// bulkFixture swaps the directory and every DB injectable the rehearsal reads,
// so these tests exercise the decision logic — which is the part that has to be
// right — without a database.
type bulkFixture struct {
	users  []models.UserProfile
	roles  map[string]map[roleKey]*models.EffectiveRole
	bundle map[string][]models.Bundle
	grants map[string][]models.DirectGrant
	// queued: what a write would cancel, by "user/op/role".
	queued map[string][]models.QueuedChange
}

func setupBulk(t *testing.T, fx bulkFixture) {
	t.Helper()

	origDir := directory.Default
	directory.Default = &snapshotFixtureDirectory{users: fx.users}
	t.Cleanup(func() { directory.Default = origDir })

	origCollect := collectUserRolesHook
	origGrants := svcGetDirectGrantsForUser
	origBundle := svcGetBundleByID
	origRoles := svcLatestVersionRoles
	origCancel := svcWouldCancel
	t.Cleanup(func() {
		collectUserRolesHook = origCollect
		svcGetDirectGrantsForUser = origGrants
		svcGetBundleByID = origBundle
		svcLatestVersionRoles = origRoles
		svcWouldCancel = origCancel
	})
	svcWouldCancel = func(_ context.Context, p db.EnqueueParams) ([]models.QueuedChange, error) {
		return fx.queued[p.UserID+"/"+p.OpType+"/"+p.RoleKeys[0]], nil
	}

	collectUserRolesHook = func(_ context.Context, uid string) (map[roleKey]*models.EffectiveRole, []models.Bundle, error) {
		roles := fx.roles[uid]
		if roles == nil {
			roles = map[roleKey]*models.EffectiveRole{}
		}
		return roles, fx.bundle[uid], nil
	}
	svcGetDirectGrantsForUser = func(_ context.Context, uid string, _ bool) ([]models.DirectGrant, error) {
		return fx.grants[uid], nil
	}
	svcGetBundleByID = func(_ context.Context, id string) (models.Bundle, error) {
		return models.Bundle{ID: id, Name: "Safety"}, nil
	}
	svcLatestVersionRoles = func(context.Context, string) (models.BundleVersion, []models.BundleRole, error) {
		return models.BundleVersion{ID: "v-latest", Version: 2}, []models.BundleRole{{}, {}, {}}, nil
	}
}

func outcomeFor(t *testing.T, plan BulkPlan, uid string) BulkOutcome {
	t.Helper()
	for _, o := range plan.Outcomes {
		if o.UserID == uid {
			return o
		}
	}
	t.Fatalf("no outcome for %s", uid)
	return BulkOutcome{}
}

func laser(rk string) roleKey { return roleKey{projectID: "pLaser", roleKey: rk} }

// A bulk selection is usually assembled from a filter, and a departed account
// matches the same filters a live one does. Granting access to someone who has
// left is the single most likely way a bulk write goes wrong, and it must be
// refused loudly rather than folded into a success count.
func TestRehearseBulk_BlocksDepartedAccountsOnGrant(t *testing.T) {
	setupBulk(t, bulkFixture{users: []models.UserProfile{
		{ID: "u_live", Name: "Ada Lovelace", Status: "active"},
		{ID: "u_gone", Name: "Leo Brooks", Status: "departed"},
	}})

	plan, err := RehearseBulk(context.Background(), BulkRequest{
		Op: BulkOpAssignRole, UserIDs: []string{"u_live", "u_gone"},
		ProjectID: "pLaser", RoleKey: "trained", DurationDays: 90,
	})
	if err != nil {
		t.Fatalf("rehearse: %v", err)
	}

	if got := outcomeFor(t, plan, "u_gone").Effect; got != EffectBlocked {
		t.Errorf("departed account must be blocked, got %q", got)
	}
	if got := outcomeFor(t, plan, "u_live").Effect; got != EffectApply {
		t.Errorf("live account must apply, got %q", got)
	}
	if plan.Summary.Blocked != 1 || plan.Summary.Apply != 1 {
		t.Errorf("summary must separate blocked from applicable: %+v", plan.Summary)
	}
}

// Removal in this product is source-specific. A bulk remove takes away a direct
// grant and nothing else, so anyone holding the role through a bundle or rule
// keeps it — and the plan has to say so before the operator confirms, or they
// will read "12 people lose access" and be wrong about at least one.
func TestRehearseBulk_RemoveNamesWhoKeepsTheRoleAnyway(t *testing.T) {
	setupBulk(t, bulkFixture{
		users: []models.UserProfile{
			{ID: "u_direct", Name: "Ada", Status: "active"},
			{ID: "u_both", Name: "Sam", Status: "active"},
			{ID: "u_bundle", Name: "Maya", Status: "active"},
		},
		grants: map[string][]models.DirectGrant{
			"u_direct": {{ID: "g1", ProjectID: "pLaser", RoleKey: "trained"}},
			"u_both":   {{ID: "g2", ProjectID: "pLaser", RoleKey: "trained"}},
		},
		roles: map[string]map[roleKey]*models.EffectiveRole{
			"u_direct": {laser("trained"): {Reasons: []models.RoleReason{{Kind: "direct"}}}},
			"u_both": {laser("trained"): {Reasons: []models.RoleReason{
				{Kind: "direct"},
				{Kind: "bundle", BundleName: "Safety"},
			}}},
			"u_bundle": {laser("trained"): {Reasons: []models.RoleReason{
				{Kind: "bundle", BundleName: "Safety"},
			}}},
		},
	})

	plan, err := RehearseBulk(context.Background(), BulkRequest{
		Op: BulkOpRemoveRole, UserIDs: []string{"u_direct", "u_both", "u_bundle"},
		ProjectID: "pLaser", RoleKey: "trained",
	})
	if err != nil {
		t.Fatalf("rehearse: %v", err)
	}

	onlyDirect := outcomeFor(t, plan, "u_direct")
	if onlyDirect.Effect != EffectApply || onlyDirect.Consequence != "" {
		t.Errorf("a direct-only holder simply loses the role: %+v", onlyDirect)
	}

	both := outcomeFor(t, plan, "u_both")
	if both.Effect != EffectApply {
		t.Errorf("a direct grant still exists to remove, got %q", both.Effect)
	}
	if !strings.Contains(both.Consequence, "Safety") {
		t.Errorf("must name the source that keeps the role: %q", both.Consequence)
	}
	if !strings.Contains(both.Consequence, "does not take their access away") {
		t.Errorf("must say the access survives: %q", both.Consequence)
	}

	bundleOnly := outcomeFor(t, plan, "u_bundle")
	if bundleOnly.Effect != EffectNoChange {
		t.Errorf("no direct grant means nothing to remove, got %q", bundleOnly.Effect)
	}
	if !strings.Contains(bundleOnly.Consequence, "Safety") {
		t.Errorf("must point at the source to remove instead: %q", bundleOnly.Consequence)
	}
	if len(bundleOnly.GrantIDs) != 0 {
		t.Errorf("a no-change row must carry no grant to act on: %v", bundleOnly.GrantIDs)
	}
}

// Granting a role somebody already has effectively is legitimate — a direct
// grant survives bundle changes — but it is a second source, not first access,
// and the rehearsal must not present it as a person gaining something.
func TestRehearseBulk_AssignLeavesSomeoneWhoHoldsItThroughABundle(t *testing.T) {
	setupBulk(t, bulkFixture{
		users: []models.UserProfile{{ID: "u1", Name: "Ada", Status: "active"}},
		roles: map[string]map[roleKey]*models.EffectiveRole{
			"u1": {laser("trained"): {Reasons: []models.RoleReason{{Kind: "bundle", BundleName: "Safety"}}}},
		},
	})

	plan, err := RehearseBulk(context.Background(), BulkRequest{
		Op: BulkOpAssignRole, UserIDs: []string{"u1"}, ProjectID: "pLaser", RoleKey: "trained",
	})
	if err != nil {
		t.Fatalf("rehearse: %v", err)
	}

	out := outcomeFor(t, plan, "u1")
	// Owner, 15 Oct: a role someone already has, however they have it, is not
	// given again. It used to add a second, direct source beside the bundle.
	if out.Effect != EffectNoChange {
		t.Errorf("already holds it through a bundle: no change, got %q", out.Effect)
	}
}

// Extend must never touch a permanent grant. Stamping an expiry onto access
// that had none would silently convert "forever" into "90 days" — the exact
// opposite of what the operator asked for.
func TestRehearseBulk_ExtendIgnoresPermanentGrants(t *testing.T) {
	soon := time.Now().Add(48 * time.Hour)
	setupBulk(t, bulkFixture{
		users: []models.UserProfile{
			{ID: "u_mixed", Name: "Ada", Status: "active"},
			{ID: "u_perm", Name: "Sam", Status: "active"},
		},
		grants: map[string][]models.DirectGrant{
			"u_mixed": {
				{ID: "g_exp", ProjectID: "pLaser", RoleKey: "trained", ExpiresAt: &soon},
				{ID: "g_perm", ProjectID: "pWood", RoleKey: "member"},
			},
			"u_perm": {{ID: "g_forever", ProjectID: "pLaser", RoleKey: "trained"}},
		},
	})

	plan, err := RehearseBulk(context.Background(), BulkRequest{
		Op: BulkOpExtend, UserIDs: []string{"u_mixed", "u_perm"}, DurationDays: 90,
	})
	if err != nil {
		t.Fatalf("rehearse: %v", err)
	}

	mixed := outcomeFor(t, plan, "u_mixed")
	if len(mixed.GrantIDs) != 1 || mixed.GrantIDs[0] != "g_exp" {
		t.Errorf("only the expiring grant may be extended, got %v", mixed.GrantIDs)
	}

	perm := outcomeFor(t, plan, "u_perm")
	if perm.Effect != EffectNoChange || len(perm.GrantIDs) != 0 {
		t.Errorf("a permanent grant has nothing to extend: %+v", perm)
	}
}

// Review › Expiring access selects grant ROWS, and this is what makes that safe.
//
// The screen used to reduce its ticked rows to their user ids, which extended every expiring grant
// each of those people held — including grants in other projects, and grants expiring months past
// the 30-day window the screen is scoped to. The operator ticked one row and renewed access they
// had never been shown.
func TestRehearseBulk_ExtendHonoursTheSelectedGrants(t *testing.T) {
	soon := time.Now().Add(48 * time.Hour)
	// Outside the review window, and the reason this matters: the queue never rendered this row,
	// so nothing on screen could have told the operator it was about to be renewed.
	distant := time.Now().Add(200 * 24 * time.Hour)
	setupBulk(t, bulkFixture{
		users: []models.UserProfile{{ID: "u1", Name: "Ada", Status: "active"}},
		grants: map[string][]models.DirectGrant{
			"u1": {
				{ID: "g_seen", ProjectID: "pLaser", RoleKey: "trained", ExpiresAt: &soon},
				{ID: "g_unseen", ProjectID: "pWood", RoleKey: "member", ExpiresAt: &distant},
			},
		},
	})

	plan, err := RehearseBulk(context.Background(), BulkRequest{
		Op: BulkOpExtend, UserIDs: []string{"u1"}, DurationDays: 90,
		GrantIDs: []string{"g_seen"},
	})
	if err != nil {
		t.Fatalf("rehearse: %v", err)
	}

	out := outcomeFor(t, plan, "u1")
	if len(out.GrantIDs) != 1 || out.GrantIDs[0] != "g_seen" {
		t.Errorf("only the ticked grant may be extended, got %v", out.GrantIDs)
	}
}

// Omitting the ids keeps the meaning the People screen needs: there, the operator selects PEOPLE,
// and "extend their expiring access" is exactly the request.
func TestRehearseBulk_ExtendWithoutIDsStillMeansEverythingExpiring(t *testing.T) {
	soon := time.Now().Add(48 * time.Hour)
	later := time.Now().Add(20 * 24 * time.Hour)
	setupBulk(t, bulkFixture{
		users: []models.UserProfile{{ID: "u1", Name: "Ada", Status: "active"}},
		grants: map[string][]models.DirectGrant{
			"u1": {
				{ID: "g_a", ProjectID: "pLaser", RoleKey: "trained", ExpiresAt: &soon},
				{ID: "g_b", ProjectID: "pWood", RoleKey: "member", ExpiresAt: &later},
			},
		},
	})

	plan, err := RehearseBulk(context.Background(), BulkRequest{
		Op: BulkOpExtend, UserIDs: []string{"u1"}, DurationDays: 90,
	})
	if err != nil {
		t.Fatalf("rehearse: %v", err)
	}
	if got := len(outcomeFor(t, plan, "u1").GrantIDs); got != 2 {
		t.Errorf("expected both expiring grants, got %d", got)
	}
}

// A flat id set across several people must not let one person's tick reach another's access. A
// grant id belongs to exactly one person, so the per-user pass is what enforces this — and if it
// ever stopped, a selection of Ada's row would extend Sam's grant of the same role.
func TestRehearseBulk_ExtendDoesNotCrossBetweenPeople(t *testing.T) {
	soon := time.Now().Add(48 * time.Hour)
	setupBulk(t, bulkFixture{
		users: []models.UserProfile{
			{ID: "u1", Name: "Ada", Status: "active"},
			{ID: "u2", Name: "Sam", Status: "active"},
		},
		grants: map[string][]models.DirectGrant{
			"u1": {{ID: "g_ada", ProjectID: "pLaser", RoleKey: "trained", ExpiresAt: &soon}},
			"u2": {{ID: "g_sam", ProjectID: "pLaser", RoleKey: "trained", ExpiresAt: &soon}},
		},
	})

	plan, err := RehearseBulk(context.Background(), BulkRequest{
		Op: BulkOpExtend, UserIDs: []string{"u1", "u2"}, DurationDays: 90,
		GrantIDs: []string{"g_ada"},
	})
	if err != nil {
		t.Fatalf("rehearse: %v", err)
	}

	if ids := outcomeFor(t, plan, "u1").GrantIDs; len(ids) != 1 || ids[0] != "g_ada" {
		t.Errorf("expected Ada's ticked grant, got %v", ids)
	}
	sam := outcomeFor(t, plan, "u2")
	if sam.Effect != EffectNoChange || len(sam.GrantIDs) != 0 {
		t.Errorf("Sam's grant was not selected and must not be extended: %+v", sam)
	}
	// And the plan must say why his row does nothing, so an operator reading it is not left to
	// guess whether he has no expiring access at all.
	if !strings.Contains(sam.Detail, "selected") {
		t.Errorf("the plan must distinguish 'not selected' from 'nothing expiring': %q", sam.Detail)
	}
}

// An id that resolves to nobody is blocked, not skipped. A silently dropped row
// would make the result count disagree with the selection the operator made.
func TestRehearseBulk_BlocksUnknownAccounts(t *testing.T) {
	setupBulk(t, bulkFixture{users: []models.UserProfile{{ID: "u1", Name: "Ada", Status: "active"}}})

	plan, err := RehearseBulk(context.Background(), BulkRequest{
		Op: BulkOpAssignBundle, UserIDs: []string{"u1", "u_ghost"}, BundleID: "b1",
	})
	if err != nil {
		t.Fatalf("rehearse: %v", err)
	}
	if len(plan.Outcomes) != 2 {
		t.Fatalf("every selected id must produce a row, got %d", len(plan.Outcomes))
	}
	if got := outcomeFor(t, plan, "u_ghost").Effect; got != EffectBlocked {
		t.Errorf("unknown account must be blocked, got %q", got)
	}
}

func TestRehearseBulk_BundleMembershipIsIdempotent(t *testing.T) {
	setupBulk(t, bulkFixture{
		users: []models.UserProfile{
			{ID: "u_in", Name: "Ada", Status: "active"},
			{ID: "u_out", Name: "Sam", Status: "active"},
		},
		bundle: map[string][]models.Bundle{"u_in": {{ID: "b1", Name: "Safety"}}},
	})

	assign, err := RehearseBulk(context.Background(), BulkRequest{
		Op: BulkOpAssignBundle, UserIDs: []string{"u_in", "u_out"}, BundleID: "b1",
	})
	if err != nil {
		t.Fatalf("rehearse: %v", err)
	}
	if got := outcomeFor(t, assign, "u_in").Effect; got != EffectNoChange {
		t.Errorf("already a member: %q", got)
	}
	if out := outcomeFor(t, assign, "u_out"); out.Effect != EffectApply || !strings.Contains(out.Consequence, "3 roles") {
		t.Errorf("joining must state the cascade size: %+v", out)
	}

	remove, err := RehearseBulk(context.Background(), BulkRequest{
		Op: BulkOpRemoveBundle, UserIDs: []string{"u_in", "u_out"}, BundleID: "b1",
	})
	if err != nil {
		t.Fatalf("rehearse: %v", err)
	}
	if got := outcomeFor(t, remove, "u_in").Effect; got != EffectApply {
		t.Errorf("a member can leave: %q", got)
	}
	if got := outcomeFor(t, remove, "u_out").Effect; got != EffectNoChange {
		t.Errorf("a non-member has nothing to leave: %q", got)
	}
}

// A duplicated id in the selection must not double-apply. Select-all across a
// filter plus a manual click is an easy way to produce one.
func TestRehearseBulk_DeduplicatesSelection(t *testing.T) {
	setupBulk(t, bulkFixture{users: []models.UserProfile{{ID: "u1", Name: "Ada", Status: "active"}}})

	plan, err := RehearseBulk(context.Background(), BulkRequest{
		Op: BulkOpAssignRole, UserIDs: []string{"u1", "u1", " u1 ", ""},
		ProjectID: "pLaser", RoleKey: "trained",
	})
	if err != nil {
		t.Fatalf("rehearse: %v", err)
	}
	if len(plan.Outcomes) != 1 {
		t.Fatalf("expected one row per distinct person, got %d", len(plan.Outcomes))
	}
}

// Blocked rows sort first: they are the only ones that require a decision
// before confirming, so they must not be buried under fifty no-ops.
func TestRehearseBulk_OrdersBlockedRowsFirst(t *testing.T) {
	setupBulk(t, bulkFixture{
		users: []models.UserProfile{
			{ID: "u_a", Name: "Aaa", Status: "active"},
			{ID: "u_z", Name: "Zzz", Status: "departed"},
		},
	})

	plan, err := RehearseBulk(context.Background(), BulkRequest{
		Op: BulkOpAssignRole, UserIDs: []string{"u_a", "u_z"}, ProjectID: "pLaser", RoleKey: "trained",
	})
	if err != nil {
		t.Fatalf("rehearse: %v", err)
	}
	if plan.Outcomes[0].Effect != EffectBlocked {
		t.Errorf("blocked rows must lead, got %q first", plan.Outcomes[0].Effect)
	}
}

func TestValidateBulkRequest(t *testing.T) {
	cases := []struct {
		name  string
		req   BulkRequest
		field string
	}{
		{"unknown op", BulkRequest{Op: "delete_everything", UserIDs: []string{"u1"}, Reason: "x"}, "op"},
		{"role op without role", BulkRequest{Op: BulkOpAssignRole, UserIDs: []string{"u1"}, ProjectID: "p", Reason: "x"}, "role_key"},
		{"role op without project", BulkRequest{Op: BulkOpRemoveRole, UserIDs: []string{"u1"}, RoleKey: "r", Reason: "x"}, "project_id"},
		{"bundle op without bundle", BulkRequest{Op: BulkOpAssignBundle, UserIDs: []string{"u1"}, Reason: "x"}, "bundle_id"},
		{"extend by nothing", BulkRequest{Op: BulkOpExtend, UserIDs: []string{"u1"}, Reason: "x"}, "duration_days"},
		// grant_ids narrows extend and nothing else. Accepted and ignored, it would let a caller
		// believe they had scoped a bundle operation they had not.
		{"grant ids on a bundle op", BulkRequest{
			Op: BulkOpAssignBundle, UserIDs: []string{"u1"}, BundleID: "b1", Reason: "x",
			GrantIDs: []string{"g1"},
		}, "grant_ids"},
		{"empty selection", BulkRequest{Op: BulkOpExtend, DurationDays: 30, Reason: "x"}, "user_ids"},
		{"blank-only selection", BulkRequest{Op: BulkOpExtend, DurationDays: 30, UserIDs: []string{" ", ""}, Reason: "x"}, "user_ids"},
		// One audit row per person, so an unexplained bulk change is an
		// unaccountable change multiplied by the size of the selection. The UI
		// enforces this too, but the boundary is what makes it true.
		{"missing reason", BulkRequest{Op: BulkOpAssignRole, UserIDs: []string{"u1"}, ProjectID: "p", RoleKey: "r"}, "reason"},
		{"whitespace-only reason", BulkRequest{Op: BulkOpAssignRole, UserIDs: []string{"u1"}, ProjectID: "p", RoleKey: "r", Reason: "   \t"}, "reason"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			problems := ValidateBulkRequest(tc.req)
			if problems == nil {
				t.Fatalf("expected a validation error on %s", tc.field)
			}
			if _, ok := problems[tc.field]; !ok {
				t.Errorf("expected %q to be flagged, got %v", tc.field, problems)
			}
		})
	}

	if problems := ValidateBulkRequest(BulkRequest{
		Op: BulkOpAssignRole, UserIDs: []string{"u1"}, ProjectID: "p", RoleKey: "r", Reason: "New cohort",
	}); problems != nil {
		t.Errorf("a complete request must validate, got %v", problems)
	}
}

func TestValidateBulkRequest_RejectsOversizeSelection(t *testing.T) {
	ids := make([]string, BulkMaxUsers+1)
	for i := range ids {
		ids[i] = string(rune('a'+i%26)) + string(rune('0'+i/26%10)) + string(rune('A'+i/260))
	}
	problems := ValidateBulkRequest(BulkRequest{
		Op: BulkOpAssignRole, UserIDs: ids, ProjectID: "p", RoleKey: "r",
	})
	if problems == nil || problems["user_ids"] == "" {
		t.Errorf("an unbounded selection must be refused, got %v", problems)
	}
}

// Giving a role is for people without it (owner, 15 Oct): anyone who holds it,
// for good or until a date, is no change. An extension skips a grant that
// already runs past it, so it never shortens.
func TestRehearseBulk_NeverPromisesAShorterEnd(t *testing.T) {
	far := time.Now().Add(200 * 24 * time.Hour)
	soon := time.Now().Add(5 * 24 * time.Hour)
	setupBulk(t, bulkFixture{
		users: []models.UserProfile{
			{ID: "u_forever", Name: "Ada", Status: "active"},
			{ID: "u_far", Name: "Sam", Status: "active"},
			{ID: "u_soon", Name: "Lee", Status: "active"},
		},
		grants: map[string][]models.DirectGrant{
			"u_forever": {{ID: "g1", ProjectID: "pLaser", RoleKey: "trained"}},
			"u_far":     {{ID: "g2", ProjectID: "pLaser", RoleKey: "trained", ExpiresAt: &far}},
			"u_soon":    {{ID: "g3", ProjectID: "pLaser", RoleKey: "trained", ExpiresAt: &soon}},
		},
	})
	assign, err := RehearseBulk(context.Background(), BulkRequest{
		Op: BulkOpAssignRole, UserIDs: []string{"u_forever", "u_far", "u_soon"},
		ProjectID: "pLaser", RoleKey: "trained", DurationDays: 30,
	})
	if err != nil {
		t.Fatal(err)
	}
	if o := outcomeFor(t, assign, "u_forever"); o.Effect != EffectNoChange {
		t.Errorf("held for good, 30 days changes nothing: %+v", o)
	}
	if o := outcomeFor(t, assign, "u_far"); o.Effect != EffectNoChange {
		t.Errorf("held past 30 days, nothing changes: %+v", o)
	}
	if o := outcomeFor(t, assign, "u_soon"); o.Effect != EffectNoChange {
		t.Errorf("already holds it (ending soon): giving is refused, extending is its own operation: %+v", o)
	}

	extend, err := RehearseBulk(context.Background(), BulkRequest{
		Op: BulkOpExtend, UserIDs: []string{"u_far", "u_soon"}, DurationDays: 90,
	})
	if err != nil {
		t.Fatal(err)
	}
	if o := outcomeFor(t, extend, "u_far"); o.Effect != EffectNoChange {
		t.Errorf("already ends past 90 days, nothing to extend: %+v", o)
	}
	if o := outcomeFor(t, extend, "u_soon"); o.Effect != EffectApply || len(o.GrantIDs) != 1 {
		t.Errorf("ending in 5 days is extended: %+v", o)
	}
}

// A row that would cancel a queued change names it before anyone applies
// (owner, 15 Oct): the apply is marked reviewed on the strength of this line.
func TestABulkRowNamesWhatItWouldCancel(t *testing.T) {
	queuedAt := time.Date(2026, 10, 15, 9, 0, 0, 0, time.UTC)
	setupBulk(t, bulkFixture{
		users: []models.UserProfile{{ID: "u1", Name: "Ada", Status: "active"}, {ID: "u2", Name: "Bo", Status: "active"}},
		queued: map[string][]models.QueuedChange{
			"u1/add/trained": {{ID: "q1", OpType: "revoke", RoleKeys: []string{"trained"}, QueuedAt: queuedAt, Reason: "left the course"}},
		},
	})
	plan, err := RehearseBulk(context.Background(), BulkRequest{Op: BulkOpAssignRole, UserIDs: []string{"u1", "u2"}, ProjectID: "pLaser", RoleKey: "trained"})
	if err != nil {
		t.Fatal(err)
	}
	got := outcomeFor(t, plan, "u1").Consequence
	if !strings.Contains(got, "Cancels a change waiting for you to send: the removal of trained queued 15 Oct 2026 (left the course).") {
		t.Fatalf("the row names the queued removal it cancels, got %q", got)
	}
	if c := outcomeFor(t, plan, "u2").Consequence; strings.Contains(c, "Cancels") {
		t.Fatalf("nothing queued, nothing said, got %q", c)
	}
}

// #32, B4: an operator never takes access from themselves in bulk. The plan
// leaves them out and says so; giving access in bulk still includes them.
func TestRehearseBulk_NeverTakesAccessFromTheRequester(t *testing.T) {
	setupBulk(t, bulkFixture{
		users: []models.UserProfile{
			{ID: "u_me", Name: "Kanishk", Status: "active"},
			{ID: "u_ada", Name: "Ada", Status: "active"},
		},
		grants: map[string][]models.DirectGrant{
			"u_me":  {{ID: "g1", ProjectID: "pLaser", RoleKey: "trained"}},
			"u_ada": {{ID: "g2", ProjectID: "pLaser", RoleKey: "trained"}},
		},
		roles: map[string]map[roleKey]*models.EffectiveRole{
			"u_me":  {laser("trained"): {Reasons: []models.RoleReason{{Kind: "direct"}}}},
			"u_ada": {laser("trained"): {Reasons: []models.RoleReason{{Kind: "direct"}}}},
		},
	})

	plan, err := RehearseBulk(context.Background(), BulkRequest{
		Op: BulkOpRemoveRole, UserIDs: []string{"u_me", "u_ada"},
		ProjectID: "pLaser", RoleKey: "trained", Actor: "u_me",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Outcomes) != 1 || plan.Outcomes[0].UserID != "u_ada" {
		t.Fatalf("only the others are in the plan: %+v", plan.Outcomes)
	}
	if len(plan.Excluded) != 1 || plan.Excluded[0] != (BulkExclusion{UserID: "u_me", Reason: "self"}) {
		t.Fatalf("the requester is named as left out: %+v", plan.Excluded)
	}

	give, err := RehearseBulk(context.Background(), BulkRequest{
		Op: BulkOpAssignRole, UserIDs: []string{"u_me", "u_ada"},
		ProjectID: "pLaser", RoleKey: "other", Actor: "u_me",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(give.Excluded) != 0 || len(give.Outcomes) != 2 {
		t.Fatalf("giving in bulk leaves nobody out: %+v %+v", give.Outcomes, give.Excluded)
	}
}
