## ADDED Requirements

### Requirement: Operator diagnostic surface for Zitadel management
The admin UI MUST provide an operator-facing diagnostic page that exercises the live `/api/v1/zitadel/*` management surface end-to-end without requiring cmdline tooling. The page MUST cover: M2M health probe (key → JWT assertion → token exchange → Management API call), projects and project-role CRUD, users and grants CRUD, and a cross-project grant overview.

#### Scenario: Operator verifies M2M service account
- **WHEN** an admin opens the diagnostic page and clicks the health check
- **THEN** the UI MUST call the backend health endpoint and render the structured response
- **AND** a successful round-trip MUST surface the Zitadel domain, M2M token-exchange latency, and the total number of projects returned by the Management API
- **AND** a disabled or failed state MUST render the backend's structured diagnostic payload (status, stage, error) rather than a generic error

#### Scenario: Operator exercises role CRUD against live Zitadel
- **WHEN** an admin selects a project in the diagnostic UI and creates, renames, or deletes a project role
- **THEN** the UI MUST call the corresponding `/api/v1/zitadel/projects/{id}/roles[/key]` endpoint through the admin-gated proxy
- **AND** the UI MUST refetch the project's roles after every mutation so the displayed state reflects Zitadel's current state
- **AND** destructive operations MUST require explicit confirmation before dispatch

#### Scenario: Operator exercises grant CRUD against live Zitadel
- **WHEN** an admin selects a user and assigns, updates, or revokes a grant
- **THEN** the UI MUST call the corresponding `/api/v1/zitadel/users/{id}/grants[/gid]` endpoint
- **AND** the UI MUST refetch the user's grants after every mutation
- **AND** the revoke action MUST require explicit confirmation

#### Scenario: Operator inspects cross-project grants
- **WHEN** an admin requests the full grant overview
- **THEN** the UI MUST call `/api/v1/zitadel/grants` and render each grant's user id, project id, role keys, and grant id
- **AND** the response MUST expose the backend `total` so truncation beyond the page limit is visible
