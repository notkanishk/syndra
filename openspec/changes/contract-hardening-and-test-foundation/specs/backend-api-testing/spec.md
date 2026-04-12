## ADDED Requirements

### Requirement: Backend-first test coverage for mission-critical flows
The system MUST maintain comprehensive backend unit and handler coverage for all authorization, request-validation, and claim-shaping flows before frontend polish work is treated as complete.

#### Scenario: Validation regression prevention
- **WHEN** a backend mutation endpoint changes its request parsing or validation behavior
- **THEN** unit or handler tests MUST detect any regression in required-field checks, enum validation, or state-transition enforcement

### Requirement: Mapping rule safety coverage
The system MUST maintain unit coverage for mapping-rule cycle detection and conflict behavior.

#### Scenario: Indirect cycle attempt
- **WHEN** a new mapping rule would create an indirect cycle through existing rules
- **THEN** automated tests MUST verify that the rule is rejected

### Requirement: Access workflow coverage
The system MUST maintain backend tests for direct grants, access requests, approvals, rejections, expiry handling, and resulting side effects.

#### Scenario: Approved request grants access
- **WHEN** a pending request is approved
- **THEN** automated tests MUST verify that the request status changes correctly
- **AND** the resulting direct grant, audit behavior, and cache rebuild side effects are exercised

### Requirement: Claim contract regression coverage
The system MUST maintain regression tests for application claim simulation and action-injection payload shaping.

#### Scenario: Legacy consumer format preserved
- **WHEN** a claim profile requires a non-default format such as CSV or space-delimited output
- **THEN** automated tests MUST verify the exact payload shape returned to the consumer

### Requirement: Claim-injection degraded-mode coverage
The system MUST maintain regression tests for cache miss, timeout, malformed cache payload, and configured safe fallback behavior in the Actions v2 claim path.

#### Scenario: Cache miss fallback exercised
- **WHEN** the claim injection path cannot find a compiled cache entry
- **THEN** automated tests MUST verify the documented fallback behavior for the affected application
- **AND** the tests MUST confirm that unsupported implicit fallback behavior is rejected

### Requirement: Zitadel Actions v2 compatibility coverage
The system MUST maintain regression coverage proving that all Zitadel-facing claim and event flows remain compatible with Actions v2 expectations.

#### Scenario: Action payload contract changes
- **WHEN** the action injection request or response contract changes
- **THEN** automated tests MUST verify continued compatibility with the expected Actions v2 flow and claim semantics

### Requirement: Internal contract hardening coverage
The system MUST validate and test purpose-built internal contracts separately from Zitadel-facing contracts.

#### Scenario: Provisioning intent contract evolves
- **WHEN** the Backend-to-Sync payload changes
- **THEN** automated tests MUST verify its authentication, validation, and failure behavior
- **AND** the tests MUST confirm that Zitadel-facing compatibility assumptions did not shift as a side effect

### Requirement: Service account control-plane coverage
The system MUST maintain tests and review gates for all backend flows that use the Zitadel service user account.

#### Scenario: Management API mutation path changes
- **WHEN** a backend feature changes how it grants, revokes, or reconciles Zitadel-managed state
- **THEN** automated tests MUST verify authorization boundaries, audit behavior, retry safety, and least-privilege assumptions for the service account path

### Requirement: Frontend critical-path contract tests
The system MUST add frontend tests after backend hardening for the member/admin proxy boundary and the highest-risk request flows.

#### Scenario: Member request scope enforced
- **WHEN** a member session fetches access requests through the UI proxy
- **THEN** automated tests MUST verify that only self-scoped records are returned
- **AND** unauthorized admin-only paths remain blocked

### Requirement: Bulk operation safety coverage
The system MUST maintain backend tests for bulk mutation preview, authorization, per-user outcome reporting, and idempotent retry behavior.

#### Scenario: Bulk mutation safety regression
- **WHEN** a bulk grant or revoke path changes
- **THEN** automated tests MUST verify preview accuracy, authorization enforcement, per-user outcome reporting, and safe retry semantics
