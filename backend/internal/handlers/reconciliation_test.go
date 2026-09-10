package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"strings"
	"syndra/internal/db"
	"syndra/internal/models"
	"syndra/internal/zitadel"
)

// testReconciliationObservedAt is the fixed observation timestamp every test
// that does not care about its exact value can share.
var testReconciliationObservedAt = time.Unix(1_766_000_000, 0).UTC()

// withReconciliationDeps swaps in deterministic stub data for the injection
// points reconciliation depends on, and restores the originals when the test
// returns. Keeps tests isolated from the real DB and Zitadel.
//
// one-truth-many-checks, "The last two readers": the handler no longer pages
// Zitadel itself — it observes (recording what it saw) and then reads back
// the store, exactly as discovery.go's grant-listing routes do. `complete`
// and `obsErr` stand in for what a real observe.Org would have recorded.
func withReconciliationDeps(
	t *testing.T,
	syndra []models.DirectGrant,
	zitadelGrants []zitadel.UserGrant,
	complete bool,
	obsErr string,
) {
	t.Helper()

	origAll := svcAllDirectGrants
	origObserveOrg := observeOrg
	origAllObserved := dbAllObservedGrants
	origRules := svcGetActiveMappingRulesRecon
	origExclusions := svcGetExclusions
	origBundled := svcAllBundleDerivedGrantsRecon

	svcAllDirectGrants = func(_ context.Context) ([]models.DirectGrant, error) {
		return syndra, nil
	}
	// Default to no rules/exclusions so existing tests (which don't care about
	// expected-set filtering) see the pre-B2 diff shape unchanged.
	svcGetActiveMappingRulesRecon = func(_ context.Context) ([]models.MappingRule, error) {
		return nil, nil
	}
	svcGetExclusions = func(_ context.Context, _ string) ([]models.ExternalGrantExclusion, error) {
		return nil, nil
	}
	// And no bundles, so a test that says nothing about them keeps describing
	// the world it was written about.
	svcAllBundleDerivedGrantsRecon = func(_ context.Context) ([]db.BundleDerivedGrant, error) {
		return nil, nil
	}
	observeOrg = func(context.Context) (db.Observation, error) {
		return db.Observation{Scope: "org", ObservedAt: testReconciliationObservedAt, Complete: complete, Error: obsErr}, nil
	}
	dbAllObservedGrants = func(context.Context) ([]db.ObservedGrant, error) {
		out := make([]db.ObservedGrant, len(zitadelGrants))
		for i, g := range zitadelGrants {
			out[i] = db.ObservedGrant{GrantID: g.ID, UserID: g.UserID, ProjectID: g.ProjectID, RoleKeys: g.RoleKeys}
		}
		return out, nil
	}

	t.Cleanup(func() {
		svcAllDirectGrants = origAll
		observeOrg = origObserveOrg
		dbAllObservedGrants = origAllObserved
		svcGetActiveMappingRulesRecon = origRules
		svcGetExclusions = origExclusions
		svcAllBundleDerivedGrantsRecon = origBundled
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

// TestReconciliation_OnlyInSyndra: a (user, project) pair exists on the
// Syndra side but Zitadel has no grant for it. Should land in only_in_syndra
// with all roles aggregated, no drift, no only_in_zitadel entry.
func TestReconciliation_OnlyInSyndra(t *testing.T) {
	withReconciliationDeps(t,
		[]models.DirectGrant{
			directGrant("u-1", "p-1", "viewer"),
			directGrant("u-1", "p-1", "editor"),
		},
		nil, true, "",
	)

	rr := getReconciliation(t)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}
	got := decodeReconciliation(t, rr)

	if len(got.OnlyInSyndra) != 1 || len(got.OnlyInZitadel) != 0 || len(got.Drift) != 0 {
		t.Fatalf("expected 1 only-in-syndra and nothing else, got %+v", got)
	}
	entry := got.OnlyInSyndra[0]
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

// TestReconciliation_OnlyInZitadel: a Zitadel grant has no Syndra counterpart.
// Most often a derived (mapping rule) grant or pre-existing manual grant.
func TestReconciliation_OnlyInZitadel(t *testing.T) {
	withReconciliationDeps(t,
		nil,
		[]zitadel.UserGrant{
			{ID: "g-7", UserID: "u-1", ProjectID: "p-1", RoleKeys: []string{"derived_role"}},
		}, true, "",
	)

	rr := getReconciliation(t)
	got := decodeReconciliation(t, rr)

	if len(got.OnlyInZitadel) != 1 || len(got.OnlyInSyndra) != 0 || len(got.Drift) != 0 {
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
		}, true, "",
	)

	got := decodeReconciliation(t, getReconciliation(t))

	if len(got.OnlyInSyndra) != 0 || len(got.OnlyInZitadel) != 0 {
		t.Fatalf("expected drift only, got %+v", got)
	}
	if len(got.Drift) != 1 {
		t.Fatalf("expected one drift entry, got %d", len(got.Drift))
	}
	d := got.Drift[0]
	if !reflect.DeepEqual(d.OnlyInSyndra, []string{"editor"}) {
		t.Fatalf("expected only-in-syndra=[editor], got %+v", d.OnlyInSyndra)
	}
	if !reflect.DeepEqual(d.OnlyInZitadel, []string{"admin"}) {
		t.Fatalf("expected only-in-zitadel=[admin], got %+v", d.OnlyInZitadel)
	}
	if d.GrantID != "g-1" {
		t.Fatalf("expected GrantID=g-1 for drill-in, got %q", d.GrantID)
	}
	if !reflect.DeepEqual(d.SyndraRoles, []string{"editor", "viewer"}) {
		t.Fatalf("SyndraRoles not sorted asc, got %+v", d.SyndraRoles)
	}
	if !reflect.DeepEqual(d.ZitadelRoles, []string{"admin", "viewer"}) {
		t.Fatalf("ZitadelRoles not sorted asc, got %+v", d.ZitadelRoles)
	}
}

// TestReconciliation_RoleSuperset: Syndra has a role that Zitadel is missing.
// Reported as drift (not only_in_syndra), because the (user, project) pair
// itself is present on both sides — only the role set differs.
func TestReconciliation_RoleSuperset(t *testing.T) {
	withReconciliationDeps(t,
		[]models.DirectGrant{
			directGrant("u-1", "p-1", "viewer"),
			directGrant("u-1", "p-1", "editor"),
		},
		[]zitadel.UserGrant{
			{ID: "g-1", UserID: "u-1", ProjectID: "p-1", RoleKeys: []string{"viewer"}},
		}, true, "",
	)

	got := decodeReconciliation(t, getReconciliation(t))
	if len(got.Drift) != 1 || len(got.OnlyInSyndra) != 0 {
		t.Fatalf("expected drift-only superset, got %+v", got)
	}
	d := got.Drift[0]
	if !reflect.DeepEqual(d.OnlyInSyndra, []string{"editor"}) || len(d.OnlyInZitadel) != 0 {
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
		}, true, "",
	)
	got := decodeReconciliation(t, getReconciliation(t))
	if len(got.OnlyInSyndra) != 0 || len(got.OnlyInZitadel) != 0 || len(got.Drift) != 0 {
		t.Fatalf("expected all-empty diff, got %+v", got)
	}
	if got.GeneratedAt.IsZero() {
		t.Fatalf("expected generated_at to be populated")
	}
}

// TestReconciliation_IncompleteObservationStillDiffsWhatWasSeen: an
// observation that did not finish still has real grants in it, and the diff
// must still run over them — only the Truncated flag says the picture might
// be missing something. Concluding nothing at all from a partial read would
// throw away findings that are perfectly real.
func TestReconciliation_IncompleteObservationStillDiffsWhatWasSeen(t *testing.T) {
	withReconciliationDeps(t,
		[]models.DirectGrant{directGrant("u-1", "p-1", "viewer")},
		[]zitadel.UserGrant{
			{ID: "g-1", UserID: "u-1", ProjectID: "p-1", RoleKeys: []string{"viewer"}},
			{ID: "g-2", UserID: "u-2", ProjectID: "p-1", RoleKeys: []string{"derived_role"}},
		}, false, "the read reached its safety limit",
	)

	got := decodeReconciliation(t, getReconciliation(t))
	if !got.Truncated {
		t.Fatal("an incomplete observation must report truncated=true")
	}
	if len(got.OnlyInZitadel) != 1 || got.OnlyInZitadel[0].UserID != "u-2" {
		t.Fatalf("grants actually seen must still be diffed, got %+v", got.OnlyInZitadel)
	}
}

// TestReconciliation_ZitadelFailure: the handler must surface a 502 when the
// observation came back with nothing at all — an ordinary "all aligned"
// snapshot from the Syndra side alone would tell an operator everybody in
// Zitadel lost their access, which is the false negative this design exists
// to prevent.
func TestReconciliation_ZitadelFailure(t *testing.T) {
	withReconciliationDeps(t, nil, nil, false, "upstream unavailable")

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
		nil, true, "",
	)

	got := decodeReconciliation(t, getReconciliation(t))
	if len(got.OnlyInSyndra) != 3 {
		t.Fatalf("expected 3 only_in_syndra entries, got %d", len(got.OnlyInSyndra))
	}
	want := []struct{ u, p string }{{"u-1", "p-1"}, {"u-1", "p-2"}, {"u-2", "p-1"}}
	for i, w := range want {
		if got.OnlyInSyndra[i].UserID != w.u || got.OnlyInSyndra[i].ProjectID != w.p {
			t.Fatalf("idx %d expected %s/%s, got %s/%s", i, w.u, w.p,
				got.OnlyInSyndra[i].UserID, got.OnlyInSyndra[i].ProjectID)
		}
	}
}

