package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"syndra/internal/db"
	"syndra/internal/zitadel"
)

// The reconciliation review must not call a bundle-derived grant unexplained.
//
// This is the THIRD detector of "does Syndra account for this grant?", and the
// second written without bundles in it. Here the consequence is sharper than in
// the sweep: `OnlyInZitadel` is computed by diffing against Syndra's DIRECT
// grants, so a bundle-projected role does not merely go unexplained — it lands
// under "In Zitadel but not in Syndra" in danger tone, on the screen an
// operator uses to decide what to adopt. Adopting it writes the redundant
// direct grant that stops bundle removal from revoking anything.

func stubBundleInventory(t *testing.T, grants []db.BundleDerivedGrant, err error) {
	t.Helper()
	orig := svcAllBundleDerivedGrantsRecon
	t.Cleanup(func() { svcAllBundleDerivedGrantsRecon = orig })
	svcAllBundleDerivedGrantsRecon = func(context.Context) ([]db.BundleDerivedGrant, error) {
		return grants, err
	}
}

func reconcile(t *testing.T) (*httptest.ResponseRecorder, ReconciliationDiff) {
	t.Helper()
	rr := httptest.NewRecorder()
	handleGetReconciliationDiff(rr, httptest.NewRequest(http.MethodGet, "/api/v1/reconciliation/grants", nil))

	var out ReconciliationDiff
	if rr.Code == http.StatusOK {
		if err := json.NewDecoder(rr.Body).Decode(&out); err != nil {
			t.Fatalf("decode: %v", err)
		}
	}
	return rr, out
}

func TestReconciliation_ABundleRoleIsNotUnexplained(t *testing.T) {
	withReconciliationDeps(t, nil, []zitadel.UserGrant{
		{ID: "g1", UserID: "shikha", ProjectID: "p-admin", RoleKeys: []string{"admin-staff"}},
	}, true, "")
	stubBundleInventory(t, []db.BundleDerivedGrant{
		{UserID: "shikha", ProjectID: "p-admin", RoleKey: "admin-staff"},
	}, nil)

	rr, out := reconcile(t)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if len(out.OnlyInZitadel) != 0 {
		t.Fatalf("a grant the bundle accounts for was reported as unexplained: %+v", out.OnlyInZitadel)
	}
}

// And one nothing accounts for is still reported, so the filter cannot pass by
// dropping everything.
func TestReconciliation_AnUnexplainedGrantIsStillReported(t *testing.T) {
	withReconciliationDeps(t, nil, []zitadel.UserGrant{
		{ID: "g1", UserID: "someone", ProjectID: "p-admin", RoleKeys: []string{"admin-staff"}},
	}, true, "")
	stubBundleInventory(t, nil, nil)

	_, out := reconcile(t)
	if len(out.OnlyInZitadel) != 1 {
		t.Fatalf("an unexplained grant must be reported, got %+v", out.OnlyInZitadel)
	}
}

// A failed bundle read fails the request rather than becoming an empty set —
// the same rule the rules and exclusions reads already follow, for the same
// reason: an empty set explains nothing, so every bundle role in the deployment
// would be painted as drift on the adoption screen.
func TestReconciliation_AFailedBundleReadFailsTheRequest(t *testing.T) {
	withReconciliationDeps(t, nil, []zitadel.UserGrant{
		{ID: "g1", UserID: "shikha", ProjectID: "p-admin", RoleKeys: []string{"admin-staff"}},
	}, true, "")
	stubBundleInventory(t, nil, context.DeadlineExceeded)

	rr, _ := reconcile(t)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected the request to fail, got %d", rr.Code)
	}
}
