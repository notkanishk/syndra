# Wave 2 · Part 4 — Sub-phase 2: Drift Detection & Triage Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Detect every out-of-band Zitadel grant (webhook real-time + a scheduled reconciliation sweep) that Syndra has no intent record for, surface it on `/governance/drift` for explicit operator triage (Attribute / Revoke / Mark external), and replay Syndra-expected-but-Zitadel-absent direct grants back through the outbox — closing B2 (right-sized, scheduled reconciliation) and C6 (overlay-cache miss backstop).

**Architecture:** Two new tables (`drift_items` + `external_grant_exclusions`, migration `000016`) with a `db/drift.go` + `db/exclusions.go` repository pair. A new `services/drift` package mirrors `services/expiry` exactly (ticker + immediate-sweep-on-boot + graceful `Done()`), running the reconciliation on `DRIFT_RECONCILIATION_INTERVAL_HOURS` (default 6) and on operator demand. `zitadel_only` grants (in Zitadel, unexplained by Syndra's direct/bundle/rule/exclusion sets) become `drift_items`; `syndra_only` direct grants (Syndra expects, Zitadel lacks) re-enqueue through the sub-phase-1 outbox. Real-time drift comes from the webhook — the self-mutation guard means only *externally*-authored grant events survive translation, so drift detection is free. The triage UI is **red, undismissible, breaks out of layout** — deliberately louder than sub-phase-1's amber, in-layout Pending Propagation surfaces.

**Tech Stack:** Go (`pgx/v5` pool, stdlib `testing`, injectable-deps pattern), PostgreSQL (golang-migrate). Next.js + TypeScript + React Query (Bun), Vitest + Testing-Library. Material (obsidian-clarity) **error/red** tokens for the drift UI.

**Scope note:** This plan covers **Sub-phase 2 only** (tasks.md Tasks 11–18). Sub-phase 1 (outbox + operator-confirmed drain) is IMPLEMENTED and this plan builds on it directly (`db.EnqueueDirectGrantPropagation`, `db.InsertPendingPropagation`, `propagation.DrainOne`, the `direct_role_grants.source`/`source_ref` columns, `db.GrantIndexHasRole`). Sub-phase 3 (bundle/rule cascade projection + `confirmation_mode`) is out of scope and gets its own writing-plans pass.

---

## OpenSpec change scope

- `openspec/changes/wave-2-part-4-zitadel-state-projection-and-drift-control/tasks.md` (Sub-phase 2, Tasks 11–18)
- `.../specs/access-governance/spec.md` — "Out-of-band Zitadel grants MUST be detected as drift and surfaced for operator triage"
- `.../specs/operational-readiness/spec.md` — Pending-vs-Drift urgency tiers + "Reconciliation MUST be right-sized… and schedulable"
- `.../design.md` — Decision 5 (drift detection), §4 (data model), §7 Q2/Q5/Q6/Q9 (resolved open questions)

---

## Global Constraints

Every task's requirements implicitly include these. Values copied verbatim from the design/specs:

- **Go module path:** `syndra`. Imports are `syndra/internal/...`.
- **Migrations dir:** `backend/db/migrations/`; highest is `000015`; **next is `000016`**. Paired `.up.sql`/`.down.sql`, `IF EXISTS`/`IF NOT EXISTS` guards, `DO $$` for constraint blocks, real down migrations.
- **`db` package has NO live-DB test harness.** It is covered only by *migration-coherence guards* (assert the SQL `CHECK` enums match the Go string literals — see `backend/internal/db/propagations_migration_test.go`). Behavioral coverage for repository logic lives in the **injectable service tests** (`services/drift`) and **handler tests**, never live SQL. Follow this exactly for `db/drift.go` + `db/exclusions.go`.
- **No new dependencies.** The repo carries NO uuid module — outbox idempotency keys are minted from `crypto/rand` via `db.newOutboxIdempotencyKey() (string, error)` (canonical v4 UUID string; `db/propagations.go:77`). **NEVER import `github.com/google/uuid`.** In-package `db` helpers call `newOutboxIdempotencyKey()` directly; `services/drift` reuses it via a one-line exported wrapper `db.NewOutboxIdempotencyKey` added to `db/propagations.go` in Task 14 (a wrapper, NOT a rename, so the existing `EnqueueDirectGrantPropagation` call site is untouched). Env parsing: **stdlib inline** (`strconv.Atoi` / `time.ParseDuration`) — there is NO `getEnvInt` helper; `cmd/api/main.go` reads env vars with per-var functions (`schedulerInterval()` etc.).
- **`applied`/`drift` doctrine (design Decision 1):** the self-mutation guard (`webhook_translate.go`: `editor == ZITADEL_M2M_USER_ID` → dropped) means Syndra's own grant mutations never return over the webhook. Surviving grant events are therefore *externally-originated* — the drift candidates.
- **Reconciliation safety cap = 2 000** (down from 10 000). Right-sized for the single-LXC ~200-user makerspace.
- **No drift auto-resolution.** Every `drift_items` row requires explicit operator action. No age-based auto-mark.
- **Drift UI is red + undismissible + breaks out of layout**; Pending UI (sub-phase 1) is amber + dismissible + in-layout. Drift MUST be louder at every dimension. Use Material `error`/`on-error`/`error-container` tokens. Respect `prefers-reduced-motion`. The optional chime is gated by a `localStorage` toggle (default on), mirroring `ui/src/lib/theme.tsx`.

---

## Reference: current code this plan touches

| Symbol | File:line | Current shape |
|---|---|---|
| `reconciliationSafetyCap` | `backend/internal/handlers/reconciliation.go:29` | `var reconciliationSafetyCap = 10_000` (drop to 2_000) |
| `computeReconciliationDiff` | `reconciliation.go:142-236` | pure; `(syndra []models.DirectGrant, zitadel []zitadel.UserGrant) ReconciliationDiff` |
| `ReconciliationDiff` / `ReconciliationGrant` / `ReconciliationDrift` | `reconciliation.go:34-68` | `OnlyInSyndra`/`OnlyInZitadel`/`Drift`/`Truncated` |
| `fetchAllZitadelGrants` | `reconciliation.go:113-138` | paginates `zitadelListAllGrants`, caps at `reconciliationSafetyCap` |
| `svcAllDirectGrants` / `zitadelListAllGrants` | reconciliation handler injectables | `(ctx) ([]models.DirectGrant, error)` / `(ctx, zitadel.SearchParams) (*SearchResult[UserGrant], error)` |
| `collectUserRoles` | `backend/internal/services/views.go:730-799` | `(ctx, userID) (map[roleKey]*EffectiveRole, []Bundle, error)`; unions direct+bundle+rule |
| `Governance` / `governanceFromSnapshot` | `views.go:463-524` | returns `models.GovernanceSummary` (already has `PendingPropagation`) |
| `GovernanceSummary` / `PendingPropagationSummary` | `backend/internal/models/models.go:221-234` | add `Drift DriftSummary` |
| `svcGetActiveMappingRules` | views injectable | `(ctx) ([]models.MappingRule, error)`; `MappingRule{SourceProject,SourceRole,TargetProject,TargetRole}` |
| self-mutation guard | `webhook_translate.go:70-76` | drops `editor == m2mID` |
| `WebhookPayload` | `webhook.go:13-21` | `{EventType,UserID,SourceProject,RoleKey,RoleKeys,ProjectIDs,GrantID}` |
| `processGrantAdded` | `webhook.go:227-255` | grant_added/changed downstream effects; hook point before `return nil` |
| `GrantIndexHasRole` / `GetGrantIndex` | `backend/internal/db/webhooks.go:215-230,195-213` | phase-1 pre-flight helpers |
| `EnqueueDirectGrantPropagation` / `EnqueueParams` | `backend/internal/db/propagation_enqueue.go` | phase-1; `op_type='revoke'` skips ledger upsert |
| `InsertPendingPropagation` | `backend/internal/db/propagations.go` | phase-1; `(ctx, opType,user,project,roleKeys,grantID,payload,idemKey,initiatedBy) (string,error)` |
| `DrainOne` | `backend/internal/services/propagation/drain.go` | phase-1; targeted single-row drain by outbox id |
| `expiry.Scheduler` | `backend/internal/services/expiry/{scheduler,sweep,deps}.go` | the drift scheduler mirrors this exactly |
| expiry scheduler wiring | `backend/cmd/api/main.go:97-109,133-139,149-186` | `NewScheduler`/`go .Start(ctx)`/join `.Done()`; env helpers |
| `usePendingPropagations` / `useDrainPropagations` | `ui/src/lib/queries/usePropagation.ts` | the drift hooks mirror these |
| `GovernanceSummary` (TS) | `ui/src/lib/queries/useGovernance.ts` | add `drift` block |
| `SidebarNav` | `ui/src/components/SidebarNav.tsx` | `NavSection[]`; reads `pending_propagation.count`; add top-level `⚠ Drift` |
| `PendingCallout` / `PendingPropagationsClient` | `ui/src/components/propagation/*` | the drift callout/client mirror these (louder) |
| `AdminDashboard` | `ui/src/components/dashboard/AdminDashboard.tsx:76-108` | renders `PendingCallout`; add undismissible `DriftCallout` |
| `ThemeProvider` | `ui/src/lib/theme.tsx` | localStorage pattern for the chime toggle |
| error tokens | `ui/src/app/globals.css` | `--color-error`/`--color-on-error`/`--color-error-container` |

---

## Task 11 — Migration `000016`: drift queue + external-grant exclusions

**Files:**
- Create: `backend/db/migrations/000016_drift_queue.up.sql`
- Create: `backend/db/migrations/000016_drift_queue.down.sql`

- [ ] **Step 11.1: Write the up migration**

```sql
-- 000016_drift_queue.up.sql
-- Wave 2 · Part 4 sub-phase 2 (B2/C6): the drift triage queue for out-of-band
-- Zitadel grants Syndra has no intent record for, plus the operator's
-- "this is legitimately external, stop flagging it" exclusion list.
-- No drift item resolves automatically (design §8): every row needs explicit
-- Attribute / Revoke / Mark-external triage.

CREATE TABLE IF NOT EXISTS drift_items (
    id                      UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id                 VARCHAR(255) NOT NULL,
    project_id              VARCHAR(255) NOT NULL,
    role_keys               TEXT[] NOT NULL,
    zitadel_grant_id        TEXT,
    detected_at             TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    detection_source        TEXT NOT NULL CHECK (detection_source IN ('webhook', 'reconciliation_sweep')),
    drift_type              TEXT NOT NULL CHECK (drift_type IN ('zitadel_only', 'syndra_only')),
    status                  TEXT NOT NULL DEFAULT 'pending_triage'
                                CHECK (status IN ('pending_triage', 'attributed', 'revoked', 'marked_external')),
    resolved_at             TIMESTAMPTZ,
    resolved_by             VARCHAR(255),
    resolution_payload_json JSONB
);

CREATE INDEX IF NOT EXISTS idx_drift_items_status ON drift_items(status, detected_at);

-- Dedupe identical PENDING detections so a noisy sweep / flapping grant cannot
-- flood the triage queue. Keyed at ROLE granularity (role_keys included) because
-- the sweep + webhook emit one single-role row per drifting role: dropping
-- role_keys from the key would silently discard the 2nd+ role on a (user,project)
-- pair. Resolved rows leave the partial index, so the same triple can re-drift.
CREATE UNIQUE INDEX IF NOT EXISTS idx_drift_items_pending_unique
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
```

- [ ] **Step 11.2: Write the down migration**

```sql
-- 000016_drift_queue.down.sql
-- Reverses 000016. drift_items is ephemeral triage state and exclusions are an
-- operator convenience list; both are re-derivable by the next reconciliation
-- sweep, so dropping them loses no canonical record.

DROP TABLE IF EXISTS external_grant_exclusions;
DROP INDEX IF EXISTS idx_drift_items_pending_unique;
DROP INDEX IF EXISTS idx_drift_items_status;
DROP TABLE IF EXISTS drift_items;
```

- [ ] **Step 11.3: Verify migration applies (round-trip)**

Run (requires a throwaway Postgres via `$DB_DSN`; if none is available, statically validate file naming + up/down symmetry and note it, exactly as sub-phase 1 Task 10.3 did — this environment has no throwaway PG):
```bash
cd backend
migrate -path db/migrations -database "$DB_DSN" up
migrate -path db/migrations -database "$DB_DSN" down 1
migrate -path db/migrations -database "$DB_DSN" up
```
Expected: no error; `\d drift_items` and `\d external_grant_exclusions` present after the final `up`.

- [ ] **Step 11.4: Commit**

```bash
git checkout -b wave-2-part-4-drift
git add backend/db/migrations/000016_drift_queue.*.sql
git commit -m "feat(db): drift_items triage queue + external_grant_exclusions (000016)"
```

---

## Task 12 — Models + `db/drift.go` + `db/exclusions.go` repositories

**Files:**
- Modify: `backend/internal/models/models.go` (add `DriftItem`, `DriftSummary`, `ExternalGrantExclusion`; add `Drift` to `GovernanceSummary`)
- Create: `backend/internal/db/drift.go`
- Create: `backend/internal/db/exclusions.go`
- Create: `backend/internal/db/drift_migration_test.go` (migration-coherence guard — the `db` package has no live-DB harness)

- [ ] **Step 12.1: Add the models**

In `models.go`, near `PendingPropagation` / `GovernanceSummary`:
```go
// DriftItem is one out-of-band grant discrepancy awaiting operator triage.
// zitadel_only: exists in Zitadel, no Syndra intent. syndra_only: Syndra
// expects it (direct grant), Zitadel lacks it. No item resolves automatically.
type DriftItem struct {
	ID                string     `json:"id"`
	UserID            string     `json:"user_id"`
	ProjectID         string     `json:"project_id"`
	RoleKeys          []string   `json:"role_keys"`
	ZitadelGrantID    string     `json:"zitadel_grant_id,omitempty"`
	DetectedAt        time.Time  `json:"detected_at"`
	DetectionSource   string     `json:"detection_source"` // webhook | reconciliation_sweep
	DriftType         string     `json:"drift_type"`       // zitadel_only | syndra_only
	Status            string     `json:"status"`           // pending_triage | attributed | revoked | marked_external
	ResolvedAt        *time.Time `json:"resolved_at,omitempty"`
	ResolvedBy        string     `json:"resolved_by,omitempty"`
	ResolutionPayload string     `json:"resolution_payload_json,omitempty"`
}

// DriftSummary feeds the red dashboard callout + sidebar dot: pending count
// plus a top-N preview for the "top-3 + Triage all →" callout.
type DriftSummary struct {
	Count int         `json:"count"`
	Top   []DriftItem `json:"top,omitempty"`
}

// ExternalGrantExclusion is an operator "this is legitimately external" marker
// keyed by (user, project, role) — future detections for the triple are filtered.
type ExternalGrantExclusion struct {
	UserID    string    `json:"user_id"`
	ProjectID string    `json:"project_id"`
	RoleKey   string    `json:"role_key"`
	MarkedBy  string    `json:"marked_by"`
	MarkedAt  time.Time `json:"marked_at"`
	Reason    string    `json:"reason,omitempty"`
}
```

Extend `GovernanceSummary` (`models.go:221-226`) with a trailing field:
```go
	Drift DriftSummary `json:"drift"`
```

- [ ] **Step 12.2: Implement `db/drift.go`**

Follows the `db` idiom exactly (package-level `PG *pgxpool.Pool`, `ctx` first, `fmt.Errorf("…: %w", err)`). Mirrors `propagations.go`.
```go
// drift.go
package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"syndra/internal/models"
)

// DriftFilter narrows a drift listing. Empty fields are ignored.
type DriftFilter struct {
	UserID          string
	ProjectID       string
	DetectionSource string // webhook | reconciliation_sweep
	Status          string // defaults to pending_triage when empty (see GetDriftItems)
}

// UpsertDriftItem inserts a pending drift row, deduped by the partial-unique
// index (user_id, project_id, drift_type, role_keys) WHERE status='pending_triage'.
// Callers pass ONE role per call (single-element role_keys); the role is part of
// the dedup key so a second drifting role on the same pair is NOT swallowed.
// Returns (id, inserted). On an existing identical pending row it returns
// ("", false) — a re-detection of the same drift is a no-op, not a second entry.
func UpsertDriftItem(ctx context.Context, userID, projectID string, roleKeys []string,
	zitadelGrantID, detectionSource, driftType string) (string, bool, error) {
	const q = `
		INSERT INTO drift_items (user_id, project_id, role_keys, zitadel_grant_id, detection_source, drift_type)
		VALUES ($1,$2,$3,NULLIF($4,''),$5,$6)
		ON CONFLICT (user_id, project_id, drift_type, role_keys) WHERE (status = 'pending_triage')
		DO NOTHING
		RETURNING id`
	var id string
	err := PG.QueryRow(ctx, q, userID, projectID, roleKeys, zitadelGrantID, detectionSource, driftType).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil // an identical pending row already exists
	}
	if err != nil {
		return "", false, fmt.Errorf("upsert drift item: %w", err)
	}
	return id, true, nil
}

// GetDriftItems lists drift rows by filter, newest first (design §7 Q5:
// detected_at DESC default). An empty Status filter defaults to pending_triage.
func GetDriftItems(ctx context.Context, f DriftFilter) ([]models.DriftItem, error) {
	status := f.Status
	if status == "" {
		status = "pending_triage"
	}
	const q = `
		SELECT id, user_id, project_id, role_keys, COALESCE(zitadel_grant_id,''),
		       detected_at, detection_source, drift_type, status,
		       resolved_at, COALESCE(resolved_by,''), COALESCE(resolution_payload_json::text,'')
		FROM drift_items
		WHERE status = $1
		  AND ($2 = '' OR user_id = $2)
		  AND ($3 = '' OR project_id = $3)
		  AND ($4 = '' OR detection_source = $4)
		ORDER BY detected_at DESC`
	rows, err := PG.Query(ctx, q, status, f.UserID, f.ProjectID, f.DetectionSource)
	if err != nil {
		return nil, fmt.Errorf("get drift items: %w", err)
	}
	defer rows.Close()
	return scanDriftItems(rows)
}

// GetDriftItem fetches one row by id (any status). ErrDriftNotFound on miss.
func GetDriftItem(ctx context.Context, id string) (models.DriftItem, error) {
	const q = `
		SELECT id, user_id, project_id, role_keys, COALESCE(zitadel_grant_id,''),
		       detected_at, detection_source, drift_type, status,
		       resolved_at, COALESCE(resolved_by,''), COALESCE(resolution_payload_json::text,'')
		FROM drift_items WHERE id = $1`
	rows, err := PG.Query(ctx, q, id)
	if err != nil {
		return models.DriftItem{}, fmt.Errorf("get drift item: %w", err)
	}
	defer rows.Close()
	items, err := scanDriftItems(rows)
	if err != nil {
		return models.DriftItem{}, err
	}
	if len(items) == 0 {
		return models.DriftItem{}, ErrDriftNotFound
	}
	return items[0], nil
}

// ResolveDriftItem transitions a pending row to a terminal status
// (attributed | revoked | marked_external), guarded on status='pending_triage'
// so a concurrent double-triage loses the race cleanly. Returns
// ErrDriftNotPending when the row is already resolved.
func ResolveDriftItem(ctx context.Context, id, status, resolvedBy, payloadJSON string) error {
	const q = `
		UPDATE drift_items
		SET status = $2, resolved_at = NOW(), resolved_by = $3,
		    resolution_payload_json = NULLIF($4,'')::jsonb
		WHERE id = $1 AND status = 'pending_triage'`
	tag, err := PG.Exec(ctx, q, id, status, resolvedBy, payloadJSON)
	if err != nil {
		return fmt.Errorf("resolve drift item: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrDriftNotPending
	}
	return nil
}

// CountPendingDrift is the number badge for the sidebar dot + dashboard callout.
func CountPendingDrift(ctx context.Context) (int, error) {
	var n int
	if err := PG.QueryRow(ctx, `SELECT COUNT(*) FROM drift_items WHERE status='pending_triage'`).Scan(&n); err != nil {
		return 0, fmt.Errorf("count pending drift: %w", err)
	}
	return n, nil
}

// HasPendingDrift reports whether a pending drift row already exists for the
// exact (user, project, role, drift_type). The sweep uses this to avoid
// re-enqueueing an syndra_only grant the operator is already triaging — scoped
// to the specific role so an unrelated missing role on the same pair is NOT
// suppressed.
func HasPendingDrift(ctx context.Context, userID, projectID, roleKey, driftType string) (bool, error) {
	const q = `SELECT EXISTS(
		SELECT 1 FROM drift_items
		WHERE user_id=$1 AND project_id=$2 AND drift_type=$4
		  AND $3 = ANY(role_keys) AND status='pending_triage')`
	var exists bool
	if err := PG.QueryRow(ctx, q, userID, projectID, roleKey, driftType).Scan(&exists); err != nil {
		return false, fmt.Errorf("has pending drift: %w", err)
	}
	return exists, nil
}

func scanDriftItems(rows pgx.Rows) ([]models.DriftItem, error) {
	var out []models.DriftItem
	for rows.Next() {
		var d models.DriftItem
		if err := rows.Scan(&d.ID, &d.UserID, &d.ProjectID, &d.RoleKeys, &d.ZitadelGrantID,
			&d.DetectedAt, &d.DetectionSource, &d.DriftType, &d.Status,
			&d.ResolvedAt, &d.ResolvedBy, &d.ResolutionPayload); err != nil {
			return nil, fmt.Errorf("scan drift item: %w", err)
		}
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

var (
	ErrDriftNotFound   = errors.New("drift item not found")
	ErrDriftNotPending = errors.New("drift item not pending")
)
```

- [ ] **Step 12.3: Implement the transactional triage helpers (in `db/drift.go`)**

A drift item MUST NOT leave the triage queue without its durable side effect, and two operators MUST NOT both act on one. These helpers do both in **one transaction**: a guarded `UPDATE … WHERE status='pending_triage'` (→ `ErrDriftNotPending` on a lost race, rolling the whole tx back) plus the outbox/exclusion writes — committed together or not at all. They reuse `enqueueWrites`, the tx-composable seam the access-request approval path uses (sub-phase-1 Task 10d).
```go
// (append to db/drift.go)

// AttributeDriftAndEnqueue claims a pending drift (→attributed) and writes the
// attribution's ledger+audit+outbox rows in ONE tx. p.OpType must be "add" (the
// grant already exists in Zitadel; the outbox row self-resolves during drain via
// the grant-index short-circuit / 409). p.PayloadJSON doubles as the resolution
// payload. ErrDriftNotPending on a lost race (whole tx rolled back — no outbox row).
func AttributeDriftAndEnqueue(ctx context.Context, driftID string, p EnqueueParams) error {
	tx, err := PG.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin attribute tx: %w", err)
	}
	defer tx.Rollback(ctx) // no-op after Commit
	if err := claimDriftTx(ctx, tx, driftID, "attributed", p.GrantedBy, p.PayloadJSON); err != nil {
		return err
	}
	key, err := newOutboxIdempotencyKey()
	if err != nil {
		return err
	}
	if _, err := enqueueWrites(ctx, tx, p, key); err != nil {
		return fmt.Errorf("attribute enqueue writes: %w", err)
	}
	return tx.Commit(ctx)
}

// RevokeDriftAndEnqueue claims a pending drift (→revoked) and enqueues a revoke
// outbox row in ONE tx (p.OpType must be "revoke"; enqueueWrites skips the ledger
// upsert for revoke). Returns the outbox id so the handler can drain it
// best-effort AFTER commit. ErrDriftNotPending on a lost race.
func RevokeDriftAndEnqueue(ctx context.Context, driftID string, p EnqueueParams) (string, error) {
	tx, err := PG.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("begin revoke tx: %w", err)
	}
	defer tx.Rollback(ctx)
	if err := claimDriftTx(ctx, tx, driftID, "revoked", p.GrantedBy, "{}"); err != nil {
		return "", err
	}
	key, err := newOutboxIdempotencyKey()
	if err != nil {
		return "", err
	}
	outboxID, err := enqueueWrites(ctx, tx, p, key)
	if err != nil {
		return "", fmt.Errorf("revoke enqueue writes: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("commit revoke tx: %w", err)
	}
	return outboxID, nil
}

// MarkDriftExternalTx claims a pending drift (→marked_external) and inserts the
// exclusion rows in ONE tx. ErrDriftNotPending on a lost race (no exclusion written).
func MarkDriftExternalTx(ctx context.Context, driftID, userID, projectID string,
	roleKeys []string, markedBy, reason, payloadJSON string) error {
	tx, err := PG.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin mark-external tx: %w", err)
	}
	defer tx.Rollback(ctx)
	if err := claimDriftTx(ctx, tx, driftID, "marked_external", markedBy, payloadJSON); err != nil {
		return err
	}
	const ins = `INSERT INTO external_grant_exclusions (user_id, project_id, role_key, marked_by, reason)
		VALUES ($1,$2,$3,$4,NULLIF($5,'')) ON CONFLICT (user_id, project_id, role_key) DO NOTHING`
	for _, rk := range roleKeys {
		if _, err := tx.Exec(ctx, ins, userID, projectID, rk, markedBy, reason); err != nil {
			return fmt.Errorf("insert exclusion in tx: %w", err)
		}
	}
	return tx.Commit(ctx)
}

// claimDriftTx is the shared guarded transition: flips a pending drift row to a
// terminal status inside the caller's tx, or returns ErrDriftNotPending (which
// makes the caller's deferred Rollback discard everything) when it is no longer
// pending. This is what makes the whole action atomic AND race-safe.
func claimDriftTx(ctx context.Context, tx pgx.Tx, driftID, status, resolvedBy, payloadJSON string) error {
	tag, err := tx.Exec(ctx, `UPDATE drift_items
		SET status=$2, resolved_at=NOW(), resolved_by=$3, resolution_payload_json=NULLIF($4,'')::jsonb
		WHERE id=$1 AND status='pending_triage'`, driftID, status, resolvedBy, payloadJSON)
	if err != nil {
		return fmt.Errorf("claim drift %s: %w", driftID, err)
	}
	if tag.RowsAffected() == 0 {
		return ErrDriftNotPending
	}
	return nil
}
```
> **Implementer notes:**
> - `enqueueWrites(ctx, tx, p, key) (string, error)` returns the outbox id as a **bare string** (confirmed `db/propagation_enqueue.go:76`) and skips the ledger upsert when `p.OpType=="revoke"`. The helpers above match that.
> - Idempotency keys come from `newOutboxIdempotencyKey()` (crypto/rand, same `db` package) — **NOT** `github.com/google/uuid` (no such module in this repo). `db/drift.go` already imports `github.com/jackc/pgx/v5` for `pgx.Rows`/`pgx.Tx`; no other new import.
> - These live in `db/drift.go` beside the plain repository fns. `ResolveDriftItem`/`InsertExclusion` (Steps 12.2/12.4) stay as the non-transactional API (used by nothing in the triage path now, but harmless single-purpose helpers — keep only if another caller wants them, else drop to avoid dead code; `go vet`/unused-lint will tell you). The sweep + webhook still use `UpsertDriftItem` (insert-only, no claim).

- [ ] **Step 12.4: Implement `db/exclusions.go`**

```go
// exclusions.go
package db

import (
	"context"
	"fmt"

	"syndra/internal/models"
)

// InsertExclusion records an operator "legitimately external" marker. Idempotent
// on (user, project, role) — re-marking the same triple is a no-op.
func InsertExclusion(ctx context.Context, userID, projectID, roleKey, markedBy, reason string) error {
	const q = `
		INSERT INTO external_grant_exclusions (user_id, project_id, role_key, marked_by, reason)
		VALUES ($1,$2,$3,$4,NULLIF($5,''))
		ON CONFLICT (user_id, project_id, role_key) DO NOTHING`
	if _, err := PG.Exec(ctx, q, userID, projectID, roleKey, markedBy, reason); err != nil {
		return fmt.Errorf("insert exclusion: %w", err)
	}
	return nil
}

// GetExclusions returns all exclusion triples so the reconciliation sweep and
// the webhook can filter known-external grants out of drift detection.
func GetExclusions(ctx context.Context) ([]models.ExternalGrantExclusion, error) {
	const q = `SELECT user_id, project_id, role_key, marked_by, marked_at, COALESCE(reason,'')
		FROM external_grant_exclusions`
	rows, err := PG.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("get exclusions: %w", err)
	}
	defer rows.Close()
	var out []models.ExternalGrantExclusion
	for rows.Next() {
		var e models.ExternalGrantExclusion
		if err := rows.Scan(&e.UserID, &e.ProjectID, &e.RoleKey, &e.MarkedBy, &e.MarkedAt, &e.Reason); err != nil {
			return nil, fmt.Errorf("scan exclusion: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
```

- [ ] **Step 12.5: Write the migration-coherence guard (the ONLY db-package test)**

The `db` package has no live-DB harness, so — exactly like `propagations_migration_test.go` — assert the migration's `CHECK` enums match the string literals the Go code writes. This catches a divergence between the SQL constraint and the values the repositories/services pass.
```go
// drift_migration_test.go
package db

import (
	"os"
	"strings"
	"testing"
)

// The db package is tested only via migration-coherence guards (no live DB).
// This asserts every drift_type / detection_source / status literal the Go
// layer writes is permitted by the 000016 CHECK constraints, and vice-versa.
func TestDriftMigrationEnumsMatchCode(t *testing.T) {
	up, err := os.ReadFile("../../db/migrations/000016_drift_queue.up.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	sql := string(up)

	// Values the Go code actually writes (UpsertDriftItem, ResolveDriftItem, sweep, webhook).
	for _, v := range []string{"'webhook'", "'reconciliation_sweep'"} {
		if !strings.Contains(sql, v) {
			t.Errorf("detection_source %s written by code but missing from 000016 CHECK", v)
		}
	}
	for _, v := range []string{"'zitadel_only'", "'syndra_only'"} {
		if !strings.Contains(sql, v) {
			t.Errorf("drift_type %s written by code but missing from 000016 CHECK", v)
		}
	}
	for _, v := range []string{"'pending_triage'", "'attributed'", "'revoked'", "'marked_external'"} {
		if !strings.Contains(sql, v) {
			t.Errorf("status %s written by code but missing from 000016 CHECK", v)
		}
	}
	// The partial-unique dedupe index must exist or UpsertDriftItem's ON CONFLICT breaks.
	if !strings.Contains(sql, "idx_drift_items_pending_unique") {
		t.Error("partial-unique dedupe index missing; UpsertDriftItem ON CONFLICT would fail")
	}
}
```

- [ ] **Step 12.6: Build + run the guard**

Run: `cd backend && go build ./internal/db/ ./internal/models/ && go test ./internal/db/ -run TestDriftMigrationEnumsMatchCode -v`
Expected: build clean; test PASS.

- [ ] **Step 12.7: Commit**

```bash
git add backend/internal/models/models.go backend/internal/db/drift.go backend/internal/db/exclusions.go backend/internal/db/drift_migration_test.go
git commit -m "feat(db): drift_items + exclusions repositories, models, migration-coherence guard"
```

---

## Task 13 — Reconciliation: cap 2 000 + rule/exclusion-aware expected set (B2)

**Files:**
- Modify: `backend/internal/handlers/reconciliation.go` (cap; filter `OnlyInZitadel`)
- Create: `backend/internal/services/expected.go` (pure classification helpers, package `services`)
- Create: `backend/internal/services/expected_test.go`
- Modify: `backend/internal/handlers/reconciliation.go` deps + `reconciliation_test.go`

**Interfaces:**
- Produces (consumed by Task 14's sweep and Task 15's webhook): `services.IsExcluded(exclusions []models.ExternalGrantExclusion, userID, projectID, roleKey string) bool`; `services.ExpectedViaRule(holder map[services.HolderKey]bool, rules []models.MappingRule, userID, projectID, roleKey string) bool`; `services.HolderKey` struct + `services.BuildHolderSet(direct []models.DirectGrant, zit []zitadel.UserGrant) map[services.HolderKey]bool`.

- [ ] **Step 13.1: Write the failing pure-classification test**

```go
// expected_test.go
package services

import (
	"testing"

	"syndra/internal/models"
	"syndra/internal/zitadel"
)

func TestExpectedViaRule_UserHoldingSourceMakesTargetExpected(t *testing.T) {
	// Rule: holding p1:member derives p2:contributor.
	rules := []models.MappingRule{{SourceProject: "p1", SourceRole: "member", TargetProject: "p2", TargetRole: "contributor"}}
	// The user holds the source (as a Zitadel grant); the derived target is therefore expected, not drift.
	holder := BuildHolderSet(
		nil,
		[]zitadel.UserGrant{{UserID: "u1", ProjectID: "p1", RoleKeys: []string{"member"}}},
	)
	if !ExpectedViaRule(holder, rules, "u1", "p2", "contributor") {
		t.Fatal("target of a fired rule must be expected_via_rule")
	}
	// A user NOT holding the source must not have the target explained by the rule.
	if ExpectedViaRule(holder, rules, "u2", "p2", "contributor") {
		t.Fatal("rule must not explain a target for a user lacking the source")
	}
}

func TestIsExcluded_MatchesTriple(t *testing.T) {
	ex := []models.ExternalGrantExclusion{{UserID: "u1", ProjectID: "p1", RoleKey: "viewer"}}
	if !IsExcluded(ex, "u1", "p1", "viewer") {
		t.Fatal("marked-external triple must be excluded")
	}
	if IsExcluded(ex, "u1", "p1", "editor") {
		t.Fatal("a different role must not be excluded")
	}
}
```

- [ ] **Step 13.2: Run — expect compile failure**

Run: `cd backend && go test ./internal/services/ -run 'TestExpectedViaRule|TestIsExcluded' -v`
Expected: FAIL — `undefined: BuildHolderSet` / `ExpectedViaRule` / `IsExcluded`.

- [ ] **Step 13.3: Implement `services/expected.go`**

```go
// expected.go
package services

import (
	"syndra/internal/models"
	"syndra/internal/zitadel"
)

// HolderKey is one (user, project, role) tuple a user actually holds — union of
// Syndra direct grants and live Zitadel grants. It is the input to rule
// derivation: a mapping rule's target is "expected" only for users who hold the
// rule's source.
type HolderKey struct {
	UserID    string
	ProjectID string
	RoleKey   string
}

// BuildHolderSet unions Syndra direct grants and Zitadel grants into the set of
// tuples each user currently holds.
func BuildHolderSet(direct []models.DirectGrant, zit []zitadel.UserGrant) map[HolderKey]bool {
	h := make(map[HolderKey]bool)
	for _, g := range direct {
		h[HolderKey{g.UserID, g.ProjectID, g.RoleKey}] = true
	}
	for _, g := range zit {
		for _, rk := range g.RoleKeys {
			h[HolderKey{g.UserID, g.ProjectID, rk}] = true
		}
	}
	return h
}

// ExpectedViaRule reports whether (userID, projectID, roleKey) is the target of
// an active mapping rule the user qualifies for (holds the source). Single-hop:
// this covers the mandated "rule-derived grant is expected_via_rule" scenario.
// ponytail: single-hop only — a multi-hop rule chain (A→B→C) where the user
// holds only A would not classify C. Rules in this codebase are single-hop
// today; widen to a fixpoint (as collectUserRoles does) if chains appear.
func ExpectedViaRule(holder map[HolderKey]bool, rules []models.MappingRule, userID, projectID, roleKey string) bool {
	for _, r := range rules {
		if r.TargetProject == projectID && r.TargetRole == roleKey &&
			holder[HolderKey{userID, r.SourceProject, r.SourceRole}] {
			return true
		}
	}
	return false
}

// IsExcluded reports whether the triple was marked legitimately-external.
func IsExcluded(exclusions []models.ExternalGrantExclusion, userID, projectID, roleKey string) bool {
	for _, e := range exclusions {
		if e.UserID == userID && e.ProjectID == projectID && e.RoleKey == roleKey {
			return true
		}
	}
	return false
}
```

- [ ] **Step 13.4: Run — expect PASS**

Run: `cd backend && go test ./internal/services/ -run 'TestExpectedViaRule|TestIsExcluded' -v`
Expected: PASS.

- [ ] **Step 13.5: Drop the cap + filter the endpoint's `OnlyInZitadel`**

In `reconciliation.go`:
```go
var reconciliationSafetyCap = 2_000 // B2: right-sized for the single-LXC ~200-user makerspace (~10× headroom)
```

Add reconciliation deps (beside `svcAllDirectGrants`):
```go
	svcGetActiveMappingRulesRecon = db.GetActiveMappingRules // (ctx) ([]models.MappingRule, error)
	svcGetExclusions              = db.GetExclusions         // (ctx) ([]models.ExternalGrantExclusion, error)
```
(Match the actual `db` function names — `GetActiveMappingRules` is the source `svcGetActiveMappingRules` already wraps in views; confirm and reuse.)

In `handleGetReconciliationDiff`, after `diff := computeReconciliationDiff(...)`, filter `OnlyInZitadel` so rule-derived / excluded grants are no longer reported as pure Zitadel drift:
```go
	// A lookup failure here MUST NOT silently become an empty set — that would
	// misclassify rule-derived / excluded grants as red drift. Fail the request.
	rules, err := svcGetActiveMappingRulesRecon(ctx)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	exclusions, err := svcGetExclusions(ctx)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	holder := services.BuildHolderSet(syndraGrants, allZitadel)
	diff.OnlyInZitadel = filterExplained(diff.OnlyInZitadel, holder, rules, exclusions)
```
Add the helper (keeps `computeReconciliationDiff` itself pure and untouched):
```go
// filterExplained drops (user,project) entries whose every role is now explained
// by an active mapping rule or an external-grant exclusion — they are no longer
// pure Zitadel drift. A partially-explained entry keeps only its unexplained roles.
func filterExplained(in []ReconciliationGrant, holder map[services.HolderKey]bool,
	rules []models.MappingRule, exclusions []models.ExternalGrantExclusion) []ReconciliationGrant {
	out := make([]ReconciliationGrant, 0, len(in))
	for _, g := range in {
		var unexplained []string
		for _, rk := range g.RoleKeys {
			if services.ExpectedViaRule(holder, rules, g.UserID, g.ProjectID, rk) ||
				services.IsExcluded(exclusions, g.UserID, g.ProjectID, rk) {
				continue
			}
			unexplained = append(unexplained, rk)
		}
		if len(unexplained) > 0 {
			g.RoleKeys = unexplained
			out = append(out, g)
		}
	}
	return out
}
```
(Import `syndra/internal/services`.)

- [ ] **Step 13.6: Write the failing endpoint test (rule-derived not reported as drift)**

Add to `reconciliation_test.go`, extending `withReconciliationDeps` to also stub the two new deps (default empty), then:
```go
func TestReconciliation_RuleDerivedNotOnlyInZitadel(t *testing.T) {
	// Syndra has the source grant; Zitadel has source + rule-derived target.
	withReconciliationDeps(t,
		[]models.DirectGrant{directGrant("u-1", "p1", "member")},
		[]zitadel.UserGrant{
			{ID: "g1", UserID: "u-1", ProjectID: "p1", RoleKeys: []string{"member"}},
			{ID: "g2", UserID: "u-1", ProjectID: "p2", RoleKeys: []string{"contributor"}},
		}, 0, nil,
	)
	// Active rule: p1:member → p2:contributor.
	origRules := svcGetActiveMappingRulesRecon
	svcGetActiveMappingRulesRecon = func(context.Context) ([]models.MappingRule, error) {
		return []models.MappingRule{{SourceProject: "p1", SourceRole: "member", TargetProject: "p2", TargetRole: "contributor"}}, nil
	}
	t.Cleanup(func() { svcGetActiveMappingRulesRecon = origRules })

	got := decodeReconciliation(t, getReconciliation(t))
	for _, e := range got.OnlyInZitadel {
		if e.ProjectID == "p2" {
			t.Fatalf("rule-derived p2:contributor must NOT be OnlyInZitadel: %+v", got.OnlyInZitadel)
		}
	}
}
```

- [ ] **Step 13.7: Run tests + commit**

Run: `cd backend && go test ./internal/handlers/ -run TestReconciliation -v && go test ./internal/services/`
Expected: PASS.
```bash
git add backend/internal/handlers/reconciliation.go backend/internal/handlers/reconciliation_test.go backend/internal/services/expected.go backend/internal/services/expected_test.go
git commit -m "feat(reconciliation): cap 10k→2k + rule/exclusion-aware expected set (B2)"
```

---

## Task 14 — `services/drift` scheduled sweep + `main.go` wiring (B2, C6)

**Files:**
- Create: `backend/internal/services/drift/deps.go`
- Create: `backend/internal/services/drift/sweep.go`
- Create: `backend/internal/services/drift/scheduler.go`
- Create: `backend/internal/services/drift/sweep_test.go`
- Modify: `backend/internal/db/propagations.go` (add the exported `NewOutboxIdempotencyKey` wrapper — `services/drift` needs cross-package key minting)
- Modify: `backend/cmd/api/main.go` (wire the scheduler beside `sched`; env helpers)
- Modify: `.env.example`

The package mirrors `services/expiry/` structure exactly. The sweep computes drift at **triple granularity** (one drift row / re-enqueue per role) using its own small diff — it does NOT import the handlers-package `computeReconciliationDiff` (that would cycle: the Task-16 `[Reconcile now]` handler imports this package). The shared, tricky part (expected-set classification) is reused from `services/expected.go`.

- [ ] **Step 14.0: Export the idempotency-key minter for cross-package use**

`services/drift` (a different package) cannot call the unexported `newOutboxIdempotencyKey`. Add a one-line exported wrapper to `backend/internal/db/propagations.go` (beside the unexported minter) — a wrapper, not a rename, so `EnqueueDirectGrantPropagation`'s existing call site stays untouched:
```go
// NewOutboxIdempotencyKey is the exported entrypoint to the crypto/rand v4 minter
// for cross-package callers (services/drift). The repo has no uuid module.
func NewOutboxIdempotencyKey() (string, error) { return newOutboxIdempotencyKey() }
```
Run: `cd backend && go build ./internal/db/`
Expected: builds clean.

- [ ] **Step 14.1: Write `deps.go` (injectables mirroring `expiry/deps.go`)**

```go
// deps.go
package drift

import (
	"context"

	"syndra/internal/db"
	"syndra/internal/models"
	"syndra/internal/services"
	"syndra/internal/zitadel"
)

// driftSafetyCap is the same right-sized cap as the on-demand reconciliation
// endpoint (B2): the sweep pages Zitadel grants and stops here.
const driftSafetyCap = 2_000
const zitadelPageSize = 500

var (
	svcAllDirectGrants = func(ctx context.Context) ([]models.DirectGrant, error) {
		return db.GetAllDirectGrants(ctx)
	}
	svcGetActiveMappingRules = func(ctx context.Context) ([]models.MappingRule, error) {
		return db.GetActiveMappingRules(ctx)
	}
	svcGetExclusions = func(ctx context.Context) ([]models.ExternalGrantExclusion, error) {
		return db.GetExclusions(ctx)
	}
	upsertDriftItem = db.UpsertDriftItem   // (ctx,user,project,roleKeys,grantID,source,type) (id,inserted,err)
	hasPendingDrift = db.HasPendingDrift   // (ctx,user,project,role,type) (bool,err)
	insertPending   = db.InsertPendingPropagation // re-enqueue path (syndra_only)

	// Reachability + paginated grant listing. A nil MgmtClient means offline.
	zitadelReachable = func(ctx context.Context) bool { return zitadel.MgmtClient != nil }
	zitadelListAllGrants = func(ctx context.Context, p zitadel.SearchParams) (*zitadel.SearchResult[zitadel.UserGrant], error) {
		return zitadel.MgmtClient.ListAllGrants(ctx, p)
	}

	// idempotency-key minting for re-enqueued rows: reuse the outbox's crypto/rand
	// helper (the repo has NO uuid module). Returns (string, error); the sweep
	// handles the error.
	newIdempotencyKey = db.NewOutboxIdempotencyKey // () (string, error)
)

// classification helpers are pure and shared with the reconciliation endpoint.
var (
	buildHolderSet  = services.BuildHolderSet
	expectedViaRule = services.ExpectedViaRule
	isExcluded      = services.IsExcluded
)
```
> `newIdempotencyKey` is `db.NewOutboxIdempotencyKey` — the exported crypto/rand v4 minter added in Step 14.0. No `github.com/google/uuid`. `InsertPendingPropagation(ctx, opType, user, project, roleKeys, grantID, payloadJSON, idempotencyKey, initiatedBy) (string, error)` takes the minted key as its 8th arg.

- [ ] **Step 14.2: Write failing sweep tests (injectable, no live DB/Zitadel)**

```go
// sweep_test.go
package drift

import (
	"context"
	"testing"

	"syndra/internal/models"
	"syndra/internal/zitadel"
)

func swap[T any](dst *T, v T) func() { o := *dst; *dst = v; return func() { *dst = o } }

// stubSweep sets safe no-op defaults; each test overrides only what it asserts.
func stubSweep(t *testing.T) {
	t.Cleanup(swap(&zitadelReachable, func(context.Context) bool { return true }))
	t.Cleanup(swap(&svcAllDirectGrants, func(context.Context) ([]models.DirectGrant, error) { return nil, nil }))
	t.Cleanup(swap(&svcGetActiveMappingRules, func(context.Context) ([]models.MappingRule, error) { return nil, nil }))
	t.Cleanup(swap(&svcGetExclusions, func(context.Context) ([]models.ExternalGrantExclusion, error) { return nil, nil }))
	t.Cleanup(swap(&zitadelListAllGrants, func(context.Context, zitadel.SearchParams) (*zitadel.SearchResult[zitadel.UserGrant], error) {
		return &zitadel.SearchResult[zitadel.UserGrant]{}, nil
	}))
	t.Cleanup(swap(&upsertDriftItem, func(context.Context, string, string, []string, string, string, string) (string, bool, error) { return "d1", true, nil }))
	t.Cleanup(swap(&hasPendingDrift, func(context.Context, string, string, string, string) (bool, error) { return false, nil }))
	t.Cleanup(swap(&insertPending, func(context.Context, string, string, string, []string, string, string, string, string) (string, error) { return "o1", nil }))
}

func TestSweep_UnexplainedZitadelGrantBecomesDrift(t *testing.T) {
	stubSweep(t)
	defer swap(&zitadelListAllGrants, func(context.Context, zitadel.SearchParams) (*zitadel.SearchResult[zitadel.UserGrant], error) {
		return &zitadel.SearchResult[zitadel.UserGrant]{
			Items: []zitadel.UserGrant{{ID: "g1", UserID: "u1", ProjectID: "p1", RoleKeys: []string{"viewer"}}},
			Total: 1,
		}, nil
	})()
	var driftType string
	defer swap(&upsertDriftItem, func(_ context.Context, _, _ string, _ []string, _, _, dtype string) (string, bool, error) {
		driftType = dtype
		return "d1", true, nil
	})()

	res, err := Sweep(context.Background())
	if err != nil { t.Fatal(err) }
	if driftType != "zitadel_only" || res.DriftItemsCreated != 1 {
		t.Fatalf("unexplained zitadel grant must create a zitadel_only drift item, got type=%q res=%+v", driftType, res)
	}
}

func TestSweep_RuleDerivedGrantIsNotDrift(t *testing.T) {
	stubSweep(t)
	defer swap(&svcAllDirectGrants, func(context.Context) ([]models.DirectGrant, error) {
		return []models.DirectGrant{{UserID: "u1", ProjectID: "p1", RoleKey: "member"}}, nil
	})()
	defer swap(&svcGetActiveMappingRules, func(context.Context) ([]models.MappingRule, error) {
		return []models.MappingRule{{SourceProject: "p1", SourceRole: "member", TargetProject: "p2", TargetRole: "contributor"}}, nil
	})()
	defer swap(&zitadelListAllGrants, func(context.Context, zitadel.SearchParams) (*zitadel.SearchResult[zitadel.UserGrant], error) {
		return &zitadel.SearchResult[zitadel.UserGrant]{
			Items: []zitadel.UserGrant{
				{ID: "g1", UserID: "u1", ProjectID: "p1", RoleKeys: []string{"member"}},
				{ID: "g2", UserID: "u1", ProjectID: "p2", RoleKeys: []string{"contributor"}},
			}, Total: 2,
		}, nil
	})()
	var created int
	defer swap(&upsertDriftItem, func(context.Context, string, string, []string, string, string, string) (string, bool, error) { created++; return "d", true, nil })()

	if _, err := Sweep(context.Background()); err != nil { t.Fatal(err) }
	if created != 0 {
		t.Fatalf("rule-derived + source grants are both explained; no drift expected, got %d", created)
	}
}

func TestSweep_SyndraOnlyDirectGrantReEnqueues(t *testing.T) {
	stubSweep(t)
	// Syndra expects u1/p1/viewer; Zitadel has nothing → re-enqueue (missed-webhook replay).
	defer swap(&svcAllDirectGrants, func(context.Context) ([]models.DirectGrant, error) {
		return []models.DirectGrant{{UserID: "u1", ProjectID: "p1", RoleKey: "viewer"}}, nil
	})()
	var opType string
	defer swap(&insertPending, func(_ context.Context, ot, _, _ string, _ []string, _, _, _, _ string) (string, error) { opType = ot; return "o1", nil })()

	res, err := Sweep(context.Background())
	if err != nil { t.Fatal(err) }
	if opType != "add" || res.ReEnqueued != 1 {
		t.Fatalf("syndra_only direct grant must re-enqueue an add, got op=%q res=%+v", opType, res)
	}
}

func TestSweep_SyndraOnlySkipsReEnqueueWhenPendingDrift(t *testing.T) {
	stubSweep(t)
	defer swap(&svcAllDirectGrants, func(context.Context) ([]models.DirectGrant, error) {
		return []models.DirectGrant{{UserID: "u1", ProjectID: "p1", RoleKey: "viewer"}}, nil
	})()
	defer swap(&hasPendingDrift, func(context.Context, string, string, string, string) (bool, error) { return true, nil })()
	var reEnqueued bool
	defer swap(&insertPending, func(context.Context, string, string, string, []string, string, string, string, string) (string, error) { reEnqueued = true; return "o1", nil })()

	res, _ := Sweep(context.Background())
	if reEnqueued || res.ReEnqueued != 0 {
		t.Fatalf("a triple already under triage must not auto-re-enqueue, res=%+v", res)
	}
}

func TestSweep_HaltsWhenZitadelOffline(t *testing.T) {
	stubSweep(t)
	defer swap(&zitadelReachable, func(context.Context) bool { return false })()
	res, err := Sweep(context.Background())
	if err != nil { t.Fatal(err) }
	if !res.Halted || res.Reason != "zitadel_offline" {
		t.Fatalf("offline sweep must halt cleanly, got %+v", res)
	}
}

func TestSweep_ExcludedGrantIsNotDrift(t *testing.T) {
	stubSweep(t)
	defer swap(&svcGetExclusions, func(context.Context) ([]models.ExternalGrantExclusion, error) {
		return []models.ExternalGrantExclusion{{UserID: "u1", ProjectID: "p1", RoleKey: "viewer"}}, nil
	})()
	defer swap(&zitadelListAllGrants, func(context.Context, zitadel.SearchParams) (*zitadel.SearchResult[zitadel.UserGrant], error) {
		return &zitadel.SearchResult[zitadel.UserGrant]{
			Items: []zitadel.UserGrant{{ID: "g1", UserID: "u1", ProjectID: "p1", RoleKeys: []string{"viewer"}}}, Total: 1,
		}, nil
	})()
	var created int
	defer swap(&upsertDriftItem, func(context.Context, string, string, []string, string, string, string) (string, bool, error) { created++; return "d", true, nil })()

	if _, err := Sweep(context.Background()); err != nil { t.Fatal(err) }
	if created != 0 { t.Fatalf("marked-external triple must not drift, got %d", created) }
}
```

- [ ] **Step 14.3: Run — expect compile failure (`Sweep` undefined)**

Run: `cd backend && go test ./internal/services/drift/ -v`
Expected: FAIL — `undefined: Sweep`.

- [ ] **Step 14.4: Implement `sweep.go`**

```go
// sweep.go
package drift

import (
	"context"
	"log"

	"syndra/internal/models"
	"syndra/internal/services"
	"syndra/internal/zitadel"
)

// DriftResult summarizes one sweep for logs + the [Reconcile now] response.
type DriftResult struct {
	ZitadelGrants     int    `json:"zitadel_grants"`
	DriftItemsCreated int    `json:"drift_items_created"` // zitadel_only, deduped
	ReEnqueued        int    `json:"re_enqueued"`         // syndra_only replays
	Truncated         bool   `json:"truncated"`
	Halted            bool   `json:"halted"`
	Reason            string `json:"reason,omitempty"`
}

// Sweep reconciles Zitadel grants against Syndra's expected set. Callable by the
// scheduler and by the operator's [Reconcile now]. Two outcomes per role:
//   - zitadel_only  (in Zitadel, unexplained by direct/rule/exclusion) → drift_items
//   - syndra_only   (direct grant Syndra expects, absent from Zitadel)  → outbox re-enqueue
// Bundle/rule-derived expected roles that are ABSENT from Zitadel are NOT drift
// in sub-phase 2 — cascade projection is sub-phase 3, so they are legitimately
// unprojected. Only source-mediated direct grants can be syndra_only here.
func Sweep(ctx context.Context) (DriftResult, error) {
	if !zitadelReachable(ctx) {
		return DriftResult{Halted: true, Reason: "zitadel_offline"}, nil
	}

	direct, err := svcAllDirectGrants(ctx)
	if err != nil {
		return DriftResult{}, err
	}
	zit, truncated, err := fetchAllZitadelGrants(ctx)
	if err != nil {
		return DriftResult{}, err
	}
	// A lookup failure MUST NOT degrade to an empty set — that would flag
	// rule-derived / excluded grants as false drift. Abort the sweep instead;
	// the scheduler retries next tick and no noisy drift rows are written.
	rules, err := svcGetActiveMappingRules(ctx)
	if err != nil {
		return DriftResult{}, fmt.Errorf("drift sweep: load rules: %w", err)
	}
	exclusions, err := svcGetExclusions(ctx)
	if err != nil {
		return DriftResult{}, fmt.Errorf("drift sweep: load exclusions: %w", err)
	}
	holder := services.BuildHolderSet(direct, zit)

	res := DriftResult{ZitadelGrants: len(zit), Truncated: truncated}

	// --- zitadel_only: unexplained live grants → drift_items ---
	directSet := services.BuildHolderSet(direct, nil) // Syndra's own direct intent
	for _, g := range zit {
		for _, rk := range g.RoleKeys {
			k := services.HolderKey{UserID: g.UserID, ProjectID: g.ProjectID, RoleKey: rk}
			if directSet[k] {
				continue // Syndra has a direct intent for this — not drift
			}
			if services.ExpectedViaRule(holder, rules, g.UserID, g.ProjectID, rk) {
				continue // expected_via_rule — not drift (design §7 Q… / access-governance scenario)
			}
			if services.IsExcluded(exclusions, g.UserID, g.ProjectID, rk) {
				continue // marked external — silently filtered
			}
			if _, inserted, err := upsertDriftItem(ctx, g.UserID, g.ProjectID,
				[]string{rk}, g.ID, "reconciliation_sweep", "zitadel_only"); err != nil {
				log.Printf("[DRIFT] upsert zitadel_only failed user=%s project=%s role=%s: %v", g.UserID, g.ProjectID, rk, err)
			} else if inserted {
				res.DriftItemsCreated++
			}
		}
	}

	// --- syndra_only: direct grants Syndra expects but Zitadel lacks → re-enqueue ---
	zitSet := services.BuildHolderSet(nil, zit)
	for _, dg := range direct {
		k := services.HolderKey{UserID: dg.UserID, ProjectID: dg.ProjectID, RoleKey: dg.RoleKey}
		if zitSet[k] {
			continue // present in Zitadel — no drift
		}
		// Skip if the operator is already triaging this triple (webhook grant_removed
		// may have surfaced it) — do not fight an in-flight triage with an auto-replay.
		if pending, _ := hasPendingDrift(ctx, dg.UserID, dg.ProjectID, dg.RoleKey, "syndra_only"); pending {
			continue
		}
		key, kerr := newIdempotencyKey()
		if kerr != nil {
			log.Printf("[DRIFT] mint idempotency key failed user=%s: %v (skipping re-enqueue)", dg.UserID, kerr)
			continue
		}
		if _, err := insertPending(ctx, "add", dg.UserID, dg.ProjectID, []string{dg.RoleKey},
			"", "{}", key, "system:drift-sweep"); err != nil {
			log.Printf("[DRIFT] re-enqueue syndra_only failed user=%s project=%s role=%s: %v", dg.UserID, dg.ProjectID, dg.RoleKey, err)
		} else {
			res.ReEnqueued++
		}
	}

	log.Printf("[DRIFT] Sweep complete: zitadel_grants=%d drift_created=%d re_enqueued=%d truncated=%v",
		res.ZitadelGrants, res.DriftItemsCreated, res.ReEnqueued, res.Truncated)
	return res, nil
}

// fetchAllZitadelGrants pages ListAllGrants, capped at driftSafetyCap (B2).
// Mirrors handlers/reconciliation.go:fetchAllZitadelGrants; kept here so the
// drift package is self-contained (avoids a handlers→drift→handlers cycle).
func fetchAllZitadelGrants(ctx context.Context) ([]zitadel.UserGrant, bool, error) {
	var all []zitadel.UserGrant
	offset := 0
	for {
		page, err := zitadelListAllGrants(ctx, zitadel.SearchParams{Limit: zitadelPageSize, Offset: offset})
		if err != nil {
			return nil, false, err
		}
		all = append(all, page.Items...)
		if len(all) >= page.Total || len(page.Items) == 0 {
			return all, false, nil
		}
		if len(all) >= driftSafetyCap {
			return all, true, nil
		}
		offset += len(page.Items)
	}
}
```

- [ ] **Step 14.5: Implement `scheduler.go` (mirror `expiry/scheduler.go`)**

```go
// scheduler.go
package drift

import (
	"context"
	"log"
	"time"
)

// Scheduler drives periodic reconciliation sweeps. Mirrors expiry.Scheduler:
// immediate sweep on boot + tick on interval + graceful Done() for shutdown.
// Single-instance assumption (single-LXC); no leader election.
type Scheduler struct {
	interval time.Duration
	done     chan struct{}
}

func NewScheduler(interval time.Duration) *Scheduler {
	if interval <= 0 {
		interval = 6 * time.Hour
	}
	return &Scheduler{interval: interval, done: make(chan struct{})}
}

func (s *Scheduler) Done() <-chan struct{} { return s.done }

func (s *Scheduler) Start(ctx context.Context) {
	log.Printf("[DRIFT] Starting reconciliation scheduler: interval=%s", s.interval)
	defer close(s.done)

	s.runOnce(ctx)

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Println("[DRIFT] Stopping on context cancellation")
			return
		case <-ticker.C:
			s.runOnce(ctx)
		}
	}
}

func (s *Scheduler) runOnce(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[DRIFT] Panic recovered during sweep: %v", r)
		}
	}()
	if _, err := Sweep(ctx); err != nil {
		log.Printf("[DRIFT] Sweep error: %v", err)
	}
}
```

- [ ] **Step 14.6: Run sweep tests — expect PASS**

Run: `cd backend && go test ./internal/services/drift/ -v`
Expected: PASS (all six sweep tests).

- [ ] **Step 14.7: Wire the scheduler in `main.go` + env helpers**

In `cmd/api/main.go`, beside the expiry `sched` block (`:97-109`):
```go
	// Drift reconciliation scheduler: periodic Zitadel↔Syndra sweep (B2/C6).
	var driftSched *drift.Scheduler
	if driftSchedulerEnabled() {
		driftSched = drift.NewScheduler(driftInterval())
		go driftSched.Start(ctx)
	} else {
		log.Println("[DRIFT] Disabled via DRIFT_SCHEDULER_ENABLED=false")
	}
```
In the graceful-shutdown block (`:133-139`), join it beside `sched`:
```go
	if driftSched != nil {
		select {
		case <-driftSched.Done():
		case <-shutdownCtx.Done():
			log.Println("[DRIFT] Shutdown deadline exceeded waiting for scheduler; closing anyway")
		}
	}
```
Add env helpers beside `schedulerInterval()` (`:149-186`):
```go
func driftSchedulerEnabled() bool {
	v := os.Getenv("DRIFT_SCHEDULER_ENABLED")
	if v == "" {
		return true
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		log.Printf("[DRIFT] Invalid DRIFT_SCHEDULER_ENABLED=%q, defaulting to enabled", v)
		return true
	}
	return b
}

func driftInterval() time.Duration {
	v := os.Getenv("DRIFT_RECONCILIATION_INTERVAL_HOURS")
	if v == "" {
		return 6 * time.Hour
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 {
		log.Printf("[DRIFT] Invalid DRIFT_RECONCILIATION_INTERVAL_HOURS=%q, defaulting to 6", v)
		return 6 * time.Hour
	}
	return time.Duration(n) * time.Hour
}
```
(Import `syndra/internal/services/drift`.)

- [ ] **Step 14.8: Document env vars in `.env.example`**

Add beside the expiry scheduler block:
```
# --- Drift Reconciliation Scheduler (optional — defaults are sensible) ---
# Periodic Zitadel↔Syndra grant reconciliation. zitadel_only grants become
# drift_items for operator triage; syndra_only direct grants re-enqueue through
# the outbox (missed-webhook replay). Also triggerable on demand via
# [Reconcile now] on /governance/drift.
# DRIFT_SCHEDULER_ENABLED=true
# DRIFT_RECONCILIATION_INTERVAL_HOURS=6
```

- [ ] **Step 14.9: Build + vet + commit**

Run: `cd backend && go build ./... && go vet ./internal/services/drift/ ./cmd/api/`
```bash
git add backend/internal/services/drift/ backend/internal/db/propagations.go backend/cmd/api/main.go .env.example
git commit -m "feat(drift): scheduled reconciliation sweep + main.go wiring (B2/C6)"
```

---

## Task 15 — Real-time webhook drift detection (C6)

**Files:**
- Modify: `backend/internal/handlers/webhook.go` (`processGrantAdded` hook)
- Modify: `backend/internal/handlers/deps.go` (add drift injectables)
- Modify: `backend/internal/handlers/webhook_test.go` (or a focused new test file)

A surviving `grant_added` reached this code only because it was NOT Syndra's own mutation (self-mutation guard). If it is unexplained by Syndra's expected set and not excluded, it is real-time drift.

**Scope decision (resolved here):** Task 15 implements `grant_added → zitadel_only` drift — the one scenario the spec mandates ("Webhook detects an externally-authored grant"). External `grant_removed` of an Syndra-expected grant is left to the sweep's `syndra_only` re-enqueue path (which already skips triples under active triage). This avoids a webhook/sweep double-handling of the same `syndra_only` triple and keeps the real-time path to the single tested behavior. `ponytail:` grant_removed real-time drift can be added later if the 6-hour sweep latency proves too slow for removals; the sweep is the documented backstop.

- [ ] **Step 15.1: Add drift injectables in `handlers/deps.go`**

```go
	dbUpsertDriftItem  = db.UpsertDriftItem
	dbGetExclusions    = db.GetExclusions
	dbHasExclusion     = func(ctx context.Context, u, p, r string) (bool, error) {
		ex, err := db.GetExclusions(ctx)
		if err != nil {
			return false, err
		}
		return services.IsExcluded(ex, u, p, r), nil
	}
	// svcUserEffectiveRoles reports whether Syndra already expects (project,role)
	// for the user — via direct grant, bundle, or mapping rule. Reuses the
	// existing per-user resolver so the webhook's "is this explained?" check is
	// one function, not a re-implementation.
	svcUserExpectsRole = services.UserExpectsRole // (ctx, userID, projectID, roleKey) (bool, error)
```
Add `services.UserExpectsRole` to `services/expected.go` (or `views.go`), reusing `collectUserRoles`:
```go
// UserExpectsRole reports whether Syndra's effective-role computation already
// includes (projectID, roleKey) for the user (direct | bundle | rule). Used by
// the webhook to decide whether a surviving external grant event is drift.
func UserExpectsRole(ctx context.Context, userID, projectID, roleKey string) (bool, error) {
	roleMap, _, err := collectUserRoles(ctx, userID)
	if err != nil {
		return false, err
	}
	_, ok := roleMap[roleKey{projectID: projectID, roleKey: roleKey}]
	return ok, nil
}
```
(`collectUserRoles` and the unexported `roleKey` type are already in `views.go`, same package.)

- [ ] **Step 15.2: Write the failing webhook-drift test**

```go
func TestProcessGrantAdded_UnexplainedGrantCreatesDrift(t *testing.T) {
	resetWebhookDeps(t) // mirror the existing webhook test reset helper
	// Downstream orchestration no-ops so the test isolates the drift hook.
	svcUserExpectsRole = func(context.Context, string, string, string) (bool, error) { return false, nil }
	dbHasExclusion = func(context.Context, string, string, string) (bool, error) { return false, nil }
	var driftUser, driftType string
	dbUpsertDriftItem = func(_ context.Context, u, _ string, _ []string, _, source, dtype string) (string, bool, error) {
		driftUser, driftType = u, dtype
		if source != "webhook" {
			t.Fatalf("detection_source must be webhook, got %q", source)
		}
		return "d1", true, nil
	}

	ev := WebhookPayload{EventType: "grant_added", UserID: "ext-u", SourceProject: "p1", RoleKeys: []string{"admin"}, GrantID: "g9"}
	if err := processGrantAdded(context.Background(), ev, "evt-1"); err != nil {
		t.Fatal(err)
	}
	if driftUser != "ext-u" || driftType != "zitadel_only" {
		t.Fatalf("unexplained external grant must create zitadel_only drift, got user=%q type=%q", driftUser, driftType)
	}
}

func TestProcessGrantAdded_ExpectedGrantNoDrift(t *testing.T) {
	resetWebhookDeps(t)
	svcUserExpectsRole = func(context.Context, string, string, string) (bool, error) { return true, nil } // Syndra expects it
	called := false
	dbUpsertDriftItem = func(context.Context, string, string, []string, string, string, string) (string, bool, error) { called = true; return "", false, nil }

	ev := WebhookPayload{EventType: "grant_added", UserID: "u1", SourceProject: "p1", RoleKeys: []string{"member"}, GrantID: "g1"}
	if err := processGrantAdded(context.Background(), ev, "evt-2"); err != nil {
		t.Fatal(err)
	}
	if called {
		t.Fatal("a grant Syndra already expects must not be flagged as drift")
	}
}
```

- [ ] **Step 15.3: Run — expect FAIL (no drift hook yet)**

Run: `cd backend && go test ./internal/handlers/ -run TestProcessGrantAdded -v`
Expected: FAIL — `dbUpsertDriftItem` never called.

- [ ] **Step 15.4: Add the hook to `processGrantAdded`**

Just before `return nil` in `processGrantAdded` (`webhook.go:254`):
```go
	// Real-time drift: a surviving (non-self, per the self-mutation guard)
	// grant event that Syndra neither expects nor has excluded is out-of-band.
	detectWebhookDrift(ctx, event)
	return nil
}

// detectWebhookDrift flags roles on a surviving grant event that Syndra has no
// intent for. Best-effort and non-fatal: a detection failure must never bounce
// a 4xx back to Zitadel (redelivery storm) — the sweep is the backstop.
func detectWebhookDrift(ctx context.Context, event WebhookPayload) {
	for _, role := range event.RoleKeys {
		expected, err := svcUserExpectsRole(ctx, event.UserID, event.SourceProject, role)
		if err != nil {
			log.Printf("[DRIFT] webhook expected-check failed user=%s role=%s: %v (skipping)", event.UserID, role, err)
			continue
		}
		if expected {
			continue
		}
		excluded, err := dbHasExclusion(ctx, event.UserID, event.SourceProject, role)
		if err != nil {
			log.Printf("[DRIFT] webhook exclusion-check failed user=%s role=%s: %v (skipping — not flagging on uncertainty)", event.UserID, role, err)
			continue
		}
		if excluded {
			continue
		}
		if _, _, err := dbUpsertDriftItem(ctx, event.UserID, event.SourceProject,
			[]string{role}, event.GrantID, "webhook", "zitadel_only"); err != nil {
			log.Printf("[DRIFT] webhook upsert failed user=%s role=%s: %v (non-fatal)", event.UserID, role, err)
		}
	}
}
```

- [ ] **Step 15.5: Run — expect PASS; then full handlers package**

Run: `cd backend && go test ./internal/handlers/ -run TestProcessGrantAdded -v && go test ./internal/handlers/`
Expected: PASS. Fix any existing `processGrantAdded` test that now needs the new deps stubbed (add them to `resetWebhookDeps`).

- [ ] **Step 15.6: Commit**

```bash
git add backend/internal/handlers/webhook.go backend/internal/handlers/deps.go backend/internal/handlers/webhook_test.go backend/internal/services/expected.go
git commit -m "feat(webhook): real-time zitadel_only drift detection for surviving external grants (C6)"
```

---

## Task 16 — Drift triage API (B2)

**Files:**
- Create: `backend/internal/handlers/drift.go`
- Modify: `backend/internal/handlers/router.go` (register routes, operator-auth)
- Modify: `backend/internal/handlers/deps.go` (drift + sweep + enqueue injectables)
- Modify: `backend/internal/services/views.go` (`Drift` block in governance summary)
- Create: `backend/internal/handlers/drift_test.go`

Endpoints (all operator-auth, mirroring the reconciliation/propagations route wiring):
| Route | Action |
|---|---|
| `GET /api/v1/governance/drift` | list `pending_triage` rows; `?user_id=&project_id=&source=` filters |
| `POST /api/v1/governance/drift/{id}/attribute` | body `{source, source_ref?}` (`external_backfill`\|`bundle`\|`rule`); write ledger via outbox `add` (self-resolves 409/short-circuit); mark `attributed` |
| `POST /api/v1/governance/drift/{id}/revoke` | enqueue `op_type='revoke'` (grantId from the drift row), `DrainOne`; mark `revoked` |
| `POST /api/v1/governance/drift/{id}/mark-external` | insert exclusion; mark `marked_external` |
| `POST /api/v1/governance/drift/bulk-attribute` | body `{ids[], source, source_ref?}`; attribute many (bootstrap case) |
| `POST /api/v1/governance/drift/reconcile` | trigger `drift.Sweep` on demand (`[Reconcile now]`) |

- [ ] **Step 16.1: Add injectables in `handlers/deps.go`**

```go
	dbGetDriftItems            = db.GetDriftItems
	dbGetDriftItem             = db.GetDriftItem
	dbAttributeDriftAndEnqueue = db.AttributeDriftAndEnqueue // atomic claim+enqueue (add)
	dbRevokeDriftAndEnqueue    = db.RevokeDriftAndEnqueue    // atomic claim+enqueue (revoke) → outbox id
	dbMarkDriftExternalTx      = db.MarkDriftExternalTx      // atomic claim+exclusion insert
	svcDriftSweep              = drift.Sweep
	svcDrainOne                = propagation.DrainOne         // phase-1 targeted drain (revoke, best-effort after commit)
	svcGetRolesForBundleDrift  = db.GetRolesForBundle         // source-remap validation for attribute→bundle
```
(Attribute/revoke enqueue via the transactional `db.*AndEnqueue` helpers — which internally reuse phase-1's `enqueueWrites` — so the drift handlers never call `dbEnqueueDirectGrantPropagation` directly and never resolve the drift row outside the enqueue's transaction. Import `syndra/internal/services/drift` and `syndra/internal/services/propagation`.)

- [ ] **Step 16.2: Write failing handler tests**

```go
func TestHandleMarkExternal_ResolvesAtomically(t *testing.T) {
	resetDriftDeps(t)
	dbGetDriftItem = func(context.Context, string) (models.DriftItem, error) {
		return models.DriftItem{ID: "d1", UserID: "u1", ProjectID: "p1", RoleKeys: []string{"viewer"}, DriftType: "zitadel_only", Status: "pending_triage"}, nil
	}
	var gotUser, gotRole string
	dbMarkDriftExternalTx = func(_ context.Context, _, user, _ string, roles []string, _, _, _ string) error {
		gotUser = user
		if len(roles) > 0 {
			gotRole = roles[0]
		}
		return nil
	}

	req := httptest.NewRequest("POST", "/api/v1/governance/drift/d1/mark-external", strings.NewReader(`{"reason":"partner org"}`))
	req.SetPathValue("id", "d1")
	w := httptest.NewRecorder()
	handleMarkDriftExternal(w, req)

	if w.Code != http.StatusOK { t.Fatalf("want 200, got %d: %s", w.Code, w.Body) }
	if gotUser != "u1" || gotRole != "viewer" {
		t.Fatalf("mark-external must pass the drift triple to the atomic tx helper (user=%q role=%q)", gotUser, gotRole)
	}
}

func TestHandleRevokeDrift_EnqueuesRevokeAtomicallyThenDrains(t *testing.T) {
	resetDriftDeps(t)
	dbGetDriftItem = func(context.Context, string) (models.DriftItem, error) {
		return models.DriftItem{ID: "d1", UserID: "u1", ProjectID: "p1", RoleKeys: []string{"viewer"}, ZitadelGrantID: "g1", DriftType: "zitadel_only", Status: "pending_triage"}, nil
	}
	var gotOp string
	dbRevokeDriftAndEnqueue = func(_ context.Context, _ string, p db.EnqueueParams) (string, error) {
		gotOp = p.OpType
		return "o1", nil
	}
	var drained string
	svcDrainOne = func(_ context.Context, id string) (propagation.DrainResult, error) { drained = id; return propagation.DrainResult{Applied: 1}, nil }

	req := httptest.NewRequest("POST", "/api/v1/governance/drift/d1/revoke", nil)
	req.SetPathValue("id", "d1")
	w := httptest.NewRecorder()
	handleRevokeDrift(w, req)

	if w.Code != http.StatusOK { t.Fatalf("want 200, got %d: %s", w.Code, w.Body) }
	if gotOp != "revoke" || drained != "o1" {
		t.Fatalf("revoke must enqueue op=revoke atomically then drain that row (op=%q drained=%q)", gotOp, drained)
	}
}

func TestHandleAttributeToBundle_RejectsBundleWithoutRole(t *testing.T) {
	resetDriftDeps(t)
	dbGetDriftItem = func(context.Context, string) (models.DriftItem, error) {
		return models.DriftItem{ID: "d1", UserID: "u1", ProjectID: "p1", RoleKeys: []string{"viewer"}, Status: "pending_triage"}, nil
	}
	// The chosen bundle does NOT contain the drift role → source-remap validation fails.
	svcGetRolesForBundleDrift = func(context.Context, string) ([]models.BundleRole, error) {
		return []models.BundleRole{{ProjectID: "p1", RoleKey: "editor"}}, nil
	}

	req := httptest.NewRequest("POST", "/api/v1/governance/drift/d1/attribute", strings.NewReader(`{"source":"bundle","source_ref":"b1"}`))
	req.SetPathValue("id", "d1")
	w := httptest.NewRecorder()
	handleAttributeDrift(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("attributing to a bundle lacking the drift role must be 400, got %d", w.Code)
	}
}

func TestHandleRevokeDrift_LostRaceIs409(t *testing.T) {
	resetDriftDeps(t)
	dbGetDriftItem = func(context.Context, string) (models.DriftItem, error) {
		return models.DriftItem{ID: "d1", UserID: "u1", ProjectID: "p1", RoleKeys: []string{"viewer"}, ZitadelGrantID: "g1", Status: "pending_triage"}, nil
	}
	// The atomic claim+enqueue's guarded UPDATE matched nothing (already resolved
	// by another operator); the whole tx rolled back — nothing was written.
	dbRevokeDriftAndEnqueue = func(context.Context, string, db.EnqueueParams) (string, error) { return "", db.ErrDriftNotPending }
	var drained bool
	svcDrainOne = func(context.Context, string) (propagation.DrainResult, error) { drained = true; return propagation.DrainResult{}, nil }

	req := httptest.NewRequest("POST", "/api/v1/governance/drift/d1/revoke", nil)
	req.SetPathValue("id", "d1")
	w := httptest.NewRecorder()
	handleRevokeDrift(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("a lost triage race must be 409, got %d", w.Code)
	}
	if drained {
		t.Fatal("no drain when the atomic claim+enqueue tx rolled back")
	}
}

func TestHandleReconcileNow_TriggersSweep(t *testing.T) {
	resetDriftDeps(t)
	svcDriftSweep = func(context.Context) (drift.DriftResult, error) { return drift.DriftResult{DriftItemsCreated: 2}, nil }
	req := httptest.NewRequest("POST", "/api/v1/governance/drift/reconcile", nil)
	w := httptest.NewRecorder()
	handleReconcileDrift(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"drift_items_created":2`) {
		t.Fatalf("reconcile-now must run the sweep, got %d %s", w.Code, w.Body)
	}
}
```

- [ ] **Step 16.3: Implement `drift.go`**

```go
// drift.go
package handlers

