package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// resetRulesDeps captures and restores all rules-related injectable vars.
func resetRulesDeps(t *testing.T) {
	t.Helper()
	origGetActive := dbGetActiveMappingRules
	origCreate := dbCreateMappingRule
	origDetect := dbDetectCycleOnInsert
	origAudit := dbInsertAuditLog
	t.Cleanup(func() {
		dbGetActiveMappingRules = origGetActive
		dbCreateMappingRule = origCreate
		dbDetectCycleOnInsert = origDetect
		dbInsertAuditLog = origAudit
	})
}

// --- handleCreateMappingRule ---

func TestHandleCreateMappingRule_MissingFields(t *testing.T) {
	resetRulesDeps(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/rules/mapping", strings.NewReader(`{"source_project":"p1","source_role":"r1"}`))
	rr := httptest.NewRecorder()
	handleCreateMappingRule(rr, req)

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

func TestHandleCreateMappingRule_UnknownField(t *testing.T) {
	resetRulesDeps(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/rules/mapping", strings.NewReader(`{"source_project":"p1","source_role":"r1","target_project":"p2","target_role":"r2","extra":"z"}`))
	rr := httptest.NewRecorder()
	handleCreateMappingRule(rr, req)

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

func TestHandleCreateMappingRule_SelfEdge(t *testing.T) {
	resetRulesDeps(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/rules/mapping", strings.NewReader(`{"source_project":"p1","source_role":"r1","target_project":"p1","target_role":"r1"}`))
	rr := httptest.NewRecorder()
	handleCreateMappingRule(rr, req)

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

func TestHandleCreateMappingRule_CycleDetected(t *testing.T) {
	resetRulesDeps(t)

	dbDetectCycleOnInsert = func(ctx context.Context, sp, sr, tp, tr string) error {
		return errors.New("cycle detected: p2:r2 → p1:r1 → p2:r2")
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/rules/mapping", strings.NewReader(`{"source_project":"p1","source_role":"r1","target_project":"p2","target_role":"r2"}`))
	rr := httptest.NewRecorder()
	handleCreateMappingRule(rr, req)

	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["error"] != "CYCLE_DETECTED" {
		t.Fatalf("expected CYCLE_DETECTED, got %v", resp["error"])
	}
}

func TestHandleCreateMappingRule_HappyPath(t *testing.T) {
	resetRulesDeps(t)

	dbDetectCycleOnInsert = func(ctx context.Context, sp, sr, tp, tr string) error {
		return nil
	}
	dbCreateMappingRule = func(ctx context.Context, sp, sr, tp, tr string) (string, error) {
		return "rule-1", nil
	}
	auditAction := ""
	auditResource := ""
	dbInsertAuditLog = func(ctx context.Context, actorID, targetID, action, resourceID string) error {
		auditAction = action
		auditResource = resourceID
		return nil
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/rules/mapping", strings.NewReader(`{"source_project":"p1","source_role":"r1","target_project":"p2","target_role":"r2"}`))
	rr := httptest.NewRecorder()
	handleCreateMappingRule(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp CreateMappingRuleResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.ID != "rule-1" {
		t.Fatalf("expected id=rule-1, got %s", resp.ID)
	}
	if auditAction != "mapping_rule.created" {
		t.Fatalf("expected audit action mapping_rule.created, got %s", auditAction)
	}
	if auditResource != "rule-1" {
		t.Fatalf("expected audit resource rule-1, got %s", auditResource)
	}
}
