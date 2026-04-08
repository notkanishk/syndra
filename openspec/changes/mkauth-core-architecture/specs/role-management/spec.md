# Requirement: Advanced Role Management

The system MUST provide a robust interface for creating and cloning roles within Zitadel projects.

## Role Creation & Metadata
Admins MUST be able to define new roles for any project managed by MkAuth.

### Scenario: Defining a specialized lab role
- **WHEN** an admin creates a new role in the "Laser Lab" project with key `safety_marshall`
- **THEN** MkAuth MUST propagate this creation to Zitadel and update the local role cache.

## Role Cloning (Snapshot & Fork)
To accelerate setup, the system MUST allow admins to "copy" an existing role's metadata when creating a new role.

### Scenario: Cloning a role across projects
- **GIVEN** an existing role `admin` in "Printing Lab" with description "Full project access"
- **WHEN** an admin selects "Clone from" during creation of a role in "Laser Lab"
- **THEN** the name and description fields MUST be pre-populated.
- **AND** the new role MUST be created as a distinct, independent entity in the "Laser Lab" project.

## Consolidated Role Inventory
The system MUST provide a unified view that lists all roles across all projects to assist in global auditing and management.

### Scenario: Auditing role saturation
- **WHEN** an admin views the Global Role Catalog
- **THEN** every role entry MUST display:
    - **Parent Project**: The project the role belongs to (e.g., `Printing Lab`).
    - **Usage Count**: Number of Bundles and Mapping Rules utilizing the role.
    - **Assigned User Count**: Total number of unique users who hold that role (Effective Access).

### Scenario: Identifying orphaned roles
- **WHEN** a role has a `Usage Count` of 0 and an `Assigned User Count` of 0
- **THEN** it MUST be visually flagged as "Unused" to suggest cleanup.

## Global Disambiguation
The system MUST ensure that users and admins can distinguish between roles with identical names in different projects.

### Scenario: Visualizing global access
- **WHEN** a user's roles are displayed in a global view (e.g., User Profile, Audit Log, Topology)
- **THEN** every role MUST be prefixed or labeled with its parent project (e.g., `Printing Lab: admin` vs `Laser Lab: admin`).
