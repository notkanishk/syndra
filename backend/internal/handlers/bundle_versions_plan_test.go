package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"syndra/internal/models"
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

// stubPublishApply records whether the write ran, so a refused citation can be
// told apart from a refused-and-written-anyway one.
func stubPublishApply(t *testing.T) *bool {
	t.Helper()
	orig := svcPublishBundleVersion
	t.Cleanup(func() { svcPublishBundleVersion = orig })
	ran := false
	svcPublishBundleVersion = func(context.Context, string, services.PublishRequest) (services.BulkPlan, models.BundleVersion, error) {
		ran = true
		return services.BulkPlan{Op: "publish_bundle_version", Applied: true}, models.BundleVersion{Version: 2}, nil
	}
	return &ran
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

func TestPublishApply_WithoutACitationIsRefusedAndWritesNothing(t *testing.T) {
	stubPlanStore(t)
	stubPublishRehearsal(t, services.BulkPlan{
		Op: "publish_bundle_version",
		Outcomes: []services.BulkOutcome{
			{UserID: "u1", Effect: services.EffectApply, Fingerprint: "fp-u1"},
		},
		RequestFingerprint: "req-fp",
	}, services.DraftDiff{})
	ran := stubPublishApply(t)

	rr := httptest.NewRecorder()
	handlePublishBundleVersion(rr, publishRequest(`{"note":"","migrate":true}`, true))

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
	if *ran {
		t.Fatal("the version was published without an approval")
	}
}

func TestPublishApply_CitingTheApprovalPublishes(t *testing.T) {
	stubPlanStore(t)
	stubPublishRehearsal(t, services.BulkPlan{
		Op: "publish_bundle_version",
		Outcomes: []services.BulkOutcome{
			{UserID: "u1", Effect: services.EffectApply, Fingerprint: "fp-u1"},
		},
		RequestFingerprint: "req-fp",
	}, services.DraftDiff{})
	ran := stubPublishApply(t)

	rehearse := httptest.NewRecorder()
	handlePublishBundleVersion(rehearse, publishRequest(`{"note":"","migrate":true}`, false))
	var issued struct{ Plan services.BulkPlan }
	if err := json.NewDecoder(rehearse.Body).Decode(&issued); err != nil {
		t.Fatalf("decode: %v", err)
	}

	rr := httptest.NewRecorder()
	handlePublishBundleVersion(rr, publishRequest(
		`{"note":"","migrate":true,"plan_id":"`+issued.Plan.PlanID+`"}`, true))

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if !*ran {
		t.Fatal("a cited approval must reach the write")
	}
}

// A bundle nobody holds rehearses to nobody, so there is no approval to issue
// and none to demand. Publishing it is still an act — it cuts the version that
// future assignments pin — and demanding a citation that cannot exist would be
// the same dead end from the other side.
func TestPublishApply_ABundleNobodyHoldsNeedsNoCitation(t *testing.T) {
	stubPlanStore(t)
	stubPublishRehearsal(t, services.BulkPlan{
		Op:                 "publish_bundle_version",
		Outcomes:           []services.BulkOutcome{},
		RequestFingerprint: "req-fp",
	}, services.DraftDiff{})
	ran := stubPublishApply(t)

	rr := httptest.NewRecorder()
	handlePublishBundleVersion(rr, publishRequest(`{"note":"first cut","migrate":false}`, true))

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if !*ran {
		t.Fatal("publishing a bundle nobody holds must still publish")
	}
}

// A holder whose reviewed delta moved invalidates the approval. Somebody
// gaining the role from a direct grant between the review and the apply turns
// "LOSES laser" into "no change", and the operator approved the first.
func TestPublishApply_AHolderWhoseDeltaMovedRefusesTheApproval(t *testing.T) {
	stubPlanStore(t)
	stubPublishRehearsal(t, services.BulkPlan{
		Op: "publish_bundle_version",
		Outcomes: []services.BulkOutcome{
			{UserID: "u1", Effect: services.EffectApply, Fingerprint: "fp-before"},
		},
		RequestFingerprint: "req-fp",
	}, services.DraftDiff{})

	rehearse := httptest.NewRecorder()
	handlePublishBundleVersion(rehearse, publishRequest(`{"note":"","migrate":true}`, false))
	var issued struct{ Plan services.BulkPlan }
	if err := json.NewDecoder(rehearse.Body).Decode(&issued); err != nil {
		t.Fatalf("decode: %v", err)
	}

	// The world moves.
	stubPublishRehearsal(t, services.BulkPlan{
		Op: "publish_bundle_version",
		Outcomes: []services.BulkOutcome{
			{UserID: "u1", Effect: services.EffectNoChange, Fingerprint: "fp-after"},
		},
		RequestFingerprint: "req-fp",
	}, services.DraftDiff{})
	ran := stubPublishApply(t)

	rr := httptest.NewRecorder()
	handlePublishBundleVersion(rr, publishRequest(
		`{"note":"","migrate":true,"plan_id":"`+issued.Plan.PlanID+`"}`, true))

	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409 PLAN_STALE, got %d: %s", rr.Code, rr.Body.String())
	}
	if *ran {
		t.Fatal("a stale approval must not reach the write")
	}
}

