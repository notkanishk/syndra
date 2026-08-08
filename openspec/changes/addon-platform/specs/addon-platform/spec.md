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

#### Scenario: Re-applying an unchanged set is a no-op

- **WHEN** the backend applies an entitlement set identical to the target's current state
- **THEN** the add-on MUST report no change
- **AND** MUST NOT issue a mutating call to the target

#### Scenario: Partial revocation is expressed as a reduced set

- **WHEN** a subject's entitlement set is reduced from three groups to two
- **THEN** the add-on MUST converge the target to exactly the two remaining groups
- **AND** MUST NOT require a separate revoke operation to remove the third

### Requirement: Every Syndra-mediated target mutation MUST leave a trace before the call

The backend MUST NOT mutate an add-on target without first durably recording the outbox row and audit entry, in one transaction, with the row's `target` set to the add-on. The target call happens during the drain, after the record is committed. A target-side change with no such record MUST be detected as drift and surfaced for triage, exactly as an untraced Zitadel change is.

#### Scenario: Entitlement change enqueues before dispatch

- **WHEN** an operator changes a subject's entitlements on a registered target
- **THEN** the backend MUST write the outbox row (with `target` set) and the audit row in one transaction
- **AND** no call to the add-on MUST have been issued before that transaction commits

#### Scenario: Untraced target change surfaces as drift

- **WHEN** the drift sweep reads target state and finds an entitlement with no corresponding ledger or outbox record
- **THEN** the backend MUST classify it as drift for that target and surface it for operator triage
- **AND** MUST NOT silently adopt it as expected state

### Requirement: Secret-bearing operations MUST leave a pre-dispatch record without leaving the secret

An operation declaring `secret_params` MUST NOT be queued in the outbox, because outbox rows are durable, retried, and retained for audit. It MUST still leave a trace before the call: the backend MUST commit an operation record — operation id, target, actor, subject, operation name, non-terminal status — before dispatching, carrying no value for any declared secret parameter, and MUST write the terminal status after the response. The operation id MUST be sent to the add-on and used by it to deduplicate, so a re-submission cannot double-apply.

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

### Requirement: Target-affecting operations MUST apply against a durable backend-issued plan

Every entitlement application and every declared operation MUST be planned before it applies. The backend MUST issue the plan, persist it under an identifier with a bounded lifetime, and record for each affected subject both the intended outcome and a fingerprint of that subject's current target state. An apply MUST cite a plan identifier; the backend MUST NOT accept an apply that carries a plan supplied by the client instead of one it issued.

#### Scenario: Plan precedes apply and binds it

- **WHEN** an operator initiates any target-affecting operation
- **THEN** the backend MUST issue a plan describing the effect on each affected subject and persist it under an identifier
- **AND** the apply MUST cite that identifier
- **AND** the apply MUST act on the subjects recorded in the plan rather than recomputing the cohort

#### Scenario: A stale plan is rejected, not silently reapplied

- **WHEN** an apply cites a plan whose recorded fingerprint no longer matches live target state for one or more subjects
- **THEN** the backend MUST reject the apply without mutating any subject
- **AND** the error MUST identify the subjects whose state changed so the surface can re-plan and show what moved
- **AND** an unknown or expired plan identifier MUST be rejected the same way

#### Scenario: Planning mutates nothing

- **WHEN** an add-on computes a plan
- **THEN** it MUST NOT issue any mutating call to its target
- **AND** MUST return the same outcome shape the apply path returns, with a fingerprint per subject

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

### Requirement: Add-ons MUST refuse operations exceeding a configured blast radius

An add-on MUST compute the number of affected subjects before applying, and MUST refuse an operation whose effect exceeds its configured subject limit unless the request carries an explicit acknowledgement of that scope.

#### Scenario: Oversized effect is refused

- **WHEN** an operation would affect more subjects than the configured limit and carries no scope acknowledgement
- **THEN** the add-on MUST refuse the operation without mutating the target
- **AND** MUST return the computed subject count so the operator can acknowledge it deliberately

### Requirement: Subject absence MUST NOT be treated as deletion

An add-on MUST act on destructive transitions only from an explicit instruction. A subject missing from a state read or a feed MUST be recorded as an anomaly and MUST NOT cause removal, deactivation, or deletion.

#### Scenario: Missing subject is reported, not deleted

- **WHEN** a subject the add-on manages is absent from the backend's expected set with no explicit removal instruction
- **THEN** the add-on MUST report the discrepancy as drift
- **AND** MUST NOT delete, lock, or otherwise mutate that subject
