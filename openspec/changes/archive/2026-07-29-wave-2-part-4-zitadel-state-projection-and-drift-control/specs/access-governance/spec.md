> **Status:** Wave 2 · Part 4 delta — Zitadel state projection & drift control (B4, B2, D3) | [< Index](../../../../INDEX.md) | [Feature Coverage](../../../syndra-core-architecture/specs/feature-coverage.md)

# Requirement: Access Governance (delta)

## ADDED Requirements

### Requirement: Every Syndra-mediated Zitadel grant mutation MUST be recorded in the intent ledger before the Zitadel API call, within one transaction

The backend MUST NOT mutate a Zitadel `user_grant` through any Syndra-mediated path without first durably recording the corresponding intent (`direct_role_grants` row with `source`, `source_ref`, `granted_by`, `reason`, `expires_at`), the audit entry, and an outbox row (`pending_zitadel_propagations`) in a single database transaction. The Zitadel call happens during the drain, after the intent is committed. Every path that creates a direct grant resolves to this one enqueue: `POST /api/v1/users/{id}/grants`, the `POST /api/v1/zitadel/users/{id}/grants` alias, AND the access-request approval path (`POST /api/v1/requests/{id}/decision` with `status=approved`). Approvals MUST NOT take the bare `UpsertDirectGrant` shortcut, because a ledger row with no matching outbox row is invisible to the Pending UI, never projected to Zitadel, and re-surfaces as `syndra_only` drift in reconciliation.

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

#### Scenario: Approving an access request enqueues and applies inline

- **WHEN** an operator approves a pending access request (`POST /api/v1/requests/{id}/decision`, `status=approved`)
- **THEN** the backend MUST resolve the request AND enqueue the grant (`op_type='add'`, `source='direct'`, `source_ref=<request id>`) in ONE transaction conditional on the request still being `pending`, NOT via a resolve-then-enqueue split and NOT via the bare `UpsertDirectGrant` — so a failed enqueue can never leave the request approved-but-ungranted
- **AND** a concurrent approve/reject that loses the race (request no longer `pending`) MUST return `409` (`ALREADY_RESOLVED`), not `500` and not a silent success
- **AND** the requester's compiled cache MUST be rebuilt from the committed ledger row BEFORE the inline drain and on a context DETACHED from the request lifecycle (bounded timeout), so access (which reads `direct_role_grants`) is effective immediately after commit and cannot be starved either by a slow/canceled drain sharing the request context or by a client disconnect
- **AND** it MUST then apply inline by draining ONLY this request's own outbox row (never the global batch); a drain failure MUST be non-fatal, leaving the row pending for a later resume
- **AND** a rejected request MUST NOT enqueue or upsert any grant, MUST resolve conditionally on `pending`, and MUST also return `409` on a lost race

### Requirement: Buffered propagations MUST drain only on explicit operator action, and `applied` MUST be the terminal success state

The drain MUST be triggered by the operator (`POST /api/v1/propagations/drain`), MUST pre-flight Zitadel reachability, and MUST treat a `2xx` Management API response as terminal confirmation (`status='applied'`). There MUST be no dependence on a webhook return-trip to confirm a Syndra-originated grant, because such events are dropped by the self-mutation guard.

The claim step MUST select both `pending` AND `in_flight` rows (the pending worklist and count report the same set), so a drain that crashed after claiming but before recording a terminal state leaves no orphaned `in_flight` row that is visible yet never re-driven. Because claiming `in_flight` rows would otherwise let a second drain re-dispatch a row the first drain is still processing, drains MUST be serialized by a session-level advisory lock: a drain that cannot acquire the lock MUST halt with reason `drain_in_progress` and MUST NOT claim or dispatch any row. Serialization guarantees the only `in_flight` rows a claiming drain ever sees are those orphaned by a crashed drain (whose session, and therefore lock, is gone). Marking a row terminal (`applied`/`failed`) or requeuing it MUST be the sole way a row leaves `in_flight`: the drain MUST NOT report a row as `applied`/`failed`/`requeued` unless that state was actually persisted, so a state-write failure never masquerades as success. An `applied` `revoke` or `replace` MUST reconcile the intent ledger (`direct_role_grants`) so Syndra stops treating removed roles as expected grants.

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
- **AND** an `in_flight` row left by a process crash between the Zitadel ACK and the status update MUST be re-claimed by the next drain (the claim covers `in_flight`) and resolve to `applied` via this same check, with no double-write
- **AND** a `replace` row MUST short-circuit ONLY when Zitadel already holds EXACTLY the desired role set; a superset (a superseded role still present) MUST NOT short-circuit, because the presence-only grant index cannot prove the absence of extras — `replace` therefore compares against a live `ListUserGrants` and issues `UpdateUserGrant` until the sets match exactly

