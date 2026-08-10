package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"syndra/internal/models"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

func resetVaultHandlerDeps(t *testing.T) {
	t.Helper()
	origRecord := svcRecordCredentialSet
	origClear := svcClearShadowPassword
	origHas := dbHasShadowCredential
	origAudit := dbGetShadowCredentialAudit
	t.Cleanup(func() {
		svcRecordCredentialSet = origRecord
		svcClearShadowPassword = origClear
		dbHasShadowCredential = origHas
		dbGetShadowCredentialAudit = origAudit
	})
}

func setupNoopVaultDeps(t *testing.T) {
	t.Helper()
	resetVaultHandlerDeps(t)
	svcRecordCredentialSet = func(_ context.Context, _, _, _ string) error { return nil }
	svcClearShadowPassword = func(_ context.Context, _, _, _ string) error { return nil }
	dbHasShadowCredential = func(_ context.Context, _ string) (models.ShadowCredentialStatus, error) {
		return models.ShadowCredentialStatus{HasCredential: false}, nil
	}
	dbGetShadowCredentialAudit = func(_ context.Context, _ string) ([]models.ShadowCredentialAudit, error) {
		return nil, nil
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
