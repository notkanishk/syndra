package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"syndra/internal/auth"
	"syndra/internal/models"
	"syndra/internal/services"
)

// Access over time (A-24 §9): GET /users/{id}/access-history, through the
// real router, as an operator and as members.

func historyRouter(t *testing.T, roles ...string) (http.Handler, *[]string) {
	t.Helper()
	t.Setenv("ZITADEL_DOMAIN", "example.zitadel.cloud")
	t.Setenv("ZITADEL_AUDIENCE", "test-aud")
	t.Setenv("ZITADEL_ADMIN_ROLE_KEY", "admin")
	origValidate, origHistory := jwtValidate, svcAccessHistory
	t.Cleanup(func() { jwtValidate, svcAccessHistory = origValidate, origHistory })
	held := map[string]struct{}{}
	for _, r := range roles {
		held[r] = struct{}{}
	}
	jwtValidate = func(context.Context, string, string, string) (*auth.Principal, error) {
		return &auth.Principal{Subject: "u1", ProjectRoles: held}, nil
	}
	var asked []string
	svcAccessHistory = func(_ context.Context, userID, since string) (models.AccessHistory, error) {
		asked = append(asked, userID+"@"+since)
		if since == "bad" {
			return models.AccessHistory{}, services.ErrInvalidHistorySince
		}
		if since == "broken" {
			return models.AccessHistory{}, errors.New("db down")
		}
		from := time.Date(2026, 8, 14, 0, 0, 0, 0, time.UTC)
		held := models.StandingHeld
		return models.AccessHistory{Since: since, Roles: []models.RoleHistory{{
			ProjectID: "p1", ProjectName: "Laser cutting", ProjectNameResolved: true, RoleKey: "operator",
			Standing: &held, Runs: []models.RoleRun{{From: &from}, {}},
		}}}, nil
	}
	return NewRouter(), &asked
}

func getHistory(h http.Handler, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("Authorization", "Bearer token")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

// A member never reads it: not someone else's, and not their own either (it
// is an operator's Advanced card).
func TestAccessHistory_MembersAreRefused(t *testing.T) {
	router, asked := historyRouter(t, "member")
	for _, path := range []string{"/api/v1/users/u2/access-history?since=2026-01-01", "/api/v1/users/u1/access-history?since=2026-01-01"} {
		if rr := getHistory(router, path); rr.Code != http.StatusForbidden {
			t.Errorf("%s as a member: status %d, want 403", path, rr.Code)
		}
	}
	if len(*asked) != 0 {
		t.Fatalf("the read ran for a member: %v", *asked)
	}
}

// An operator gets the shape the card reads: since, truncated, and per role
// its standing and runs, an unknown start and an open end as null.
func TestAccessHistory_TheShape(t *testing.T) {
	router, asked := historyRouter(t, "admin")
	rr := getHistory(router, "/api/v1/users/u2/access-history?since=2026-01-01")
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rr.Code, rr.Body)
	}
	if len(*asked) != 1 || (*asked)[0] != "u2@2026-01-01" {
		t.Fatalf("asked %v, want the path's person and the since as sent", *asked)
	}
	var got struct {
		Since     *string `json:"since"`
		Truncated *bool   `json:"truncated"`
		Roles     []struct {
			ProjectID   string           `json:"project_id"`
			ProjectName string           `json:"project_name"`
			RoleKey     string           `json:"role_key"`
			Standing    *string          `json:"standing"`
			Runs        []map[string]any `json:"runs"`
		} `json:"roles"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Since == nil || *got.Since != "2026-01-01" || got.Truncated == nil || len(got.Roles) != 1 {
		t.Fatalf("body %s", rr.Body)
	}
	role := got.Roles[0]
	if role.ProjectName != "Laser cutting" || role.RoleKey != "operator" || role.Standing == nil || *role.Standing != "held" || len(role.Runs) != 2 {
		t.Fatalf("role %+v", role)
	}
	for _, key := range []string{"from", "to"} {
		if v, ok := role.Runs[1][key]; !ok || v != nil {
			t.Fatalf("run %v: %q must be present and null when not known", role.Runs[1], key)
		}
	}
	if role.Runs[0]["from"] != "2026-08-14T00:00:00Z" {
		t.Fatalf("from %v", role.Runs[0]["from"])
	}
}

// A since outside a year back to today is the caller's mistake (400, naming
// the field); a failed read is the server's (500).
func TestAccessHistory_Bounds(t *testing.T) {
	router, _ := historyRouter(t, "admin")
	rr := getHistory(router, "/api/v1/users/u2/access-history?since=bad")
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("bad since: status %d", rr.Code)
	}
	var body struct {
		Details map[string]string `json:"details"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &body)
	if body.Details["since"] != "invalid" {
		t.Fatalf("bad since names no field: %s", rr.Body)
	}
	if rr := getHistory(router, "/api/v1/users/u2/access-history?since=broken"); rr.Code != http.StatusInternalServerError {
		t.Fatalf("failed read: status %d", rr.Code)
	}
}
