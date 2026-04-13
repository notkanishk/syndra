package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// resetActionDeps restores the injectable vars used by HandleActionInject and
// degradedResponse. Called via t.Cleanup in each test.
func resetActionDeps(t *testing.T, origRedis func(context.Context, string) (string, error), origMode func(context.Context, string) (string, map[string]interface{}, error)) {
	t.Helper()
	t.Cleanup(func() {
		redisGetClaims = origRedis
		dbGetClaimFailureMode = origMode
	})
}

func decodeActionResponse(t *testing.T, rr *httptest.ResponseRecorder) ActionResponse {
	t.Helper()
	var got ActionResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode ActionResponse: %v\nbody: %s", err, rr.Body.String())
	}
	return got
}

func TestHandleActionInject_CacheMissFailClosed(t *testing.T) {
	origRedis, origMode := redisGetClaims, dbGetClaimFailureMode
	resetActionDeps(t, origRedis, origMode)

	redisGetClaims = func(ctx context.Context, key string) (string, error) {
		return "", fmt.Errorf("redis: nil") // cache miss
	}
	dbGetClaimFailureMode = func(ctx context.Context, projectID string) (string, map[string]interface{}, error) {
		return "fail_closed", nil, nil
	}

	req := httptest.NewRequest(http.MethodPost, "/api/action/inject",
		strings.NewReader(`{"user_id":"u1","project_id":"p1"}`))
	rr := httptest.NewRecorder()
	HandleActionInject(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	got := decodeActionResponse(t, rr)
	if len(got.CustomClaims) != 0 {
		t.Fatalf("fail_closed: expected empty claims, got %v", got.CustomClaims)
	}
}

func TestHandleActionInject_CacheMissMinimalSafe(t *testing.T) {
	origRedis, origMode := redisGetClaims, dbGetClaimFailureMode
	resetActionDeps(t, origRedis, origMode)

	redisGetClaims = func(ctx context.Context, key string) (string, error) {
		return "", fmt.Errorf("redis: nil")
	}
	dbGetClaimFailureMode = func(ctx context.Context, projectID string) (string, map[string]interface{}, error) {
		return "minimal_safe", map[string]interface{}{"role": "guest"}, nil
	}

	req := httptest.NewRequest(http.MethodPost, "/api/action/inject",
		strings.NewReader(`{"user_id":"u1","project_id":"p1"}`))
	rr := httptest.NewRecorder()
	HandleActionInject(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	got := decodeActionResponse(t, rr)
	if got.CustomClaims["role"] != "guest" {
		t.Fatalf("minimal_safe: expected role=guest, got %v", got.CustomClaims)
	}
}

func TestHandleActionInject_MalformedCache_FailClosed(t *testing.T) {
	origRedis, origMode := redisGetClaims, dbGetClaimFailureMode
	resetActionDeps(t, origRedis, origMode)

	redisGetClaims = func(ctx context.Context, key string) (string, error) {
		return "not-valid-json{{", nil // cache hit but malformed
	}
	dbGetClaimFailureMode = func(ctx context.Context, projectID string) (string, map[string]interface{}, error) {
		return "fail_closed", nil, nil
	}

	req := httptest.NewRequest(http.MethodPost, "/api/action/inject",
		strings.NewReader(`{"user_id":"u1","project_id":"p1"}`))
	rr := httptest.NewRecorder()
	HandleActionInject(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	got := decodeActionResponse(t, rr)
	if len(got.CustomClaims) != 0 {
		t.Fatalf("malformed cache fail_closed: expected empty claims, got %v", got.CustomClaims)
	}
}

func TestHandleActionInject_DBOutage_DefaultsToFailClosed(t *testing.T) {
	// When GetClaimFailureMode itself returns an error (DB outage), the handler
	// must still return empty claims and not panic or expose the DB error to callers.
	origRedis, origMode := redisGetClaims, dbGetClaimFailureMode
	resetActionDeps(t, origRedis, origMode)

	redisGetClaims = func(ctx context.Context, key string) (string, error) {
		return "", fmt.Errorf("redis: nil")
	}
	dbGetClaimFailureMode = func(ctx context.Context, projectID string) (string, map[string]interface{}, error) {
		return "fail_closed", nil, fmt.Errorf("DB connection refused")
	}

	req := httptest.NewRequest(http.MethodPost, "/api/action/inject",
		strings.NewReader(`{"user_id":"u1","project_id":"p1"}`))
	rr := httptest.NewRecorder()
	HandleActionInject(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	got := decodeActionResponse(t, rr)
	if len(got.CustomClaims) != 0 {
		t.Fatalf("DB outage: expected empty claims, got %v", got.CustomClaims)
	}
}

func TestHandleActionInject_CacheHit(t *testing.T) {
	origRedis, origMode := redisGetClaims, dbGetClaimFailureMode
	resetActionDeps(t, origRedis, origMode)

	redisGetClaims = func(ctx context.Context, key string) (string, error) {
		return `{"x-groups":"admin,lab"}`, nil
	}
	// dbGetClaimFailureMode should NOT be called on a cache hit
	dbGetClaimFailureMode = func(ctx context.Context, projectID string) (string, map[string]interface{}, error) {
		t.Error("GetClaimFailureMode called on cache hit — should not happen")
		return "fail_closed", nil, nil
	}

	req := httptest.NewRequest(http.MethodPost, "/api/action/inject",
		strings.NewReader(`{"user_id":"u1","project_id":"p1"}`))
	rr := httptest.NewRecorder()
	HandleActionInject(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	got := decodeActionResponse(t, rr)
	if got.CustomClaims["x-groups"] != "admin,lab" {
		t.Fatalf("cache hit: expected x-groups=admin,lab, got %v", got.CustomClaims)
	}
}
