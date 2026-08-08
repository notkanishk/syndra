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

### Requirement: Secret-bearing operations MUST NOT enter the outbox

An operation declaring `secret_params` MUST be dispatched synchronously and MUST NOT be queued in the outbox, because outbox rows are durable and retained for audit. Such an operation MUST be recorded in the audit trail as an event, with the secret absent.

#### Scenario: Password set is dispatched without a durable payload

- **WHEN** a member submits a new infrastructure credential
- **THEN** the backend MUST call the add-on synchronously
- **AND** MUST NOT create an outbox row containing the credential
- **AND** the audit entry MUST record the actor, subject, and timestamp with no credential value

### Requirement: Target-affecting operations MUST support dry-run

Every entitlement application and every declared operation MUST accept a dry-run request and return the same outcome shape as the apply path, describing the effect per subject before anything is written. The operator surface MUST present that plan before an apply is possible.

#### Scenario: Plan precedes apply

- **WHEN** an operator initiates any target-affecting operation
- **THEN** the backend MUST first obtain and present a plan describing the effect on each affected subject
- **AND** the apply MUST act on the identified rows from that plan rather than recomputing the effect

#### Scenario: Dry-run mutates nothing

- **WHEN** an add-on receives a request with dry-run set
- **THEN** it MUST NOT issue any mutating call to its target
- **AND** MUST return the same outcome shape the apply path would return

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
