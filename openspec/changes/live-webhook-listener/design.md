## Rationale

The webhook handler needed to evolve from a single-path processor to an event-type-aware dispatcher. The design choice was Option B: extend the existing `WebhookPayload` with an `event_type` discriminator rather than parsing native Zitadel wire format. This keeps the handler simple, testable, and decoupled from Zitadel's version-specific payload structure.

## Technical Specification

### 1. Event Type Dispatch

`WebhookPayload.EventType` defaults to `"grant_added"` when absent (backward compat). Validation is event-type-aware: `role_key` is required for grant events only.

| Event Type | Cache | Orchestration | Onboarding |
|-----------|-------|---------------|------------|
| `grant_added` | Rebuild | EnforceMappingRules | If role_key="new_user" |
| `grant_removed` | Invalidate | RevokeMappingRules | No |
| `grant_changed` | Rebuild | EnforceMappingRules | No |
| `user_deactivated` | Invalidate | No | No |
| `user_locked` | Invalidate | No | No |
| `user_created` | No | No | Yes |

### 2. Event Persistence and Deduplication

Events are persisted in `webhook_events` table before processing. The idempotency key is the HMAC signature from the `X-Zitadel-Signature` header — it is unique per payload+timestamp and avoids lossy bucketing that could suppress distinct events arriving in rapid succession (e.g. revoke/regrant cycles). In local-dev mode (no signature), the key falls back to `event_type:user_id:source_project:role_key:timestamp`. `ON CONFLICT DO NOTHING` catches exact duplicates.

### 3. Reverse Orchestration

`RevokeMappingRules(ctx, userID, sourceProjectID, sourceRoleKey)`:
1. Loads mapping rules via `dbGetActiveMappingRules`
2. Fetches user's grants once via `MgmtClient.ListUserGrants`
3. Indexes by `projectID:roleKey → full grant object` (not just grant ID — the full role list is needed)
4. For each matching rule:
   - **Single-role grant**: calls `MgmtClient.RemoveUserGrant` (deletes the grant)
   - **Multi-role grant**: calls `MgmtClient.UpdateUserGrant` with the remaining roles (preserves the user's other access on the same grant)
5. Errors logged but don't abort (same pattern as EnforceMappingRules)

The `ZitadelClient` interface includes `UpdateUserGrant(ctx, userID, grantID, roleKeys)` (`PUT /management/v1/users/{userId}/grants/{grantId}`) specifically for this role-aware revocation.

### 4. Operator Endpoint

`GET /api/v1/webhook/events?status=failed` — returns persisted events. Same pattern as `/api/v1/onboarding/triggers`.

### 5. Injectable Dependencies

All webhook processing functions (cache, orchestration, onboarding, persistence) are injectable via `handlers/deps.go` for isolated testing.

## Verification

```bash
cd backend && go build ./...
cd backend && go vet ./...
cd backend && go test ./...  # 127 tests pass
```
