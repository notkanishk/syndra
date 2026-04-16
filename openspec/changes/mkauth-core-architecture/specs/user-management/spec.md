# Requirement: Unified User access Management

The system MUST provide a comprehensive view of a user's access across all projects and groups in the ecosystem.

## Access Lineage ("Why do I have this?")
The system MUST track and display the source of every effective role assigned to a user.

### Scenario: Auditing a derived role
- **GIVEN** a user has the `door_access` role due to a mapping rule from their `printing_staff` status
- **WHEN** an admin views the user's access profile
- **THEN** the system MUST display the role as "Derived" and link it back to the `printing_staff` source role and the specific mapping rule.

## Direct vs. Bundle Management
The system MUST support both high-level "Bundle" assignments and fine-grained "Direct Grants."

### Scenario: Assigning a student bundle
- **GIVEN** a "Freshman Maker" bundle containing basic printing and wiki roles
- **WHEN** an admin assigns this bundle to a new user
- **THEN** all underlying roles MUST propagate to the user's effective access list across the respective projects.

## Advanced Filter Engine
The system MUST allow admins to filter the global user list across multiple dimensions to perform precise audits and maintenance as part of MkAuth's admin-console-first workflow.

### Scenario: Filtering by project and role
- **WHEN** an admin applies a filter for `Project: Printing Lab` AND `Role: Calibrator`
- **THEN** only users who possess that specific role in that specific project context MUST be displayed.

### Scenario: Temporal filtering
- **WHEN** an admin filters by `Account Age: < 30 days`
- **THEN** the list MUST show only newly created users who may need their "Welcome" assignments verified.

## Governance & Cleanup
The system MUST identify potential security risks or redundant permissions for a user.

### Scenario: Flagging excessive permissions
- **WHEN** a user has roles across more than 5 projects or has direct grants that have not been used/reviewed in 6 months
- **THEN** the system MUST surface these as "Cleanup Hints" in the user management view.

> **Status:** The Advanced Filter Engine and Governance & Cleanup requirements above are specified but deferred to Phase 5. Currently only basic project-level filtering exists.
