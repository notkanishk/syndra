## ADDED Requirements

### Requirement: Every apply MUST cite a persisted plan, on every target including Zitadel

Plan-then-apply MUST be a backend guarantee rather than a convention resting on the client re-sending the same request. This is the canonical statement of the rule; other capabilities reference it rather than restating it.

The rehearsal request MUST persist its outcomes under a plan identifier with a bounded lifetime and return that identifier, recording for each subject the intended outcome and a fingerprint of the state that was reviewed. The apply request MUST cite that identifier rather than re-submitting the original request for recomputation, and the backend MUST re-verify every fingerprint before dispatching. This applies to `POST /api/v1/grants/bulk`, `POST /api/v1/requests/bulk-decision`, `POST /api/v1/governance/drift/bulk-attribute`, `POST /api/v1/governance/drift/bulk-mark-external`, and every add-on target operation.

The fingerprint MUST cover the object the operator reviewed, not only the subject's grants — for drift triage it MUST include the drift row's own status, so that a row resolved by someone else while the operator was reading the list fails verification instead of being re-resolved.

The sole exemption is an operation scoped to `member` acting on the acting subject alone, which has no cohort and no diff to review and MUST be dispatched synchronously.

A plan MUST be spent by at most one apply, and the transition that spends it MUST be the apply's authority rather than a check the caller performs. A lifetime bounds how long a plan may be cited but not how many times it may be cited, and while a first apply's rows are still waiting in the outbox the target has not moved, so fingerprint verification would not catch the repeat.

A plan MUST be citable only by the operator who approved it, and only on the surface and target it was issued for. An approval is a person's judgement about a specific diff on a specific screen, and an identifier read out of a log or a screenshot is not that judgement.

A persisted plan row MUST record the decision and MUST NOT record its rendering. Every value it stores MUST be either a member of a closed vocabulary the backend owns or an identifier the backend allocated, and MUST be refused at the write if it is neither. A closed vocabulary MUST be closed at compile time rather than merely small — a mutable package-level collection is widenable by any caller before the check that reads it, which is not a vocabulary but a default. An identifier MUST be verified by lookup rather than by pattern: a syntactically valid identifier that names no row is not a reference, and a plan MUST NOT cite a row belonging to a subject other than the one whose plan row cites it, nor a desired-state snapshot taken for a different subject or a different target — a foreign key proves existence, and existence was never the property. Cited identifiers MUST be canonicalised before they are compared or stored, so that a citation differing only in case from the stored form is recognised as the same identifier rather than refused as a fabricated one. Free text is not sufficient protection against a declared secret reaching durable storage: a submitted secret is itself a string, so a struct of free-text fields is a route to the column however carefully its first writer avoids it, and no character class distinguishes a password from a role name. The sentences a rehearsal displays MUST be rendered from the recorded decision when a plan is read, not stored beside it.

Every per-subject row MUST record a fingerprint, and a plan MUST NOT be persisted with a row that has none. Verification compares a recorded fingerprint against a live read, so an absent recorded value would match an absent live one and the subject would pass verification precisely when the target could not be read.

#### Scenario: An approval is spent once

- **WHEN** an apply cites a plan identifier that has already been applied
- **THEN** the backend MUST reject it and MUST NOT enqueue or dispatch anything a second time
- **AND** two applies citing one identifier concurrently MUST result in exactly one of them proceeding

#### Scenario: An approval belongs to the operator who gave it

- **WHEN** an admin cites a plan identifier approved by a different operator, or cites a plan on a surface or target other than the one it was issued for
- **THEN** the backend MUST reject the apply
- **AND** the refusal MUST distinguish "this is not your approval" from "this approval has expired", because they are different operator actions

#### Scenario: A plan row cannot hold a submitted value

- **WHEN** any value on a per-subject plan row is neither a member of the backend's closed vocabulary nor an identifier the backend allocated
- **THEN** the backend MUST refuse to persist the plan before contacting the database
- **AND** the refusal MUST NOT echo the rejected value, which is the likeliest thing on the row to be a misplaced secret