import (
	"encoding/json"
	"net/http"

	"syndra/internal/db"
	"syndra/internal/models"
)

func handleListDrift(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	items, err := dbGetDriftItems(r.Context(), db.DriftFilter{
		UserID:          q.Get("user_id"),
		ProjectID:       q.Get("project_id"),
		DetectionSource: q.Get("source"),
	})
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	if items == nil {
		items = []models.DriftItem{}
	}
	jsonResponse(w, http.StatusOK, map[string]any{"drift": items})
}

type attributeRequest struct {
	Source    string `json:"source"`     // external_backfill | bundle | rule
	SourceRef string `json:"source_ref"` // bundle_id / rule_id
}

func handleAttributeDrift(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req attributeRequest
	if err := decodeJSONStrict(r, &req); err != nil {
		jsonErrorResponse(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	switch req.Source {
	case "external_backfill", "bundle", "rule":
	default:
		jsonErrorResponse(w, http.StatusBadRequest, "BAD_SOURCE", "source must be external_backfill, bundle, or rule")
		return
	}
	item, err := dbGetDriftItem(r.Context(), id)
	if err != nil {
		jsonErrorResponse(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}
	if err := attributeOneDrift(r.Context(), item, req, resolveActor(r, "operator")); err != nil {
		writeDriftActionError(w, err)
		return
	}
	jsonResponse(w, http.StatusOK, map[string]any{"status": "attributed"})
}

// attributeOneDrift writes the ledger intent for a zitadel_only drift and marks
// it attributed. The grant already exists in Zitadel, so it flows through the
// outbox as an `add` that self-resolves (grant-index short-circuit / 409→applied)
// — one code path, no special "skip Zitadel" branch. Bundle attribution is
// source-remap-validated: the bundle must actually contain the drift role.
//
// Atomicity (review-hardened): read-only validation first, then a SINGLE
// transaction (db.AttributeDriftAndEnqueue) that guard-transitions the drift row
// to 'attributed' AND writes the ledger+audit+outbox rows together. A lost
// concurrent-triage race returns ErrDriftNotPending (→409) with the whole tx
// rolled back; a write failure rolls back the resolution too — the drift never
// leaves the triage queue without its durable outbox row.
func attributeOneDrift(ctx context.Context, item models.DriftItem, req attributeRequest, actor string) error {
	if req.Source == "bundle" {
		roles, err := svcGetRolesForBundleDrift(ctx, req.SourceRef)
		if err != nil {
			return err
		}
		for _, rk := range item.RoleKeys {
			if !bundleHasRole(roles, item.ProjectID, rk) {
				return errDriftBadRemap // handler maps to 400
			}
		}
	}
	payload, _ := json.Marshal(req)
	return dbAttributeDriftAndEnqueue(ctx, item.ID, db.EnqueueParams{
		UserID: item.UserID, ProjectID: item.ProjectID, RoleKeys: item.RoleKeys,
		GrantedBy: actor, Reason: "drift attribution", Source: req.Source, SourceRef: req.SourceRef,
		OpType: "add", ZitadelGrantID: item.ZitadelGrantID, PayloadJSON: string(payload),
	})
}

func handleRevokeDrift(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	item, err := dbGetDriftItem(r.Context(), id)
	if err != nil {
		jsonErrorResponse(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}
	actor := resolveActor(r, "operator")
	// ONE tx: guard-transition to 'revoked' AND enqueue the revoke outbox row
	// together. A lost race 409s with nothing written; a write failure rolls the
	// resolution back too. Drain is best-effort AFTER commit — the durable revoke
	// row stays pending in the worklist if Zitadel is unreachable.
	outboxID, err := dbRevokeDriftAndEnqueue(r.Context(), id, db.EnqueueParams{
		UserID: item.UserID, ProjectID: item.ProjectID, RoleKeys: item.RoleKeys,
		GrantedBy: actor, OpType: "revoke",
		ZitadelGrantID: item.ZitadelGrantID, PayloadJSON: "{}",
	})
	if err != nil {
		writeDriftActionError(w, err) // ErrDriftNotPending → 409, else 500
		return
	}
	_, _ = svcDrainOne(r.Context(), outboxID)
	jsonResponse(w, http.StatusOK, map[string]any{"status": "revoked", "outbox_id": outboxID})
}

type markExternalRequest struct {
	Reason string `json:"reason"`
}

func handleMarkDriftExternal(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req markExternalRequest
	_ = decodeJSONStrict(r, &req) // reason is optional
	item, err := dbGetDriftItem(r.Context(), id)
	if err != nil {
		jsonErrorResponse(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}
	payload, _ := json.Marshal(req)
	// ONE tx: guard-transition to 'marked_external' AND insert the exclusion rows
	// together. A lost race 409s with no exclusion written; a write failure rolls
	// the resolution back too.
	if err := dbMarkDriftExternalTx(r.Context(), id, item.UserID, item.ProjectID,
		item.RoleKeys, resolveActor(r, "operator"), req.Reason, string(payload)); err != nil {
		writeDriftActionError(w, err) // ErrDriftNotPending → 409, else 500
		return
	}
	jsonResponse(w, http.StatusOK, map[string]any{"status": "marked_external"})
}

type bulkAttributeRequest struct {
	IDs       []string `json:"ids"`
	Source    string   `json:"source"`
	SourceRef string   `json:"source_ref"`
}

func handleBulkAttributeDrift(w http.ResponseWriter, r *http.Request) {
	var req bulkAttributeRequest
	if err := decodeJSONStrict(r, &req); err != nil {
		jsonErrorResponse(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	actor := resolveActor(r, "operator")
	attributed, failed := 0, 0
	for _, id := range req.IDs {
		item, err := dbGetDriftItem(r.Context(), id)
		if err != nil {
			failed++
			continue
		}
		if err := attributeOneDrift(r.Context(), item, attributeRequest{Source: req.Source, SourceRef: req.SourceRef}, actor); err != nil {
			failed++
			continue
		}
		attributed++
	}
	jsonResponse(w, http.StatusOK, map[string]any{"attributed": attributed, "failed": failed})
}

func handleReconcileDrift(w http.ResponseWriter, r *http.Request) {
	res, err := svcDriftSweep(r.Context())
	if err != nil {
		jsonErrorResponse(w, http.StatusBadGateway, "SWEEP_ERROR", err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, res)
}

func bundleHasRole(roles []models.BundleRole, projectID, roleKey string) bool {
	for _, r := range roles {
		if r.ProjectID == projectID && r.RoleKey == roleKey {
			return true
		}
	}
	return false
}
```
> Supporting pieces to add (small, same file or `deps.go`):
> - `var errDriftBadRemap = errors.New("bundle does not contain the drift role")` and `writeDriftActionError(w, err)` mapping `errDriftBadRemap`→400, `db.ErrDriftNotPending`→409, else→500. **`db.ErrDriftNotPending`→409 is load-bearing**: it is how a lost concurrent-triage race is reported (the atomic claim+enqueue transaction guarantees the whole action rolled back — no side effect ran).
> - All three actions build their resolution payload with `json.Marshal` (never hand-built JSON string concatenation).
> - Confirm the real names: `resolveActor` (used by phase-1 handlers), `decodeJSONStrict`, `db.GetRolesForBundle`, `models.BundleRole` (fields `ProjectID`/`RoleKey`). Match them to what `services/views.go`'s `svcGetRolesForBundle` returns.

- [ ] **Step 16.4: Register routes in `router.go`**

Beside the reconciliation + propagations routes:
```go
	mux.HandleFunc("GET /api/v1/governance/drift", withCORS(withOperatorAuth(handleListDrift)))
	mux.HandleFunc("POST /api/v1/governance/drift/reconcile", withCORS(withOperatorAuth(handleReconcileDrift)))
	mux.HandleFunc("POST /api/v1/governance/drift/bulk-attribute", withCORS(withOperatorAuth(handleBulkAttributeDrift)))
	mux.HandleFunc("POST /api/v1/governance/drift/{id}/attribute", withCORS(withOperatorAuth(handleAttributeDrift)))
	mux.HandleFunc("POST /api/v1/governance/drift/{id}/revoke", withCORS(withOperatorAuth(handleRevokeDrift)))
	mux.HandleFunc("POST /api/v1/governance/drift/{id}/mark-external", withCORS(withOperatorAuth(handleMarkDriftExternal)))
```
(Match the actual middleware names used by the phase-1 propagations routes — `withCORS`/`withOperatorAuth` or whatever `router.go` uses. Note the `/reconcile`, `/bulk-attribute` static segments must register before or unambiguously against `/{id}/...`; Go 1.22 ServeMux prefers more-specific patterns, so this ordering is safe.)

- [ ] **Step 16.5: Add the `Drift` block to the governance summary**

In `views.go:governanceFromSnapshot`, beside the `PendingPropagation` block:
```go
	driftCount, _ := svcCountPendingDrift(snap.ctx)
	topDrift, _ := svcGetTopDrift(snap.ctx, 3) // GetDriftItems with default pending filter, capped to 3
```
and add to the returned `GovernanceSummary`:
```go
		Drift: models.DriftSummary{Count: driftCount, Top: topDrift},
```
Add injectables in `views.go` deps: `svcCountPendingDrift = db.CountPendingDrift` and `svcGetTopDrift = func(ctx, n) ([]models.DriftItem, error) { items, err := db.GetDriftItems(ctx, db.DriftFilter{}); if err!=nil||len(items)<=n {return items,err}; return items[:n], nil }`.

- [ ] **Step 16.6: Run tests + vet + commit**

Run: `cd backend && go test ./internal/handlers/ -run 'Drift|Reconcile' -v && go test ./... && go vet ./...`
Expected: PASS, vet clean.
```bash
git add backend/internal/handlers/drift.go backend/internal/handlers/router.go backend/internal/handlers/deps.go backend/internal/handlers/drift_test.go backend/internal/services/views.go
git commit -m "feat(handlers): drift triage API (list/attribute/revoke/mark-external/bulk/reconcile) + governance drift block (B2)"
```

---

## Task 17 — Frontend Drift surfaces: nav, banner, callout, hooks (B2, red/undismissible)

**Files:**
- Create: `ui/src/lib/queries/useDrift.ts`
- Modify: `ui/src/lib/queries/useGovernance.ts` (add `drift` to `GovernanceSummary`)
- Modify: `ui/src/components/SidebarNav.tsx` (top-level `⚠ Drift` item + persistent red dot)
- Create: `ui/src/components/drift/DriftBanner.tsx` (sticky, undismissible)
- Create: `ui/src/components/drift/DriftCallout.tsx` (dashboard, undismissible, top-3)
- Modify: `ui/src/components/dashboard/AdminDashboard.tsx` (render `DriftCallout`)
- Modify: the admin layout that wraps pages (where the sticky banner mounts — likely `ui/src/app/(admin)/layout.tsx` or the `Sidebar` shell)
- Create: `ui/src/components/drift/DriftCallout.test.tsx`

- [ ] **Step 17.1: Extend the governance summary type + add drift hooks**

In `useGovernance.ts`, add to `GovernanceSummary`:
```typescript
export interface DriftItem {
  id: string;
  user_id: string;
  project_id: string;
  role_keys: string[];
  drift_type: string;
  detection_source: string;
  detected_at: string;
}
export interface DriftSummary { count: number; top?: DriftItem[]; }
```
add `drift: DriftSummary;` to `GovernanceSummary`, and default it in the mapper:
```typescript
        drift: data?.drift ?? { count: 0, top: [] },
```
Create `useDrift.ts` (mirror `usePropagation.ts`):
```typescript
"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { request } from "@/lib/api-client";
import { governanceQueryKeys, type DriftItem } from "./useGovernance";

const KEYS = { list: (f?: DriftFilter) => ["drift", "list", f ?? {}] as const };
export interface DriftFilter { user_id?: string; project_id?: string; source?: string; }

export function useDriftItems(filter?: DriftFilter) {
  const qs = new URLSearchParams(
    Object.entries(filter ?? {}).filter(([, v]) => v) as [string, string][],
  ).toString();
  return useQuery({
    queryKey: KEYS.list(filter),
    queryFn: async () => (await request<{ drift: DriftItem[] }>(`/governance/drift${qs ? `?${qs}` : ""}`)).drift ?? [],
  });
}

function useDriftMutation<B>(path: (id: string) => string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({ id, body }: { id: string; body?: B }) =>
      request(path(id), { method: "POST", body }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["drift"] });
      qc.invalidateQueries({ queryKey: governanceQueryKeys.summary });
    },
  });
}

export const useAttributeDrift = () => useDriftMutation<{ source: string; source_ref?: string }>((id) => `/governance/drift/${id}/attribute`);
export const useRevokeDrift = () => useDriftMutation<undefined>((id) => `/governance/drift/${id}/revoke`);
export const useMarkExternalDrift = () => useDriftMutation<{ reason?: string }>((id) => `/governance/drift/${id}/mark-external`);

export function useReconcileNow() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async () => request("/governance/drift/reconcile", { method: "POST" }),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ["drift"] }); qc.invalidateQueries({ queryKey: governanceQueryKeys.summary }); },
  });
}
```

- [ ] **Step 17.2: Sidebar — top-level `⚠ Drift` item + persistent red dot**

In `SidebarNav.tsx`: add `driftCount` state populated from `data?.drift?.count` in the same `fetch("/api/proxy/governance/summary")` effect. Insert a **top-level** section ABOVE Governance (per operational-readiness: "dedicated top-level `⚠ Drift` item above `/governance/*` with a persistent red dot"):
```typescript
    ...(driftCount > 0
      ? [{
          title: "",
          items: [{ href: "/governance/drift", label: "⚠ Drift", badge: driftCount }],
        }]
      : []),
```
Render the Drift item's badge with **error** tokens (`bg-error text-on-error`) and a persistent (non-animated) red dot — distinct from the amber `secondary` Pending badge. The dot persists while `driftCount > 0` (no dismiss).

- [ ] **Step 17.3: Sticky undismissible banner**

Create `DriftBanner.tsx`:
```tsx
"use client";
import Link from "next/link";
import { useGovernanceSummary } from "@/lib/queries/useGovernance";

/**
 * Sticky, UNDISMISSIBLE top banner shown on every admin page while drift exists.
 * Red (error tokens), breaks out of the normal in-layout flow — deliberately
 * louder than the amber, dismissible Pending callout. Slide-in motion is
 * suppressed under prefers-reduced-motion.
 */
