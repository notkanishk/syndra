package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"mkauth/internal/models"
	"mkauth/internal/zitadel"
)

// withReconciliationDeps swaps in deterministic stub data for the two
// injection points reconciliation depends on, and restores the originals
// when the test returns. Keeps tests isolated from the real DB and Zitadel.
func withReconciliationDeps(
	t *testing.T,
	mkauth []models.DirectGrant,
	zitadelGrants []zitadel.UserGrant,
	zitadelTotal int,
	zitadelErr error,
) {
	t.Helper()

	origAll := svcAllDirectGrants
	origZitadel := zitadelListAllGrants

	svcAllDirectGrants = func(_ context.Context) ([]models.DirectGrant, error) {
		return mkauth, nil
	}
	// Pagination-aware stub: slices the master list by the requested offset
	// and limit so the handler's pagination loop terminates correctly. Total
	// always reports the master length (or zitadelTotal override) so the loop
	// knows whether more pages remain.
	zitadelListAllGrants = func(_ context.Context, p zitadel.SearchParams) (*zitadel.SearchResult[zitadel.UserGrant], error) {
		if zitadelErr != nil {
			return nil, zitadelErr
		}
		total := zitadelTotal
		if total == 0 {
			total = len(zitadelGrants)
		}
		start := min(p.Offset, len(zitadelGrants))
		end := min(start+p.Limit, len(zitadelGrants))
		if p.Limit == 0 {
			end = len(zitadelGrants)
		}
		return &zitadel.SearchResult[zitadel.UserGrant]{
			Items: zitadelGrants[start:end],
			Total: total,
		}, nil
	}

	t.Cleanup(func() {
		svcAllDirectGrants = origAll
		zitadelListAllGrants = origZitadel
	})
}

func directGrant(userID, projectID, roleKey string) models.DirectGrant {
	return models.DirectGrant{
		ID:        userID + ":" + projectID + ":" + roleKey,
		UserID:    userID,
		ProjectID: projectID,
		RoleKey:   roleKey,
		GrantedBy: "test",
		CreatedAt: time.Unix(0, 0).UTC(),
		UpdatedAt: time.Unix(0, 0).UTC(),
	}
}

func getReconciliation(t *testing.T) *httptest.ResponseRecorder {
	t.Helper()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/reconciliation/grants", nil)
	handleGetReconciliationDiff(rr, req)
	return rr
}

func decodeReconciliation(t *testing.T, rr *httptest.ResponseRecorder) ReconciliationDiff {
	t.Helper()
	var got ReconciliationDiff
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return got
}

// TestReconciliation_OnlyInMkAuth: a (user, project) pair exists on the
// MkAuth side but Zitadel has no grant for it. Should land in only_in_mkauth
// with all roles aggregated, no drift, no only_in_zitadel entry.
func TestReconciliation_OnlyInMkAuth(t *testing.T) {
	withReconciliationDeps(t,
		[]models.DirectGrant{
			directGrant("u-1", "p-1", "viewer"),
			directGrant("u-1", "p-1", "editor"),
		},
		nil, 0, nil,
	)

	rr := getReconciliation(t)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}
	got := decodeReconciliation(t, rr)

	if len(got.OnlyInMkAuth) != 1 || len(got.OnlyInZitadel) != 0 || len(got.Drift) != 0 {
		t.Fatalf("expected 1 only-in-mkauth and nothing else, got %+v", got)
	}
	entry := got.OnlyInMkAuth[0]
	if entry.UserID != "u-1" || entry.ProjectID != "p-1" {
		t.Fatalf("unexpected pair: %+v", entry)
	}
	if !reflect.DeepEqual(entry.RoleKeys, []string{"editor", "viewer"}) {
		t.Fatalf("expected sorted [editor viewer], got %+v", entry.RoleKeys)
	}
	if got.Truncated {
		t.Fatalf("did not expect truncation flag")
	}
}

// TestReconciliation_OnlyInZitadel: a Zitadel grant has no MkAuth counterpart.
// Most often a derived (mapping rule) grant or pre-existing manual grant.
func TestReconciliation_OnlyInZitadel(t *testing.T) {
	withReconciliationDeps(t,
		nil,
		[]zitadel.UserGrant{
			{ID: "g-7", UserID: "u-1", ProjectID: "p-1", RoleKeys: []string{"derived_role"}},
		}, 0, nil,
	)

	rr := getReconciliation(t)
	got := decodeReconciliation(t, rr)

	if len(got.OnlyInZitadel) != 1 || len(got.OnlyInMkAuth) != 0 || len(got.Drift) != 0 {
		t.Fatalf("expected only_in_zitadel=1, got %+v", got)
	}
	entry := got.OnlyInZitadel[0]
	if entry.GrantID != "g-7" {
		t.Fatalf("expected grant id g-7 to flow through for drill-in, got %q", entry.GrantID)
	}
	if !reflect.DeepEqual(entry.RoleKeys, []string{"derived_role"}) {
		t.Fatalf("unexpected roles: %+v", entry.RoleKeys)
	}
}

