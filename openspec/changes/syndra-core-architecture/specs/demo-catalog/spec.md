> **Status:** Integrated | [< Index](../../../../INDEX.md) | [Feature Coverage](../feature-coverage.md)

## ADDED Requirements

### Requirement: Seeded demo catalog
The system MUST expose a seeded demo catalog of users, projects, and applications for local development and test drives.

#### Scenario: Load seeded catalog
- **WHEN** a developer opens the dashboard or queries the catalog API in a local environment
- **THEN** the system returns demo users, demo projects, and demo applications without any manual setup

### Requirement: Catalog-backed dashboard views
The system MUST use the seeded catalog to populate overview, user, project, application, bundle, and policy views.

#### Scenario: Dashboard renders without live Zitadel
- **WHEN** the backend is running with demo data only
- **THEN** the dashboard still renders the main admin views with meaningful dummy records

### Requirement: Claim metadata is visible
The system MUST expose application claim metadata and project role metadata in the catalog payloads.

#### Scenario: Inspect application output shape
- **WHEN** a developer inspects an application record
- **THEN** the payload includes the claim name, format type, and consumer context
