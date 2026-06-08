> **Status:** Wave 2 · Part 4 delta — Zitadel state projection & drift control (B4, B2, D3) | [< Index](../../../../INDEX.md) | [Feature Coverage](../../../mkauth-core-architecture/specs/feature-coverage.md)

# Requirement: Access Governance (delta)

## ADDED Requirements

### Requirement: Every MkAuth-mediated Zitadel grant mutation MUST be recorded in the intent ledger before the Zitadel API call, within one transaction

The backend MUST NOT mutate a Zitadel `user_grant` through any MkAuth-mediated path without first durably recording the corresponding intent (`direct_role_grants` row with `source`, `source_ref`, `granted_by`, `reason`, `expires_at`), the audit entry, and an outbox row (`pending_zitadel_propagations`) in a single database transaction. The Zitadel call happens during the drain, after the intent is committed. Both `POST /api/v1/users/{id}/grants` and `POST /api/v1/zitadel/users/{id}/grants` resolve to this one path.

#### Scenario: Operator point mutation enqueues atomically

- **WHEN** an operator calls `POST /api/v1/users/{id}/grants` with a valid `project_id` and `role_key`
- **THEN** the backend MUST write the `direct_role_grants` row (`source='direct'`, `source_ref=NULL`), an `audit_logs` row (`direct_grant.upserted`), and a `pending_zitadel_propagations` row (`op_type='add'`, `status='pending'`, fresh `idempotency_key`) in one transaction
- **AND** the backend MUST respond `202` with `{outbox_id, idempotency_key, status:"pending"}`
- **AND** no Zitadel Management API call MUST have been issued yet

#### Scenario: Transactional rollback leaves no partial state

- **WHEN** the outbox insert fails after the `direct_role_grants` upsert within `EnqueueDirectGrantPropagation`
- **THEN** the transaction MUST roll back so that neither the grant row, the audit row, nor the outbox row is persisted
- **AND** the handler MUST respond with a `500` error and not report success

#### Scenario: Legacy /zitadel/* CRUD routes flow through the same canonical path

- **WHEN** an operator calls `POST /api/v1/zitadel/users/{id}/grants` (the backward-compatible alias)
- **THEN** the request MUST be handled by the canonical enqueue path, not by a direct `zitadelAddUserGrant` call
- **AND** the response shape MUST be `{outbox_id, idempotency_key, status}`, identical to `/api/v1/users/{id}/grants`

### Requirement: Buffered propagations MUST drain only on explicit operator action, and `applied` MUST be the terminal success state

The drain MUST be triggered by the operator (`POST /api/v1/propagations/drain`), MUST pre-flight Zitadel reachability, and MUST treat a `2xx` Management API response as terminal confirmation (`status='applied'`). There MUST be no dependence on a webhook return-trip to confirm a MkAuth-originated grant, because such events are dropped by the self-mutation guard.

#### Scenario: Drain halts cleanly when Zitadel is unreachable

- **WHEN** the operator triggers the drain and the `/zitadel/health` pre-flight reports unreachable
- **THEN** the drain MUST halt without transitioning any outbox row to `in_flight`
- **AND** the response MUST indicate the halt reason `zitadel_offline` so the UI can keep the pending callout visible with the resume button disabled

#### Scenario: Per-row ACK classification

- **WHEN** the drain dispatches a pending outbox row and the Management API returns `2xx`
- **THEN** the row MUST transition to `status='applied'` with `completed_at` set
- **AND WHEN** the call returns a `4xx`, the row MUST transition to `status='failed'` with `last_error` recorded, without halting the remaining rows
- **AND WHEN** the call returns `5xx` or times out, the row MUST return to `status='pending'` with `attempts` incremented, and the drain MUST halt once `attempts` exceeds `OUTBOX_MAX_RETRIES`

#### Scenario: Already-exists check short-circuits redundant and replayed mutations

- **WHEN** the drain processes an `add` outbox row whose `(user_id, project_id, role_key)` already exists in the webhook-derived grant index (or a live `ListUserGrants`)
- **THEN** the row MUST be marked `applied` without issuing a Management API call
- **AND** an `in_flight` row left by a process crash between the Zitadel ACK and the status update MUST resolve to `applied` on the next drain via this same check, with no double-write

### Requirement: Out-of-band Zitadel grants MUST be detected as drift and surfaced for operator triage

A grant that exists in Zitadel with no matching MkAuth expected record and no `external_grant_exclusions` entry MUST be recorded as a `drift_items` row and surfaced on `/governance/drift`. Detection is real-time via webhook with a scheduled reconciliation sweep (cap 2 000) as backstop. Triage offers exactly Attribute / Revoke / Mark external. No drift item resolves automatically.

#### Scenario: Webhook detects an externally-authored grant

- **WHEN** the webhook processes a `grant_added` event that survives the self-mutation guard, matches no `external_grant_exclusions` row, and matches no MkAuth expected grant
- **THEN** the backend MUST insert a `drift_items` row (`detection_source='webhook'`, `drift_type='zitadel_only'`, `status='pending_triage'`)
- **AND** a duplicate detection for the same `(user_id, project_id, drift_type)` while still `pending_triage` MUST NOT create a second row

#### Scenario: Reconciliation classifies rule-derived grants as expected, not drift

- **WHEN** the reconciliation sweep compares Zitadel grants against the expected set (`direct_role_grants ∪ bundle_expansions ∪ rule_outputs ∪ external_grant_exclusions`)
- **THEN** a Zitadel grant produced by an active mapping rule MUST be classified `expected_via_rule` and MUST NOT produce a `drift_items` row
- **AND** a grant present in MkAuth's expected set but absent from Zitadel (`mkauth_only`) MUST re-enqueue an outbox row rather than create a drift item

#### Scenario: Mark external suppresses future detections

- **WHEN** an operator resolves a drift item via Mark external
- **THEN** the backend MUST insert an `external_grant_exclusions` row keyed by `(user_id, project_id, role_key)` and mark the drift item `marked_external`
- **AND** future webhook and sweep detections for that triple MUST be silently filtered