// TestReconciliation_RoleMismatch: same (user, project) pair on both sides
// but role sets differ. The drift entry must enumerate the missing-from-each-
// side keys explicitly so the operator sees the exact gap.
func TestReconciliation_RoleMismatch(t *testing.T) {
	withReconciliationDeps(t,
		[]models.DirectGrant{
			directGrant("u-1", "p-1", "viewer"),
			directGrant("u-1", "p-1", "editor"),
		},
		[]zitadel.UserGrant{
			{ID: "g-1", UserID: "u-1", ProjectID: "p-1", RoleKeys: []string{"viewer", "admin"}},
		}, 0, nil,
	)

	got := decodeReconciliation(t, getReconciliation(t))

	if len(got.OnlyInMkAuth) != 0 || len(got.OnlyInZitadel) != 0 {
		t.Fatalf("expected drift only, got %+v", got)
	}
	if len(got.Drift) != 1 {
		t.Fatalf("expected one drift entry, got %d", len(got.Drift))
	}
	d := got.Drift[0]
	if !reflect.DeepEqual(d.OnlyInMkAuth, []string{"editor"}) {
		t.Fatalf("expected only-in-mkauth=[editor], got %+v", d.OnlyInMkAuth)
	}
	if !reflect.DeepEqual(d.OnlyInZitadel, []string{"admin"}) {
		t.Fatalf("expected only-in-zitadel=[admin], got %+v", d.OnlyInZitadel)
	}
	if d.GrantID != "g-1" {
		t.Fatalf("expected GrantID=g-1 for drill-in, got %q", d.GrantID)
	}
	if !reflect.DeepEqual(d.MkAuthRoles, []string{"editor", "viewer"}) {
		t.Fatalf("MkAuthRoles not sorted asc, got %+v", d.MkAuthRoles)
	}
	if !reflect.DeepEqual(d.ZitadelRoles, []string{"admin", "viewer"}) {
		t.Fatalf("ZitadelRoles not sorted asc, got %+v", d.ZitadelRoles)
	}
}

// TestReconciliation_RoleSuperset: MkAuth has a role that Zitadel is missing.
// Reported as drift (not only_in_mkauth), because the (user, project) pair
// itself is present on both sides — only the role set differs.
func TestReconciliation_RoleSuperset(t *testing.T) {
	withReconciliationDeps(t,
		[]models.DirectGrant{
			directGrant("u-1", "p-1", "viewer"),
			directGrant("u-1", "p-1", "editor"),
		},
		[]zitadel.UserGrant{
			{ID: "g-1", UserID: "u-1", ProjectID: "p-1", RoleKeys: []string{"viewer"}},
		}, 0, nil,
	)

	got := decodeReconciliation(t, getReconciliation(t))
	if len(got.Drift) != 1 || len(got.OnlyInMkAuth) != 0 {
		t.Fatalf("expected drift-only superset, got %+v", got)
	}
	d := got.Drift[0]
	if !reflect.DeepEqual(d.OnlyInMkAuth, []string{"editor"}) || len(d.OnlyInZitadel) != 0 {
		t.Fatalf("unexpected drift shape: %+v", d)
	}
}

// TestReconciliation_Aligned: identical role sets on both sides emit no
// drift entry and no other side reports.
func TestReconciliation_Aligned(t *testing.T) {
	withReconciliationDeps(t,
		[]models.DirectGrant{
			directGrant("u-1", "p-1", "viewer"),
			directGrant("u-1", "p-1", "editor"),
		},
		[]zitadel.UserGrant{
			{ID: "g-1", UserID: "u-1", ProjectID: "p-1", RoleKeys: []string{"editor", "viewer"}},
		}, 0, nil,
	)
	got := decodeReconciliation(t, getReconciliation(t))
	if len(got.OnlyInMkAuth) != 0 || len(got.OnlyInZitadel) != 0 || len(got.Drift) != 0 {
		t.Fatalf("expected all-empty diff, got %+v", got)
	}
	if got.GeneratedAt.IsZero() {
		t.Fatalf("expected generated_at to be populated")
	}
}

