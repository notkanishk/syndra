# Backend-Owned Onboarding & Security Boundary — Implementation Notes (Phase 3)

These notes document what was built to satisfy the requirements in this change. They were previously embedded in the spec files and have been extracted here to keep specs focused on testable requirements.

> **Requirements live in:** [production-security-boundary spec](specs/production-security-boundary/spec.md), and the consolidated canonical specs for [automation-policies](../syndra-core-architecture/specs/automation-policies/spec.md), [application-claims](../syndra-core-architecture/specs/application-claims/spec.md), [contract-quality](../contract-hardening-and-test-foundation/specs/contract-quality/spec.md), [backend-api-testing](../contract-hardening-and-test-foundation/specs/backend-api-testing/spec.md).

---

## Production Security Boundary

### Frontend PKCE Authorization Code Flow

- `ui/src/lib/oidc.ts` — PKCE crypto (`generateCodeVerifier`, `generateCodeChallenge`, `generateState`), token exchange (`exchangeCodeForToken`), and claim parsing (`parseJwtClaims`, `extractSessionFields`); no external auth library
- `ui/src/app/auth/zitadel/route.ts` — generates PKCE verifier/challenge/state; sets a short-lived `syndra_pkce` HttpOnly cookie scoped to `/auth/callback`; redirects to Zitadel `/oauth/v2/authorize`
- `ui/src/app/auth/callback/route.ts` — validates `state` (CSRF) and PKCE TTL; exchanges code for token; parses Zitadel claims to extract `sub`, display name, email, and admin role; stores raw access token in `syndra_session` cookie

### Session and Token Forwarding

- `ui/src/lib/session.ts` — `syndra_session` cookie uses a discriminated union (`type: "demo" | "oidc"`); OIDC sessions carry `accessToken` (raw JWT) and `expiresAt`; `getSession()` rejects expired OIDC tokens before they reach the backend
- `ui/src/app/api/proxy/[...path]/route.ts` — forwards `session.accessToken` as `Authorization: Bearer <token>` for OIDC sessions
- `ui/src/lib/api.ts` — all SSR server-component fetchers accept an optional `token` parameter; when `ZITADEL_DOMAIN` is set every backend call carries the user's JWT; fallback to shared API key only in demo/local-dev mode

### Admin Role Determination

- Role (`admin` | `user`) is derived from `urn:zitadel:iam:org:project:roles` in the Zitadel access token; the key is configurable via `ZITADEL_ADMIN_ROLE_KEY` env var (default `"admin"`)

### Required Env Vars (UI)

- `ZITADEL_DOMAIN` — activates OIDC mode; login page shows "Continue with Zitadel" instead of demo picker
- `ZITADEL_CLIENT_ID` — PKCE public client app ID registered in Zitadel
- `ZITADEL_ADMIN_ROLE_KEY` — role key that maps to admin in the UI (default `"admin"`)

### Backend User-Token Authorization

- `backend/internal/auth/jwt.go` — RS256 JWT validation via `golang-jwt/jwt/v5`; JWKS fetched from `https://{ZITADEL_DOMAIN}/oauth/v2/keys` with 1-hour cache; validates issuer, audience, expiry, and signing method
- `backend/internal/handlers/router.go` — `withUserAuth`: production (ZITADEL_DOMAIN set) requires a Zitadel-issued bearer token; local-dev falls back to shared API key (SYNDRA_API_KEY)
- `backend/internal/handlers/adminctx.go` — typed context key propagates acting admin user ID; mutation handlers use it for audit attribution
- Required env vars: `ZITADEL_DOMAIN`, `ZITADEL_AUDIENCE`

### Webhook Authenticity

- `backend/internal/handlers/webhook.go` — `verifyWebhookSignature(body, tsHeader, sigHeader)`: HMAC-SHA256 over `tsHeader + "\n" + body` using `ZITADEL_WEBHOOK_SECRET`; timestamp is part of the signed input so a captured signature cannot be replayed with a fresh timestamp
- `verifyWebhookFreshness`: rejects events older than 5 minutes or more than 30 seconds ahead
- Both checks skip when `ZITADEL_WEBHOOK_SECRET` is unset (local-dev)

### Action-Injection Perimeter