// The cohort is derived from the world, not named in the request, so a person
// assigned the bundle after the review is not a subject on the approval and no
// per-subject check will notice them. The request fingerprint is what does.
func TestPublishApply_ACohortThatGrewRefusesTheApproval(t *testing.T) {
	stubPlanStore(t)
	stubPublishRehearsal(t, services.BulkPlan{
		Op: "publish_bundle_version",
		Outcomes: []services.BulkOutcome{
			{UserID: "u1", Effect: services.EffectApply, Fingerprint: "fp-u1"},
		},
		RequestFingerprint: "one-holder",
	}, services.DraftDiff{})

	rehearse := httptest.NewRecorder()
	handlePublishBundleVersion(rehearse, publishRequest(`{"note":"","migrate":true}`, false))
	var issued struct{ Plan services.BulkPlan }
	if err := json.NewDecoder(rehearse.Body).Decode(&issued); err != nil {
		t.Fatalf("decode: %v", err)
	}

	// Somebody else is given the bundle. u1's own row is untouched.
	stubPublishRehearsal(t, services.BulkPlan{
		Op: "publish_bundle_version",
		Outcomes: []services.BulkOutcome{
			{UserID: "u1", Effect: services.EffectApply, Fingerprint: "fp-u1"},
			{UserID: "u2", Effect: services.EffectApply, Fingerprint: "fp-u2"},
		},
		RequestFingerprint: "two-holders",
	}, services.DraftDiff{})
	ran := stubPublishApply(t)

	rr := httptest.NewRecorder()
	handlePublishBundleVersion(rr, publishRequest(
		`{"note":"","migrate":true,"plan_id":"`+issued.Plan.PlanID+`"}`, true))

	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["error"] != "PLAN_REQUEST_MISMATCH" {
		t.Fatalf("a widened cohort must not pass as the same request, got %v", resp["error"])
	}
	if *ran {
		t.Fatal("the publish moved a holder the operator never reviewed")
	}
}

