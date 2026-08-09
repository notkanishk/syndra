## ADDED Requirements

### Requirement: Add-ons MUST run as isolated services reachable only by the backend

Each add-on MUST run as its own container on the internal network with no published host port, holding only its own target's credentials. The backend MUST be the only caller. An add-on MUST NOT hold credentials for a target it does not serve, and MUST NOT share a process with the backend's Zitadel service account.

#### Scenario: Add-on is unreachable from outside the internal network

- **WHEN** the deployment is inspected for exposed ports
- **THEN** no add-on service MUST publish a host port
- **AND** each add-on MUST authenticate the backend on every request and reject unauthenticated callers

#### Scenario: Credential isolation across targets

- **WHEN** two add-ons are registered
- **THEN** each MUST receive only its own target's credentials through its own environment
- **AND** neither MUST be able to read the other's credentials or the backend's Zitadel machine key

### Requirement: Add-ons MUST declare their capability through a manifest

An add-on MUST expose `GET /capabilities` returning an entitlement schema, an operation set, and the target product and version. The backend MUST render operator and member surfaces from this manifest rather than from add-on-specific code. Each operation MUST declare its `scope` (`member` or `admin`), whether it requires confirmation, and which of its parameters carry secrets.

#### Scenario: Backend renders surfaces from the manifest

- **WHEN** an add-on is registered and its manifest is fetched
- **THEN** the backend MUST expose only the operations the manifest declares
- **AND** an operation absent from the manifest MUST NOT be callable, even if the add-on implements it

#### Scenario: Secret parameters are never persisted or logged

- **WHEN** an operation declares a parameter in `secret_params` and is invoked
- **THEN** the backend MUST NOT write that parameter's value to any table, audit row, or log
- **AND** the audit record MUST state that the operation occurred, its actor, and its subject, with the secret value absent

### Requirement: Backend policy MUST bound what a manifest can offer

A manifest declares capability, not authorization. The backend MUST hold its own policy for each operation identifier — the scope it may be offered at, whether it requires confirmation, and its parameter schema — and the effective operation set MUST be the intersection of manifest and policy, with policy prevailing wherever they disagree. That parameter schema MUST be enforced on every invocation: a parameter the schema does not declare MUST be rejected rather than dropped, a declared required parameter MUST be present, and a value of the wrong type MUST be refused. A refusal MUST NOT contain any submitted value, since refusals are logged, returned, and captured in traces. An operation identifier with no backend policy MUST be unavailable regardless of what the manifest declares.

#### Scenario: A manifest cannot widen an operation's scope

- **WHEN** a manifest declares an operation at a broader scope than backend policy permits
- **THEN** the backend MUST offer it only at the scope policy permits
- **AND** MUST NOT render it to principals outside that scope

#### Scenario: A manifest cannot remove a confirmation requirement

- **WHEN** a manifest declares an operation as requiring no confirmation and backend policy requires one
- **THEN** the backend MUST require confirmation

#### Scenario: An undeclared parameter does not reach the target

- **WHEN** an invocation carries a parameter backend policy does not declare for that operation
- **THEN** the backend MUST refuse the invocation
- **AND** MUST NOT record it as an attempt or forward any part of it to the add-on

#### Scenario: An unknown operation fails closed

- **WHEN** a manifest declares an operation identifier absent from backend policy
- **THEN** the operation MUST be unavailable
- **AND** MUST NOT become available merely because a later add-on version declares it

#### Scenario: A manifest may narrow

- **WHEN** a manifest omits or disables an operation backend policy permits
- **THEN** the operation MUST be unavailable

### Requirement: Operations MUST carry individual availability

Target compatibility MUST be expressed per operation, not only per target version, because a supported target release may still lack a specific method. An operation the add-on cannot perform against its current target MUST be declared unavailable with a stated reason, and MUST be presented as disabled and explained rather than omitted or left to fail on use.

