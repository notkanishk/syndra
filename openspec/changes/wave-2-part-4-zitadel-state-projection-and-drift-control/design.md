# Wave 2 · Part 4 — Zitadel State Projection & Drift Control Design

**Status:** Approved for implementation (phased)
**Parent design:** [`docs/superpowers/specs/2026-05-08-may-2026-audit-resolution-design.md`](../../../docs/superpowers/specs/2026-05-08-may-2026-audit-resolution-design.md) §2–§5, §7 Theme 2
**Sister documents:** [Wave 2 · Part 2 design](../wave-2-part-2-backend-coherence/design.md) (clean substrate), [Wave 2 · Part 1 design](../wave-2-part-1-frontend-palette-finalization/design.md) (Material palette)

---

## 1. Aim

Make the two-layer model (parent design §2, §4.1) real: every Zitadel grant ends with a MkAuth intent record. MkAuth-mediated mutations write the ledger before the Zitadel call; direct-Zitadel mutations are detected as drift and triaged. This wave is the only one that introduces the new doctrine (parent design §6.4, §7.1).

The wave is **three independently-shippable sub-phases**, one OpenSpec change, one phased `tasks.md`. The detailed step-by-step plan for **Sub-phase 1** lives at [`docs/superpowers/plans/2026-06-09-wave-2-part-4-phase-1-outbox.md`](../../../docs/superpowers/plans/2026-06-09-wave-2-part-4-phase-1-outbox.md). Sub-phases 2 and 3 are task-level ledgers here until their own writing-plans pass.

---

## 2. What the codebase already gives us (and why this is not a from-scratch build)

The exploration that preceded this design found four existing structures that this wave **evolves** rather than reinvents. Honoring them is the difference between an elegant change and an over-engineered one.

| Existing structure | Where | How this wave reuses it |
|---|---|---|
| **Intent-ledger pattern** | `db/intents.go` — `provisioning_intents` with `idempotency_key`, `ClaimPendingIntents` (`FOR UPDATE SKIP LOCKED`), `status` transitions | The outbox table copies this shape. Same claim-and-process idiom, same idempotency-key dedup. |
| **Reconciliation diff** | `handlers/reconciliation.go` — on-demand `GET /api/v1/reconciliation/grants`, already buckets `OnlyInMkAuth`/`OnlyInZitadel`/`Drift` | Sub-phase 2 lowers its cap (B2), schedules it, and wires its buckets to `drift_items` + outbox re-enqueue. The pure `computeReconciliationDiff` function is kept. |
| **Background scheduler** | `services/expiry/{scheduler,sweep,deps}.go` — ticker + immediate-sweep-on-boot + graceful `Done()` shutdown, injectable deps | The drift sweep and (if ever backgrounded) the drain mirror this package structure exactly. `main.go` wires it beside `sched` with `DRIFT_*` env vars. |
| **Webhook-derived grant index** | `db/webhooks.go` — `zitadel_grants_index` keyed by `grant_id`, `Upsert/Get/DeleteGrantIndex` | The drain's already-exists check and the drift confirmation both read this index instead of hammering `ListUserGrants`. |

`direct_role_grants` already carries `granted_by`, `reason`, `expires_at`. Only `source` + `source_ref` are new. Bundle/rule changes currently project **nothing** to Zitadel (read-side computation only, `services/views.go:collectUserRoles`), so cascade projection (sub-phase 3) is purely additive — it does not have to unwind existing behavior.

---

## 3. Design decisions

### Decision 1 — `applied` is terminal success; the webhook return-trip is for drift, not self-confirmation

**This is the load-bearing decision and a deliberate deviation from parent design §4.3 step 7.**

Parent design §4.3 sketches a loop where MkAuth calls Zitadel, then a `user.grant.added` webhook returns and flips the outbox row `applied → confirmed`. That loop cannot exist in this codebase, because `webhook_translate.go` already contains a **self-mutation guard**:

