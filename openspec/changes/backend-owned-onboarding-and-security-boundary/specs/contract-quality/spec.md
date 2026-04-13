## MODIFIED Requirements

### Requirement: Production orchestration trust boundary is explicit
The system MUST treat production authorization and orchestration edges as explicit contract boundaries rather than as deployment-time assumptions.

#### Scenario: Privileged production path review
- **WHEN** a backend-owned orchestration path can grant, revoke, onboard, or emit provisioning work in production
- **THEN** the contract for that path MUST define authentication, authorization, idempotency, observability, and failure behavior
- **AND** the path MUST NOT rely on undocumented trust in frontend or trigger-origin assumptions

---

## Implementation notes (Phase 3)

Each security-boundary path now has an explicit contract:

| Path | Auth | Idempotency | Observability | Failure behavior |
|------|------|-------------|---------------|-----------------|
| `POST /api/v1/*` mutations | Zitadel JWT (prod) / API key (dev) via `withUserAuth` | Per-request | `[AUTH]` log with admin user ID | `401` on missing/invalid token |
| `POST /api/webhooks/zitadel` | HMAC-SHA256 over ts+body + freshness | `onboarding_triggers.idempotency_key` | `[WEBHOOK]` log per step | `401` bad sig / `400` stale |
| `POST /api/action/inject` | Intentionally wide-open (Actions v2 caller) | Stateless read | `[DATA PLANE]` log with cache hit/miss/timeout | `degradedResponse` per configured mode |
| `TriggerOnboarding` | Called only from verified webhook | `idempotency_key UNIQUE` insert | `[ONBOARDING]` log + `onboarding_triggers` record | `FailOnboardingTrigger` on error |

All paths log with structured prefixes (`[AUTH]`, `[WEBHOOK]`, `[DATA PLANE]`, `[ONBOARDING]`) so failures are distinguishable in operator logs without parsing free-form text.