#### Scenario: An unsupported operation is disabled with a reason

- **WHEN** the target lacks the capability an operation depends on
- **THEN** the manifest MUST mark that operation unavailable with a reason
- **AND** the operator surface MUST show it disabled with that reason
- **AND** invoking it MUST be refused rather than attempted

### Requirement: Registration MUST be a data fact, not a schema constant

The set of valid targets MUST be represented as registry rows referenced by the tables that carry a target, so that registering a further add-on is a configuration and data operation rather than a schema migration. Rows naming an unregistered target MUST be refused at write time.

#### Scenario: Registering a target requires no schema change

- **WHEN** a new add-on is registered
- **THEN** its target MUST become valid for propagation, grant, and drift rows without altering the schema

#### Scenario: An unregistered target cannot be written

- **WHEN** a write names a target with no registry row
- **THEN** the database MUST reject it
- **AND** the drain MUST NOT dispatch work for a target that is not registered

### Requirement: Entitlement application MUST be level-triggered

An add-on MUST accept a resolved desired entitlement set for a subject and converge the target to exactly that set. Applying the same set twice MUST produce the same result as applying it once. Granting and revoking partial access MUST be expressed as the same call with a different desired set, not as separate add and remove operations.

#### Scenario: Applying to a subject with no account creates it

- **WHEN** an entitlement set is applied for a subject with no account on the target
- **THEN** the add-on MUST create the account as part of convergence and report the account name in its outcome
- **AND** the apply MUST NOT depend on a separately queued creation operation having run first
- **AND** a plan for such a subject MUST fingerprint them as absent

#### Scenario: Re-applying an unchanged set is a no-op

- **WHEN** the backend applies an entitlement set identical to the target's current state
- **THEN** the add-on MUST report no change
- **AND** MUST NOT issue a mutating call to the target

#### Scenario: Partial revocation is expressed as a reduced set

- **WHEN** a subject's entitlement set is reduced from three groups to two
- **THEN** the add-on MUST converge the target to exactly the two remaining groups
- **AND** MUST NOT require a separate revoke operation to remove the third

### Requirement: Account lifecycle state MUST be entitlement, not operation

Whether a subject's target account is enabled, and whether its service access is enabled, MUST be fields in the target's entitlement schema, computed by the resolver and overridable by allowance, not effects of one-shot operations and not bindable by a mapping. Deprovisioning MUST resolve them to disabled and restoration MUST resolve them to enabled, both converged by the ordinary apply path. A creation operation MUST NOT be the mechanism by which a disabled account is restored.

#### Scenario: Regaining a mapped role restores the account through convergence

- **WHEN** a subject who lost their last mapped role regains one
- **THEN** the resolved entitlement set MUST mark the account and its service access enabled
- **AND** the ordinary apply path MUST converge the target to that state
- **AND** restoration MUST NOT depend on the creation operation, which finds the account already present

#### Scenario: An operator suspension is not undone by a role grant

- **WHEN** a subject under an explicit operator suspension receives a mapped role
- **THEN** the suspension MUST continue to resolve the account as disabled until it lapses or is lifted
- **AND** the role grant MUST NOT re-enable the account

#### Scenario: A lifecycle disable does not outlive its cause

- **WHEN** a subject's account was disabled solely because they held no mapped role
- **THEN** regaining a mapped role MUST re-enable it without operator action

### Requirement: A member-scoped operation MUST act only on the authenticated actor

Scope MUST bind the subject as well as the audience. The backend MUST reject a `member`-scoped operation whose subject is anyone other than the authenticated actor, and MUST enforce this independently of the manifest and of the operation policy, because the manifest is untrusted and the policy describes operations rather than requests.

#### Scenario: A member cannot act on another subject

- **WHEN** a member invokes a `member`-scoped operation naming a subject other than themselves
- **THEN** the backend MUST reject the request
- **AND** MUST NOT call the add-on