export function DriftBanner() {
  const { data } = useGovernanceSummary();
  const count = data?.drift?.count ?? 0;
  if (count <= 0) return null;
  return (
    <div
      role="alert"
      className="sticky top-0 z-50 flex items-center justify-between gap-4 border-b border-error/50 bg-[color-mix(in_srgb,var(--error)_20%,transparent)] px-6 py-2 text-sm text-on-surface motion-safe:animate-slide-in-down"
    >
      <span>
        <span aria-hidden>⚠ </span>
        <strong>{count}</strong> drift {count === 1 ? "item" : "items"} detected — out-of-band changes need triage
      </span>
      <Link href="/governance/drift" className="font-medium text-error hover:underline">Review →</Link>
    </div>
  );
}
```
Mount `<DriftBanner />` at the top of the admin layout shell (the component that wraps all `/` admin pages — find where `<SidebarNav />` / the admin `<main>` renders and place the banner above page content so it's on every admin page). Add the `slide-in-down` keyframe to `globals.css` if absent (guard with `motion-safe:`).

- [ ] **Step 17.4: Write the failing dashboard-callout test**

```tsx
// DriftCallout.test.tsx
// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { DriftCallout } from "./DriftCallout";

const item = (id: string) => ({ id, user_id: "u", project_id: "p", role_keys: ["r"], drift_type: "zitadel_only", detection_source: "webhook", detected_at: "2026-07-06T00:00:00Z" });

