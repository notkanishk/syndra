> **Status:** Integrated (bulk ops deferred P5) | [< Index](../../../../INDEX.md) | [Feature Coverage](../feature-coverage.md)

## ADDED Requirements

### Requirement: Direct grants with expiry
The system MUST allow direct role grants to be assigned to a user with an optional expiration timestamp.

#### Scenario: Create expiring grant
- **WHEN** an admin assigns a direct role grant with an expiry date
- **THEN** the system stores the grant and includes it in effective access until it expires

### Requirement: Access requests and decisions
The system MUST allow users to create access requests (either directly for a Bundle/Role or via the Service Catalog) and admins to approve or reject them with a review note.

#### Scenario: Service-initiated request
- **WHEN** a user requests access to the "Printing Lab" service
- **THEN** the system MUST generate an access request for the associated `printing_staff` bundle.
- **AND** the admin MUST see the original service context in the approval queue (e.g., "User requested access to Printing Lab service").

#### Scenario: Resolve a pending request
- **WHEN** an admin approves a pending request
- **THEN** the request status becomes approved and the review note and reviewer are recorded

### Requirement: Governance summary
The system MUST provide a governance summary that includes pending requests, expiring grants, and cleanup hints.

#### Scenario: Review governance queue
- **WHEN** an admin opens the governance summary
- **THEN** the system shows the current pending requests and any grants that are approaching expiry

### Requirement: Bulk Operation Mode
The system MUST provide a high-efficiency mode for performing mass assignments or revocations across multiple users.

#### Scenario: Entering bulk mode
- **WHEN** an admin toggles "Bulk Mode" on the user list
- **THEN** checkboxes MUST appear for each user, and a floating contextual action bar MUST be displayed.

#### Scenario: Performing a bulk grant
- **GIVEN** three users are selected in Bulk Mode
- **WHEN** the admin chooses "Grant Bundle" and selects the "Contractor" bundle
- **THEN** Syndra MUST initiate the assignment process for all selected users simultaneously.
- **AND** a summary confirmation MUST be displayed before the operation is finalized.

### Requirement: Safe bulk operation execution
Bulk grants and revocations MUST provide preview, authorization enforcement, idempotency, and per-user outcome reporting.

#### Scenario: Reviewing a bulk grant before commit
- **GIVEN** multiple users are selected for a bulk grant
- **WHEN** the admin proceeds to confirmation
- **THEN** Syndra MUST display a preview of the affected users, target bundle or role, expected derived access changes, and any users that will be skipped or rejected.

#### Scenario: Partial failure handling in bulk mode
- **WHEN** a bulk operation encounters one or more failed users
- **THEN** Syndra MUST return per-user results
- **AND** the audit log MUST record the attempted bulk action and individual outcomes
- **AND** retrying the same bulk action MUST be safe and idempotent.

#### Scenario: Privileged bulk action authorization
- **WHEN** a bulk mutation is submitted
- **THEN** the backend MUST validate that the acting admin is authorized for the target action and scope
- **AND** frontend selection state alone MUST NOT be trusted as authorization proof.

### Requirement: Direct grants affect lineage
The system MUST include direct grants in the effective access computation and lineage views.

#### Scenario: Explain a user's access
- **WHEN** a user has both bundle-derived roles and direct grants
- **THEN** the access explanation separates the source roles from derived roles and includes the direct grant reason


### Requirement: Grant expiration enforcement
The system MUST automatically revoke expired direct grants rather than relying on query-time filtering or manual cleanup.

#### Scenario: Expired grant is revoked
- **WHEN** a direct grant's `expires_at` timestamp passes
- **THEN** a background scheduler MUST revoke the grant within a configurable enforcement window
- **AND** trigger cache invalidation and LLDAP provisioning intents for the affected user

#### Scenario: Expired grant does not confer access
- **WHEN** a grant has expired but has not yet been processed by the enforcement scheduler
- **THEN** the effective access computation MUST exclude the expired grant

#### Scenario: Concurrent renewal is not revoked
- **GIVEN** the scheduler has selected a grant as a candidate based on its `expires_at`
- **WHEN** the grant is renewed (via an in-place upsert that preserves the row ID) before the scheduler completes its delete step
- **THEN** the grant MUST NOT be deleted
- **AND** no provisioning intent, audit entry, cache invalidation, or Zitadel cascade MUST be produced for that grant

#### Scenario: Scheduler retries on transient delete failure
- **WHEN** the guarded delete step fails due to a transient database error
- **THEN** the candidate grants MUST remain in the database
- **AND** no downstream side effects MUST be produced
- **AND** the sweep MUST be retried on the next tick

#### Scenario: Zitadel cascade is best-effort
- **WHEN** the Zitadel derived-grant cascade fails for an expired grant
- **THEN** the local delete, audit log, and cache invalidation MUST still succeed
- **AND** the orphan MUST be logged for the future reconciler to clean up

> **Status:** Integrated. `backend/internal/services/expiry` runs a periodic sweep (interval + batch size configurable via `EXPIRY_SCHEDULER_INTERVAL` / `EXPIRY_SCHEDULER_BATCH_SIZE`; default `5m` / `500`) that emits LLDAP removal intents, hard-deletes expired rows, invalidates the user cache, writes a `direct_grant.revoked_by_expiry` audit entry, and best-effort cascades derived Zitadel grants. Zitadel failures are log-and-continue — orphaned derived grants inherit the Phase-5 deferred "Partial Failure Rollback" compromise.

### Requirement: Bulk operations deferred
> **Status:** The bulk operation requirements (Bulk Mode, Safe bulk execution) above are specified but deferred to Phase 5. No bulk grant/revoke endpoints or UI currently exist.

### Requirement: Current implementation scope is documented honestly
The system MUST keep the documented distinction between the current project/role-based request flow and the intended service-to-bundle abstraction explicit until the implementation is hardened and aligned.

#### Scenario: Service abstraction not yet fully integrated
- **WHEN** the service catalog flow still relies on direct project/role request payloads under the hood
- **THEN** the documentation MUST call that behavior partial rather than fully integrated
- **AND** the hardening and alignment work MUST remain visible as an immediate next step
