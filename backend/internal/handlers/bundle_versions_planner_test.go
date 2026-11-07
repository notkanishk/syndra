package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"syndra/internal/db"
	"syndra/internal/models"
	"syndra/internal/services"
)

// Publish and move apply through the planner (task 1.3): the handler hands the
// request and its citation to services.ApplyPlannedBulk, which claims the plan,
// re-reads the version and the cohort under the lock and writes only the
// reviewed Change. These pin the hand-off and the answers.

type plannedCall struct {
	ran  bool
	req  services.BulkRequest
	cite db.PlanCitation
}

func stubCohortApply(t *testing.T, plan services.BulkPlan, err error) *plannedCall {
	t.Helper()
	orig, origRebuild := svcApplyPlannedBulk, rebuildUserCacheDetachedFn
	t.Cleanup(func() { svcApplyPlannedBulk, rebuildUserCacheDetachedFn = orig, origRebuild })
	rebuildUserCacheDetachedFn = func(context.Context, string) {}
	call := &plannedCall{}
	svcApplyPlannedBulk = func(_ context.Context, req services.BulkRequest, c db.PlanCitation) (services.BulkPlan, map[string]services.Change, error) {
		call.ran, call.req, call.cite = true, req, c
		return plan, map[string]services.Change{}, err
	}
	return call
}

func TestPublishApply_GoesThroughThePlannerCitingThePlan(t *testing.T) {
	call := stubCohortApply(t, services.BulkPlan{Op: services.OpPublish, Applied: true, Outcomes: []services.BulkOutcome{},
		Version: models.BundleVersion{ID: "v-new", Version: 3}}, nil)
	rr := httptest.NewRecorder()
	handlePublishBundleVersion(rr, publishRequest(
		`{"note":"term start","migrate":true,"includers":[{"bundle_id":"staff","action":"leave"}],"plan_id":"plan-1"}`, true))

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if call.req.Op != services.OpPublish || call.req.BundleID != "b1" || !call.req.Migrate || call.req.Note != "term start" ||
		len(call.req.Includers) != 1 {
		t.Fatalf("the publish reached the planner as %+v", call.req)
	}
	if call.cite.PlanID != "plan-1" || call.cite.Surface != planSurfaceBundlePublish {
		t.Fatalf("the citation must be this screen's plan: %+v", call.cite)
	}
	var resp struct {
		Version models.BundleVersion `json:"version"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil || resp.Version.Version != 3 {
		t.Fatalf("the answer carries the version cut: %+v (%v)", resp, err)
	}
}

// A bundle nobody holds rehearses to nobody, so there is no approval to cite;
// the planner publishes it if, under the lock, nobody holds it still.
func TestPublishApply_WithNoCitationTheServiceDecides(t *testing.T) {
	call := stubCohortApply(t, services.BulkPlan{Op: services.OpPublish, Outcomes: []services.BulkOutcome{}}, nil)
	rr := httptest.NewRecorder()
	handlePublishBundleVersion(rr, publishRequest(`{"note":"","migrate":true}`, true))
	if rr.Code != http.StatusOK || !call.ran || call.cite.PlanID != "" {
		t.Fatalf("expected the planner to be asked with no plan, got %d %+v", rr.Code, call.cite)
	}
}

func TestPublishApply_Refusals(t *testing.T) {
	for _, tc := range []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{"somebody holds it and nothing was cited", services.ErrPlanRequired, http.StatusBadRequest, "PLAN_REQUIRED"},
		{"the plan moved", &services.StalePlan{IDs: []string{"u1"}}, http.StatusConflict, "PLAN_STALE"},
		{"the cohort or contents moved", db.ErrPlanRequestMismatch, http.StatusConflict, "PLAN_REQUEST_MISMATCH"},
		{"another screen's plan", db.ErrPlanNotCitableHere, http.StatusConflict, "PLAN_NOT_CITABLE_HERE"},
		{"nothing to publish", errors.New("nothing to publish: the bundle matches v2"), http.StatusConflict, "NOTHING_TO_PUBLISH"},
		{"an including bundle unanswered", &services.ErrIncludersUnanswered{Names: []string{"Staff"}}, http.StatusConflict, "INCLUDERS_UNANSWERED"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stubCohortApply(t, services.BulkPlan{}, tc.err)
			rr := httptest.NewRecorder()
			handlePublishBundleVersion(rr, publishRequest(`{"note":"","migrate":true,"plan_id":"plan-1"}`, true))
			var body struct{ Error, Message string }
			_ = json.NewDecoder(rr.Body).Decode(&body)
			if rr.Code != tc.status || body.Error != tc.code {
				t.Fatalf("got %d %s, want %d %s", rr.Code, body.Error, tc.status, tc.code)
			}
			if tc.status == http.StatusConflict && tc.code != "PLAN_STALE" && tc.code != "PLAN_REQUEST_MISMATCH" &&
				tc.code != "PLAN_NOT_CITABLE_HERE" && !strings.Contains(body.Message, "Nothing was changed.") {
				t.Fatalf("a refusal says nothing changed: %q", body.Message)
			}
		})
	}
}

func TestMoveHoldersApply_WithoutACitationIsRefused(t *testing.T) {
	call := stubCohortApply(t, services.BulkPlan{}, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/bundles/b1/holders/move?apply=true",
		strings.NewReader(`{"version_id":"v2","user_ids":["u1"]}`))
	req.SetPathValue("id", "b1")
	rr := httptest.NewRecorder()
	handleMoveBundleHolders(rr, req)
	if rr.Code != http.StatusBadRequest || call.ran {
		t.Fatalf("expected 400 with nothing applied, got %d (ran %v)", rr.Code, call.ran)
	}
}

// An approval from the publish screen is not spendable on the move screen:
// the move cites its own surface, which the plan store checks.
func TestMoveHoldersApply_CitesTheMoveSurface(t *testing.T) {
	call := stubCohortApply(t, services.BulkPlan{Op: services.OpMove, Outcomes: []services.BulkOutcome{}}, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/bundles/b1/holders/move?apply=true",
		strings.NewReader(`{"version_id":"v2","user_ids":["u1"],"plan_id":"plan-1"}`))
	req.SetPathValue("id", "b1")
	rr := httptest.NewRecorder()
	handleMoveBundleHolders(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if call.req.Op != services.OpMove || call.req.VersionID != "v2" || len(call.req.UserIDs) != 1 ||
		call.cite.Surface != planSurfaceBundleMove || call.cite.PlanID != "plan-1" {
		t.Fatalf("the move reached the planner as %+v cited %+v", call.req, call.cite)
	}
}
