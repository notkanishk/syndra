package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"syndra/internal/addons"
)

// The target's own audit log, reached at last.
//
// `activity.get` was implemented by the add-on and declared in the backend's
// policy table from the day the platform landed, and no route ever called it —
// so the two TrueNAS roles it needs, `audit.query` and `sharing.smb.query`,
// were configured on the deployment's API key and exercised by nothing. Two
// halves that agreed with each other and never met, which is the defect shape
// this whole branch kept finding.

func withActivity(t *testing.T, stub func(ctx context.Context, target, subject, since string) addons.ActivityReport) {
	t.Helper()
	original := addonsActivity
	addonsActivity = stub
	t.Cleanup(func() { addonsActivity = original })
}

func activityRequest(query string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/targets/truenas/activity"+query, nil)
	req.SetPathValue("target", "truenas")
	rr := httptest.NewRecorder()
	handleTargetActivity(rr, req)
	return rr
}

func TestTargetActivity_RequiresASubject(t *testing.T) {
	withActivity(t, func(context.Context, string, string, string) addons.ActivityReport {
		t.Fatal("the add-on must not be called without a subject")
		return addons.ActivityReport{}
	})

	rr := activityRequest("")
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

// `since` reaches the target's own query builder. A value the backend cannot
// parse is one the add-on would have to decide about, which is the wrong place
// for that decision.
func TestTargetActivity_RefusesAnUnparseableSince(t *testing.T) {
	withActivity(t, func(context.Context, string, string, string) addons.ActivityReport {
		t.Fatal("the add-on must not be called with a since it cannot trust")
		return addons.ActivityReport{}
	})

	rr := activityRequest("?subject=u1&since=last-tuesday")
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestTargetActivity_PassesSubjectAndSinceThrough(t *testing.T) {
	var gotSubject, gotSince string
	withActivity(t, func(_ context.Context, _, subject, since string) addons.ActivityReport {
		gotSubject, gotSince = subject, since
		return addons.ActivityReport{Events: []addons.ActivityEvent{}, Outcome: addons.OutcomeSucceeded}
	})

	rr := activityRequest("?subject=u1&since=2026-08-01T00:00:00Z")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if gotSubject != "u1" || gotSince != "2026-08-01T00:00:00Z" {
		t.Fatalf("subject/since not passed through: %q %q", gotSubject, gotSince)
	}
}

// The distinction this surface exists for. "Nobody was watching" and "nothing
// happened" are the same empty list otherwise, and they are opposite answers.
func TestTargetActivity_UnreadableIsNotEmpty(t *testing.T) {
	withActivity(t, func(context.Context, string, string, string) addons.ActivityReport {
		return addons.ActivityReport{Outcome: addons.OutcomeUnreached, Err: errors.New("target unreachable")}
	})

	rr := activityRequest("?subject=u1")
	// 200, not an error: the add-on being down is the answer to the question,
	// the same way the health surface answers it.
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["readable"] != false {
		t.Fatalf("an unreachable target must say so, got %v", body["readable"])
	}
	if _, present := body["events"]; present {
		t.Fatal("an unreadable log must not carry an events list at all")
	}
}

func TestTargetActivity_CarriesTheUncoveredShares(t *testing.T) {
	withActivity(t, func(context.Context, string, string, string) addons.ActivityReport {
		return addons.ActivityReport{
			Events:          []addons.ActivityEvent{{At: "2026-08-20T10:00:00Z", Event: "CONNECT", Share: "lab", Success: true}},
			UncoveredShares: []string{"scratch"},
			Outcome:         addons.OutcomeSucceeded,
		}
	})

	rr := activityRequest("?subject=u1")
	var body struct {
		Readable        bool     `json:"readable"`
		UncoveredShares []string `json:"uncovered_shares"`
		Events          []struct {
			Event string `json:"event"`
		} `json:"events"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !body.Readable || len(body.Events) != 1 || body.Events[0].Event != "CONNECT" {
		t.Fatalf("unexpected body: %s", rr.Body.String())
	}
	// Without this an operator reads a short list as "quiet" when half the
	// shares were never being watched.
	if len(body.UncoveredShares) != 1 || body.UncoveredShares[0] != "scratch" {
		t.Fatalf("the uncovered shares must travel: %s", rr.Body.String())
	}
}