#### Scenario: The binding does not depend on the manifest

- **WHEN** a manifest declares an operation `member`-scoped without any subject constraint
- **THEN** the backend MUST still enforce subject equal to actor
- **AND** the enforcement MUST NOT be defeasible by manifest or policy content

### Requirement: Drift MUST cover only bound subjects

A target holds accounts the backend never provisioned. The drift sweep MUST compute drift only over subjects with a recorded binding. An unbound target account MUST be reported as unmanaged inventory rather than entered into triage, and MUST become managed only through an explicit operator adoption.

#### Scenario: Pre-existing accounts do not flood triage

- **WHEN** the first sweep runs against a target holding accounts with no recorded binding
- **THEN** none of them MUST appear as drift
- **AND** they MUST be reported as unmanaged inventory

#### Scenario: Adoption is explicit and singular

- **WHEN** an unbound target account is to become managed
- **THEN** it MUST require an explicit operator decision
- **AND** the sweep MUST NOT bind it by inference
- **AND** adoption reached from a binding conflict during convergence MUST invoke the same action as adoption reached from the unmanaged inventory, leaving identical state

### Requirement: A version-rejected row MUST terminate as superseded, not failed

A row rejected because a newer version for its `(subject, target)` has already been applied MUST reach a terminal state distinct from failure. Revocation-first ordering makes this outcome ordinary, and reporting it as a failure would show operators a phantom failure for a row the system deliberately discarded.

#### Scenario: A grant overtaken by a later revoke is superseded

- **WHEN** a grant at an older version is dispatched after a revocation at a newer version for the same subject and target
- **THEN** the grant row MUST terminate as superseded
- **AND** MUST NOT be counted or presented as failed

### Requirement: Declared secrets MUST NOT reach the transport or diagnostic layers

The `secret_params` rules MUST apply to request logging, error responses, and captured stack traces, not only to durable stores. A declared secret parameter MUST NOT appear in a logged request body, an error payload echoing the request, or a panic capture, on any leg of the path including the member-to-backend request.

#### Scenario: Request logging does not capture the value

- **WHEN** request logging is enabled and an operation carrying a declared secret is invoked
- **THEN** the logged record MUST NOT contain the value

#### Scenario: Errors and panics do not echo the value

- **WHEN** an operation carrying a declared secret fails or panics
- **THEN** neither the error response nor any captured trace MUST contain the value

### Requirement: The manifest MUST declare a contract version checked at registration

An add-on MUST declare the version of the backend-to-add-on contract it implements, and the backend MUST refuse to register an add-on whose declared version it does not support. Version mismatch MUST fail at registration rather than surfacing later as an absent field.

#### Scenario: An unsupported contract version is refused at registration

- **WHEN** an add-on declares a contract version the backend does not support
- **THEN** registration MUST fail with that reason
- **AND** the add-on MUST NOT become callable

### Requirement: Lifecycle fields MUST NOT be mapping-bindable

The entitlement-schema fields governing whether an account and its service access are enabled MUST be computed by the resolver from whether the subject holds any mapped role for the target, and MUST be overridable only by an allowance. Structural validation MUST reject a role-to-target mapping naming one of them, because a mapping binding a role to a disabled account would contradict the derived lifecycle state on every resolution.

#### Scenario: A mapping onto a lifecycle field is rejected

- **WHEN** an operator submits a mapping whose field is a lifecycle field
- **THEN** the backend MUST reject it during structural validation
- **AND** MUST NOT call the add-on

#### Scenario: Lifecycle state remains derived and allowance-overridable

- **WHEN** a subject's mapped roles change
- **THEN** the lifecycle fields MUST be recomputed from whether any mapped role remains
- **AND** an allowance MUST remain able to override them

### Requirement: Desired state MUST be snapshotted, versioned, and applied in order per subject

