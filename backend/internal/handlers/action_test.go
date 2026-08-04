package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/redis/go-redis/v9"

	"syndra/internal/claims"
)

// resetActionDeps restores the injectable vars used by HandleActionInject and
// degradedResponse. Stubs the claim_mode read-through cache vars to bypass
// Redis (always miss → DB) so legacy tests that only configure
// dbGetClaimFailureMode keep flowing through to the DB stub.
//
// The claim-shape read-through is stubbed the same way: Redis always misses
// and dbResolveClaimProfiles returns the built-in default profile, so tests
// that care about degraded behaviour rather than shaping stay short.
func resetActionDeps(t *testing.T, origRedis func(context.Context, string) (string, error), origMode func(context.Context, string) (string, map[string]interface{}, error)) {
	t.Helper()
	origGetMode, origSetMode := redisGetClaimMode, redisSetClaimMode
	origGetKey, origSetKey, origResolve := redisGetKey, redisSetKey, dbResolveClaimProfiles
	redisGetClaimMode = func(context.Context, string) (string, error) { return "", redis.Nil }
	redisSetClaimMode = func(context.Context, string, string, int) error { return nil }
	redisGetKey = func(context.Context, string) (string, error) { return "", redis.Nil }
	redisSetKey = func(context.Context, string, string, int) error { return nil }
	dbResolveClaimProfiles = func(_ context.Context, projectID string) ([]claims.Profile, error) {
		return []claims.Profile{{
			ProjectID:  projectID,
			ClaimName:  claims.DefaultClaimName,
			FormatType: claims.DefaultFormat,
		}}, nil
	}
	t.Cleanup(func() {
		redisGetClaims = origRedis
		dbGetClaimFailureMode = origMode
		redisGetClaimMode = origGetMode
		redisSetClaimMode = origSetMode
		redisGetKey = origGetKey
		redisSetKey = origSetKey
		dbResolveClaimProfiles = origResolve
	})
}

