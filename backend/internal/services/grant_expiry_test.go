package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"syndra/internal/db"
	"syndra/internal/models"
)

// Expiry used to delete the ledger row and then call Zitadel directly to revoke
// whatever mapping rules derived from the lapsed role. The grant itself was
// never revoked upstream at all — so the access stayed live in Zitadel with no
// Syndra record explaining it, and the next drift sweep raised it as
// unexplained access for a human to triage. Expiry manufactured drift out of
// its own inaction.
//
// These tests pin the delta, because a delta of nothing is exactly what the
// obvious implementation produces: base holdings already exclude expired
// grants, so reading "before" and "after" from the database compares a world
// without this grant against a world without this grant.

// revokedRoles is every role the params revoke, "project/role".
func revokedRoles(params []db.EnqueueParams) []string {
	out := []string{}
	for _, p := range params {
		if p.OpType == "revoke" {
			for _, r := range p.RoleKeys {
				out = append(out, p.ProjectID+"/"+r)
			}
		}
	}
	return out
}

// lapsedGrant is the sweep's candidate: g_88, pLaser/trained, ended an hour ago.
func lapsedGrant() models.DirectGrant {
	ended := time.Now().Add(-time.Hour)
	return models.DirectGrant{ID: "g_88", UserID: "u1", ProjectID: "pLaser", RoleKey: "trained", ExpiresAt: &ended}
}

// The planner decides an expiry on plain data (task 1.9). "Until 30 Nov" is
// the whole of 30 Nov in the makerspace's zone (IST): at 23:59:58 that night
// the grant still stands and nothing is planned; at midnight it has ended and
// its role is revoked.
func TestPlanExpiry_EndsAtMidnightInTheMakerspacesZone(t *testing.T) {
	ist, err := time.LoadLocation("Asia/Kolkata")
	if err != nil {
		t.Skip("no tz data")
	}
	end := time.Date(2026, 11, 30, 23, 59, 59, 0, ist) // clock.EndOfDay("2026-11-30") in IST
	g := models.DirectGrant{ID: "g1", UserID: "u1", ProjectID: "pLaser", RoleKey: "trained", ExpiresAt: &end}
	s := PlanSnapshot{Access: db.AccessSnapshot{}}

	s.Now = time.Date(2026, 11, 30, 23, 59, 58, 0, ist)
	if ch := planExpiry(g, db.PersonAccess{}, s); ch.Refusal != "renewed" || len(ch.Writes) != 0 {
		t.Fatalf("on its last day the grant stands: %+v", ch)
	}
	s.Now = time.Date(2026, 12, 1, 0, 0, 0, 0, ist)
	ch := planExpiry(g, db.PersonAccess{}, s)
	if ch.Refusal != "" || len(ch.Writes) != 1 || ch.Writes[0].OpType != "revoke" || ch.Writes[0].SourceRef != "g1" ||
		ch.Writes[0].Delivery != DeliveryStraightAway {
		t.Fatalf("at midnight its role is revoked, straight away: %+v", ch.Writes)
	}
	// In UTC that midnight is still 30 Nov (18:30): the day is the makerspace's.
	if s.Now.UTC().Day() != 30 {
		t.Fatal("fixture: the boundary must fall on a different UTC day")
	}
}

// A grant renewed since the sweep read it is held under the lock with its new
// end, and the planner reads that, not the sweep's stale candidate.
func TestPlanExpiry_ARenewedGrantIsNotEnded(t *testing.T) {
	past, later := time.Now().Add(-time.Hour), time.Now().AddDate(0, 0, 7)
	stale := models.DirectGrant{ID: "g1", UserID: "u1", ProjectID: "pLaser", RoleKey: "trained", ExpiresAt: &past}
	person := db.PersonAccess{Direct: []models.DirectGrant{{ID: "g1", UserID: "u1", ProjectID: "pLaser", RoleKey: "trained", ExpiresAt: &later}}}
	if ch := planExpiry(stale, person, PlanSnapshot{Now: time.Now()}); ch.Refusal != "renewed" {
		t.Fatalf("a renewed grant is not ended: %+v", ch)
	}
}