Each entitlement change MUST record an immutable desired-state snapshot for its `(subject, target)` with a monotonically increasing version. The outbox row MUST reference the plan's per-subject row — which holds both that snapshot and the fingerprint of the reviewed state — rather than referencing the snapshot directly or instructing the drain to re-resolve, so that one durable object carries what was approved, against what state, and by whom. Plan expiry MUST NOT delete snapshots, which are audit records and outlive the plan that produced them. An operator-initiated change MUST apply its recorded snapshot, so that what was approved is what lands. Periodic reconciliation MUST instead resolve current state, so that convergence does not replay superseded snapshots. Application MUST be serialized per `(subject, target)`.

#### Scenario: An approved change applies what was approved

- **WHEN** policy changes between an operator's approval and the drain that dispatches it
- **THEN** the drain MUST apply the snapshot recorded at approval
- **AND** MUST NOT substitute a state resolved at drain time

#### Scenario: Reconciliation converges rather than replays

- **WHEN** the periodic reconcile runs for a subject with an older recorded snapshot
- **THEN** it MUST resolve current desired state
- **AND** MUST NOT reapply the superseded snapshot

#### Scenario: One object carries the approval

- **WHEN** the drain dispatches an outbox row
- **THEN** the desired state and the fingerprint it verifies MUST come from the same per-subject plan row
- **AND** an expired plan MUST NOT have removed the snapshot that row referenced

#### Scenario: A stale version cannot undo a newer one

- **WHEN** an apply carries a snapshot version older than the last version applied for that `(subject, target)`
- **THEN** the backend MUST reject it without dispatching
- **AND** a queued grant MUST NOT be able to land after a later revoke

#### Scenario: Concurrent applies for one subject do not interleave

- **WHEN** two changes for the same `(subject, target)` are dispatched concurrently
- **THEN** they MUST be serialized
- **AND** the resulting target state MUST equal the state of the higher version

### Requirement: A stale read MUST NOT be classified as drift

The drift sweep MUST consume only target reads the add-on reports as current. A read served from a last-known snapshot MUST NOT be diffed against desired state to produce drift findings, because every outage would otherwise manufacture findings for every change made during it. The backend MUST instead record the target as unreconciled for the period and say so.

#### Scenario: An outage produces no drift findings

- **WHEN** the drift sweep runs while a target is unreachable and only stale reads are available
- **THEN** it MUST NOT raise drift for that target
- **AND** MUST record the target as unreconciled, with the age of the last current read

#### Scenario: Reconciliation resumes on return

- **WHEN** the target becomes reachable and a current read is available
- **THEN** the sweep MUST diff against that read
- **AND** changes made during the outage MUST be classified on their own merits, not as a backlog of outage artefacts

### Requirement: Add-on transport MUST be mutually authenticated and bind the request

Calls between the backend and an add-on MUST use mutual TLS verified against the deployment's own private certificate authority, or signed requests carrying a timestamp and a hash of the body where mutual TLS is impractical. A bearer shared secret alone MUST NOT be sufficient, because it authenticates the caller without binding anything to the request. The private CA is not optional trimming of the mutual-TLS mode: without it the backend verifies the add-on against the public web PKI, under which the add-on's own certificate fails and any publicly issued certificate passes — a different and wrong trust anchor rather than a weaker version of the right one. A registration carrying incomplete mutual-TLS material MUST NOT be treated as mutually authenticated, and a registration carrying neither complete mode MUST NOT register at all. Every registered add-on's base URL MUST be HTTPS, and a target configured otherwise MUST NOT register: a client's transport settings are consulted only where a TLS handshake occurs, so a plaintext base URL means no certificate is presented and no authority is consulted while the registration still reports itself mutually authenticated. Signed-request mode MUST also run over TLS, because a request signature establishes neither the confidentiality of a secret-bearing body nor the authenticity of the response, and an unauthenticated response allows an on-path peer to forge a success the backend records as a completed mutation. The registered base URL MUST be the only authority the backend contacts for that target: an add-on's response MUST NOT redirect it. A followed redirect re-sends the body of a mutating call to a host the add-on chose, carrying the request signature that authenticates it there, and the redirect's own success would then be recorded against a target that never acted.

