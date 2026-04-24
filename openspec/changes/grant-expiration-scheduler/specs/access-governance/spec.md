## MODIFIED Requirements

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
