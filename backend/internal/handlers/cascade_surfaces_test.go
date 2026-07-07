package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"mkauth/internal/models"
	"mkauth/internal/services"
	"mkauth/internal/services/propagation"
)

// resetRemoveCascadeDeps captures and restores the revoke-side cascade injectables (Task 21).
func resetRemoveCascadeDeps(t *testing.T) {
	t.Helper()
	origBundleRemoved := svcCascadeBundleRemoved
	origRoleRemoved := svcCascadeRoleRemoved
	t.Cleanup(func() {
		svcCascadeBundleRemoved = origBundleRemoved
		svcCascadeRoleRemoved = origRoleRemoved
	})
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
		return services.CascadeResult{Enqueued: 1, Mode: "auto", Drain: propagation.DrainResult{Applied: 1}}, nil
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
		return services.CascadeResult{Enqueued: 0, Mode: "auto"}, nil
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

func TestHandleRemoveRoleFromBundle_DeletesThenCascades(t *testing.T) {
	resetRemoveCascadeDeps(t)

	var gotBundleID, gotProjectID, gotRoleKey string
	svcCascadeRoleRemoved = func(ctx context.Context, actor, bundleID, projectID, roleKey string) (services.CascadeResult, error) {
		gotBundleID, gotProjectID, gotRoleKey = bundleID, projectID, roleKey
		return services.CascadeResult{Enqueued: 2, Mode: "auto"}, nil
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
	if gotBundleID != "b1" || gotProjectID != "p1" || gotRoleKey != "admin" {
		t.Fatalf("cascade called with bundleID=%q projectID=%q roleKey=%q", gotBundleID, gotProjectID, gotRoleKey)
	}
	var resp map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["message"] != "Role removed from bundle" {
		t.Fatalf("unexpected message: %v", resp["message"])
	}
}

func TestHandleRemoveRoleFromBundle_CoveredMembersYieldZeroEnqueued(t *testing.T) {
	resetRemoveCascadeDeps(t)

	svcCascadeRoleRemoved = func(ctx context.Context, actor, bundleID, projectID, roleKey string) (services.CascadeResult, error) {
		return services.CascadeResult{Enqueued: 0, Mode: "manual"}, nil
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

func TestHandleRemoveRoleFromBundle_CascadeErrorIs500(t *testing.T) {
	resetRemoveCascadeDeps(t)

	svcCascadeRoleRemoved = func(ctx context.Context, actor, bundleID, projectID, roleKey string) (services.CascadeResult, error) {
		return services.CascadeResult{}, errors.New("remove tx failed")
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
	if resp["error"] != "CASCADE_ERROR" {
		t.Fatalf("expected CASCADE_ERROR, got %v", resp["error"])
	}
}

// resetConfirmationModeDeps captures and restores the Task-22 confirmation-mode injectables.
func resetConfirmationModeDeps(t *testing.T) {
	t.Helper()
	origGetConfig := dbGetConfigSetting
	origSetConfig := dbSetConfigSetting
	origSetRuleMode := dbSetRuleConfirmationMode
	origSetBundleMode := dbSetBundleConfirmationMode
	origGetRecent := dbGetRecentCascades
	t.Cleanup(func() {
		dbGetConfigSetting = origGetConfig
		dbSetConfigSetting = origSetConfig
		dbSetRuleConfirmationMode = origSetRuleMode
		dbSetBundleConfirmationMode = origSetBundleMode
		dbGetRecentCascades = origGetRecent
	})
}

// --- handleGetGlobalConfirmationDefault ---

func TestHandleGetGlobalConfirmationDefault_ReturnsSeededAuto(t *testing.T) {
	resetConfirmationModeDeps(t)

	dbGetConfigSetting = func(ctx context.Context, key string) (string, error) {
		if key != "global.default_rule_confirmation_mode" {
			t.Fatalf("unexpected config key %q", key)
		}
		return "auto", nil
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/config/confirmation-mode-default", nil)
	rr := httptest.NewRecorder()
	handleGetGlobalConfirmationDefault(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["mode"] != "auto" {
		t.Fatalf("expected mode=auto, got %v", resp["mode"])
	}
}

func TestHandleGetGlobalConfirmationDefault_UnsetFallsBackToAuto(t *testing.T) {
	resetConfirmationModeDeps(t)

	dbGetConfigSetting = func(ctx context.Context, key string) (string, error) { return "", nil }

	req := httptest.NewRequest(http.MethodGet, "/api/v1/config/confirmation-mode-default", nil)
	rr := httptest.NewRecorder()
	handleGetGlobalConfirmationDefault(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["mode"] != "auto" {
		t.Fatalf("expected mode=auto fallback, got %v", resp["mode"])
	}
}

func TestHandleGetGlobalConfirmationDefault_DBErrorIs500(t *testing.T) {
	resetConfirmationModeDeps(t)

	dbGetConfigSetting = func(ctx context.Context, key string) (string, error) {
		return "", errors.New("db unreachable")
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/config/confirmation-mode-default", nil)
	rr := httptest.NewRecorder()
	handleGetGlobalConfirmationDefault(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rr.Code, rr.Body.String())
	}
}

// --- handleSetGlobalConfirmationDefault ---

func TestHandleSetGlobalConfirmationDefault_Persists(t *testing.T) {
	resetConfirmationModeDeps(t)

	var gotKey, gotValue string
	dbSetConfigSetting = func(ctx context.Context, key, value, updatedBy string) error {
		gotKey, gotValue = key, value
		return nil
	}

	req := httptest.NewRequest(http.MethodPut, "/api/v1/config/confirmation-mode-default", strings.NewReader(`{"mode":"manual"}`))
	rr := httptest.NewRecorder()
	handleSetGlobalConfirmationDefault(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if gotKey != "global.default_rule_confirmation_mode" || gotValue != "manual" {
		t.Fatalf("expected persisted key/value global.default_rule_confirmation_mode/manual, got %s/%s", gotKey, gotValue)
	}
	var resp map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["mode"] != "manual" {
		t.Fatalf("expected mode=manual, got %v", resp["mode"])
	}
}

// TestHandleSetGlobalConfirmationDefault_UnknownField is Minor Fix #5: this endpoint uses
// decodeJSONStrict same as the create-bundle/create-rule endpoints, so an unrecognized field
// must 400, mirroring TestHandleCreateBundle_UnknownField's convention.
func TestHandleSetGlobalConfirmationDefault_UnknownField(t *testing.T) {
	resetConfirmationModeDeps(t)

	called := false
	dbSetConfigSetting = func(ctx context.Context, key, value, updatedBy string) error {
		called = true
		return nil
	}

	req := httptest.NewRequest(http.MethodPut, "/api/v1/config/confirmation-mode-default", strings.NewReader(`{"mode":"manual","extra":"z"}`))
	rr := httptest.NewRecorder()
	handleSetGlobalConfirmationDefault(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
	if called {
		t.Fatal("dbSetConfigSetting must not be called when the body has an unknown field")
	}
}

func TestHandleSetGlobalConfirmationDefault_InvalidMode(t *testing.T) {
	resetConfirmationModeDeps(t)

	called := false
	dbSetConfigSetting = func(ctx context.Context, key, value, updatedBy string) error {
		called = true
		return nil
	}

	req := httptest.NewRequest(http.MethodPut, "/api/v1/config/confirmation-mode-default", strings.NewReader(`{"mode":"sometimes"}`))
	rr := httptest.NewRecorder()
	handleSetGlobalConfirmationDefault(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
	if called {
		t.Fatal("dbSetConfigSetting must not be called for an invalid mode")
	}
}

// --- handleBulkSetConfirmationMode ---

func TestHandleBulkSetConfirmationMode_UpdatesSelectedRuleIDs(t *testing.T) {
	resetConfirmationModeDeps(t)

	var gotIDs []string
	var gotMode string
	dbSetRuleConfirmationMode = func(ctx context.Context, ids []string, mode string) error {
		gotIDs, gotMode = ids, mode
		return nil
	}
	dbSetBundleConfirmationMode = func(ctx context.Context, ids []string, mode string) error {
		t.Fatal("bundle setter must not be called for kind=rule")
		return nil
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/policies/confirmation-mode",
		strings.NewReader(`{"kind":"rule","ids":["r1","r2"],"mode":"manual"}`))
	rr := httptest.NewRecorder()
	handleBulkSetConfirmationMode(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if len(gotIDs) != 2 || gotIDs[0] != "r1" || gotIDs[1] != "r2" || gotMode != "manual" {
		t.Fatalf("expected ids=[r1 r2] mode=manual, got ids=%v mode=%s", gotIDs, gotMode)
	}
}

func TestHandleBulkSetConfirmationMode_UpdatesSelectedBundleIDs(t *testing.T) {
	resetConfirmationModeDeps(t)

	var gotIDs []string
	dbSetRuleConfirmationMode = func(ctx context.Context, ids []string, mode string) error {
		t.Fatal("rule setter must not be called for kind=bundle")
		return nil
	}
	dbSetBundleConfirmationMode = func(ctx context.Context, ids []string, mode string) error {
		gotIDs = ids
		return nil
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/policies/confirmation-mode",
		strings.NewReader(`{"kind":"bundle","ids":["b1"],"mode":"auto"}`))
	rr := httptest.NewRecorder()
	handleBulkSetConfirmationMode(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if len(gotIDs) != 1 || gotIDs[0] != "b1" {
		t.Fatalf("expected ids=[b1], got %v", gotIDs)
	}
}

// TestHandleBulkSetConfirmationMode_UnknownField is Minor Fix #5: same decodeJSONStrict
// convention as the other cascade-surface endpoints.
func TestHandleBulkSetConfirmationMode_UnknownField(t *testing.T) {
	resetConfirmationModeDeps(t)

	called := false
	dbSetRuleConfirmationMode = func(ctx context.Context, ids []string, mode string) error {
		called = true
		return nil
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/policies/confirmation-mode",
		strings.NewReader(`{"kind":"rule","ids":["r1"],"mode":"auto","extra":"z"}`))
	rr := httptest.NewRecorder()
	handleBulkSetConfirmationMode(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
	if called {
		t.Fatal("dbSetRuleConfirmationMode must not be called when the body has an unknown field")
	}
}

func TestHandleBulkSetConfirmationMode_InvalidKind(t *testing.T) {
	resetConfirmationModeDeps(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/policies/confirmation-mode",
		strings.NewReader(`{"kind":"user","ids":["u1"],"mode":"auto"}`))
	rr := httptest.NewRecorder()
	handleBulkSetConfirmationMode(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleBulkSetConfirmationMode_EmptyIDs(t *testing.T) {
	resetConfirmationModeDeps(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/policies/confirmation-mode",
		strings.NewReader(`{"kind":"rule","ids":[],"mode":"auto"}`))
	rr := httptest.NewRecorder()
	handleBulkSetConfirmationMode(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleBulkSetConfirmationMode_DBErrorIs500(t *testing.T) {
	resetConfirmationModeDeps(t)

	dbSetRuleConfirmationMode = func(ctx context.Context, ids []string, mode string) error {
		return errors.New("update failed")
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/policies/confirmation-mode",
		strings.NewReader(`{"kind":"rule","ids":["r1"],"mode":"auto"}`))
	rr := httptest.NewRecorder()
	handleBulkSetConfirmationMode(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rr.Code, rr.Body.String())
	}
}

// --- handleGetRecentCascades ---

func TestHandleGetRecentCascades_ReturnsRows(t *testing.T) {
	resetConfirmationModeDeps(t)

	var gotLimit int
	dbGetRecentCascades = func(ctx context.Context, limit int) ([]models.CascadeSummary, error) {
		gotLimit = limit
		return []models.CascadeSummary{
			{ID: "c1", OpType: "add", UserID: "u1", ProjectID: "p1", RoleKeys: []string{"role1"},
				Source: "rule", SourceRef: "rule-1", Status: "applied"},
		}, nil
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/propagations/cascades", nil)
	rr := httptest.NewRecorder()
	handleGetRecentCascades(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if gotLimit != recentCascadesLimit {
		t.Fatalf("expected limit=%d, got %d", recentCascadesLimit, gotLimit)
	}
	var resp struct {
		Cascades []models.CascadeSummary `json:"cascades"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp.Cascades) != 1 || resp.Cascades[0].ID != "c1" {
		t.Fatalf("expected one cascade row with id=c1, got %v", resp.Cascades)
	}
}

func TestHandleGetRecentCascades_NilRowsEncodeAsEmptyArray(t *testing.T) {
	resetConfirmationModeDeps(t)

	dbGetRecentCascades = func(ctx context.Context, limit int) ([]models.CascadeSummary, error) {
		return nil, nil
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/propagations/cascades", nil)
	rr := httptest.NewRecorder()
	handleGetRecentCascades(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"cascades":[]`) {
		t.Fatalf("expected cascades:[] for nil rows, got %s", rr.Body.String())
	}
}

func TestHandleGetRecentCascades_DBErrorIs500(t *testing.T) {
	resetConfirmationModeDeps(t)

	dbGetRecentCascades = func(ctx context.Context, limit int) ([]models.CascadeSummary, error) {
		return nil, errors.New("db unreachable")
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/propagations/cascades", nil)
	rr := httptest.NewRecorder()
	handleGetRecentCascades(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rr.Code, rr.Body.String())
	}
}

// --- resolveConfirmationMode ---

func TestResolveConfirmationMode_OverridePreferredOverDefault(t *testing.T) {
	resetConfirmationModeDeps(t)

	dbGetConfigSetting = func(ctx context.Context, key string) (string, error) {
		t.Fatal("dbGetConfigSetting must not be called when a reqMode override is supplied")
		return "", nil
	}

	mode, err := resolveConfirmationMode(context.Background(), "manual")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mode != "manual" {
		t.Fatalf("expected mode=manual, got %s", mode)
	}
}

// TestResolveConfirmationMode_TrimsBeforeNormalize is Minor Fix #3: trimmedNonEmpty gates on
// the trimmed value, but the untrimmed value used to reach NormalizeConfirmationMode — so
// " manual" (leading space) fell through to the "auto" default instead of resolving to manual.
func TestResolveConfirmationMode_TrimsBeforeNormalize(t *testing.T) {
	resetConfirmationModeDeps(t)

	dbGetConfigSetting = func(ctx context.Context, key string) (string, error) {
		t.Fatal("dbGetConfigSetting must not be called when a reqMode override is supplied")
		return "", nil
	}

	mode, err := resolveConfirmationMode(context.Background(), " manual")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mode != "manual" {
		t.Fatalf("expected untrimmed ' manual' to resolve to manual, got %q", mode)
	}
}

func TestResolveConfirmationMode_FallsBackToGlobalDefault(t *testing.T) {
	resetConfirmationModeDeps(t)

	dbGetConfigSetting = func(ctx context.Context, key string) (string, error) { return "manual", nil }

	mode, err := resolveConfirmationMode(context.Background(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mode != "manual" {
		t.Fatalf("expected mode=manual from configured default, got %s", mode)
	}
}
