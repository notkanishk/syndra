## MODIFIED Requirements

### Requirement: Backend-owned welcome-bundle assignment
The system MUST treat Welcome Bundle assignment as a backend-owned business mutation rather than as a direct Zitadel-hosted mutation flow.

#### Scenario: New user detected through compatible trigger intake
- **WHEN** Zitadel signals that a new user account was created
- **THEN** MkAuth MAY accept that signal through an Actions v2-compatible trigger path or a validated backend webhook
- **AND** the MkAuth Backend MUST perform the actual welcome-bundle assignment

#### Scenario: Retry-safe onboarding mutation
- **WHEN** MkAuth retries a previously attempted welcome-bundle assignment
- **THEN** the operation MUST be idempotent
- **AND** the system MUST avoid duplicate grants while preserving operator visibility into the retry outcome

---

## Implementation notes (Phase 3)

**Allowed event sources (task 2.3)**
Only two sources may signal onboarding without becoming mutation authorities:
1. A validated backend webhook carrying `role_key == "new_user"` — HMAC-SHA256 signature and freshness are verified before any work begins
2. A future manual admin trigger path

The frontend and any Zitadel-hosted trigger code cannot directly initiate mutations; they signal the backend, which decides whether and how to act.

**Idempotency, audit, and retry (task 2.2)**
- `backend/db/migrations/000005_security_boundary.up.sql` — `onboarding_triggers` table with `idempotency_key TEXT NOT NULL UNIQUE`
- `backend/internal/db/repositories.go` — `InsertOnboardingTrigger`: `ON CONFLICT (idempotency_key) DO NOTHING RETURNING id`; `pgx.ErrNoRows` is the expected signal for a genuine duplicate (not treated as a DB fault)
- `backend/internal/services/onboarding.go` — `TriggerOnboarding`: records trigger before any mutation; failure calls `FailOnboardingTrigger`; success calls `CompleteOnboardingTrigger`; all mutations attributed to `system:onboarding` in the audit log
- `backend/internal/handlers/onboarding.go` — `GET /api/v1/onboarding/triggers` exposes the trigger log for operator inspection
