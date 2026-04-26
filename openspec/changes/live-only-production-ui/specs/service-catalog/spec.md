> **Status:** Integrated | [< Index](../../../../INDEX.md)

## ADDED Requirements

### Requirement: Member portal MUST render an explanatory empty state when no services are available

When a member opens the portal and no applications are published yet, the Service Catalog MUST render an explanatory empty state instead of an empty grid.

#### Scenario: Member portal with zero applications
- **WHEN** a member-role session opens `/`
- **AND** `GET /api/v1/applications` returns `[]`
- **THEN** the Service Catalog card MUST render an `<EmptyState>` titled "No services available yet"
- **AND** the description MUST tell the member that an administrator hasn't published any apps they can request

#### Scenario: Published-services list in /requests is empty
- **WHEN** a member opens `/requests`
- **AND** `GET /api/v1/applications` returns `[]`
- **THEN** the "Published services" panel MUST render explanatory copy (no published apps yet)
- **AND** the request-creation form MUST surface the underlying reason (no services available) rather than presenting an empty project picker