```go
// webhook_translate.go — events authored by MkAuth's own M2M account are dropped
m2mID := os.Getenv("ZITADEL_M2M_USER_ID")
...
} else if editor := ev.editorID(); editor == m2mID {
    log.Printf("[WEBHOOK] dropped self-mutation event=%s aggregate=%s editor=%s", ...)
    return WebhookPayload{}, true, errSelfMutation
}
```

The drain calls the Management API as that same M2M account. So **every grant MkAuth makes is dropped at the translator boundary** — by design, to prevent orchestration loops (a grant we made re-triggering mapping-rule enforcement). A webhook "return-trip confirmation" would therefore never fire for MkAuth-originated grants, and the pending count would never decrement.

The resolution is simpler than the parent sketch, not more complex:

- **The synchronous `2xx` from the Management API is the confirmation.** MkAuth made the call; MkAuth knows it succeeded. The outbox lifecycle is `pending → in_flight → applied` (success) | `failed` (4xx) | back to `pending` (5xx/timeout, retry). There is **no `confirmed` state**.
- **Pending Propagation count** = rows in `('pending','in_flight')`.
- **The webhook path is reserved for drift** (sub-phase 2). The self-mutation guard is exactly what makes drift detection clean: the only grant events that survive translation are externally-originated ones — which are precisely the drift candidates. We get drift detection "for free" from a guard that already exists.

The outbox `status` CHECK is therefore `('pending','in_flight','applied','failed')`. The parent design's `confirmed` value is dropped. This deviation is logged here, surfaced in the spec delta, and is strictly *less* machinery than the sketch.

> **Consequence for the idempotency key:** because there is no Zitadel round-trip, the `idempotency_key` is **not** stamped into Zitadel call metadata (parent design §4.3 said `metadata={idempotency_key}`). It is purely MkAuth's own outbox dedup token (UNIQUE constraint + already-exists check guard double-drain). We do not depend on Zitadel echoing custom grant metadata — a capability we cannot guarantee. Rejected alternative documented in Decision 6.

### Decision 2 — One migration per sub-phase; full enum installed up front

Three migrations, each self-contained so each sub-phase ships independently:

- `000015` (sub-phase 1): `pending_zitadel_propagations` + `direct_role_grants.source`/`source_ref`.
- `000016` (sub-phase 2): `drift_items` + `external_grant_exclusions`.
- `000017` (sub-phase 3): `mapping_rules.confirmation_mode` + `bundles.confirmation_mode` + `config_settings`.

The `source` CHECK enum in `000015` lists **all five** values (`direct`, `bundle`, `rule`, `external_backfill`, `lifecycle_cascade`) even though sub-phase 1 only writes `direct`. This means sub-phase 3 adds no `ALTER`. Migrations follow the established idiom (`IF EXISTS`/`IF NOT EXISTS` guards, `DO $$` constraint blocks, paired `.up`/`.down`) seen in `000013`/`000014`. Down migrations are real (drop tables / columns; restore prior CHECK where one was relaxed).

```sql
-- 000015_zitadel_propagation_outbox.up.sql (sub-phase 1)
CREATE TABLE IF NOT EXISTS pending_zitadel_propagations (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    op_type         TEXT NOT NULL CHECK (op_type IN ('add', 'revoke', 'replace')),
    user_id         VARCHAR(255) NOT NULL,
    project_id      VARCHAR(255) NOT NULL,
    role_keys       TEXT[] NOT NULL,
    zitadel_grant_id TEXT,                 -- set for revoke/replace; resolved from the grant index
    payload_json    JSONB NOT NULL,
    idempotency_key UUID NOT NULL UNIQUE,
    status          TEXT NOT NULL DEFAULT 'pending'
                        CHECK (status IN ('pending', 'in_flight', 'applied', 'failed')),
    attempts        INT NOT NULL DEFAULT 0,
    last_error      TEXT,
    initiated_by    VARCHAR(255) NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at      TIMESTAMPTZ,
    completed_at    TIMESTAMPTZ
);
CREATE INDEX idx_pending_zitadel_propagations_status
    ON pending_zitadel_propagations(status, created_at);

ALTER TABLE direct_role_grants
    ADD COLUMN IF NOT EXISTS source TEXT NOT NULL DEFAULT 'direct'
        CHECK (source IN ('direct', 'bundle', 'rule', 'external_backfill', 'lifecycle_cascade'));
ALTER TABLE direct_role_grants
    ADD COLUMN IF NOT EXISTS source_ref TEXT;   -- bundle_id / rule_id when source ∈ {bundle, rule}
```

