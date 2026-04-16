# Live Webhook Listener — Implementation Record

**Phase:** 3 | **Status:** Complete | **Tests:** 13

## What Was Built
Event-type-aware webhook dispatch with 6 event types, event persistence with idempotency deduplication, and reverse orchestration for grant revocations.

### Event Types
| Event Type | Cache | Orchestration | Onboarding |
|-----------|-------|---------------|------------|
| `grant_added` | Rebuild | EnforceMappingRules | If role_key="new_user" |
| `grant_removed` | Invalidate | RevokeMappingRules | No |
| `grant_changed` | Rebuild | EnforceMappingRules | No |
| `user_deactivated` | Invalidate | No | No |
| `user_locked` | Invalidate | No | No |
| `user_created` | No | No | Yes |

### Key Design Choices
- HMAC signature as idempotency key (unique per payload+timestamp, no lossy bucketing)
- Role-aware revocation: `UpdateUserGrant` for multi-role grants, `RemoveUserGrant` for single-role
- `event_type` defaults to `grant_added` when absent (backward compat)

## Key Files
- `backend/internal/handlers/webhook.go` — event dispatch
- `backend/internal/handlers/webhook_events.go` — operator endpoint
- `backend/internal/zitadel/orchestrator.go` — RevokeMappingRules
- `backend/db/migrations/000007_webhook_events.up.sql`

## Verification
```bash
cd backend && go test ./... && go vet ./...  # 127 tests
```
