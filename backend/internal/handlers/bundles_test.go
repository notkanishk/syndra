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
	"github.com/jackc/pgx/v5/pgconn"

	"mkauth/internal/models"
	"mkauth/internal/services"
)

// resetBundleDeps captures and restores all bundle-related injectable vars.
func resetBundleDeps(t *testing.T) {
	t.Helper()
	origCreate := dbCreateBundle
	origGetAll := dbGetAllBundles
	origGetRoles := dbGetRolesForBundle
	origGetUserBundles := dbGetBundlesForUser
	origSetWelcome := dbSetWelcomeBundle
	origUpdate := dbUpdateBundle
	origGetByID := dbGetBundleByID
	origCascadeDelete := svcCascadeBundleDeleted
	origAudit := dbInsertAuditLog
	origGetConfig := dbGetConfigSetting
	// Default: no configured global default (resolveConfirmationMode normalizes "" to "auto") —
	// keeps every existing test that doesn't set confirmation_mode from hitting a real DB call.
	dbGetConfigSetting = func(ctx context.Context, key string) (string, error) { return "", nil }
	t.Cleanup(func() {
		dbCreateBundle = origCreate
		dbGetAllBundles = origGetAll
		dbGetRolesForBundle = origGetRoles
		dbGetBundlesForUser = origGetUserBundles
		dbSetWelcomeBundle = origSetWelcome
		dbUpdateBundle = origUpdate
		dbGetBundleByID = origGetByID
		svcCascadeBundleDeleted = origCascadeDelete
		dbInsertAuditLog = origAudit
		dbGetConfigSetting = origGetConfig
	})
}