- `backend/internal/handlers/action.go` — Redis call wrapped with a 50 ms context timeout; cache miss, timeout, and malformed data all route through `degradedResponse`
- `backend/internal/db/repositories.go` — `GetClaimFailureMode`: returns configured `claim_failure_mode` (`fail_closed` | `minimal_safe`); distinguishes `pgx.ErrNoRows` (no profile row -> safe default) from real DB faults
- `backend/db/migrations/000005_security_boundary.up.sql` — adds `claim_failure_mode` and `minimal_safe_claims` to `claim_profiles`

### Onboarding Auditability

- `backend/db/migrations/000005_security_boundary.up.sql` — `onboarding_triggers` table: `idempotency_key UNIQUE`, `status` enum, `bundle_id`, `error_message`, `completed_at`
- `backend/internal/db/repositories.go` — `InsertOnboardingTrigger`: `ON CONFLICT DO NOTHING`; distinguishes `pgx.ErrNoRows` (duplicate -> idempotent skip) from real errors
- `backend/internal/services/onboarding.go` — `TriggerOnboarding`: records trigger -> resolves welcome bundle -> assigns -> writes audit log -> marks completed; failure marks trigger `failed` for operator visibility
- `backend/internal/handlers/onboarding.go` + `GET /api/v1/onboarding/triggers` — operator view of trigger log

### Security Boundary Path Contracts

| Path | Auth | Idempotency | Observability | Failure behavior |
|------|------|-------------|---------------|-----------------|
| `POST /api/v1/*` mutations | Zitadel JWT (prod) / API key (dev) via `withUserAuth` | Per-request | `[AUTH]` log with admin user ID | `401` on missing/invalid token |
| `POST /api/webhooks/zitadel` | HMAC-SHA256 over ts+body + freshness | `onboarding_triggers.idempotency_key` | `[WEBHOOK]` log per step | `401` bad sig / `400` stale |
| `POST /api/action/inject` | Intentionally wide-open (Actions v2 caller) | Stateless read | `[DATA PLANE]` log with cache hit/miss/timeout | `degradedResponse` per configured mode |
| `TriggerOnboarding` | Called only from verified webhook | `idempotency_key UNIQUE` insert | `[ONBOARDING]` log + `onboarding_triggers` record | `FailOnboardingTrigger` on error |

---

## Automation Policies

### Allowed Event Sources

Only two sources may signal onboarding without becoming mutation authorities:
1. A validated backend webhook carrying `role_key == "new_user"` — HMAC-SHA256 signature and freshness are verified before any work begins
2. A future manual admin trigger path

The frontend and any Zitadel-hosted trigger code cannot directly initiate mutations; they signal the backend, which decides whether and how to act.

### Idempotency, Audit, and Retry

- `backend/db/migrations/000005_security_boundary.up.sql` — `onboarding_triggers` table with `idempotency_key TEXT NOT NULL UNIQUE`
- `backend/internal/db/repositories.go` — `InsertOnboardingTrigger`: `ON CONFLICT (idempotency_key) DO NOTHING RETURNING id`; `pgx.ErrNoRows` is the expected signal for a genuine duplicate (not treated as a DB fault)
- `backend/internal/services/onboarding.go` — `TriggerOnboarding`: records trigger before any mutation; failure calls `FailOnboardingTrigger`; success calls `CompleteOnboardingTrigger`; all mutations attributed to `system:onboarding` in the audit log
- `backend/internal/handlers/onboarding.go` — `GET /api/v1/onboarding/triggers` exposes the trigger log for operator inspection

---

## Test Coverage

### JWT Authorization Tests (`backend/internal/auth/jwt_test.go`)

- Valid RS256 token accepted; subject returned
- Audience as single string and as array (Zitadel sends both)
- Expired token rejected
- Wrong audience rejected
- Wrong issuer rejected
- Tampered signature rejected
- Malformed JWT (two-part, empty, garbage) rejected
- Unknown `kid` rejected

### Webhook Authenticity Tests (`backend/internal/handlers/webhook_test.go`)

- Valid signature (over `tsHeader + "\n" + body`) accepted
- Invalid signature rejected with `401 WEBHOOK_UNAUTHORIZED`
- Replay attack: fresh timestamp with captured-body signature fails
- Missing signature header rejected
- Missing timestamp header rejected at signature check
- Stale timestamp (10 min old) rejected with `400 WEBHOOK_STALE`
- Local-dev mode (no secret) skips both checks
- Handler integration: `HandleZitadelWebhook` returns correct codes for bad signature and stale timestamp
