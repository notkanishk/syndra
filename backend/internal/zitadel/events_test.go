package zitadel

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

// rawResp answers with a body verbatim, because these fixtures are about the
// exact JSON a documented API says it returns — re-encoding them through a Go
// value would assert against this package's idea of the shape rather than the
// shape itself.
func rawResp(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}
}

// The event that created a grant, which is the one thing the reconciliation
// sweep cannot work out for itself.
//
// The payload below follows Zitadel's PUBLISHED schema for
// `POST /admin/v1/events/_search` and has NOT been observed from a live
// instance. That distinction is the whole lesson of this branch, so what these
// assert is mostly the decoder's tolerance: `type` and `aggregate.type` are
// OBJECTS, every field is optional, and no single surprise may cost the whole
// response.

// A response in the documented shape, with the nesting that a naive decoder
// gets wrong.
const documentedEventsResponse = `{
  "events": [
    {
      "editor": {"userId": "212418735923626245", "displayName": "Maya Chen", "service": "Management-API"},
      "aggregate": {
        "id": "385621222114722822",
        "type": {"type": "usergrant", "localized": {"key": "EventTypes.usergrant"}},
        "resourceOwner": "212418214204538113"
      },
      "sequence": "9",
      "creationDate": "2026-08-03T09:25:46.211127Z",
      "payload": {"roleKeys": ["admin"]},
      "type": {"type": "user.grant.added", "localized": {"key": "EventTypes.user.grant.added"}}
    },
    {
      "editor": {"userId": "212418735923626245", "displayName": "Maya Chen", "service": "Management-API"},
      "aggregate": {"id": "385621222114722822", "type": {"type": "usergrant"}},
      "sequence": "14",
      "creationDate": "2026-08-05T11:02:00Z",
      "type": {"type": "user.grant.changed"}
    }
  ]
}`

func TestGrantOriginReadsTheEarliestEvent(t *testing.T) {
	resetDeps(t)
	client, _ := testClient(t)

	var path, method string
	var body map[string]any
	httpDo = func(_ *http.Client, req *http.Request) (*http.Response, error) {
		path, method = req.URL.Path, req.Method
		raw, _ := io.ReadAll(req.Body)
		json.Unmarshal(raw, &body)
		return rawResp(200, documentedEventsResponse), nil
	}

	origin, err := client.GrantOriginByID(context.Background(), "385621222114722822")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if method != "POST" || path != "/admin/v1/events/_search" {
		t.Fatalf("unexpected request: %s %s", method, path)
	}
	// The ADMIN api, not management. Every other call this client makes is
	// management, and reaching the event log through the wrong one 404s.
	if body["aggregateId"] != "385621222114722822" {
		t.Errorf("must filter by the grant's aggregate id: %v", body["aggregateId"])
	}
	// Ascending, so the first result is the creation rather than the newest
	// change. Descending here would attribute the grant to whoever last
	// touched it, which is a different person and a wrong accusation.
	if body["asc"] != true {
		t.Errorf("must ask ascending: %v", body["asc"])
	}
	if body["from"] == nil || body["from"] == "" {
		t.Error("`from` is required by the API; omitting it fails the call")
	}
	// No event-type or aggregate-type filter: this package has never seen
	// Zitadel's event vocabulary, and a guessed constant that matches nothing
	// returns an empty list that reads as "no history".
	if _, guessed := body["eventTypes"]; guessed {
		t.Error("must not filter on an event-type name this package has not observed")
	}
	if _, guessed := body["aggregateTypes"]; guessed {
		t.Error("must not filter on an aggregate-type name this package has not observed")
	}

	if origin == nil {
		t.Fatal("want an origin")
	}
	if origin.ActorID != "212418735923626245" || origin.ActorName != "Maya Chen" {
		t.Errorf("wrong actor: %+v", origin)
	}
	if origin.Service != "Management-API" {
		t.Errorf("the machine actor is most of the triage decision: %q", origin.Service)
	}
	// `type` is an OBJECT. A string field here fails the entire response and
	// the origin comes back as "unknown", which is indistinguishable from a
	// grant Zitadel genuinely could not attribute.
	if origin.EventType != "user.grant.added" {
		t.Errorf("event type must be read from type.type: %q", origin.EventType)
	}
	if origin.At.Format("2006-01-02") != "2026-08-03" {
		t.Errorf("wrong timestamp: %v", origin.At)
	}
	if !origin.Attributed() {
		t.Error("an event naming a person is attributed")
	}
}

// A grant older than the retained history. Not an error — "the log does not go
// back that far" is an answer, and turning it into a failure would put a red
// state on the triage row for a grant that is merely old.
func TestNoEventsIsNotAFailure(t *testing.T) {
	resetDeps(t)
	client, _ := testClient(t)
	httpDo = func(*http.Client, *http.Request) (*http.Response, error) {
		return rawResp(200, `{"events": []}`), nil
	}

	origin, err := client.GrantOriginByID(context.Background(), "g-1")
	if err != nil {
		t.Fatalf("an empty log is not an error: %v", err)
	}
	if origin != nil {
		t.Fatalf("want no origin, got %+v", origin)
	}
}

// Every field optional, every level tolerated. This is the shape an unobserved
// API is most likely to differ in, and one absent nesting must not cost the
// whole read.
func TestAPartialEventStillAttributesWhatItCan(t *testing.T) {
	resetDeps(t)
	client, _ := testClient(t)
	httpDo = func(*http.Client, *http.Request) (*http.Response, error) {
		// No editor, no aggregate, an unparseable date.
		return rawResp(200, `{"events":[{"type":{"type":"user.grant.added"},"creationDate":"not a date"}]}`), nil
	}

	origin, err := client.GrantOriginByID(context.Background(), "g-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if origin.EventType != "user.grant.added" {
		t.Errorf("the field that was present must survive: %q", origin.EventType)
	}
	if !origin.At.IsZero() {
		t.Error("an unparseable date costs the date, not the read")
	}
	// The event exists and names nobody. That is a real answer and a different
	// one from "we could not ask", so the surface must be able to tell them
	// apart.
	if origin.Attributed() {
		t.Error("an event with no editor is not attributed")
	}
}

func TestAGrantIDIsRequired(t *testing.T) {
	resetDeps(t)
	client, _ := testClient(t)
	called := false
	httpDo = func(*http.Client, *http.Request) (*http.Response, error) {
		called = true
		return rawResp(200, `{"events":[]}`), nil
	}

	if _, err := client.GrantOriginByID(context.Background(), "  "); err == nil {
		t.Fatal("want a refusal")
	}
	if called {
		t.Error("an unfiltered event search returns the whole deployment's history")
	}
}
