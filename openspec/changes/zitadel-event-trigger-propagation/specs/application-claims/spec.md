## MODIFIED Requirements

### Requirement: Actions v2 Target Deployment

The Zitadel Actions v2 target configuration and deployment assets MUST be maintained in the Syndra repository, and MUST cover **both** the function-trigger claim integration target (`syndra-claim-injector`) AND the event-trigger lifecycle listener target (`syndra-event-listener`).

* **Multi-target manifest**: `zitadel/actions/targets.json` MUST declare a `targets[]` array (not a single target object) so multiple Action targets can coexist. Each entry MUST include `name`, `endpoint`, `timeout`, `payloadType`, and exactly one of the `restCall` or `restAsync` submessages. The manifest MUST declare an `executions[]` array whose entries reference targets by **name** (`target: "syndra-claim-injector"`, `target: "syndra-event-listener"`); `register.sh` MUST resolve names to captured target IDs before issuing the SetExecution PUT.
* **Function-trigger target (`syndra-claim-injector`)**: type MUST be `restCall` (parses response body for `append_claims`); executions MUST cover `function.preaccesstoken` and `function.preuserinfo`.
* **Event-trigger target (`syndra-event-listener`)**: type MUST be `restAsync` (fire-and-forget — Zitadel does not block the originating actor on Syndra latency); endpoint MUST be `${SYNDRA_EXTERNAL_URL}/api/webhooks/zitadel`; executions MUST cover `condition.event` for at minimum `user.human.added`, `user.deactivated`, `user.locked`, `user.grant.added`, `user.grant.changed`, `user.grant.removed`. Self-registration (`user.human.selfregistered`) MAY also be bound when self-service signup is enabled. Event names MUST match Zitadel's registered event types exactly (`internal/repository/user/`, `internal/repository/usergrant/`); `register.sh` calls SetExecution which validates the condition against `EventExisting()` and rejects unknown names with HTTP 404 + `COMMAND-74aaqj8fv9`. In particular, deactivation and lock are user-aggregate events (`userEventTypePrefix + "deactivated"` / `+ "locked"`), NOT human-aggregate — `user.human.deactivated` and `user.human.locked` do not exist in Zitadel and MUST NOT appear in the manifest.
* **Single operator command**: `make zitadel-actions-register` MUST register both targets in a single invocation, capture both signing keys to per-target `.action-signing-key.<name>` files (mode `0600`), and emit the corresponding env-var pairs (`ZITADEL_ACTION_SIGNING_KEY=...`, `ZITADEL_EVENT_SIGNING_KEY=...`, plus their `_ROTATED_AT` companions) into a single `.action-env.fragment` apply file.
* **Per-target rotation**: `zitadel/actions/rotate.sh` MUST accept a `--target NAME` flag to rotate one target's signing key in place; calling without the flag MUST rotate every target in the manifest. Per-target `.action-signing-key.<name>{,.previous,.rotated_at}` files isolate the rotation surface so a leak or operator handoff for one target does not force rotation of the other.

#### Scenario: Both targets registered in a single operator command
- **WHEN** an operator runs `make zitadel-actions-register` against a fresh Zitadel instance
- **THEN** the script MUST create both `syndra-claim-injector` and `syndra-event-listener` targets via `POST /v2/actions/targets`
- **AND** MUST capture each target's `signingKey` to `.action-signing-key.<name>` (mode `0600`)
- **AND** MUST bind every execution in `targets.json` to the correct target ID via a single `PUT /v2/actions/executions`
- **AND** MUST emit `ZITADEL_ACTION_SIGNING_KEY=...` and `ZITADEL_EVENT_SIGNING_KEY=...` lines (plus their `_ROTATED_AT` companions) to `.action-env.fragment` for one-shot env-var application.