// TestReconciliation_RuleDerivedNotOnlyInZitadel: a Zitadel grant that is the
// target of an active mapping rule the user qualifies for (holds the source)
// is expected, not drift — it must not appear in only_in_zitadel.
func TestReconciliation_RuleDerivedNotOnlyInZitadel(t *testing.T) {
	// Syndra has the source grant; Zitadel has source + rule-derived target.
	withReconciliationDeps(t,
		[]models.DirectGrant{directGrant("u-1", "p1", "member")},
		[]zitadel.UserGrant{
			{ID: "g1", UserID: "u-1", ProjectID: "p1", RoleKeys: []string{"member"}},
			{ID: "g2", UserID: "u-1", ProjectID: "p2", RoleKeys: []string{"contributor"}},
		}, true, "",
	)
	// Active rule: p1:member → p2:contributor.
	origRules := svcGetActiveMappingRulesRecon
	svcGetActiveMappingRulesRecon = func(context.Context) ([]models.MappingRule, error) {
		return []models.MappingRule{{SourceProject: "p1", SourceRole: "member", TargetProject: "p2", TargetRole: "contributor"}}, nil
	}
	t.Cleanup(func() { svcGetActiveMappingRulesRecon = origRules })

	got := decodeReconciliation(t, getReconciliation(t))
	for _, e := range got.OnlyInZitadel {
		if e.ProjectID == "p2" {
			t.Fatalf("rule-derived p2:contributor must NOT be OnlyInZitadel: %+v", got.OnlyInZitadel)
		}
	}
}

