package handlers

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"mkauth/internal/db"
)

// signWebhook computes the correct HMAC-SHA256 over (tsHeader + "\n" + body).
func signWebhook(t *testing.T, secret string, tsHeader string, body []byte) string {
	t.Helper()
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(tsHeader))
	mac.Write([]byte("\n"))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

func TestVerifyWebhookSignature_ValidSignature(t *testing.T) {
	t.Setenv("ZITADEL_WEBHOOK_SECRET", "test-secret")

	body := []byte(`{"user_id":"u1","source_project":"p1","role_key":"member"}`)
	ts := fmt.Sprintf("%d", time.Now().Unix())
	sig := signWebhook(t, "test-secret", ts, body)

	if err := verifyWebhookSignature(body, ts, sig); err != nil {
		t.Fatalf("expected valid signature to pass: %v", err)
	}
}

func TestVerifyWebhookSignature_InvalidSignature(t *testing.T) {
	t.Setenv("ZITADEL_WEBHOOK_SECRET", "test-secret")

	body := []byte(`{"user_id":"u1","source_project":"p1","role_key":"member"}`)
	ts := fmt.Sprintf("%d", time.Now().Unix())
	if err := verifyWebhookSignature(body, ts, "deadbeefdeadbeef"); err == nil {
		t.Fatal("expected invalid signature to fail")
	}
}

func TestVerifyWebhookSignature_FreshTimestampWithStaleBodySignature(t *testing.T) {
	// Core replay attack scenario: attacker has a captured (body, sig) pair and
	// replaces the stale timestamp with a fresh one. The signature must now fail
	// because the fresh timestamp was not part of the original signed input.
	t.Setenv("ZITADEL_WEBHOOK_SECRET", "test-secret")

	body := []byte(`{"user_id":"u1","source_project":"p1","role_key":"member"}`)
	staleTs := fmt.Sprintf("%d", time.Now().Add(-10*time.Minute).Unix())

	// Capture: signature computed over the stale timestamp
	capturedSig := signWebhook(t, "test-secret", staleTs, body)

	// Replay attempt: reuse captured body+sig with a fresh timestamp
	freshTs := fmt.Sprintf("%d", time.Now().Unix())
	if err := verifyWebhookSignature(body, freshTs, capturedSig); err == nil {
		t.Fatal("expected replay with swapped timestamp to fail signature check")
	}
}

func TestVerifyWebhookSignature_MissingSignatureHeader(t *testing.T) {
	t.Setenv("ZITADEL_WEBHOOK_SECRET", "test-secret")

	body := []byte(`{}`)
	ts := fmt.Sprintf("%d", time.Now().Unix())
	if err := verifyWebhookSignature(body, ts, ""); err == nil {
		t.Fatal("expected missing signature header to fail")
	}
}

func TestVerifyWebhookSignature_MissingTimestampHeader(t *testing.T) {
	t.Setenv("ZITADEL_WEBHOOK_SECRET", "test-secret")

	body := []byte(`{}`)
	if err := verifyWebhookSignature(body, "", "somesig"); err == nil {
		t.Fatal("expected missing timestamp header to fail signature check")
	}
}

func TestVerifyWebhookSignature_NoSecretLocalDev(t *testing.T) {
	// When ZITADEL_WEBHOOK_SECRET is not set, verification is skipped (local-dev mode)
	t.Setenv("ZITADEL_WEBHOOK_SECRET", "")

	body := []byte(`{"user_id":"u1"}`)
	if err := verifyWebhookSignature(body, "", ""); err != nil {
		t.Fatalf("expected no error in local-dev mode: %v", err)
	}
}

func TestVerifyWebhookFreshness_FreshTimestamp(t *testing.T) {
	t.Setenv("ZITADEL_WEBHOOK_SECRET", "test-secret")

	ts := strconv.FormatInt(time.Now().Unix(), 10)
	if err := verifyWebhookFreshness(ts); err != nil {
		t.Fatalf("expected fresh timestamp to pass: %v", err)
	}
}

