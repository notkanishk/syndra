package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"syndra/internal/db"
)

// resetObserveGrantDeps saves/restores the injectables the two observed-grants
// surfaces (handleListAllZitadelGrants, handleListZitadelUserGrants) use.
func resetObserveGrantDeps(t *testing.T) {
	t.Helper()
	origOrg := observeOrg
	origUser := observeUser
	origPage := dbObservedGrantsPage
	origFor := dbObservedGrantsFor
	t.Cleanup(func() {
		observeOrg = origOrg
		observeUser = origUser
		dbObservedGrantsPage = origPage
		dbObservedGrantsFor = origFor
	})
}

// TestHandleListAllZitadelGrants_ObservesThenAnswersFromStore is
// one-truth-many-checks/3.3: the org-wide grants surface must observe (which
// records) rather than list Zitadel directly, and its response must be built
// from what the store now holds — not from whatever page the live call
// itself happened to return.
func TestHandleListAllZitadelGrants_ObservesThenAnswersFromStore(t *testing.T) {
	resetObserveGrantDeps(t)

	var observed bool
	at := time.Date(2026, 9, 11, 3, 0, 0, 0, time.UTC)
	observeOrg = func(context.Context) (db.Observation, error) {
		observed = true
		return db.Observation{Scope: "org", ObservedAt: at, Complete: true, GrantsSeen: 2}, nil
	}
	dbObservedGrantsPage = func(_ context.Context, limit, offset int) ([]db.ObservedGrant, int, error) {
		if limit != 500 || offset != 0 {
			t.Errorf("expected the default page params to reach the store read, got limit=%d offset=%d", limit, offset)
		}
		return []db.ObservedGrant{
			{GrantID: "g1", UserID: "u1", ProjectID: "p1", RoleKeys: []string{"member"}},
		}, 1, nil
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/zitadel/grants", nil)
	w := httptest.NewRecorder()
	handleListAllZitadelGrants(w, req)

	if !observed {
		t.Fatal("expected observeOrg to be called")
	}
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	var body struct {
		Items      []map[string]any `json:"items"`
		Total      int              `json:"total"`
		ObservedAt *time.Time       `json:"observed_at"`
		Complete   *bool            `json:"complete"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Total != 1 || len(body.Items) != 1 || body.Items[0]["id"] != "g1" {
		t.Fatalf("unexpected items from the STORE read, not the live call: %+v", body)
	}
	if body.ObservedAt == nil || !body.ObservedAt.Equal(at) {
		t.Fatalf("expected observed_at %v, got %v", at, body.ObservedAt)
	}
	if body.Complete == nil || !*body.Complete {
		t.Fatalf("expected complete=true, got %v", body.Complete)
	}
}

// A DB failure recording the observation is the one case that still 500s —
// the answer would otherwise be silently unrecorded.
func TestHandleListAllZitadelGrants_ObserveDBFailure_Returns500(t *testing.T) {
	resetObserveGrantDeps(t)
	observeOrg = func(context.Context) (db.Observation, error) {
		return db.Observation{}, errors.New("write failed")
	}
	dbObservedGrantsPage = func(context.Context, int, int) ([]db.ObservedGrant, int, error) {
		t.Fatal("must not read the store when the observation failed to record")
		return nil, 0, nil
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/zitadel/grants", nil)
	w := httptest.NewRecorder()
	handleListAllZitadelGrants(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("want 500, got %d: %s", w.Code, w.Body.String())
	}
}

// A live Zitadel failure is not a handler error: observeOrg still returns
// (nil error), just with Complete=false, and the surface answers from
// whatever the store already holds — degraded and SAYING SO, not a 502.
func TestHandleListAllZitadelGrants_LiveReadFailed_StillServesStoreWithComplete(t *testing.T) {
	resetObserveGrantDeps(t)
	observeOrg = func(context.Context) (db.Observation, error) {
		return db.Observation{Scope: "org", ObservedAt: time.Now().UTC(), Complete: false, Error: "connection refused"}, nil
	}
	dbObservedGrantsPage = func(context.Context, int, int) ([]db.ObservedGrant, int, error) {
		return nil, 0, nil
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/zitadel/grants", nil)
	w := httptest.NewRecorder()
	handleListAllZitadelGrants(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200 (degraded, not failed), got %d: %s", w.Code, w.Body.String())
	}
	var body struct {
		Complete *bool `json:"complete"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body.Complete == nil || *body.Complete {
		t.Fatalf("expected complete=false to reach the caller, got %v", body.Complete)
	}
}

func TestHandleListZitadelUserGrants_ObservesThenAnswersFromStore(t *testing.T) {
	resetObserveGrantDeps(t)

	var observedUser string
	observeUser = func(_ context.Context, userID string) (db.Observation, error) {
		observedUser = userID
		return db.Observation{Scope: "user", SubjectID: userID, ObservedAt: time.Now().UTC(), Complete: true}, nil
	}
	dbObservedGrantsFor = func(_ context.Context, userID string) ([]db.ObservedGrant, error) {
		return []db.ObservedGrant{
			{GrantID: "g1", UserID: userID, ProjectID: "p1", RoleKeys: []string{"member"}},
			{GrantID: "g2", UserID: userID, ProjectID: "p2", RoleKeys: []string{"lead"}},
		}, nil
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/zitadel/users/u1/grants", nil)
	req.SetPathValue("id", "u1")
	w := httptest.NewRecorder()
	handleListZitadelUserGrants(w, req)

	if observedUser != "u1" {
		t.Fatalf("expected observeUser called for u1, got %q", observedUser)
	}
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	var body struct {
		Items []map[string]any `json:"items"`
		Total int              `json:"total"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body.Total != 2 || len(body.Items) != 2 {
		t.Fatalf("unexpected items: %+v", body)
	}
}

func TestHandleListZitadelUserGrants_MissingUserID_Returns400(t *testing.T) {
	resetObserveGrantDeps(t)
	observeUser = func(context.Context, string) (db.Observation, error) {
		t.Fatal("must not observe without a user id")
		return db.Observation{}, nil
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/zitadel/users//grants", nil)
	w := httptest.NewRecorder()
	handleListZitadelUserGrants(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d: %s", w.Code, w.Body.String())
	}
}

// paginateObservedGrants is the in-memory slice a person's full observed set
// is cut down with; exercised directly since the pagination edge cases (past
// the end, a non-positive limit) are easy to get off-by-one on.
func TestPaginateObservedGrants(t *testing.T) {
	all := []db.ObservedGrant{{GrantID: "a"}, {GrantID: "b"}, {GrantID: "c"}}

	if page, total := paginateObservedGrants(all, 2, 0); total != 3 || len(page) != 2 || page[0].GrantID != "a" {
		t.Fatalf("first page: got %+v total=%d", page, total)
	}
	if page, total := paginateObservedGrants(all, 2, 2); total != 3 || len(page) != 1 || page[0].GrantID != "c" {
		t.Fatalf("second page: got %+v total=%d", page, total)
	}
	if page, total := paginateObservedGrants(all, 10, 10); total != 3 || page != nil {
		t.Fatalf("offset past the end: got %+v total=%d", page, total)
	}
	if page, total := paginateObservedGrants(all, 0, 0); total != 3 || len(page) != 3 {
		t.Fatalf("non-positive limit falls back to everything: got %+v total=%d", page, total)
	}
}

// observationFields must never render "never observed" as the Unix epoch.
func TestObservationFields_ZeroObservedAt_OmitsBoth(t *testing.T) {
	at, complete := observationFields(db.Observation{})
	if at != nil || complete != nil {
		t.Fatalf("expected both nil for a zero observation, got at=%v complete=%v", at, complete)
	}
}

// A member reads what Zitadel holds for THEM through the same observer pipe
// the operator page uses; another person's grants stay operator-only. Routed
// through the real mux so the guard on the route, not just the wrapper, is
// what is tested.
func TestZitadelUserGrantsRoute_SelfReadableByMember(t *testing.T) {
	resetObserveGrantDeps(t)
	stubMemberAuth(t, "u1")
	observeUser = func(_ context.Context, userID string) (db.Observation, error) {
		return db.Observation{Scope: "user", SubjectID: userID, ObservedAt: time.Now().UTC(), Complete: true}, nil
	}
	dbObservedGrantsFor = func(_ context.Context, userID string) ([]db.ObservedGrant, error) {
		return []db.ObservedGrant{{GrantID: "g1", UserID: userID, ProjectID: "p1", RoleKeys: []string{"member"}}}, nil
	}
	mux := NewRouter()

	self := httptest.NewRequest(http.MethodGet, "/api/v1/zitadel/users/u1/grants", nil)
	self.Header.Set("Authorization", "Bearer t")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, self)
	if w.Code != http.StatusOK {
		t.Fatalf("member reading own observed grants: want 200, got %d: %s", w.Code, w.Body.String())
	}

	other := httptest.NewRequest(http.MethodGet, "/api/v1/zitadel/users/u2/grants", nil)
	other.Header.Set("Authorization", "Bearer t")
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, other)
	if w.Code != http.StatusForbidden {
		t.Fatalf("member reading another person's grants: want 403, got %d", w.Code)
	}
}
