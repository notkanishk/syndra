package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"syndra/internal/db"
)

// Paging through the org's grants is one read, not one per page: only the first
// page observes, and the later ones state the freshness of what it recorded.
func TestListAllZitadelGrants_OnlyTheFirstPageObserves(t *testing.T) {
	resetObserveGrantDeps(t)
	at := time.Date(2026, 9, 11, 3, 0, 0, 0, time.UTC)
	observes := 0
	observeOrg = func(context.Context) (db.Observation, error) {
		observes++
		return db.Observation{Scope: "org", ObservedAt: at, Complete: true}, nil
	}
	dbObservedGrantsPage = func(context.Context, int, int) ([]db.ObservedGrant, int, error) {
		return []db.ObservedGrant{{GrantID: "g1", UserID: "u1", ProjectID: "p1"}}, 1001, nil
	}
	dbObservationPage = func(context.Context, int, int) (db.Observation, []db.ObservedGrant, int, error) {
		return db.Observation{Scope: "org", ObservedAt: at, Complete: true},
			[]db.ObservedGrant{{GrantID: "g1", UserID: "u1", ProjectID: "p1"}}, 1001, nil
	}

	for _, url := range []string{"/api/v1/zitadel/grants?limit=500", "/api/v1/zitadel/grants?limit=500&offset=500"} {
		w := httptest.NewRecorder()
		handleListAllZitadelGrants(w, httptest.NewRequest(http.MethodGet, url, nil))
		var body struct {
			ObservedAt *time.Time `json:"observed_at"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &body)
		if w.Code != http.StatusOK || body.ObservedAt == nil || !body.ObservedAt.Equal(at) {
			t.Fatalf("%s: status %d, observed_at %v", url, w.Code, body.ObservedAt)
		}
	}
	if observes != 1 {
		t.Fatalf("two pages made %d org observations, want 1", observes)
	}
}

// A later page of a store nobody has observed yet is still an answer, not a 500.
func TestListAllZitadelGrants_LaterPageBeforeAnyObservation(t *testing.T) {
	resetObserveGrantDeps(t)
	observeOrg = func(context.Context) (db.Observation, error) {
		t.Fatal("a later page must not observe")
		return db.Observation{}, nil
	}
	dbObservationPage = func(context.Context, int, int) (db.Observation, []db.ObservedGrant, int, error) {
		return db.Observation{}, nil, 0, db.ErrNoObservation
	}

	w := httptest.NewRecorder()
	handleListAllZitadelGrants(w, httptest.NewRequest(http.MethodGet, "/api/v1/zitadel/grants?offset=500", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
}

// A later page takes its header, total and rows from the one snapshot: the
// page handler must not stitch them from separate reads a sweep can split.
func TestListAllZitadelGrants_LaterPageUsesOneSnapshot(t *testing.T) {
	resetObserveGrantDeps(t)
	at := time.Date(2026, 9, 11, 3, 0, 0, 0, time.UTC)
	observeOrg = func(context.Context) (db.Observation, error) {
		t.Fatal("a later page must not observe")
		return db.Observation{}, nil
	}
	dbLatestOrgObservation = func(context.Context) (db.Observation, error) {
		t.Fatal("the header must come from the snapshot, not a separate read")
		return db.Observation{}, nil
	}
	dbObservedGrantsPage = func(context.Context, int, int) ([]db.ObservedGrant, int, error) {
		t.Fatal("the rows must come from the snapshot, not a separate read")
		return nil, 0, nil
	}
	dbObservationPage = func(context.Context, int, int) (db.Observation, []db.ObservedGrant, int, error) {
		return db.Observation{ObservedAt: at, Complete: true}, []db.ObservedGrant{{GrantID: "g9"}}, 600, nil
	}

	w := httptest.NewRecorder()
	handleListAllZitadelGrants(w, httptest.NewRequest(http.MethodGet, "/api/v1/zitadel/grants?offset=500", nil))
	var body struct {
		Total      int        `json:"total"`
		ObservedAt *time.Time `json:"observed_at"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if w.Code != http.StatusOK || body.Total != 600 || body.ObservedAt == nil || !body.ObservedAt.Equal(at) {
		t.Fatalf("status %d total %d observed_at %v", w.Code, body.Total, body.ObservedAt)
	}
}

// The reconciliation diff reuses the sweep's observation while it is recent,
// and observes for itself when it is old, absent, or the operator asked for now.
func TestReconciliation_ObservesOnlyWhenTheStoreIsOldOrAskedFresh(t *testing.T) {
	recent := func(context.Context) (db.Observation, error) {
		return db.Observation{ObservedAt: time.Now().Add(-time.Minute), Complete: true}, nil
	}
	cases := []struct {
		name    string
		url     string
		latest  func(context.Context) (db.Observation, error)
		observe bool
	}{
		{"recent store is reused", "/api/v1/reconciliation/grants", recent, false},
		{"old store is re-observed", "/api/v1/reconciliation/grants",
			func(context.Context) (db.Observation, error) {
				return db.Observation{ObservedAt: time.Now().Add(-time.Hour), Complete: true}, nil
			}, true},
		{"never observed is observed", "/api/v1/reconciliation/grants",
			func(context.Context) (db.Observation, error) { return db.Observation{}, db.ErrNoObservation }, true},
		{"a recent but incomplete store is re-observed", "/api/v1/reconciliation/grants",
			func(context.Context) (db.Observation, error) {
				return db.Observation{ObservedAt: time.Now().Add(-time.Minute), Complete: false}, nil
			}, true},
		{"a recent store whose read failed is re-observed", "/api/v1/reconciliation/grants",
			func(context.Context) (db.Observation, error) {
				return db.Observation{ObservedAt: time.Now().Add(-time.Minute), Complete: true, Error: "listing failed"}, nil
			}, true},
		{"fresh=1 forces a read over a recent store", "/api/v1/reconciliation/grants?fresh=1", recent, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			withReconciliationDeps(t, nil, nil, true, "")
			dbLatestOrgObservation = c.latest
			observed := false
			inner := observeOrg
			observeOrg = func(ctx context.Context) (db.Observation, error) {
				observed = true
				return inner(ctx)
			}
			rr := httptest.NewRecorder()
			handleGetReconciliationDiff(rr, httptest.NewRequest(http.MethodGet, c.url, nil))
			if rr.Code != http.StatusOK {
				t.Fatalf("status %d: %s", rr.Code, rr.Body.String())
			}
			if observed != c.observe {
				t.Fatalf("observed=%v, want %v", observed, c.observe)
			}
		})
	}
}
