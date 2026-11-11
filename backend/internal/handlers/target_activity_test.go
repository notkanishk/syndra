package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"syndra/internal/addons"
	"syndra/internal/auth"
)

// The target's own audit log, reached at last.
//
// `activity.get` was implemented by the add-on and declared in the backend's
// policy table from the day the platform landed, and no route ever called it —
// so the two TrueNAS roles it needs, `audit.query` and `sharing.smb.query`,
// were configured on the deployment's API key and exercised by nothing. Two
// halves that agreed with each other and never met, which is the defect shape
// this whole branch kept finding.

func withActivity(t *testing.T, stub func(ctx context.Context, target, subject, actor, since string) addons.ActivityReport) {
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
	withActivity(t, func(context.Context, string, string, string, string) addons.ActivityReport {
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
	withActivity(t, func(context.Context, string, string, string, string) addons.ActivityReport {
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
	withActivity(t, func(_ context.Context, _, subject, _, since string) addons.ActivityReport {
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
	withActivity(t, func(context.Context, string, string, string, string) addons.ActivityReport {
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
	if body["reason"] != "unreachable" {
		t.Fatalf("expected reason=unreachable, got %v", body["reason"])
	}
	if _, present := body["events"]; present {
		t.Fatal("an unreadable log must not carry an events list at all")
	}
}

// A 422 on activity.get is the add-on saying the subject has no bound
// account — a positive fact, not a failure — and it must never surface as
// "addon returned 422".
func TestTargetActivity_NoBoundAccountIsNotAFailure(t *testing.T) {
	withActivity(t, func(context.Context, string, string, string, string) addons.ActivityReport {
		return addons.ActivityReport{
			Outcome: addons.OutcomeRejected,
			Status:  http.StatusUnprocessableEntity,
			Err:     errors.New("addon returned 422"),
		}
	})

	rr := activityRequest("?subject=u1")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["readable"] != false {
		t.Fatalf("expected readable=false, got %v", body["readable"])
	}
	if body["reason"] != "no_account" {
		t.Fatalf("expected reason=no_account, got %v", body["reason"])
	}
	if body["detail"] != "" {
		t.Fatalf("no_account must carry no detail text, got %v", body["detail"])
	}
}

// Every other failure keeps its own plain-language detail — never a raw
// status number leaking to the client.
func TestTargetActivity_OtherFailuresGetPlainLanguageDetail(t *testing.T) {
	withActivity(t, func(context.Context, string, string, string, string) addons.ActivityReport {
		return addons.ActivityReport{
			Outcome: addons.OutcomeIndeterminate,
			Status:  http.StatusInternalServerError,
			Err:     errors.New("addon returned 500"),
		}
	})

	rr := activityRequest("?subject=u1")
	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["reason"] != "unreachable" {
		t.Fatalf("expected reason=unreachable, got %v", body["reason"])
	}
	detail, _ := body["detail"].(string)
	if detail == "" || strings.Contains(detail, "500") || strings.Contains(detail, "422") {
		t.Fatalf("detail must be plain language with no raw status code, got %q", detail)
	}
}

func TestTargetActivity_CarriesTheUncoveredResources(t *testing.T) {
	taken := time.Date(2026, 8, 20, 10, 5, 0, 0, time.UTC)
	withActivity(t, func(context.Context, string, string, string, string) addons.ActivityReport {
		return addons.ActivityReport{
			Events:      []addons.ActivityEvent{{At: "2026-08-20T10:00:00Z", Event: "CONNECT", Resource: "lab", Success: true}},
			Uncovered:   []string{"scratch"},
			Truncated:   true,
			Observation: &addons.Observation{TakenAt: taken, Current: true},
			Outcome:     addons.OutcomeSucceeded,
		}
	})

	rr := activityRequest("?subject=u1")
	var body struct {
		Readable  bool      `json:"readable"`
		Uncovered []string  `json:"uncovered"`
		Truncated bool      `json:"truncated"`
		TakenAt   time.Time `json:"taken_at"`
		Events    []struct {
			Event    string `json:"event"`
			Resource string `json:"resource"`
		} `json:"events"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !body.Readable || len(body.Events) != 1 || body.Events[0].Event != "CONNECT" || body.Events[0].Resource != "lab" {
		t.Fatalf("unexpected body: %s", rr.Body.String())
	}
	// Without this an operator reads a short list as "quiet" when half the
	// shares were never being watched.
	if len(body.Uncovered) != 1 || body.Uncovered[0] != "scratch" || !body.Truncated || !body.TakenAt.Equal(taken) {
		t.Fatalf("the uncovered resources, truncation and read time must travel: %s", rr.Body.String())
	}
}

// The operator's reads of a target name the person as subject and the operator
// as actor (the guide's subject/actor split): neither the member as their own
// reader, nor nobody.
func TestTargetOperatorReads_CarryTheOperatorAsActor(t *testing.T) {
	asOperator := func(req *http.Request) *http.Request {
		return req.WithContext(withPrincipal(req.Context(), &auth.Principal{Subject: "operator-289471021834760199"}))
	}

	var subject, actor string
	withActivity(t, func(_ context.Context, _, s, a, _ string) addons.ActivityReport {
		subject, actor = s, a
		return addons.ActivityReport{Outcome: addons.OutcomeSucceeded}
	})
	req := asOperator(httptest.NewRequest(http.MethodGet, "/api/v1/targets/truenas/activity?subject=u1", nil))
	req.SetPathValue("target", "truenas")
	handleTargetActivity(httptest.NewRecorder(), req)
	if subject != "u1" || actor != "operator-289471021834760199" {
		t.Errorf("activity: subject %q actor %q", subject, actor)
	}

	actor = ""
	withSystemHealth(t, func(_ context.Context, _, a string) addons.SystemReport {
		actor = a
		return addons.SystemReport{Outcome: addons.OutcomeSucceeded}
	})
	req = asOperator(httptest.NewRequest(http.MethodGet, "/api/v1/targets/truenas/system-health", nil))
	req.SetPathValue("target", "truenas")
	handleTargetSystemHealth(httptest.NewRecorder(), req)
	if actor != "operator-289471021834760199" {
		t.Errorf("system health: actor %q", actor)
	}
}
