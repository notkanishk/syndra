package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"syndra/internal/services"
)

// Publishing a bundle version, and moving its holders, join the plan gate.
//
// These two were the last rehearsed mutations in the product that issued no
// approval, and the symptom was not subtle: the shared dialog disables Apply
// without a `plan_id`, so every publish that reached anybody could be previewed
// and never applied.

func stubPublishRehearsal(t *testing.T, plan services.BulkPlan, draft services.DraftDiff) {
	t.Helper()
	orig := svcRehearsePublish
	t.Cleanup(func() { svcRehearsePublish = orig })
	svcRehearsePublish = func(context.Context, services.PublishRequest) (services.BulkPlan, services.DraftDiff, error) {
		return plan, draft, nil
	}
}
func publishRequest(body string, apply bool) *http.Request {
	path := "/api/v1/bundles/b1/publish"
	if apply {
		path += "?apply=true"
	}
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.SetPathValue("id", "b1")
	return req
}

// The bug an operator hit. A publish that reaches somebody has to come back
// with the id its apply will cite; without one the button is disabled forever.
func TestPublishRehearsal_IssuesTheApprovalItsApplyMustCite(t *testing.T) {
	stubPlanStore(t)
	stubPublishRehearsal(t, services.BulkPlan{
		Op: "publish_bundle_version",
		Outcomes: []services.BulkOutcome{
			{UserID: "u1", Effect: services.EffectApply, Fingerprint: "fp-u1"},
		},
		RequestFingerprint: "req-fp",
	}, services.DraftDiff{LatestVersion: 1, NextVersion: 2})

	rr := httptest.NewRecorder()
	handlePublishBundleVersion(rr, publishRequest(`{"note":"","migrate":true}`, false))

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp struct{ Plan services.BulkPlan }
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Plan.PlanID == "" {
		t.Fatal("the rehearsal returned no plan_id, so the dialog can never enable Apply")
	}
}