### Decision 3 — Transactional enqueue, then operator-confirmed drain

The enqueue is the only place that must be atomic. A new repository function runs three writes in one `pgx.Tx`:

```go
// db/propagations.go
func EnqueueDirectGrantPropagation(ctx context.Context, p EnqueueParams) (EnqueueResult, error) {
    tx, err := PG.Begin(ctx)
    if err != nil { return EnqueueResult{}, err }
    defer tx.Rollback(ctx)            // no-op after Commit

    var grantID string
    // 1) intent ledger (source='direct', source_ref NULL for point mutations)
    err = tx.QueryRow(ctx, upsertDirectGrantSQL, p.UserID, p.ProjectID, p.RoleKey,
        p.GrantedBy, p.Reason, p.ExpiresAt, p.Source, p.SourceRef).Scan(&grantID)
    if err != nil { return EnqueueResult{}, ... }

    // 2) audit
    _, err = tx.Exec(ctx, insertAuditSQL, p.GrantedBy, p.UserID, "direct_grant.upserted", grantID)
    ...
    // 3) outbox
    key := uuid.New()
    var outboxID string
    err = tx.QueryRow(ctx, insertPropagationSQL, p.OpType, p.UserID, p.ProjectID,
        p.RoleKeys, p.ZitadelGrantID, p.PayloadJSON, key, p.GrantedBy).Scan(&outboxID)
    ...
    if err = tx.Commit(ctx); err != nil { return EnqueueResult{}, err }
    return EnqueueResult{GrantID: grantID, OutboxID: outboxID, IdempotencyKey: key, Status: "pending"}, nil
}
```

The drain is a separate, operator-triggered step (`services/propagation/drain.go`). It is **not** a background ticker in sub-phase 1 — the operator owns the "Resume now" decision per parent design §8. (If a future need arises, it slots into the `expiry.Scheduler` shape with a `PROPAGATION_DRAIN_INTERVAL`; out of scope now.) Drain pseudocode:

```
Drain(ctx):
  release, ok = TryAcquireDrainLock(ctx)                # session-level pg_advisory lock — serialize drains
  if !ok:  return DrainResult{Halted:true, Reason:"drain_in_progress"}   # another drain running; skip
  defer release()
  if !zitadelHealth(ctx).Reachable:  return DrainResult{Halted:true, Reason:"zitadel_offline"}
  rows = ClaimPendingPropagations(ctx, limit)          # FOR UPDATE SKIP LOCKED, status IN (pending,in_flight)→in_flight
  for row in rows (created_at order):
      if alreadyInDesiredState(ctx, row):              # add: desired ⊆ live (index, then live ListUserGrants)
          applyRow(ctx, row); continue                 #   replace: live == desired EXACTLY (extras must be removed)
      err = dispatch(ctx, row)                          #   revoke: none of the roles remain
      switch classify(err):                             # add→AddUserGrant / revoke→RemoveUserGrant / replace→UpdateUserGrant
        nil | AlreadyExists/409:  applyRow(ctx, row)          # Zitadel is the authority on existence — 409 is idempotent success
        4xx (except 429, 408):    MarkFailed(ctx, row.id, err)  # bad request — operator must inspect; do NOT halt others
        5xx | timeout | 429 | 408: Requeue(ctx, row.id, err)   # transient; attempts++; halt if attempts>OUTBOX_MAX_RETRIES
      # a state-write failure (mark/requeue/reconcile) → count `errored`, leave row in_flight, continue (next drain reclaims)
  PruneTerminalPropagations(ctx, OUTBOX_RETENTION_DAYS)  # opportunistic cleanup; see §4.1
  return DrainResult{Applied:n, Failed:m, Requeued:r, Errored:e, ...}

# applyRow reconciles the ledger BEFORE the terminal mark, so a failed reconcile
# leaves the row reclaimable rather than stranding a terminal row beside a stale ledger.
applyRow(ctx, row):
  if row.op_type in (revoke, replace):
      ReconcileLedgerOnApplied(ctx, row)   # revoke: delete named (user,project,role) rows;
                                           # replace: delete source='direct' rows on (user,project) not in the new set
  MarkApplied(ctx, row.id)
```