#### Scenario: A well-formed identifier that names nothing is still refused

- **WHEN** a plan cites an identifier that is syntactically valid but was never allocated, or that was allocated to a different subject, or — for a desired-state snapshot — was taken for a different target
- **THEN** the backend MUST refuse to persist the plan, having read the identifier's row rather than only its shape
- **AND** the refusal MUST distinguish an identifier that names nothing from one that names another subject's row and from one that names another target's
- **AND** MUST NOT disclose who that other subject is or which target it was taken for

#### Scenario: A citation differing only in case is the same identifier

- **WHEN** a plan cites an identifier whose text differs in case from the form the database stores
- **THEN** the backend MUST treat it as the identifier it names rather than refusing it as unallocated
- **AND** MUST persist the canonical form, so that every later comparison is against what the database returns

#### Scenario: A plan cannot record an unverifiable subject

- **WHEN** a rehearsal would persist a subject row without a fingerprint of the state it reviewed
- **THEN** the backend MUST refuse to persist the plan rather than store a row that verifies vacuously

#### Scenario: Apply cites the plan the operator reviewed

- **WHEN** an operator rehearses a bulk or drift-triage action and then applies it
- **THEN** the rehearsal MUST return a plan identifier
- **AND** the apply MUST cite that identifier rather than causing the plan to be recomputed
- **AND** the backend MUST reject an apply that cites no identifier

#### Scenario: Verification happens at dispatch, not at acceptance

- **WHEN** an apply is accepted and its rows wait in the outbox before an operator resumes the drain
- **THEN** fingerprints MUST be verified again at dispatch
- **AND** a subject whose state moved after acceptance but before dispatch MUST NOT be written

#### Scenario: A grant set that moved invalidates the plan

- **WHEN** a subject's grants change between the rehearsal and the apply
- **THEN** the recorded fingerprint MUST no longer match
- **AND** the backend MUST reject the apply without mutating any subject
- **AND** the error MUST name the subjects whose state changed

#### Scenario: A concurrently resolved drift row invalidates the plan

- **WHEN** a drift row in the plan is resolved by another operator before the apply
- **THEN** fingerprint verification MUST fail for that row
- **AND** the apply MUST NOT re-resolve it

#### Scenario: Plan identifiers expire

- **WHEN** an apply cites a plan identifier that is unknown or past its lifetime
- **THEN** the backend MUST reject it and MUST NOT mutate any subject

#### Scenario: Member self-service is exempt

- **WHEN** a member invokes a `member`-scoped operation against themselves
- **THEN** it MUST dispatch synchronously without a plan
- **AND** no other operation MUST be exempt on that basis

### Requirement: The cohort guard MUST live where the cohort is known

An operation's affected-subject count MUST be computed and enforced by the backend at plan time, because the backend is the only component that holds the cohort — a per-subject apply call cannot know how many subjects an operation touches. The backend MUST refuse to issue a plan exceeding the configured subject count without an explicit scope acknowledgement. An add-on MAY additionally cap the subjects one request may affect as defence in depth, but MUST NOT be the sole enforcement point.

#### Scenario: An oversized cohort is refused at plan time

- **WHEN** an operation would affect more subjects than the configured limit and carries no scope acknowledgement
- **THEN** the backend MUST refuse to issue the plan
- **AND** MUST report the computed subject count so the operator can acknowledge it deliberately
- **AND** no add-on MUST have been called

### Requirement: Unconfirmed revocations MUST drain ahead of grants and carry a containment path

Revocations MUST be dispatched before grants for the same target, and MUST be eligible for background draining while grants remain operator-gated, because a delayed grant withholds access while a delayed revocation retains it. A revocation exhausting its retry budget MUST escalate onto the unconfirmed-revocation surface as a finding rather than silently halting a background pass, since no operator is watching a background loop. The background runner MUST back off rather than spin when it cannot take the drain lock, and MUST pre-flight target reachability so an unreachable target costs a probe rather than a retry budget. When a target is unreachable and access must end immediately, the escalation surface MUST carry the out-of-band procedure for that target, since the backend has no path to it. A change an operator makes out of band to contain an incident MUST be recognised by the drift sweep as reconciling the outstanding revocation, not raised as fresh drift.

