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
	"mkauth/internal/services/propagation"
)

// resetRulesDeps captures and restores all rules-related injectable vars.
func resetRulesDeps(t *testing.T) {
	t.Helper()
	origGetActive := dbGetActiveMappingRules
	origDetect := dbDetectCycleOnInsert
	origAudit := dbInsertAuditLog
	origCascadeCreate := svcCascadeRuleCreated
	origGetRule := dbGetMappingRuleByID
	origDetectUpdate := dbDetectCycleOnUpdate
	origCascadeUpdate := svcCascadeRuleUpdated
	origCascadeDelete := svcCascadeRuleDeleted
	origGetConfig := dbGetConfigSetting
	// Default: no configured global default (resolveConfirmationMode normalizes "" to "auto") —
	// keeps every existing test that doesn't set confirmation_mode from hitting a real DB call.
	dbGetConfigSetting = func(ctx context.Context, key string) (string, error) { return "", nil }
	t.Cleanup(func() {
		dbGetActiveMappingRules = origGetActive
		dbDetectCycleOnInsert = origDetect
		dbInsertAuditLog = origAudit
		svcCascadeRuleCreated = origCascadeCreate
		dbGetMappingRuleByID = origGetRule
		dbDetectCycleOnUpdate = origDetectUpdate
		svcCascadeRuleUpdated = origCascadeUpdate
		svcCascadeRuleDeleted = origCascadeDelete
		dbGetConfigSetting = origGetConfig
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
	var gotSP, gotSR, gotTP, gotTR string
	svcCascadeRuleCreated = func(ctx context.Context, actor, sp, sr, tp, tr, mode string) (string, services.CascadeResult, error) {
		gotSP, gotSR, gotTP, gotTR = sp, sr, tp, tr
		return "rule-1", services.CascadeResult{Enqueued: 1, Mode: "auto", Drain: propagation.DrainResult{Applied: 1}}, nil
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/rules/mapping", strings.NewReader(`{"source_project":"p1","source_role":"r1","target_project":"p2","target_role":"r2"}`))
	rr := httptest.NewRecorder()
	handleCreateMappingRule(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	if gotSP != "p1" || gotSR != "r1" || gotTP != "p2" || gotTR != "r2" {
		t.Fatalf("cascade called with sp=%q sr=%q tp=%q tr=%q", gotSP, gotSR, gotTP, gotTR)
	}
	var resp map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["id"] != "rule-1" {
		t.Fatalf("expected id=rule-1, got %v", resp["id"])
	}
	if resp["cascade"] == nil {
		t.Fatal("expected a cascade field in the response")
	}
}

// TestHandleCreateMappingRule_UniqueViolationIs409 is the review fix: a pg unique-index
// violation (idx_mapping_rules_logic, SQLSTATE 23505) surfacing from
// db.CreateMappingRuleAndEnqueue must be reported as 409 CONFLICT, not the generic 500
// CASCADE_ERROR every other cascade failure gets.
func TestHandleCreateMappingRule_UniqueViolationIs409(t *testing.T) {
	resetRulesDeps(t)

	dbDetectCycleOnInsert = func(ctx context.Context, sp, sr, tp, tr string) error { return nil }
	svcCascadeRuleCreated = func(ctx context.Context, actor, sp, sr, tp, tr, mode string) (string, services.CascadeResult, error) {
		return "", services.CascadeResult{}, &pgconn.PgError{Code: "23505", ConstraintName: "idx_mapping_rules_logic"}
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
	if resp["error"] != "CONFLICT" {
		t.Fatalf("expected CONFLICT, got %v", resp["error"])
	}
}

func TestHandleCreateMappingRule_CascadeErrorIs500(t *testing.T) {
	resetRulesDeps(t)

	dbDetectCycleOnInsert = func(ctx context.Context, sp, sr, tp, tr string) error {
		return nil
	}
	svcCascadeRuleCreated = func(ctx context.Context, actor, sp, sr, tp, tr, mode string) (string, services.CascadeResult, error) {
		return "", services.CascadeResult{}, errors.New("insert rule failed")
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/rules/mapping", strings.NewReader(`{"source_project":"p1","source_role":"r1","target_project":"p2","target_role":"r2"}`))
	rr := httptest.NewRecorder()
	handleCreateMappingRule(rr, req)

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

// --- handleUpdateMappingRule ---

func TestHandleUpdateMappingRule_UnknownID(t *testing.T) {
	resetRulesDeps(t)

	dbGetMappingRuleByID = func(ctx context.Context, id string) (models.MappingRule, error) {
		return models.MappingRule{}, errors.New("no rows")
	}

	req := httptest.NewRequest(http.MethodPut, "/api/v1/rules/mapping/missing",
		strings.NewReader(`{"source_project":"p1","source_role":"r1","target_project":"p2","target_role":"r2"}`))
	req.SetPathValue("id", "missing")
	rr := httptest.NewRecorder()
	handleUpdateMappingRule(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["error"] != "NOT_FOUND" {
		t.Fatalf("expected NOT_FOUND, got %v", resp["error"])
	}
}

func TestHandleUpdateMappingRule_SelfEdge(t *testing.T) {
	resetRulesDeps(t)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/rules/mapping/rule1",
		strings.NewReader(`{"source_project":"p1","source_role":"r1","target_project":"p1","target_role":"r1"}`))
	req.SetPathValue("id", "rule1")
	rr := httptest.NewRecorder()
	handleUpdateMappingRule(rr, req)

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

func TestHandleUpdateMappingRule_CycleDetected(t *testing.T) {
	resetRulesDeps(t)

	dbGetMappingRuleByID = func(ctx context.Context, id string) (models.MappingRule, error) {
		return models.MappingRule{ID: id, SourceProject: "p1", SourceRole: "r1", TargetProject: "p2", TargetRole: "r2"}, nil
	}
	dbDetectCycleOnUpdate = func(ctx context.Context, excludeRuleID, sp, sr, tp, tr string) error {
		return errors.New("circular dependency detected")
	}

	req := httptest.NewRequest(http.MethodPut, "/api/v1/rules/mapping/rule1",
		strings.NewReader(`{"source_project":"p3","source_role":"r3","target_project":"p4","target_role":"r4"}`))
	req.SetPathValue("id", "rule1")
	rr := httptest.NewRecorder()
	handleUpdateMappingRule(rr, req)

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

// TestHandleUpdateMappingRule_RetargetOnlyCyclingWithOldEdge_Accepted guards the reason
// DetectCycleOnUpdate exists (vs reusing DetectCycleOnInsert): a retarget that would only cycle
// if the rule's OWN old edge were still in the graph must be ACCEPTED. This test stubs
// dbDetectCycleOnUpdate directly (the real exclusion logic is unit-tested in
// db.TestExcludeRuleFromGraph_UpdateVsInsertCycleDifference, which has no DB); here we assert the
// handler wires the update-aware detector — not dbDetectCycleOnInsert — into the request path.
func TestHandleUpdateMappingRule_RetargetOnlyCyclingWithOldEdge_Accepted(t *testing.T) {
	resetRulesDeps(t)

	old := models.MappingRule{ID: "rule1", SourceProject: "p1", SourceRole: "r1", TargetProject: "p2", TargetRole: "r2", ConfirmationMode: "auto"}
	dbGetMappingRuleByID = func(ctx context.Context, id string) (models.MappingRule, error) {
		return old, nil
	}
	var gotExclude string
	dbDetectCycleOnUpdate = func(ctx context.Context, excludeRuleID, sp, sr, tp, tr string) error {
		gotExclude = excludeRuleID
		return nil // excluding rule1's own old edge, the reverse retarget is not a cycle
	}
	var gotOld models.MappingRule
	svcCascadeRuleUpdated = func(ctx context.Context, actor string, o models.MappingRule, sp, sr, tp, tr string) (services.CascadeResult, error) {
		gotOld = o
		return services.CascadeResult{Mode: "auto"}, nil
	}

	// Retarget rule1 to the reverse edge (p2:r2 -> p1:r1) — this only cycles if rule1's own old
	// edge (p1:r1 -> p2:r2) were still counted, which DetectCycleOnUpdate excludes.
	req := httptest.NewRequest(http.MethodPut, "/api/v1/rules/mapping/rule1",
		strings.NewReader(`{"source_project":"p2","source_role":"r2","target_project":"p1","target_role":"r1"}`))
	req.SetPathValue("id", "rule1")
	rr := httptest.NewRecorder()
	handleUpdateMappingRule(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 (retarget accepted), got %d: %s", rr.Code, rr.Body.String())
	}
	if gotExclude != "rule1" {
		t.Fatalf("expected dbDetectCycleOnUpdate to exclude rule1, got %q", gotExclude)
	}
	if gotOld.ID != "rule1" {
		t.Fatalf("expected the cascade to receive the pre-update rule, got %+v", gotOld)
	}
}

func TestHandleUpdateMappingRule_HappyPath(t *testing.T) {
	resetRulesDeps(t)

	old := models.MappingRule{ID: "rule1", SourceProject: "p1", SourceRole: "r1", TargetProject: "p2", TargetRole: "r2old", ConfirmationMode: "auto"}
	dbGetMappingRuleByID = func(ctx context.Context, id string) (models.MappingRule, error) {
		return old, nil
	}
	dbDetectCycleOnUpdate = func(ctx context.Context, excludeRuleID, sp, sr, tp, tr string) error {
		return nil
	}
	var gotSP, gotSR, gotTP, gotTR string
	svcCascadeRuleUpdated = func(ctx context.Context, actor string, o models.MappingRule, sp, sr, tp, tr string) (services.CascadeResult, error) {
		gotSP, gotSR, gotTP, gotTR = sp, sr, tp, tr
		return services.CascadeResult{Enqueued: 2, Mode: "auto", Drain: propagation.DrainResult{Applied: 2}}, nil
	}

	req := httptest.NewRequest(http.MethodPut, "/api/v1/rules/mapping/rule1",
		strings.NewReader(`{"source_project":"p1","source_role":"r1","target_project":"p2","target_role":"r2new"}`))
	req.SetPathValue("id", "rule1")
	rr := httptest.NewRecorder()
	handleUpdateMappingRule(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if gotSP != "p1" || gotSR != "r1" || gotTP != "p2" || gotTR != "r2new" {
		t.Fatalf("cascade called with sp=%q sr=%q tp=%q tr=%q", gotSP, gotSR, gotTP, gotTR)
	}
	var resp map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["message"] != "Mapping rule updated" {
		t.Fatalf("unexpected message: %v", resp["message"])
	}
	if resp["cascade"] == nil {
		t.Fatal("expected a cascade field in the response")
	}
}

// TestHandleUpdateMappingRule_UniqueViolationIs409 mirrors the create-side fix: a retarget that
// collides with idx_mapping_rules_logic must also 409, not 500.
func TestHandleUpdateMappingRule_UniqueViolationIs409(t *testing.T) {
	resetRulesDeps(t)

	dbGetMappingRuleByID = func(ctx context.Context, id string) (models.MappingRule, error) {
		return models.MappingRule{ID: id}, nil
	}
	dbDetectCycleOnUpdate = func(ctx context.Context, excludeRuleID, sp, sr, tp, tr string) error { return nil }
	svcCascadeRuleUpdated = func(ctx context.Context, actor string, o models.MappingRule, sp, sr, tp, tr string) (services.CascadeResult, error) {
		return services.CascadeResult{}, &pgconn.PgError{Code: "23505", ConstraintName: "idx_mapping_rules_logic"}
	}

	req := httptest.NewRequest(http.MethodPut, "/api/v1/rules/mapping/rule1",
		strings.NewReader(`{"source_project":"p1","source_role":"r1","target_project":"p2","target_role":"r2"}`))
	req.SetPathValue("id", "rule1")
	rr := httptest.NewRecorder()
	handleUpdateMappingRule(rr, req)

	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["error"] != "CONFLICT" {
		t.Fatalf("expected CONFLICT, got %v", resp["error"])
	}
}

// TestHandleUpdateMappingRule_UnknownField is Minor Fix #5: handleUpdateMappingRule uses
// decodeJSONStrict same as the other cascade-trigger endpoints; an unrecognized field must 400,
// mirroring TestHandleCreateBundle_UnknownField's convention.
func TestHandleUpdateMappingRule_UnknownField(t *testing.T) {
	resetRulesDeps(t)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/rules/mapping/rule1",
		strings.NewReader(`{"source_project":"p1","source_role":"r1","target_project":"p2","target_role":"r2","extra":"z"}`))
	req.SetPathValue("id", "rule1")
	rr := httptest.NewRecorder()
	handleUpdateMappingRule(rr, req)

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

func TestHandleUpdateMappingRule_CascadeErrorIs500(t *testing.T) {
	resetRulesDeps(t)

	dbGetMappingRuleByID = func(ctx context.Context, id string) (models.MappingRule, error) {
		return models.MappingRule{ID: id}, nil
	}
	dbDetectCycleOnUpdate = func(ctx context.Context, excludeRuleID, sp, sr, tp, tr string) error {
		return nil
	}
	svcCascadeRuleUpdated = func(ctx context.Context, actor string, o models.MappingRule, sp, sr, tp, tr string) (services.CascadeResult, error) {
		return services.CascadeResult{}, errors.New("update tx failed")
	}

	req := httptest.NewRequest(http.MethodPut, "/api/v1/rules/mapping/rule1",
		strings.NewReader(`{"source_project":"p1","source_role":"r1","target_project":"p2","target_role":"r2"}`))
	req.SetPathValue("id", "rule1")
	rr := httptest.NewRecorder()
	handleUpdateMappingRule(rr, req)

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

// --- handleDeleteMappingRule ---

func TestHandleDeleteMappingRule_UnknownIDIs404(t *testing.T) {
	resetRulesDeps(t)

	dbGetMappingRuleByID = func(ctx context.Context, id string) (models.MappingRule, error) {
		return models.MappingRule{}, errors.New("no rows in result set")
	}
	svcCascadeRuleDeleted = func(ctx context.Context, actor string, old models.MappingRule) (services.CascadeResult, error) {
		t.Fatal("must not cascade for a rule that does not exist")
		return services.CascadeResult{}, nil
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/rules/mapping/nope", nil)
	req.SetPathValue("id", "nope")
	rr := httptest.NewRecorder()
	handleDeleteMappingRule(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

// The cascade gets the rule as it stands, not the id: the revoke set is computed from the edge
// the rule contributes, and the id alone does not say what that was.
func TestHandleDeleteMappingRule_PassesThePreDeleteRuleAndReportsTheCascade(t *testing.T) {
	resetRulesDeps(t)

	stored := models.MappingRule{ID: "rule-1", SourceProject: "sp", SourceRole: "sr",
		TargetProject: "tp", TargetRole: "tr", ConfirmationMode: "manual"}
	dbGetMappingRuleByID = func(ctx context.Context, id string) (models.MappingRule, error) {
		return stored, nil
	}
	var gotRule models.MappingRule
	svcCascadeRuleDeleted = func(ctx context.Context, actor string, old models.MappingRule) (services.CascadeResult, error) {
		gotRule = old
		return services.CascadeResult{Enqueued: 3, Mode: "manual"}, nil
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/rules/mapping/rule-1", nil)
	req.SetPathValue("id", "rule-1")
	rr := httptest.NewRecorder()
	handleDeleteMappingRule(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if gotRule != stored {
		t.Fatalf("cascade received %+v, want the pre-delete rule %+v", gotRule, stored)
	}
	var resp struct {
		Cascade services.CascadeResult `json:"cascade"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	// The queued revokes are the whole consequence of the click; a response that omitted them
	// would let the screen report a deletion as if nothing was waiting.
	if resp.Cascade.Enqueued != 3 || resp.Cascade.Mode != "manual" {
		t.Fatalf("cascade not reported: %+v", resp.Cascade)
	}
}

// The row went between the read and the transaction: whoever won already enqueued its revokes,
// so this is a 404, not a 500 the caller would retry into duplicate writes.
func TestHandleDeleteMappingRule_ConcurrentDeleteIs404(t *testing.T) {
	resetRulesDeps(t)

	dbGetMappingRuleByID = func(ctx context.Context, id string) (models.MappingRule, error) {
		return models.MappingRule{ID: id}, nil
	}
	svcCascadeRuleDeleted = func(ctx context.Context, actor string, old models.MappingRule) (services.CascadeResult, error) {
		return services.CascadeResult{}, pgx.ErrNoRows
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/rules/mapping/rule-1", nil)
	req.SetPathValue("id", "rule-1")
	rr := httptest.NewRecorder()
	handleDeleteMappingRule(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}
