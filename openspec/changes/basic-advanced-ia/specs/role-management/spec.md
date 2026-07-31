> **Status:** basic-advanced-ia delta — role → members, honest partial coverage | [< Index](../../../../INDEX.md)

# Requirement: Role Management (delta)

## ADDED Requirements

### Requirement: Role → members MUST be answerable

`GET /api/v1/projects/{id}/roles/{key}/members` MUST return every effective holder of a `(project, role)` pair with the access sources that produced each one, ordered Direct → Via bundle → Automatic, plus per-source counts for the filter pills.

A holder whose source is a direct grant MUST carry that grant's id, since a row cannot offer a removal it has no identifier for.

The members list MUST be an empty array rather than null when nobody holds the role, so an empty state is distinguishable from a failed load.

#### Scenario: Every source on a row

- **GIVEN** a person holds `pLaser/trained` directly, through a bundle, and through a rule
- **WHEN** the role's members are fetched
- **THEN** their row MUST list all three sources in the fixed order

#### Scenario: Role metadata is best-effort

- **GIVEN** a role that exists in the identity provider but has no MkAuth-local row
- **WHEN** its members are fetched
- **THEN** the request MUST succeed with the members listed and the display metadata absent

### Requirement: A partially-backed list MUST say so

`GET /api/v1/roles` resolves through `GetAllLocalRoles`, which returns only roles created through MkAuth. The cross-project role index MUST carry an explicit scope notice stating that roles created directly in the identity provider are not listed, with a link to check upstream.

Silently partial lists are how somebody concludes a role does not exist and creates a duplicate.

#### Scenario: The scope is stated, not implied

- **WHEN** the `/roles` index renders
- **THEN** a notice MUST state that only MkAuth-managed roles are shown

### Requirement: Project MUST be the first column of any cross-project role list

The same key in two projects means two different things. The project column MUST come first and MUST NOT collapse.

#### Scenario: Two projects, one key

- **GIVEN** `Laser Lab / trained` and `Metal Shop / trained` both exist
- **WHEN** the role index renders
- **THEN** each row MUST lead with its project
