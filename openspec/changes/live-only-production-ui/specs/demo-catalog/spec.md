> **Status:** Integrated | [< Index](../../../../INDEX.md)

## ADDED Requirements

### Requirement: Production UI MUST NOT serialize demo catalog entities

When the deployment is configured for live Zitadel (`ZITADEL_DOMAIN` is set), the web UI MUST NOT serialize demo identity data (user IDs, names, emails, titles, teams) or demo project/role identifiers into HTML, RSC payloads, JavaScript bundles, or form default values.

#### Scenario: Login page in OIDC mode hides demo identities
- **WHEN** `ZITADEL_DOMAIN` is set
- **AND** an unauthenticated user requests `/login`
- **THEN** the rendered HTML, the RSC payload, and the JS bundle MUST NOT contain any string from the `DEMO_USERS` catalog (e.g., "sam_student", "alice@makerspace.local", "Maya Chen")
- **AND** the demo identity card MUST NOT be present in the DOM

#### Scenario: Forms do not pre-fill demo identifiers in OIDC mode
- **WHEN** `ZITADEL_DOMAIN` is set
- **AND** a user opens `/bundles`, `/users`, or `/requests` before the live catalog has resolved
- **THEN** form select elements MUST be empty (no `"printing"`, `"member"`, `"laser"`, `"trainee"`, or `"ava_guest"` literal as a default value)
- **AND** the form submit button MUST be disabled until the catalog resolves and populates the defaults

#### Scenario: Stale demo session cookies do not resolve in OIDC mode
- **WHEN** `ZITADEL_DOMAIN` is set
- **AND** the request carries a session cookie of `type: "demo"` (e.g., left over from a prior local-dev session)
- **THEN** `getSession()` MUST return `null` so demo identity data never reaches the page render

### Requirement: Demo mode MUST remain fully functional for local development

The demo catalog and demo session flow MUST keep working unchanged when `ZITADEL_DOMAIN` is unset.

#### Scenario: Login page in demo mode shows demo identities
- **WHEN** `ZITADEL_DOMAIN` is unset
- **AND** an unauthenticated user requests `/login`
- **THEN** the demo identity card MUST render with the seeded users
- **AND** clicking a user's "Continue as" button MUST establish a demo session
