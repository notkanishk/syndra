package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"mkauth/internal/auth"
	"mkauth/internal/models"
	"mkauth/internal/services"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

func resetVaultHandlerDeps(t *testing.T) {
	t.Helper()
	origSet := svcSetShadowPassword
	origClear := svcClearShadowPassword
	origHas := dbHasShadowCredential
	origGet := dbGetShadowCredential
	origAudit := dbGetShadowCredentialAudit
	t.Cleanup(func() {
		svcSetShadowPassword = origSet
		svcClearShadowPassword = origClear
		dbHasShadowCredential = origHas
		dbGetShadowCredential = origGet
		dbGetShadowCredentialAudit = origAudit
	})
}

func setupNoopVaultDeps(t *testing.T) {
	t.Helper()
	resetVaultHandlerDeps(t)
	svcSetShadowPassword = func(_ context.Context, _, _, _, _ string) error { return nil }
	svcClearShadowPassword = func(_ context.Context, _, _, _ string) error { return nil }
	dbHasShadowCredential = func(_ context.Context, _ string) (models.ShadowCredentialStatus, error) {
		return models.ShadowCredentialStatus{HasCredential: false}, nil
	}
	dbGetShadowCredential = func(_ context.Context, _ string) (models.ShadowCredential, error) {
		return models.ShadowCredential{}, fmt.Errorf("get shadow credential: %w", pgx.ErrNoRows)
	}
	dbGetShadowCredentialAudit = func(_ context.Context, _ string) ([]models.ShadowCredentialAudit, error) {
		return nil, nil
	}
}

// ---------------------------------------------------------------------------
// handleSetShadowCredential tests
// ---------------------------------------------------------------------------

