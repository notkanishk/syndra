## ADDED Requirements

### Requirement: Direct grants with expiry
The system MUST allow direct role grants to be assigned to a user with an optional expiration timestamp.

#### Scenario: Create expiring grant
- **WHEN** an admin assigns a direct role grant with an expiry date
- **THEN** the system stores the grant and includes it in effective access until it expires

### Requirement: Access requests and decisions
The system MUST allow users to create access requests and admins to approve or reject them with a review note.

#### Scenario: Resolve a pending request
- **WHEN** an admin approves a pending request
- **THEN** the request status becomes approved and the review note and reviewer are recorded

### Requirement: Governance summary
The system MUST provide a governance summary that includes pending requests, expiring grants, and cleanup hints.

#### Scenario: Review governance queue
- **WHEN** an admin opens the governance summary
- **THEN** the system shows the current pending requests and any grants that are approaching expiry

### Requirement: Direct grants affect lineage
The system MUST include direct grants in the effective access computation and lineage views.

#### Scenario: Explain a user's access
- **WHEN** a user has both bundle-derived roles and direct grants
- **THEN** the access explanation separates the source roles from derived roles and includes the direct grant reason
