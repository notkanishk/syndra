# production-security-boundary Specification

## Purpose
TBD - created by archiving change wave-3-frontend-remainder-and-consolidation. Update Purpose after archive.
## Requirements
### Requirement: The admin/member routing and proxy boundary MUST be regression-tested

The admin/member split is enforced in two places — the Next.js `middleware.ts` (route gating) and the `/api/proxy/[...path]` route (backend-call gating). These enforcements MUST carry automated tests so a regression is caught before deploy, not in production.

#### Scenario: Member is redirected away from admin-only routes
- **WHEN** a session with `role: "user"` requests an admin-only path (applications, audit, bundles, graph, policies, projects, users)
- **THEN** `middleware.ts` redirects to `/`

#### Scenario: Stale demo cookie is cleared in production mode
- **WHEN** `ZITADEL_DOMAIN` is set and a request carries a demo/legacy session cookie
- **THEN** `middleware.ts` redirects to `/login` and clears the cookie (maxAge 0)

#### Scenario: Member proxy calls are self-scoped and allowlisted
- **WHEN** a `role: "user"` session calls the proxy for `users/{ownId}/grants`
- **THEN** the call is permitted
- **WHEN** the same session calls `users/{otherId}/grants` or any non-allowlisted path
- **THEN** the proxy returns 403
- **AND** a member GET is limited to `catalog`, `applications`, `requests`, or self-scoped paths, and a member POST is limited to `requests`

#### Scenario: Member request writes are pinned to the caller
- **WHEN** a `role: "user"` session POSTs an access request through the proxy
- **THEN** `requester_id` is forced to the session's own id
- **AND** a member's `requests` list response is filtered to rows where `requester_id` equals the session id

### Requirement: Every admin page MUST be gated by one enforcement mechanism, and both mechanisms MUST be tested

Every admin page MUST be gated by exactly one of two mechanisms: the `middleware.ts` `ADMIN_ONLY_PATHS` list (redirect before render) or a page-level server guard (`getSession()` → `redirect`). No admin page may be left gated by neither. Both mechanisms MUST carry regression tests so a page cannot silently ship ungated (as `/zitadel` did before this change).

#### Scenario: Middleware-gated admin paths redirect members
- **WHEN** a `role: "user"` session requests any path in `ADMIN_ONLY_PATHS`
- **THEN** middleware redirects to `/`

#### Scenario: Page-gated admin routes redirect members and the unauthenticated
- **WHEN** a non-admin session invokes a page-gated admin route (`/operations`, `/operations/cascades`, `/governance/pending`, `/governance/drift`, `/grants`, `/zitadel`)
- **THEN** the page's server guard calls `redirect("/")`
- **AND** when there is no session, it calls `redirect("/login")`
- **AND** the client island never hydrates for a non-admin

#### Scenario: The Zitadel diagnostics surface is admin-gated
- **WHEN** a `role: "user"` session navigates to `/zitadel`
- **THEN** the server guard redirects to `/` before the diagnostics surface hydrates (previously it rendered, protected only by proxy 403s on its data calls)

### Requirement: Production trust boundary gate
MkAuth MUST satisfy explicit trust-boundary controls before live Zitadel-backed orchestration is treated as production-ready.

#### Scenario: Production rollout readiness review
- **WHEN** the project is evaluated for live orchestration readiness
- **THEN** the system MUST demonstrate backend user-token authorization, validated webhook authenticity, bounded and observable action-injection behavior, and documented degraded behavior for claim injection
- **AND** missing any of those controls MUST block production-ready classification

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

### Requirement: Webhook authenticity is verified before orchestration
The system MUST verify webhook authenticity and freshness before allowing cache invalidation, onboarding triggers, or downstream mutation work to proceed.

#### Scenario: Unverified webhook received
- **WHEN** MkAuth receives a structurally valid but unverified webhook payload
- **THEN** the system MUST reject it as non-authoritative for orchestration
- **AND** no downstream mutation or cache invalidation MUST occur

### Requirement: Action-injection perimeter is production-hardened
The claim-injection path MUST be bounded, observable, and operate with a documented security posture. The endpoint is intentionally unauthenticated because it is called by Zitadel Actions v2 during the token flow; security relies on network isolation (internal-only access) and deterministic degraded behavior rather than caller authentication.

#### Scenario: Action injection under degraded dependency conditions
- **WHEN** the claim path encounters a timeout, cache miss, malformed cache entry, or unreachable dependency
- **THEN** the system MUST apply the application's documented failure posture
- **AND** the degraded outcome MUST be observable to operators

### Requirement: High-risk orchestration failures are auditable
The system MUST leave an auditable trail for onboarding and other high-risk orchestration outcomes.

#### Scenario: Welcome-bundle assignment fails
- **WHEN** MkAuth cannot complete a backend-owned onboarding mutation
- **THEN** the failed attempt MUST be visible through audit or operator-facing diagnostics
- **AND** the retry path MUST avoid duplicate grants