func TestHandleSetShadowCredential_Success(t *testing.T) {
	setupNoopVaultDeps(t)

	body := `{"password":"Str0ng!Pass99"}`
	// Dev mode (no JWT actor) — explicit ?actor= supplies audit attribution.
	req := httptest.NewRequest(http.MethodPut, "/api/v1/users/u1/shadow-credential?actor=u1", strings.NewReader(body))
	req.SetPathValue("uid", "u1")
	rr := httptest.NewRecorder()
	handleSetShadowCredential(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleSetShadowCredential_SelfOnly_Forbidden(t *testing.T) {
	setupNoopVaultDeps(t)

	body := `{"password":"Str0ng!Pass99"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/users/u1/shadow-credential", strings.NewReader(body))
	req.SetPathValue("uid", "u1")
	// Set a different actor in context.
	ctx := withPrincipal(req.Context(), &auth.Principal{Subject: "other-user"})
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handleSetShadowCredential(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleSetShadowCredential_ValidationError(t *testing.T) {
	setupNoopVaultDeps(t)

	svcSetShadowPassword = func(_ context.Context, _, _, _, _ string) error {
		return fmt.Errorf("%w: must be at least 12 characters", services.ErrComplexity)
	}

	body := `{"password":"weak"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/users/u1/shadow-credential?actor=u1", strings.NewReader(body))
	req.SetPathValue("uid", "u1")
	rr := httptest.NewRecorder()
	handleSetShadowCredential(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleSetShadowCredential_MissingPassword(t *testing.T) {
	setupNoopVaultDeps(t)

	body := `{"password":""}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/users/u1/shadow-credential?actor=u1", strings.NewReader(body))
	req.SetPathValue("uid", "u1")
	rr := httptest.NewRecorder()
	handleSetShadowCredential(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

// ---------------------------------------------------------------------------
// handleClearShadowCredential tests
// ---------------------------------------------------------------------------

func TestHandleClearShadowCredential_Success(t *testing.T) {
	setupNoopVaultDeps(t)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/users/u1/shadow-credential?actor=u1", nil)
	req.SetPathValue("uid", "u1")
	rr := httptest.NewRecorder()
	handleClearShadowCredential(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

// ---------------------------------------------------------------------------
// handleGetShadowCredentialStatus tests
// ---------------------------------------------------------------------------

func TestHandleGetShadowCredentialStatus_HasCredential(t *testing.T) {
	setupNoopVaultDeps(t)

	now := time.Now()
	dbHasShadowCredential = func(_ context.Context, _ string) (models.ShadowCredentialStatus, error) {
		return models.ShadowCredentialStatus{
			HasCredential: true,
			Algorithm:     "argon2id",
			CreatedAt:     &now,
			UpdatedAt:     &now,
		}, nil
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/u1/shadow-credential/status", nil)
	req.SetPathValue("uid", "u1")
	rr := httptest.NewRecorder()
	handleGetShadowCredentialStatus(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var status models.ShadowCredentialStatus
	if err := json.Unmarshal(rr.Body.Bytes(), &status); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !status.HasCredential {
		t.Error("expected has_credential=true")
	}
}

func TestHandleGetShadowCredentialStatus_NoCredential(t *testing.T) {
	setupNoopVaultDeps(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/u1/shadow-credential/status", nil)
	req.SetPathValue("uid", "u1")
	rr := httptest.NewRecorder()
	handleGetShadowCredentialStatus(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var status models.ShadowCredentialStatus
	if err := json.Unmarshal(rr.Body.Bytes(), &status); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if status.HasCredential {
		t.Error("expected has_credential=false")
	}
}

// ---------------------------------------------------------------------------
// handleGetShadowCredentialHash tests
// ---------------------------------------------------------------------------

func TestHandleGetShadowCredentialHash_Success(t *testing.T) {
	setupNoopVaultDeps(t)

	dbGetShadowCredential = func(_ context.Context, _ string) (models.ShadowCredential, error) {
		return models.ShadowCredential{
			ID:             "cred-1",
			CredentialHash: "$argon2id$v=19$m=65536,t=1,p=4$fakesalt$fakekey",
			Algorithm:      "argon2id",
		}, nil
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/shadow-credentials/u1/hash", nil)
	req.SetPathValue("uid", "u1")
	rr := httptest.NewRecorder()
	handleGetShadowCredentialHash(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["algorithm"] != "argon2id" {
		t.Errorf("expected algorithm=argon2id, got %s", resp["algorithm"])
	}
	if resp["credential_hash"] == "" {
		t.Error("expected non-empty credential_hash")
	}
}

func TestHandleGetShadowCredentialHash_NotFound(t *testing.T) {
	setupNoopVaultDeps(t)
	// Default mock already returns pgx.ErrNoRows-wrapped error.

	req := httptest.NewRequest(http.MethodGet, "/api/v1/shadow-credentials/u1/hash", nil)
	req.SetPathValue("uid", "u1")
	rr := httptest.NewRecorder()
	handleGetShadowCredentialHash(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleGetShadowCredentialHash_DBError(t *testing.T) {
	setupNoopVaultDeps(t)

	dbGetShadowCredential = func(_ context.Context, _ string) (models.ShadowCredential, error) {
		return models.ShadowCredential{}, fmt.Errorf("connection refused")
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/shadow-credentials/u1/hash", nil)
	req.SetPathValue("uid", "u1")
	rr := httptest.NewRecorder()
	handleGetShadowCredentialHash(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleClearShadowCredential_DBError(t *testing.T) {
	setupNoopVaultDeps(t)

	svcClearShadowPassword = func(_ context.Context, _, _, _ string) error {
		return fmt.Errorf("connection refused")
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/users/u1/shadow-credential?actor=u1", nil)
	req.SetPathValue("uid", "u1")
	rr := httptest.NewRecorder()
	handleClearShadowCredential(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rr.Code, rr.Body.String())
	}
}

// ---------------------------------------------------------------------------
// handleGetShadowCredentialAudit tests
// ---------------------------------------------------------------------------

func TestHandleGetShadowCredentialAudit_Success(t *testing.T) {
	setupNoopVaultDeps(t)

	dbGetShadowCredentialAudit = func(_ context.Context, _ string) ([]models.ShadowCredentialAudit, error) {
		return []models.ShadowCredentialAudit{
			{ID: "a1", UserID: "u1", Action: "set", ActorID: "u1", CreatedAt: time.Now()},
		}, nil
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/u1/shadow-credential/audit", nil)
	req.SetPathValue("uid", "u1")
	rr := httptest.NewRecorder()
	handleGetShadowCredentialAudit(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var entries []models.ShadowCredentialAudit
	if err := json.Unmarshal(rr.Body.Bytes(), &entries); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(entries))
	}
}

// ---------------------------------------------------------------------------
// Dev-mode actor requirement (audit C3)
// ---------------------------------------------------------------------------

func TestHandleSetShadowCredential_DevModeRequiresActor(t *testing.T) {
	setupNoopVaultDeps(t)

	body := `{"password":"Str0ng!Pass99"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/users/u1/shadow-credential", strings.NewReader(body))
	req.SetPathValue("uid", "u1")
	// No actor in JWT context (dev mode), no ?actor= query param.
	rr := httptest.NewRecorder()

	handleSetShadowCredential(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("dev-mode mutation without ?actor= must return 400, got %d (body=%s)", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "MISSING_ACTOR") {
		t.Fatalf("expected MISSING_ACTOR in body, got %s", rr.Body.String())
	}
}

func TestHandleSetShadowCredential_DevModeWithExplicitActor(t *testing.T) {
	setupNoopVaultDeps(t)

	receivedActor := ""
	svcSetShadowPassword = func(_ context.Context, uid, actorID, _, _ string) error {
		if uid != "u1" {
			t.Fatalf("expected uid u1, got %q", uid)
		}
		receivedActor = actorID
		return nil
	}

	body := `{"password":"Str0ng!Pass99"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/users/u1/shadow-credential?actor=alice@cli", strings.NewReader(body))
	req.SetPathValue("uid", "u1")
	rr := httptest.NewRecorder()

	handleSetShadowCredential(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 with explicit actor, got %d: %s", rr.Code, rr.Body.String())
	}
	if receivedActor != "alice@cli" {
		t.Fatalf("expected actor=alice@cli, got %q", receivedActor)
	}
}

func TestHandleGetShadowCredentialStatus_DevModeNoActorRequired(t *testing.T) {
	// Reads are NOT affected by the new requirement — operators inspecting
	// status in dev mode would otherwise need a meaningless ?actor=.
	setupNoopVaultDeps(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/u1/shadow-credential/status", nil)
	req.SetPathValue("uid", "u1")
	rr := httptest.NewRecorder()

	handleGetShadowCredentialStatus(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("dev-mode read must not require ?actor=, got %d: %s", rr.Code, rr.Body.String())
	}
}
