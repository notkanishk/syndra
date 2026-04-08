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

## Governance & Cleanup
The system MUST identify potential security risks or redundant permissions for a user.

### Scenario: Flagging excessive permissions
- **WHEN** a user has roles across more than 5 projects or has direct grants that have not been used/reviewed in 6 months
- **THEN** the system MUST surface these as "Cleanup Hints" in the user management view.
