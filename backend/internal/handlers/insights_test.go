package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"syndra/internal/auth"
	"syndra/internal/models"
	"syndra/internal/services"
)

func stubInsights(t *testing.T) {
	t.Helper()
	f, w := svcInsightsFindings, svcInsightsWho
	t.Cleanup(func() { svcInsightsFindings, svcInsightsWho = f, w })
}

func getWho(query url.Values) *httptest.ResponseRecorder {
	rr := httptest.NewRecorder()
	handleInsightsWho(rr, httptest.NewRequest(http.MethodGet, "/api/v1/insights/who?"+query.Encode(), nil))
	return rr
}

func TestInsightsFindingsHandler_ReturnsTheReport(t *testing.T) {
	stubInsights(t)
	svcInsightsFindings = func(context.Context) (models.InsightsFindings, error) {
		return models.InsightsFindings{
			SingleRoutes: []models.InsightSingleRoute{{ProjectID: "p1", RoleKey: "x", OnlyThatWay: 9, Holders: 10,
				Source: models.InsightSource{Kind: "bundle", ID: "b1", Name: "Alpha"}}},
		}, nil
	}
	rr := httptest.NewRecorder()
	handleInsightsFindings(rr, httptest.NewRequest(http.MethodGet, "/api/v1/insights/findings", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rr.Code, rr.Body.String())
	}
	var body struct {
		SingleRoutes []struct {
			ProjectID   string `json:"project_id"`
			OnlyThatWay int    `json:"only_that_way"`
			Source      struct {
				Kind string `json:"kind"`
			} `json:"source"`
		} `json:"single_routes"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.SingleRoutes) != 1 || body.SingleRoutes[0].OnlyThatWay != 9 || body.SingleRoutes[0].Source.Kind != "bundle" {
		t.Fatalf("body = %s", rr.Body.String())
	}
}

func TestInsightsFindingsHandler_FailureIs500(t *testing.T) {
	stubInsights(t)
	svcInsightsFindings = func(context.Context) (models.InsightsFindings, error) {
		return models.InsightsFindings{}, errors.New("boom")
	}
	rr := httptest.NewRecorder()
	handleInsightsFindings(rr, httptest.NewRequest(http.MethodGet, "/api/v1/insights/findings", nil))
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status %d", rr.Code)
	}
}

func TestInsightsWhoHandler_PassesParsedRefs(t *testing.T) {
	stubInsights(t)
	var gotHas, gotWithout services.InsightRef
	svcInsightsWho = func(_ context.Context, has, without services.InsightRef) (models.InsightsWho, error) {
		gotHas, gotWithout = has, without
		return models.InsightsWho{People: []models.InsightPerson{{UserID: "u1", Name: "Ada", Email: "ada@example.org"}}}, nil
	}
	rr := getWho(url.Values{"has": {"role:p1:x"}, "without": {"bundle:b1"}})
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rr.Code, rr.Body.String())
	}
	if gotHas != (services.InsightRef{Kind: "role", ProjectID: "p1", RoleKey: "x"}) || gotWithout != (services.InsightRef{Kind: "bundle", BundleID: "b1"}) {
		t.Fatalf("refs passed = %+v / %+v", gotHas, gotWithout)
	}

	// `without` is optional and reaches the service as the zero ref.
	if rr = getWho(url.Values{"has": {"bundle:b1"}}); rr.Code != http.StatusOK || gotWithout != (services.InsightRef{}) {
		t.Fatalf("status %d, without = %+v", rr.Code, gotWithout)
	}
}

func TestInsightsWhoHandler_MalformedRefIs400ValidationFailed(t *testing.T) {
	stubInsights(t)
	called := false
	svcInsightsWho = func(context.Context, services.InsightRef, services.InsightRef) (models.InsightsWho, error) {
		called = true
		return models.InsightsWho{}, nil
	}
	for name, q := range map[string]url.Values{
		"missing has":       {},
		"empty has":         {"has": {""}},
		"bad has":           {"has": {"role:p1"}},
		"bad without":       {"has": {"role:p1:x"}, "without": {"user:u1"}},
		"empty without":     {"has": {"role:p1:x"}, "without": {""}},
		"extra segment":     {"has": {"role:p1:x:y"}},
		"unknown kind":      {"has": {"group:g1"}},
		"whitespace bundle": {"has": {"bundle:b 1"}},
	} {
		rr := getWho(q)
		var body ErrorResponse
		_ = json.Unmarshal(rr.Body.Bytes(), &body)
		if rr.Code != http.StatusBadRequest || body.Error != "VALIDATION_FAILED" {
			t.Errorf("%s: status %d body %s, want 400 VALIDATION_FAILED", name, rr.Code, rr.Body.String())
		}
	}
	if called {
		t.Fatal("a malformed ref must never reach the service")
	}
}

func TestInsightsWhoHandler_FailureIs500(t *testing.T) {
	stubInsights(t)
	svcInsightsWho = func(context.Context, services.InsightRef, services.InsightRef) (models.InsightsWho, error) {
		return models.InsightsWho{}, errors.New("boom")
	}
	if rr := getWho(url.Values{"has": {"role:p1:x"}}); rr.Code != http.StatusInternalServerError {
		t.Fatalf("status %d", rr.Code)
	}
}

// Through the REAL router: a member is refused, an operator reaches the
// handler. A downgrade to withUserAuth on either route fails this.
func TestRouter_InsightsAreOperatorOnly(t *testing.T) {
	stubInsights(t)
	svcInsightsFindings = func(context.Context) (models.InsightsFindings, error) { return models.InsightsFindings{}, nil }
	svcInsightsWho = func(context.Context, services.InsightRef, services.InsightRef) (models.InsightsWho, error) {
		return models.InsightsWho{People: []models.InsightPerson{}}, nil
	}

	t.Setenv("ZITADEL_DOMAIN", "example.zitadel.cloud")
	t.Setenv("ZITADEL_AUDIENCE", "test-aud")
	t.Setenv("ZITADEL_ADMIN_ROLE_KEY", "admin")
	origValidate := jwtValidate
	t.Cleanup(func() { jwtValidate = origValidate })
	router := NewRouter()

	paths := []string{"/api/v1/insights/findings", "/api/v1/insights/who?has=role:p1:x"}
	for role, want := range map[string]int{"member": http.StatusForbidden, "admin": http.StatusOK} {
		jwtValidate = func(context.Context, string, string, string) (*auth.Principal, error) {
			return &auth.Principal{Subject: "u", ProjectRoles: map[string]struct{}{role: {}}}, nil
		}
		for _, p := range paths {
			req := httptest.NewRequest(http.MethodGet, p, nil)
			req.Header.Set("Authorization", "Bearer token")
			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)
			if rr.Code != want {
				t.Errorf("%s as %s: status %d, want %d: %s", p, role, rr.Code, want, rr.Body.String())
			}
		}
	}
}
