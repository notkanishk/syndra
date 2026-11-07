package handlers

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"syndra/internal/clock"
	"syndra/internal/db"
	"syndra/internal/models"
	"syndra/internal/services"
)

// An ask "until the end of term" carries the term's end date. Approving it late
// lands on that date, not on now plus the days it was worth when it was asked,
// and approving it after the date is refused instead of giving a grant that has
// already ended.

func TestPlanApproval_AskedUntil(t *testing.T) {
	now := time.Date(2026, 12, 15, 12, 0, 0, 0, time.UTC)
	termEnd, _ := clock.EndOfDay("2026-12-18") // the whole of 18 Dec in the makerspace's zone
	gone := now.AddDate(0, 0, -1)
	held := models.RequestHolding{Kind: models.HoldingUntil, Until: ptrTime(now.Add(24 * time.Hour))}
	none := models.RequestHolding{Kind: models.HoldingNone}

	t.Run("a late approval lands on the term end, not now plus the days", func(t *testing.T) {
		// Asked in September: 90 days then. Decided three days before term end.
		plan, err := services.PlanApproval(models.AccessRequest{Kind: services.RequestKindAccess, DurationDays: days(90), AskedUntil: &termEnd}, none, now)
		if err != nil || plan.ExpiresAt == nil || !plan.ExpiresAt.Equal(termEnd) {
			t.Fatalf("want %v, got %v (%v)", termEnd, plan.ExpiresAt, err)
		}
		if plan.Effect != services.EffectGive || plan.EndRule != db.EndKept {
			t.Fatalf("an ask still gives the role: %s/%s", plan.Effect, plan.EndRule)
		}
	})

	t.Run("keeping it to a date lands on that date", func(t *testing.T) {
		plan, err := services.PlanApproval(models.AccessRequest{Kind: services.RequestKindExtend, DurationDays: days(90), AskedUntil: &termEnd}, held, now)
		if err != nil || plan.ExpiresAt == nil || !plan.ExpiresAt.Equal(termEnd) || plan.Effect != services.EffectExtend {
			t.Fatalf("want extend to %v, got %v/%v (%v)", termEnd, plan.Effect, plan.ExpiresAt, err)
		}
	})

	t.Run("a date that has passed is refused, for either kind", func(t *testing.T) {
		for _, kind := range []string{services.RequestKindAccess, services.RequestKindExtend} {
			h := none
			if kind == services.RequestKindExtend {
				h = held
			}
			if _, err := services.PlanApproval(models.AccessRequest{Kind: kind, DurationDays: days(30), AskedUntil: &gone}, h, now); !errors.Is(err, services.ErrAskEnded) {
				t.Fatalf("%s: want services.ErrAskEnded, got %v", kind, err)
			}
		}
	})

	t.Run("a days ask is unchanged: it counts from the decision", func(t *testing.T) {
		plan, err := services.PlanApproval(models.AccessRequest{Kind: services.RequestKindAccess, DurationDays: days(30)}, none, now)
		if want := clock.EndAfterDays(now, 30); err != nil || plan.ExpiresAt == nil || !plan.ExpiresAt.Equal(want) {
			t.Fatalf("want %v, got %v (%v)", want, plan.ExpiresAt, err)
		}
	})
}

func ptrTime(t time.Time) *time.Time { return &t }

// An ask sent as UTC midnight (the old date picker) still means the whole of
// that day in the makerspace's zone, not until 05:30 IST.
func TestAskedEndIsTheWholeOfItsDay(t *testing.T) {
	sent := time.Date(2026, 11, 30, 0, 0, 0, 0, time.UTC) // 05:30 IST on 30 Nov
	want, _ := clock.EndOfDay("2026-11-30")
	if got := services.AskedEnd(&sent); !got.Equal(want) {
		t.Fatalf("services.AskedEnd(UTC midnight 30 Nov) = %v, want %v", got, want)
	}
	if !want.Equal(time.Date(2026, 11, 30, 18, 29, 59, 0, time.UTC)) {
		t.Fatalf("the end of 30 Nov is 23:59:59 IST, got %v", want)
	}
}

// The planner's refusal for an ask whose end passed is said in plain words, as a 409 END_PASSED
// (the dates themselves: TestPlanApproval_AskedUntil, and services' tests).
func TestApprovingAfterTheAskedEndIsRefused(t *testing.T) {
	resetAccessDeps(t)
	probe := stubDecide(t)
	probe.DecideErr = services.ErrAskEnded

	req := httptest.NewRequest(http.MethodPost, "/api/v1/requests/r1/decision",
		strings.NewReader(`{"status":"approved","reviewer_id":"op-2","review_note":"x"}`))
	req.SetPathValue("id", "r1")
	rr := httptest.NewRecorder()
	handleResolveAccessRequest(rr, req)

	if rr.Code != http.StatusConflict || !strings.Contains(rr.Body.String(), "END_PASSED") {
		t.Fatalf("want 409 END_PASSED, got %d %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "has already passed") {
		t.Fatalf("the refusal should say why in plain words: %s", rr.Body.String())
	}
	probe.wantOrder(t, "decide")
}