#### Scenario: Revocations are dispatched first

- **WHEN** the drain has both revocations and grants queued for one target
- **THEN** it MUST dispatch the revocations before the grants

#### Scenario: An exhausted retry budget escalates rather than halting silently

- **WHEN** a revocation exceeds its retry budget in a background pass
- **THEN** it MUST appear on the unconfirmed-revocation surface as a finding carrying its last error
- **AND** MUST NOT be left as a silently halted row

#### Scenario: Lock contention slows the runner rather than starving it

- **WHEN** the background runner cannot acquire the drain lock because an operator drain holds it
- **THEN** it MUST back off and retry later
- **AND** MUST NOT spin, and MUST NOT be starved indefinitely by repeated operator drains

#### Scenario: Escalation carries the manual procedure

- **WHEN** an unconfirmed revocation crosses its age threshold and the target is unreachable
- **THEN** the surface MUST present the out-of-band containment procedure for that target

#### Scenario: Out-of-band containment reconciles rather than alerts

- **WHEN** an operator removes the access out of band and the drift sweep next runs
- **THEN** the sweep MUST reconcile it against the outstanding revocation
- **AND** MUST NOT raise it as new untraced drift

### Requirement: Role-to-target mappings MUST be versioned, validated, and the sole source of role-derived entitlements

A mapping binds a role on a project to a value for a field the target's entitlement schema declares. Mappings MUST be versioned with the same change history, rollback, and audit as bundle definitions, because editing one silently changes what every holder of that role can reach. The resolver MUST derive the role half of a subject's entitlement set from these mappings and from nothing else. A mapping write MUST be validated for structure by the backend and for reference by the add-on, and MUST be rejected if the add-on cannot confirm the value resolves on its target.

#### Scenario: Mapping edits are versioned and reversible

- **WHEN** an operator changes a role-to-target mapping
- **THEN** the change MUST be recorded as a new version with its actor and time
- **AND** the previous version MUST remain available to roll back to

#### Scenario: A mapping naming an unresolvable value is rejected

- **WHEN** an operator submits a mapping whose value does not resolve on the target
- **THEN** the add-on MUST report the value as unresolvable
- **AND** the backend MUST reject the mapping without persisting it

#### Scenario: A mapping naming an undeclared field is rejected

- **WHEN** an operator submits a mapping for a field absent from the target's declared entitlement schema
- **THEN** the backend MUST reject it without calling the add-on

#### Scenario: A mapping naming a lifecycle field is rejected

- **WHEN** an operator submits a mapping whose field governs whether the account or its service access is enabled
- **THEN** the backend MUST reject it during structural validation
- **AND** lifecycle state MUST remain resolver-computed and allowance-overridable only

#### Scenario: Editing a mapping is planned like any other bulk effect

- **WHEN** an operator edits or deletes a mapping held by existing subjects
- **THEN** the backend MUST plan the effect across every affected subject before applying
- **AND** the change MUST be subject to the same blast-radius guard as any other bulk effect

### Requirement: Allowances MUST be an explicit third access band, never inferred

Access on an add-on target derives from a Zitadel role the operator maps to target resources. Anything beyond that — a quota, a specific path, a restriction — MUST be recorded as an explicit Syndra allowance with an actor and a timestamp, and MUST NOT be inferred from role membership. The access lineage surface MUST present allowances as a band distinct from source roles and derived grants, so the question "why does this user have access to X" resolves to exactly one of: the role grants it, a rule derived it, or a named operator granted it.

#### Scenario: Lineage attributes access to exactly one origin

- **WHEN** an operator inspects a subject's access on a target
- **THEN** each entitlement MUST be attributed to a source role, a derivation rule, or an explicit allowance
- **AND** an allowance MUST display the granting actor and the time it was granted

