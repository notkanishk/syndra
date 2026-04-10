# Requirement: Application Claim Selection & Shaping

The system MUST provide a way for downstream applications to define which roles they consume and how those roles are presented in the JWT.

## Claim selection from projects
The system MUST allow an application to be associated with a specific project context, pulling all active roles (source or derived) for that project.

### Scenario: High-precision claim scoping
- **GIVEN** a user has multiple roles across different projects (e.g., Printing, Laser, Door Access)
- **WHEN** the "Printing Portal" application requests claims for this user
- **THEN** it only receives roles relevant to the "Printing" project, ensuring least-privilege for the application.

## Claim shaping for JWT payload
The system MUST allow applications to define a custom claim name and format for their roles.

### Scenario: Shaping roles for a legacy consumer
- **GIVEN** an application that expects roles in a space-delimited string (e.g., "admin operator trainee")
- **WHEN** the application is configured with `FormatType: space_delimited` and `ClaimName: permissions`
- **THEN** the token simulation and data plane response MUST return the roles in that specific format.

## Cross-project claim propagation
The system MUST support "selecting" claims from other projects by utilizing mapping rules to project the desired roles into the application's local context.

### Scenario: Granting door access based on lab certification
- **GIVEN** a mapping rule: `IF project:printing role:calibrator THEN ADD project:doors role:3d_lab_pin`
- **WHEN** the "Door Controller" application (Project: doors) requests claims for a user with the Printing Calibrator role
- **THEN** the JWT MUST include the `3d_lab_pin` role.

## Implementation: Actions v2 Integration
The data plane MUST be implemented as a Zitadel **Actions v2** script that runs during the `token_response` or `userinfo_response` flows.
- **SetCustomClaims**: The script MUST use the v2 `claims` namespace to inject the pre-compiled roles from the MkAuth cache.
- **Latency Budget**: V2 execution MUST be optimized for sub-millisecond response times to match Zitadel's high-speed token issue performance.
- **Compatibility Boundary**: Zitadel Actions v2 MUST remain the only supported source-of-truth-facing claim integration model.


## Claim contract hardening
The system MUST treat claim-shaping payloads and action responses as hardened contracts, with strict validation and regression coverage for every supported format.

### Scenario: Unsupported claim format blocked
- **WHEN** an application configuration declares an unknown claim format
- **THEN** the backend MUST reject or fail the configuration deterministically
- **AND** automated tests MUST verify that unsupported formats do not silently degrade into a permissive payload

### Scenario: Internal contract does not replace Actions v2
- **WHEN** MkAuth introduces or changes an internal payload between the UI, backend, or sync service
- **THEN** that change MUST NOT replace, bypass, or redefine the Zitadel Actions v2 compatibility contract
- **AND** the source-of-truth-facing claim path MUST remain anchored to the Actions v2 flow
