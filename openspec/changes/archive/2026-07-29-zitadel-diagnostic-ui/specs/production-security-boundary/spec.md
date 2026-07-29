## MODIFIED Requirements

### Requirement: Backend authorization is authoritative for privileged mutations
The backend MUST be the final authorization authority for privileged administrative mutations. Every endpoint under `/api/v1/zitadel/*` — including diagnostic read probes such as the M2M health check — MUST require a Zitadel-issued admin access token (`withOperatorAuth`). A shared internal API key MUST NOT be a sufficient substitute in production mode (when `ZITADEL_DOMAIN` is configured).

#### Scenario: Admin mutation reaches backend
- **WHEN** a privileged grant, revoke, bundle assignment, onboarding, or `/api/v1/zitadel/*` mutation is submitted
- **THEN** the request MUST carry a Zitadel-issued user access token
- **AND** the backend MUST validate that token, identify the acting admin, and evaluate their authorization before executing the mutation
- **AND** possession of a shared internal API key alone MUST NOT be treated as sufficient production authorization

#### Scenario: Diagnostic health probe reaches backend
- **WHEN** an operator requests `GET /api/v1/zitadel/health` from the admin UI
- **THEN** the request MUST carry a Zitadel-issued admin access token with the configured admin role key
- **AND** the backend MUST reject the request with 403 if the token lacks the admin role
- **AND** the same `withOperatorAuth` chain that gates discovery and mutation endpoints MUST gate the health probe

#### Scenario: Development-mode cmdline probe
- **WHEN** `ZITADEL_DOMAIN` is unset (local development mode)
- **THEN** the `/api/v1/zitadel/health` endpoint MUST continue to accept `MKAUTH_API_KEY` via the `withUserAuth` fallback
- **AND** no production deployment MUST rely on this fallback