**Ledger reconciliation on applied revoke/replace (review-hardened).** The transactional enqueue writes/keeps `direct_role_grants` rows for `add`/`replace` but cannot know, at enqueue time, which *old* rows a `revoke` or a narrowing `replace` supersedes — that is only settled once the Zitadel mutation applies. Without a cleanup path a `revoke` would leave the revoked role in the ledger (still counted as an expected grant by the access-decision compiler, and later re-added as `mkauth_only` drift by sub-phase 2), and a `replace` would leave superseded roles behind. `applyRow` closes this: on an applied `revoke`/`replace` — including the already-exists short-circuit — it prunes `direct_role_grants` to match the desired state. The reconcile runs before the terminal `MarkApplied`, so a persistence failure leaves the row `in_flight` and the next drain retries it.

**Crash recovery via in_flight reclaim, made safe by serialized drains.** `ClaimPendingPropagations` claims `status IN ('pending','in_flight')` — the same set the pending worklist/count report. A drain that dies after claiming but before recording a terminal state leaves `in_flight` rows; the next drain re-claims them and the idempotent already-exists check (409→applied) resolves any operation that actually reached Zitadel. Because a row leaves `in_flight` only by a *persisted* terminal/requeue transition, a state-write failure is counted as `errored` (never `applied`/`failed`) and the row stays reclaimable.

Reclaiming `in_flight` rows is only safe because drains are **serialized by a session-level Postgres advisory lock** (`TryAcquireDrainLock`, `pg_try_advisory_lock`). Without it, two concurrent drains (a double-clicked "Resume now", or an inline `?apply=true` overlapping a manual drain) could each claim the *same* freshly-`in_flight` row and issue the same Zitadel mutation — for a `revoke`, one worker succeeds while the other gets a `404` and marks the row `failed`. The lock is held on a dedicated pooled connection for the drain's whole run and released (unlock + connection return) on exit, so the only `in_flight` rows a claiming drain ever sees are those orphaned by a crashed drain whose session (and lock) is gone. A drain that cannot take the lock returns `Halted{Reason:"drain_in_progress"}`. For a single-instance LXC deployment an in-process mutex would suffice, but the advisory lock is topology-independent (safe if the backend is ever scaled out) at negligible extra cost.

`dispatch` reuses the existing `zitadelAddUserGrant`/`zitadelUpdateUserGrant`/`zitadelRemoveUserGrant` injectables unchanged. `revoke`/`replace` read `zitadel_grant_id` from the outbox row (resolved at enqueue from the grant index).

**ACK-classification refinements (review-hardened).** Three cases the naive "4xx terminal / 5xx transient" split gets wrong:

- **`429 Too Many Requests` and `408 Request Timeout`** are technically 4xx but are *transient* — they must `Requeue`, not `MarkFailed`. The Zitadel client already retries 429/503 internally with backoff; a 429 that survives that retry budget and reaches the drain must still land in the transient bucket, never in operator triage.
- **`AlreadyExists`/`409`** on an `add`/`replace` is *idempotent success* (`MarkApplied`), not a failure. This makes the pre-flight already-exists check (below) a pure latency optimization rather than a correctness gate — Zitadel itself is the authority on whether the grant exists, so a stale grant index (in *either* direction) can never cause a wrong outcome.
- The classifier therefore reads the Zitadel client's typed status, not the error string. If the client does not already expose a typed status, add a `zitadel.StatusError{Code int}` and classify on `Code`.