// An expiry's writes are delivered as its role says (decision 11), and the
// give of the lapsed role still waiting to be sent is the one queued change
// it cancels: sending it later would restore access that ended. Nothing else
// is cancelled (system work cancels only what it planned).
func TestPlanExpiry_WaitsAsItsRoleDoesAndCancelsOnlyItsOwnWaitingGive(t *testing.T) {
	past := time.Now().Add(-time.Hour)
	g := models.DirectGrant{ID: "g1", UserID: "u1", ProjectID: "pLaser", RoleKey: "trained", ExpiresAt: &past}
	rules := []models.MappingRule{{ID: "r1", SourceProject: "pLaser", SourceRole: "trained", TargetProject: "pWiki", TargetRole: "editor"}}
	person := db.PersonAccess{Queued: []models.QueuedChange{
		{ID: "q-give", UserID: "u1", OpType: "add", ProjectID: "pLaser", RoleKeys: []string{"trained"}, Source: "direct", SourceRef: "g1"},
		{ID: "q-other", UserID: "u1", OpType: "add", ProjectID: "pShop", RoleKeys: []string{"member"}, Source: "direct"},
	}}
	s := PlanSnapshot{Now: time.Now(), Access: db.AccessSnapshot{Rules: rules,
		RoleWaits: map[db.RoleRef]bool{{ProjectID: "pWiki", RoleKey: "editor"}: true}}}

	ch := planExpiry(g, person, s)
	by := map[string]Write{}
	for _, w := range ch.Writes {
		by[w.ProjectID+"/"+w.RoleKeys[0]] = w
	}
	if w := by["pLaser/trained"]; w.Delivery != DeliveryNone {
		t.Errorf("the lapsed role's give never went, so its revoke only cancels it: %+v", w)
	}
	if w := by["pWiki/editor"]; w.Delivery != DeliveryWaits || w.Source != "rule" || w.SourceRef != "r1" {
		t.Errorf("the rule's role waits for you, as the rule's row: %+v", w)
	}
	if len(ch.Row.CancelIDs) != 1 || ch.Row.CancelIDs[0] != "q-give" {
		t.Errorf("cancels only its own waiting give, got %v", ch.Row.CancelIDs)
	}
}

// expiryFixture wires the closure-diff injectables and captures what the expiry
// enqueues, alongside the error the guarded delete returns.
func expiryFixture(
	t *testing.T,
	directs []models.DirectGrant,
	bundleRoles map[string][]models.BundleRole,
	rules []models.MappingRule,
	deleteErr error,
) *[]db.EnqueueParams {
	t.Helper()
	resetCascadeDeps(t)

	orig := svcDeleteExpiredDirectGrantAndEnqueue
	origTx := svcInTxLockingAccess
	t.Cleanup(func() {
		svcDeleteExpiredDirectGrantAndEnqueue = orig
		svcInTxLockingAccess = origTx
	})
	// The real one opens a transaction and takes the subject lock; neither is
	// reachable without a database. What the fake must preserve is the ordering
	// the lock exists for, which the source guard in internal/db asserts.
	svcInTxLockingAccess = func(ctx context.Context, fn func(context.Context) error) error {
		return fn(ctx)
	}

	svcGetDirectGrantsForUser = func(context.Context, string, bool) ([]models.DirectGrant, error) {
		return directs, nil
	}
	svcGetUserBundleRolesGrouped = func(context.Context, string) (map[string][]models.BundleRole, error) {
		return bundleRoles, nil
	}
	svcGetActiveMappingRules = func(context.Context) ([]models.MappingRule, error) {
		return rules, nil
	}

	captured := &[]db.EnqueueParams{}
	svcDeleteExpiredDirectGrantAndEnqueue = func(
		_ context.Context, _, _, _ string, params []db.EnqueueParams,
	) (string, string, []string, error) {
		*captured = params
		if deleteErr != nil {
			return "", "", nil, deleteErr
		}
		ids := make([]string, len(params))
		for i := range params {
			ids[i] = "ob"
		}
		return "pLaser", "trained", ids, nil
	}
	return captured
}

// The regression this file exists for: a lapsed grant nothing else covers must
// produce a revoke. The expired row is already absent from every holdings read,
// so the "before" state has to be reconstructed from the fact that it lapsed.
func TestExpireDirectGrant_LapsedRoleIsRevoked(t *testing.T) {
	captured := expiryFixture(t, nil, nil, nil, nil)

	res, err := ExpireDirectGrant(context.Background(), lapsedGrant(), "system:scheduler")
	if err != nil {
		t.Fatalf("ExpireDirectGrant: %v", err)
	}
	if got := revokedRoles(*captured); len(got) != 1 || got[0] != "pLaser/trained" {
		t.Fatalf("the lapsed role must be revoked upstream, got %v", got)
	}
	if len(res.OutboxIDs) != 1 {
		t.Fatalf("the revoke must be queued, got %v", res.OutboxIDs)
	}
	if len(res.Retained) != 0 {
		t.Errorf("nothing else covers this role; nothing is retained, got %v", res.Retained)
	}
	// Every queued row is a revoke, on the target direct grants live on.
	for _, p := range *captured {
		if p.OpType != "revoke" {
			t.Errorf("expiry queues revocations only, got op %q", p.OpType)
		}
		if p.SourceRef != "g_88" {
			t.Errorf("the queued row must name the grant that lapsed, got %q", p.SourceRef)
		}
	}
}

