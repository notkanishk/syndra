package zitadel

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// SearchEvents: the documented request shape, newest first, one more than the
// limit to know about truncation, and a tolerant decode.
func TestSearchEventsAsksByTypeAndWindowAndSaysWhenCutOff(t *testing.T) {
	resetDeps(t)
	client, _ := testClient(t)

	var body map[string]any
	var path string
	httpDo = func(_ *http.Client, req *http.Request) (*http.Response, error) {
		path = req.URL.Path
		raw, _ := io.ReadAll(req.Body)
		json.Unmarshal(raw, &body)
		return rawResp(200, `{"events":[
		  {"editor":{"userId":"u1"},"aggregate":{"id":"u1","type":{"type":"user"}},"creationDate":"2026-11-02T09:00:00Z",
		   "type":{"type":"user.token.added"},"payload":{"applicationId":"app-1"}},
		  {"aggregate":{"id":"s1","type":{"type":"oidc_session"}},"creationDate":"2026-11-01T09:00:00Z",
		   "type":{"type":"oidc_session.added"},"payload":{"userID":"u2","clientID":"c@p"}},
		  {"type":{"type":"user.token.added"},"creationDate":"not a date"}
		]}`), nil
	}
	since := time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC)
	until := since.Add(30 * 24 * time.Hour)
	events, truncated, err := client.SearchEvents(context.Background(), EventSearch{
		Types: []string{"user.token.added", "oidc_session.added"}, Since: since, Until: until, Limit: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if path != "/admin/v1/events/_search" || body["asc"] != false || body["limit"] != float64(3) {
		t.Fatalf("request %s %v", path, body)
	}
	rng, _ := body["range"].(map[string]any)
	if rng["since"] != "2026-10-03T00:00:00Z" || rng["until"] != "2026-11-02T00:00:00Z" {
		t.Errorf("range = %v", rng)
	}
	if types, _ := body["eventTypes"].([]any); len(types) != 2 {
		t.Errorf("eventTypes = %v", body["eventTypes"])
	}
	if !truncated || len(events) != 2 {
		t.Fatalf("events=%d truncated=%v; want the first 2, cut off", len(events), truncated)
	}
	first, second := events[0], events[1]
	if first.Type != "user.token.added" || first.AggregateID != "u1" || first.AggregateType != "user" ||
		first.EditorUserID != "u1" || !first.At.Equal(time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC)) ||
		!strings.Contains(string(first.Payload), "app-1") {
		t.Errorf("first = %+v", first)
	}
	if second.AggregateType != "oidc_session" || !strings.Contains(string(second.Payload), `"u2"`) {
		t.Errorf("second = %+v", second)
	}
}

func TestSearchEventsRefusesAnUnboundedAsk(t *testing.T) {
	resetDeps(t)
	client, _ := testClient(t)
	called := false
	httpDo = func(*http.Client, *http.Request) (*http.Response, error) {
		called = true
		return rawResp(200, `{"events":[]}`), nil
	}
	now := time.Now()
	for name, q := range map[string]EventSearch{
		"no types":     {Since: now.Add(-time.Hour), Until: now, Limit: 10},
		"no window":    {Types: []string{"x"}, Limit: 10},
		"backwards":    {Types: []string{"x"}, Since: now, Until: now.Add(-time.Hour), Limit: 10},
		"no limit":     {Types: []string{"x"}, Since: now.Add(-time.Hour), Until: now},
		"over the cap": {Types: []string{"x"}, Since: now.Add(-time.Hour), Until: now, Limit: 1001},
	} {
		if _, _, err := client.SearchEvents(context.Background(), q); err == nil {
			t.Errorf("%s: want a refusal", name)
		}
	}
	if called {
		t.Error("a refused search must not reach Zitadel")
	}
}
