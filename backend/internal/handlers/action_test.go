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

func decodeActionResponse(t *testing.T, rr *httptest.ResponseRecorder) ActionV2Response {
	t.Helper()
	var got ActionV2Response
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode ActionV2Response: %v\nbody: %s", err, rr.Body.String())
	}
	return got
}

// claimByKey returns the first claim matching key, or the zero value.
func claimByKey(claims []ActionV2Claim, key string) (ActionV2Claim, bool) {
	for _, c := range claims {
		if c.Key == key {
			return c, true
		}
	}
	return ActionV2Claim{}, false
}

// v2Body builds a minimal v2 function-trigger request body for a single user
// and grant list.
func v2Body(userID string, grants ...ActionV2UserGrantRef) string {
	b, _ := json.Marshal(map[string]interface{}{
		"function":    "function/preaccesstoken",
		"user":        map[string]string{"id": userID},
		"user_grants": grants,
	})
	return string(b)
}

func TestHandleActionInject_CacheMissFailClosed(t *testing.T) {
	origRedis, origMode := redisGetClaims, dbGetClaimFailureMode
	resetActionDeps(t, origRedis, origMode)

	redisGetClaims = func(ctx context.Context, key string) (string, error) {
		return "", fmt.Errorf("redis: nil")
	}
	dbGetClaimFailureMode = func(ctx context.Context, projectID string) (string, map[string]interface{}, error) {
		return "fail_closed", nil, nil
	}

	req := httptest.NewRequest(http.MethodPost, "/api/action/inject",
		strings.NewReader(v2Body("u1", ActionV2UserGrantRef{ProjectID: "p1", Roles: []string{"admin"}})))
	rr := httptest.NewRecorder()
	HandleActionInject(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	got := decodeActionResponse(t, rr)
	if len(got.AppendClaims) != 0 {
		t.Fatalf("fail_closed: expected empty append_claims, got %v", got.AppendClaims)
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
		strings.NewReader(v2Body("u1", ActionV2UserGrantRef{ProjectID: "p1", Roles: []string{"viewer"}})))
	rr := httptest.NewRecorder()
	HandleActionInject(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	got := decodeActionResponse(t, rr)
	claim, ok := claimByKey(got.AppendClaims, "role")
	if !ok {
		t.Fatalf("minimal_safe: expected claim key=role, got %v", got.AppendClaims)
	}
	if claim.Value != "guest" {
		t.Fatalf("minimal_safe: expected role=guest, got %v", claim.Value)
	}
}

func TestHandleActionInject_MalformedCache_FailClosed(t *testing.T) {
	origRedis, origMode := redisGetClaims, dbGetClaimFailureMode
	resetActionDeps(t, origRedis, origMode)

	redisGetClaims = func(ctx context.Context, key string) (string, error) {
		return "not-valid-json{{", nil
	}
	dbGetClaimFailureMode = func(ctx context.Context, projectID string) (string, map[string]interface{}, error) {
		return "fail_closed", nil, nil
	}

	req := httptest.NewRequest(http.MethodPost, "/api/action/inject",
		strings.NewReader(v2Body("u1", ActionV2UserGrantRef{ProjectID: "p1", Roles: []string{"admin"}})))
	rr := httptest.NewRecorder()
	HandleActionInject(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	got := decodeActionResponse(t, rr)
	if len(got.AppendClaims) != 0 {
		t.Fatalf("malformed cache fail_closed: expected empty append_claims, got %v", got.AppendClaims)
	}
}

func TestHandleActionInject_DBOutage_DefaultsToFailClosed(t *testing.T) {
	origRedis, origMode := redisGetClaims, dbGetClaimFailureMode
	resetActionDeps(t, origRedis, origMode)

	redisGetClaims = func(ctx context.Context, key string) (string, error) {
		return "", fmt.Errorf("redis: nil")
	}
	dbGetClaimFailureMode = func(ctx context.Context, projectID string) (string, map[string]interface{}, error) {
		return "fail_closed", nil, fmt.Errorf("DB connection refused")
	}

	req := httptest.NewRequest(http.MethodPost, "/api/action/inject",
		strings.NewReader(v2Body("u1", ActionV2UserGrantRef{ProjectID: "p1", Roles: []string{"admin"}})))
	rr := httptest.NewRecorder()
	HandleActionInject(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	got := decodeActionResponse(t, rr)
	if len(got.AppendClaims) != 0 {
		t.Fatalf("DB outage: expected empty append_claims, got %v", got.AppendClaims)
	}
}

func TestHandleActionInject_CacheHit_SingleProject_FlatKeys(t *testing.T) {
	origRedis, origMode := redisGetClaims, dbGetClaimFailureMode
	resetActionDeps(t, origRedis, origMode)

	redisGetClaims = func(ctx context.Context, key string) (string, error) {
		return `{"x-groups":"admin,lab"}`, nil
	}
	dbGetClaimFailureMode = func(ctx context.Context, projectID string) (string, map[string]interface{}, error) {
		t.Error("GetClaimFailureMode called on cache hit — should not happen")
		return "fail_closed", nil, nil
	}

	req := httptest.NewRequest(http.MethodPost, "/api/action/inject",
		strings.NewReader(v2Body("u1", ActionV2UserGrantRef{ProjectID: "p1", Roles: []string{"admin"}})))
	rr := httptest.NewRecorder()
	HandleActionInject(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	got := decodeActionResponse(t, rr)
	claim, ok := claimByKey(got.AppendClaims, "x-groups")
	if !ok {
		t.Fatalf("cache hit: expected flat key x-groups (no namespace), got %v", got.AppendClaims)
	}
	if claim.Value != "admin,lab" {
		t.Fatalf("cache hit: expected x-groups=admin,lab, got %v", claim.Value)
	}
}

func TestHandleActionInject_MultiProject_NamespacedKeys(t *testing.T) {
	origRedis, origMode := redisGetClaims, dbGetClaimFailureMode
	resetActionDeps(t, origRedis, origMode)

	// Two distinct projects, both hit Redis. Returned keys MUST be namespaced
	// so claims across projects cannot collide in the issued token.
	redisGetClaims = func(ctx context.Context, key string) (string, error) {
		switch key {
		case "mapping:u1:pPrint":
			return `{"role":"operator"}`, nil
		case "mapping:u1:pDoor":
			return `{"role":"keyholder"}`, nil
		}
		return "", fmt.Errorf("unexpected key: %s", key)
	}
	dbGetClaimFailureMode = func(ctx context.Context, projectID string) (string, map[string]interface{}, error) {
		t.Errorf("GetClaimFailureMode called for project=%s on cache hit", projectID)
		return "fail_closed", nil, nil
	}

	req := httptest.NewRequest(http.MethodPost, "/api/action/inject",
		strings.NewReader(v2Body("u1",
			ActionV2UserGrantRef{ProjectID: "pPrint", Roles: []string{"operator"}},
			ActionV2UserGrantRef{ProjectID: "pDoor", Roles: []string{"keyholder"}},
		)))
	rr := httptest.NewRecorder()
	HandleActionInject(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	got := decodeActionResponse(t, rr)
	printClaim, okP := claimByKey(got.AppendClaims, "mkauth.pPrint.role")
	doorClaim, okD := claimByKey(got.AppendClaims, "mkauth.pDoor.role")
	if !okP || !okD {
		t.Fatalf("multi-project: expected namespaced keys, got %v", got.AppendClaims)
	}
	if printClaim.Value != "operator" || doorClaim.Value != "keyholder" {
		t.Fatalf("multi-project: wrong values; got print=%v door=%v", printClaim.Value, doorClaim.Value)
	}
}

func TestHandleActionInject_MultiProject_OneDegraded(t *testing.T) {
	origRedis, origMode := redisGetClaims, dbGetClaimFailureMode
	resetActionDeps(t, origRedis, origMode)

	// pPrint hits; pDoor misses and is fail_closed. Final response MUST contain
	// only the pPrint claim — degraded projects MUST NOT block sibling projects.
	redisGetClaims = func(ctx context.Context, key string) (string, error) {
		if key == "mapping:u1:pPrint" {
			return `{"role":"operator"}`, nil
		}
		return "", fmt.Errorf("redis: nil")
	}
	dbGetClaimFailureMode = func(ctx context.Context, projectID string) (string, map[string]interface{}, error) {
		if projectID == "pDoor" {
			return "fail_closed", nil, nil
		}
		t.Errorf("GetClaimFailureMode called for unexpected project=%s", projectID)
		return "fail_closed", nil, nil
	}

	req := httptest.NewRequest(http.MethodPost, "/api/action/inject",
		strings.NewReader(v2Body("u1",
			ActionV2UserGrantRef{ProjectID: "pPrint", Roles: []string{"operator"}},
			ActionV2UserGrantRef{ProjectID: "pDoor", Roles: []string{"keyholder"}},
		)))
	rr := httptest.NewRecorder()
	HandleActionInject(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	got := decodeActionResponse(t, rr)
	if len(got.AppendClaims) != 1 {
		t.Fatalf("expected exactly 1 claim (sibling should not be blocked), got %v", got.AppendClaims)
	}
	claim, ok := claimByKey(got.AppendClaims, "mkauth.pPrint.role")
	if !ok || claim.Value != "operator" {
		t.Fatalf("expected mkauth.pPrint.role=operator, got %v", got.AppendClaims)
	}
}

func TestHandleActionInject_NoGrants_EmptyClaims(t *testing.T) {
	origRedis, origMode := redisGetClaims, dbGetClaimFailureMode
	resetActionDeps(t, origRedis, origMode)

	redisGetClaims = func(ctx context.Context, key string) (string, error) {
		t.Errorf("Redis called with no grants; should short-circuit. key=%s", key)
		return "", fmt.Errorf("unreachable")
	}
	dbGetClaimFailureMode = func(ctx context.Context, projectID string) (string, map[string]interface{}, error) {
		t.Errorf("GetClaimFailureMode called with no grants; should short-circuit. project=%s", projectID)
		return "fail_closed", nil, nil
	}

	req := httptest.NewRequest(http.MethodPost, "/api/action/inject",
		strings.NewReader(v2Body("u1")))
	rr := httptest.NewRecorder()
	HandleActionInject(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	got := decodeActionResponse(t, rr)
	if len(got.AppendClaims) != 0 {
		t.Fatalf("no grants: expected empty append_claims, got %v", got.AppendClaims)
	}
}

func TestHandleActionInject_DuplicateProjectGrants_DedupedToFlatKeys(t *testing.T) {
	// Zitadel emits one grant row per role. Two rows with the same projectId
	// but different roles MUST dedupe to a single project in MkAuth's view,
	// keeping the response on the single-project fast path (flat keys).
	origRedis, origMode := redisGetClaims, dbGetClaimFailureMode
	resetActionDeps(t, origRedis, origMode)

	calls := 0
	redisGetClaims = func(ctx context.Context, key string) (string, error) {
		calls++
		if key != "mapping:u1:p1" {
			t.Errorf("unexpected cache key: %s", key)
		}
		return `{"role":"admin"}`, nil
	}
	dbGetClaimFailureMode = func(ctx context.Context, projectID string) (string, map[string]interface{}, error) {
		t.Error("should not call failure mode on cache hit")
		return "fail_closed", nil, nil
	}

	req := httptest.NewRequest(http.MethodPost, "/api/action/inject",
		strings.NewReader(v2Body("u1",
			ActionV2UserGrantRef{ProjectID: "p1", Roles: []string{"admin"}},
			ActionV2UserGrantRef{ProjectID: "p1", Roles: []string{"operator"}},
		)))
	rr := httptest.NewRecorder()
	HandleActionInject(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if calls != 1 {
		t.Fatalf("expected exactly one Redis call after dedup, got %d", calls)
	}
	got := decodeActionResponse(t, rr)
	if claim, ok := claimByKey(got.AppendClaims, "role"); !ok || claim.Value != "admin" {
		t.Fatalf("expected flat role=admin (single-project fast path), got %v", got.AppendClaims)
	}
}

func TestHandleActionInject_InvalidJSON_400(t *testing.T) {
	origRedis, origMode := redisGetClaims, dbGetClaimFailureMode
	resetActionDeps(t, origRedis, origMode)

	req := httptest.NewRequest(http.MethodPost, "/api/action/inject",
		strings.NewReader(`not-json`))
	rr := httptest.NewRecorder()
	HandleActionInject(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid JSON, got %d", rr.Code)
	}
}

func TestHandleActionInject_UnknownFields_Accepted(t *testing.T) {
	// The v2 payload is Zitadel-owned and may grow. Unknown fields MUST NOT
	// cause a 400 — they are silently ignored by the lenient decoder.
	origRedis, origMode := redisGetClaims, dbGetClaimFailureMode
	resetActionDeps(t, origRedis, origMode)

	redisGetClaims = func(ctx context.Context, key string) (string, error) {
		return `{"role":"admin"}`, nil
	}
	dbGetClaimFailureMode = func(ctx context.Context, projectID string) (string, map[string]interface{}, error) {
		return "fail_closed", nil, nil
	}

	body := `{
		"function":"function/preaccesstoken",
		"user":{"id":"u1","human":{"profile":{"displayName":"Test"}}},
		"user_grants":[{"projectId":"p1","roles":["admin"],"projectName":"Printing"}],
		"org":{"id":"org1","name":"Ashoka"},
		"user_metadata":[{"key":"foo","value":"YmFy"}]
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/action/inject", strings.NewReader(body))
	rr := httptest.NewRecorder()
	HandleActionInject(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 with unknown fields accepted, got %d (body=%s)", rr.Code, rr.Body.String())
	}
	got := decodeActionResponse(t, rr)
	if claim, ok := claimByKey(got.AppendClaims, "role"); !ok || claim.Value != "admin" {
		t.Fatalf("expected role=admin, got %v", got.AppendClaims)
	}
}

