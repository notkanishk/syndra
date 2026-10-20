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
		{"keeping it counts from the end they have, so a far end stays extended", requestKindExtend, untilFar, days(30), effectExtend, db.EndSet, nil},
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
		{"counts from the end they have, however far", models.RequestHolding{Kind: models.HoldingUntil, Until: &far},
			`{"requester_id":"u1","project_id":"p","role_key":"r","justification":"x","duration_days":30,"kind":"extend"}`, http.StatusCreated, ""},
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
			dbCreateAccessRequestOfKind = func(_ context.Context, kind string, _, _, _, _ string, _ *int, _ *time.Time) (string, error) {
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

// "Another month" on a role that ends 18 Dec means 18 Jan: the extension counts
// from the end they have when that is still ahead, and from now when it isn't
// (A-04 §2).
func TestExtendCountsFromTheCurrentEnd(t *testing.T) {
	now := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)
	dec18 := time.Date(2026, 12, 18, 12, 0, 0, 0, time.UTC)
	past := now.Add(-48 * time.Hour)
	for _, tc := range []struct {
		name string
		held models.RequestHolding
		want time.Time
	}{
		{"future end: counts from it", models.RequestHolding{Kind: models.HoldingUntil, Until: &dec18}, dec18.Add(31 * 24 * time.Hour)},
		{"end already passed: counts from now", models.RequestHolding{Kind: models.HoldingUntil, Until: &past}, now.Add(31 * 24 * time.Hour)},
		{"lapsed and gone: given again from now", models.RequestHolding{Kind: models.HoldingNone}, now.Add(31 * 24 * time.Hour)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			plan, err := planApproval(models.AccessRequest{Kind: requestKindExtend, DurationDays: days(31)}, tc.held, now)
			if err != nil || plan.ExpiresAt == nil || !plan.ExpiresAt.Equal(tc.want) {
				t.Fatalf("want %v, got %v (%v)", tc.want, plan.ExpiresAt, err)
			}
		})
	}
	// Asking agrees with deciding.
	if err := checkRequestKind(requestKindExtend, models.RequestHolding{Kind: models.HoldingUntil, Until: &dec18}, days(31), nil, now); err != nil {
		t.Fatalf("asking: %v", err)
	}
}

// The rule that refuses a shortening still stands for any end that is not later.
func TestCheckExtension_RefusesAnEndThatIsNotLater(t *testing.T) {
	end := time.Date(2026, 12, 18, 0, 0, 0, 0, time.UTC)
	h := models.RequestHolding{Kind: models.HoldingUntil, Until: &end}
	if err := checkExtension(h, &end); !errors.Is(err, errExtensionShortens) {
		t.Fatalf("an equal end: got %v", err)
	}
}

// The queue shows the same end Approve will set (A-04 §2): each waiting
// extension carries the end they have and the end approving would give.
func TestWithHolding_ExtendCarriesCurrentAndNewEnd(t *testing.T) {
	resetAccessDeps(t)
	dec18 := time.Now().UTC().Add(60 * 24 * time.Hour).Truncate(time.Second)
	dbRequestHolding = func(context.Context, string, string, string) (models.RequestHolding, error) {
		return models.RequestHolding{Kind: models.HoldingUntil, Until: &dec18}, nil
	}
	out := withHolding(context.Background(), []models.AccessRequest{
		{ID: "a", Kind: requestKindExtend, Status: "pending", DurationDays: days(31)},
		{ID: "b", Kind: requestKindAccess, Status: "pending", DurationDays: days(31)},
		{ID: "c", Kind: requestKindExtend, Status: "approved", DurationDays: days(31)},
	})
	a := out[0]
	if a.CurrentUntil == nil || !a.CurrentUntil.Equal(dec18) {
		t.Fatalf("current_until: %v", a.CurrentUntil)
	}
	if a.Until == nil || !a.Until.Equal(dec18.Add(31*24*time.Hour)) {
		t.Fatalf("until: %v", a.Until)
	}
	if out[1].CurrentUntil != nil || out[1].Until != nil || out[2].Until != nil {
		t.Fatalf("only a waiting extension carries them: %+v %+v", out[1], out[2])
	}
}
