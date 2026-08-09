## ADDED Requirements

### Requirement: Every apply MUST cite a persisted plan, on every target including Zitadel

Plan-then-apply MUST be a backend guarantee rather than a convention resting on the client re-sending the same request. This is the canonical statement of the rule; other capabilities reference it rather than restating it.

The rehearsal request MUST persist its outcomes under a plan identifier with a bounded lifetime and return that identifier, recording for each subject the intended outcome and a fingerprint of the state that was reviewed. The apply request MUST cite that identifier rather than re-submitting the original request for recomputation, and the backend MUST re-verify every fingerprint before dispatching. This applies to `POST /api/v1/grants/bulk`, `POST /api/v1/requests/bulk-decision`, `POST /api/v1/governance/drift/bulk-attribute`, `POST /api/v1/governance/drift/bulk-mark-external`, and every add-on target operation.

The fingerprint MUST cover the object the operator reviewed, not only the subject's grants — for drift triage it MUST include the drift row's own status, so that a row resolved by someone else while the operator was reading the list fails verification instead of being re-resolved.

Where an apply still carries the original request body — because that body holds what the write needs and the per-subject outcome does not — the plan MUST bind it. The backend MUST record a digest of the request the rehearsal was computed for and MUST compare it as one more dimension of the citation, so a submitted body that does not match loses in the database and spends nothing. Bound is not the same as trusted: what the apply executes MUST still be the recorded outcome, and the body MUST NOT be a second source for the cohort or the diff. Only the fields that change the effect may be bound. An annotation an operator writes at apply time — a reason, a review note — changes nothing about who gets what, and binding it would make correcting a typo cost a re-plan, which teaches operators to click through the stale-plan dialog on the surface where that dialog matters most. What the plan stores of the request MUST be a digest and nothing else, for the same reason no plan column may hold free text: a body is where a submitted secret lives.

Verification MUST be all-or-nothing across the plan's subjects. A partial apply on a stale plan lands the subjects that did not move and leaves the operator to work out which, from a batch they approved as a unit, on the screen where a bulk mistake is hardest to unpick. A refused apply MUST leave the approval unspent, because re-plan-and-apply is the operator's only recovery and it must not then be refused as already-applied for something that never happened. A subject present on the approval and absent from the fresh read MUST count as moved rather than be skipped: the cohort is bound, so it cannot be a different selection — it is a subject that could no longer be evaluated, and unverifiable is not verified.

The value compared on each side MUST be computed by the same code. A fingerprint the rehearsal derives one way and the apply derives another verifies nothing, in the same way a token preview computed by different code from the token it previews is a preview of nothing.

The sole exemption is an operation scoped to `member` acting on the acting subject alone, which has no cohort and no diff to review and MUST be dispatched synchronously.

A plan MUST be spent by at most one apply, and the transition that spends it MUST be the apply's authority rather than a check the caller performs. A lifetime bounds how long a plan may be cited but not how many times it may be cited, and while a first apply's rows are still waiting in the outbox the target has not moved, so fingerprint verification would not catch the repeat.

A plan MUST be citable only by the operator who approved it, and only on the surface and target it was issued for. An approval is a person's judgement about a specific diff on a specific screen, and an identifier read out of a log or a screenshot is not that judgement.

A persisted plan row MUST record the decision and MUST NOT record its rendering. Every value it stores MUST be either a member of a closed vocabulary the backend owns or an identifier the backend allocated, and MUST be refused at the write if it is neither. A closed vocabulary MUST be closed at compile time rather than merely small — a mutable package-level collection is widenable by any caller before the check that reads it, which is not a vocabulary but a default. An identifier MUST be verified by lookup rather than by pattern: a syntactically valid identifier that names no row is not a reference, and a plan MUST NOT cite a row belonging to a subject other than the one whose plan row cites it, nor a desired-state snapshot taken for a different subject or a different target — a foreign key proves existence, and existence was never the property. Cited identifiers MUST be canonicalised before they are compared or stored, so that a citation differing only in case from the stored form is recognised as the same identifier rather than refused as a fabricated one. Free text is not sufficient protection against a declared secret reaching durable storage: a submitted secret is itself a string, so a struct of free-text fields is a route to the column however carefully its first writer avoids it, and no character class distinguishes a password from a role name. The sentences a rehearsal displays MUST be rendered from the recorded decision when a plan is read, not stored beside it.