describe("DriftCallout", () => {
  it("renders count, a top-3 preview, and Triage all when drift exists", () => {
    render(<DriftCallout count={5} top={[item("1"), item("2"), item("3")]} />);
    expect(screen.getByRole("alert")).toHaveTextContent(/5/);
    expect(screen.getByRole("link", { name: /triage all/i })).toBeInTheDocument();
  });
  it("renders nothing when count is 0", () => {
    const { container } = render(<DriftCallout count={0} top={[]} />);
    expect(container).toBeEmptyDOMElement();
  });
  it("has no dismiss control (undismissible)", () => {
    render(<DriftCallout count={2} top={[item("1")]} />);
    expect(screen.queryByRole("button", { name: /dismiss/i })).not.toBeInTheDocument();
  });
});
```

- [ ] **Step 17.5: Implement `DriftCallout.tsx` + wire into `AdminDashboard`**

```tsx
"use client";
import Link from "next/link";
import type { DriftItem } from "@/lib/queries/useGovernance";
import { UserName } from "@/components/names/UserName";
import { ProjectName } from "@/components/names/ProjectName";

interface Props { count: number; top: DriftItem[]; }

/**
 * Full-width, UNDISMISSIBLE dashboard callout above the stat grid. Red, with a
 * top-3 preview and "Triage all →". No dismiss control — drift must not be
 * silenced. Contrast with the dismissible amber PendingCallout.
 */
