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
	"syndra/internal/zitadel"
)

// "Who did this" — the question the reconciliation sweep cannot answer.
//
// The sweep compares grant SETS, so every row it raises says "unknown actor".
// The add-on targets close that gap with a recorded merge base; Zitadel does
// not need one, because it is event-sourced and the creating event carries its
// editor. These assert the distinctions that make the answer trustworthy:
// could-not-ask, no-history, and recorded-but-anonymous are three different
// answers and none of them may render as another.

func withDriftOrigin(t *testing.T, item models.DriftItem,
	origin func(ctx context.Context, grantID string) (*zitadel.GrantOrigin, error)) {
	t.Helper()
	oldItem, oldOrigin := dbGetDriftItem, zitadelGrantOrigin
	dbGetDriftItem = func(context.Context, string) (models.DriftItem, error) { return item, nil }
	zitadelGrantOrigin = origin
	t.Cleanup(func() { dbGetDriftItem, zitadelGrantOrigin = oldItem, oldOrigin })
}

func originBody(t *testing.T) map[string]any {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/governance/drift/d1/origin", nil)
	req.SetPathValue("id", "d1")
	rr := httptest.NewRecorder()
	handleDriftOrigin(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200 so the row can say what happened, got %d: %s", rr.Code, rr.Body)
	}
	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return body
}

func TestDriftOriginNamesWhoMadeTheGrant(t *testing.T) {
	withDriftOrigin(t,
		models.DriftItem{ID: "d1", ZitadelGrantID: "385621222114722822"},
		func(_ context.Context, grantID string) (*zitadel.GrantOrigin, error) {
			if grantID != "385621222114722822" {
				t.Errorf("must ask about the row's own grant, got %q", grantID)
			}
			return &zitadel.GrantOrigin{
				ActorID: "212418735923626245", ActorName: "Maya Chen",
				Service: "Management-API", EventType: "user.grant.added",
				At: time.Date(2026, 8, 3, 9, 25, 46, 0, time.UTC),
			}, nil
		})

	body := originBody(t)
	if body["readable"] != true || body["recorded"] != true || body["attributed"] != true {
		t.Fatalf("want a fully attributed origin: %v", body)
	}
	if body["actor_name"] != "Maya Chen" || body["service"] != "Management-API" {
		t.Errorf("wrong actor: %v", body)
	}
	if body["at"] != "2026-08-03T09:25:46Z" {
		t.Errorf("wrong timestamp: %v", body["at"])
	}
}

// Zitadel unreachable. Not a 502: the finding is fine, only the lookup failed,
// and rendering the row as broken would send an operator to the wrong problem.
func TestDriftOriginUnreachableIsNotUnattributed(t *testing.T) {
	withDriftOrigin(t,
		models.DriftItem{ID: "d1", ZitadelGrantID: "g-1"},
		func(context.Context, string) (*zitadel.GrantOrigin, error) {
			return nil, errors.New("zitadel api 503")
		})

	body := originBody(t)
	if body["readable"] != false {
		t.Fatal("a failed lookup is not a readable answer")
	}
	// Crucially absent, not empty: an empty actor field reads as "nobody did
	// this", which is a claim nothing supports.
	for _, key := range []string{"actor_id", "actor_name", "attributed", "recorded"} {
		if _, present := body[key]; present {
			t.Errorf("%q must be absent when nothing was read, got %v", key, body[key])
		}
	}
	if body["detail"] == nil || body["detail"] == "" {
		t.Error("an operator is owed the reason")
	}
}

// The grant predates the retained event history. Readable — we asked and
// Zitadel answered — but with nothing recorded. A different fact from a failed
// lookup, and the row should say the log is short rather than that it is down.
func TestDriftOriginDistinguishesAShortLogFromAFailedRead(t *testing.T) {
	withDriftOrigin(t,
		models.DriftItem{ID: "d1", ZitadelGrantID: "g-1"},
		func(context.Context, string) (*zitadel.GrantOrigin, error) { return nil, nil })

	body := originBody(t)
	if body["readable"] != true {
		t.Fatal("Zitadel answered, so the read is readable")
	}
	if body["recorded"] != false {
		t.Fatal("...and it recorded nothing for this aggregate")
	}
}

// An event that exists and names nobody. Readable, recorded, unattributed —
// the third state, and the one a boolean pair would collapse.
func TestDriftOriginSeparatesAnonymousFromAbsent(t *testing.T) {
	withDriftOrigin(t,
		models.DriftItem{ID: "d1", ZitadelGrantID: "g-1"},
		func(context.Context, string) (*zitadel.GrantOrigin, error) {
			return &zitadel.GrantOrigin{EventType: "user.grant.added"}, nil
		})

	body := originBody(t)
	if body["readable"] != true || body["recorded"] != true {
		t.Fatalf("the event exists: %v", body)
	}
	if body["attributed"] != false {
		t.Fatal("an event with no editor names nobody, and must say so")
	}
	if body["event_type"] != "user.grant.added" {
		t.Errorf("what happened is still worth carrying: %v", body["event_type"])
	}
}

// A `syndra_only` row describes access Zitadel does not have, so there is no
// aggregate to ask about. Zitadel must not be called at all.
func TestDriftOriginDoesNotAskAboutARowWithNoGrant(t *testing.T) {
	called := false
	withDriftOrigin(t,
		models.DriftItem{ID: "d1", DriftType: "syndra_only"},
		func(context.Context, string) (*zitadel.GrantOrigin, error) {
			called = true
			return nil, nil
		})

	body := originBody(t)
	if called {
		t.Error("nothing to ask about; the call is a wasted round trip and a confusing log line")
	}
	if body["readable"] != false {
		t.Fatal("no grant means no event to read")
	}
}
