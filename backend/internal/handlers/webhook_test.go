package handlers

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"mkauth/internal/db"
)

func TestWebhookRoute_RejectsInvalidActionsV2Signature(t *testing.T) {
	t.Setenv("ZITADEL_EVENT_SIGNING_KEY", "test-key")
	t.Setenv("MKAUTH_API_KEY", "irrelevant-for-webhook-route")

	body := []byte(`{"event_type":"user_created","user_id":"u1","source_project":"p1"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/webhooks/zitadel", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("ZITADEL-Signature", "t=0,v1=deadbeef")
	rr := httptest.NewRecorder()

	NewRouter().ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 from middleware, got %d body=%s", rr.Code, rr.Body.String())
	}
	got := decodeErrorResponse(t, rr)
	if got.Error != "INVALID_SIGNATURE" {
		t.Fatalf("expected INVALID_SIGNATURE error code, got %q", got.Error)
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

func TestHandleZitadelWebhook_TranslatesZitadelShape(t *testing.T) {
	t.Setenv("ZITADEL_M2M_USER_ID", "")
	setupNoopWebhookDeps(t)

	var seenRoles []string
	prevEnforce := webhookEnforceMappingRules
	t.Cleanup(func() { webhookEnforceMappingRules = prevEnforce })
	webhookEnforceMappingRules = func(_ context.Context, _, _, role string) error {
		seenRoles = append(seenRoles, role)
		return nil
	}

	body := []byte(`{
		"aggregate": {"id":"agg-1","type":"user","resourceOwner":"org-1"},
		"event": "user.grant.added",
		"editorUserId": "human-operator-1",
		"payload": {"userId":"u1","projectId":"p1","roleKeys":["alpha","beta"]}
	}`)
	rr := postWebhook(t, body)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}
	if !reflect.DeepEqual(seenRoles, []string{"alpha", "beta"}) {
		t.Fatalf("expected [alpha beta], got %v", seenRoles)
	}
}

func TestHandleZitadelWebhook_DropsSelfMutation(t *testing.T) {
	t.Setenv("ZITADEL_M2M_USER_ID", "service-user-99")
	setupNoopWebhookDeps(t)

	called := false
	prevEnforce := webhookEnforceMappingRules
	t.Cleanup(func() { webhookEnforceMappingRules = prevEnforce })
	webhookEnforceMappingRules = func(_ context.Context, _, _, _ string) error {
		called = true
		return nil
	}

	body := []byte(`{"aggregate":{"id":"u1"},"event":"user.grant.added","editorUserId":"service-user-99","payload":{"userId":"u1","projectId":"p1","roleKeys":["x"]}}`)
	rr := postWebhook(t, body)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 short-circuit, got %d", rr.Code)
	}
	if called {
		t.Fatal("orchestrator MUST NOT be called on self-mutation event")
	}
}

func TestHandleZitadelWebhook_LifecycleEvents_DispatchWithoutSourceProject(t *testing.T) {
	// Native Zitadel user lifecycle triggers carry no project context. The
	// translator returns EventType + UserID only; the handler must not reject
	// such events with a 400 over a missing source_project.
	cases := []struct {
		name      string
		event     string
		expectInv bool // user_deactivated / user_locked → cacheInvalidateUser
		expectOnb bool // user_created → onboarding trigger
	}{
		{"user.human.added → user_created", "user.human.added", false, true},
		{"user.deactivated → user_deactivated", "user.deactivated", true, false},
		{"user.locked → user_locked", "user.locked", true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("ZITADEL_M2M_USER_ID", "")
			setupNoopWebhookDeps(t)

			var invalidated bool
			var onboarded bool
			cacheInvalidateUser = func(_ context.Context, _ string) error {
				invalidated = true
				return nil
			}
			webhookTriggerOnboarding = func(_ context.Context, _, _, _ string) error {
				onboarded = true
				return nil
			}

			body := fmt.Appendf(nil,
				`{"aggregate":{"id":"u-lifecycle-1","resourceOwner":"org-1"},"event":"%s","editorUserId":"human-1","payload":{}}`,
				tc.event,
			)
			rr := postWebhook(t, body)
			if rr.Code != http.StatusOK {
				t.Fatalf("expected 200 (lifecycle event has no source_project), got %d body=%s", rr.Code, rr.Body.String())
			}
			if invalidated != tc.expectInv {
				t.Errorf("cache invalidate: want %v, got %v", tc.expectInv, invalidated)
			}
			if onboarded != tc.expectOnb {
				t.Errorf("onboarding: want %v, got %v", tc.expectOnb, onboarded)
			}
		})
	}
}

func TestHandleZitadelWebhook_GrantEvent_StillRequiresSourceProject(t *testing.T) {
	// Regression guard for the relaxed-validation fix: grant events must
	// still 400 when source_project is missing. Internal-shape (not Zitadel-
	// shape) so it bypasses the translator and exercises the handler's
	// validation block directly.
	setupNoopWebhookDeps(t)
	body := []byte(`{"event_type":"grant_added","user_id":"u1","role_keys":["alpha"]}`)
	rr := postWebhook(t, body)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 (grant event still requires source_project), got %d", rr.Code)
	}
}

func TestHandleZitadelWebhook_UnknownZitadelEvent_NoOp(t *testing.T) {
	t.Setenv("ZITADEL_M2M_USER_ID", "")
	setupNoopWebhookDeps(t)

	called := false
	prevEnforce := webhookEnforceMappingRules
	t.Cleanup(func() { webhookEnforceMappingRules = prevEnforce })
	webhookEnforceMappingRules = func(_ context.Context, _, _, _ string) error {
		called = true
		return nil
	}

	body := []byte(`{"aggregate":{"id":"u1"},"event":"user.password.changed","editorUserId":"human-1","payload":{}}`)
	rr := postWebhook(t, body)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 no-op, got %d body=%s", rr.Code, rr.Body.String())
	}
	if called {
		t.Fatal("orchestrator MUST NOT be called on unknown event")
	}
}
