package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"syndra/internal/models"
	"syndra/internal/services"
	"syndra/internal/services/propagation"
)

// resetRemoveCascadeDeps captures and restores the revoke-side cascade injectables (Task 21).
func resetRemoveCascadeDeps(t *testing.T) {
	t.Helper()
	origBundleRemoved := svcCascadeBundleRemoved
	origEdit := svcEditBundleWorkingCopy
	origDraft := svcBundleDraft
	t.Cleanup(func() {
		svcCascadeBundleRemoved = origBundleRemoved
		svcEditBundleWorkingCopy = origEdit
		svcBundleDraft = origDraft
	})
	svcBundleDraft = func(context.Context, string) (services.DraftDiff, error) {
		return services.DraftDiff{LatestVersion: 2, NextVersion: 3}, nil
	}
}

// --- handleRemoveBundleFromUser ---

func TestHandleRemoveBundleFromUser_EmptyPathParams(t *testing.T) {
	resetRemoveCascadeDeps(t)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/users//bundles/", nil)
	req.SetPathValue("id", "")
	req.SetPathValue("bundleId", "")
	rr := httptest.NewRecorder()
	handleRemoveBundleFromUser(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["error"] != "VALIDATION_FAILED" {
		t.Fatalf("expected VALIDATION_FAILED, got %v", resp["error"])
	}
}

func TestHandleRemoveBundleFromUser_DeletesThenCascades(t *testing.T) {
	resetRemoveCascadeDeps(t)

	var gotActor, gotUserID, gotBundleID string
	svcCascadeBundleRemoved = func(ctx context.Context, actor, userID, bundleID string) (services.CascadeResult, error) {
		gotActor, gotUserID, gotBundleID = actor, userID, bundleID
		return services.CascadeResult{Enqueued: 1, Drain: propagation.DrainResult{Applied: 1}}, nil
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/users/u1/bundles/b1", nil)
	req.SetPathValue("id", "u1")
	req.SetPathValue("bundleId", "b1")
	rr := httptest.NewRecorder()
	handleRemoveBundleFromUser(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if gotActor != "system" || gotUserID != "u1" || gotBundleID != "b1" {
		t.Fatalf("cascade called with actor=%q userID=%q bundleID=%q", gotActor, gotUserID, gotBundleID)
	}
	var resp map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["message"] != "Bundle removed from user" {
		t.Fatalf("unexpected message: %v", resp["message"])
	}
	if resp["cascade"] == nil {
		t.Fatal("expected a cascade field in the response")
	}
}

func TestHandleRemoveBundleFromUser_CoveredRoleYieldsZeroEnqueued(t *testing.T) {
	resetRemoveCascadeDeps(t)

	svcCascadeBundleRemoved = func(ctx context.Context, actor, userID, bundleID string) (services.CascadeResult, error) {
		return services.CascadeResult{Enqueued: 0}, nil
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/users/u1/bundles/b1", nil)
	req.SetPathValue("id", "u1")
	req.SetPathValue("bundleId", "b1")
	rr := httptest.NewRecorder()
	handleRemoveBundleFromUser(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Cascade services.CascadeResult `json:"cascade"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Cascade.Enqueued != 0 {
		t.Fatalf("expected cascade.enqueued=0, got %d", resp.Cascade.Enqueued)
	}
}

func TestHandleRemoveBundleFromUser_CascadeErrorIs500(t *testing.T) {
	resetRemoveCascadeDeps(t)

	svcCascadeBundleRemoved = func(ctx context.Context, actor, userID, bundleID string) (services.CascadeResult, error) {
		return services.CascadeResult{}, errors.New("remove tx failed")
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/users/u1/bundles/b1", nil)
	req.SetPathValue("id", "u1")
	req.SetPathValue("bundleId", "b1")
	rr := httptest.NewRecorder()
	handleRemoveBundleFromUser(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["error"] != "CASCADE_ERROR" {
		t.Fatalf("expected CASCADE_ERROR, got %v", resp["error"])
	}
}

// --- handleRemoveRoleFromBundle ---

func TestHandleRemoveRoleFromBundle_EmptyPathParams(t *testing.T) {
	resetRemoveCascadeDeps(t)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/bundles/b1/roles//", nil)
	req.SetPathValue("id", "b1")
	req.SetPathValue("projectId", "")
	req.SetPathValue("roleKey", "")
	rr := httptest.NewRecorder()
	handleRemoveRoleFromBundle(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["error"] != "VALIDATION_FAILED" {
		t.Fatalf("expected VALIDATION_FAILED, got %v", resp["error"])
	}
}

// Removing a role edits the working copy. The holders keep it until a version
// that lacks it is published and they are moved onto it — which is the whole
// reason removing a role from a bundle is no longer a frightening click.
func TestHandleRemoveRoleFromBundle_EditsTheWorkingCopyAndCascadesToNobody(t *testing.T) {
	resetRemoveCascadeDeps(t)

	var gotBundleID, gotProjectID, gotRoleKey string
	var gotAdd = true
	svcEditBundleWorkingCopy = func(ctx context.Context, actor, bundleID, projectID, roleKey string, add bool) error {
		gotBundleID, gotProjectID, gotRoleKey, gotAdd = bundleID, projectID, roleKey, add
		return nil
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/bundles/b1/roles/p1/admin", nil)
	req.SetPathValue("id", "b1")
	req.SetPathValue("projectId", "p1")
	req.SetPathValue("roleKey", "admin")
	rr := httptest.NewRecorder()
	handleRemoveRoleFromBundle(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if gotBundleID != "b1" || gotProjectID != "p1" || gotRoleKey != "admin" || gotAdd {
		t.Fatalf("edit called with bundleID=%q projectID=%q roleKey=%q add=%v", gotBundleID, gotProjectID, gotRoleKey, gotAdd)
	}
	var resp map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["cascade"] != nil {
		t.Fatal("a working-copy edit must not report a cascade")
	}
	if resp["draft"] == nil {
		t.Fatal("expected the draft in the response")
	}
}

func TestHandleRemoveRoleFromBundle_WriteErrorIs500(t *testing.T) {
	resetRemoveCascadeDeps(t)

	svcEditBundleWorkingCopy = func(ctx context.Context, actor, bundleID, projectID, roleKey string, add bool) error {
		return errors.New("remove tx failed")
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/bundles/b1/roles/p1/admin", nil)
	req.SetPathValue("id", "b1")
	req.SetPathValue("projectId", "p1")
	req.SetPathValue("roleKey", "admin")
	rr := httptest.NewRecorder()
	handleRemoveRoleFromBundle(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["error"] != "DB_ERROR" {
		t.Fatalf("expected DB_ERROR, got %v", resp["error"])
	}
}

// --- handleGetCascadeGroups ---

// C6 — an audit row's trace link lands here with `?cascade=<id>`. The filter has to be answered
// by the query, not by trimming the glance list afterwards: the audit tail is walkable back to
// the first day, so a trace from an event older than the 50 most recent cascades would otherwise
// arrive at a page saying nothing happened.
func TestHandleGetCascadeGroups_PassesTheCascadeFilterThrough(t *testing.T) {
	orig := dbGetCascadeGroups
	t.Cleanup(func() { dbGetCascadeGroups = orig })

	var gotLimit int
	var gotCascade string
	dbGetCascadeGroups = func(_ context.Context, limit int, cascadeID string) ([]models.CascadeGroup, error) {
		gotLimit, gotCascade = limit, cascadeID
		return []models.CascadeGroup{{CascadeID: cascadeID, Applied: 3}}, nil
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/propagations/cascade-groups?cascade=%20c-9%20", nil)
	rr := httptest.NewRecorder()
	handleGetCascadeGroups(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if gotCascade != "c-9" {
		t.Errorf("expected the trimmed cascade id to reach the query, got %q", gotCascade)
	}
	if gotLimit != cascadeGroupsLimit {
		t.Errorf("expected limit %d, got %d", cascadeGroupsLimit, gotLimit)
	}
}

// No parameter is the glance list, unchanged.
func TestHandleGetCascadeGroups_UnfilteredByDefault(t *testing.T) {
	orig := dbGetCascadeGroups
	t.Cleanup(func() { dbGetCascadeGroups = orig })

	var gotCascade = "sentinel"
	dbGetCascadeGroups = func(_ context.Context, _ int, cascadeID string) ([]models.CascadeGroup, error) {
		gotCascade = cascadeID
		return nil, nil
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/propagations/cascade-groups", nil)
	rr := httptest.NewRecorder()
	handleGetCascadeGroups(rr, req)

	if gotCascade != "" {
		t.Errorf("expected no cascade filter, got %q", gotCascade)
	}
	// nil groups must serialise as [] — a console rendering `null` as an error state would
	// report "couldn't load" for an org that has simply never cascaded anything.
	if !strings.Contains(rr.Body.String(), `"cascades":[]`) {
		t.Errorf("expected an empty array, got %s", rr.Body.String())
	}
}
