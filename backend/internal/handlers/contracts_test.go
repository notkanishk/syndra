package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func decodeErrorResponse(t *testing.T, rr *httptest.ResponseRecorder) ErrorResponse {
	t.Helper()
	var got ErrorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	return got
}

func TestHandleCreateAccessRequestRejectsUnknownField(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/requests", strings.NewReader(`{"requester_id":"u1","project_id":"p1","role_key":"r1","justification":"need access","extra":"x"}`))
	rr := httptest.NewRecorder()

	handleCreateAccessRequest(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rr.Code)
	}
	got := decodeErrorResponse(t, rr)
	if got.Error != "VALIDATION_FAILED" {
		t.Fatalf("expected VALIDATION_FAILED, got %s", got.Error)
	}
	if got.Details["body"] == "" {
		t.Fatalf("expected body validation detail to be present")
	}
}

func TestHandleCreateAccessRequestRejectsNegativeDuration(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/requests", strings.NewReader(`{"requester_id":"u1","project_id":"p1","role_key":"r1","justification":"need access","duration_days":-2}`))
	rr := httptest.NewRecorder()

	handleCreateAccessRequest(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rr.Code)
	}
	got := decodeErrorResponse(t, rr)
	if got.Details["duration_days"] == "" {
		t.Fatalf("expected duration_days detail")
	}
}

func TestHandleResolveAccessRequestRequiresReviewerOnApprove(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/requests/abc/decision", strings.NewReader(`{"status":"approved"}`))
	req.SetPathValue("id", "abc")
	rr := httptest.NewRecorder()

	handleResolveAccessRequest(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rr.Code)
	}
	got := decodeErrorResponse(t, rr)
	if got.Details["reviewer_id"] == "" {
		t.Fatalf("expected reviewer_id detail")
	}
}

func TestHandleCreateMappingRuleRejectsSelfEdge(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/rules/mapping", strings.NewReader(`{"source_project":"p1","source_role":"r1","target_project":"p1","target_role":"r1"}`))
	rr := httptest.NewRecorder()

	handleCreateMappingRule(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rr.Code)
	}
	got := decodeErrorResponse(t, rr)
	if got.Error != "VALIDATION_FAILED" {
		t.Fatalf("expected VALIDATION_FAILED, got %s", got.Error)
	}
}

func TestHandleActionInjectRejectsInvalidMethod(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/action/inject", nil)
	rr := httptest.NewRecorder()

	HandleActionInject(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status 405, got %d", rr.Code)
	}
	got := decodeErrorResponse(t, rr)
	if got.Error != "METHOD_NOT_ALLOWED" {
		t.Fatalf("expected METHOD_NOT_ALLOWED, got %s", got.Error)
	}
}

func TestHandleActionInjectRejectsMissingFields(t *testing.T) {
	// v2 payload without user.id must be rejected deterministically. Unknown
	// top-level fields (like the legacy "user_id") are accepted silently by
	// the lenient decoder used for the Zitadel-owned schema.
	req := httptest.NewRequest(http.MethodPost, "/api/action/inject", strings.NewReader(`{"user":{"id":""},"user_grants":[{"projectId":"p1","roles":["admin"]}]}`))
	rr := httptest.NewRecorder()

	HandleActionInject(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rr.Code)
	}
	got := decodeErrorResponse(t, rr)
	if got.Details["user.id"] == "" {
		t.Fatalf("expected user.id detail, got %v", got.Details)
	}
}

func TestHandleZitadelWebhookRejectsMissingRequiredFields(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/webhooks/zitadel", strings.NewReader(`{"user_id":"u1","source_project":"p1"}`))
	rr := httptest.NewRecorder()

	HandleZitadelWebhook(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rr.Code)
	}
	got := decodeErrorResponse(t, rr)
	if got.Details["role_key"] == "" {
		t.Fatalf("expected role_key detail")
	}
}
