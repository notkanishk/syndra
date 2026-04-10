package handlers

import (
	"context"
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