export function DriftCallout({ count, top }: Props) {
  if (count <= 0) return null;
  return (
    <div
      role="alert"
      className="rounded-card border border-error/50 bg-[color-mix(in_srgb,var(--error)_15%,transparent)] px-5 py-4"
    >
      <div className="flex items-center justify-between gap-4">
        <span className="text-on-surface">
          <span aria-hidden>⚠ </span>
          <strong>{count}</strong> out-of-band {count === 1 ? "change needs" : "changes need"} triage
        </span>
        <Link href="/governance/drift" className="font-medium text-error hover:underline">Triage all →</Link>
      </div>
      <ul className="mt-3 space-y-1 text-sm text-on-surface-variant">
        {top.slice(0, 3).map((d) => (
          <li key={d.id}>
            <UserName id={d.user_id} /> · <ProjectName id={d.project_id} /> · {d.role_keys.join(", ")}
            <span className="ml-2 text-xs opacity-70">({d.drift_type})</span>
          </li>
        ))}
      </ul>
    </div>
  );
}
```
In `AdminDashboard.tsx`, render `<DriftCallout count={drift.count} top={drift.top ?? []} />` ABOVE the `<PendingCallout .../>` (drift is louder / higher), reading `const drift = summary.governance.data?.drift ?? { count: 0, top: [] };`. No dismiss state (undismissible).

- [ ] **Step 17.6: Run UI tests + lint + build; commit**

Run: `cd ui && bun run test && bun run lint && bun run build`
Expected: PASS.
```bash
git add ui/src/lib/queries/useDrift.ts ui/src/lib/queries/useGovernance.ts ui/src/components/SidebarNav.tsx ui/src/components/drift/ ui/src/components/dashboard/AdminDashboard.tsx ui/src/app
git commit -m "feat(ui): drift nav item, sticky banner, dashboard callout, hooks (red/undismissible)"
```

---

## Task 18 (was UI cont.) — `/governance/drift` triage page + chime, then verification gate

This task carries two deliverables that a reviewer gates together: the triage page (three actions + bulk + filters + Reconcile now + optional chime) and the sub-phase verification gate.

**Files:**
- Create: `ui/src/app/governance/drift/page.tsx` (server-gated, mirror `governance/pending/page.tsx`)
- Create: `ui/src/components/drift/DriftTriageClient.tsx`
- Create: `ui/src/lib/driftChime.ts` (localStorage toggle, mirror `theme.tsx`)
- Create: `ui/src/components/drift/DriftTriageClient.test.tsx`

- [ ] **Step 18.1: Server-gated page**

```tsx
// ui/src/app/governance/drift/page.tsx
import { redirect } from "next/navigation";
import { DriftTriageClient } from "@/components/drift/DriftTriageClient";
import { getSession } from "@/lib/session";

