package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"mkauth/internal/models"
)

func resetIntentDeps(t *testing.T) {
	t.Helper()
	origGet := dbGetProvisioningIntents
	origClaim := dbClaimPendingIntents
	origComplete := dbCompleteIntent
	origFail := dbFailIntent
	t.Cleanup(func() {
		dbGetProvisioningIntents = origGet
		dbClaimPendingIntents = origClaim
		dbCompleteIntent = origComplete
		dbFailIntent = origFail
	})
}

func setupNoopIntentDeps(t *testing.T) {
	t.Helper()
	resetIntentDeps(t)
	dbGetProvisioningIntents = func(_ context.Context, _ string) ([]models.ProvisioningIntent, error) {
		return nil, nil
	}
	dbClaimPendingIntents = func(_ context.Context, _ int) ([]models.ProvisioningIntent, error) {
		return nil, nil
	}
	dbCompleteIntent = func(_ context.Context, _ string) error { return nil }
	dbFailIntent = func(_ context.Context, _, _ string) error { return nil }
}

func sampleIntent() models.ProvisioningIntent {
	now := time.Now()
	return models.ProvisioningIntent{
		ID:             "int-1",
		TargetUID:      "u1",
		Action:         "add",
		LLDAPGroup:     "printing_member",
		SourceProject:  "printing",
		SourceRole:     "member",
		IdempotencyKey: "add:u1:printing_member:evt-1",
		Status:         "acknowledged",
		CreatedAt:      now,
		AcknowledgedAt: &now,
	}
}

func TestHandleGetProvisioningIntents_Success(t *testing.T) {
	setupNoopIntentDeps(t)

	dbGetProvisioningIntents = func(_ context.Context, _ string) ([]models.ProvisioningIntent, error) {
		return []models.ProvisioningIntent{sampleIntent()}, nil
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/intents", nil)
	rr := httptest.NewRecorder()
	handleGetProvisioningIntents(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var intents []models.ProvisioningIntent
	if err := json.Unmarshal(rr.Body.Bytes(), &intents); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(intents) != 1 || intents[0].ID != "int-1" {
		t.Errorf("unexpected intents: %+v", intents)
	}
}

func TestHandleGetProvisioningIntents_StatusFilter(t *testing.T) {
	setupNoopIntentDeps(t)

	var capturedFilter string
	dbGetProvisioningIntents = func(_ context.Context, filter string) ([]models.ProvisioningIntent, error) {
		capturedFilter = filter
		return nil, nil
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/intents?status=failed", nil)
	rr := httptest.NewRecorder()
	handleGetProvisioningIntents(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if capturedFilter != "failed" {
		t.Errorf("expected filter=failed, got %q", capturedFilter)
	}
}

func TestHandleGetProvisioningIntents_Empty(t *testing.T) {
	setupNoopIntentDeps(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/intents", nil)
	rr := httptest.NewRecorder()
	handleGetProvisioningIntents(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if rr.Body.String() != "[]\n" {
		t.Errorf("expected empty array, got %q", rr.Body.String())
	}
}

func TestHandleClaimIntents_Success(t *testing.T) {
	setupNoopIntentDeps(t)

	dbClaimPendingIntents = func(_ context.Context, limit int) ([]models.ProvisioningIntent, error) {
		return []models.ProvisioningIntent{sampleIntent()}, nil
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/intents/claim", nil)
	rr := httptest.NewRecorder()
	handleClaimIntents(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var intents []models.ProvisioningIntent
	if err := json.Unmarshal(rr.Body.Bytes(), &intents); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(intents) != 1 || intents[0].Status != "acknowledged" {
		t.Errorf("expected 1 acknowledged intent, got %+v", intents)
	}
}

func TestHandleClaimIntents_WithLimit(t *testing.T) {
	setupNoopIntentDeps(t)

	var capturedLimit int
	dbClaimPendingIntents = func(_ context.Context, limit int) ([]models.ProvisioningIntent, error) {
		capturedLimit = limit
		return nil, nil
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/intents/claim?limit=5", nil)
	rr := httptest.NewRecorder()
	handleClaimIntents(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if capturedLimit != 5 {
		t.Errorf("expected limit=5, got %d", capturedLimit)
	}
}

func TestHandleClaimIntents_Empty(t *testing.T) {
	setupNoopIntentDeps(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/intents/claim", nil)
	rr := httptest.NewRecorder()
	handleClaimIntents(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if rr.Body.String() != "[]\n" {
		t.Errorf("expected empty array, got %q", rr.Body.String())
	}
}

func TestHandleCompleteIntent_Success(t *testing.T) {
	setupNoopIntentDeps(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/intents/int-1/complete", nil)
	req.SetPathValue("id", "int-1")
	rr := httptest.NewRecorder()
	handleCompleteIntent(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleFailIntent_Success(t *testing.T) {
	setupNoopIntentDeps(t)

	var capturedMsg string
	dbFailIntent = func(_ context.Context, _, msg string) error {
		capturedMsg = msg
		return nil
	}

	body := []byte(`{"error_message":"ldap connection timeout"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/intents/int-1/fail", bytes.NewReader(body))
	req.SetPathValue("id", "int-1")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handleFailIntent(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if capturedMsg != "ldap connection timeout" {
		t.Errorf("expected error_message captured, got %q", capturedMsg)
	}
}

func TestHandleFailIntent_NotFound(t *testing.T) {
	setupNoopIntentDeps(t)

	dbFailIntent = func(_ context.Context, _, _ string) error {
		return fmt.Errorf("intent not found or already terminal")
	}

	body := []byte(`{"error_message":"some error"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/intents/bad-id/fail", bytes.NewReader(body))
	req.SetPathValue("id", "bad-id")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handleFailIntent(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}