// A role the subject still holds through a bundle is not lost when one grant of
// it lapses. Revoking it would take access away that they demonstrably still
// have, until the next compile put it back.
func TestExpireDirectGrant_RoleAlsoInBundle_RevokesNothing(t *testing.T) {
	captured := expiryFixture(t, nil,
		map[string][]models.BundleRole{"b_lab": {{ProjectID: "pLaser", RoleKey: "trained"}}},
		nil, nil)

	res, err := ExpireDirectGrant(context.Background(), lapsedGrant(), "system:scheduler")
	if err != nil {
		t.Fatalf("ExpireDirectGrant: %v", err)
	}
	if got := revokedRoles(*captured); len(got) != 0 {
		t.Fatalf("the bundle still carries pLaser/trained; nothing may be revoked, got %v", got)
	}
	if len(res.Retained) != 1 || res.Retained[0] != "pLaser/trained" {
		t.Errorf("the retained role must be reported, got %v", res.Retained)
	}
}

// A rule-derived role this grant alone supported goes with it. One the subject
// still derives another way does not.
func TestExpireDirectGrant_DerivedRolesFollowTheirSupport(t *testing.T) {
	rules := []models.MappingRule{
		{SourceProject: "pLaser", SourceRole: "trained", TargetProject: "pWiki", TargetRole: "editor"},
		{SourceProject: "pShop", SourceRole: "member", TargetProject: "pWiki", TargetRole: "editor"},
	}

	// Nothing else derives pWiki/editor: it lapses with the grant.
	captured := expiryFixture(t, nil, nil, rules, nil)
	if _, err := ExpireDirectGrant(context.Background(), lapsedGrant(), "system:scheduler"); err != nil {
		t.Fatal(err)
	}
	got := revokedRoles(*captured)
	if len(got) != 2 || !contains(got, "pLaser/trained") || !contains(got, "pWiki/editor") {
		t.Fatalf("the derived role must lapse with its only support, got %v", got)
	}

	// The subject also holds pShop/member, which derives the same role.
	captured = expiryFixture(t,
		[]models.DirectGrant{{ID: "g_other", UserID: "u1", ProjectID: "pShop", RoleKey: "member"}},
		nil, rules, nil)
	if _, err := ExpireDirectGrant(context.Background(), lapsedGrant(), "system:scheduler"); err != nil {
		t.Fatal(err)
	}
	got = revokedRoles(*captured)
	if len(got) != 1 || got[0] != "pLaser/trained" {
		t.Fatalf("a derived role with another live source must survive, got %v", got)
	}
}

// A grant renewed between the sweep's fetch and its write is alive again. The
// refusal comes from the delete's own predicate, and the delta computed for a
// world where the grant is gone goes with the transaction.
func TestExpireDirectGrant_RenewedGrantSurfacesAsSuch(t *testing.T) {
	captured := expiryFixture(t, nil, nil, nil, db.ErrGrantRenewed)

	res, err := ExpireDirectGrant(context.Background(), lapsedGrant(), "system:scheduler")
	if !errors.Is(err, db.ErrGrantRenewed) {
		t.Fatalf("a renewed grant must surface as ErrGrantRenewed, got %v", err)
	}
	if len(res.OutboxIDs) != 0 || len(res.Revoked) != 0 {
		t.Errorf("nothing may be reported as queued when the write refused, got %+v", res)
	}
	// The delta was still computed and offered — it is the transaction that
	// discards it, not the caller's judgement.
	if len(*captured) == 0 {
		t.Error("the delta must be handed to the write, which is what decides")
	}
}

func contains(hay []string, needle string) bool {
	for _, h := range hay {
		if h == needle {
			return true
		}
	}
	return false
}

// A grant somebody else removed while this sweep was mid-way through expiring
// it is not a renewal. Reporting it as one would tell the operator everything
// is fine; it is the other verdict that says something is wrong.
func TestExpireDirectGrant_MissingGrantIsNotARenewal(t *testing.T) {
	expiryFixture(t, nil, nil, nil, db.ErrGrantNotFound)

	_, err := ExpireDirectGrant(context.Background(), lapsedGrant(), "system:scheduler")
	if !errors.Is(err, db.ErrGrantNotFound) {
		t.Fatalf("an absent grant must surface as ErrGrantNotFound, got %v", err)
	}
	if errors.Is(err, db.ErrGrantRenewed) {
		t.Error("absence must not be reported as a renewal")
	}
}

