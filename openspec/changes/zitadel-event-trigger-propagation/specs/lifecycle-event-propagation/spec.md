## ADDED Requirements

### Requirement: Producer-Side Event Target Registration

The Zitadel Actions v2 deployment MUST register a target named `mkauth-event-listener` of type `restAsync` with `payloadType = PAYLOAD_TYPE_JSON` pointing at `${MKAUTH_EXTERNAL_URL}/api/webhooks/zitadel`, bound to `condition.event` executions for at minimum: `user.human.added`, `user.deactivated`, `user.locked`, `user.grant.added`, `user.grant.changed`, `user.grant.removed`. Each event name MUST be a real Zitadel event type — Zitadel's `ExecutionEventCondition.Existing()` validator rejects unknown names with HTTP 404 + `COMMAND-74aaqj8fv9` "Execution condition is invalid". Deactivation and lock are user-aggregate events; `user.human.deactivated` / `user.human.locked` are NOT registered in Zitadel and MUST NOT be used. The deployment MUST be applicable in one operator command — `make zitadel-actions-register` MUST register both the `mkauth-claim-injector` (function triggers) and `mkauth-event-listener` (event triggers) targets in a single invocation and capture both signing keys.

#### Scenario: Default bundle on first user creation
- **GIVEN** a welcome bundle is configured (`bundles.is_welcome = true` row exists)
- **AND** Zitadel's `mkauth-event-listener` target is registered and reachable
- **WHEN** an operator creates a human user in Zitadel via the console
- **THEN** `/api/webhooks/zitadel` MUST receive a `user.human.added` event with a valid signature
- **AND** `processUserCreated` MUST insert an `onboarding_triggers` row with status=`completed`
- **AND** `direct_role_grants` MUST contain rows for every role in the welcome bundle
- **AND** an audit row attributed to `system:onboarding` MUST exist

#### Scenario: Out-of-band grant added in Zitadel console
- **WHEN** an operator adds a project grant to a user via the Zitadel console
- **THEN** `/api/webhooks/zitadel` MUST receive a `user.grant.added` event with a valid signature
- **AND** `processGrantAdded` MUST run mapping-rule enforcement for every role in the grant
- **AND** the user's compiled cache MUST be rebuilt (or invalidated)

### Requirement: Consumer Authentication and Translation

`/api/webhooks/zitadel` MUST verify request signatures via the canonical Zitadel `ZITADEL-Signature` header scheme using `ZITADEL_EVENT_SIGNING_KEY`; the legacy `X-Zitadel-Signature` / `ZITADEL_WEBHOOK_SECRET` scheme MUST NOT coexist. The handler MUST accept Zitadel-native event payloads (top-level `aggregateID` field — Zitadel's `ContextInfoEvent` wire format from `zitadel/zitadel:internal/repository/execution/queue.go`) and translate them to the internal `WebhookPayload` shape. Existing internal-shape callers (top-level `event_type` field — operator curl, contract tests) MUST continue to work for back-compat. Multi-role grant events MUST surface every affected role via `WebhookPayload.RoleKeys`; idempotency MUST be per underlying event, not per HTTP delivery: Zitadel-shape events dedupe on the stable `aggregateID:eventType:sequence` tuple (never the `ZITADEL-Signature` header, whose embedded timestamp changes on every redelivery — July 2026 audit SC5), so a Zitadel retry of the same event produces exactly one `webhook_events` row and one dispatch, regardless of role count. Internal-shape callers dedupe on the payload-derived `eventType:userID:project:roleKey` key. Unmapped Zitadel event types MUST receive `200 OK` with a structured log line and MUST NOT result in dispatch or a `webhook_events` row. Grant events whose payload omits `projectId` (`user.grant.changed`) or `roleKeys` (`user.grant.removed`) MUST be enriched from a local `zitadel_grants_index` table populated by prior `grant.added` events, falling back to a synchronous Zitadel `ListUserGrants` call on miss. Both enrichment lookups are best-effort — when both fail, the handler MUST process the partial payload rather than 4xx the request (see `application-claims/spec.md` "Grant enrichment").

#### Scenario: Bad signature
- **GIVEN** `ZITADEL_EVENT_SIGNING_KEY` is set
- **WHEN** a request arrives at `/api/webhooks/zitadel` with a missing or invalid `ZITADEL-Signature` header
- **THEN** the handler MUST respond `401 INVALID_SIGNATURE`
- **AND** MUST NOT decode the body or dispatch

#### Scenario: Internal-shape caller still accepted
- **WHEN** a caller POSTs an internal `WebhookPayload` (top-level `event_type` field) to `/api/webhooks/zitadel`
- **THEN** the handler MUST treat it as the internal shape and dispatch through the existing processor map without invoking the Zitadel translator

#### Scenario: Multi-role grant added
- **WHEN** Zitadel POSTs a `user.grant.added` event with two role keys (`alpha`, `beta`)
- **THEN** the translator MUST populate `role_keys: ["alpha", "beta"]` on the produced `WebhookPayload`
- **AND** `processGrantAdded` MUST run mapping-rule enforcement for both roles within the same delivery
- **AND** exactly one `webhook_events` row MUST be inserted for the delivery

#### Scenario: Unknown event type acknowledged but not dispatched
- **WHEN** Zitadel POSTs an event with an unmapped `event` field (e.g. `user.password.changed`)
- **THEN** the translator MUST return `200 OK` with no dispatch and a `[WEBHOOK] unknown event` log line
- **AND** no `webhook_events` row MUST be inserted

### Requirement: Self-Mutation Loop Suppression

Events whose editor user ID matches `ZITADEL_M2M_USER_ID` MUST be dropped before dispatch — this is REQUIRED, not OPTIONAL, because backend-initiated Zitadel mutations otherwise echo back through Actions v2 and re-trigger orchestration. The translator MUST probe the editor-identity at Zitadel's documented `ContextInfoEvent` location: top-level `userID` (the editor; the subject of grant events lives in `event_payload.userId`, the subject of user-aggregate events is the `aggregateID`). On match, the translator MUST short-circuit with `200 OK`. Dropped events MUST emit a structured `[WEBHOOK] dropped self-mutation` log line carrying the event type, aggregate ID, and editor ID for operator debugging. The `webhook_events.idempotency_key` deduplication MUST remain in place as a backup defense. When `ZITADEL_M2M_USER_ID` is unset (acceptable only in local-dev), the handler MUST emit a one-time process-lifetime warning that the guard is disabled.

#### Scenario: Self-mutation loop suppression
- **GIVEN** `ZITADEL_M2M_USER_ID` is set to the backend's service-user ID
- **WHEN** the backend calls `RemoveUserGrant` via Management API
- **AND** Zitadel emits the corresponding `user.grant.removed` event back to the listener with top-level `userID == ZITADEL_M2M_USER_ID`
- **THEN** the translator MUST detect the match and short-circuit with `200 OK`
- **AND** a `[WEBHOOK] dropped self-mutation` log line carrying event, aggregate, and editor MUST be emitted
- **AND** no `webhook_events` row MUST be inserted

#### Scenario: Disabled guard warning in local-dev
- **GIVEN** `ZITADEL_M2M_USER_ID` is unset
- **WHEN** the first Zitadel-shape event arrives at `/api/webhooks/zitadel`
- **THEN** the handler MUST emit a one-time process-lifetime log line announcing that the self-mutation guard is disabled
- **AND** subsequent events in the same process MUST NOT re-emit the warning
- **AND** events MUST otherwise translate and dispatch normally
