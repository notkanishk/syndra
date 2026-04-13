package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"mkauth/internal/models"
)

func TestHandleResolveAccessRequestApprovedCreatesGrantAndRebuildsCache(t *testing.T) {
	originalGetByID := dbGetAccessRequestByID
	originalResolve := dbResolveAccessRequest
	originalUpsertGrant := dbUpsertDirectGrant
	originalAudit := dbInsertAuditLog
	originalRebuild := cacheRebuildUser
	defer func() {
		dbGetAccessRequestByID = originalGetByID
		dbResolveAccessRequest = originalResolve
		dbUpsertDirectGrant = originalUpsertGrant
		dbInsertAuditLog = originalAudit
		cacheRebuildUser = originalRebuild
	}()

	duration := 5
	dbGetAccessRequestByID = func(ctx context.Context, id string) (models.AccessRequest, error) {
		return models.AccessRequest{
			ID:          id,
			RequesterID: "u1",
			ProjectID:   "p1",
			RoleKey:     "admin",
			DurationDays: &duration,
		}, nil
	}

	resolved := false
	dbResolveAccessRequest = func(ctx context.Context, id, status, reviewerID, reviewNote string) error {
		resolved = true
		if status != "approved" {
			t.Fatalf("expected approved status, got %s", status)
		}
		if reviewerID != "reviewer-1" {
			t.Fatalf("unexpected reviewer: %s", reviewerID)
		}
		return nil
	}

	grantCreated := false
	dbUpsertDirectGrant = func(ctx context.Context, userID, projectID, roleKey, grantedBy, reason string, expiresAt *time.Time) (string, error) {
		grantCreated = true
		if userID != "u1" || projectID != "p1" || roleKey != "admin" {
			t.Fatalf("unexpected grant args: %s %s %s", userID, projectID, roleKey)
		}
		if grantedBy != "reviewer-1" {
			t.Fatalf("unexpected grantedBy: %s", grantedBy)
		}
		if reason != "Approved from access request" {
			t.Fatalf("unexpected reason: %s", reason)
		}
		if expiresAt == nil {
			t.Fatalf("expected expiry for duration-backed request")
		}
		return "g1", nil
	}

	rebuiltFor := ""
	cacheRebuildUser = func(ctx context.Context, userID string, projectIDs []string) {
		rebuiltFor = userID
	}

	auditAction := ""
	dbInsertAuditLog = func(ctx context.Context, actorID, targetID, action, resourceID string) error {
		auditAction = action
		return nil
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/requests/req-1/decision", strings.NewReader(`{"status":"approved","reviewer_id":"reviewer-1","review_note":"ok"}`))
	req.SetPathValue("id", "req-1")
	rr := httptest.NewRecorder()
	handleResolveAccessRequest(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if !resolved {
		t.Fatalf("expected request to be resolved")
	}
	if !grantCreated {
		t.Fatalf("expected direct grant upsert on approval")
	}
	if rebuiltFor != "u1" {
		t.Fatalf("expected cache rebuild for u1, got %s", rebuiltFor)
	}
	if auditAction != "access_request.approved" {
		t.Fatalf("unexpected audit action %s", auditAction)
	}
}

// resetAccessDeps captures and restores all access-related injectable vars.
func resetAccessDeps(t *testing.T) {
	t.Helper()
	origGetByID := dbGetAccessRequestByID
	origResolve := dbResolveAccessRequest
	origUpsert := dbUpsertDirectGrant
	origCreate := dbCreateAccessRequest
	origAudit := dbInsertAuditLog
	origRebuild := cacheRebuildUser
	t.Cleanup(func() {
		dbGetAccessRequestByID = origGetByID
		dbResolveAccessRequest = origResolve
		dbUpsertDirectGrant = origUpsert
		dbCreateAccessRequest = origCreate
		dbInsertAuditLog = origAudit
		cacheRebuildUser = origRebuild
	})
}

// --- handleUpsertUserDirectGrant ---

func TestHandleUpsertUserDirectGrant_ExpiryCalculation(t *testing.T) {
	resetAccessDeps(t)

	var capturedExpiry *time.Time
	dbUpsertDirectGrant = func(ctx context.Context, userID, projectID, roleKey, grantedBy, reason string, expiresAt *time.Time) (string, error) {
		capturedExpiry = expiresAt
		return "grant-1", nil
	}
	dbInsertAuditLog = func(ctx context.Context, actorID, targetID, action, resourceID string) error { return nil }
	cacheRebuildUser = func(ctx context.Context, userID string, projectIDs []string) {}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/u1/grants",
		strings.NewReader(`{"project_id":"printing","role_key":"member","duration_days":7}`))
	req.SetPathValue("id", "u1")
	rr := httptest.NewRecorder()
	handleUpsertUserDirectGrant(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if capturedExpiry == nil {
		t.Fatalf("expected non-nil expiresAt for DurationDays=7")
	}
	expected := time.Now().UTC().Add(7 * 24 * time.Hour)
	diff := capturedExpiry.Sub(expected)
	if diff < -time.Minute || diff > time.Minute {
		t.Fatalf("expiresAt %v is not within 1 minute of expected %v", capturedExpiry, expected)
	}
}

func TestHandleUpsertUserDirectGrant_ZeroDuration_NoExpiry(t *testing.T) {
	resetAccessDeps(t)

	var capturedExpiry *time.Time
	dbUpsertDirectGrant = func(ctx context.Context, userID, projectID, roleKey, grantedBy, reason string, expiresAt *time.Time) (string, error) {
		capturedExpiry = expiresAt
		return "grant-2", nil
	}
	dbInsertAuditLog = func(ctx context.Context, actorID, targetID, action, resourceID string) error { return nil }
	cacheRebuildUser = func(ctx context.Context, userID string, projectIDs []string) {}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/u1/grants",
		strings.NewReader(`{"project_id":"printing","role_key":"member","duration_days":0}`))
	req.SetPathValue("id", "u1")
	rr := httptest.NewRecorder()
	handleUpsertUserDirectGrant(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if capturedExpiry != nil {
		t.Fatalf("expected nil expiresAt for DurationDays=0, got %v", capturedExpiry)
	}
}

func TestHandleUpsertUserDirectGrant_CacheRebuilt(t *testing.T) {
	resetAccessDeps(t)

	dbUpsertDirectGrant = func(ctx context.Context, userID, projectID, roleKey, grantedBy, reason string, expiresAt *time.Time) (string, error) {
		return "grant-3", nil
	}
	dbInsertAuditLog = func(ctx context.Context, actorID, targetID, action, resourceID string) error { return nil }

	rebuiltUserID := ""
	cacheRebuildUser = func(ctx context.Context, userID string, projectIDs []string) {
		rebuiltUserID = userID
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/u42/grants",
		strings.NewReader(`{"project_id":"printing","role_key":"member"}`))
	req.SetPathValue("id", "u42")
	rr := httptest.NewRecorder()
	handleUpsertUserDirectGrant(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if rebuiltUserID != "u42" {
		t.Fatalf("expected cache rebuild for u42, got %s", rebuiltUserID)
	}
}

// --- handleCreateAccessRequest ---

func TestHandleCreateAccessRequest_PersistenceAndAudit(t *testing.T) {
	resetAccessDeps(t)

	dbCreateAccessRequest = func(ctx context.Context, requesterID, projectID, roleKey, justification string, durationDays *int) (string, error) {
		return "req-1", nil
	}
	auditAction := ""
	dbInsertAuditLog = func(ctx context.Context, actorID, targetID, action, resourceID string) error {
		auditAction = action
		return nil
	}

	body := `{"requester_id":"u1","project_id":"printing","role_key":"member","justification":"need access"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/requests", strings.NewReader(body))
	rr := httptest.NewRecorder()
	handleCreateAccessRequest(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["id"] != "req-1" {
		t.Fatalf("expected id=req-1, got %v", resp["id"])
	}
	if auditAction != "access_request.created" {
		t.Fatalf("expected audit action access_request.created, got %s", auditAction)
	}
}

func TestHandleCreateAccessRequest_ZeroDuration_NilPointer(t *testing.T) {
	resetAccessDeps(t)

	var capturedDuration *int
	dbCreateAccessRequest = func(ctx context.Context, requesterID, projectID, roleKey, justification string, durationDays *int) (string, error) {
		capturedDuration = durationDays
		return "req-2", nil
	}
	dbInsertAuditLog = func(ctx context.Context, actorID, targetID, action, resourceID string) error { return nil }

	body := `{"requester_id":"u1","project_id":"printing","role_key":"member","justification":"need access","duration_days":0}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/requests", strings.NewReader(body))
	rr := httptest.NewRecorder()
	handleCreateAccessRequest(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	if capturedDuration != nil {
		t.Fatalf("expected nil durationDays for DurationDays=0, got %v", *capturedDuration)
	}
}

// --- handleResolveAccessRequest (idempotency) ---

func TestHandleResolveAccessRequest_AlreadyApproved_Returns409(t *testing.T) {
	resetAccessDeps(t)

	dbGetAccessRequestByID = func(ctx context.Context, id string) (models.AccessRequest, error) {
		return models.AccessRequest{
			ID:          id,
			RequesterID: "u1",
			ProjectID:   "p1",
			RoleKey:     "admin",
			Status:      "approved",
		}, nil
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/requests/req-1/decision",
		strings.NewReader(`{"status":"approved","reviewer_id":"reviewer-1"}`))
	req.SetPathValue("id", "req-1")
	rr := httptest.NewRecorder()
	handleResolveAccessRequest(rr, req)

	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["error"] != "ALREADY_RESOLVED" {
		t.Fatalf("expected ALREADY_RESOLVED, got %v", resp["error"])
	}
}

func TestHandleResolveAccessRequest_AlreadyRejected_Returns409(t *testing.T) {
	resetAccessDeps(t)

	dbGetAccessRequestByID = func(ctx context.Context, id string) (models.AccessRequest, error) {
		return models.AccessRequest{
			ID:          id,
			RequesterID: "u1",
			ProjectID:   "p1",
			RoleKey:     "admin",
			Status:      "rejected",
		}, nil
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/requests/req-2/decision",
		strings.NewReader(`{"status":"approved","reviewer_id":"reviewer-1"}`))
	req.SetPathValue("id", "req-2")
	rr := httptest.NewRecorder()
	handleResolveAccessRequest(rr, req)

	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["error"] != "ALREADY_RESOLVED" {
		t.Fatalf("expected ALREADY_RESOLVED, got %v", resp["error"])
	}
}

func TestHandleResolveAccessRequest_Approved_ExpiryFromDuration(t *testing.T) {
	resetAccessDeps(t)

	duration := 3
	dbGetAccessRequestByID = func(ctx context.Context, id string) (models.AccessRequest, error) {
		return models.AccessRequest{
			ID:           id,
			RequesterID:  "u1",
			ProjectID:    "p1",
			RoleKey:      "admin",
			Status:       "pending",
			DurationDays: &duration,
		}, nil
	}
	dbResolveAccessRequest = func(ctx context.Context, id, status, reviewerID, reviewNote string) error {
		return nil
	}

	var capturedExpiry *time.Time
	dbUpsertDirectGrant = func(ctx context.Context, userID, projectID, roleKey, grantedBy, reason string, expiresAt *time.Time) (string, error) {
		capturedExpiry = expiresAt
		return "g1", nil
	}
	cacheRebuildUser = func(ctx context.Context, userID string, projectIDs []string) {}
	dbInsertAuditLog = func(ctx context.Context, actorID, targetID, action, resourceID string) error { return nil }

	req := httptest.NewRequest(http.MethodPost, "/api/v1/requests/req-3/decision",
		strings.NewReader(`{"status":"approved","reviewer_id":"reviewer-1"}`))
	req.SetPathValue("id", "req-3")
	rr := httptest.NewRecorder()
	handleResolveAccessRequest(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if capturedExpiry == nil {
		t.Fatalf("expected non-nil expiry for 3-day request")
	}
	expected := time.Now().UTC().Add(3 * 24 * time.Hour)
	diff := capturedExpiry.Sub(expected)
	if diff < -time.Minute || diff > time.Minute {
		t.Fatalf("expiresAt %v not within 1 minute of expected %v", capturedExpiry, expected)
	}
}

func TestHandleResolveAccessRequestRejectedDoesNotCreateGrant(t *testing.T) {
	originalGetByID := dbGetAccessRequestByID
	originalResolve := dbResolveAccessRequest
	originalUpsertGrant := dbUpsertDirectGrant
	originalAudit := dbInsertAuditLog
	originalRebuild := cacheRebuildUser
	defer func() {
		dbGetAccessRequestByID = originalGetByID
		dbResolveAccessRequest = originalResolve
		dbUpsertDirectGrant = originalUpsertGrant
		dbInsertAuditLog = originalAudit
		cacheRebuildUser = originalRebuild
	}()

	dbGetAccessRequestByID = func(ctx context.Context, id string) (models.AccessRequest, error) {
		return models.AccessRequest{
			ID:          id,
			RequesterID: "u1",
			ProjectID:   "p1",
			RoleKey:     "admin",
		}, nil
	}
	dbResolveAccessRequest = func(ctx context.Context, id, status, reviewerID, reviewNote string) error {
		return nil
	}

	grantCreated := false
	dbUpsertDirectGrant = func(ctx context.Context, userID, projectID, roleKey, grantedBy, reason string, expiresAt *time.Time) (string, error) {
		grantCreated = true
		return "", nil
	}

	rebuilt := false
	cacheRebuildUser = func(ctx context.Context, userID string, projectIDs []string) {
		rebuilt = true
	}

	auditAction := ""
	dbInsertAuditLog = func(ctx context.Context, actorID, targetID, action, resourceID string) error {
		auditAction = action
		return nil
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/requests/req-2/decision", strings.NewReader(`{"status":"rejected","reviewer_id":"reviewer-2","review_note":"deny"}`))
	req.SetPathValue("id", "req-2")
	rr := httptest.NewRecorder()
	handleResolveAccessRequest(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if grantCreated {
		t.Fatalf("did not expect direct grant on rejection")
	}
	if rebuilt {
		t.Fatalf("did not expect cache rebuild on rejection")
	}
	if auditAction != "access_request.rejected" {
		t.Fatalf("unexpected audit action %s", auditAction)
	}
}
