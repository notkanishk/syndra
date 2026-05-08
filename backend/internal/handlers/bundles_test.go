package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"mkauth/internal/models"
)

// resetBundleDeps captures and restores all bundle-related injectable vars.
func resetBundleDeps(t *testing.T) {
	t.Helper()
	origCreate := dbCreateBundle
	origGetAll := dbGetAllBundles
	origGetRoles := dbGetRolesForBundle
	origAddRole := dbAddRoleToBundle
	origGetUserBundles := dbGetBundlesForUser
	origAssign := dbAssignBundleToUser
	origSetWelcome := dbSetWelcomeBundle
	origAudit := dbInsertAuditLog
	t.Cleanup(func() {
		dbCreateBundle = origCreate
		dbGetAllBundles = origGetAll
		dbGetRolesForBundle = origGetRoles
		dbAddRoleToBundle = origAddRole
		dbGetBundlesForUser = origGetUserBundles
		dbAssignBundleToUser = origAssign
		dbSetWelcomeBundle = origSetWelcome
		dbInsertAuditLog = origAudit
	})
}

// --- handleCreateBundle ---

func TestHandleCreateBundle_EmptyName(t *testing.T) {
	resetBundleDeps(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/bundles", strings.NewReader(`{"name":""}`))
	rr := httptest.NewRecorder()
	handleCreateBundle(rr, req)

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
	details, ok := resp["details"].(map[string]interface{})
	if !ok || details["name"] != "required" {
		t.Fatalf("expected details.name=required, got %v", resp["details"])
	}
}

func TestHandleCreateBundle_WhitespaceName(t *testing.T) {
	resetBundleDeps(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/bundles", strings.NewReader(`{"name":"   "}`))
	rr := httptest.NewRecorder()
	handleCreateBundle(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
	var resp map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["error"] != "VALIDATION_FAILED" {
		t.Fatalf("expected VALIDATION_FAILED, got %v", resp["error"])
	}
}

func TestHandleCreateBundle_UnknownField(t *testing.T) {
	resetBundleDeps(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/bundles", strings.NewReader(`{"name":"Engineering","extra":"y"}`))
	rr := httptest.NewRecorder()
	handleCreateBundle(rr, req)

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

func TestHandleCreateBundle_HappyPath(t *testing.T) {
	resetBundleDeps(t)

	dbCreateBundle = func(ctx context.Context, name, description string) (string, error) {
		return "bundle-1", nil
	}
	auditAction := ""
	dbInsertAuditLog = func(ctx context.Context, actorID, targetID, action, resourceID string) error {
		auditAction = action
		return nil
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/bundles", strings.NewReader(`{"name":"Engineering","description":"Eng team bundle"}`))
	rr := httptest.NewRecorder()
	handleCreateBundle(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["id"] != "bundle-1" {
		t.Fatalf("expected id=bundle-1, got %v", resp["id"])
	}
	if auditAction != "bundle.created" {
		t.Fatalf("expected audit action bundle.created, got %s", auditAction)
	}
}

// --- handleAddRoleToBundle ---

func TestHandleAddRoleToBundle_EmptyRoleKey(t *testing.T) {
	resetBundleDeps(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/bundles/b1/roles", strings.NewReader(`{"project_id":"p1","role_key":""}`))
	req.SetPathValue("id", "b1")
	rr := httptest.NewRecorder()
	handleAddRoleToBundle(rr, req)

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

func TestHandleAddRoleToBundle_UnknownField(t *testing.T) {
	resetBundleDeps(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/bundles/b1/roles", strings.NewReader(`{"project_id":"p1","role_key":"admin","extra":"z"}`))
	req.SetPathValue("id", "b1")
	rr := httptest.NewRecorder()
	handleAddRoleToBundle(rr, req)

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

func TestHandleAddRoleToBundle_HappyPath(t *testing.T) {
	resetBundleDeps(t)

	dbAddRoleToBundle = func(ctx context.Context, bundleID, projectID, roleKey string) error {
		return nil
	}
	auditAction := ""
	dbInsertAuditLog = func(ctx context.Context, actorID, targetID, action, resourceID string) error {
		auditAction = action
		return nil
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/bundles/b1/roles", strings.NewReader(`{"project_id":"p1","role_key":"member"}`))
	req.SetPathValue("id", "b1")
	rr := httptest.NewRecorder()
	handleAddRoleToBundle(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["message"] != "Role added to bundle" {
		t.Fatalf("unexpected message: %s", resp["message"])
	}
	if auditAction != "bundle.role_added" {
		t.Fatalf("expected audit action bundle.role_added, got %s", auditAction)
	}
}

func TestHandleAddRoleToBundle_DBError(t *testing.T) {
	resetBundleDeps(t)

	dbAddRoleToBundle = func(ctx context.Context, bundleID, projectID, roleKey string) error {
		return errors.New("violates foreign key constraint")
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/bundles/b1/roles", strings.NewReader(`{"project_id":"p1","role_key":"admin"}`))
	req.SetPathValue("id", "b1")
	rr := httptest.NewRecorder()
	handleAddRoleToBundle(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}
	var resp map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["error"] != "DB_ERROR" {
		t.Fatalf("expected DB_ERROR, got %v", resp["error"])
	}
}

// --- handleAssignBundleToUser ---

func TestHandleAssignBundleToUser_UnknownField(t *testing.T) {
	resetBundleDeps(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/u1/bundles", strings.NewReader(`{"bundle_id":"b1","extra":"z"}`))
	req.SetPathValue("id", "u1")
	rr := httptest.NewRecorder()
	handleAssignBundleToUser(rr, req)

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

func TestHandleAssignBundleToUser_EmptyBundleID(t *testing.T) {
	resetBundleDeps(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/u1/bundles", strings.NewReader(`{"bundle_id":""}`))
	req.SetPathValue("id", "u1")
	rr := httptest.NewRecorder()
	handleAssignBundleToUser(rr, req)

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

func TestHandleAssignBundleToUser_Idempotent(t *testing.T) {
	resetBundleDeps(t)

	// Simulate ON CONFLICT DO NOTHING — always succeeds, second call is a no-op.
	dbAssignBundleToUser = func(ctx context.Context, userID, bundleID string) error {
		return nil
	}
	dbInsertAuditLog = func(ctx context.Context, actorID, targetID, action, resourceID string) error {
		return nil
	}

	call := func() int {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/users/u1/bundles", strings.NewReader(`{"bundle_id":"b1"}`))
		req.SetPathValue("id", "u1")
		rr := httptest.NewRecorder()
		handleAssignBundleToUser(rr, req)
		return rr.Code
	}

	if got := call(); got != http.StatusOK {
		t.Fatalf("first call: expected 200, got %d", got)
	}
	if got := call(); got != http.StatusOK {
		t.Fatalf("second call: expected 200, got %d", got)
	}
}

func TestHandleAssignBundleToUser_AuditLogged(t *testing.T) {
	resetBundleDeps(t)

	dbAssignBundleToUser = func(ctx context.Context, userID, bundleID string) error {
		return nil
	}
	auditAction := ""
	auditTarget := ""
	dbInsertAuditLog = func(ctx context.Context, actorID, targetID, action, resourceID string) error {
		auditAction = action
		auditTarget = targetID
		return nil
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/u1/bundles", strings.NewReader(`{"bundle_id":"b2"}`))
	req.SetPathValue("id", "u1")
	rr := httptest.NewRecorder()
	handleAssignBundleToUser(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if auditAction != "bundle.assigned" {
		t.Fatalf("expected audit action bundle.assigned, got %s", auditAction)
	}
	if auditTarget != "u1" {
		t.Fatalf("expected audit target u1, got %s", auditTarget)
	}
}

// --- handleSetWelcomeBundle ---

func TestHandleSetWelcomeBundle_Success(t *testing.T) {
	resetBundleDeps(t)

	called := ""
	dbSetWelcomeBundle = func(_ context.Context, id string) error {
		called = id
		return nil
	}
	dbInsertAuditLog = func(_ context.Context, _, _, _, _ string) error { return nil }

	req := httptest.NewRequest(http.MethodPut, "/api/v1/bundles/b-123/welcome", nil)
	req.SetPathValue("id", "b-123")
	rr := httptest.NewRecorder()

	handleSetWelcomeBundle(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if called != "b-123" {
		t.Fatalf("expected SetWelcomeBundle called with b-123, got %q", called)
	}
}

func TestHandleSetWelcomeBundle_NotFound(t *testing.T) {
	resetBundleDeps(t)
	dbSetWelcomeBundle = func(_ context.Context, _ string) error {
		return pgx.ErrNoRows
	}

	req := httptest.NewRequest(http.MethodPut, "/api/v1/bundles/missing/welcome", nil)
	req.SetPathValue("id", "missing")
	rr := httptest.NewRecorder()

	handleSetWelcomeBundle(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

// --- handleGetBundles ---

func TestHandleGetBundles_NilSafe(t *testing.T) {
	resetBundleDeps(t)

	dbGetAllBundles = func(ctx context.Context) ([]models.Bundle, error) {
		return nil, nil
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/bundles", nil)
	rr := httptest.NewRecorder()
	handleGetBundles(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	body := rr.Body.String()
	if strings.TrimSpace(body) == "null" {
		t.Fatalf("expected [] not null for empty bundles")
	}
}