// factsJSON builds the cache body the compiler writes: facts, not claims.
func factsJSON(userID, projectID string, roles ...string) string {
	b, _ := json.Marshal(claims.Facts{
		Roles:      roles,
		UserID:     userID,
		ProjectID:  projectID,
		Email:      userID + "@makerspace.local",
		Name:       "Test " + userID,
		Team:       "Fabrication",
		CompiledAt: "2026-07-30T11:04:00Z",
	})
	return string(b)
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

func TestHandleActionInject_CacheHit_UsesConfiguredClaimNameAndFormat(t *testing.T) {
	origRedis, origMode := redisGetClaims, dbGetClaimFailureMode
	resetActionDeps(t, origRedis, origMode)

	redisGetClaims = func(ctx context.Context, key string) (string, error) {
		return factsJSON("u1", "p1", "admin", "lab"), nil
	}
	dbGetClaimFailureMode = func(ctx context.Context, projectID string) (string, map[string]interface{}, error) {
		t.Error("GetClaimFailureMode called on cache hit — should not happen")
		return "fail_closed", nil, nil
	}
	dbResolveClaimProfiles = func(_ context.Context, projectID string) ([]claims.Profile, error) {
		return []claims.Profile{{
			ProjectID:  projectID,
			ClaimName:  "x-groups",
			FormatType: claims.FormatCSV,
		}}, nil
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
		t.Fatalf("expected the operator-configured claim key x-groups, got %v", got.AppendClaims)
	}
	if claim.Value != "admin,lab" {
		t.Fatalf("expected csv-formatted roles admin,lab, got %v", claim.Value)
	}
}

// The token an operator previews and the token Zitadel receives must be shaped
// by the same profile. This pins the half of that contract the data plane owns:
// an attribute claim and a static claim configured on the profile both reach
// the envelope, alongside the roles.
func TestHandleActionInject_EmitsAttributeAndStaticClaims(t *testing.T) {
	origRedis, origMode := redisGetClaims, dbGetClaimFailureMode
	resetActionDeps(t, origRedis, origMode)

	redisGetClaims = func(ctx context.Context, key string) (string, error) {
		return factsJSON("u1", "pLaser", "trained"), nil
	}
	dbGetClaimFailureMode = func(ctx context.Context, projectID string) (string, map[string]interface{}, error) {
		t.Error("GetClaimFailureMode called on cache hit — should not happen")
		return "fail_closed", nil, nil
	}
	dbResolveClaimProfiles = func(_ context.Context, projectID string) ([]claims.Profile, error) {
		return []claims.Profile{{
			ProjectID:       projectID,
			ClaimName:       "syndra.laser.roles",
			FormatType:      claims.FormatArray,
			AttributeClaims: map[string]string{"syndra.laser.team": claims.AttrTeam},
			StaticClaims:    map[string]any{"syndra.tenant": "makerspace"},
		}}, nil
	}

	req := httptest.NewRequest(http.MethodPost, "/api/action/inject",
		strings.NewReader(v2Body("u1", ActionV2UserGrantRef{ProjectID: "pLaser", Roles: []string{"trained"}})))
	rr := httptest.NewRecorder()
	HandleActionInject(rr, req)

	got := decodeActionResponse(t, rr)
	roles, okRoles := claimByKey(got.AppendClaims, "syndra.laser.roles")
	team, okTeam := claimByKey(got.AppendClaims, "syndra.laser.team")
	tenant, okTenant := claimByKey(got.AppendClaims, "syndra.tenant")
	if !okRoles || !okTeam || !okTenant {
		t.Fatalf("expected roles, attribute and static claims, got %v", got.AppendClaims)
	}
	if team.Value != "Fabrication" {
		t.Errorf("expected team claim from cached facts, got %v", team.Value)
	}
	if tenant.Value != "makerspace" {
		t.Errorf("expected static claim verbatim, got %v", tenant.Value)
	}
	list, ok := roles.Value.([]interface{})
	if !ok || len(list) != 1 || list[0] != "trained" {
		t.Errorf("expected array-formatted roles, got %#v", roles.Value)
	}
}

// A project's token carries the project default AND every application
// override, because the Actions v2 payload does not say which application the
// token is for. Each app reads its own key.
func TestHandleActionInject_EmitsProjectDefaultAndAppOverrides(t *testing.T) {
	origRedis, origMode := redisGetClaims, dbGetClaimFailureMode
	resetActionDeps(t, origRedis, origMode)

	redisGetClaims = func(ctx context.Context, key string) (string, error) {
		return factsJSON("u1", "pLaser", "trained", "maintainer"), nil
	}
	dbResolveClaimProfiles = func(_ context.Context, projectID string) ([]claims.Profile, error) {
		return []claims.Profile{
			{ProjectID: projectID, ClaimName: "syndra.laser.roles", FormatType: claims.FormatArray},
			{ProjectID: projectID, ApplicationID: "app_badge", ClaimName: "badge.roles", FormatType: claims.FormatCSV},
		}, nil
	}

	req := httptest.NewRequest(http.MethodPost, "/api/action/inject",
		strings.NewReader(v2Body("u1", ActionV2UserGrantRef{ProjectID: "pLaser", Roles: []string{"trained"}})))
	rr := httptest.NewRecorder()
	HandleActionInject(rr, req)

	got := decodeActionResponse(t, rr)
	if len(got.AppendClaims) != 2 {
		t.Fatalf("expected both the default and the override key, got %v", got.AppendClaims)
	}
	badge, ok := claimByKey(got.AppendClaims, "badge.roles")
	if !ok || badge.Value != "trained,maintainer" {
		t.Fatalf("expected the override to apply its own csv format, got %v", got.AppendClaims)
	}
}

func TestHandleActionInject_MultiProject_KeepsEachProjectsOwnKeys(t *testing.T) {
	origRedis, origMode := redisGetClaims, dbGetClaimFailureMode
	resetActionDeps(t, origRedis, origMode)

	// Two distinct projects, both hit Redis. Each keeps the claim key its
	// operator configured — no "syndra.<projectID>." prefix, because keys are
	// validated unique across projects at save time and an application that
	// asked for "printing.roles" must receive exactly that.
	redisGetClaims = func(ctx context.Context, key string) (string, error) {
		switch key {
		case "mapping:u1:pPrint":
			return factsJSON("u1", "pPrint", "operator"), nil
		case "mapping:u1:pDoor":
			return factsJSON("u1", "pDoor", "keyholder"), nil
		}
		return "", fmt.Errorf("unexpected key: %s", key)
	}
	dbGetClaimFailureMode = func(ctx context.Context, projectID string) (string, map[string]interface{}, error) {
		t.Errorf("GetClaimFailureMode called for project=%s on cache hit", projectID)
		return "fail_closed", nil, nil
	}
	dbResolveClaimProfiles = func(_ context.Context, projectID string) ([]claims.Profile, error) {
		return []claims.Profile{{
			ProjectID:  projectID,
			ClaimName:  projectID + ".roles",
			FormatType: claims.FormatCSV,
		}}, nil
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
	printClaim, okP := claimByKey(got.AppendClaims, "pPrint.roles")
	doorClaim, okD := claimByKey(got.AppendClaims, "pDoor.roles")
	if !okP || !okD {
		t.Fatalf("multi-project: expected each project's own key, got %v", got.AppendClaims)
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
			return factsJSON("u1", "pPrint", "operator"), nil
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
	claim, ok := claimByKey(got.AppendClaims, "roles")
	list, isList := claim.Value.([]interface{})
	if !ok || !isList || len(list) != 1 || list[0] != "operator" {
		t.Fatalf("expected the healthy project's roles claim to survive, got %v", got.AppendClaims)
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
	// but different roles MUST dedupe to a single project in Syndra's view,
	// keeping the response on the single-project fast path (flat keys).
	origRedis, origMode := redisGetClaims, dbGetClaimFailureMode
	resetActionDeps(t, origRedis, origMode)

	calls := 0
	redisGetClaims = func(ctx context.Context, key string) (string, error) {
		calls++
		if key != "mapping:u1:p1" {
			t.Errorf("unexpected cache key: %s", key)
		}
		return factsJSON("u1", "p1", "admin"), nil
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
	claim, ok := claimByKey(got.AppendClaims, "roles")
	list, isList := claim.Value.([]interface{})
	if !ok || !isList || len(list) != 1 || list[0] != "admin" {
		t.Fatalf("expected one deduped roles claim, got %v", got.AppendClaims)
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
		return factsJSON("u1", "p1", "admin"), nil
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
	claim, ok := claimByKey(got.AppendClaims, "roles")
	list, isList := claim.Value.([]interface{})
	if !ok || !isList || len(list) != 1 || list[0] != "admin" {
		t.Fatalf("expected roles=[admin], got %v", got.AppendClaims)
	}
}

// ---------------------------------------------------------------------------
// claimFailureModeRead (C5) — Redis read-through cache for the per-project
// failure mode. Tests focus on the helper's three branches: cache hit
// (Redis returns a value), cache miss + DB success (refresh + cache), and
// cache miss + DB error (defaults to fail_closed).
// ---------------------------------------------------------------------------

func resetClaimModeCacheDeps(t *testing.T) {
	t.Helper()
	origGet, origSet, origMode := redisGetClaimMode, redisSetClaimMode, dbGetClaimFailureMode
	t.Cleanup(func() {
		redisGetClaimMode = origGet
		redisSetClaimMode = origSet
		dbGetClaimFailureMode = origMode
	})
}

func TestClaimFailureModeRead_DBError_FallsBackToCachedValue(t *testing.T) {
	resetClaimModeCacheDeps(t)

	redisGetClaimMode = func(ctx context.Context, projectID string) (string, error) {
		if projectID != "proj-1" {
			t.Errorf("unexpected projectID %q", projectID)
		}
		if _, ok := ctx.Deadline(); !ok {
			t.Error("expected redisGetClaimMode to receive a deadline-bounded context (redisTimeout wrap missing)")
		}
		return `{"mode":"minimal_safe","minimal_safe_claims":{"reason":"degraded"}}`, nil
	}
	redisSetClaimMode = func(ctx context.Context, projectID, value string, ttlSeconds int) error {
		t.Errorf("redisSetClaimMode must not be called on cache hit; got projectID=%s", projectID)
		return nil
	}
	dbGetClaimFailureMode = func(ctx context.Context, projectID string) (string, map[string]interface{}, error) {
		t.Errorf("dbGetClaimFailureMode must not be called on cache hit; got projectID=%s", projectID)
		return "fail_closed", nil, nil
	}

	mode, claims, err := claimFailureModeRead(context.Background(), "proj-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mode != "minimal_safe" {
		t.Errorf("expected mode=minimal_safe; got %q", mode)
	}
	if got := claims["reason"]; got != "degraded" {
		t.Errorf("expected claims.reason=degraded; got %v", got)
	}
}

func TestClaimFailureModeRead_CacheMiss_DBSuccess_Caches(t *testing.T) {
	resetClaimModeCacheDeps(t)

	redisGetClaimMode = func(ctx context.Context, projectID string) (string, error) {
		return "", redis.Nil
	}
	var setCalls int
	redisSetClaimMode = func(ctx context.Context, projectID, value string, ttlSeconds int) error {
		setCalls++
		if projectID != "proj-2" {
			t.Errorf("expected projectID=proj-2; got %q", projectID)
		}
		if !strings.Contains(value, `"mode":"fail_closed"`) {
			t.Errorf("expected fail_closed in cached value; got %q", value)
		}
		if ttlSeconds <= 0 {
			t.Errorf("expected positive TTL; got %d", ttlSeconds)
		}
		return nil
	}
	dbGetClaimFailureMode = func(ctx context.Context, projectID string) (string, map[string]interface{}, error) {
		return "fail_closed", nil, nil
	}

	mode, _, err := claimFailureModeRead(context.Background(), "proj-2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mode != "fail_closed" {
		t.Errorf("expected mode=fail_closed; got %q", mode)
	}
	if setCalls != 1 {
		t.Errorf("expected exactly 1 SET; got %d", setCalls)
	}
}

func TestClaimFailureModeRead_CacheMissAndDBError_DefaultsFailClosed(t *testing.T) {
	resetClaimModeCacheDeps(t)

	redisGetClaimMode = func(ctx context.Context, projectID string) (string, error) {
		return "", redis.Nil
	}
	redisSetClaimMode = func(ctx context.Context, projectID, value string, ttlSeconds int) error {
		t.Errorf("must not cache on DB error")
		return nil
	}
	dbGetClaimFailureMode = func(ctx context.Context, projectID string) (string, map[string]interface{}, error) {
		return "fail_closed", nil, errors.New("simulated db outage")
	}

	mode, claims, err := claimFailureModeRead(context.Background(), "proj-3")
	if err != nil {
		t.Fatalf("expected nil error (helper swallows for safety); got %v", err)
	}
	if mode != "fail_closed" {
		t.Errorf("expected fail_closed default; got %q", mode)
	}
	if claims != nil {
		t.Errorf("expected nil claims; got %v", claims)
	}
}
