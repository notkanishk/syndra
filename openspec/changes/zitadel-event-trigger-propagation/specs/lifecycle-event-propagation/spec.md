# Capability: lifecycle-event-propagation

> **Status:** ADDED in zitadel-event-trigger-propagation
> **Origin:** zitadel-event-trigger-propagation
> **See also:** application-claims (claim integration), webhook-invalidation (downstream dispatch)

## Purpose

Lifecycle changes that originate in Zitadel (operator console, SCIM, or future external IdP triggers) MUST reach MkAuth's orchestration plane within the Actions v2 retry window so welcome-bundle assignment, mapping-rule cascade, cache invalidation, and LLDAP provisioning intents fire automatically.

## Requirements

### Producer
- The Zitadel Actions v2 deployment MUST register a target named `mkauth-event-listener` of type `restAsync` with `payloadType = PAYLOAD_TYPE_JSON` pointing at `${MKAUTH_EXTERNAL_URL}/api/webhooks/zitadel`.
- The deployment MUST bind executions for at minimum: `user.human.added`, `user.human.deactivated`, `user.human.locked`, `user.grant.added`, `user.grant.changed`, `user.grant.removed`.
- The deployment MUST be applicable in one operator command. `make zitadel-actions-register` MUST register both the `mkauth-claim-injector` (function triggers) and `mkauth-event-listener` (event triggers) targets in a single invocation and capture both signing keys.

### Consumer
- `/api/webhooks/zitadel` MUST verify request signatures via the canonical Zitadel `ZITADEL-Signature` header scheme using `ZITADEL_EVENT_SIGNING_KEY`. The legacy `X-Zitadel-Signature` / `ZITADEL_WEBHOOK_SECRET` scheme MUST be removed.
- The handler MUST accept Zitadel-native event payloads (top-level `aggregate` field) and translate them to the internal `WebhookPayload` shape. Existing internal-shape callers (top-level `event_type`) MUST continue to work for back-compat.
- Multi-role grant events MUST surface every affected role; idempotency MUST remain per HTTP delivery (one delivery = one `webhook_events` row).
- Events whose `editorUserId` matches `ZITADEL_M2M_USER_ID` MUST be dropped before dispatch with a log line. Loop suppression is REQUIRED, not OPTIONAL.

## Scenarios

### Scenario: Default bundle on first user creation
- **GIVEN** a welcome bundle is configured (`bundles.is_welcome = true` row exists)
- **AND** Zitadel's `mkauth-event-listener` target is registered and reachable
- **WHEN** an operator creates a human user in Zitadel via the console
- **THEN** `/api/webhooks/zitadel` MUST receive a `user.human.added` event with a valid signature
- **AND** `processUserCreated` MUST insert an `onboarding_triggers` row with status=`completed`
- **AND** `direct_role_grants` MUST contain rows for every role in the welcome bundle
- **AND** an audit row attributed to `system:onboarding` MUST exist

### Scenario: Out-of-band grant added in Zitadel console
- **WHEN** an operator adds a project grant to a user via the Zitadel console
- **THEN** `processGrantAdded` MUST run mapping-rule enforcement for every role in the grant
- **AND** the user's compiled cache MUST be rebuilt (or invalidated)

### Scenario: Self-mutation loop suppression
- **GIVEN** `ZITADEL_M2M_USER_ID` is set to the backend's service-user ID
- **WHEN** the backend calls `RemoveUserGrant` via Management API
- **AND** Zitadel emits the corresponding `user.grant.removed` event back to the listener
- **THEN** the translator MUST detect `editorUserId == ZITADEL_M2M_USER_ID` and short-circuit with `200 OK` + a `[WEBHOOK] dropped self-mutation` log line
- **AND** no `webhook_events` row MUST be inserted

### Scenario: Unknown event type
- **WHEN** Zitadel POSTs an event with an unmapped `event` field (e.g. `user.password.changed`)
- **THEN** the translator MUST return `200 OK` with no dispatch and a `[WEBHOOK] unknown event` log line
- **AND** no `webhook_events` row MUST be inserted

### Scenario: Bad signature
- **GIVEN** `ZITADEL_EVENT_SIGNING_KEY` is set
- **WHEN** a request arrives at `/api/webhooks/zitadel` with a missing or invalid `ZITADEL-Signature` header
- **THEN** the handler MUST respond `401 INVALID_SIGNATURE` and MUST NOT decode the body
