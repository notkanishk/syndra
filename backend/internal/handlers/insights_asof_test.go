package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"syndra/internal/models"
	"syndra/internal/services"
)

func stubAsOf(t *testing.T) {
	t.Helper()
	a, tl := svcInsightsAsOf, svcInsightsTimeline
	t.Cleanup(func() { svcInsightsAsOf, svcInsightsTimeline = a, tl })
}

func getAsOf(query string) *httptest.ResponseRecorder {
	rr := httptest.NewRecorder()
	handleInsightsAsOf(rr, httptest.NewRequest(http.MethodGet, "/api/v1/insights/asof"+query, nil))
	return rr
}

func getTimeline(query string) *httptest.ResponseRecorder {
	rr := httptest.NewRecorder()
	handleInsightsTimeline(rr, httptest.NewRequest(http.MethodGet, "/api/v1/insights/timeline"+query, nil))
	return rr
}

func TestInsightsAsOfHandler_ReturnsTheSnapshot(t *testing.T) {
	stubAsOf(t)
	var asked string
	svcInsightsAsOf = func(_ context.Context, day string) (models.InsightsAsOf, error) {
		asked = day
		return models.InsightsAsOf{
			Day: "2026-10-12", TakenAt: time.Date(2026, 10, 12, 3, 0, 0, 0, time.UTC),
			Topology:   models.TopologyGraph{Nodes: []models.TopologyNode{}, Edges: []models.TopologyEdge{}},
			RolePeople: map[string]int{"p:r": 4}, FirstDay: "2026-10-10", ChangesSince: 7,
		}, nil
	}
	rr := getAsOf("?date=2020-10-13")
	if rr.Code != http.StatusOK || asked != "2020-10-13" {
		t.Fatalf("status %d asked %q: %s", rr.Code, asked, rr.Body.String())
	}
	var body map[string]json.RawMessage
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"day", "taken_at", "topology", "role_people", "first_day", "changes_since"} {
		if _, ok := body[k]; !ok {
			t.Errorf("response has no %q: %s", k, rr.Body.String())
		}
	}
	if string(body["changes_since"]) != "7" || string(body["day"]) != `"2026-10-12"` {
		t.Errorf("body = %s", rr.Body.String())
	}
}

func TestInsightsAsOfHandler_BadDatesAre400AndNeverReachTheService(t *testing.T) {
	stubAsOf(t)
	called := false
	svcInsightsAsOf = func(context.Context, string) (models.InsightsAsOf, error) {
		called = true
		return models.InsightsAsOf{}, nil
	}
	tomorrow := time.Now().AddDate(0, 0, 1).Format("2006-01-02")
	for _, q := range []string{"", "?date=", "?date=" + tomorrow, "?date=2026-13-01", "?date=yesterday", "?date=2026-1-5"} {
		rr := getAsOf(q)
		var body ErrorResponse
		_ = json.Unmarshal(rr.Body.Bytes(), &body)
		if rr.Code != http.StatusBadRequest || body.Error != "VALIDATION_FAILED" {
			t.Errorf("%q: status %d body %s, want 400 VALIDATION_FAILED", q, rr.Code, rr.Body.String())
		}
	}
	if called {
		t.Fatal("an invalid date must never reach the service")
	}
}

func TestInsightsAsOfHandler_NoSnapshotIs404WithFirstDay(t *testing.T) {
	stubAsOf(t)
	first := "2026-10-10"
	for name, tc := range map[string]struct {
		err  *services.NoSnapshotError
		want string
	}{
		"before the first": {&services.NoSnapshotError{FirstDay: &first}, `"2026-10-10"`},
		"none at all":      {&services.NoSnapshotError{}, `null`},
	} {
		svcInsightsAsOf = func(context.Context, string) (models.InsightsAsOf, error) {
			return models.InsightsAsOf{}, tc.err
		}
		rr := getAsOf("?date=2020-10-01")
		var body struct {
			Error   string                     `json:"error"`
			Details map[string]json.RawMessage `json:"details"`
		}
		if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		fd, present := body.Details["first_day"]
		if rr.Code != http.StatusNotFound || body.Error != "NO_SNAPSHOT" || !present || string(fd) != tc.want {
			t.Errorf("%s: status %d body %s", name, rr.Code, rr.Body.String())
		}
	}
}

func TestInsightsAsOfHandler_FailureIs500(t *testing.T) {
	stubAsOf(t)
	svcInsightsAsOf = func(context.Context, string) (models.InsightsAsOf, error) {
		return models.InsightsAsOf{}, errors.New("boom")
	}
	if rr := getAsOf("?date=2020-10-01"); rr.Code != http.StatusInternalServerError {
		t.Fatalf("status %d", rr.Code)
	}
}

func TestInsightsTimelineHandler_RangeLimits(t *testing.T) {
	stubAsOf(t)
	var gotFrom, gotTo string
	svcInsightsTimeline = func(_ context.Context, from, to string) (models.InsightsTimeline, error) {
		gotFrom, gotTo = from, to
		return models.InsightsTimeline{Days: []models.TimelineDay{{Day: from, Changes: 2}}}, nil
	}
	rr := getTimeline("?from=2026-01-01&to=2027-02-04") // 400 days: the limit
	if rr.Code != http.StatusOK || gotFrom != "2026-01-01" || gotTo != "2027-02-04" {
		t.Fatalf("status %d from %q to %q: %s", rr.Code, gotFrom, gotTo, rr.Body.String())
	}
	var body struct {
		Days         []map[string]any `json:"days"`
		SnapshotDays []*string        `json:"snapshot_days"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil || len(body.Days) != 1 || len(body.SnapshotDays) != 2 {
		t.Fatalf("body %s (%v)", rr.Body.String(), err)
	}

	gotFrom = ""
	for _, q := range []string{"", "?from=2026-01-01", "?from=2026-01-01&to=2027-02-05", "?from=2026-02-02&to=2026-02-01", "?from=x&to=y"} {
		rr := getTimeline(q)
		var e ErrorResponse
		_ = json.Unmarshal(rr.Body.Bytes(), &e)
		if rr.Code != http.StatusBadRequest || e.Error != "VALIDATION_FAILED" {
			t.Errorf("%q: status %d body %s, want 400", q, rr.Code, rr.Body.String())
		}
	}
	if gotFrom != "" {
		t.Fatal("an invalid range must never reach the service")
	}
}
