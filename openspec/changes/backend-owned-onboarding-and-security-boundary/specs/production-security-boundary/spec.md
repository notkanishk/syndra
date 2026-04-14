## ADDED Requirements

### Requirement: Production trust boundary gate
MkAuth MUST satisfy explicit trust-boundary controls before live Zitadel-backed orchestration is treated as production-ready.

#### Scenario: Production rollout readiness review
- **WHEN** the project is evaluated for live orchestration readiness
- **THEN** the system MUST demonstrate backend user-token authorization, validated webhook authenticity, authenticated action-injection access, and documented degraded behavior for claim injection
- **AND** missing any of those controls MUST block production-ready classification

### Requirement: Backend authorization is authoritative for privileged mutations
The backend MUST be the final authorization authority for privileged administrative mutations.

#### Scenario: Admin mutation reaches backend
- **WHEN** a privileged grant, revoke, bundle assignment, or onboarding mutation is submitted
- **THEN** the request MUST carry a Zitadel-issued user access token
- **AND** the backend MUST validate that token, identify the acting admin, and evaluate their authorization before executing the mutation
- **AND** possession of a shared internal API key alone MUST NOT be treated as sufficient production authorization

### Requirement: Webhook authenticity is verified before orchestration
The system MUST verify webhook authenticity and freshness before allowing cache invalidation, onboarding triggers, or downstream mutation work to proceed.

#### Scenario: Unverified webhook received
- **WHEN** MkAuth receives a structurally valid but unverified webhook payload
- **THEN** the system MUST reject it as non-authoritative for orchestration
- **AND** no downstream mutation or cache invalidation MUST occur

### Requirement: Action-injection perimeter is production-hardened
The claim-injection path MUST use an authenticated, bounded, and observable production perimeter.

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

---

## Implementation notes (Phase 3 — frontend OIDC)

The "Backend authorization is authoritative" requirement is now fully satisfied end-to-end. The frontend no longer relies on a shared API key as the primary identity proof for privileged requests.

**Frontend PKCE authorization code flow**
- `ui/src/lib/oidc.ts` — PKCE crypto (`generateCodeVerifier`, `generateCodeChallenge`, `generateState`), token exchange (`exchangeCodeForToken`), and claim parsing (`parseJwtClaims`, `extractSessionFields`); no external auth library
- `ui/src/app/auth/zitadel/route.ts` — generates PKCE verifier/challenge/state; sets a short-lived `mkauth_pkce` HttpOnly cookie scoped to `/auth/callback`; redirects to Zitadel `/oauth/v2/authorize`
- `ui/src/app/auth/callback/route.ts` — validates `state` (CSRF) and PKCE TTL; exchanges code for token; parses Zitadel claims to extract `sub`, display name, email, and admin role; stores raw access token in `mkauth_session` cookie

**Session and token forwarding**
- `ui/src/lib/session.ts` — `mkauth_session` cookie now uses a discriminated union (`type: "demo" | "oidc"`); OIDC sessions carry `accessToken` (raw JWT) and `expiresAt`; `getSession()` rejects expired OIDC tokens before they reach the backend
- `ui/src/app/api/proxy/[...path]/route.ts` — forwards `session.accessToken` as `Authorization: Bearer <token>` for OIDC sessions
- `ui/src/lib/api.ts` — all SSR server-component fetchers accept an optional `token` parameter; when `ZITADEL_DOMAIN` is set every backend call carries the user's JWT; fallback to shared API key only in demo/local-dev mode

**Admin role determination**
- Role (`admin` | `user`) is derived from `urn:zitadel:iam:org:project:roles` in the Zitadel access token; the key is configurable via `ZITADEL_ADMIN_ROLE_KEY` env var (default `"admin"`)

**Required env vars (UI)**
- `ZITADEL_DOMAIN` — activates OIDC mode; login page shows "Continue with Zitadel" instead of demo picker
- `ZITADEL_CLIENT_ID` — PKCE public client app ID registered in Zitadel
- `ZITADEL_ADMIN_ROLE_KEY` — role key that maps to admin in the UI (default `"admin"`)

---

## Implementation notes (Phase 3 — backend security boundary)

All requirements above are satisfied by the implementation shipped in this change.

**Backend user-token authorization**
- `backend/internal/auth/jwt.go` — RS256 JWT validation via `golang-jwt/jwt/v5`; JWKS fetched from `https://{ZITADEL_DOMAIN}/oauth/v2/keys` with 1-hour cache; validates issuer, audience, expiry, and signing method
- `backend/internal/handlers/router.go` — `withUserAuth`: production (ZITADEL_DOMAIN set) requires a Zitadel-issued bearer token; local-dev falls back to shared API key (MKAUTH_API_KEY)
- `backend/internal/handlers/adminctx.go` — typed context key propagates acting admin user ID; mutation handlers use it for audit attribution
- Required env vars: `ZITADEL_DOMAIN`, `ZITADEL_AUDIENCE`

**Webhook authenticity**
- `backend/internal/handlers/webhook.go` — `verifyWebhookSignature(body, tsHeader, sigHeader)`: HMAC-SHA256 over `tsHeader + "\n" + body` using `ZITADEL_WEBHOOK_SECRET`; timestamp is part of the signed input so a captured signature cannot be replayed with a fresh timestamp
- `verifyWebhookFreshness`: rejects events older than 5 minutes or more than 30 seconds ahead
- Both checks skip when `ZITADEL_WEBHOOK_SECRET` is unset (local-dev)

**Action-injection perimeter**
- `backend/internal/handlers/action.go` — Redis call wrapped with a 50 ms context timeout; cache miss, timeout, and malformed data all route through `degradedResponse`
- `backend/internal/db/repositories.go` — `GetClaimFailureMode`: returns configured `claim_failure_mode` (`fail_closed` | `minimal_safe`); distinguishes `pgx.ErrNoRows` (no profile row → safe default) from real DB faults (error returned and logged)
- `backend/db/migrations/000005_security_boundary.up.sql` — adds `claim_failure_mode` and `minimal_safe_claims` to `claim_profiles`

**Onboarding auditability**
- `backend/db/migrations/000005_security_boundary.up.sql` — `onboarding_triggers` table: `idempotency_key UNIQUE`, `status` enum, `bundle_id`, `error_message`, `completed_at`
- `backend/internal/db/repositories.go` — `InsertOnboardingTrigger`: `ON CONFLICT DO NOTHING`; distinguishes `pgx.ErrNoRows` (duplicate → idempotent skip) from real errors
- `backend/internal/services/onboarding.go` — `TriggerOnboarding`: records trigger → resolves welcome bundle → assigns → writes audit log → marks completed; failure marks trigger `failed` for operator visibility
- `backend/internal/handlers/onboarding.go` + `GET /api/v1/onboarding/triggers` — operator view of trigger log