**Already-exists check (latency optimization, not a gate).** Before an `add`/`replace` API call the drain checks the webhook-derived grant index; on an index miss it does **one** live `ListUserGrants` per row (not per role) as the stale-index fallback. A hit short-circuits to `applied`. Because `409→applied` is the safety net, this check only ever *saves* a redundant call — it is never load-bearing for correctness.

### 3.1 Outbox retention

The outbox table holds ephemeral workflow state; the canonical intent lives in `direct_role_grants`. Terminal rows (`applied`/`failed`) past a retention window are prunable. Rather than introduce a sub-phase-1 background scheduler, the drain prunes opportunistically at its tail:

```sql
DELETE FROM pending_zitadel_propagations
WHERE status IN ('applied','failed') AND completed_at < NOW() - ($1 || ' days')::interval
```

`OUTBOX_RETENTION_DAYS` defaults to 30. On a 200-user makerspace the volume is trivial, so drain-tail pruning is sufficient; when sub-phase 2 adds the drift reconciliation scheduler, the prune can move there (one scheduler, two jobs) if a steadier cadence is ever wanted. `failed` rows are kept the full window deliberately — they are the audit trail of mutations that needed operator attention.

### Decision 4 — `/api/v1/zitadel/*` and `/api/v1/users/{id}/grants` converge on one path (B4/D3)

There are two URL families today:

- `/api/v1/users/{id}/grants` (`access.go:handleUpsertUserDirectGrant`) — writes `direct_role_grants`, **no Zitadel call**. The intent ledger without the mirror.
- `/api/v1/zitadel/users/{id}/grants` (`discovery.go:218-282`) — calls Zitadel **directly**, writes no ledger. The mirror without the intent.

Each is half of the contract. This wave merges them: both call `EnqueueDirectGrantPropagation` then (optionally) `Drain`. After this change:

| Route | Before | After |
|---|---|---|
| `POST /api/v1/users/{id}/grants` | ledger only | enqueue (ledger + outbox), `202 {outbox_id, idempotency_key, status:"pending"}` |
| `POST /api/v1/zitadel/users/{id}/grants` | Zitadel only, `{status:"granted"}` | **same handler path**, `202 {outbox_id, idempotency_key, status}` |
| `PUT /api/v1/zitadel/users/{id}/grants/{grantId}` | Zitadel only | enqueue `op_type='replace'` (grantId + roleKeys; grant→user/project resolved via grant index) |
| `DELETE /api/v1/zitadel/users/{id}/grants/{grantId}` | Zitadel only | enqueue `op_type='revoke'` (grantId resolved via grant index) |

The `/zitadel/*` URLs are kept as **backward-compatible aliases** (operator bookmarks, the diagnostics page). One mutation pathway in code, two URL-space entry points. The `update`/`remove` handlers resolve `grantId → (user_id, project_id, role_keys)` through `db.GetGrantIndex` (falling back to a live `ListUserGrants` on index miss) so the canonical `(user, project, role)`-shaped path can enqueue. The frontend caller (`app/zitadel/page.tsx`) is updated for the new response shape — folded in here since it changes anyway in Theme 4's U5 split.

