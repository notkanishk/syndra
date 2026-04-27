> **Status:** Proposed | [< Index](../../../../INDEX.md)

## ADDED Requirements

### Requirement: Operators MUST have a global grants viewer at /grants

The dashboard MUST expose a `/grants` admin route surfacing the union of MkAuth-direct grants, derived grants, and Zitadel grants (via `GET /api/v1/zitadel/grants`). Each row MUST resolve user/project/role names; the grant source (mkauth/zitadel/derived) MUST be visible.

#### Scenario: All grants table renders resolved names
- **WHEN** an admin opens `/grants` "All grants" tab
- **THEN** each row MUST display user via `<UserName/>`, project via `<ProjectName/>`, role via `<RoleName/>`
- **AND** each row MUST display a "Source" pill (mkauth-direct / zitadel-only / derived-from-rule)
- **AND** filter pills for project, user, source MUST be present on the filter rail

#### Scenario: All grants tab is admin-gated
- **WHEN** a non-admin session navigates to `/grants`
- **THEN** the RSC layer MUST redirect to `/`

### Requirement: Reconciliation viewer MUST diff MkAuth vs Zitadel grants

The `/grants` page MUST include a "Reconciliation" tab that renders the three drift categories returned by `GET /api/v1/reconciliation/grants`: `only_in_mkauth`, `only_in_zitadel`, and `drift` (role-set mismatches). The viewer is read-only; no remediation action MAY be invoked from this surface.

#### Scenario: Drift summary card
- **WHEN** the Reconciliation tab renders
- **THEN** a summary card MUST show three counts: only-in-mkauth, only-in-zitadel, role-mismatch
- **AND** each count MUST be clickable to scope the table below

#### Scenario: Drift row detail
- **WHEN** an admin clicks a drift row
- **THEN** a `<Drawer/>` MUST open showing both the MkAuth and Zitadel grant records side-by-side via `<JsonView/>`
- **AND** the differing fields MUST be highlighted (consistent with the Token Simulator compare highlight pattern)

#### Scenario: No remediation actions present
- **WHEN** the Reconciliation tab renders
- **THEN** the surface MUST NOT contain "Apply", "Sync", or any other action button
- **AND** auto-correction is explicitly deferred to a later change (Phase 5/6 reconciliation engine)

### Requirement: Token Simulator MUST preserve copy/highlight/compare across the redesign

The Token Simulator (per `dashboard-ux-elevation` § Token Simulator) MUST continue to support `<CopyButton/>` per JWT field, key/value tokenization via `<JsonView/>`, and side-by-side compare with differing-value highlighting. The Card variant migration to `glass` MUST NOT regress these affordances.

#### Scenario: CopyButton on each token field
- **WHEN** the Token Simulator renders a JWT payload
- **THEN** every top-level claim row MUST have a `<CopyButton/>` adjacent to it
- **AND** clicking it MUST copy the resolved value to the clipboard with a Sonner success toast

#### Scenario: Compare highlights differing values
- **WHEN** the admin selects a second user via the "Compare with" select
- **THEN** the two payload panels MUST render side-by-side
- **AND** every key whose values differ MUST be tinted amber on both sides
