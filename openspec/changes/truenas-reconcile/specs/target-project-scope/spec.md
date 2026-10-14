## ADDED Requirements

### Requirement: Each target serves exactly one Zitadel project
A target SHALL have at most one Zitadel project, and a project SHALL serve at most one target. Every mapping on a target SHALL belong to the target's project, enforced by the database.

#### Scenario: A taken project is refused
- **WHEN** an operator sets a project already set on another target
- **THEN** the request is refused with `409 PROJECT_TAKEN` naming that target

#### Scenario: Changing a project with mappings is refused
- **WHEN** an operator changes a target's project while mappings exist on it
- **THEN** the request is refused with `409 MAPPINGS_EXIST` and the mapping count, and nothing changes

#### Scenario: Migration infers and refuses to guess
- **WHEN** migration 000051 runs and a target's mappings span two projects
- **THEN** the migration fails and names the target and both projects; no row is altered

### Requirement: No write reaches a target without its project
Every write that touches a person's access on a target, or its mapping set, SHALL be refused with `409 PROJECT_REQUIRED` while the target has no project. Reads and lifecycle changes SHALL be exempt, as SHALL the `zitadel` target.

#### Scenario: Adopt before a project is set
- **WHEN** an operator adopts an account on a target with no project
- **THEN** the request is refused with `409 PROJECT_REQUIRED` and nothing is dispatched

#### Scenario: Lifecycle stays available
- **WHEN** an operator drains a target with no project
- **THEN** the lifecycle change proceeds