Every per-subject row MUST record a fingerprint, and a plan MUST NOT be persisted with a row that has none. Verification compares a recorded fingerprint against a live read, so an absent recorded value would match an absent live one and the subject would pass verification precisely when the target could not be read.

An accepted apply MUST spend the plan and queue its work in one transaction, so that a failure partway through leaves neither. It MUST NOT queue work for a target whose registration is not active: that target was removed from the deployment, so nothing will drain those rows, and a row that never drains is counted as queued — which reads as recorded. That is distinct from a target that is merely unreachable, which is still deployed and whose work MUST queue.

Every bound value on a queued row — the subject, the target, and the operator who approved it — MUST be derived from the approval by the write itself rather than supplied alongside it. A reference that is only checked for existence proves the approval exists and nothing more: not that it is this subject's, not that its plan was ever claimed, and not that its target still takes work. Each of those MUST be a condition of the write, so a row that should not exist is never written rather than written and then argued about.

An approval MUST authorise at most one queued row, enforced by a uniqueness constraint and not only by a predicate, because a predicate cannot see what a concurrent transaction has not yet committed and two callers racing on one approval would each find no existing row. The loser of that race MUST receive the same refusal the non-concurrent case receives: a raised constraint violation aborts the transaction it was raised in, so the caller would get a database error about an index instead of the typed answer, and no diagnosis could run.

The target's registration MUST be read under a lock that serialises against the reconciliation which disables targets, and that lock MUST be taken by the write itself rather than by a caller ahead of it. An unlocked join is an MVCC read: work can begin while the target is active, a deregistration can disable it and sweep what it had queued, and the original snapshot can still commit a fresh row behind that sweep — undrainable, and invisible to the sweep that already ran. A lock held only by one caller is not an invariant, because the write is reachable without it. An unlocked read can return active, be overtaken by a committed disable, and let the apply commit the permanently undrainable row the check exists to refuse.

Serialising is necessary and not sufficient: an apply that wins that race still commits work against a target about to be deregistered. Deregistration MUST therefore resolve the work it strands, in the same transaction that changes the registration state — across two transactions an apply commits into the gap between them. Stranded rows MUST reach a terminal state that is neither `failed` (which claims an attempt was made) nor `superseded` (which claims a later decision won), MUST be terminated rather than deleted so the subject, the approval, and the reason survive, MUST leave an audit trace written by that same statement, and MUST distinguish a row that was never dispatched from one that was in flight and whose outcome is therefore unknowable. Deregistration MUST NOT be refused for having queued work: a deployment change would then fail because of a queue, and a backend that died mid-drain would leave a target that can never be removed. Abandoned work MUST NOT be counted as failed, and MUST NOT be counted as queued — nobody is waiting for it — so any count that reaches these rows by asking for "not applied" rather than for the state it means is wrong.

#### Scenario: An apply carries a different request than was reviewed

- **WHEN** an apply cites a valid, unspent approval but submits a body whose cohort, operation, or parameters differ from the one the rehearsal was computed for
- **THEN** it MUST be refused with a reason distinct from staleness, since the world did not move — the operator edited the form
- **AND** the approval MUST remain unspent
- **AND** nothing MUST be written
- **AND** a body differing only in an annotation written at apply time MUST be accepted

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

#### Scenario: An apply queues work and spends the plan together

- **WHEN** any part of an accepted apply fails after the plan has been claimed
- **THEN** no work MUST remain queued
- **AND** the plan MUST remain unspent, so the operator can apply the approval they still hold

#### Scenario: A target the deployment dropped takes no work

