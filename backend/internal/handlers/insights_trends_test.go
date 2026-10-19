package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"syndra/internal/models"
	"syndra/internal/services"
)

func getTrends(query string) *httptest.ResponseRecorder {
	rr := httptest.NewRecorder()
	handleInsightsTrends(rr, httptest.NewRequest(http.MethodGet, "/api/v1/insights/trends"+query, nil))
	return rr
}

func TestInsightsTrendsHandler_PeriodDefaultsToTermAndPassesThrough(t *testing.T) {
	stubInsights(t)
	for query, want := range map[string]services.TrendPeriod{
		"":             services.TrendTerm,
		"?period=term": services.TrendTerm,
		"?period=30d":  services.Trend30Days,
		"?period=year": services.TrendYear,
	} {
		var got services.TrendPeriod
		svcInsightsTrends = func(_ context.Context, p services.TrendPeriod) (models.InsightsTrends, error) {
			got = p
			return models.InsightsTrends{Window: models.TrendWindow{Label: "x"}, Weekly: []models.TrendWeek{}}, nil
		}
		rr := getTrends(query)
		if rr.Code != http.StatusOK || got != want {
			t.Errorf("%q: status %d period %q, want 200 %q", query, rr.Code, got, want)
		}
		var body struct {
			Window map[string]string `json:"window"`
			Weekly []any             `json:"weekly"`
		}
		if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil || body.Window["label"] != "x" || body.Weekly == nil {
			t.Errorf("%q: body %s (%v)", query, rr.Body.String(), err)
		}
	}
}

func TestInsightsTrendsHandler_AnythingElseIs400AndNeverReachesTheService(t *testing.T) {
	stubInsights(t)
	called := false
	svcInsightsTrends = func(context.Context, services.TrendPeriod) (models.InsightsTrends, error) {
		called = true
		return models.InsightsTrends{}, nil
	}
	for _, q := range []string{"?period=week", "?period=30D", "?period=%20term", "?period=all"} {
		rr := getTrends(q)
		var body ErrorResponse
		_ = json.Unmarshal(rr.Body.Bytes(), &body)
		if rr.Code != http.StatusBadRequest || body.Error != "VALIDATION_FAILED" {
			t.Errorf("%q: status %d body %s, want 400 VALIDATION_FAILED", q, rr.Code, rr.Body.String())
		}
	}
	if called {
		t.Fatal("an invalid period must never reach the service")
	}
}

func TestInsightsTrendsHandler_FailureIs500(t *testing.T) {
	stubInsights(t)
	svcInsightsTrends = func(context.Context, services.TrendPeriod) (models.InsightsTrends, error) {
		return models.InsightsTrends{}, errors.New("boom")
	}
	if rr := getTrends("?period=30d"); rr.Code != http.StatusInternalServerError {
		t.Fatalf("status %d", rr.Code)
	}
}