#### Scenario: Per-target signing key rotation
- **WHEN** an operator runs `make zitadel-actions-rotate-key TARGET=syndra-event-listener`
- **THEN** the script MUST rotate only the `syndra-event-listener` target via `POST /v2/actions/targets/{id}` with `{"expirationSigningKey":"0s"}`
- **AND** MUST overwrite `.action-signing-key.syndra-event-listener` with the new key, preserving the prior key at `.action-signing-key.syndra-event-listener.previous`
- **AND** MUST NOT rotate the `syndra-claim-injector` target's signing key.

### Requirement: Event-Trigger Authentication and Translation

The `/api/webhooks/zitadel` endpoint MUST authenticate Zitadel-origin event POSTs using the canonical Actions v2 signature scheme and translate the Zitadel-native event payload into the internal `WebhookPayload` shape before dispatch.

* **Single canonical signature scheme**: the endpoint MUST verify request signatures via the existing `withZitadelActionSignature` middleware keyed off `ZITADEL_EVENT_SIGNING_KEY`. The legacy `X-Zitadel-Signature` / `ZITADEL_WEBHOOK_SECRET` HMAC dialect MUST NOT coexist on this endpoint; both schemes for one boundary are explicitly disallowed.
* **Shape detection**: the handler MUST distinguish between Zitadel-native event payloads (top-level `aggregateID` field present — Zitadel's `ContextInfoEvent` wire format) and internal-shape callers (no `aggregateID`; the internal shape uses top-level `event_type` + `user_id`) by inspecting the parsed JSON. The two shapes MUST remain unambiguously distinguishable; Content-Type sniffing MUST NOT be used.
* **Internal-shape back-compat**: existing callers that POST the internal `WebhookPayload` schema (operator curl, contract tests, smoke tests) MUST continue to work without modification.
* **Wire format**: the translator MUST decode against Zitadel's actual `ContextInfoEvent` wire format (`zitadel/zitadel:internal/repository/execution/queue.go`): top-level flat fields `aggregateID`, `aggregateType`, `event_type`, `event_payload`, `userID` (the editor — NOT the subject). The shape detector MUST probe for `aggregateID`; bodies without that key MUST fall through to the internal-shape strict decoder. Smoke-test fixtures and unit-test bodies MUST use the real shape — the prior translator was built against a fictional `{aggregate:{id,...}, event, payload}` shape that no Zitadel-originated event actually emits, and tests passed only because they used the same fictional shape.
* **Translator coverage**: the translator MUST map the following Zitadel events into internal `event_type` values, populating per-event fields (`user_id`, `source_project`, `role_keys[]`, `grant_id`):
    * `user.human.added`, `user.human.selfregistered` → `user_created`
    * `user.deactivated` → `user_deactivated`
    * `user.locked` → `user_locked`
    * `user.grant.added` → `grant_added`
    * `user.grant.changed` → `grant_changed`
    * `user.grant.removed` → `grant_removed`
* **Grant enrichment**: `user.grant.changed` does not carry `projectId` and `user.grant.removed` does not carry `roleKeys` (verified against `zitadel/zitadel:internal/repository/usergrant/user_grant.go`). The translator MUST enrich those events from a local `zitadel_grants_index` table (populated by `grant.added`, refreshed by `grant.changed`, deleted on `grant.removed`); on a local-index miss it MUST fall back to a synchronous Zitadel `ListUserGrants` call. Both lookups are best-effort: when both fail, the translator MUST return the unenriched payload and let the handler/processor degrade gracefully — it MUST NOT 4xx Zitadel, since that produces redelivery storms with no clean resolution. Index maintenance ops (upsert/delete) MUST be non-fatal: failures log but MUST NOT block dispatch.
* **Multi-role grants**: the `WebhookPayload` schema MUST carry a `role_keys []string` field alongside the existing singular `role_key`. Grant events with multiple role keys MUST surface every role in the `role_keys` array; processors MUST iterate. One HTTP delivery MUST produce exactly one `webhook_events` row regardless of role count (idempotency key remains the `ZITADEL-Signature` header).
* **Unknown-event passthrough**: unmapped event types MUST receive `200 OK` with a `[WEBHOOK] unknown event` log line and MUST NOT result in dispatch or a `webhook_events` row.

#### Scenario: Bad signature on event POST
- **GIVEN** `ZITADEL_EVENT_SIGNING_KEY` is set
- **WHEN** a request arrives at `/api/webhooks/zitadel` with a missing or invalid `ZITADEL-Signature` header
- **THEN** the handler MUST respond `401 INVALID_SIGNATURE` and MUST NOT decode the body or dispatch.

#### Scenario: Internal-shape caller still accepted
- **WHEN** a caller POSTs an internal `WebhookPayload` (top-level `event_type` field) to `/api/webhooks/zitadel`
- **THEN** the handler MUST treat it as the internal shape and dispatch through the existing processor map without invoking the Zitadel translator.

#### Scenario: Multi-role grant added
- **WHEN** Zitadel POSTs a `user.grant.added` event with two role keys (`alpha`, `beta`)
- **THEN** the translator MUST populate `role_keys: ["alpha", "beta"]` on the produced `WebhookPayload`
- **AND** `processGrantAdded` MUST run mapping-rule enforcement for both roles within the same delivery
- **AND** exactly one `webhook_events` row MUST be inserted for the delivery.

#### Scenario: Unknown event acknowledged but not dispatched
- **WHEN** Zitadel POSTs an event with an unmapped `event_type` field (e.g. `user.password.changed`)
- **THEN** the handler MUST respond `200 OK` with no dispatch
- **AND** MUST NOT insert a `webhook_events` row.

#### Scenario: Grant event with unresolved enrichment acknowledged without dispatch
- **GIVEN** Zitadel POSTs a `user.grant.changed` or `user.grant.removed` event whose payload omits `projectId` / `roleKeys`
- **AND** neither the local `zitadel_grants_index` nor the Zitadel `ListUserGrants` fallback resolves the missing fields (e.g. the aggregate has been hard-deleted)
- **THEN** the handler MUST respond `200 OK` with a structured log line and MUST NOT dispatch downstream processors or insert a `webhook_events` row
- **AND** MUST NOT respond `400 Bad Request` (which would trigger Zitadel redelivery storms with no clean resolution).

### Requirement: Self-Mutation Loop Suppression

When the Syndra backend mutates Zitadel via the Management API (e.g. `RemoveUserGrant` from `RevokeMappingRules`), Zitadel emits the corresponding event back to the listener. The handler MUST detect and suppress these self-mutation echoes before dispatch.

* **Editor-based detection**: events whose top-level `userID` (Zitadel's `ContextInfoEvent` carries the editor in this field, NOT the subject) matches the configured `ZITADEL_M2M_USER_ID` (the backend's own service-user ID) MUST be dropped before any downstream processor runs.
* **Observable suppression**: dropped events MUST emit a `[WEBHOOK] dropped self-mutation` log line carrying enough context (event type, aggregate ID, editor ID) for operator debugging.
* **Idempotency safety net**: the `webhook_events.idempotency_key` deduplication MUST remain in place as a backup defense; the editor check is the primary defense, the idempotency table is the backup.
* **Disabled guard for local-dev**: when `ZITADEL_M2M_USER_ID` is unset, the handler MUST log a one-time startup warning and accept all events (acceptable in local-dev; explicitly required in any environment that accepts real Zitadel traffic).

#### Scenario: Backend-initiated grant revoke does not loop
- **GIVEN** `ZITADEL_M2M_USER_ID` is set to the backend's M2M service-user ID
- **WHEN** the backend calls Zitadel's `RemoveUserGrant`
- **AND** Zitadel emits the resulting `user.grant.removed` event back to `/api/webhooks/zitadel`
- **THEN** the translator MUST detect top-level `userID == ZITADEL_M2M_USER_ID` and respond `200 OK` without dispatch
- **AND** no `webhook_events` row MUST be inserted
- **AND** a `[WEBHOOK] dropped self-mutation` log line MUST be emitted.
