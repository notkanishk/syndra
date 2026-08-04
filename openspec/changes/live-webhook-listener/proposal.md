## Why

The webhook handler processed all events identically — no event type discrimination, no grant revocation flow, no event persistence for deduplication, and no user deactivation handling. When a role was revoked in Zitadel, Syndra couldn't cascade that revocation through mapping rules. Duplicate webhook deliveries were processed multiple times. Event history was invisible to operators.

## What Changes

* Extends `WebhookPayload` with an `event_type` field supporting 6 event types: `grant_added`, `grant_removed`, `grant_changed`, `user_deactivated`, `user_locked`, `user_created`. Defaults to `grant_added` when absent for backward compatibility.
* Adds event-type-specific dispatch: grant additions trigger cache rebuild + orchestration, grant removals trigger cache invalidation + reverse orchestration (`RevokeMappingRules`), user deactivation/lock triggers full cache invalidation, user creation triggers onboarding.
* Persists webhook events in a new `webhook_events` table with idempotency key deduplication. Uses the HMAC signature as idempotency key (unique per payload+timestamp, no lossy bucketing). Duplicate deliveries return 200 without reprocessing.
* Adds `RevokeMappingRules` to the orchestrator — fetches user's grants once, indexes by project:role, and performs role-aware revocation: updates multi-role grants (removing only the derived role via `UpdateUserGrant`) or deletes single-role grants (via `RemoveUserGrant`).
* Exposes `GET /api/v1/webhook/events` operator endpoint with optional `?status=` filter.

## Capabilities

### New Capabilities
* `webhook-event-dispatch`: Event-type-aware processing with 6 distinct event types.
* `grant-revocation-cascade`: Reverse orchestration removes derived grants when source roles are revoked.
* `webhook-event-persistence`: Events persisted for deduplication, audit, and operator visibility.

### Modified Capabilities
* `webhook-invalidation`: Upgraded from Partial to Integrated in feature coverage.
* `orchestration-engine`: Now supports both forward (EnforceMappingRules) and reverse (RevokeMappingRules) orchestration.

## Impact

* Adds `webhook_events` table (migration 000007).
* Modifies `webhook.go` (event dispatch), `orchestrator.go` (RevokeMappingRules), `deps.go` (injectable vars), `router.go` (new route), `repositories.go` (4 new functions).
* Creates `webhook_events.go` (operator endpoint).
* 13 new tests (8 webhook handler + 5 orchestrator).
* Zero new go.mod dependencies. Full backward compatibility preserved.