- **WHEN** an apply cites a plan for a target whose registration is not active
- **THEN** the backend MUST refuse it
- **AND** MUST NOT spend the plan, so the approval survives the target being re-registered
- **AND** the refusal MUST be distinguishable from a target that is deployed and unreachable
- **AND** a target disabled concurrently with an in-flight apply MUST NOT leave queued work behind

#### Scenario: A settle finds the row is no longer its own

- **WHEN** work is terminated while its dispatch is out, and the dispatcher then records the outcome
- **THEN** the recording MUST act only on work still in flight and MUST NOT overwrite the terminal state
- **AND** a retry path MUST NOT return terminated work to a queued state, which would recreate what the termination resolved and place it beyond the sweep that already ran
- **AND** the dispatcher MUST count it as neither a success nor a failure, because the call may have reached the target and nothing confirmed it

#### Scenario: Deregistering a target resolves what it strands

- **WHEN** a target is deregistered while it has queued or in-flight work
- **THEN** that work MUST reach a terminal state in the same transaction as the deregistration
- **AND** each terminated row MUST be preserved with its subject, its approval, and the reason
- **AND** a row that was already in flight MUST be distinguishable from one that was never dispatched, because only the first may have applied
- **AND** the deregistration MUST NOT be refused for having work queued

#### Scenario: Work cannot be queued under another subject's approval

- **WHEN** work is enqueued citing an approval
- **THEN** the subject, target, and approving operator recorded on it MUST come from that approval
- **AND** an approval whose plan was never claimed MUST queue nothing
- **AND** a second row under an approval that already has one MUST be refused, including under a concurrent second caller

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

Revocations MUST be dispatched ahead of grants for the same target, and MUST be eligible for background draining while grants remain operator-gated, because a delayed grant withholds access while a delayed revocation retains it. That priority MUST be applied to the subject holding the revocation, never to the individual row: ordering by a row's own operation type overtakes an OLDER grant for the same subject, so the grant lands afterwards and restores precisely the access being withdrawn — both rows applied, neither failed, and nothing on any surface disagreeing. Within one subject, intent order MUST be preserved absolutely, and intent order MUST be the final ordering key so that nothing after it can reorder two rows of one subject. For the same reason a background runner MUST NOT claim a revocation while an older access-conferring row for that subject is still unresolved: waiting is the safe direction, because the delay is visible on the unconfirmed-revocation surface while the inversion is not visible anywhere. A revocation exhausting its retry budget MUST escalate onto the unconfirmed-revocation surface as a finding rather than silently halting a background pass, since no operator is watching a background loop. The background runner MUST back off rather than spin when it cannot take the drain lock, and MUST pre-flight target reachability so an unreachable target costs a probe rather than a retry budget. When a target is unreachable and access must end immediately, the escalation surface MUST carry the out-of-band procedure for that target, since the backend has no path to it. A change an operator makes out of band to contain an incident MUST be recognised by the drift sweep as reconciling the outstanding revocation, not raised as fresh drift.

#### Scenario: Revocations are dispatched first

- **WHEN** the drain has both revocations and grants queued for one target
- **THEN** it MUST dispatch the revocations before the grants of any other subject
- **AND** a grant queued for the revoked subject BEFORE that revocation MUST still be dispatched first, because dispatching it afterwards would restore the access being withdrawn

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

Ending access is subject to the same rule as conferring it, and expiry is a path that ends it. A grant reaching its expiry MUST resolve to the same enqueue, in the same transaction as the ledger delete: an expired grant whose row is deleted with nothing queued leaves the access live on the target, unexplained by any Syndra record, and the next reconciliation correctly raises it as untraced drift — the sweep manufacturing findings out of the expiry's own inaction.

What expiry queues MUST be the effective-access delta, not an unconditional revocation of whatever the lapsed grant named. A role the subject still holds through a bundle, or still derives through another live grant, MUST NOT be revoked; a derived role the lapsed grant alone supported MUST be. This is the same delta an operator's removal computes, and it MUST be computed per grant and applied in sequence, because two grants lapsing in one pass can each support the same derived role — computed together against one snapshot, each sees the other still covering it and neither revokes.