#### Scenario: Applied revoke and replace reconcile the intent ledger

- **WHEN** a `revoke` outbox row is marked `applied` (whether by a `2xx`/`409` dispatch or by the already-exists short-circuit for a role already absent in Zitadel)
- **THEN** the backend MUST delete the named `(user_id, project_id, role_key)` rows from `direct_role_grants` so the revoked role is no longer treated as an expected grant by the access-decision compiler
- **AND WHEN** a `replace` outbox row is marked `applied`
- **THEN** the backend MUST delete any `source='direct'` `direct_role_grants` row on `(user_id, project_id)` whose role is not in the new set (the new roles were upserted at enqueue), leaving the ledger equal to the replace target
- **AND** the ledger reconcile MUST run BEFORE the terminal `applied` write, so a reconcile failure leaves the row `in_flight` for the next drain rather than stranding a terminal row beside a stale ledger

#### Scenario: A state-persistence failure is never reported as success

- **WHEN** the Zitadel outcome for a row is decided (`2xx`/`409`/`4xx`/transient) but the corresponding state write (`mark applied`, `mark failed`, `requeue`, or ledger reconcile) fails
- **THEN** the drain MUST NOT increment `applied`/`failed`/`requeued` for that row, MUST count it under `errored`, and MUST continue processing the remaining rows rather than halting the batch or returning overall success
- **AND** the row MUST remain `in_flight` so the next drain re-claims and re-drives it to a terminal state

#### Scenario: Concurrent drains are serialized

- **WHEN** a drain is triggered while another drain (a second `POST /api/v1/propagations/drain`, or an inline `?apply=true` drain) already holds the drain advisory lock
- **THEN** the second drain MUST halt with reason `drain_in_progress` without claiming, dispatching, or transitioning any row
- **AND** the lock MUST be released when the holding drain returns (including early halts), so the next drain proceeds normally

#### Scenario: Inline apply drains only the requesting row and reports its own status

- **WHEN** a grant mutation is submitted with `?apply=true`
- **THEN** the compiled cache MUST be rebuilt from the committed ledger row BEFORE the inline drain runs, on a context detached from the request lifecycle, so access is effective regardless of the drain outcome or a client disconnect
- **AND** the inline drain MUST target ONLY this request's own outbox row (`DrainOne` by `outbox_id`), never the global oldest-first batch — applying one grant MUST NOT project unrelated mutations an operator left queued
- **AND** the `202` response's `status` MUST reflect the current status of that row (read back by `outbox_id`), so a requeued/errored row reports its actual status (e.g. `pending`), never a false `applied`

### Requirement: Out-of-band Zitadel grants MUST be detected as drift and surfaced for operator triage

A grant that exists in Zitadel with no matching Syndra expected record and no `external_grant_exclusions` entry MUST be recorded as a `drift_items` row and surfaced on `/governance/drift`. Detection is real-time via webhook with a scheduled reconciliation sweep (cap 2 000) as backstop. Triage offers exactly Attribute / Revoke / Mark external. No drift item resolves automatically.

#### Scenario: Webhook detects an externally-authored grant

- **WHEN** the webhook processes a `grant_added` event that survives the self-mutation guard, matches no `external_grant_exclusions` row, and matches no Syndra expected grant
- **THEN** the backend MUST insert a `drift_items` row (`detection_source='webhook'`, `drift_type='zitadel_only'`, `status='pending_triage'`)
- **AND** a duplicate detection for the same `(user_id, project_id, drift_type)` while still `pending_triage` MUST NOT create a second row

#### Scenario: Reconciliation classifies rule-derived grants as expected, not drift

- **WHEN** the reconciliation sweep compares Zitadel grants against the expected set (`direct_role_grants ∪ bundle_expansions ∪ rule_outputs ∪ external_grant_exclusions`)
- **THEN** a Zitadel grant produced by an active mapping rule MUST be classified `expected_via_rule` and MUST NOT produce a `drift_items` row
- **AND** a grant present in Syndra's expected set but absent from Zitadel (`syndra_only`) MUST re-enqueue an outbox row rather than create a drift item

#### Scenario: Mark external suppresses future detections

- **WHEN** an operator resolves a drift item via Mark external
- **THEN** the backend MUST insert an `external_grant_exclusions` row keyed by `(user_id, project_id, role_key)` and mark the drift item `marked_external`
- **AND** future webhook and sweep detections for that triple MUST be silently filtered
