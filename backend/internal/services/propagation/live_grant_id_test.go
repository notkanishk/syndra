package propagation

import (
	"context"
	"testing"

	"syndra/internal/db"
	"syndra/internal/models"
	"syndra/internal/zitadel"
)

// row carrying the grant id the outbox remembered when it was enqueued.
func rowWithRememberedGrant(id, op string, roleKeys []string) func(context.Context, string, int) ([]models.PendingPropagation, error) {
	return func(context.Context, string, int) ([]models.PendingPropagation, error) {
		return []models.PendingPropagation{{
			ID: id, Target: db.TargetZitadel, OpType: op,
			UserID: "u", ProjectID: "p", RoleKeys: roleKeys,
			ZitadelGrantID: "remembered-and-stale",
		}}, nil
	}
}

// propagation_outbox.zitadel_grant_id is read from the grant index at ENQUEUE
// time (db/cascade.go). In manual mode the row then waits in Pending changes
// until somebody confirms it — minutes or days. If the grant was removed and
// recreated in Zitadel in that window, the remembered id names nothing, and the
// revoke 404s on every attempt until its budget runs out. The read that decides
// what survives already returns the id that exists now; use that one.
func TestDrain_RevokeWritesToTheGrantThatExistsNow(t *testing.T) {
	stubDrainDeps(t)
	claimPending = rowWithRememberedGrant("rv-live", "revoke", []string{"r1"})
	liveUserGrant = func(context.Context, string, string) (string, map[string]bool, error) {
		return "live-id", map[string]bool{"r1": true, "r2": true}, nil
	}
	var updatedWith string
	zitadelUpdateUserGrant = func(_ context.Context, _, grantID string, _ []string) error {
		updatedWith = grantID
		return nil
	}

	if _, err := Drain(context.Background()); err != nil {
		t.Fatalf("drain: %v", err)
	}
	if updatedWith != "live-id" {
		t.Fatalf("revoke must write to the live grant id, got %q", updatedWith)
	}
}

// Same defect on the whole-grant branch: nothing survives the revoke, so the
// grant is removed — and removing the remembered id removes nothing.
func TestDrain_RevokeOfEveryRoleRemovesTheGrantThatExistsNow(t *testing.T) {
	stubDrainDeps(t)
	claimPending = rowWithRememberedGrant("rv-sole-live", "revoke", []string{"r1"})
	liveUserGrant = func(context.Context, string, string) (string, map[string]bool, error) {
		return "live-id", map[string]bool{"r1": true}, nil
	}
	var removedID string
	zitadelRemoveUserGrant = func(_ context.Context, _, grantID string) error {
		removedID = grantID
		return nil
	}

	if _, err := Drain(context.Background()); err != nil {
		t.Fatalf("drain: %v", err)
	}
	if removedID != "live-id" {
		t.Fatalf("revoke must remove the live grant id, got %q", removedID)
	}
}

// The grant goes away between the spent-check and the dispatch — another row of
// the same cascade removed it, or an operator did, in the seconds between two
// reads. The row now asks for an end state that is already the live one, and
// the read saying so is complete (liveUserGrant refuses a truncated page).
// Settle it; do not spend the retry budget calling Remove on an id nobody
// holds. alreadyExists cannot cover this — it answered before the grant went.
func TestDrain_RevokeOfAGrantAlreadyGoneSettlesWithoutCalling(t *testing.T) {
	stubDrainDeps(t)
	claimPending = rowWithRememberedGrant("rv-gone", "revoke", []string{"r1"})
	reads := 0
	liveUserGrant = func(context.Context, string, string) (string, map[string]bool, error) {
		reads++
		if reads == 1 {
			return "live-id", map[string]bool{"r1": true}, nil // still there: the revoke must run
		}
		return "", map[string]bool{}, nil // gone by the time we dispatch
	}
	called := false
	zitadelRemoveUserGrant = func(context.Context, string, string) error { called = true; return nil }
	zitadelUpdateUserGrant = func(context.Context, string, string, []string) error { called = true; return nil }

	res, err := Drain(context.Background())
	if err != nil {
		t.Fatalf("drain: %v", err)
	}
	if called {
		t.Fatal("nothing to revoke, yet a Zitadel write was attempted")
	}
	if res.Applied != 1 || res.Requeued != 0 || res.Failed != 0 {
		t.Fatalf("want the row settled, got %+v", res)
	}
}