The expiry re-check MUST live in the delete's own predicate rather than in the sweep that called it. Renewal pushes the date forward on the same row, so a grant read as expired can be alive by the time the write runs; a caller-side check decides from a snapshot, while the predicate decides from the row. A grant that no longer matches MUST leave the whole transaction — delete, audit, and queued revocations — unapplied, and MUST be distinguishable from a grant that is missing, since one means leave it alone and the other means something is wrong. That distinction MUST be established by looking, not assumed from the shape of the failure: a conditional delete matching no row proves only that some predicate failed, and the identity predicate and the expiry predicate mean opposite things. The lookup that establishes it MUST run on the same transaction and MUST be scoped to the same subject, or it answers about a different row and calls that a renewal. A lookup that itself fails MUST reach neither verdict, since reporting a renewal the backend did not observe says everything is fine.

An effective-access delta MUST be computed under the same lock that its write is committed under, and that lock MUST be taken before the reads rather than around the write. A delta is a statement about a world; the window that matters runs from the read that observed that world to the commit that acts on it. A change committing inside that window makes a queued revocation a statement about a world that no longer exists — and because the change queues its own grant first, the target settles without access the subject is currently owed. Locking only around the write serialises the writes and leaves every computation racing, which is the same failure with the appearance of a fix on top. Every path that computes an effective-access delta MUST hold that lock across its own reads AND its own source write, not merely across the enqueue. Taking it at the enqueue alone serialises commits and nothing else: a cascade can read while a grant still covers a role, conclude it has nothing to add, write its own source row, and only then queue behind the lock — by which time the expiry that removed the cover has committed a revocation, and the cascade commits an empty delta over it. The subject holds the source and not the access, and no queued row disagrees. The enqueue MUST take the lock as well, so that a path which has not been wrapped is still serialised against one that has, but that acquisition is a backstop and not the guarantee.

The lock MUST be a single lock rather than one per subject, because the subjects of a cascade are not always known before its reads: a mapping-rule change reaches every holder of the source role, and which holders those are is what the reads determine. A lock taken after that answer is a lock taken after the question it was meant to settle. It MUST be transaction-scoped, so a crashed holder cannot hold every access change in the deployment. It MUST NOT be held across a call to a target, in either direction: draining inside it would make one unreachable system serialise every access change behind it, and so would resolving a display name, since a directory lookup in live mode reaches the identity provider through a cache that can miss. A rehearsal that runs inside the lock MUST therefore compute state only; the names it renders MUST be filled in outside it. Presentation is not state.

A queued revocation MUST be visible to effective-access computation from the moment it is queued, not from the moment the target confirms it. A component that compares the ledger against the target directly, rather than through that computation, MUST make the same allowance in its write: a revocation dispatched and not yet settled is indistinguishable from a grant the target lost — the ledger carries it, the target does not have it — and replaying it queues an intent newer than the revocation, which preserves the ledger row and puts back access somebody asked to remove. The subordination MUST be a condition of the write rather than a check its caller performs, or a revocation queued between the read and the insert overtakes it anyway. A refused replay MUST be reported as refused, not as a failure and not as work done. The intent ledger deliberately keeps a grant until the target confirms — so a failed dispatch never leaves the backend believing access is gone while the target still has it — which makes the ledger, on its own, a wrong answer to what a subject effectively holds for as long as the row waits. A delta computed from that answer concludes nothing is missing and queues nothing, and the revocation lands anyway: the subject holds the source and not the access, with no queued row disagreeing. Locking the reconciliation does not reach this, because the reconciliation runs after the target has already applied the revocation. A `replace` MUST be read as its complement — its named roles are the ones that survive, and taken as removals they would subtract exactly the access being kept.

