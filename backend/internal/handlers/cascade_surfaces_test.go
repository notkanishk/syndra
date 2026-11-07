package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"syndra/internal/models"
	"syndra/internal/services"
	"syndra/internal/services/propagation"
)

// resetRemoveCascadeDeps captures and restores the revoke-side injectables (Task 21). Removing a
// bundle from a person goes through svcApplyAct: see stubAct.
func resetRemoveCascadeDeps(t *testing.T) {
	t.Helper()
	origEdit := svcEditBundleWorkingCopy
	origDraft := svcBundleDraft
	t.Cleanup(func() {
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

func removeBundle(query string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/users/u1/bundles/b1"+query, nil)
	req.SetPathValue("id", "u1")
	req.SetPathValue("bundleId", "b1")
	rr := httptest.NewRecorder()
	handleRemoveBundleFromUser(rr, req)
	return rr
}

func TestHandleRemoveBundleFromUser_PlansTheRemovalThenRebuildsThenSends(t *testing.T) {
	resetRemoveCascadeDeps(t)
	probe := stubAct(t)
	probe.Change = services.Change{Writes: []services.Write{{OutboxID: "ob1"}}}
	probe.Delivery = services.Delivery{Sent: []string{"ob1"}, Drain: propagation.DrainResult{Applied: 1}}

	rr := removeBundle("")

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	a, w, seen := probe.only(t)
	if a != (services.Action{Op: services.BulkOpRemoveBundle, UserID: "u1", BundleID: "b1"}) || w.Actor != "system" || seen != nil {
		t.Fatalf("planned %+v %+v seen=%+v", a, w, seen)
	}
	probe.wantOrder(t, "apply", "rebuild", "deliver")
	if !probe.RebuildDetached {
		t.Fatal("the rebuild must run on a context the request cannot cancel")
	}
	var resp struct {
		Message string                 `json:"message"`
		Cascade services.CascadeResult `json:"cascade"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Message != "Bundle removed from user" || resp.Cascade.Enqueued != 1 || resp.Cascade.Drain.Applied != 1 {
		t.Fatalf("unexpected answer: %+v", resp)
	}
}

// ?fingerprint= and ?cancel_ids= on the DELETE are what the sheet was shown.
func TestHandleRemoveBundleFromUser_CarriesWhatTheSheetWasShownInTheQuery(t *testing.T) {
	resetRemoveCascadeDeps(t)
	probe := stubAct(t)

	const id = "22222222-2222-2222-2222-222222222222"
	if rr := removeBundle("?fingerprint=fp-3&cancel_ids=" + id); rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	_, _, seen := probe.only(t)
	if seen == nil || seen.Fingerprint != "fp-3" || !reflect.DeepEqual(seen.CancelIDs, []string{id}) {
		t.Fatalf("seen = %+v", seen)
	}
}

// A role the person still holds another way is not revoked: the write happened and changed
// nobody's effective access, which is not the same answer as a no-op.
func TestHandleRemoveBundleFromUser_CoveredRoleYieldsZeroEnqueued(t *testing.T) {
	resetRemoveCascadeDeps(t)
	stubAct(t).Change = services.Change{}

	rr := removeBundle("")

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Cascade services.CascadeResult `json:"cascade"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Cascade.Enqueued != 0 || resp.Cascade.NoOp {
		t.Fatalf("expected enqueued=0 and not a no-op, got %+v", resp.Cascade)
	}
}

// Not holding it is the planner's "not_held": 200, and it says nothing changed.
func TestHandleRemoveBundleFromUser_NotHeldIsAnIdempotentNoOp(t *testing.T) {
	resetRemoveCascadeDeps(t)
	probe := stubAct(t)
	probe.Err = &services.Refused{Change: services.Change{Refusal: "not_held"}}

	rr := removeBundle("")

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	resp := decodeMap(t, rr.Body.String())
	if c, _ := resp["cascade"].(map[string]any); c["no_op"] != true {
		t.Fatalf("expected cascade.no_op=true, got %v", resp["cascade"])
	}
	if !strings.Contains(resp["message"].(string), "Doesn't hold") {
		t.Fatalf("unexpected message: %v", resp["message"])
	}
	probe.wantOrder(t, "apply")
}

func TestHandleRemoveBundleFromUser_StaleIs409AndFailureIs500(t *testing.T) {
	resetRemoveCascadeDeps(t)
	probe := stubAct(t)

	probe.Err = &services.StalePlan{IDs: []string{"q1"}}
	wantError(t, removeBundle("?fingerprint=old"), http.StatusConflict, "PLAN_STALE")

	probe.Err = errors.New("remove tx failed")
	wantError(t, removeBundle(""), http.StatusInternalServerError, "DB_ERROR")
	probe.wantOrder(t, "apply", "apply")
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