// replace carried the same remembered id.
func TestDrain_ReplaceWritesToTheGrantThatExistsNow(t *testing.T) {
	stubDrainDeps(t)
	claimPending = rowWithRememberedGrant("rp-live", "replace", []string{"new"})
	liveUserGrant = func(context.Context, string, string) (string, map[string]bool, error) {
		return "live-id", map[string]bool{"old": true}, nil
	}
	var updatedWith string
	var updatedRoles []string
	zitadelUpdateUserGrant = func(_ context.Context, _, grantID string, roles []string) error {
		updatedWith, updatedRoles = grantID, roles
		return nil
	}

	if _, err := Drain(context.Background()); err != nil {
		t.Fatalf("drain: %v", err)
	}
	if updatedWith != "live-id" {
		t.Fatalf("replace must write to the live grant id, got %q", updatedWith)
	}
	if len(updatedRoles) != 1 || updatedRoles[0] != "new" {
		t.Fatalf("replace sets exactly the row's roles, got %v", updatedRoles)
	}
}

// A replace whose grant Zitadel no longer holds: the end state the row names is
// still a grant with those roles, so create it rather than failing forever
// against an id that is gone.
func TestDrain_ReplaceRecreatesAGrantZitadelNoLongerHolds(t *testing.T) {
	stubDrainDeps(t)
	claimPending = rowWithRememberedGrant("rp-gone", "replace", []string{"new"})
	liveUserGrant = func(context.Context, string, string) (string, map[string]bool, error) {
		return "", map[string]bool{}, nil
	}
	var addedRoles []string
	zitadelAddUserGrant = func(_ context.Context, _, _ string, roles []string) error {
		addedRoles = roles
		return nil
	}
	zitadelUpdateUserGrant = func(context.Context, string, string, []string) error {
		t.Fatal("no grant exists; update cannot be the call")
		return nil
	}

	res, err := Drain(context.Background())
	if err != nil {
		t.Fatalf("drain: %v", err)
	}
	if len(addedRoles) != 1 || addedRoles[0] != "new" {
		t.Fatalf("want the grant recreated with the row's roles, got %v", addedRoles)
	}
	if res.Applied != 1 {
		t.Fatalf("want 1 applied, got %+v", res)
	}
}

// truncatedGrants answers a user-grant listing with fewer items than it says
// exist — the shape a person with more grants than one page holds.
type truncatedGrants struct {
	zitadel.ZitadelClient
	items []zitadel.UserGrant
	total int
}

func (f truncatedGrants) ListUserGrants(context.Context, string, zitadel.SearchParams) (*zitadel.SearchResult[zitadel.UserGrant], error) {
	return &zitadel.SearchResult[zitadel.UserGrant]{Items: f.items, Total: f.total}, nil
}

// Every caller of liveUserGrant draws a conclusion from an EMPTY answer: add
// creates a grant, replace recreates one, revoke calls itself done. Absence may
// only be concluded from a read that saw everything, so a page that admits it
// did not is an error rather than an empty answer.
func TestLiveUserGrant_RefusesAPageThatDidNotSeeEverything(t *testing.T) {
	prev := zitadel.MgmtClient
	t.Cleanup(func() { zitadel.MgmtClient = prev })

	zitadel.MgmtClient = truncatedGrants{
		items: []zitadel.UserGrant{{ID: "g1", ProjectID: "other", RoleKeys: []string{"x"}}},
		total: 120,
	}
	if _, _, err := liveUserGrant(context.Background(), "u", "p"); err == nil {
		t.Fatal("a truncated listing must not be reported as 'no grant on this project'")
	}

	// The same listing, complete, answers normally.
	zitadel.MgmtClient = truncatedGrants{
		items: []zitadel.UserGrant{{ID: "g1", ProjectID: "p", RoleKeys: []string{"x"}}},
		total: 1,
	}
	id, roles, err := liveUserGrant(context.Background(), "u", "p")
	if err != nil {
		t.Fatalf("complete listing: %v", err)
	}
	if id != "g1" || !roles["x"] {
		t.Fatalf("want the grant read back, got id=%q roles=%v", id, roles)
	}
}