Reconciling a revocation or a replacement MUST NOT retract a role an unresolved later intent is establishing, and MUST apply that protection to both. What protects a ledger row is an intent that would ESTABLISH that row — matched by source, since cascade intent writes no ledger row at all and treating any queued addition as proof would keep a direct grant alive that nothing maintains, where it reads as coverage forever and the next removal queues nothing. The protecting intent MUST also be newer than the decision being reconciled; one queued before it is older intent and the reconciliation is the later word. Newer MUST be decided by an ordering value allocated while the serialising lock is held, not by a transaction-start timestamp. A row's creation time is fixed when its transaction begins and the lock is taken after that, so a transaction can start first, wait, and commit second while carrying the earlier time — which inverts the order for exactly the pair of writes the lock exists to order. The same ordering MUST govern dispatch, or a serially older intent is applied after the one that overtook it and the target settles on the wrong decision. Every value the reconciliation scopes by — the tuple, the source, and the moment — MUST be read from the row being reconciled rather than supplied alongside it, since a caller can assemble a set that never existed together and the ordering comparison means nothing unless the moment is that row's own. Once queued revocations are visible, a cascade that wants the role back queues an add behind the revoke, and that add's enqueue has already written the ledger row it needs.

Every write that changes an effective-access source MUST take the lock, not only the ones a cascade issues. A drain reconciling the intent ledger after a confirmed revocation deletes such a source, and a cascade holding the lock can read that row while it is still present, conclude the role it was about to add is already covered, and commit an empty delta while the deletion lands behind it — with nobody left who believes anything is missing. Assigning a bundle MUST go through the same path as any other assignment, including automated onboarding: a bare insert changes what a subject holds without the lock, and it projects nothing, so the access the assignment confers exists in the backend and nowhere else until some later cascade happens to recompute it.

The reads and the write MUST share one transaction, so that a write cannot be committed outside the lock its reads were taken under.

The outbox table is `propagation_outbox`. It was `pending_zitadel_propagations`, a name that becomes false the moment a second target exists; it carries a `target` column resolved against the `targets` registry, and its Zitadel-shaped columns (`project_id`, `role_keys`, `zitadel_grant_id`) are nullable for rows whose target is not Zitadel and MUST remain populated for rows whose target is. `direct_role_grants` gains no target column: direct grants are intents against Zitadel `user_grant`s, and add-on entitlements come from mappings and allowances instead.

#### Scenario: An expiring grant queues its revocation rather than becoming drift

- **WHEN** a direct grant reaches its expiry and the sweep processes it
- **THEN** the ledger delete, the audit row and the revocation MUST commit together
- **AND** a role the subject still holds another way MUST NOT be revoked, and MUST be reported as retained
- **AND** a role derived only from the lapsed grant MUST be revoked
- **AND** a grant renewed between the sweep's read and its write MUST survive, with nothing queued and nothing audited
- **AND** a grant that something else removed in that window MUST be reported as absent rather than as renewed
- **AND** a change to the subject's access committing between the delta's read and its commit MUST NOT be able to leave a revocation queued for a role they now hold
- **AND** a cascade that read before that change MUST NOT be able to commit an empty delta over a revocation that landed after its read
- **AND** the ledger reconciliation a drain performs after a confirmed revocation MUST take the same lock
- **AND** no call to a target or to the directory MUST happen while it is held
- **AND** a role with an unresolved revocation MUST NOT count as held by any delta computed while it waits

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

Every write that records a finding, and every read that answers a question about one, MUST name its target in the statement itself. The `target` columns MUST NOT carry a schema default beyond the migration that backfills them: a default is the schema answering "which target?" on behalf of a statement that forgot to say, and the answer it gives — `zitadel` — is the one that survives review, because it is what every row said before. With the default gone, an omission fails at the statement instead of producing a plausible wrong row. A detector that supplies no target MUST be refused before anything is opened.

Scope MUST live in the predicate, never in the caller's discipline. This applies symmetrically to reads: an exclusion lookup, a queued-work lookup, and a drift listing MUST each narrow by target, and a filter that narrows the query MUST also be visible to whatever chooses between a scoped and an unscoped response — a field that reaches one but not the other returns everything to a caller who asked for one thing. A pure filter given a set that spans targets MUST compare the target too, since an exported function is handed whatever its caller loaded, and cross-target suppression is a finding silenced by a decision nobody made about it.

