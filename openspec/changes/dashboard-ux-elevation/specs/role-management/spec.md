> **Status:** Integrated | [< Index](../../../../INDEX.md)

## ADDED Requirements

### Requirement: Project view MUST show per-role bundle and rule rollups

The `/projects` page MUST let admins drill into each project's role catalog to see exactly which bundles consume the role and which mapping rules reference it as a source or target.

#### Scenario: Click-to-expand role row
- **WHEN** an admin opens a project card
- **THEN** each role in the Role Catalog MUST be presented as a clickable row with `aria-expanded`
- **AND** the row's collapsed state MUST display summary counts (bundles, rules in, rules out)

#### Scenario: Expanded role surfaces bundles and rules
- **WHEN** an admin expands a role row
- **THEN** the expanded panel MUST list:
  - bundles that include this role (one Badge per bundle)
  - mapping rules where this role is the target ("Inherited from") with the source pair shown
  - mapping rules where this role is the source ("Triggers") with the target pair shown
- **AND** if no bundles or rules reference the role, the panel MUST display an explanatory empty line
