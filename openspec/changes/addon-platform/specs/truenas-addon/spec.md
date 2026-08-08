## ADDED Requirements

### Requirement: The add-on MUST authenticate with a scoped, expiring API key and avoid privilege escalation paths

The add-on MUST connect over the JSON-RPC 2.0 WebSocket API using a user-linked API key with `expires_at` set, whose linked account holds only the roles its feature set requires. It MUST set and rotate passwords through `user.create` / `user.update`, which require `ACCOUNT_WRITE`, and MUST NOT use `user.set_password`, which requires `FULL_ADMIN` when the target is another user.

The add-on's own credential MUST NOT carry account deletion. Deletion MUST require an elevated credential supplied by the operator at the moment of purge, used for that call alone and never persisted, so that the one irreversible operation is not available to the add-on unaided.

#### Scenario: Password is set without full administrator rights

- **WHEN** the add-on sets or rotates a member's password
- **THEN** it MUST use `user.update` with the password field
- **AND** MUST NOT call `user.set_password`

#### Scenario: Deletion is impossible with the add-on's own credential

- **WHEN** the add-on attempts an account deletion using its own API key
- **THEN** the target MUST refuse it for want of privilege
- **AND** a purge MUST succeed only when carrying an operator-supplied elevated credential
- **AND** that credential MUST NOT be persisted, cached, or logged after the call

#### Scenario: Key expiry is visible before it breaks provisioning

- **WHEN** the add-on's API key approaches expiry
- **THEN** the add-on MUST report the expiry through its health surface
- **AND** the operator surface MUST show it before provisioning fails

### Requirement: Password material MUST be forwarded and never stored

TrueNAS accepts plaintext passwords only; no hash form can be written. The add-on MUST forward a member-supplied password to the target and MUST NOT persist it in any store, cache, snapshot, or log. Syndra MUST retain only credential existence and rotation metadata.

#### Scenario: Credential is not retained after forwarding

- **WHEN** the add-on applies a password set and the target accepts it
- **THEN** the plaintext MUST NOT appear in the add-on's idempotency store, snapshot, or mutation log
- **AND** the mutation log entry MUST record that a password was set, for whom, and when

#### Scenario: Existence is tracked for drift without the secret

- **WHEN** the backend reports on a member's infrastructure credential
- **THEN** it MUST derive the answer from existence and last-change metadata
- **AND** MUST NOT require the credential value to do so

### Requirement: A credential set MUST fail closed

A credential set cannot be queued, because queuing requires retaining the secret. It MUST also be rate-limited per subject, because it is a member-driven write terminating in a single rate-limited session shared with every other operation, and repeated resets would otherwise wedge the target for everyone. An unreachable target, a lifecycle-state refusal, or a subject with no account yet MUST therefore produce an immediate explicit failure rather than a queued row, and the member MUST be told plainly that nothing was recorded and to retry.

#### Scenario: An unreachable target fails the set outright

- **WHEN** a member submits a credential and the target is unreachable or refusing for lifecycle state
- **THEN** the backend MUST fail the operation immediately
- **AND** MUST NOT record it as queued or pending
- **AND** the member MUST be told that nothing was set and that they should retry

#### Scenario: Repeated resets are rate-limited

- **WHEN** a member submits credential sets faster than the configured per-subject rate
- **THEN** the backend MUST refuse the excess without calling the add-on
- **AND** the limit MUST be generous enough that ordinary use never reaches it

#### Scenario: A credential set before the account exists fails closed

- **WHEN** a member submits a credential before their target account has been created
- **THEN** the operation MUST fail explicitly
- **AND** MUST NOT be retained for later application

### Requirement: Credential hashes MUST NOT leave the add-on

`user.query` returns `unixhash` and `smbhash`. The NT hash is a usable authentication credential. The add-on MUST request only the fields it needs and MUST strip hash fields from every response it returns, every snapshot it stores, and every log it writes.

#### Scenario: Hash fields are absent from add-on responses

- **WHEN** the backend reads subject state from the add-on
- **THEN** no response body MUST contain `unixhash` or `smbhash`
- **AND** no stored snapshot or log entry MUST contain them

### Requirement: Revoking access MUST state its true effect

The target exposes no method to terminate an established SMB session. Cutting a subject's access is therefore composed of an entitlement change resolving the account and SMB fields to disabled, plus a credential rotation, and the operator surface MUST state that established sessions end on reconnect rather than immediately.

#### Scenario: Revocation applies both halves

- **WHEN** an operator revokes a subject's access
- **THEN** the resolved entitlement set MUST mark the account and SMB access disabled, converged through the apply path
- **AND** the credential MUST be rotated through the rotation operation
- **AND** the target MUST reflect `locked` set and `smb` cleared

#### Scenario: The effect is described honestly

- **WHEN** a revocation is presented to an operator
- **THEN** the surface MUST state that an established session persists until it reconnects
- **AND** MUST NOT present the action as immediate session termination

#### Scenario: Rotation does not retain the credential

- **WHEN** the credential is rotated as part of a revocation
- **THEN** the generated value MUST NOT be persisted, cached, or logged
- **AND** the record MUST show that a rotation occurred, its actor, and its time

### Requirement: Deprovisioning MUST be reversible and purge MUST be deliberate

Losing the last entitlement on the target MUST resolve the account and its SMB access to disabled while preserving the account and its data, and regaining an entitlement MUST resolve them back to enabled through the same apply path. Deletion MUST require an explicit operator action that first discloses what data the account holds.

