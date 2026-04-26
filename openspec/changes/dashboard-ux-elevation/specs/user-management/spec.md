> **Status:** Integrated | [< Index](../../../../INDEX.md)

## ADDED Requirements

### Requirement: Access Lineage MUST visually distinguish Source from Derived roles

The `/users` view MUST render Source roles (direct grants) and Derived roles (from bundles or mapping rules) with distinct visual treatment so admins can answer "Why does this user have access to X?" at a glance.

#### Scenario: Source and Derived columns are color-coded
- **WHEN** an admin opens a user's access lineage panel
- **THEN** the Source column header MUST use the primary color tint
- **AND** the Derived column header MUST use the emerald color tint
- **AND** each Source role card MUST have a primary-tinted border
- **AND** each Derived role card MUST have an emerald-tinted border with an inheritance arrow (↳) prefix on the reason line
- **AND** each role card MUST display both a human-readable label (e.g., "3D Lab · Member") and the raw `project_id:role_key` pair as a small monospace tag

### Requirement: Bundle assignment MUST preview the contained roles before confirmation

When an admin assigns a bundle to a user, the UI MUST show the exact roles that will be applied before the assignment is committed.

#### Scenario: Each bundle button shows a role-count badge
- **WHEN** the Assign Bundle panel renders
- **THEN** each bundle MUST display a Badge with the number of contained roles
- **AND** the first 4 contained roles MUST be displayed as chips below the description
- **AND** if the bundle contains more than 4 roles, a "+N more" Badge MUST be displayed

#### Scenario: Clicking Assign opens a confirmation modal listing roles
- **WHEN** an admin clicks "Assign" on a bundle
- **THEN** a modal MUST appear listing the exact roles that will be granted
- **AND** the assignment MUST only commit after the admin clicks the modal's "Assign bundle" button
- **AND** clicking Cancel or pressing Escape MUST close the modal without changing user state

### Requirement: Direct grant duration picker MUST be human-friendly

The Direct Grant form MUST surface common durations as discrete buttons rather than requiring the admin to type a raw day count.

#### Scenario: Predefined duration buttons
- **WHEN** an admin opens the Direct Grant form
- **THEN** the duration row MUST present buttons for "1 week", "1 month", "1 semester" (120 days), and "Permanent"
- **AND** a small custom-day input MUST remain available for arbitrary durations
- **AND** the selected option MUST highlight visually
- **AND** an inline preview line MUST describe the chosen duration in plain language

#### Scenario: Existing grants display countdown urgency
- **WHEN** the existing direct grants list renders
- **THEN** each grant with an expiry MUST display a countdown badge (e.g., "expires in 3 days")
- **AND** grants expiring within 7 days MUST use the destructive Badge variant
- **AND** permanent grants MUST display a neutral "Permanent" badge
