## ADDED Requirements

### Requirement: The add-on MUST authenticate with a scoped, expiring API key and avoid privilege escalation paths

The add-on MUST connect over the JSON-RPC 2.0 WebSocket API using a user-linked API key with `expires_at` set, whose linked account holds only the roles its feature set requires. It MUST set and rotate passwords through `user.create` / `user.update`, which require `ACCOUNT_WRITE`, and MUST NOT use `user.set_password`, which requires `FULL_ADMIN` when the target is another user.

#### Scenario: Password is set without full administrator rights

- **WHEN** the add-on sets or rotates a member's password
- **THEN** it MUST use `user.update` with the password field
- **AND** MUST NOT call `user.set_password`

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

### Requirement: Credential hashes MUST NOT leave the add-on

`user.query` returns `unixhash` and `smbhash`. The NT hash is a usable authentication credential. The add-on MUST request only the fields it needs and MUST strip hash fields from every response it returns, every snapshot it stores, and every log it writes.

#### Scenario: Hash fields are absent from add-on responses

- **WHEN** the backend reads subject state from the add-on
- **THEN** no response body MUST contain `unixhash` or `smbhash`
- **AND** no stored snapshot or log entry MUST contain them

### Requirement: Account lock MUST state its true effect

The target exposes no method to terminate an established SMB session. The lock operation MUST apply `locked` and clear the SMB flag and rotate the password, and the operator surface MUST state that established sessions end on reconnect rather than immediately.

#### Scenario: Lock is applied and described honestly

- **WHEN** an operator locks a subject's account
- **THEN** the add-on MUST set `locked`, clear `smb`, and rotate the password
- **AND** the operator surface MUST state that an established session persists until it reconnects
- **AND** MUST NOT present the action as immediate session termination

### Requirement: Deprovisioning MUST be reversible and purge MUST be deliberate

Losing the last entitlement on the target MUST lock the account and clear its SMB access while preserving the account and its data. Deletion MUST require an explicit operator action that first discloses what data the account holds.

#### Scenario: Last entitlement removal preserves the account

- **WHEN** a subject's final target-granting role is removed
- **THEN** the add-on MUST lock the account and clear SMB access
- **AND** MUST NOT delete the account or its home data
- **AND** the action MUST be reversible by restoring the role

#### Scenario: Purge discloses retained data first

- **WHEN** an operator initiates a purge
- **THEN** the plan MUST state what data the account holds before the apply is possible
- **AND** account deletion MUST NOT imply deletion of that data unless separately requested

### Requirement: Account names MUST derive from the email localpart once and then be recorded

The target generates no username and requires one at creation. The add-on MUST derive it from the subject's primary email localpart — lowercased, sub-addressing removed, characters outside `/^[a-zA-Z0-9_][a-zA-Z0-9_.-]*[$]?$/` replaced, truncated to 32 characters — and MUST resolve any collision with a deterministic suffix derived from the subject's stable identity, never a counter. The derived name MUST be reported back and recorded against the subject, and the recorded binding MUST be authoritative thereafter.

#### Scenario: Derivation is deterministic and valid

- **WHEN** the add-on derives an account name for a subject
- **THEN** the result MUST match the target's permitted pattern and length
- **AND** deriving again from the same email MUST produce the same name

#### Scenario: Collision resolves without reusing another subject's name

- **WHEN** two subjects derive to the same candidate name
- **THEN** the add-on MUST append a suffix derived from the subject's stable identity
- **AND** the resolved name MUST NOT equal any name already bound to another subject
- **AND** resolution MUST be reproducible from the subject's identity alone

#### Scenario: A later email change does not rename the account

- **WHEN** a subject's primary email changes after their account exists
- **THEN** the add-on MUST NOT rename the target account
- **AND** the recorded binding MUST continue to resolve the subject to the existing account

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
