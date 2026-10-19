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
	"syndra/internal/db"
	"syndra/internal/models"
)

func stubUsage(t *testing.T) {
	t.Helper()
	u, g := svcInsightsUsage, dbGetUserUsage
	t.Cleanup(func() { svcInsightsUsage, dbGetUserUsage = u, g })
}

func getUsage(query string) *httptest.ResponseRecorder {
	rr := httptest.NewRecorder()
	handleInsightsUsage(rr, httptest.NewRequest(http.MethodGet, "/api/v1/insights/usage"+query, nil))
	return rr
}

func TestInsightsUsageHandler_DaysDefaultsAndIsValidated(t *testing.T) {
	stubUsage(t)
	var asked int
	svcInsightsUsage = func(_ context.Context, days int) (models.InsightsUsage, error) {
		asked = days
		return models.InsightsUsage{HeldNotUsed: []models.HeldNotUsed{}, WindowDays: days}, nil
	}
	for q, want := range map[string]int{"": 90, "?days=1": 1, "?days=365": 365, "?days=30": 30} {
		asked = 0
		if rr := getUsage(q); rr.Code != http.StatusOK || asked != want {
			t.Errorf("%q: status %d asked %d, want %d", q, rr.Code, asked, want)
		}
	}
	asked = 0
	for _, q := range []string{"?days=0", "?days=366", "?days=-3", "?days=abc", "?days=1.5", "?days=%207", "?days=99999999999999999999"} {
		if rr := getUsage(q); rr.Code != http.StatusBadRequest || asked != 0 {
			t.Errorf("%q: status %d (asked %d), want 400 and no read", q, rr.Code, asked)
		}
	}
}

func TestInsightsUsageHandler_ShapeAndFailure(t *testing.T) {
	stubUsage(t)
	at := time.Date(2026, 10, 15, 8, 0, 0, 0, time.UTC)
	svcInsightsUsage = func(context.Context, int) (models.InsightsUsage, error) {
		return models.InsightsUsage{
			Sources:     models.UsageSources{SignIn: models.UsageSource{Receiving: true, LastAt: &at}},
			HeldNotUsed: []models.HeldNotUsed{{ProjectID: "p", RoleKey: "r", People: 3}},
			WindowDays:  90,
		}, nil
	}
	rr := getUsage("")
	var body struct {
		Sources struct {
			SignIn map[string]any `json:"sign_in"`
			Badge  map[string]any `json:"badge"`
		} `json:"sources"`
		HeldNotUsed []map[string]any `json:"held_not_used"`
		WindowDays  int              `json:"window_days"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Sources.SignIn["receiving"] != true || body.Sources.SignIn["last_at"] == nil ||
		body.Sources.Badge["receiving"] != false || body.Sources.Badge["last_at"] != nil ||
		len(body.HeldNotUsed) != 1 || body.HeldNotUsed[0]["people"] != float64(3) || body.WindowDays != 90 {
		t.Errorf("body = %s", rr.Body.String())
	}

	svcInsightsUsage = func(context.Context, int) (models.InsightsUsage, error) {
		return models.InsightsUsage{}, errors.New("boom")
	}
	if rr := getUsage(""); rr.Code != http.StatusInternalServerError {
		t.Errorf("status %d", rr.Code)
	}
}

func TestUserUsageHandler(t *testing.T) {
	stubUsage(t)
	at := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	var asked string
	dbGetUserUsage = func(_ context.Context, id string) ([]db.UsageRow, error) {
		asked = id
		return []db.UsageRow{{ProjectID: "p", Kind: "sign_in", LastAt: at}}, nil
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/u7/usage", nil)
	req.SetPathValue("id", "u7")
	rr := httptest.NewRecorder()
	handleGetUserUsage(rr, req)
	var rows []map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &rows); err != nil || rr.Code != 200 || asked != "u7" ||
		len(rows) != 1 || rows[0]["project_id"] != "p" || rows[0]["kind"] != "sign_in" || rows[0]["last_at"] == nil {
		t.Fatalf("status %d asked %q body %s", rr.Code, asked, rr.Body.String())
	}

	dbGetUserUsage = func(context.Context, string) ([]db.UsageRow, error) { return nil, errors.New("boom") }
	rr = httptest.NewRecorder()
	handleGetUserUsage(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("status %d", rr.Code)
	}
}

// Through the REAL router: a member is refused on both routes, an operator
// reaches the handler.
func TestRouter_UsageRoutesAreOperatorOnly(t *testing.T) {
	stubUsage(t)
	svcInsightsUsage = func(context.Context, int) (models.InsightsUsage, error) {
		return models.InsightsUsage{HeldNotUsed: []models.HeldNotUsed{}}, nil
	}
	dbGetUserUsage = func(context.Context, string) ([]db.UsageRow, error) { return []db.UsageRow{}, nil }

	t.Setenv("ZITADEL_DOMAIN", "example.zitadel.cloud")
	t.Setenv("ZITADEL_AUDIENCE", "test-aud")
	t.Setenv("ZITADEL_ADMIN_ROLE_KEY", "admin")
	origValidate := jwtValidate
	t.Cleanup(func() { jwtValidate = origValidate })
	router := NewRouter()

	for role, want := range map[string]int{"member": http.StatusForbidden, "admin": http.StatusOK} {
		jwtValidate = func(context.Context, string, string, string) (*auth.Principal, error) {
			return &auth.Principal{Subject: "u", ProjectRoles: map[string]struct{}{role: {}}}, nil
		}
		for _, p := range []string{"/api/v1/insights/usage", "/api/v1/users/u1/usage"} {
			req := httptest.NewRequest(http.MethodGet, p, nil)
			req.Header.Set("Authorization", "Bearer token")
			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)
			if rr.Code != want {
				t.Errorf("%s as %s: status %d, want %d", p, role, rr.Code, want)
			}
		}
	}
	// And no token at all.
	for _, p := range []string{"/api/v1/insights/usage", "/api/v1/users/u1/usage"} {
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, p, nil))
		if rr.Code != http.StatusUnauthorized && rr.Code != http.StatusForbidden {
			t.Errorf("%s without a token: status %d", p, rr.Code)
		}
	}
}
