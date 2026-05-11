> **Status:** Wave 2 · Part 2 delta — Backend coherence (B6, C11, D8) | [< Index](../../../../INDEX.md)

# Requirement: Lifecycle Event Propagation (delta)

## ADDED Requirements

### Requirement: Internal-shape webhook callers with missing event_type MUST receive a structured 400

Internal-shape webhook callers (operator curl, contract tests, internal smoke scripts — any caller that bypasses Zitadel's `translateZitadelEvent` translation path) submitting a payload without an `event_type` field MUST receive a `400 VALIDATION_FAILED` response identical in shape to the existing missing-`user_id` and missing-`source_project` responses. The previous silent fallthrough that defaulted `event.EventType = "grant_added"` MUST be removed — it masked genuine producer bugs as successful grant additions.

This requirement applies only to internal-shape callers. Zitadel-shape callers are unaffected: `translateZitadelEvent` always resolves the event type before the strict check, and unknown Zitadel events still return `200 OK` with the existing `"event acknowledged, no dispatch"` message to prevent redelivery storms.

#### Scenario: Internal-shape payload missing event_type returns 400

- **WHEN** an operator submits an internal-shape webhook payload `{"user_id":"u1","source_project":"p1","role_keys":["r1"]}` without an `event_type` field
- **THEN** the handler MUST respond with `400 VALIDATION_FAILED`
- **AND** the response body MUST include `"details":{"event_type":"required"}`
- **AND** the handler MUST NOT default `event.EventType` to `"grant_added"` or any other value
- **AND** the handler MUST NOT invoke `processGrantAdded`, `processGrantRemoved`, `processUserDeactivated`, or `processUserCreated`

#### Scenario: Internal-shape payload with explicit event_type still validates as today

- **WHEN** an operator submits an internal-shape webhook payload `{"event_type":"grant_added","user_id":"u1","source_project":"p1","role_keys":["r1"]}`
- **THEN** the handler MUST continue to behave exactly as before this delta — accept the payload, validate downstream fields, and dispatch normally

#### Scenario: Zitadel-shape unknown event type still 200-acknowledges

- **WHEN** Zitadel posts an event whose translation yields `translated.EventType == ""`
- **THEN** the handler MUST respond with `200 OK` and message `"event acknowledged, no dispatch"` exactly as today
- **AND** the strict `event_type` 400 path MUST NOT fire for translated Zitadel payloads

### Requirement: Webhook events dropped for unresolvable enrichment MUST persist with a distinguishable status

When a Zitadel-shape grant event (`grant.added`, `grant.removed`, `grant.changed`) reaches the handler and the enrichment step at `webhook.go:104-109` cannot resolve the `source_project` or `role_keys`, the existing storm-prevention `200 OK` response MUST be preserved (Zitadel's redelivery back-off would otherwise loop indefinitely on grant events for already-removed aggregates). However the dropped event MUST be persisted to `webhook_events` with `status = 'dropped_enrichment_incomplete'` so operators can observe the volume of silently-dropped events without scraping stdout.

#### Scenario: Zitadel grant.removed for unresolvable aggregate persists as dropped, not as success

- **WHEN** Zitadel posts a `grant.removed` event for an aggregate that no longer exists in the local grants index and is not present in `ListUserGrants`
- **AND** `enrichGrantPayload` returns a payload with `SourceProject == ""` and `len(RoleKeys) == 0`
- **THEN** the handler MUST respond with `200 OK` and message `"grant event acknowledged, dispatch skipped (enrichment incomplete)"`
- **AND** the handler MUST insert a row into `webhook_events` with `status = 'dropped_enrichment_incomplete'`
- **AND** the row MUST be queryable via `GET /api/v1/webhook/events?status=dropped_enrichment_incomplete`

#### Scenario: Successful Zitadel grant event still records status='success'

- **WHEN** Zitadel posts a `grant.added` event whose enrichment fully resolves the `source_project` and `role_keys`
- **THEN** the existing `success` / `failed` paths MUST behave exactly as today
- **AND** `webhook_events.status` MUST NOT be set to `dropped_enrichment_incomplete` for any successfully-dispatched event

(Audit refs: B6, C11, D8)