Every leg of the add-on contract MUST travel over that same authenticated transport, including the capability read. A manifest is what backend policy is intersected against to decide what is callable, so a manifest read over an unauthenticated channel is a capability set anyone on the path can edit — able to withdraw a working operation or offer one that must not be offered. Replay protection for mutations MUST rest on the operation identifier and the plan fingerprints, and a separate nonce store MUST NOT be introduced to duplicate that guarantee. Deduplication by operation identifier MUST therefore cover every mutating call rather than a subset, since that universality is what makes the nonce store unnecessary. The retention of that record is the replay window, and MUST exceed any plausible retry or outage window; beyond it, replay is bounded by the request timestamp in signed-request mode and by the level-triggered nature of entitlement application.

#### Scenario: An unauthenticated or unbound call is refused

- **WHEN** a call arrives without a valid client certificate, or with a signature that does not match its body and timestamp
- **THEN** the add-on MUST refuse it
- **AND** MUST NOT mutate its target

#### Scenario: A replayed mutation cannot double-apply

- **WHEN** a previously accepted mutating call is replayed verbatim
- **THEN** the add-on MUST recognise the operation identifier and MUST NOT apply it again
- **AND** where the call also carries fingerprints, verification MUST fail if target state has since moved

### Requirement: A dispatch outcome MUST distinguish what the target may have done

The backend MUST classify every dispatched call by what the ADD-ON may have done, not by what is convenient to report, and MUST distinguish four outcomes rather than two: applied, refused-without-acting, never-arrived, and sent-but-unanswered. Where the evidence is ambiguous the pessimistic reading MUST win. Only never-arrived MUST be treated as safe to dispatch again; sent-but-unanswered MUST NOT be automatically retried and MUST NOT be counted as either succeeded or failed, because a retry may duplicate a mutation the target already performed and a count either way asserts something the backend does not know.

Transport-level failure MUST NOT be inferred from a deterministic refusal. A refusal the add-on issued after validating the call is evidence the add-on is healthy, and MUST NOT contribute to any health signal that withholds traffic from the target — otherwise one malformed request repeated by one operator takes the target offline for everyone.

#### Scenario: A redirect is refused rather than followed

- **WHEN** an add-on answers any call with a redirect
- **THEN** the backend MUST NOT issue a request to the redirect target
- **AND** MUST NOT record the call as succeeded

#### Scenario: A response too large to read whole is not a success

- **WHEN** an add-on returns a success status with a body exceeding the backend's read bound
- **THEN** the backend MUST NOT record it as succeeded, because it did not read what the add-on said it did

#### Scenario: A timeout is not a failure

- **WHEN** a dispatched call times out or its connection is lost after the request could have been delivered
- **THEN** the backend MUST record it as unresolved rather than as succeeded or failed
- **AND** MUST NOT dispatch it again automatically

#### Scenario: An unreachable target leaves the intent intact

- **WHEN** a call cannot reach the add-on at all
- **THEN** the backend MUST record that nothing happened on the target
- **AND** the row MUST remain queued and eligible for dispatch

#### Scenario: A refusal does not withhold traffic from a healthy target

- **WHEN** an add-on repeatedly refuses a specific malformed request while otherwise serving
- **THEN** the backend MUST continue dispatching other calls to that target

### Requirement: Transport credentials MUST be rotatable and their expiry surfaced

Transport material MUST be reloadable without restarting the backend, since restarting the component that governs every other target to rotate one target's certificate makes rotation an outage. Material replaced on disk MUST be picked up by subsequent calls, and a call already in flight MUST complete on the material it started with. A reload that fails MUST leave the last working material in service rather than failing every call in the window, matching the rule that a refused manifest refresh keeps the last accepted one.

