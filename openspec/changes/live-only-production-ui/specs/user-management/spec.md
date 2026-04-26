> **Status:** Integrated | [< Index](../../../../INDEX.md)

## ADDED Requirements

### Requirement: Empty user directory MUST render an explanatory state

The `/users` view MUST render an explanatory empty state when the directory returns zero users, instead of rendering a blank list area.

#### Scenario: Live directory returns zero users
- **WHEN** `GET /api/v1/users` returns `[]`
- **THEN** the `/users` page MUST render an `<EmptyState>` with title "No users found"
- **AND** the description MUST direct the admin to verify Zitadel sync (e.g., that `ZITADEL_DOMAIN` and `ZITADEL_MACHINE_KEY_PATH` are set)

#### Scenario: Search yields zero matches
- **WHEN** the user list is non-empty in general but the current search query returns no matches
- **THEN** the page MUST render an `<EmptyState>` with title "No users match that search" and a hint to clear the query

### Requirement: Grant authorship MUST come from the authenticated principal

The backend MUST derive the `granted_by` audit attribute from the authenticated subject (Zitadel JWT `sub`) in production, not from the request body. The UI MUST NOT send a `granted_by` literal.

#### Scenario: Direct grant in OIDC mode records the JWT subject
- **WHEN** an admin authenticated with a Zitadel-issued JWT submits `POST /api/v1/users/{id}/grants`
- **THEN** the audit log entry's actor MUST be the JWT `sub` claim
- **AND** any `granted_by` field in the request body MUST be ignored in favor of the authenticated subject

#### Scenario: Direct grant in demo mode falls back to the proxied session id
- **WHEN** the deployment is in demo mode (no `ZITADEL_DOMAIN`)
- **AND** the proxy injects `granted_by: <session.id>` for an admin demo session
- **THEN** the audit log entry's actor MUST be the demo session's user id (not "system" and not "alice.rivera")