A sweep MUST name the target it reconciled in its result, including when it halts: "nothing to report" about an unnamed target reads as a clean bill of health for all of them.

A triage resolution MUST refuse a finding on a target it cannot act on, and the refusal MUST live in the claim rather than at the call sites, because those are exported and an invariant a caller enforces is one the next caller can skip. Attribute and Revoke write Zitadel-shaped side effects — a ledger row keyed by the Zitadel project, a revoke bound to the Zitadel dispatcher — so applied to an add-on finding they would mutate one system while marking the other's finding resolved, and the finding would be gone. Mark external stays target-generic, because the exclusion it writes carries the target of the row it resolves. The refusal MUST be typed distinctly from a lost triage race and MUST NOT be reported as one: a race says try again, an unsupported target says this action has no reach into the system holding the access.

The triage listing MUST return one row shape whether or not a filter was applied. A response whose type depends on a query parameter is a contract the client cannot hold: the surface reads the enrichment fields off every row, an absent field is indistinguishable from a false one, and a filtered listing that omitted them silently withdrew the "role not in catalogue" warning from rows that had earned it. Per-person context on those rows MUST be counted over the whole pending queue rather than over the filtered subset, since "this person has two more items" is a fact about the person and not about the query — counted within a filter it shrinks to match whatever the operator was looking at, and reads as reassurance.

Enrichment that only one target's data can support MUST be gated on the target having it. Syndra's role catalogue describes Zitadel projects and roles; a permission on another target is not absent from that catalogue, it is not the kind of thing the catalogue lists. Reporting it as absent MUST NOT happen, because "role not in catalogue" is the queue's loudest signal and would attach to every add-on row on the strength of a lookup that could never have succeeded. The surface MUST be able to tell "not in the catalogue" from "no catalogue applies", rather than inferring the second from a false first.

#### Scenario: Two targets drifting on one user do not suppress each other

- **WHEN** two registered targets each drift on the same user, project, and role
- **THEN** both MUST produce their own `drift_items` row
- **AND** marking one external MUST NOT suppress detection on the other
- **AND** an exclusion recorded against one target MUST NOT satisfy the exclusion check for the other, even when the checking code is handed exclusions for both
- **AND** queued work on one target MUST NOT be counted as work that will reconcile the other

#### Scenario: A finding says what it looked at, or is refused

- **WHEN** a detector records a drift finding
- **THEN** the statement MUST name the target rather than relying on a column default
- **AND** a finding submitted with no target MUST be refused before any write or transaction is opened
- **AND** the drift listing MUST return the target on every row and MUST narrow by it when one is given

#### Scenario: A Zitadel-only resolution refuses an add-on finding

- **WHEN** Attribute or Revoke is invoked on a drift row whose target is not Zitadel
- **THEN** the transaction MUST refuse before writing any ledger, audit, or outbox row
- **AND** the drift row MUST remain `pending_triage`
- **AND** the refusal MUST be distinguishable from a lost triage race, and MUST NOT be presented as one
- **AND** Mark external MUST still resolve that row, writing an exclusion carrying the row's own target

#### Scenario: A filtered triage listing is the same shape as an unfiltered one

- **WHEN** the triage listing is requested with any filter
- **THEN** the rows returned MUST carry the same enrichment as an unfiltered request
- **AND** the per-person item count MUST be taken over the whole pending queue, not over the filtered subset

#### Scenario: An add-on finding is not judged against Zitadel's role catalogue

- **WHEN** the triage queue enriches a drift row whose target has no role catalogue
- **THEN** the row MUST report that no catalogue applies, rather than reporting the role as missing from one
- **AND** it MUST NOT be ranked above routine drift on the strength of that absence
- **AND** a row on a target that does have a catalogue MUST still be enriched and ranked by it

### Requirement: Buffered propagations MUST drain only on explicit operator action, and `applied` MUST be the terminal success state