Expiry MUST be surfaced before it fails, and MUST reflect the soonest expiry in the chain rather than the client certificate alone: a current certificate presented against an expired authority fails exactly as hard as an expired one.

#### Scenario: Rotation mid-operation

- **WHEN** transport material is replaced while an operation is in flight
- **THEN** the in-flight operation MUST complete on its original material
- **AND** the next call MUST use the replacement

#### Scenario: Expiry is visible before it bites

- **WHEN** any certificate in an add-on's transport chain is within the warning window
- **THEN** the operator surface MUST report that target as expiring with the date and remaining days

### Requirement: The mutation log MUST be durable and tamper-evident

Each add-on MUST write every mutation it performs to an append-only local log with file permissions restricting it to its own service account, flushed to disk before the operation is reported complete, rotated by size with a bounded retention that exists only to prevent unbounded growth. Each record MUST carry the digest of the preceding record, and MUST be redacted by the same rules as any other record. Because a digest chain cannot detect truncation of its own tail, the add-on MUST additionally publish its log head digest and record count for the backend to persist outside the add-on's writable storage.

#### Scenario: A mutation is durably logged before it is reported

- **WHEN** the add-on completes a mutating call
- **THEN** the corresponding record MUST be flushed to disk before the operation is reported complete

#### Scenario: Alteration and interior removal are detectable

- **WHEN** a record in the log is altered, or removed from within the retained chain
- **THEN** verification of the digest chain MUST fail at that point

#### Scenario: The log head is anchored outside the add-on

- **WHEN** the add-on reports health
- **THEN** it MUST include its current log head digest and record count
- **AND** the backend MUST persist each observation

#### Scenario: Tail truncation is detectable by the anchor

- **WHEN** the reported record count has decreased, or the reported head does not extend the last head the backend recorded
- **THEN** the backend MUST report the log as truncated
- **AND** MUST NOT treat a locally valid chain as evidence the log is intact

#### Scenario: The log carries no secrets

- **WHEN** a mutation carried a declared secret parameter
- **THEN** its log record MUST record the operation, actor, subject, and time
- **AND** MUST NOT contain the parameter value

### Requirement: Every Syndra-mediated target mutation MUST leave a trace before the call

The backend MUST NOT mutate an add-on target without first durably recording the outbox row and audit entry, in one transaction, with the row's `target` set to the add-on. Exactly one exception exists: an operation declaring `secret_params` cannot be queued and MUST instead leave its pre-dispatch trace in the operation record described below. No other operation MUST be exempt. The target call happens during the drain, after the record is committed. A target-side change with no such record MUST be detected as drift and surfaced for triage, exactly as an untraced Zitadel change is.

#### Scenario: Entitlement change enqueues before dispatch

- **WHEN** an operator changes a subject's entitlements on a registered target
- **THEN** the backend MUST write the outbox row (with `target` set) and the audit row in one transaction
- **AND** no call to the add-on MUST have been issued before that transaction commits

#### Scenario: Untraced target change surfaces as drift

- **WHEN** the drift sweep reads target state and finds an entitlement with no corresponding ledger or outbox record
- **THEN** the backend MUST classify it as drift for that target and surface it for operator triage
- **AND** MUST NOT silently adopt it as expected state

### Requirement: Secret-bearing operations MUST leave a pre-dispatch record without leaving the secret

