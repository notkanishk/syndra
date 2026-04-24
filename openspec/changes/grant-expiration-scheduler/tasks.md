## 1. Repository layer

- [x] 1.1 Add `GetExpiredDirectGrants(ctx, limit int)` to `backend/internal/db/repositories.go` — predicate `expires_at IS NOT NULL AND expires_at <= NOW()`, ordered ASC, `LIMIT $1`.
- [x] 1.2 Add `DeleteExpiredDirectGrantsByIDs(ctx, userID, ids)` — user-scoped **guarded** hard delete: `DELETE ... WHERE user_id=$1 AND id = ANY($2::uuid[]) AND expires_at IS NOT NULL AND expires_at <= NOW() RETURNING *`. Returns the rows actually deleted so downstream steps drive off authoritative DB state, not the pre-fetch snapshot. Closes the concurrent-renewal race with `UpsertDirectGrant`'s `ON CONFLICT DO UPDATE`.

## 2. Services layer

- [x] 2.1 Add `EmitProvisioningIntentFromScheduler` to `backend/internal/services/provisioning.go` — scheduler-origin wrapper with grantID-discriminated idempotency key (fixes the `webhookEventID=""` collision bug on re-grants).

## 3. Expiry package

- [x] 3.1 Create `backend/internal/services/expiry/deps.go` with injectable function vars mirroring the `services/deps.go` pattern.
- [x] 3.2 Create `backend/internal/services/expiry/sweep.go` with `sweep` + `processUser` + `groupByUser`. Pipeline order: guarded delete → audit → intent → cache invalidate → Zitadel cascade. Every post-delete step drives off the rows actually returned by `DeleteExpiredDirectGrantsByIDs`, never the pre-fetch snapshot.
- [x] 3.3 Create `backend/internal/services/expiry/scheduler.go` with `Scheduler`, `NewScheduler` (clamps pathological inputs), `Start(ctx)` (runs once at boot then ticker), `runOnce` (panic-recover wrapper), and `Done()` (channel closed only after `Start` fully returns — enables safe shutdown-join in main).

## 4. Main wiring

- [x] 4.1 Move `signal.NotifyContext` above the HTTP server goroutine launch in `backend/cmd/api/main.go` so background workers share the shutdown context.
- [x] 4.2 Parse `EXPIRY_SCHEDULER_ENABLED` / `EXPIRY_SCHEDULER_INTERVAL` / `EXPIRY_SCHEDULER_BATCH_SIZE` in `main.go` with sensible defaults and invalid-value fallbacks.
- [x] 4.3 Launch `go sched.Start(ctx)` when enabled; log the disabled case explicitly.
- [x] 4.4 Block on `<-sched.Done()` (bounded by the shutdown timeout) before calling `db.PG.Close()` / `db.Redis.Close()` so an in-flight sweep cannot race connection teardown.
- [x] 4.5 Add env-var documentation to `.env.example`.

## 5. Tests

- [x] 5.1 `TestSweep_NoExpired_NoOp` — empty fetch results in zero side effects.
- [x] 5.2 `TestSweep_SingleExpired_FullFlow` — delete → audit → intent → cache → zitadel all invoked in order.
- [x] 5.3 `TestSweep_GrantRenewedMidSweep_NotRevoked` — **P1 regression guard**: when the guarded delete returns zero rows (candidate was concurrently renewed via `ON CONFLICT DO UPDATE`), no audit/intent/cache/zitadel work fires.
- [x] 5.4 `TestSweep_PartialRenewal_OnlyActuallyDeletedProgressDownstream` — when only a subset of candidates is still expired at delete time, only that subset flows through the post-delete pipeline.
- [x] 5.5 `TestSweep_MultiUser_OneInvalidateEach` — cache invalidated once per user.
- [x] 5.6 `TestSweep_DeleteFails_NoSideEffects` — DB failure on delete → no downstream work.
- [x] 5.7 `TestSweep_IntentFailsAfterDelete_AuditAndCacheStillLand` — log-and-continue: delete commit is authoritative; cache and cascade still fire.
- [x] 5.8 `TestSweep_ZitadelFails_OtherStepsSucceed` — cascade failure does not roll back earlier steps.
- [x] 5.9 `TestSweep_BatchSizeRespected` — `limit` flows through to DB; 500 grants fully processed.
- [x] 5.10 `TestSweep_IntentIdempotencyAcrossReGrants` — regression guard on Bug Fix #3.
- [x] 5.11 `TestSweep_UserScopedDelete_NoCrossUserBleed` — two users' grants delete with correct user scoping.
- [x] 5.12 `TestSweep_ZitadelDedupPerProjectRole` — duplicate (project, role) tuples produce a single cascade call.
- [x] 5.13 `TestScheduler_CtxCancel_ExitsCleanly` — `Done()` closes within 500ms of ctx cancel.
- [x] 5.14 `TestScheduler_Done_BlocksUntilInFlightSweepCompletes` — **P2 regression guard**: `Done()` does not close while a sweep is still executing; this is the contract main relies on for safe teardown.
- [x] 5.15 `TestScheduler_TickerFires_InvokesSweep` — at least 2 sweeps (initial + tick).
- [x] 5.16 `TestScheduler_PanicRecovered_LoopContinues` — forced panic on first sweep; second sweep still runs.
- [x] 5.17 `TestNewScheduler_ClampsBadInputs` — zero/negative interval defaults to 5m; batch size clamps to [1, 10000].

## 6. OpenSpec deltas

- [x] 6.1 Create `openspec/changes/grant-expiration-scheduler/` with `proposal.md`, `design.md`, `tasks.md`, and `specs/access-governance/spec.md` delta.
- [x] 6.2 Update `openspec/changes/mkauth-core-architecture/specs/access-governance/spec.md`: drop "expiry enforcement deferred P5" from status header; replace the *Grant expiration enforcement* status note with an Integrated description; add two scenarios (retry on transient failure, Zitadel cascade best-effort).
- [x] 6.3 Tick `Grant Expiration Scheduler` item in `openspec/changes/mkauth-core-architecture/ROADMAP.md`.
- [x] 6.4 Update `openspec/changes/mkauth-core-architecture/specs/feature-coverage.md` — bump last-updated date; flip "Temporary roles auto-expire" row to Integrated.
- [x] 6.5 Add this change to `openspec/INDEX.md` Change Log and update `access-governance` capability status.

## 7. Verification

- [x] 7.1 `go vet ./... && go build ./...` in `backend/` — clean.
- [x] 7.2 `go test ./...` in `backend/` — 243 tests pass (17 new in `services/expiry/`, including P1 renewal-race and P2 shutdown-join regression guards).
- [ ] 7.3 Local smoke test: seed grant with `expires_at=NOW()+'10s'`; confirm row removed, audit+intent rows present, log line `[SCHEDULER] Sweep complete duration=...`.
- [ ] 7.4 Run `mcp__codebase-memory-mcp__detect_changes` on the backend scope and re-index so the graph picks up the new package.
