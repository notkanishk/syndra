> **Status:** Integrated | [< Index](../../../../INDEX.md)

## ADDED Requirements

### Requirement: Token Simulator MUST handle zero applications gracefully

The `/applications` view MUST render an explanatory empty state when no applications are registered, instead of an empty grid or a Token Simulator without selectable inputs.

#### Scenario: No applications registered
- **WHEN** `GET /api/v1/applications` returns `[]`
- **THEN** the `/applications` page MUST render an `<EmptyState>` directing the admin to register an application in Zitadel
- **AND** the Token Simulator MUST NOT render alongside an empty selector

### Requirement: Access-request authorship MUST come from the authenticated principal

The backend MUST derive the `reviewer_id` audit attribute (and the `granted_by` of the resulting direct grant) from the authenticated subject in production, not from the request body. The UI MUST NOT send a `reviewer_id` literal.

#### Scenario: Approving a request in OIDC mode records the JWT subject
- **WHEN** an admin authenticated with a Zitadel-issued JWT submits `POST /api/v1/requests/{id}/decision` with `status: "approved"`
- **THEN** the resolved request's `reviewer_id` MUST be the JWT `sub` claim
- **AND** the resulting direct grant's `granted_by` MUST be the JWT `sub` claim
- **AND** the audit log actor MUST be the JWT `sub` claim
- **AND** any `reviewer_id` field in the request body MUST be ignored in favor of the authenticated subject

#### Scenario: Approval requires an authenticated reviewer
- **WHEN** a decision request reaches the backend with no resolvable actor (no JWT subject in context AND no body fallback)
- **THEN** the backend MUST reject the approval with a 400 validation error explaining that an authenticated reviewer is required