// A remembered world is not a comparison made just now.
//
// `observe.Org` answers a zero Observation in local-policy-only mode — there is
// no client to ask — and the store may still hold rows from when there was one.
// Diffing against those put a stale set on screen under a `GeneratedAt` of the
// Unix epoch: a comparison nobody made, at a time that never happened.
func TestReconciliationRefusesWhenThereIsNothingToAsk(t *testing.T) {
	withReconciliationDeps(t, nil, nil, true, "")
	// No client, so nothing was observed by this request...
	observeOrg = func(context.Context) (db.Observation, error) {
		return db.Observation{}, nil
	}
	// ...while the store still remembers a world from when there was one.
	dbAllObservedGrants = func(context.Context) ([]db.ObservedGrant, error) {
		return []db.ObservedGrant{{
			GrantID: "g1", UserID: "u1", ProjectID: "p1", RoleKeys: []string{"member"},
		}}, nil
	}

	rr := httptest.NewRecorder()
	handleGetReconciliationDiff(rr, httptest.NewRequest(http.MethodGet, "/api/v1/reconciliation/diff", nil))

	if rr.Code != http.StatusBadGateway {
		t.Fatalf("a diff was served from a remembered store: status %d, body %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "nothing to compare against") {
		t.Fatalf("the refusal does not say why: %s", rr.Body.String())
	}
}
