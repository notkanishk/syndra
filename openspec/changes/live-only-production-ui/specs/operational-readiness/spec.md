> **Status:** Integrated | [< Index](../../../../INDEX.md)

## ADDED Requirements

### Requirement: System exposes a directory-mode diagnostic endpoint

The backend MUST expose `GET /api/v1/system/mode` (auth-gated, identical to other `/api/v1/*` routes) that returns the active directory source and whether the deployment has unexpectedly degraded to the demo fallback. The UI MUST render an unobtrusive indicator that fires only when something is unusual.

#### Scenario: Live mode reports zitadel and stays silent in the chrome
- **WHEN** `ZITADEL_DOMAIN` is set and the management client is healthy
- **AND** an authenticated admin GETs `/api/v1/system/mode`
- **THEN** the response MUST be `{"directory": "zitadel", "zitadel_configured": true, "degraded": false, "seed_active": false}`
- **AND** the sidebar `<SystemModeBadge>` MUST render `null` (no chrome noise)

#### Scenario: Demo mode reports demo
- **WHEN** `ZITADEL_DOMAIN` is unset
- **THEN** `/api/v1/system/mode` MUST return `{"directory": "demo", "zitadel_configured": false, "degraded": false, ...}`
- **AND** the sidebar MUST render a small outline "Demo mode" badge so the developer always knows they're in local-dev

#### Scenario: Degraded mode is detectable and visually prominent
- **WHEN** `ZITADEL_DOMAIN` is set but `directory.Default.Tag()` returns `"demo"` (silent fallback because the management key is missing or unreadable)
- **THEN** `/api/v1/system/mode` MUST return `{"degraded": true, "directory": "demo", "zitadel_configured": true, ...}`
- **AND** the sidebar MUST render a destructive-variant badge with "DEGRADED · demo fallback" copy
- **AND** the badge MUST include an accessible hover tooltip pointing to backend logs for the InitClient error

#### Scenario: Endpoint is auth-gated
- **WHEN** an unauthenticated request hits `/api/v1/system/mode`
- **THEN** the backend MUST reject it with the same 401 path as other `/api/v1/*` routes (no leak of deployment posture to anonymous probes)

### Requirement: Mode-badge fetch errors MUST NOT break the chrome

The frontend `fetchSystemMode()` helper MUST swallow any error path (network, decoding, auth) and return `null` so the sidebar continues to render even when the diagnostic endpoint is unreachable.

#### Scenario: /system/mode is unreachable
- **WHEN** `fetchSystemMode()` throws or returns a non-2xx response
- **THEN** the helper MUST return `null`
- **AND** `<SystemModeBadge>` MUST render `null`
- **AND** the rest of the sidebar MUST render normally