func TestVerifyWebhookFreshness_StaleTimestamp(t *testing.T) {
	t.Setenv("ZITADEL_WEBHOOK_SECRET", "test-secret")

	// 10 minutes ago — beyond the 5-minute window
	ts := strconv.FormatInt(time.Now().Add(-10*time.Minute).Unix(), 10)
	if err := verifyWebhookFreshness(ts); err == nil {
		t.Fatal("expected stale timestamp to fail")
	}
}

func TestVerifyWebhookFreshness_MissingTimestamp(t *testing.T) {
	t.Setenv("ZITADEL_WEBHOOK_SECRET", "test-secret")

	if err := verifyWebhookFreshness(""); err == nil {
		t.Fatal("expected missing timestamp to fail")
	}
}

func TestVerifyWebhookFreshness_NoSecretLocalDev(t *testing.T) {
	t.Setenv("ZITADEL_WEBHOOK_SECRET", "")

	if err := verifyWebhookFreshness(""); err != nil {
		t.Fatalf("expected no error in local-dev mode: %v", err)
	}
}

func TestHandleZitadelWebhook_RejectsInvalidSignature(t *testing.T) {
	t.Setenv("ZITADEL_WEBHOOK_SECRET", "test-secret")

	body := []byte(`{"user_id":"u1","source_project":"p1","role_key":"member","project_ids":["p1"]}`)
	ts := fmt.Sprintf("%d", time.Now().Unix())
	req := httptest.NewRequest(http.MethodPost, "/api/webhooks/zitadel", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Zitadel-Signature", "badhash")
	req.Header.Set("X-Zitadel-Timestamp", ts)
	rr := httptest.NewRecorder()

	HandleZitadelWebhook(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
	got := decodeErrorResponse(t, rr)
	if got.Error != "WEBHOOK_UNAUTHORIZED" {
		t.Fatalf("expected WEBHOOK_UNAUTHORIZED, got %s", got.Error)
	}
}

func TestHandleZitadelWebhook_RejectsStaleTimestamp(t *testing.T) {
	t.Setenv("ZITADEL_WEBHOOK_SECRET", "test-secret")

	body := []byte(`{"user_id":"u1","source_project":"p1","role_key":"member","project_ids":["p1"]}`)
	staleTs := fmt.Sprintf("%d", time.Now().Add(-10*time.Minute).Unix())

	// Signature computed over the stale timestamp (as a legitimate sender would)
	sig := signWebhook(t, "test-secret", staleTs, body)

	req := httptest.NewRequest(http.MethodPost, "/api/webhooks/zitadel", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Zitadel-Signature", sig)
	req.Header.Set("X-Zitadel-Timestamp", staleTs)
	rr := httptest.NewRecorder()

	HandleZitadelWebhook(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for stale webhook, got %d", rr.Code)
	}
	got := decodeErrorResponse(t, rr)
	if got.Error != "WEBHOOK_STALE" {
		t.Fatalf("expected WEBHOOK_STALE, got %s", got.Error)
	}
}

// --- Event type dispatch tests ---

// resetWebhookDeps saves and restores injectable vars for webhook handler tests.
func resetWebhookDeps(t *testing.T) {
	t.Helper()
	origRebuild := cacheRebuildUser
	origInvalidate := cacheInvalidateUser
	origEnforce := webhookEnforceMappingRules
	origRevoke := webhookRevokeMappingRules
	origOnboard := webhookTriggerOnboarding
	origInsert := dbInsertWebhookEvent
	origComplete := dbCompleteWebhookEvent
	origFail := dbFailWebhookEvent
	origEmitIntent := webhookEmitProvisioningIntent
	t.Cleanup(func() {
		cacheRebuildUser = origRebuild
		cacheInvalidateUser = origInvalidate
		webhookEnforceMappingRules = origEnforce
		webhookRevokeMappingRules = origRevoke
		webhookTriggerOnboarding = origOnboard
		dbInsertWebhookEvent = origInsert
		dbCompleteWebhookEvent = origComplete
		dbFailWebhookEvent = origFail
		webhookEmitProvisioningIntent = origEmitIntent
	})
}

// setupNoopWebhookDeps configures all webhook deps as no-ops for isolated testing.
func setupNoopWebhookDeps(t *testing.T) {
	t.Helper()
	resetWebhookDeps(t)
	cacheRebuildUser = func(_ context.Context, _ string, _ []string) {}
	cacheInvalidateUser = func(_ context.Context, _ string) error { return nil }
	webhookEnforceMappingRules = func(_ context.Context, _, _, _ string) error { return nil }
	webhookRevokeMappingRules = func(_ context.Context, _, _, _ string) error { return nil }
	webhookTriggerOnboarding = func(_ context.Context, _, _, _ string) error { return nil }
	dbInsertWebhookEvent = func(_ context.Context, _, _, _, _, _ string) (string, bool, error) {
		return "evt-1", true, nil
	}
	dbCompleteWebhookEvent = func(_ context.Context, _ string) error { return nil }
	dbFailWebhookEvent = func(_ context.Context, _, _ string) error { return nil }
	webhookEmitProvisioningIntent = func(_ context.Context, _, _, _, _, _ string) error { return nil }
}

func postWebhook(t *testing.T, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	t.Setenv("ZITADEL_WEBHOOK_SECRET", "")

	req := httptest.NewRequest(http.MethodPost, "/api/webhooks/zitadel", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	HandleZitadelWebhook(rr, req)
	return rr
}

func TestWebhook_EventTypeDefault(t *testing.T) {
	setupNoopWebhookDeps(t)

	var enforceCalled bool
	webhookEnforceMappingRules = func(_ context.Context, _, _, _ string) error {
		enforceCalled = true
		return nil
	}

	// No event_type field → should default to grant_added
	body := []byte(`{"user_id":"u1","source_project":"p1","role_key":"editor"}`)
	rr := postWebhook(t, body)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if !enforceCalled {
		t.Error("expected EnforceMappingRules to be called for default grant_added")
	}
}

func TestWebhook_GrantRemoved(t *testing.T) {
	setupNoopWebhookDeps(t)

	var revokeCalled bool
	var invalidateCalled bool
	webhookRevokeMappingRules = func(_ context.Context, _, _, _ string) error {
		revokeCalled = true
		return nil
	}
	cacheInvalidateUser = func(_ context.Context, _ string) error {
		invalidateCalled = true
		return nil
	}

	body := []byte(`{"event_type":"grant_removed","user_id":"u1","source_project":"p1","role_key":"editor"}`)
	rr := postWebhook(t, body)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if !revokeCalled {
		t.Error("expected RevokeMappingRules to be called")
	}
	if !invalidateCalled {
		t.Error("expected cache invalidation")
	}
}

func TestWebhook_UserDeactivated(t *testing.T) {
	setupNoopWebhookDeps(t)

	var invalidateCalled bool
	var enforceCalled, revokeCalled bool
	cacheInvalidateUser = func(_ context.Context, _ string) error {
		invalidateCalled = true
		return nil
	}
	webhookEnforceMappingRules = func(_ context.Context, _, _, _ string) error {
		enforceCalled = true
		return nil
	}
	webhookRevokeMappingRules = func(_ context.Context, _, _, _ string) error {
		revokeCalled = true
		return nil
	}

	body := []byte(`{"event_type":"user_deactivated","user_id":"u1","source_project":"p1"}`)
	rr := postWebhook(t, body)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if !invalidateCalled {
		t.Error("expected cache invalidation")
	}
	if enforceCalled {
		t.Error("enforce should NOT be called for user_deactivated")
	}
	if revokeCalled {
		t.Error("revoke should NOT be called for user_deactivated")
	}
}

func TestWebhook_UserCreated(t *testing.T) {
	setupNoopWebhookDeps(t)

	var onboardCalled bool
	webhookTriggerOnboarding = func(_ context.Context, userID, _, _ string) error {
		onboardCalled = true
		if userID != "u-new" {
			t.Errorf("expected userID u-new, got %s", userID)
		}
		return nil
	}

	body := []byte(`{"event_type":"user_created","user_id":"u-new","source_project":"p1"}`)
	rr := postWebhook(t, body)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if !onboardCalled {
		t.Error("expected onboarding trigger for user_created")
	}
}

func TestWebhook_InvalidEventType(t *testing.T) {
	setupNoopWebhookDeps(t)

	body := []byte(`{"event_type":"bogus","user_id":"u1","source_project":"p1","role_key":"x"}`)
	rr := postWebhook(t, body)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "event_type") {
		t.Error("expected error about event_type")
	}
}

func TestWebhook_DeduplicationSkips(t *testing.T) {
	setupNoopWebhookDeps(t)

	// Second insert returns duplicate
	dbInsertWebhookEvent = func(_ context.Context, _, _, _, _, _ string) (string, bool, error) {
		return "", false, nil
	}

	var enforceCalled bool
	webhookEnforceMappingRules = func(_ context.Context, _, _, _ string) error {
		enforceCalled = true
		return nil
	}

	body := []byte(`{"user_id":"u1","source_project":"p1","role_key":"editor"}`)
	rr := postWebhook(t, body)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if enforceCalled {
		t.Error("should NOT process duplicate event")
	}
	if !strings.Contains(rr.Body.String(), "Duplicate") {
		t.Error("expected duplicate message in response")
	}
}

func TestWebhook_GrantAddedRequiresRoleKey(t *testing.T) {
	setupNoopWebhookDeps(t)

	body := []byte(`{"event_type":"grant_added","user_id":"u1","source_project":"p1"}`)
	rr := postWebhook(t, body)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "role_key") {
		t.Error("expected error about role_key")
	}
}

func TestWebhook_UserDeactivatedNoRoleKeyRequired(t *testing.T) {
	setupNoopWebhookDeps(t)

	body := []byte(`{"event_type":"user_deactivated","user_id":"u1","source_project":"p1"}`)
	rr := postWebhook(t, body)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 (no role_key needed), got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestWebhook_GrantAdded_EmitsAddIntent(t *testing.T) {
	setupNoopWebhookDeps(t)

	var emittedAction, emittedUID, emittedProject, emittedRole string
	webhookEmitProvisioningIntent = func(_ context.Context, uid, action, project, role, _ string) error {
		emittedUID = uid
		emittedAction = action
		emittedProject = project
		emittedRole = role
		return nil
	}

	body := []byte(`{"event_type":"grant_added","user_id":"u1","source_project":"p1","role_key":"editor"}`)
	rr := postWebhook(t, body)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if emittedAction != "add" {
		t.Errorf("expected add intent, got %q", emittedAction)
	}
	if emittedUID != "u1" || emittedProject != "p1" || emittedRole != "editor" {
		t.Errorf("wrong intent args: uid=%s project=%s role=%s", emittedUID, emittedProject, emittedRole)
	}
}

func TestWebhook_GrantRemoved_EmitsRemoveIntent(t *testing.T) {
	setupNoopWebhookDeps(t)

	var emittedAction string
	webhookEmitProvisioningIntent = func(_ context.Context, _, action, _, _, _ string) error {
		emittedAction = action
		return nil
	}

	body := []byte(`{"event_type":"grant_removed","user_id":"u1","source_project":"p1","role_key":"editor"}`)
	rr := postWebhook(t, body)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if emittedAction != "remove" {
		t.Errorf("expected remove intent, got %q", emittedAction)
	}
}

func TestWebhook_GrantAdded_IntentFailureNonFatal(t *testing.T) {
	setupNoopWebhookDeps(t)

	webhookEmitProvisioningIntent = func(_ context.Context, _, _, _, _, _ string) error {
		return fmt.Errorf("intent DB unavailable")
	}

	body := []byte(`{"event_type":"grant_added","user_id":"u1","source_project":"p1","role_key":"editor"}`)
	rr := postWebhook(t, body)

	// Should still return 200 — intent failure is non-fatal.
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 (intent failure non-fatal), got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestWebhook_UserDeactivated_NoIntentEmitted(t *testing.T) {
	setupNoopWebhookDeps(t)

	var intentCalled bool
	webhookEmitProvisioningIntent = func(_ context.Context, _, _, _, _, _ string) error {
		intentCalled = true
		return nil
	}

	body := []byte(`{"event_type":"user_deactivated","user_id":"u1","source_project":"p1"}`)
	rr := postWebhook(t, body)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if intentCalled {
		t.Error("should NOT emit provisioning intent for user_deactivated in Change 1")
	}
}

// Verify the unused import doesn't cause issues — db is used for type reference in deps.
var _ = db.WebhookEvent{}

func TestHandleZitadelWebhook_RoleKeysPlural_DispatchesEachRole(t *testing.T) {
	setupNoopWebhookDeps(t)

	// Capture the roles passed to the orchestrator. Other deps stay at the
	// no-op defaults installed by the helper above.
	prevEnforce := webhookEnforceMappingRules
	t.Cleanup(func() { webhookEnforceMappingRules = prevEnforce })

	var seenRoles []string
	webhookEnforceMappingRules = func(ctx context.Context, userID, project, role string) error {
		seenRoles = append(seenRoles, role)
		return nil
	}

	body := []byte(`{
		"event_type": "grant_added",
		"user_id": "u1",
		"source_project": "p1",
		"role_keys": ["alpha", "beta"],
		"project_ids": ["p1"]
	}`)
	rr := postWebhook(t, body)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}
	if !reflect.DeepEqual(seenRoles, []string{"alpha", "beta"}) {
		t.Fatalf("expected roles [alpha beta], got %v", seenRoles)
	}
}

func TestHandleZitadelWebhook_RoleKeysBlankEntries_RejectedOrFiltered(t *testing.T) {
	cases := []struct {
		name     string
		body     string
		wantCode int
		wantSeen []string // nil = expect no orchestrator call
	}{
		{
			name:     "all blank entries → 400",
			body:     `{"event_type":"grant_added","user_id":"u1","source_project":"p1","role_keys":["",""]}`,
			wantCode: http.StatusBadRequest,
			wantSeen: nil,
		},
		{
			name:     "whitespace-only entries → 400",
			body:     `{"event_type":"grant_added","user_id":"u1","source_project":"p1","role_keys":["  "," \t"]}`,
			wantCode: http.StatusBadRequest,
			wantSeen: nil,
		},
		{
			name:     "mixed valid+blank → only valid roles dispatch",
			body:     `{"event_type":"grant_added","user_id":"u1","source_project":"p1","role_keys":["","alpha"," ","beta"],"project_ids":["p1"]}`,
			wantCode: http.StatusOK,
			wantSeen: []string{"alpha", "beta"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setupNoopWebhookDeps(t)

			prevEnforce := webhookEnforceMappingRules
			t.Cleanup(func() { webhookEnforceMappingRules = prevEnforce })

			var seen []string
			webhookEnforceMappingRules = func(ctx context.Context, userID, project, role string) error {
				seen = append(seen, role)
				return nil
			}

			rr := postWebhook(t, []byte(tc.body))

			if rr.Code != tc.wantCode {
				t.Fatalf("expected %d, got %d body=%s", tc.wantCode, rr.Code, rr.Body.String())
			}
			if !reflect.DeepEqual(seen, tc.wantSeen) {
				t.Fatalf("expected seen=%v, got %v", tc.wantSeen, seen)
			}
		})
	}
}

func TestHandleZitadelWebhook_RoleKeysOverridesSingular_ForAuditAndLog(t *testing.T) {
	setupNoopWebhookDeps(t)

	// Capture both the dispatch role AND the role passed to dbInsertWebhookEvent
	// (which becomes the webhook_events.role_key audit value).
	var dispatchedRole string
	var persistedRole string

	prevEnforce := webhookEnforceMappingRules
	prevInsert := dbInsertWebhookEvent
	t.Cleanup(func() {
		webhookEnforceMappingRules = prevEnforce
		dbInsertWebhookEvent = prevInsert
	})

	webhookEnforceMappingRules = func(ctx context.Context, userID, project, role string) error {
		dispatchedRole = role
		return nil
	}
	dbInsertWebhookEvent = func(ctx context.Context, eventType, userID, sourceProject, roleKey, idempotencyKey string) (string, bool, error) {
		persistedRole = roleKey
		return "evt-test", true, nil
	}

	// Mismatched payload: singular says "alpha", plural says "beta". Plural must win for both.
	body := []byte(`{
		"event_type": "grant_added",
		"user_id": "u1",
		"source_project": "p1",
		"role_key": "alpha",
		"role_keys": ["beta"],
		"project_ids": ["p1"]
	}`)
	rr := postWebhook(t, body)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}
	if dispatchedRole != "beta" {
		t.Errorf("dispatched role: expected beta, got %q", dispatchedRole)
	}
	if persistedRole != "beta" {
		t.Errorf("persisted role (webhook_events.role_key): expected beta, got %q (audit must align with dispatch)", persistedRole)
	}
}

func TestHandleZitadelWebhook_RoleKeysExplicitEmpty_RejectedDistinctFromOmitted(t *testing.T) {
	cases := []struct {
		name           string
		body           string
		wantCode       int
		wantDispatched string // empty = expect no orchestrator call
	}{
		{
			// Back-compat: omitted plural + valid singular dispatches singular.
			name:           "omitted plural + singular alpha → 200, dispatches alpha",
			body:           `{"event_type":"grant_added","user_id":"u1","source_project":"p1","role_key":"alpha","project_ids":["p1"]}`,
			wantCode:       http.StatusOK,
			wantDispatched: "alpha",
		},
		{
			// JSON convention: null is equivalent to omitted.
			name:           "null plural + singular alpha → 200, dispatches alpha",
			body:           `{"event_type":"grant_added","user_id":"u1","source_project":"p1","role_key":"alpha","role_keys":null,"project_ids":["p1"]}`,
			wantCode:       http.StatusOK,
			wantDispatched: "alpha",
		},
		{
			// Explicit empty plural contradicts singular under "plural wins". Reject.
			name:           "explicit [] plural + singular alpha → 400 (no dispatch)",
			body:           `{"event_type":"grant_added","user_id":"u1","source_project":"p1","role_key":"alpha","role_keys":[],"project_ids":["p1"]}`,
			wantCode:       http.StatusBadRequest,
			wantDispatched: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setupNoopWebhookDeps(t)

			prevEnforce := webhookEnforceMappingRules
			t.Cleanup(func() { webhookEnforceMappingRules = prevEnforce })

			var dispatched string
			webhookEnforceMappingRules = func(ctx context.Context, userID, project, role string) error {
				dispatched = role
				return nil
			}

			rr := postWebhook(t, []byte(tc.body))
			if rr.Code != tc.wantCode {
				t.Fatalf("expected %d, got %d body=%s", tc.wantCode, rr.Code, rr.Body.String())
			}
			if dispatched != tc.wantDispatched {
				t.Fatalf("dispatched: expected %q, got %q", tc.wantDispatched, dispatched)
			}
		})
	}
}