An operation declaring `secret_params` MUST NOT be queued in the outbox, because outbox rows are durable, retried, and retained for audit. It MUST still leave a trace before the call: the backend MUST commit an operation record — operation id, target, actor, subject, operation name, non-terminal status — before dispatching, carrying no value for any declared secret parameter, and MUST write the terminal status after the response. The record table MUST NOT contain any column able to hold a parameter value, including free-text or JSON columns intended for diagnostics: a column shaped to hold arbitrary content is where an add-on's echoed error payload — and with it a submitted secret — comes to rest, and a convention not to write one there is the part that fails. The record's status MUST be a closed vocabulary, and a terminal status MUST be writable only over a non-terminal one, so that a duplicated settle cannot resolve an unresolved operation on no evidence. The operation id MUST be sent to the add-on and used by it to deduplicate, so a re-submission cannot double-apply. The transport MUST require evidence that the record exists and describes this exact call — its target, operation, and subject — rather than accepting an identifier as given: an identifier a caller can supply unverified makes the pre-dispatch record a convention that holds only where callers observe it, and a record that can be pointed at the wrong call produces an audit trail describing something that did not happen, which is worse than none because it will be believed. A record that has already settled MUST NOT authorise a further dispatch.

#### Scenario: Record is committed before the add-on is called

- **WHEN** a member submits a new infrastructure credential
- **THEN** the backend MUST commit the operation record with a non-terminal status before issuing the call
- **AND** the record MUST contain no credential value
- **AND** the terminal status MUST be written only after the add-on responds

#### Scenario: A crash mid-dispatch leaves an honest unresolved state

- **WHEN** the backend fails between dispatch and the terminal write
- **THEN** the operation record MUST remain in its non-terminal status
- **AND** the operator surface MUST present it as unresolved rather than as either succeeded or failed
- **AND** the backend MUST NOT automatically retry it, because the secret is not retained

#### Scenario: Re-submission cannot double-apply

- **WHEN** an operation carrying an operation id already applied is dispatched again
- **THEN** the add-on MUST recognise the operation id and MUST NOT apply it a second time
- **AND** MUST return the original outcome

### Requirement: Add-ons MUST support planning and honour plan fingerprints

The governing rule for plan-then-apply is stated in the `access-governance` capability and is not restated here. Under it, an add-on MUST compute a plan without mutating its target, MUST return outcomes in the shared plan shape with a state fingerprint per subject, and MUST refuse a mutating call whose supplied fingerprints no longer match live target state.

An add-on MAY cap the number of subjects a single request may affect, as defence in depth. It MUST NOT be the sole enforcement point for cohort size, because a per-subject call cannot observe a cohort.

#### Scenario: Planning mutates nothing

- **WHEN** an add-on computes a plan
- **THEN** it MUST NOT issue any mutating call to its target
- **AND** MUST return the same outcome shape the apply path returns, with a fingerprint per subject

#### Scenario: A moved subject fails verification at the add-on

- **WHEN** a mutating call arrives whose supplied fingerprint no longer matches live target state for a subject
- **THEN** the add-on MUST refuse the call without mutating that subject
- **AND** MUST identify the subject whose state moved

#### Scenario: The plan store holds no secrets

- **WHEN** a plan is persisted for an operation declaring secret parameters
- **THEN** the plan MUST record the intent and the subject without any declared secret value
- **AND** the value MUST travel on the apply request only and MUST NOT be retained after it

### Requirement: A change approved while a target is unreachable MUST produce a provisional plan

A live state fingerprint cannot be obtained from an unreachable target, so planning MUST NOT be a precondition for recording an entitlement decision. A change approved while the target is unreachable MUST retain its approved desired-state snapshot and MUST produce a provisional plan computed against the add-on's last-known state, labelled with the age of that state. Before any dispatch, the plan MUST be re-fingerprinted against live target state. Fail-open MUST apply to the entitlement decision only, never to dispatching an unreviewed change.

#### Scenario: An outage does not block the decision

- **WHEN** an operator changes entitlements while the target is unreachable
- **THEN** the backend MUST record the change and its desired-state snapshot
- **AND** MUST issue a provisional plan against last-known state, labelled with that state's age
- **AND** MUST NOT refuse the change for want of a live fingerprint

#### Scenario: Nothing moved, so the plan dispatches