// resetCascadeHandlerDeps captures and restores the bundle-side add-cascade injectables
// (sub-phase 3, Task 20).
func resetCascadeHandlerDeps(t *testing.T) {
	t.Helper()
	origAssigned := svcCascadeBundleAssigned
	origEdit := svcEditBundleWorkingCopy
	origDraft := svcBundleDraft
	t.Cleanup(func() {
		svcCascadeBundleAssigned = origAssigned
		svcEditBundleWorkingCopy = origEdit
		svcBundleDraft = origDraft
	})
	svcBundleDraft = func(context.Context, string) (services.DraftDiff, error) {
		return services.DraftDiff{LatestVersion: 2, NextVersion: 3}, nil
	}
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

	dbCreateBundle = func(ctx context.Context, name, description, confirmationMode string) (string, error) {
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

// TestHandleCreateBundle_ResolvedModeReachesCreate is Minor Fix #5: the RESOLVED confirmation
// mode (not the raw request body) must be the value that actually reaches dbCreateBundle.
func TestHandleCreateBundle_ResolvedModeReachesCreate(t *testing.T) {
	resetBundleDeps(t)

	var gotMode string
	dbCreateBundle = func(ctx context.Context, name, description, confirmationMode string) (string, error) {
		gotMode = confirmationMode
		return "bundle-1", nil
	}
	dbInsertAuditLog = func(ctx context.Context, actorID, targetID, action, resourceID string) error { return nil }

	req := httptest.NewRequest(http.MethodPost, "/api/v1/bundles", strings.NewReader(`{"name":"Engineering","confirmation_mode":"manual"}`))
	rr := httptest.NewRecorder()
	handleCreateBundle(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	if gotMode != "manual" {
		t.Fatalf("expected resolved mode 'manual' to reach dbCreateBundle, got %q", gotMode)
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

// Adding a role writes the WORKING COPY and reaches nobody. The response says
// so and hands back the draft, because the next question an operator has is
// "what will publishing this do".
func TestHandleAddRoleToBundle_EditsTheWorkingCopyAndCascadesToNobody(t *testing.T) {
	resetBundleDeps(t)
	resetCascadeHandlerDeps(t)

	var gotBundleID, gotProjectID, gotRoleKey string
	var gotAdd bool
	svcEditBundleWorkingCopy = func(ctx context.Context, actor, bundleID, projectID, roleKey string, add bool) error {
		gotBundleID, gotProjectID, gotRoleKey, gotAdd = bundleID, projectID, roleKey, add
		return nil
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/bundles/b1/roles", strings.NewReader(`{"project_id":"p1","role_key":"member"}`))
	req.SetPathValue("id", "b1")
	rr := httptest.NewRecorder()
	handleAddRoleToBundle(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if gotBundleID != "b1" || gotProjectID != "p1" || gotRoleKey != "member" || !gotAdd {
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

func TestHandleAddRoleToBundle_WriteError(t *testing.T) {
	resetBundleDeps(t)
	resetCascadeHandlerDeps(t)

	svcEditBundleWorkingCopy = func(ctx context.Context, actor, bundleID, projectID, roleKey string, add bool) error {
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
	resetCascadeHandlerDeps(t)

	// The atomic AssignBundleAndEnqueue is itself ON CONFLICT DO NOTHING — the cascade stub
	// always succeeds, second call is a no-op from the handler's perspective.
	svcCascadeBundleAssigned = func(ctx context.Context, actor, userID, bundleID string) (services.CascadeResult, error) {
		return services.CascadeResult{Mode: "auto"}, nil
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

// TestHandleAssignBundleToUser_CallsCascadeWithActorAndTarget asserts the handler resolves the
// actor (falling back to "system" in dev-mode API-key auth, since no principal is stashed in
// this test's request context) and passes userID/bundleID straight through to the cascade. Audit
// logging now happens inside the atomic db.AssignBundleAndEnqueue tx (not a separate handler-level
// dbInsertAuditLog call), so it is no longer observable at this layer — see
// db.TestCascadeAndEnqueue_WriteNoDirectRoleGrants and cascade.go's own audit insert for that.
func TestHandleAssignBundleToUser_CallsCascadeWithActorAndTarget(t *testing.T) {
	resetBundleDeps(t)
	resetCascadeHandlerDeps(t)

	var gotActor, gotUserID, gotBundleID string
	svcCascadeBundleAssigned = func(ctx context.Context, actor, userID, bundleID string) (services.CascadeResult, error) {
		gotActor, gotUserID, gotBundleID = actor, userID, bundleID
		return services.CascadeResult{Mode: "auto"}, nil
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/u1/bundles", strings.NewReader(`{"bundle_id":"b2"}`))
	req.SetPathValue("id", "u1")
	rr := httptest.NewRecorder()
	handleAssignBundleToUser(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if gotActor != "system" {
		t.Fatalf("expected actor to fall back to system, got %q", gotActor)
	}
	if gotUserID != "u1" || gotBundleID != "b2" {
		t.Fatalf("expected userID=u1 bundleID=b2, got userID=%q bundleID=%q", gotUserID, gotBundleID)
	}
}

func TestHandleAssignBundleToUser_CascadeErrorIs500(t *testing.T) {
	resetBundleDeps(t)
	resetCascadeHandlerDeps(t)

	svcCascadeBundleAssigned = func(ctx context.Context, actor, userID, bundleID string) (services.CascadeResult, error) {
		return services.CascadeResult{}, errors.New("assign tx failed")
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/u1/bundles", strings.NewReader(`{"bundle_id":"b2"}`))
	req.SetPathValue("id", "u1")
	rr := httptest.NewRecorder()
	handleAssignBundleToUser(rr, req)

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

// --- handleUpdateBundle ---

func TestHandleUpdateBundle_BlankNameRejected(t *testing.T) {
	resetBundleDeps(t)

	dbUpdateBundle = func(ctx context.Context, id, name, description string) error {
		t.Fatal("must not write a bundle with no name")
		return nil
	}

	req := httptest.NewRequest(http.MethodPut, "/api/v1/bundles/b1", strings.NewReader(`{"name":"   ","description":"x"}`))
	req.SetPathValue("id", "b1")
	rr := httptest.NewRecorder()
	handleUpdateBundle(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

// Bundle names are unique, and colliding with one is an ordinary thing an operator does — it
// must read as "that name is taken", not as a server fault.
func TestHandleUpdateBundle_DuplicateNameIs409(t *testing.T) {
	resetBundleDeps(t)

	dbUpdateBundle = func(ctx context.Context, id, name, description string) error {
		return &pgconn.PgError{Code: "23505"}
	}

	req := httptest.NewRequest(http.MethodPut, "/api/v1/bundles/b1", strings.NewReader(`{"name":"Lab Tech","description":""}`))
	req.SetPathValue("id", "b1")
	rr := httptest.NewRecorder()
	handleUpdateBundle(rr, req)

	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleUpdateBundle_UnknownIDIs404(t *testing.T) {
	resetBundleDeps(t)

	dbUpdateBundle = func(ctx context.Context, id, name, description string) error {
		return pgx.ErrNoRows
	}

	req := httptest.NewRequest(http.MethodPut, "/api/v1/bundles/nope", strings.NewReader(`{"name":"Lab Tech","description":""}`))
	req.SetPathValue("id", "nope")
	rr := httptest.NewRecorder()
	handleUpdateBundle(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

// A rename reaches nobody, so it must not run a cascade — and the trimmed name is what gets
// written, not the raw field.
func TestHandleUpdateBundle_TrimsAndDoesNotCascade(t *testing.T) {
	resetBundleDeps(t)

	var gotName, gotDesc string
	dbUpdateBundle = func(ctx context.Context, id, name, description string) error {
		gotName, gotDesc = name, description
		return nil
	}
	svcCascadeBundleDeleted = func(ctx context.Context, actor, bundleID string) (services.CascadeResult, error) {
		t.Fatal("a rename must not cascade")
		return services.CascadeResult{}, nil
	}
	var auditAction string
	dbInsertAuditLog = func(ctx context.Context, actorID, targetID, action, resourceID string) error {
		auditAction = action
		return nil
	}

	req := httptest.NewRequest(http.MethodPut, "/api/v1/bundles/b1", strings.NewReader(`{"name":"  Lab Tech  ","description":"Trained on the mill"}`))
	req.SetPathValue("id", "b1")
	rr := httptest.NewRecorder()
	handleUpdateBundle(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if gotName != "Lab Tech" || gotDesc != "Trained on the mill" {
		t.Fatalf("wrote name=%q desc=%q", gotName, gotDesc)
	}
	if auditAction != "bundle.updated" {
		t.Fatalf("audit action = %q, want bundle.updated", auditAction)
	}
}

// --- handleDeleteBundle ---

func TestHandleDeleteBundle_UnknownIDIs404(t *testing.T) {
	resetBundleDeps(t)

	dbGetBundleByID = func(ctx context.Context, id string) (models.Bundle, error) {
		return models.Bundle{}, pgx.ErrNoRows
	}
	svcCascadeBundleDeleted = func(ctx context.Context, actor, bundleID string) (services.CascadeResult, error) {
		t.Fatal("must not cascade for a bundle that does not exist")
		return services.CascadeResult{}, nil
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/bundles/nope", nil)
	req.SetPathValue("id", "nope")
	rr := httptest.NewRecorder()
	handleDeleteBundle(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

// Deleting the welcome bundle silently stops onboarding granting anything. The flag is read
// before the row goes and reported back, because afterwards there is nothing left to ask.
func TestHandleDeleteBundle_ReportsThatItWasTheWelcomeBundle(t *testing.T) {
	resetBundleDeps(t)

	dbGetBundleByID = func(ctx context.Context, id string) (models.Bundle, error) {
		return models.Bundle{ID: id, Name: "Newcomer", IsWelcome: true}, nil
	}
	svcCascadeBundleDeleted = func(ctx context.Context, actor, bundleID string) (services.CascadeResult, error) {
		return services.CascadeResult{Enqueued: 2, Mode: "auto"}, nil
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/bundles/b1", nil)
	req.SetPathValue("id", "b1")
	rr := httptest.NewRecorder()
	handleDeleteBundle(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		WasWelcome bool                   `json:"was_welcome"`
		Cascade    services.CascadeResult `json:"cascade"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if !resp.WasWelcome {
		t.Fatal("was_welcome must be true — new members stop receiving a welcome bundle")
	}
	if resp.Cascade.Enqueued != 2 {
		t.Fatalf("cascade not reported: %+v", resp.Cascade)
	}
}

func TestHandleDeleteBundle_OrdinaryBundleIsNotFlaggedAsWelcome(t *testing.T) {
	resetBundleDeps(t)

	dbGetBundleByID = func(ctx context.Context, id string) (models.Bundle, error) {
		return models.Bundle{ID: id, Name: "Lab Tech"}, nil
	}
	svcCascadeBundleDeleted = func(ctx context.Context, actor, bundleID string) (services.CascadeResult, error) {
		return services.CascadeResult{}, nil
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/bundles/b1", nil)
	req.SetPathValue("id", "b1")
	rr := httptest.NewRecorder()
	handleDeleteBundle(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if strings.Contains(rr.Body.String(), `"was_welcome":true`) {
		t.Fatalf("must not claim an ordinary bundle was the welcome bundle: %s", rr.Body.String())
	}
}