#### Scenario: Allowances are not synthesized from roles

- **WHEN** a subject holds a target-granting role and no allowance is recorded
- **THEN** the allowance band MUST be empty for that subject
- **AND** the backend MUST NOT create an allowance as a side effect of the role grant

### Requirement: Subtractive allowances MUST be bounded by an expiry or a review date

An allowance that removes access the subject's role would otherwise grant MUST carry either an expiry, making it a self-lapsing suspension, or — where the suspension is genuinely indefinite — a mandatory review date that surfaces in governance once it passes. A denial with neither MUST be rejected, because an open-ended carve-out nobody is prompted to revisit becomes permanent by inattention. Permanent removal of one person's access MUST be expressed by revoking that person's role grant, never by editing the role mapping, which changes access for every holder of that role.

#### Scenario: A denial with neither bound is rejected

- **WHEN** an operator submits a subtractive allowance with no expiry and no review date
- **THEN** the backend MUST reject it
- **AND** the error MUST offer the two valid forms and name role-grant revocation as the permanent per-person path

#### Scenario: An indefinite suspension surfaces at its review date

- **WHEN** a subtractive allowance with no expiry passes its review date
- **THEN** it MUST surface in governance for an explicit decision
- **AND** it MUST remain in force until that decision is made

#### Scenario: Expiring suspension restores role-derived access

- **WHEN** a subtractive allowance reaches its expiry
- **THEN** the expiry sweep MUST remove it and re-converge the subject to their role-derived entitlements
- **AND** the restoration MUST be recorded in the audit trail

#### Scenario: Carve-out is visible wherever the role appears

- **WHEN** a subject holds a role whose access is partially suspended by a subtractive allowance
- **THEN** every surface presenting that role for that subject MUST show the carve-out — the user view, project role-holder lists, filtered cohorts, and bulk selection alike
- **AND** MUST NOT present the role's full access as effective
- **AND** a role-holder list MUST NOT count a suspended subject as holding the access it lists

### Requirement: Unconfirmed revocations MUST escalate as a security finding

Because target propagation fails open, a revocation may be recorded in Syndra without being confirmed on the target. Unconfirmed revocations MUST be surfaced on a dedicated operator view alongside drift triage, and MUST escalate to a distinct, more urgent presentation once they exceed a configured age.

#### Scenario: Queued revoke is surfaced apart from queued grants

- **WHEN** a revocation is recorded but not confirmed on the target
- **THEN** it MUST appear on the unconfirmed-revocation surface
- **AND** MUST be counted apart from succeeded operations
- **AND** MUST NOT be presented with the same urgency as an unconfirmed grant

#### Scenario: Age threshold escalates the presentation

- **WHEN** an unconfirmed revocation exceeds the configured age threshold
- **THEN** the surface MUST present it as a live security finding rather than a pending task
- **AND** MUST state how long the subject has retained access beyond the recorded revocation

## MODIFIED Requirements

### Requirement: Every Syndra-mediated Zitadel grant mutation MUST be recorded in the intent ledger before the Zitadel API call, within one transaction

The backend MUST NOT mutate a Zitadel `user_grant` through any Syndra-mediated path without first durably recording the corresponding intent (`direct_role_grants` row with `source`, `source_ref`, `granted_by`, `reason`, `expires_at`), the audit entry, and an outbox row (`propagation_outbox`) in a single database transaction. The Zitadel call happens during the drain, after the intent is committed. Every path that creates a direct grant resolves to this one enqueue: `POST /api/v1/users/{id}/grants`, the `POST /api/v1/zitadel/users/{id}/grants` alias, AND the access-request approval path (`POST /api/v1/requests/{id}/decision` with `status=approved`). Approvals MUST NOT take the bare `UpsertDirectGrant` shortcut, because a ledger row with no matching outbox row is invisible to the Pending UI, never projected to Zitadel, and re-surfaces as `syndra_only` drift in reconciliation.