**Access-request approval is the third entry point (review-hardened).** `access.go:handleResolveAccessRequest` (`status=approved`) also creates a direct grant, and originally took the bare `UpsertDirectGrant` shortcut — writing the ledger row but no outbox row. That left the approved grant invisible to the Pending UI, unprojected to Zitadel, and destined to re-surface as `mkauth_only` drift in sub-phase 2 (the ledger says "expected", Zitadel says "absent"). It now flows through the outbox exactly like the operator endpoint (`op_type='add'`, `source='direct'`, `source_ref=<request id>`), then applies inline (the approval *is* the operator's confirmation).

Two follow-on hardenings: (1) **Resolution and enqueue are one conditional transaction** (`db.ApproveRequestAndEnqueue` → `UPDATE access_requests … WHERE id=$1 AND status='pending'` then `enqueueWrites` on the same `pgx.Tx`). Previously the handler resolved the request in one call and enqueued in a separate transaction, so a failed enqueue stranded an approved-but-ungranted request and a retry hit `ALREADY_RESOLVED`; the `WHERE status='pending'` guard (also added to the reject path's `ResolveAccessRequest`) closes the concurrent approve/reject race, returning `ErrRequestNotPending` → `409`. (2) The inline apply drains **only this row** (`DrainOne`), not the global batch — see the inline-apply decision below. The inline drain is best-effort/non-fatal: access is already effective the moment the ledger row commits (the claim compiler reads `direct_role_grants`), so a drain failure just leaves the row pending in the worklist. The request-resolution audit (`access_request.approved`) and the grant audit (`direct_grant.upserted`, written inside the same tx) are both recorded.

### Decision 5 — Drift detection reuses `computeReconciliationDiff`; the sweep gets scheduled and wired to tables (sub-phase 2)

`handlers/reconciliation.go` already contains a pure, tested `computeReconciliationDiff(mkauth, zitadel)` returning `OnlyInMkAuth`/`OnlyInZitadel`/`Drift`. Sub-phase 2 keeps that function and changes three things around it:

1. **Cap 10 000 → 2 000** (B2). One-line change to `reconciliationSafetyCap`; right-sized for a 200-user makerspace (~10× headroom).
2. **`expected` set widened to include rule outputs** so a mapping-rule-derived grant lands as `expected_via_rule`, not `OnlyInZitadel` (parent design §4.5). The expected set becomes `direct_role_grants ∪ bundle_expansions ∪ rule_outputs ∪ external_grant_exclusions` — all computable from existing `services/views.go` helpers.
3. **Outputs wired to durable state.** A new scheduled sweep (`services/drift/{scheduler,sweep,deps}.go`, mirroring `expiry/`) runs every `DRIFT_RECONCILIATION_INTERVAL_HOURS` (default 6) — and on operator demand via a `[Reconcile now]` button (parent design §9 Q2) — calling the diff then: `zitadel_only → drift_items.upsert(detection_source='reconciliation_sweep')`; `mkauth_only → re-enqueue outbox row` (the missed-webhook replay, parent design §4.5).

Real-time drift comes from the webhook: a surviving (non-self) `grant_added`/`grant_removed` that matches no `external_grant_exclusions` and no expected grant inserts `drift_items(detection_source='webhook')`. The **C6** overlay-cache partial-result gap is closed by this same backstop — a grant that slipped past a partial overlay surfaces at the next sweep; documented, not separately engineered.

### Decision 6 — Rejected alternatives (kept here so they are not re-litigated)

- **Round-tripping `idempotency_key` through Zitadel grant metadata** (parent design §4.3 literal reading). Rejected: depends on Zitadel echoing arbitrary custom metadata on `user.grant.*` events, which is unverified and brittle; and is mooted by Decision 1 (synchronous ACK confirms, self-mutation guard drops the echo anyway). Confirmation needs no round-trip.
- **A background drain ticker in sub-phase 1.** Rejected for now: the operator owns the resume decision (parent design §8). The scheduler shape is ready (`expiry.Scheduler`) if backgrounding is ever wanted.
- **A new `intents`-style generic table instead of a dedicated outbox.** Rejected: `provisioning_intents` is LLDAP-provisioning-shaped (`lldap_group`, `target_uid`); grant propagation is `(user, project, role_keys, op_type)`-shaped. Sharing the table would force a lowest-common-denominator schema. Mirror the *pattern*, not the table.
- **Removing the self-mutation guard so the webhook can confirm.** Rejected: the guard prevents orchestration loops and is what makes drift detection clean. It is an asset, not an obstacle (Decision 1).

---

## 4. Data model (full, across sub-phases)

Migration `000015` (sub-phase 1) is shown in Decision 2. Sub-phases 2–3 add:

```sql
-- 000016_drift_queue.up.sql (sub-phase 2)
CREATE TABLE IF NOT EXISTS drift_items (
    id                UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id           VARCHAR(255) NOT NULL,
    project_id        VARCHAR(255) NOT NULL,
    role_keys         TEXT[] NOT NULL,
    zitadel_grant_id  TEXT,
    detected_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    detection_source  TEXT NOT NULL CHECK (detection_source IN ('webhook', 'reconciliation_sweep')),
    drift_type        TEXT NOT NULL CHECK (drift_type IN ('zitadel_only', 'mkauth_only')),
    status            TEXT NOT NULL DEFAULT 'pending_triage'
                          CHECK (status IN ('pending_triage', 'attributed', 'revoked', 'marked_external')),
    resolved_at       TIMESTAMPTZ,
    resolved_by       VARCHAR(255),
    resolution_payload_json JSONB
);
CREATE INDEX idx_drift_items_status ON drift_items(status, detected_at);
-- Dedupe identical pending detections so a noisy sweep does not pile rows.
-- Keyed at ROLE granularity (role_keys included) because the sweep + webhook
-- emit one single-role row per drifting role; dropping role_keys from the key
-- would silently discard the 2nd+ role on a (user, project) pair:
CREATE UNIQUE INDEX idx_drift_items_pending_unique
    ON drift_items(user_id, project_id, drift_type, role_keys)
    WHERE status = 'pending_triage';

CREATE TABLE IF NOT EXISTS external_grant_exclusions (
    user_id     VARCHAR(255) NOT NULL,
    project_id  VARCHAR(255) NOT NULL,
    role_key    VARCHAR(255) NOT NULL,
    marked_by   VARCHAR(255) NOT NULL,
    marked_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    reason      TEXT,
    PRIMARY KEY (user_id, project_id, role_key)
);

-- 000017_confirmation_mode.up.sql (sub-phase 3)
ALTER TABLE mapping_rules ADD COLUMN IF NOT EXISTS confirmation_mode TEXT NOT NULL DEFAULT 'auto'
    CHECK (confirmation_mode IN ('auto', 'manual'));
ALTER TABLE bundles ADD COLUMN IF NOT EXISTS confirmation_mode TEXT NOT NULL DEFAULT 'auto'
    CHECK (confirmation_mode IN ('auto', 'manual'));
CREATE TABLE IF NOT EXISTS config_settings (
    key        TEXT PRIMARY KEY,
    value      TEXT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_by VARCHAR(255)
);
INSERT INTO config_settings(key, value, updated_by)
    VALUES ('global.default_rule_confirmation_mode', 'auto', 'migration')
    ON CONFLICT (key) DO NOTHING;
```

The `idx_drift_items_pending_unique` partial unique index is an addition beyond the parent design — it makes the webhook/sweep `drift_items` insert an idempotent upsert (`ON CONFLICT DO NOTHING`), so a flapping grant or a re-run sweep cannot flood the triage queue with duplicates. Resolved rows leave the partial index, so the same triple can legitimately re-drift later.

---

## 5. Execution order & blast radius

Sub-phases are sequential (each builds on the prior). Within each, tasks land ascending by blast radius so every commit is green.

| Sub-phase | Audit refs | Ships | Rough effort |
|---|---|---|---|
| **1 — Outbox & operator-confirmed flow** | B4, D3 | Operator point mutations buffered + drained end-to-end; Pending UI | ~1.5 weeks |
| **2 — Drift detection & triage** | B2, C6 | Webhook + scheduled drift; `/governance/drift`; three actions | ~1 week |
| **3 — Bundle/rule cascade projection** | (Theme 2 core) | Cascade enqueueing; per-rule confirmation_mode; bulk toggle | ~3–5 days |

Sub-phases 1+2 are a shippable milestone without 3 (parent design §6.5). The detailed bite-sized plan for sub-phase 1 is the companion plan document; 2 and 3 get their own writing-plans pass when 1 lands.

**Sub-phase-3 cascade transaction batching.** `EnqueueDirectGrantPropagation` in sub-phase 1 wraps a single `(user, project, role-set)` mutation in one `pgx.Tx` — small and safe. A sub-phase-3 cascade (a mapping-rule matcher change, or a role added to a large bundle) fans out to one outbox row per affected user. On this single-LXC, ~200-user audience a bundle maxes at ~200 rows, so the fan-out is bounded — the same sizing premise that drops the reconciliation cap to 2 000 (B2). Even so, the cascade enqueue MUST NOT hold one transaction open across an unbounded user set: sub-phase 3 batches the transactional enqueue at **500 rows per transaction**, committing each batch before opening the next, so no single `pgx.Tx` holds locks or buffers proportional to the whole cascade. This is cheap insurance, not a load-bearing constraint at the stated scale; it is captured in `tasks.md` Task 20.

---

## 6. Verification gates

Per-task tests are narrow (transactional-enqueue rollback-on-failure, drain ACK-classification, already-exists short-circuit, response-shape, webhook-drift insertion, UI render). Wave-level gate:

```bash
cd backend && go test ./... && go vet ./...
cd ui && bun run lint && bun run test && bun run build
# migration round-trip on a throwaway DB: up → down 1 → up for 000015 (and 000016/000017 per sub-phase)
gofmt -d <touch set>
openspec validate wave-2-part-4-zitadel-state-projection-and-drift-control --strict   # from repo root
```

Plus codebase-memory refresh (`detect_changes` + reindex affected scope) after each sub-phase.

---

## 7. Open questions resolved (parent design §9)

The parent design left nine writing-plans-phase questions. Resolutions adopted here:

1. **Inline "Apply now" vs queue+click.** Single mutations from inline forms may drain inline: the handler enqueues then triggers a drain when `?apply=true` (and unconditionally for approvals). The inline drain is **targeted** (`DrainOne(outbox_id)`), not the global batch `Drain` — applying one grant must not prematurely project unrelated mutations an operator deliberately left queued (e.g. for a maintenance window). `DrainOne` shares `Drain`'s advisory-lock serialization, reachability pre-flight, and per-row processing (`processRow`), differing only in claiming one row by id instead of the oldest batch. The handler then reports the requesting row's own post-drain status via `GetPropagationStatus(outbox_id)`. **Ordering & lifecycle:** the compiled cache is rebuilt from the committed ledger row *before* the inline drain (so a slow/canceled Zitadel call can't starve it) and on a context *detached* from the request (`context.WithTimeout(context.WithoutCancel(r.Context()), 10s)`, via `rebuildUserCacheDetached`), so a client disconnect after commit also can't leave access ineffective. The grant is durable once the enqueue tx commits, so "effective immediately" means "after commit, regardless of client lifecycle or projection outcome." Cascades always queue (no inline apply). 
2. **Manual reconcile trigger.** Yes — `[Reconcile now]` button on `/governance/drift` calls the sweep on demand.
3. **Outbox per-row detail.** `Resume now` opens a confirmation modal listing each pending row before draining.
4. **Bundle-removal other-source check.** Real-time at enqueue; the small race is reconciliation-resolved.
5. **Drift queue ordering.** `detected_at DESC` default; filters (user/project/source) on `/governance/drift`.
6. **Source-remap validation.** Attribute-to-bundle modal disables bundles whose roles don't include the drift role-key.
7. **Sub-phase split.** One change, phased `tasks.md`, archived together (this document).
8. **U7-style middleware/proxy tests.** Out of scope here (Theme 4 / Wave 3); drift/pending surfaces get their own admin-gating tests.
9. **Drift audio kill-switch.** Option (b): `localStorage` flag + avatar-menu popover toggle, mirroring the existing `theme.tsx` localStorage pattern. No `/settings` page built.

---

## 8. Out of scope / explicit non-goals

Carried from `proposal.md` "Out of scope" verbatim in intent: no auto-resume on Zitadel recovery, no drift auto-resolution, no Zitadel-metadata round-trip, no data-plane/claim changes, no multi-replica outbox coordination, no `services/views.go` semantic changes (only additive projection lookups). If implementation surfaces an unforeseen dependency, halt and re-design rather than work around.
