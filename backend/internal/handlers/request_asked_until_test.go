package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"syndra/internal/db"
	"syndra/internal/models"
	"syndra/internal/services/propagation"
)

// An ask "until the end of term" carries the term's end date. Approving it late
// lands on that date, not on now plus the days it was worth when it was asked,
// and approving it after the date is refused instead of giving a grant that has
// already ended.

func TestPlanApproval_AskedUntil(t *testing.T) {
	now := time.Date(2026, 12, 15, 12, 0, 0, 0, time.UTC)
	termEnd := time.Date(2026, 12, 18, 23, 59, 59, 0, time.UTC)
	gone := now.Add(-time.Hour)
	held := models.RequestHolding{Kind: models.HoldingUntil, Until: ptrTime(now.Add(24 * time.Hour))}
	none := models.RequestHolding{Kind: models.HoldingNone}

	t.Run("a late approval lands on the term end, not now plus the days", func(t *testing.T) {
		// Asked in September: 90 days then. Decided three days before term end.
		plan, err := planApproval(models.AccessRequest{Kind: requestKindAccess, DurationDays: days(90), AskedUntil: &termEnd}, none, now)
		if err != nil || plan.ExpiresAt == nil || !plan.ExpiresAt.Equal(termEnd) {
			t.Fatalf("want %v, got %v (%v)", termEnd, plan.ExpiresAt, err)
		}
		if plan.Effect != effectGive || plan.EndRule != db.EndKept {
			t.Fatalf("an ask still gives the role: %s/%s", plan.Effect, plan.EndRule)
		}
	})

	t.Run("keeping it to a date lands on that date", func(t *testing.T) {
		plan, err := planApproval(models.AccessRequest{Kind: requestKindExtend, DurationDays: days(90), AskedUntil: &termEnd}, held, now)
		if err != nil || plan.ExpiresAt == nil || !plan.ExpiresAt.Equal(termEnd) || plan.Effect != effectExtend {
			t.Fatalf("want extend to %v, got %v/%v (%v)", termEnd, plan.Effect, plan.ExpiresAt, err)
		}
	})

	t.Run("a date that has passed is refused, for either kind", func(t *testing.T) {
		for _, kind := range []string{requestKindAccess, requestKindExtend} {
			h := none
			if kind == requestKindExtend {
				h = held
			}
			if _, err := planApproval(models.AccessRequest{Kind: kind, DurationDays: days(30), AskedUntil: &gone}, h, now); !errors.Is(err, errAskEnded) {
				t.Fatalf("%s: want errAskEnded, got %v", kind, err)
			}
		}
	})

	t.Run("a days ask is unchanged: it counts from the decision", func(t *testing.T) {
		plan, err := planApproval(models.AccessRequest{Kind: requestKindAccess, DurationDays: days(30)}, none, now)
		if want := now.Add(30 * 24 * time.Hour); err != nil || plan.ExpiresAt == nil || !plan.ExpiresAt.Equal(want) {
			t.Fatalf("want %v, got %v (%v)", want, plan.ExpiresAt, err)
		}
	})
}

func ptrTime(t time.Time) *time.Time { return &t }

// decideAsk runs the decision endpoint for a pending request and returns the
// grant's end as it reached the queue, and whether anything was approved.
func decideAsk(t *testing.T, request models.AccessRequest) (code int, body string, expiresAt *time.Time, approved bool) {
	t.Helper()
	resetAccessDeps(t)
	request.ID, request.RequesterID, request.ProjectID, request.RoleKey, request.Status = "r1", "u1", "p1", "r", "pending"
	dbGetAccessRequestByID = func(context.Context, string) (models.AccessRequest, error) { return request, nil }
	dbApproveRequestAndEnqueue = func(_ context.Context, _, _, _ string, p db.EnqueueParams, _, _ *time.Time) (db.EnqueueResult, error) {
		expiresAt, approved = p.ExpiresAt, true
		return db.EnqueueResult{OutboxID: "ob", Status: "pending"}, nil
	}
	svcDrainPropagationRow = func(context.Context, string) (propagation.DrainResult, error) {
		return propagation.DrainResult{Applied: 1}, nil
	}
	cacheRebuildUser = func(context.Context, string, []string) {}
	origConverge := svcConvergeProjectChange
	t.Cleanup(func() { svcConvergeProjectChange = origConverge })
	svcConvergeProjectChange = func(context.Context, string, string, string) error { return nil }
	dbInsertAuditLog = func(context.Context, string, string, string, string) error { return nil }

	req := httptest.NewRequest(http.MethodPost, "/api/v1/requests/r1/decision",
		strings.NewReader(`{"status":"approved","reviewer_id":"op-2","review_note":"x"}`))
	req.SetPathValue("id", "r1")
	rr := httptest.NewRecorder()
	handleResolveAccessRequest(rr, req)
	return rr.Code, rr.Body.String(), expiresAt, approved
}