// An approval issued on the publish screen must not be spendable on the move
// screen: same bundle, same people, different act.
func TestPublishApproval_IsNotCitableOnTheMoveSurface(t *testing.T) {
	stubPlanStore(t)
	stubPublishRehearsal(t, services.BulkPlan{
		Op: "publish_bundle_version",
		Outcomes: []services.BulkOutcome{
			{UserID: "u1", Effect: services.EffectApply, Fingerprint: "fp-u1"},
		},
		RequestFingerprint: "shared-fp",
	}, services.DraftDiff{})

	rehearse := httptest.NewRecorder()
	handlePublishBundleVersion(rehearse, publishRequest(`{"note":"","migrate":true}`, false))
	var issued struct{ Plan services.BulkPlan }
	if err := json.NewDecoder(rehearse.Body).Decode(&issued); err != nil {
		t.Fatalf("decode: %v", err)
	}

	origRehearseMove, origMove := svcRehearseMoveHolders, svcMoveHolders
	t.Cleanup(func() { svcRehearseMoveHolders, svcMoveHolders = origRehearseMove, origMove })
	svcRehearseMoveHolders = func(context.Context, services.MoveHoldersRequest) (services.BulkPlan, error) {
		return services.BulkPlan{
			Op:                 "move_bundle_holders",
			Outcomes:           []services.BulkOutcome{{UserID: "u1", Effect: services.EffectApply, Fingerprint: "fp-u1"}},
			RequestFingerprint: "shared-fp",
		}, nil
	}
	moved := false
	svcMoveHolders = func(context.Context, string, services.MoveHoldersRequest) (services.BulkPlan, error) {
		moved = true
		return services.BulkPlan{}, nil
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/bundles/b1/holders/move?apply=true",
		strings.NewReader(`{"version_id":"v2","user_ids":["u1"],"plan_id":"`+issued.Plan.PlanID+`"}`))
	req.SetPathValue("id", "b1")
	rr := httptest.NewRecorder()
	handleMoveBundleHolders(rr, req)

	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409 PLAN_NOT_CITABLE_HERE, got %d: %s", rr.Code, rr.Body.String())
	}
	if moved {
		t.Fatal("an approval from another screen reached the write")
	}
}

func TestMoveHoldersApply_WithoutACitationIsRefused(t *testing.T) {
	stubPlanStore(t)

	origRehearse, origMove := svcRehearseMoveHolders, svcMoveHolders
	t.Cleanup(func() { svcRehearseMoveHolders, svcMoveHolders = origRehearse, origMove })
	svcRehearseMoveHolders = func(context.Context, services.MoveHoldersRequest) (services.BulkPlan, error) {
		return services.BulkPlan{
			Op:                 "move_bundle_holders",
			Outcomes:           []services.BulkOutcome{{UserID: "u1", Effect: services.EffectApply, Fingerprint: "fp-u1"}},
			RequestFingerprint: "req-fp",
		}, nil
	}
	moved := false
	svcMoveHolders = func(context.Context, string, services.MoveHoldersRequest) (services.BulkPlan, error) {
		moved = true
		return services.BulkPlan{}, nil
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/bundles/b1/holders/move?apply=true",
		strings.NewReader(`{"version_id":"v2","user_ids":["u1"]}`))
	req.SetPathValue("id", "b1")
	rr := httptest.NewRecorder()
	handleMoveBundleHolders(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
	if moved {
		t.Fatal("holders were repinned without an approval")
	}
}

func TestMoveHoldersRehearsal_IssuesTheApprovalItsApplyMustCite(t *testing.T) {
	stubPlanStore(t)

	origRehearse := svcRehearseMoveHolders
	t.Cleanup(func() { svcRehearseMoveHolders = origRehearse })
	svcRehearseMoveHolders = func(context.Context, services.MoveHoldersRequest) (services.BulkPlan, error) {
		return services.BulkPlan{
			Op:                 "move_bundle_holders",
			Outcomes:           []services.BulkOutcome{{UserID: "u1", Effect: services.EffectApply, Fingerprint: "fp-u1"}},
			RequestFingerprint: "req-fp",
		}, nil
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/bundles/b1/holders/move",
		strings.NewReader(`{"version_id":"v2","user_ids":["u1"]}`))
	req.SetPathValue("id", "b1")
	rr := httptest.NewRecorder()
	handleMoveBundleHolders(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var plan services.BulkPlan
	if err := json.NewDecoder(rr.Body).Decode(&plan); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if plan.PlanID == "" {
		t.Fatal("the rehearsal returned no plan_id, so the dialog can never enable Apply")
	}
}
