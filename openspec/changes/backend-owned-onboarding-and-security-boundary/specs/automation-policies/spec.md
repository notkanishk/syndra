## MODIFIED Requirements

### Requirement: Backend-owned welcome-bundle assignment
The system MUST treat Welcome Bundle assignment as a backend-owned business mutation rather than as a direct Zitadel-hosted mutation flow.

#### Scenario: New user detected through compatible trigger intake
- **WHEN** Zitadel signals that a new user account was created
- **THEN** MkAuth MAY accept that signal through an Actions v2-compatible trigger path or a validated backend webhook
- **AND** the MkAuth Backend MUST perform the actual welcome-bundle assignment

#### Scenario: Retry-safe onboarding mutation
- **WHEN** MkAuth retries a previously attempted welcome-bundle assignment
- **THEN** the operation MUST be idempotent
- **AND** the system MUST avoid duplicate grants while preserving operator visibility into the retry outcome