func TestApprovingALateTermAskGrantsToTheTermEnd(t *testing.T) {
	termEnd := time.Now().UTC().Add(5 * 24 * time.Hour).Truncate(time.Second)
	// Asked with 120 days; approved with five left in the term.
	code, body, got, approved := decideAsk(t, models.AccessRequest{Kind: requestKindAccess, DurationDays: days(120), AskedUntil: &termEnd})
	if code != http.StatusOK || !approved || got == nil || !got.Equal(termEnd) {
		t.Fatalf("want 200 and a grant ending %v, got %d %s (end %v)", termEnd, code, body, got)
	}
}

func TestApprovingAfterTheAskedEndIsRefused(t *testing.T) {
	over := time.Now().UTC().Add(-time.Hour)
	code, body, _, approved := decideAsk(t, models.AccessRequest{Kind: requestKindAccess, DurationDays: days(30), AskedUntil: &over})
	if code != http.StatusConflict || !strings.Contains(body, "END_PASSED") || approved {
		t.Fatalf("want 409 END_PASSED and nothing approved, got %d %s (approved %v)", code, body, approved)
	}
	if !strings.Contains(body, "has already passed") {
		t.Fatalf("the refusal should say why in plain words: %s", body)
	}
}

func TestApprovingADaysAskStillCountsFromTheDecision(t *testing.T) {
	before := time.Now().UTC()
	code, body, got, approved := decideAsk(t, models.AccessRequest{Kind: requestKindAccess, DurationDays: days(30)})
	want := before.Add(30 * 24 * time.Hour)
	if code != http.StatusOK || !approved || got == nil || got.Before(want) || got.After(want.Add(time.Minute)) {
		t.Fatalf("want a grant ending about %v, got %d %s (end %v)", want, code, body, got)
	}
}

func TestCreatingAnAskWithAnEndDate(t *testing.T) {
	future := time.Now().UTC().Add(48 * time.Hour).Truncate(time.Second)
	for _, tc := range []struct {
		name   string
		body   string
		status int
		until  *time.Time
	}{
		{"an end date is filed with the ask", `{"requester_id":"u1","project_id":"p","role_key":"r","justification":"x","duration_days":3,"until":"` + future.Format(time.RFC3339) + `"}`, http.StatusCreated, &future},
		{"days only, as older clients send it, files no end date", `{"requester_id":"u1","project_id":"p","role_key":"r","justification":"x","duration_days":30}`, http.StatusCreated, nil},
		{"an end date that has passed is refused", `{"requester_id":"u1","project_id":"p","role_key":"r","justification":"x","until":"2020-01-01T00:00:00Z"}`, http.StatusBadRequest, nil},
		{"an end date that isn't a date is refused", `{"requester_id":"u1","project_id":"p","role_key":"r","justification":"x","until":"end of term"}`, http.StatusBadRequest, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resetAccessDeps(t)
			dbInsertAuditLog = func(context.Context, string, string, string, string) error { return nil }
			var filed *time.Time
			dbCreateAccessRequestOfKind = func(_ context.Context, _ string, _, _, _, _ string, _ *int, until *time.Time) (string, error) {
				filed = until
				return "req-1", nil
			}
			rr := httptest.NewRecorder()
			handleCreateAccessRequest(rr, httptest.NewRequest(http.MethodPost, "/api/v1/requests", strings.NewReader(tc.body)))
			if rr.Code != tc.status {
				t.Fatalf("want %d, got %d: %s", tc.status, rr.Code, rr.Body.String())
			}
			if (tc.until == nil) != (filed == nil) || (tc.until != nil && !filed.Equal(*tc.until)) {
				t.Fatalf("filed end %v, want %v", filed, tc.until)
			}
		})
	}
}
