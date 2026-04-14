## 1. Database

- [x] 1.1 Create `000007_webhook_events.up.sql` migration
- [x] 1.2 Create `000007_webhook_events.down.sql` migration
- [x] 1.3 Add `WebhookEvent` struct to `repositories.go`
- [x] 1.4 Add `InsertWebhookEvent` with idempotency (ON CONFLICT DO NOTHING)
- [x] 1.5 Add `CompleteWebhookEvent` and `FailWebhookEvent`
- [x] 1.6 Add `GetWebhookEvents` with optional status filter

## 2. Reverse orchestration

- [x] 2.1 Add `RevokeMappingRules` to `orchestrator.go`
- [x] 2.2 Fetch grants once and index by projectID:roleKey → full grant object
- [x] 2.3 Role-aware revocation: UpdateUserGrant for multi-role grants, RemoveUserGrant for single-role grants
- [x] 2.4 Add `UpdateUserGrant` to ZitadelClient interface and managementClient (PUT endpoint)
- [x] 2.5 Graceful degradation when MgmtClient is nil

## 3. Webhook handler

- [x] 3.1 Add `event_type` field to `WebhookPayload`
- [x] 3.2 Default to `grant_added` when absent (backward compat)
- [x] 3.3 Validate event_type against known types
- [x] 3.4 Make role_key optional for non-grant events
- [x] 3.5 Add event persistence and deduplication
- [x] 3.6 Implement event-type dispatch (switch statement)
- [x] 3.7 Update event status after processing (complete/fail)

## 4. Injectable deps

- [x] 4.1 Add `cacheInvalidateUser` to `deps.go`
- [x] 4.2 Add webhook orchestration vars (`webhookEnforceMappingRules`, `webhookRevokeMappingRules`, `webhookTriggerOnboarding`)
- [x] 4.3 Add webhook event DB vars
- [x] 4.4 Refactor webhook handler to use injectable vars

## 5. Operator endpoint

- [x] 5.1 Create `webhook_events.go` handler
- [x] 5.2 Add `GET /api/v1/webhook/events` route

## 6. Tests

- [x] 6.1 `TestWebhook_EventTypeDefault` — omitted field defaults to grant_added
- [x] 6.2 `TestWebhook_GrantRemoved` — cache invalidation + revocation
- [x] 6.3 `TestWebhook_UserDeactivated` — cache invalidation only
- [x] 6.4 `TestWebhook_UserCreated` — onboarding trigger
- [x] 6.5 `TestWebhook_InvalidEventType` — 400
- [x] 6.6 `TestWebhook_DeduplicationSkips` — duplicate returns 200 without processing
- [x] 6.7 `TestWebhook_GrantAddedRequiresRoleKey` — 400 without role_key
- [x] 6.8 `TestWebhook_UserDeactivatedNoRoleKeyRequired` — passes without role_key
- [x] 6.9 `TestRevokeMappingRules_NilClient` — graceful skip
- [x] 6.10 `TestRevokeMappingRules_SoleRole_RemovesGrant` — single-role grant deleted via RemoveUserGrant
- [x] 6.11 `TestRevokeMappingRules_MultiRole_UpdatesGrant` — multi-role grant updated via UpdateUserGrant, preserving other roles
- [x] 6.12 `TestRevokeMappingRules_NoMatchingGrant` — no removal attempted
- [x] 6.13 `TestRevokeMappingRules_ErrorContinues` — tries all rules

## 7. P1 fixes

- [x] 7.1 Replace 10-second bucket idempotency key with HMAC signature (unique per payload+timestamp)
- [x] 7.2 Role-aware revocation: UpdateUserGrant for multi-role grants instead of unconditional RemoveUserGrant
- [x] 7.3 Add UpdateUserGrant to ZitadelClient interface and managementClient
- [x] 7.4 Update tests: split MatchingRule into SoleRole and MultiRole cases