- **WHEN** the target becomes reachable and live fingerprints match those recorded provisionally
- **THEN** the backend MUST dispatch the change without requiring fresh approval

#### Scenario: The target moved, so the plan needs re-approval

- **WHEN** the target becomes reachable and any live fingerprint differs from the provisional one
- **THEN** the backend MUST withhold dispatch
- **AND** MUST require fresh approval, presenting what changed since the original approval

#### Scenario: A provisional plan does not expire on the ordinary plan lifetime

- **WHEN** a target remains unreachable for longer than the ordinary plan lifetime
- **THEN** the provisional plan MUST remain valid and its approved snapshot MUST be retained
- **AND** the backend MUST NOT discard the operator's approved change because the target was away
- **AND** re-fingerprinting on return, not elapsed time, MUST be what gates its dispatch

#### Scenario: Provisional is visibly distinct from applied

- **WHEN** a change is held under a provisional plan
- **THEN** the operator surface MUST present it as recorded and awaiting the target
- **AND** MUST NOT count it as applied

### Requirement: Unresolved operations MUST be counted apart from both outcomes

An operation whose result the backend does not know MUST be presented and counted as unresolved, never folded into either succeeded or failed. Counting it as succeeded asserts something nobody knows; counting it as failed tells a member to try again against a target that may already hold their new credential. A record awaiting an answer MUST NOT be surfaced as unresolved until it can no longer be in flight, since a dispatch in progress and a dispatch whose backend died are indistinguishable by status alone.

#### Scenario: An unresolved operation is in neither total

- **WHEN** an operation record is non-terminal or records a lost answer
- **THEN** the summary MUST report it as unresolved
- **AND** MUST exclude it from both the succeeded and the failed counts

#### Scenario: An in-flight dispatch is not yet an open question

- **WHEN** an operation was dispatched moments ago and has not yet answered
- **THEN** it MUST NOT appear on the unresolved surface until it has outlived the dispatch timeout

### Requirement: An unreachable add-on MUST fail open with queued accounting

An unreachable or failing add-on MUST NOT block the entitlement decision. The backend MUST record the change and leave the propagation queued for the drain. Queued rows MUST be counted apart from succeeded rows so a summary cannot report unconfirmed work as success.

#### Scenario: Grant succeeds while the target is unreachable

- **WHEN** an entitlement change is applied and the add-on is unreachable
- **THEN** the backend MUST record the change and the outbox row
- **AND** MUST report the row as queued, not succeeded
- **AND** the operator surface MUST show the subject's access as pending on that target

#### Scenario: Drain resumes when the add-on returns

- **WHEN** a previously unreachable add-on becomes reachable and the drain runs
- **THEN** the queued rows for that target MUST be dispatched and driven to a terminal state
- **AND** rows for other targets MUST be unaffected by the earlier outage

### Requirement: Add-ons MUST cap the subjects a single request may affect

An add-on MUST refuse a request affecting more subjects than its configured per-request limit unless the request carries an explicit acknowledgement of that scope. This is a backstop against a backend asking for too much at once; the authoritative cohort guard is specified in `access-governance` and enforced at plan time, where the cohort is known.

#### Scenario: An oversized request is refused at the add-on

- **WHEN** a single request would affect more subjects than the add-on's configured limit and carries no scope acknowledgement
- **THEN** the add-on MUST refuse it without mutating the target
- **AND** MUST return the count it computed

### Requirement: Subject absence MUST NOT be treated as deletion

An add-on MUST act on destructive transitions only from an explicit instruction. A subject missing from a state read or a feed MUST be recorded as an anomaly and MUST NOT cause removal, deactivation, or deletion.

#### Scenario: Missing subject is reported, not deleted

- **WHEN** a subject the add-on manages is absent from the backend's expected set with no explicit removal instruction
- **THEN** the add-on MUST report the discrepancy as drift
- **AND** MUST NOT delete, lock, or otherwise mutate that subject
