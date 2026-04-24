# Design: Grant Expiration Scheduler

## Goals

1. Enforce `direct_role_grants.expires_at` end-to-end: when `expires_at <= NOW()`, the row is deleted, LLDAP group membership is removed, Redis cache is invalidated, an audit row records the revocation, and any derived Zitadel grants are cascade-revoked.
2. Reuse every existing primitive. No new schemas, no new mutation surfaces.
3. Establish the **first** backend-side background worker in a reusable shape.

## Non-goals

- Compensating revocations on Zitadel partial failure — that's its own Phase 5 item (`Partial Failure Rollback`). This scheduler inherits the same log-and-continue compromise.
- Bundle grant expiry (bundles don't have `expires_at`).
- Access request expiry (`access_requests` has `status`, not `expires_at`).
- Leader election for multi-instance backends.
- Observability metrics beyond structured logs.

## Architecture

```
                 ┌─ Scheduler.Start(ctx) ─────────────────────┐
                 │  runOnce (at boot)  ─┐                      │
                 │  time.Ticker ────────┼──► runOnce (panic recovered)
                 │                      │          │           │
                 │                      │          ▼           │
                 │                      │      sweep(ctx,batch)│
                 │                      │          │           │
                 └──────────────────────┴──────────┼───────────┘
                                                  │
                                                  ▼
          db.GetExpiredDirectGrants(ctx, batchSize)
                                                  │
                                                  ▼
                                           groupByUser
                                                  │
          per user ────► processUser(ctx, userID, userGrants):
                           1. EmitProvisioningIntentFromScheduler  (per grant, idempotent)
                           2. DeleteDirectGrantsByIDs              (single user-scoped SQL)
                           3. InsertAuditLog                        (one per deleted grant)
                           4. cache.InvalidateUser                  (once per user)
                           5. zitadel.RevokeMappingRules            (per unique project|role, best-effort)
```

Every step is a function variable (injectable dep), mirroring `services/deps.go`, `cache/deps.go`, `zitadel/deps.go`. Tests swap the vars via `t.Cleanup` — no testcontainers, no sqlmock, no live Redis.

## Sweep order — partial-failure semantics are load-bearing

### Per-user pipeline

| Step | Failure handling | Rationale |
|---|---|---|
| 1. Guarded atomic delete (`DELETE … WHERE id = ANY(…) AND expires_at <= NOW() RETURNING *`) | On error, abort all downstream work for this user. On zero rows returned, skip — every candidate was concurrently renewed or removed. | **The DB is the sole source of truth.** `UpsertDirectGrant` renews grants via `ON CONFLICT DO UPDATE`, so the row ID is reused; a renewal between fetch and delete would otherwise look identical to the pre-fetch snapshot. Re-validating `expires_at <= NOW()` atomically inside the `DELETE` is the only way to guarantee the sweep cannot revoke an active grant. |
| 2. Audit (per row actually returned) | Log and continue. | Audit immediately after the authoritative DB commit so the trail survives any later side-effect failure. Driven off the RETURNING slice, never the pre-fetch snapshot. |
| 3. Emit provisioning intent (per row returned) | Log-and-continue. LLDAP orphan surfaced for the reconciler. | Intent is idempotent via grantID-discriminated key (Bug Fix #3). A failed emit cannot be retried by a later sweep (the row is already gone), so this is the one remaining compensating-revocation gap — same deferred Phase-5 compromise `RevokeMappingRules` already accepts. |
| 4. Cache invalidate (once per user, only when anything was deleted) | Log-and-continue. | `InvalidateUser` (lazy rebuild on next request) matches the webhook convention. Invalidating *before* delete would let the next request recompile a cache that still includes the expired grant. |
| 5. Zitadel cascade (per unique project\|role) | Log-and-continue. | Inherits the existing Phase-5 deferred-compensating-revocation compromise. Orphaned derived grants are surfaced for the future reconciler. |

### Why delete-first (with re-validation) rather than intent-first?

The naive design emitted intents before deleting. That choice was "self-healing" on crash (intent idempotency absorbed re-runs), but it was **unsafe under concurrent renewal**: the snapshot-driven decision cannot distinguish an expired row from a renewed-in-place row sharing the same ID. Re-validating under the guarded `DELETE` eliminates the race. The remaining crash window (delete committed, intent not yet emitted) is a pure LLDAP-orphan risk, which is already the documented compromise for every other mutation pipeline that touches Zitadel or LLDAP after a DB commit.

### Why audit between delete and intent?

Audit is an internal durable trail. Writing it first after the delete commits guarantees the revocation is recorded even if every external side-effect below fails. Writing audit before delete would create "revoked by expiry" entries for grants still present if a crash landed between.

## Idempotency — the critical bug in the naive design

`services.EmitProvisioningIntent` computes its idempotency key as `action:uid:group:webhookEventID`. With `webhookEventID=""` (scheduler-origin), this sequence collides:

1. Grant `G1 = (user U, project P, role R)` created and later expires.
2. Scheduler sweep: emits intent with key `remove:U:P-R:` (empty event ID). Row deleted.
3. Admin re-grants `G2 = (user U, project P, role R)` (new row, same tuple by uniqueness).
4. G2 later expires.
5. Scheduler sweep: computes the *same* key `remove:U:P-R:`. `ON CONFLICT DO NOTHING` silently skips the insert.
6. Sync service never processes a removal intent for G2 → LLDAP membership lingers forever.

`EmitProvisioningIntentFromScheduler` fixes this by computing `remove:U:P-R:sched:G2_id` — a fresh key per grant row. `webhook_event_id` stays `NULL`, so intents remain unambiguously scheduler-originated. `TestSweep_IntentIdempotencyAcrossReGrants` protects the fix.

## Config

| Env var | Default | Clamped | Parsed in |
|---|---|---|---|
| `EXPIRY_SCHEDULER_ENABLED` | `true` | — | `main.go` before goroutine launch |
| `EXPIRY_SCHEDULER_INTERVAL` | `5m` | `≤0` falls back to default | `time.ParseDuration` |
| `EXPIRY_SCHEDULER_BATCH_SIZE` | `500` | `[1, 10000]` | `strconv.Atoi` |

The scheduler constructor (`NewScheduler`) re-clamps defensively so tests and callers passing raw values can't produce pathological loops.

## Lifecycle

- **Startup**: `runOnce` fires immediately after `Start`, then the ticker takes over. Restart windows don't leak up to one full interval of stale expired grants.
- **Panic recovery**: `runOnce` wraps `sweep` in `defer recover()`. A panic logs and the loop continues. A background goroutine that dies silently would be worse than a visible failure.
- **Graceful shutdown**: scheduler shares `main`'s `signal.NotifyContext`. On SIGINT/SIGTERM, `Start` returns. The 10-second shutdown timeout in `main` provides the ceiling; per-step calls are ctx-bound so slow Zitadel requests respect cancellation. A cooperative `ctx.Err()` check at the top of each user's batch prevents leaving half-processed users.
- **Shutdown join** (closes the shutdown race): `Scheduler.Done()` returns a channel that closes *after* `Start` has fully returned. Main blocks on `<-sched.Done()` (bounded by the same shutdown deadline) before calling `db.PG.Close()` / `db.Redis.Close()`. This guarantees an in-flight sweep finishes its DB/Redis/Zitadel work against live clients rather than mid-teardown. A genuinely stuck sweep still cannot hold shutdown open past the 10-second ceiling — in that case main logs and closes anyway, accepting that the doomed sweep's last few calls will surface closed-pool errors and log-and-continue.

## Batch size rationale

Default 500 matches Zitadel's `DefaultSearchLimit` — same order of magnitude, no new operator-facing constant. At semester boundaries (when many grants can expire simultaneously), 500 is enough to make visible progress without saturating Zitadel's rate limits. A sweep that hits the limit logs, and the next tick picks up the next batch. We intentionally do *not* drain-to-empty in one tick: ticks would overlap and re-entrance is not free.

## Testing

15 tests in the new package (`sweep_test.go` + `scheduler_test.go`):

- **Flow tests** for the happy path, partial failures at each step, and the log-and-continue semantics for cache/Zitadel.
- **Multi-user batching** — one cache invalidate per user, not per grant.
- **Idempotency-key regression** — `TestSweep_IntentIdempotencyAcrossReGrants` protects Bug Fix #3.
- **User-scoped delete** — defensive test that cross-user IDs cannot bleed.
- **Zitadel dedup per (project, role)** — even under relaxed future uniqueness.
- **Lifecycle** — ticker fires, ctx cancel exits cleanly, panic recovers, clamp values.

All tests use the save-swap-restore pattern from `cache/compiler_test.go`, running in milliseconds without external infra.
