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
)

func days(n int) *int { return &n }

// The rules of the two kinds, as plain data (owner, 16 Oct 2026).
func TestPlanApproval_KindForKind(t *testing.T) {
	now := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	soon := now.Add(5 * 24 * time.Hour)
	far := now.Add(400 * 24 * time.Hour)
	none := models.RequestHolding{Kind: models.HoldingNone}
	until := models.RequestHolding{Kind: models.HoldingUntil, Until: &soon}
	untilFar := models.RequestHolding{Kind: models.HoldingUntil, Until: &far}
	forGood := models.RequestHolding{Kind: models.HoldingForGood}

	for _, tc := range []struct {
		name    string
		kind    string
		held    models.RequestHolding
		days    *int
		effect  approvalEffect
		endRule db.EndRule
		err     error
	}{
		{"an ask for a role they don't have gives it", requestKindAccess, none, days(30), effectGive, db.EndKept, nil},
		{"an ask for a role they have is refused", requestKindAccess, until, days(30), "", "", errRequestAlreadyHeld},
		{"keeping it longer moves the end", requestKindExtend, until, days(30), effectExtend, db.EndSet, nil},
		{"keeping it with no end moves the end to none", requestKindExtend, until, nil, effectExtend, db.EndSet, nil},
		{"keeping it longer when it lapsed meanwhile gives it back", requestKindExtend, none, days(30), effectGiveAgain, db.EndKept, nil},
		{"keeping it longer when it now has no end changes nothing", requestKindExtend, forGood, days(30), "", "", errRequestAlreadyHeld},
		{"keeping it to an earlier end would shorten it", requestKindExtend, untilFar, days(30), "", "", errExtensionShortens},
	} {
		t.Run(tc.name, func(t *testing.T) {
			plan, err := planApproval(models.AccessRequest{Kind: tc.kind, DurationDays: tc.days}, tc.held, now)
			if !errors.Is(err, tc.err) {
				t.Fatalf("err: want %v, got %v", tc.err, err)
			}
			if err != nil {
				return
			}
			if plan.Effect != tc.effect || plan.EndRule != tc.endRule {
				t.Fatalf("want %s/%q, got %s/%q", tc.effect, tc.endRule, plan.Effect, plan.EndRule)
			}
			if (tc.days == nil) != (plan.ExpiresAt == nil) {
				t.Fatalf("end: want set=%v, got %v", tc.days != nil, plan.ExpiresAt)
			}
		})
	}
}

// Asking to keep it longer is filed as an extension, and refused when there
// is nothing to extend or it would shorten what they have.
func TestHandleCreateAccessRequest_AskToKeepItLonger(t *testing.T) {
	soon := time.Now().Add(5 * 24 * time.Hour)
	far := time.Now().Add(400 * 24 * time.Hour)
	for _, tc := range []struct {
		name   string
		held   models.RequestHolding
		body   string
		status int
		code   string
	}{
		{"filed as an extension", models.RequestHolding{Kind: models.HoldingUntil, Until: &soon},
			`{"requester_id":"u1","project_id":"p","role_key":"r","justification":"still training","duration_days":30,"kind":"extend"}`, http.StatusCreated, ""},
		{"nothing held", models.RequestHolding{Kind: models.HoldingNone},
			`{"requester_id":"u1","project_id":"p","role_key":"r","justification":"x","duration_days":30,"kind":"extend"}`, http.StatusConflict, "NOTHING_TO_EXTEND"},
		{"held with no end", models.RequestHolding{Kind: models.HoldingForGood},
			`{"requester_id":"u1","project_id":"p","role_key":"r","justification":"x","duration_days":30,"kind":"extend"}`, http.StatusConflict, "NOTHING_TO_EXTEND"},
		{"would shorten it", models.RequestHolding{Kind: models.HoldingUntil, Until: &far},
			`{"requester_id":"u1","project_id":"p","role_key":"r","justification":"x","duration_days":30,"kind":"extend"}`, http.StatusConflict, "ALREADY_RUNS_PAST"},
		{"an ordinary ask for a role that ends points to keeping it longer", models.RequestHolding{Kind: models.HoldingUntil, Until: &soon},
			`{"requester_id":"u1","project_id":"p","role_key":"r","justification":"x"}`, http.StatusConflict, "ALREADY_HELD"},
		{"an unknown kind", models.RequestHolding{Kind: models.HoldingNone},
			`{"requester_id":"u1","project_id":"p","role_key":"r","justification":"x","kind":"upgrade"}`, http.StatusBadRequest, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resetAccessDeps(t)
			dbRequestHolding = func(context.Context, string, string, string) (models.RequestHolding, error) { return tc.held, nil }
			dbInsertAuditLog = func(context.Context, string, string, string, string) error { return nil }
			var filed string
			dbCreateAccessRequestOfKind = func(_ context.Context, kind string, _, _, _, _ string, _ *int) (string, error) {
				filed = kind
				return "req-1", nil
			}
			rr := httptest.NewRecorder()
			handleCreateAccessRequest(rr, httptest.NewRequest(http.MethodPost, "/api/v1/requests", strings.NewReader(tc.body)))
			if rr.Code != tc.status {
				t.Fatalf("want %d, got %d: %s", tc.status, rr.Code, rr.Body.String())
			}
			if tc.code != "" && !strings.Contains(rr.Body.String(), tc.code) {
				t.Fatalf("want %s, got %s", tc.code, rr.Body.String())
			}
			if tc.status == http.StatusCreated && filed != requestKindExtend {
				t.Fatalf("filed as %q, want extend", filed)
			}
			if tc.code == "ALREADY_HELD" && !strings.Contains(rr.Body.String(), "Ask to keep it longer instead") {
				t.Fatalf("an ask for a role that ends should point to keeping it longer: %s", rr.Body.String())
			}
		})
	}
}