#### Scenario: Last entitlement removal preserves the account

- **WHEN** a subject's final target-granting role is removed
- **THEN** the apply path MUST converge the account to disabled with SMB cleared
- **AND** MUST NOT delete the account or its home data

#### Scenario: Restoring the role restores the account

- **WHEN** the subject regains a target-granting role
- **THEN** the apply path MUST converge the account back to enabled with SMB restored per their entitlements
- **AND** MUST NOT create a second account

#### Scenario: Purge discloses retained data first

- **WHEN** an operator initiates a purge
- **THEN** the plan MUST state what data the account holds before the apply is possible
- **AND** account deletion MUST NOT imply deletion of that data unless separately requested

### Requirement: Account names MUST derive from the email localpart once and then be recorded

The target generates no username and requires one at creation. Localpart uniqueness is guaranteed by the identity provider only within a single Workspace domain; if a second domain is ever federated the collision suffix becomes routine rather than exceptional, and MUST remain correct in that case. The add-on MUST derive it from the subject's primary email localpart — lowercased, sub-addressing removed, characters outside `/^[a-zA-Z0-9_][a-zA-Z0-9_.-]*[$]?$/` replaced, truncated to 32 characters — and MUST resolve any collision with a deterministic suffix derived from the subject's stable identity, never a counter. The derived name MUST be reported back and recorded against the subject, and the recorded binding MUST be authoritative thereafter.

#### Scenario: Derivation is deterministic and valid

- **WHEN** the add-on derives an account name for a subject
- **THEN** the result MUST match the target's permitted pattern and length
- **AND** deriving again from the same email MUST produce the same name

#### Scenario: Collision resolves without reusing another subject's name

- **WHEN** two subjects derive to the same candidate name
- **THEN** the add-on MUST append a suffix derived from the subject's stable identity
- **AND** the resolved name MUST NOT equal any name already bound to another subject
- **AND** resolution MUST be reproducible from the subject's identity alone

#### Scenario: An unusable localpart falls back deterministically

- **WHEN** a subject's email localpart normalizes to nothing usable
- **THEN** the add-on MUST derive a name deterministically from the subject's stable identity
- **AND** MUST NOT assign a random or sequential name

#### Scenario: Truncation cannot consume the collision suffix

- **WHEN** a name requiring a collision suffix also exceeds the length limit
- **THEN** the suffix MUST be reserved before truncation
- **AND** the result MUST be within the limit and MUST still disambiguate

#### Scenario: A later email change does not rename the account

- **WHEN** a subject's primary email changes after their account exists
- **THEN** the add-on MUST NOT rename the target account
- **AND** the recorded binding MUST continue to resolve the subject to the existing account

### Requirement: Binding conflicts MUST be an operator decision, never an inference

Account creation happens as part of entitlement convergence and is query-then-create, so it can encounter an existing account holding the derived name. The add-on MUST NOT adopt an account that is not already bound to the subject, because that account may belong to someone else and adopting it would hand them the subject's entitlements. A collision MUST halt the operation and be reported for an operator decision. Reconciliation MUST likewise report an account whose name has changed out of band beneath a recorded binding, rather than treating the subject as missing.

#### Scenario: An unbound account with the derived name halts creation

- **WHEN** the derived name is already held by an account not bound to this subject
- **THEN** the add-on MUST NOT create, adopt, or modify any account
- **AND** the apply MUST fail for that subject rather than converging their entitlements onto a stranger's account
- **AND** MUST report a binding conflict identifying the existing account
- **AND** the operator MUST choose between adopting it and creating under a disambiguated name

#### Scenario: An out-of-band rename is reported, not recreated

- **WHEN** reconciliation finds the account behind a recorded binding under a different name
- **THEN** it MUST report the rename against the existing binding
- **AND** MUST NOT create a replacement account

### Requirement: The add-on MUST probe and gate on the target version

The target's API is versioned per release with methods gained and removed across majors. The add-on MUST read the target version at startup and before resuming after an outage, MUST report it through its health surface, and MUST refuse to operate against a major version it does not support.

#### Scenario: Unsupported target version halts writes

- **WHEN** the probed target major version is outside the supported range
- **THEN** the add-on MUST refuse mutating operations
- **AND** MUST report the detected version and the supported range through health

### Requirement: The add-on MUST hold one session and break the circuit on rate limiting

The target limits authentication attempts and imposes an extended lockout when exceeded. The add-on MUST maintain a single persistent session rather than authenticating per request, and MUST open a circuit breaker on rate-limit responses so a lockout cannot wedge the drain.

#### Scenario: Rate limiting opens the circuit instead of retrying

- **WHEN** the target returns a rate-limit response
- **THEN** the add-on MUST stop issuing further calls for the cooldown period
- **AND** MUST report the open circuit through health rather than failing each queued row individually

### Requirement: Activity reporting MUST disclose where auditing is disabled

SMB auditing is configured per share. When auditing is off for a share, activity queries return nothing for it. The add-on MUST report which shares have auditing disabled alongside any activity result.

#### Scenario: Empty activity distinguishes quiet from unaudited

- **WHEN** an operator requests activity and one or more shares have auditing disabled
- **THEN** the response MUST name those shares
- **AND** MUST NOT present an empty result as evidence of no activity