The outbox table is `propagation_outbox`. It was `pending_zitadel_propagations`, a name that becomes false the moment a second target exists; it carries a `target` column resolved against the `targets` registry, and its Zitadel-shaped columns (`project_id`, `role_keys`, `zitadel_grant_id`) are nullable for rows whose target is not Zitadel and MUST remain populated for rows whose target is. `direct_role_grants` gains no target column: direct grants are intents against Zitadel `user_grant`s, and add-on entitlements come from mappings and allowances instead.

#### Scenario: A Zitadel outbox row cannot be written half-formed

- **WHEN** an outbox row names `target='zitadel'`
- **THEN** the database MUST refuse it unless `project_id` and `role_keys` are both present
- **AND** the relaxation that lets an add-on row omit them MUST NOT relax them for Zitadel

### Requirement: Out-of-band Zitadel grants MUST be detected as drift and surfaced for operator triage

A grant that exists on a target with no matching Syndra expected record and no `external_grant_exclusions` entry MUST be recorded as a `drift_items` row and surfaced on `/governance/drift`. Detection is real-time via webhook with a scheduled reconciliation sweep (cap 2 000) as backstop. Triage offers exactly Attribute / Revoke / Mark external. No drift item resolves automatically.

The drift type MUST NOT name a target inside its own value. `zitadel_only` becomes `target_only` — "present on the target, unexplained by Syndra" — with the target named by the row's `target` column beside it, because `drift_type='zitadel_only'` on a TrueNAS drift row is a false statement. `syndra_only` is unchanged. The pending-dedupe unique index and the `external_grant_exclusions` primary key MUST both include the target, so two targets drifting on one user cannot suppress each other, and an exclusion recorded against one target does not silence another.

#### Scenario: Webhook detects an externally-authored grant

- **WHEN** the webhook processes a `grant_added` event that survives the self-mutation guard, matches no `external_grant_exclusions` row, and matches no Syndra expected grant
- **THEN** the backend MUST insert a `drift_items` row (`detection_source='webhook'`, `drift_type='target_only'`, `target='zitadel'`, `status='pending_triage'`)
- **AND** a duplicate detection for the same `(target, user_id, project_id, drift_type, role_keys)` while still `pending_triage` MUST NOT create a second row

#### Scenario: Two targets drifting on one user do not suppress each other

- **WHEN** two registered targets each drift on the same user, project, and role
- **THEN** both MUST produce their own `drift_items` row
- **AND** marking one external MUST NOT suppress detection on the other

### Requirement: Buffered propagations MUST drain only on explicit operator action, and `applied` MUST be the terminal success state

Buffered **grants** MUST drain only on explicit operator action, on every target. Buffered **revocations** MAY additionally be drained by a background runner, because a delayed grant withholds access while a delayed revocation retains it, and the consent property this requirement protects concerns conferring access rather than withdrawing it. A background revocation drain MUST obey every other rule in this requirement — the same advisory lock, the same claim semantics, the same terminal-state discipline — and MUST NOT dispatch any row whose `op_type` confers access. The operator surface MUST state, for each submitted operation, whether it will drain on its own or wait for an operator, so that neither behaviour is inferred.

The operator-triggered drain MUST be triggered by the operator (`POST /api/v1/propagations/drain`), MUST pre-flight target reachability, and MUST treat a `2xx` Management API response as terminal confirmation (`status='applied'`). There MUST be no dependence on a webhook return-trip to confirm a Syndra-originated grant, because such events are dropped by the self-mutation guard.

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

#### Scenario: A background runner drains revocations but never grants

- **WHEN** the background revocation runner claims work
- **THEN** it MUST claim only rows whose operation withdraws access
- **AND** MUST leave access-conferring rows for the operator-triggered drain
- **AND** MUST acquire the same advisory lock, so it cannot run concurrently with an operator drain

#### Scenario: The apply surface states which rule applies

- **WHEN** an operator submits an operation that enters the outbox
- **THEN** the response and the surface MUST state whether it will drain automatically or wait for an explicit resume
- **AND** an operator MUST NOT have to know the operation's type to infer whether further action is required of them