// TestReconciliation_TruncationFlag: when the Zitadel grant inventory exceeds
// the safety cap the handler stops paginating and sets truncated=true.
// Override the package-level cap to a small number so the test can construct
// a master list that exceeds it without bloating the test fixture.
func TestReconciliation_TruncationFlag(t *testing.T) {
	origCap := reconciliationSafetyCap
	origPageSize := reconciliationPageSize
	reconciliationSafetyCap = 3
	reconciliationPageSize = 2 // force multi-page iteration so cap fires
	t.Cleanup(func() {
		reconciliationSafetyCap = origCap
		reconciliationPageSize = origPageSize
	})

	// 5 grants × page size 2 → after page 1 (2 items) the loop continues;
	// after page 2 (4 items) the cap (3) trips on the next iteration.
	grants := []zitadel.UserGrant{
		{ID: "g-1", UserID: "u-1", ProjectID: "p-1", RoleKeys: []string{"r"}},
		{ID: "g-2", UserID: "u-2", ProjectID: "p-1", RoleKeys: []string{"r"}},
		{ID: "g-3", UserID: "u-3", ProjectID: "p-1", RoleKeys: []string{"r"}},
		{ID: "g-4", UserID: "u-4", ProjectID: "p-1", RoleKeys: []string{"r"}},
		{ID: "g-5", UserID: "u-5", ProjectID: "p-1", RoleKeys: []string{"r"}},
	}
	withReconciliationDeps(t, nil, grants, 0, nil)

	got := decodeReconciliation(t, getReconciliation(t))
	if !got.Truncated {
		t.Fatalf("expected Truncated=true when zitadel inventory exceeds safety cap")
	}
}

// TestReconciliation_PaginatesUntilTotalReached: with a per-page size smaller
// than the Zitadel inventory, the handler MUST iterate every page so the
// diff is authoritative. Comparing MkAuth's full inventory against only the
// first page would false-positive grants on later pages into only_in_mkauth.
func TestReconciliation_PaginatesUntilTotalReached(t *testing.T) {
	origPageSize := reconciliationPageSize
	reconciliationPageSize = 2
	t.Cleanup(func() { reconciliationPageSize = origPageSize })

	zitadelGrants := []zitadel.UserGrant{
		{ID: "g-1", UserID: "u-1", ProjectID: "p-1", RoleKeys: []string{"r"}},
		{ID: "g-2", UserID: "u-2", ProjectID: "p-1", RoleKeys: []string{"r"}},
		{ID: "g-3", UserID: "u-3", ProjectID: "p-1", RoleKeys: []string{"r"}},
		{ID: "g-4", UserID: "u-4", ProjectID: "p-1", RoleKeys: []string{"r"}},
		{ID: "g-5", UserID: "u-5", ProjectID: "p-1", RoleKeys: []string{"r"}},
	}
	// MkAuth side mirrors u-3..u-5 — these would false-positive into
	// only_in_mkauth if the loop stopped after page 1 (which only contains
	// u-1 and u-2 at page size 2).
	mkauthGrants := []models.DirectGrant{
		directGrant("u-3", "p-1", "r"),
		directGrant("u-4", "p-1", "r"),
		directGrant("u-5", "p-1", "r"),
	}
	withReconciliationDeps(t, mkauthGrants, zitadelGrants, 0, nil)

	got := decodeReconciliation(t, getReconciliation(t))
	if got.Truncated {
		t.Fatalf("did not expect truncation under the cap")
	}
	if len(got.OnlyInMkAuth) != 0 {
		t.Fatalf("pagination should have matched u-3..u-5 against later Zitadel pages, got only_in_mkauth=%+v", got.OnlyInMkAuth)
	}
	if len(got.Drift) != 0 {
		t.Fatalf("u-3..u-5 role sets agree on both sides, expected no drift, got %+v", got.Drift)
	}
}

// TestReconciliation_ZitadelFailure: the handler must surface a 502 when
// Zitadel is unreachable rather than returning a misleading "all aligned"
// snapshot from the MkAuth side alone.
func TestReconciliation_ZitadelFailure(t *testing.T) {
	withReconciliationDeps(t, nil, nil, 0, errors.New("upstream unavailable"))

	rr := getReconciliation(t)
	if rr.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d body=%s", rr.Code, rr.Body.String())
	}
}

// TestReconciliation_MultipleUsersStableOrder: the response orders entries
// deterministically (user_id, project_id ascending) so consumers can rely on
// a stable diff between fetches.
func TestReconciliation_MultipleUsersStableOrder(t *testing.T) {
	withReconciliationDeps(t,
		[]models.DirectGrant{
			directGrant("u-2", "p-1", "r"),
			directGrant("u-1", "p-2", "r"),
			directGrant("u-1", "p-1", "r"),
		},
		nil, 0, nil,
	)

	got := decodeReconciliation(t, getReconciliation(t))
	if len(got.OnlyInMkAuth) != 3 {
		t.Fatalf("expected 3 only_in_mkauth entries, got %d", len(got.OnlyInMkAuth))
	}
	want := []struct{ u, p string }{{"u-1", "p-1"}, {"u-1", "p-2"}, {"u-2", "p-1"}}
	for i, w := range want {
		if got.OnlyInMkAuth[i].UserID != w.u || got.OnlyInMkAuth[i].ProjectID != w.p {
			t.Fatalf("idx %d expected %s/%s, got %s/%s", i, w.u, w.p,
				got.OnlyInMkAuth[i].UserID, got.OnlyInMkAuth[i].ProjectID)
		}
	}
}