Buffered **grants** MUST drain only on explicit operator action, on every target. Buffered **revocations** MAY additionally be drained by a background runner, because a delayed grant withholds access while a delayed revocation retains it, and the consent property this requirement protects concerns conferring access rather than withdrawing it. A background revocation drain MUST obey every other rule in this requirement — the same advisory lock, the same claim semantics, the same terminal-state discipline — and MUST NOT dispatch any row whose `op_type` confers access. The operator surface MUST state, for each submitted operation, whether it will drain on its own or wait for an operator, so that neither behaviour is inferred.

The operator-triggered drain MUST be triggered by the operator (`POST /api/v1/propagations/drain`), MUST pre-flight target reachability, and MUST treat a `2xx` Management API response as terminal confirmation (`status='applied'`). There MUST be no dependence on a webhook return-trip to confirm a Syndra-originated grant, because such events are dropped by the self-mutation guard.

The claim step MUST select both `pending` AND `in_flight` rows (the pending worklist and count report the same set), so a drain that crashed after claiming but before recording a terminal state leaves no orphaned `in_flight` row that is visible yet never re-driven. Because claiming `in_flight` rows would otherwise let a second drain re-dispatch a row the first drain is still processing, drains MUST be serialized by a session-level advisory lock: a drain that cannot acquire the lock MUST halt with reason `drain_in_progress` and MUST NOT claim or dispatch any row. Serialization guarantees the only `in_flight` rows a claiming drain ever sees are those orphaned by a crashed drain (whose session, and therefore lock, is gone). Within the drain, marking a row terminal (`applied`/`failed`) or requeuing it MUST be the sole way a row leaves `in_flight`, and the drain MUST NOT report a row as `applied`/`failed`/`requeued` unless that state was actually persisted, so a state-write failure never masquerades as success.

A claim MUST be scoped to the target its caller can dispatch, on every claim path including the ones that name specific rows. Claiming a row and then discovering it is undispatchable is not equivalent: releasing it costs something on every route — a requeue spends a retry and records a dispatch failure for a dispatch that never happened, so repeated targeted applies would exhaust an add-on row's budget before its dispatcher exists and its first genuine transient response would halt it. Not claiming it costs nothing. A drain MUST report the targets whose rows it declined, because a pass that silently dispatched nothing is indistinguishable from a pass with nothing to do, and that report MUST be diagnostic: failing to produce it MUST NOT fail the drain.

Exactly one transition out of `in_flight` originates outside the drain: deregistering a target MUST abandon its unresolved rows, as required by "Every apply MUST cite a persisted plan, on every target including Zitadel". That is the reason every drain finalizer MUST be conditional on the row still being `in_flight` rather than on its identifier alone. A drain whose dispatch was in the air when its row was abandoned MUST leave that row terminal, MUST NOT requeue it — which would return it to `pending` on a target that no longer exists — and MUST count it as neither applied nor failed. An `applied` `revoke` or `replace` MUST reconcile the intent ledger (`direct_role_grants`) so Syndra stops treating removed roles as expected grants.

#### Scenario: A drain never claims work it cannot dispatch

- **WHEN** any drain path claims rows — the batch pass, an inline apply, or a named set of rows
- **THEN** the claim MUST be scoped to the target that drain dispatches
- **AND** a row for another target MUST NOT be claimed, dispatched, released, or counted as an outcome
- **AND** no retry budget MUST be spent on a row that was never dispatched

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
- **THEN** it MUST claim only rows whose operation withdraws access, and that restriction MUST live in the claim rather than in the runner, so no caller of the claim can widen it
- **AND** MUST leave access-conferring rows for the operator-triggered drain, including a row that both confers and withdraws
- **AND** MUST NOT claim a revocation while an older access-conferring row for that subject is still unresolved
- **AND** MUST acquire the same advisory lock, so it cannot run concurrently with an operator drain

#### Scenario: The apply surface states which rule applies

- **WHEN** an operator submits an operation that enters the outbox
- **THEN** the response and the surface MUST state whether it will drain automatically or wait for an explicit resume
- **AND** an operator MUST NOT have to know the operation's type to infer whether further action is required of them