export default async function DriftPage() {
  const session = await getSession();
  if (!session) redirect("/login");
  if (session.role !== "admin") redirect("/");
  return <DriftTriageClient />;
}
```

- [ ] **Step 18.2: Triage client (three actions + bulk + filters + Reconcile now)**

Implement `DriftTriageClient.tsx` mirroring `PendingPropagationsClient.tsx` structure (Eyebrow/header/Card list), but with:
- Filters (user/project/source) driving `useDriftItems(filter)`.
- `[Reconcile now]` button → `useReconcileNow().mutate()`.
- Per-row action buttons: **Attribute** (opens a small inline form: source select `external_backfill|bundle|rule` + optional `source_ref`; when `source=bundle`, disable bundles whose roles don't include the drift role — design §7 Q6), **Revoke** (`useRevokeDrift().mutate({id})`), **Mark external** (`useMarkExternalDrift().mutate({id, body:{reason}})`).
- **Bulk attribute**: multi-select rows + one Attribute call to `/governance/drift/bulk-attribute` (bootstrap case).
- Red/error visual treatment; new-row highlight + count-up under `motion-safe:` only.

Keep the component focused; extract the per-row action menu into a small `DriftRowActions` sub-component if it grows past ~120 lines.

- [ ] **Step 18.3: Chime toggle (localStorage, avatar-menu)**

Create `driftChime.ts` mirroring `theme.tsx`'s localStorage pattern:
```typescript
const STORAGE_KEY = "syndra-drift-chime";
export function isChimeEnabled(): boolean {
  if (typeof window === "undefined") return true;
  return localStorage.getItem(STORAGE_KEY) !== "off"; // default on
}
export function setChimeEnabled(on: boolean) {
  if (typeof window !== "undefined") localStorage.setItem(STORAGE_KEY, on ? "on" : "off");
}
```
Play a short WebAudio beep (no asset dependency — `AudioContext` oscillator, ~120ms) when the drift count transitions upward AND `isChimeEnabled()` AND `!prefers-reduced-motion`. Fire it from a small effect in the layout/banner that watches `drift.count`. First play shows a one-time tooltip explaining the cue (a `localStorage` "seen" flag). Add a toggle to the avatar-menu popover beside the existing theme control. `ponytail:` a synthesized beep avoids shipping/inlining an audio asset; swap for a real chime file only if the beep reads as too utilitarian.

- [ ] **Step 18.4: Write + run a triage-client test**

```tsx
// DriftTriageClient.test.tsx — @vitest-environment jsdom
// Render inside a QueryClientProvider (mirror how other query-backed components are tested).
// Assert: rows render; clicking Revoke calls the revoke mutation with the row id;
// attribute-to-bundle disables a bundle lacking the drift role.
```
Run: `cd ui && bun run test`
Expected: PASS.

- [ ] **Step 18.5: Backend full suite + vet**

Run: `cd backend && go test ./... && go vet ./...`
Expected: all PASS, no vet warnings.

- [ ] **Step 18.6: UI gate**

Run: `cd ui && bun run lint && bun run test && bun run build`
Expected: all PASS.

- [ ] **Step 18.7: Migration round-trip for `000016`**

Run (throwaway DB, or static validation note as in Task 11.3):
```bash
cd backend
migrate -path db/migrations -database "$DB_DSN" up
migrate -path db/migrations -database "$DB_DSN" down 1
migrate -path db/migrations -database "$DB_DSN" up
```
Expected: clean up→down→up; `drift_items` + `external_grant_exclusions` present after final `up`.

- [ ] **Step 18.8: gofmt scoped to the backend touch set**

Run:
```bash
cd backend && gofmt -d \
  internal/db/drift.go internal/db/exclusions.go internal/db/drift_migration_test.go internal/db/propagations.go \
  internal/models/models.go internal/handlers/reconciliation.go internal/handlers/drift.go \
  internal/handlers/deps.go internal/handlers/router.go internal/handlers/webhook.go \
  internal/services/expected.go internal/services/views.go internal/services/drift/*.go cmd/api/main.go
```
Expected: zero diff.

- [ ] **Step 18.9: Codebase-memory refresh**

```
mcp__codebase-memory-mcp__detect_changes
mcp__codebase-memory-mcp__index_repository   # scope: backend/internal/{db,handlers,services,services/drift}, ui/src
```

- [ ] **Step 18.10: OpenSpec validate + tick the ledger**

```bash
cd /Users/notkanishk/Documents/Mkrspc/Projects/Syndra
openspec validate wave-2-part-4-zitadel-state-projection-and-drift-control --strict
```
Then check off Sub-phase 2 Tasks 11–18 in `tasks.md` (append per-task notes as sub-phase 1 did) and commit:
```bash
git add openspec/changes/wave-2-part-4-zitadel-state-projection-and-drift-control/tasks.md
git commit -m "chore(openspec): tick wave-2-part-4 sub-phase 2 (drift) tasks complete"
```

---

## Self-review checklist (run after implementation, before requesting review)

1. **Detection completeness:** `zitadel_only` drift arrives both real-time (webhook `grant_added`, Task 15) and via the scheduled sweep (Task 14); `syndra_only` direct grants re-enqueue through the sub-phase-1 outbox (Task 14). C6 overlay-cache misses are backstopped by the sweep — documented, not separately engineered.
2. **No false drift:** rule-derived grants classify `expected_via_rule` (Task 13 + sweep), excluded triples are silently filtered (Tasks 13/14/15), and Syndra's own mutations never reach the webhook (self-mutation guard). Bundle/rule-derived roles ABSENT from Zitadel are NOT `syndra_only` drift in sub-phase 2 (cascade projection is sub-phase 3).
3. **No auto-resolution:** every `drift_items` row needs explicit Attribute / Revoke / Mark-external. The only automatic action is the `syndra_only` re-enqueue (design-mandated missed-webhook replay), and it skips the specific role already under triage (`HasPendingDrift`, role-scoped so an unrelated missing role is not suppressed).
4. **Dedup at role granularity:** the partial-unique index and `UpsertDriftItem` `ON CONFLICT` key on `(user, project, drift_type, role_keys)` — one single-role row per drifting role, so a 2nd role on the same pair is never swallowed; resolved rows leave the index so a role can legitimately re-drift.
4a. **Triage is atomic + race-safe (no recovery gap):** each action resolves the drift row AND writes its side effect (outbox row / exclusions) in ONE transaction — `db.AttributeDriftAndEnqueue` / `RevokeDriftAndEnqueue` / `MarkDriftExternalTx`, all composing phase-1's `enqueueWrites` seam behind a guarded `UPDATE … WHERE status='pending_triage'`. A lost concurrent-triage race returns `409` with the whole tx rolled back (nothing written); a side-effect write failure rolls the resolution back too, so the drift never leaves the triage queue without its durable side effect. Revoke's inline drain is best-effort AFTER commit. Rule/exclusion lookup failures propagate (endpoint 500 / sweep abort) rather than degrading to an empty set and flagging false drift.
5. **Reconciliation right-sized (B2):** cap is 2 000 everywhere (endpoint + sweep); the sweep is scheduled (`DRIFT_RECONCILIATION_INTERVAL_HOURS`, default 6) and on-demand (`POST …/drift/reconcile`).
6. **Urgency tiers:** Drift is red + undismissible + breaks out of layout (top-level nav + persistent red dot, sticky banner, undismissible dashboard callout); Pending stays amber + dismissible + in-layout. Motion `motion-safe:`-gated; chime `localStorage`-gated, default on.
7. **Type consistency:** `drift_type ∈ {zitadel_only, syndra_only}`, `detection_source ∈ {webhook, reconciliation_sweep}`, `status ∈ {pending_triage, attributed, revoked, marked_external}` — identical between the `000016` CHECKs (Task 11), the migration-coherence guard (Task 12), the repositories, the sweep/webhook, and the TS `DriftItem`. `DriftResult` field names match between Go and any TS that reads the reconcile response.
8. **Reuse over reinvention:** attribute/revoke reuse phase-1's `EnqueueDirectGrantPropagation` + `DrainOne`; the sweep + endpoint share `services` classification helpers; the drift scheduler mirrors `expiry.Scheduler`; the drift UI mirrors the pending UI. No package extraction, no import cycles (handlers→services/drift only, matching handlers→services/propagation).
9. **Scope discipline:** no `confirmation_mode`, no cascade enqueueing, no bundle/rule projection — those are sub-phase 3.

---

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-07-06-wave-2-part-4-phase-2-drift.md`. Two execution options:

1. **Subagent-Driven (recommended)** — dispatch a fresh subagent per task, review between tasks (`superpowers:subagent-driven-development`).
2. **Inline Execution** — execute tasks in this session with checkpoints (`superpowers:executing-plans`).
