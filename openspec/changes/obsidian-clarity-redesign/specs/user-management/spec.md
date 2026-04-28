> **Status:** Proposed | [< Index](../../../../INDEX.md)

## ADDED Requirements

### Requirement: User-facing surfaces MUST replace UUID dropdowns with combobox pills

The `/users` filter rail MUST replace raw `<select>` elements populated with UUIDs with combobox-style filter pills backed by resolved entity lists. The internal value MUST remain matchable against the corresponding backend filter, but the visible label MUST be a human-readable name.

Stage 2 ships project filter pills using the project-name strings already exposed via the user list's `key_projects` field, so no UUID round-trip is required. A role filter pill backed by `useProjects()` is reserved for Stage 3 once the role facet is added to the backend `/users` query.

#### Scenario: Project filter pill shows project names
- **WHEN** the `/users` filter rail renders the project filter
- **THEN** each pill MUST display the project name (sourced from `key_projects` on the user list, or `useProjects()` once role-aware filtering lands)
- **AND** toggling a pill MUST narrow the visible user list to entries whose `key_projects` contain that name
- **AND** an aria-pressed attribute MUST reflect the on/off state for assistive tech

#### Scenario: Role filter pill shows role display names (Stage 3)
- **WHEN** the role filter is added in Stage 3
- **THEN** each pill MUST display the role display name + project name (e.g. "3D Lab · Member")
- **AND** the selected value MUST remain the `(project_id, role_key)` pair

### Requirement: Access Lineage MUST visually distinguish source vs derived without raw IDs

The Access Lineage view on `/users` MUST continue to separate source roles from derived roles (per the existing user-management spec) AND MUST do so using `<Eyebrow>SOURCE</Eyebrow>` / `<Eyebrow>DERIVED</Eyebrow>` labels with `<RoleName/>` / `<ProjectName/>` / `<BundleName/>` for every entity reference. Raw `project_id:role_key` pairs MAY be shown as a secondary monospace tag for power users (consistent with `dashboard-ux-elevation` § 2.1) but MUST NOT be the primary label.

#### Scenario: Source role row format
- **WHEN** a source (raw Zitadel) role is displayed
- **THEN** the row MUST lead with `<Eyebrow>SOURCE</Eyebrow>` and `<RoleName projectId roleKey/>` as the primary text
- **AND** the granting bundle (if any) MUST be shown via `<BundleName/>`
- **AND** the granted-by user MUST be shown via `<UserName/>`

#### Scenario: Derived role row format
- **WHEN** a derived role (via mapping rule) is displayed
- **THEN** the row MUST lead with `<Eyebrow>DERIVED</Eyebrow>` and `<RoleName projectId roleKey/>`
- **AND** the originating mapping rule's source role MUST be shown via `<RoleName/>` with the inheritance arrow ↳

### Requirement: Granted-by attribution MUST resolve to a name

Every "Granted by" or "Reviewed by" or "Approved by" attribution surfaced anywhere on `/users` MUST render through `<UserName/>` and MUST NOT show the raw `actor_id` UUID.

#### Scenario: Granted-by column on direct grants
- **WHEN** a user's direct grants list renders
- **THEN** the "Granted by" cell MUST render `<UserName id={grant.granted_by} />`
- **AND** the resolved display name MUST be visible (loading skeleton during resolution)

### Requirement: Mutation flows on /users MUST continue to use ConfirmModal

Every mutation that changes effective access on `/users` MUST gate confirmation through the styled `<ConfirmModal/>` (per `dashboard-ux-elevation`). The Modal refactor (Stage 1) MUST NOT regress this. Stage 2 covers the bundle-assign confirmation; revoke flows are introduced in Stage 4 alongside the bundle CRUD surfaces and follow the same contract.

#### Scenario: Bundle assign confirmation
- **WHEN** an admin clicks "Assign" on an unassigned bundle row
- **THEN** a `<ConfirmModal/>` MUST open titled `Assign "{bundle.name}"?`
- **AND** the description MUST list (up to six) role display names that will be added by the assignment
- **AND** Esc + click-outside MUST cancel without mutating

#### Scenario: Revoke direct grant (Stage 4)
- **WHEN** an admin clicks "Revoke" on a direct grant row
- **THEN** a `<ConfirmModal/>` MUST open with destructive variant styling
- **AND** the confirmation copy MUST name the role being revoked via `<RoleName/>` (not the raw role key)
- **AND** Esc + click-outside MUST cancel the action without mutation

### Requirement: User cards MUST surface reachable identity completeness signals

When `live-directory-identity-completeness` overlays (`Title`, `Team`, `Location`) are available from the catalog, the user card on `/users` MUST surface them — without exposing raw `attributes.{key}` UUIDs.

#### Scenario: Title and Team rendered as eyebrow + body
- **WHEN** a user has `Title` and/or `Team` metadata
- **THEN** the card MUST display them as a small eyebrow chip (e.g. "TITLE · Lab Manager", "TEAM · 3D Lab")
- **AND** missing values MUST be omitted (no "—" placeholder cluttering the card)
