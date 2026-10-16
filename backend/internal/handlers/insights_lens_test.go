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

func stubLens(t *testing.T) (days *int, group *string) {
	t.Helper()
	e, g := svcInsightsLensEnding, svcInsightsLensGroup
	t.Cleanup(func() { svcInsightsLensEnding, svcInsightsLensGroup = e, g })
	days, group = new(int), new(string)
	svcInsightsLensEnding = func(_ context.Context, d int) (models.InsightsLensEnding, error) {
		*days = d
		return models.InsightsLensEnding{Kind: "ending", WindowDays: d, Roles: []models.LensRole{}}, nil
	}
	svcInsightsLensGroup = func(_ context.Context, lg services.LensGroup) (models.InsightsLensGroup, error) {
		*group = lg.String()
		return models.InsightsLensGroup{Kind: "group", Group: lg.String(), Roles: []models.LensRole{}}, nil
	}
	return days, group
}

func getLens(query string) *httptest.ResponseRecorder {
	rr := httptest.NewRecorder()
	handleInsightsLens(rr, httptest.NewRequest(http.MethodGet, "/api/v1/insights/lens"+query, nil))
	return rr
}

func TestInsightsLensHandler_EndingDaysDefaultAndClamp(t *testing.T) {
	days, _ := stubLens(t)
	for q, want := range map[string]int{
		"?kind=ending": 30, "?kind=ending&days=7": 7, "?kind=ending&days=0": 1,
		"?kind=ending&days=-4": 1, "?kind=ending&days=9999": 365, "?kind=ending&days=365": 365,
	} {
		*days = 0
		if rr := getLens(q); rr.Code != http.StatusOK || *days != want {
			t.Errorf("%q: status %d days %d, want %d", q, rr.Code, *days, want)
		}
	}
}

func TestInsightsLensHandler_RejectsBadQuestions(t *testing.T) {
	days, group := stubLens(t)
	for _, q := range []string{
		"", "?kind=", "?kind=people", "?kind=Ending", "?days=5",
		"?kind=ending&days=abc", "?kind=ending&days=1.5", "?kind=ending&days=99999999999999999999",
		"?kind=ending&group=left",
		"?kind=group", "?kind=group&group=", "?kind=group&group=everyone", "?kind=group&group=Left",
		"?kind=group&group=team:", "?kind=group&group=team:%20%20", "?kind=group&group=team",
		"?kind=group&group=left&days=5",
	} {
		rr := getLens(q)
		if rr.Code != http.StatusBadRequest || *days != 0 || *group != "" {
			t.Errorf("%q: status %d (read: days %d group %q), want 400 and no read", q, rr.Code, *days, *group)
		}
	}
}

func TestInsightsLensHandler_GroupsReachTheService(t *testing.T) {
	_, group := stubLens(t)
	for q, want := range map[string]string{
		"?kind=group&group=left":           "left",
		"?kind=group&group=new_term":       "new_term",
		"?kind=group&group=team:Fab%20Lab": "team:Fab Lab",
	} {
		*group = ""
		rr := getLens(q)
		var body struct {
			Kind  string `json:"kind"`
			Group string `json:"group"`
		}
		if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil || rr.Code != http.StatusOK || *group != want || body.Kind != "group" || body.Group != want {
			t.Errorf("%q: status %d group %q body %s", q, rr.Code, *group, rr.Body)
		}
	}
}

func TestInsightsLensHandler_FailureIs500(t *testing.T) {
	stubLens(t)
	svcInsightsLensEnding = func(context.Context, int) (models.InsightsLensEnding, error) {
		return models.InsightsLensEnding{}, errors.New("boom")
	}
	if rr := getLens("?kind=ending"); rr.Code != http.StatusInternalServerError {
		t.Errorf("status %d", rr.Code)
	}
}
