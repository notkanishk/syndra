## Why

Syndra's `direct_role_grants.expires_at` can be set (e.g. semester-bound access), but nothing enforces expiry. Expired grants silently persist: rows stay in the DB, LLDAP group memberships remain, and any Zitadel derived grants that the direct grant produced linger. There is no audit trail recording the lapse.

Effective access at query time is already correct — `GetDirectGrantsForUser(..., includeExpired=false)` filters expired rows out of views and the cache compiler. The gap is the **cleanup side-effects**: LLDAP membership removal via provisioning intents, Zitadel derived-grant cascade, Redis cache invalidation, audit trail, and hard-deleting the row (per existing convention — `direct_role_grants` has no status column).

This change closes Phase 5's **Grant Expiration Scheduler** item ([ROADMAP.md](../syndra-core-architecture/ROADMAP.md) line 55). It is also Syndra's first backend-side background worker, establishing a pattern future periodic jobs (access-request expiry, token-rotation reminders) should follow.

## What Changes

- Adds a `backend/internal/services/expiry/` package containing a `Scheduler` struct (ticker loop with panic-recover + startup run-once) and a `sweep` function that groups expired grants per user and runs an ordered cleanup pipeline against existing primitives.
- Adds `db.GetExpiredDirectGrants(ctx, limit int)` — predicate `expires_at IS NOT NULL AND expires_at <= NOW()`. The existing `GetExpiringDirectGrants` (window query) is left untouched.
- Adds `db.DeleteExpiredDirectGrantsByIDs(ctx, userID, ids)` — user-scoped *guarded* hard-delete: re-validates `expires_at <= NOW()` atomically inside the `DELETE` and returns only the rows that were actually removed. This prevents revoking a grant that was renewed between fetch and delete (`UpsertDirectGrant` uses `ON CONFLICT DO UPDATE`, so renewals reuse the row ID and would otherwise be invisible to snapshot-driven logic).
- Adds `services.EmitProvisioningIntentFromScheduler` — a thin wrapper that discriminates the provisioning-intent idempotency key by `grantID`, preventing the pre-existing collision on re-grant of the same `(user, project, role)` tuple (see Design §"Idempotency").
- Wires the scheduler into `cmd/api/main.go` sharing the same `signal.NotifyContext` as the HTTP server, with three env-var knobs: `EXPIRY_SCHEDULER_ENABLED` (default `true`), `EXPIRY_SCHEDULER_INTERVAL` (default `5m`), `EXPIRY_SCHEDULER_BATCH_SIZE` (default `500`, clamped to `[1, 10000]`). Main blocks on `sched.Done()` (bounded by the shutdown deadline) before closing `db.PG` / `db.Redis` so in-flight sweep work doesn't race connection teardown.

## Capabilities

### Modified Capabilities

- `access-governance`: Integrated (filters deferred P5) → **Integrated (bulk ops deferred P5)**. Drops the "expiry enforcement deferred P5" note; updates the *Grant expiration enforcement* requirement to Integrated and adds two scenarios covering retry and best-effort Zitadel cascade.

## Impact

- **No migration required** — `direct_role_grants` already has `expires_at` and the `idx_direct_role_grants_expiry` index.
- Modifies: `backend/internal/db/repositories.go`, `backend/internal/services/provisioning.go`, `backend/cmd/api/main.go`, `.env.example`.
- Creates: `backend/internal/services/expiry/{scheduler,sweep,deps}.go` and companion tests.
- Living docs updated: `openspec/INDEX.md`, `openspec/changes/syndra-core-architecture/ROADMAP.md`, `openspec/changes/syndra-core-architecture/specs/feature-coverage.md`, `openspec/changes/syndra-core-architecture/specs/access-governance/spec.md`.
- 14 new tests (9 sweep + 5 scheduler lifecycle).
- Zero new Go-module dependencies.
- **Known limitation inherited from Phase 5 "Partial Failure Rollback"**: Zitadel cascade is best-effort. If Zitadel is unreachable during a sweep, derived-grant orphans may remain and will be surfaced for the future reconciler to clean up.
- **Single-instance assumption**: the scheduler should run on exactly one backend replica. Operators can set `EXPIRY_SCHEDULER_ENABLED=false` on extras. PG advisory-lock leader election is a future enhancement, not shipped here.
