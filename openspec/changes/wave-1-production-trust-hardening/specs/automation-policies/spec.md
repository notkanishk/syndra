> **Status:** Wave 1 delta — explicit `is_welcome` flag replaces convention-based name match | [< Index](../../../../INDEX.md)

# Requirement: Welcome Bundle Configuration (delta)

## ADDED Requirements

### Schema-enforced single welcome bundle
The `bundles` table MUST carry an `is_welcome BOOLEAN NOT NULL DEFAULT FALSE` column with a partial unique index `(is_welcome) WHERE is_welcome = TRUE`. At most one bundle MAY be marked as the welcome bundle at any time.

### Explicit-only welcome resolution
`GetWelcomeBundle` MUST return an error (`db.ErrNoWelcomeBundleConfigured`) when no bundle has `is_welcome = TRUE`. Convention-based fallbacks (name match, "first bundle by created_at") MUST NOT be used.

### Operator-facing toggle
`PUT /api/v1/bundles/{id}/welcome` MUST clear any previously-flagged bundle and mark the named bundle as welcome in a single transaction. The bundle list UI MUST show a `Welcome` badge on the flagged bundle and expose a `Set as welcome bundle` action on every other bundle row.

### Operator-only authorization
`PUT /api/v1/bundles/{id}/welcome` MUST be gated by `withOperatorAuth` (Zitadel admin project role required in production; shared API key in dev mode). Plain `withUserAuth` is insufficient because the welcome flag changes global onboarding policy — every newly-created Zitadel user receives the flagged bundle. A non-admin token MUST receive `403 FORBIDDEN`.

### Audit trail
Every `bundle.welcome_set` action MUST emit an `audit_logs` entry attributed to the operator (or `system` in dev mode without `?actor=`).

### Startup warning when unconfigured
At backend startup (after DB connection + seed), the process MUST query for the welcome bundle and emit a clear `[STARTUP] WARNING: no welcome bundle configured ...` log line when none is set. This is observability-only — it does NOT block startup. Migration `000012_welcome_bundle_flag.up.sql` intentionally leaves `is_welcome=FALSE` on every existing row; operators upgrading from a prior deployment will see the warning until they explicitly mark a bundle.

## REMOVED behaviour

- Name-pattern match `LOWER(name) LIKE '%welcome%'` is removed.
- "First bundle by created_at" silent fallback is removed.

(Audit ref: D1)