// The delta is computed inside the transaction that takes the subject lock. A
// bundle assignment landing between the read and the commit makes the revoke a
// statement about a world that no longer exists — and the add it queued lands
// first, so the subject ends up without access they are currently owed.
func TestExpireDirectGrant_ComputesTheDeltaUnderTheAccessLock(t *testing.T) {
	resetCascadeDeps(t)
	origTx := svcInTxLockingAccess
	origDel := svcDeleteExpiredDirectGrantAndEnqueue
	t.Cleanup(func() {
		svcInTxLockingAccess = origTx
		svcDeleteExpiredDirectGrantAndEnqueue = origDel
	})

	var order []string
	svcInTxLockingAccess = func(ctx context.Context, fn func(context.Context) error) error {
		order = append(order, "lock")
		return fn(ctx)
	}
	svcGetActiveMappingRules = func(context.Context) ([]models.MappingRule, error) {
		order = append(order, "read:rules")
		return nil, nil
	}
	svcGetDirectGrantsForUser = func(context.Context, string, bool) ([]models.DirectGrant, error) {
		order = append(order, "read:grants")
		return nil, nil
	}
	svcGetUserBundleRolesGrouped = func(context.Context, string) (map[string][]models.BundleRole, error) {
		return nil, nil
	}
	svcDeleteExpiredDirectGrantAndEnqueue = func(
		_ context.Context, _, _, _ string, ps []db.EnqueueParams,
	) (string, string, []string, error) {
		order = append(order, "write")
		return "pLaser", "trained", oneIDEach(ps), nil
	}

	if _, err := ExpireDirectGrant(context.Background(), lapsedGrant(), "system:scheduler"); err != nil {
		t.Fatal(err)
	}
	if len(order) < 3 || order[0] != "lock" || order[len(order)-1] != "write" {
		t.Fatalf("the lock must precede the reads and the write must follow them, got %v", order)
	}
	for _, step := range order[1 : len(order)-1] {
		if step == "lock" {
			t.Fatalf("the lock must be taken once, before the reads, got %v", order)
		}
	}
}

// The intent ledger keeps a grant until the target confirms it is gone, so for
// as long as a revocation waits in the queue the ledger says the subject still
// holds the role while the decision to remove it has already been taken.
//
// A cascade computing a delta from that answer concludes nothing is missing and
// queues nothing, and the revocation lands anyway: the subject ends up holding
// the source and not the access, with no queued row disagreeing. Locking the
// reconciliation does not help — it runs after the target has already applied
// the revoke.
func TestClosureTreatsAQueuedRevocationAsAlreadyGone(t *testing.T) {
	resetCascadeDeps(t)
	svcGetDirectGrantsForUser = func(context.Context, string, bool) ([]models.DirectGrant, error) {
		return []models.DirectGrant{{ID: "g1", UserID: "u1", ProjectID: "pLaser", RoleKey: "trained"}}, nil
	}
	svcGetUserBundleRolesGrouped = func(context.Context, string) (map[string][]models.BundleRole, error) {
		return nil, nil
	}

	// Nothing queued: the ledger row is the answer.
	base, err := userBaseHoldings(context.Background(), "u1")
	if err != nil {
		t.Fatal(err)
	}
	if !base[roleKey{projectID: "pLaser", roleKey: "trained"}] {
		t.Fatal("a live grant with nothing queued against it is held")
	}

	// A revocation waiting in the outbox is a decision already taken.
	svcQueuedRevocations = func(context.Context, string) ([]db.RoleRef, error) {
		return []db.RoleRef{{ProjectID: "pLaser", RoleKey: "trained"}}, nil
	}
	base, err = userBaseHoldings(context.Background(), "u1")
	if err != nil {
		t.Fatal(err)
	}
	if base[roleKey{projectID: "pLaser", roleKey: "trained"}] {
		t.Fatal("a role with an unresolved revocation must not count as held — the delta that follows would queue nothing and the revoke would land anyway")
	}

	// And a cascade that wants the role therefore queues it, behind the revoke.
	captured := expiryFixture(t, nil, nil, nil, nil)
	svcQueuedRevocations = func(context.Context, string) ([]db.RoleRef, error) {
		return []db.RoleRef{{ProjectID: "pLaser", RoleKey: "trained"}}, nil
	}
	_ = captured
	// The exclusion applies to every base read, not only the plain one.
	svcGetDirectGrantsForUser = func(context.Context, string, bool) ([]models.DirectGrant, error) {
		return []models.DirectGrant{{ID: "g1", UserID: "u1", ProjectID: "pLaser", RoleKey: "trained"}}, nil
	}
	for name, read := range map[string]func() (map[roleKey]bool, error){
		"excluding bundle": func() (map[roleKey]bool, error) {
			return userBaseHoldingsExcludingBundle(context.Background(), "u1", "b_other")
		},
		"with a bundle at a version": func() (map[roleKey]bool, error) {
			return userBaseHoldingsWithBundleAt(context.Background(), "u1", "b_other", nil)
		},
	} {
		got, err := read()
		if err != nil {
			t.Fatal(err)
		}
		if got[roleKey{projectID: "pLaser", roleKey: "trained"}] {
			t.Errorf("%s: a queued revocation must be subtracted from every effective-access read", name)
		}
	}
}
