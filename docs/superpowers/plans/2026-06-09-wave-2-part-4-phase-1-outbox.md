# Wave 2 · Part 4 — Sub-phase 1: Outbox & Operator-Confirmed Propagation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make every Syndra-mediated Zitadel grant mutation flow through a durable intent-ledger + outbox buffer that the operator drains explicitly — closing the B4/D3 "single mutation authority" contradiction without changing observable success semantics for the operator.

**Architecture:** A new `pending_zitadel_propagations` outbox table (mirroring the existing `provisioning_intents` idempotency/claim pattern) plus `source`/`source_ref` columns on `direct_role_grants`. A transactional enqueue (`db.EnqueueDirectGrantPropagation`) writes ledger + audit + outbox atomically; an operator-triggered drain (`services/propagation`, mirroring `services/expiry`) calls Zitadel and classifies the ACK. `applied` (synchronous 2xx) is terminal success — there is no webhook `confirmed` state, because the existing self-mutation guard drops Syndra's own grant events (see `design.md` Decision 1). Both `/api/v1/users/{id}/grants` and the legacy `/api/v1/zitadel/*` CRUD converge on this one path.

**Tech Stack:** Go (`pgx/v5` pool + transactions, stdlib `testing`), PostgreSQL (golang-migrate), Next.js + TypeScript + React Query (Bun). Material (obsidian-clarity) tokens for the amber Pending UI.

**Scope note:** This plan covers **Sub-phase 1 only**. Drift detection/triage (sub-phase 2) and bundle/rule cascade projection (sub-phase 3) are task-level in `openspec/changes/wave-2-part-4-zitadel-state-projection-and-drift-control/tasks.md` and get their own writing-plans pass when this sub-phase lands.

---

## OpenSpec change scope

- `openspec/changes/wave-2-part-4-zitadel-state-projection-and-drift-control/proposal.md`
- `openspec/changes/wave-2-part-4-zitadel-state-projection-and-drift-control/design.md`
- `openspec/changes/wave-2-part-4-zitadel-state-projection-and-drift-control/tasks.md`
- `openspec/changes/wave-2-part-4-zitadel-state-projection-and-drift-control/specs/{access-governance,automation-policies,operational-readiness}/spec.md`

---

## Reference: current code this plan touches

| Symbol | File:line | Current shape |
|---|---|---|
| `handleUpsertUserDirectGrant` | `backend/internal/handlers/access.go:62-106` | writes `direct_role_grants`, no Zitadel call, returns `{id,message}` |
| `UpsertDirectGrantRequest` | `backend/internal/handlers/access.go:15-24` | `{ProjectID, RoleKey, GrantedBy, Reason, DurationDays}` |
| `handleAssignZitadelGrant` etc. | `backend/internal/handlers/discovery.go:218-282` | direct `zitadelAddUserGrant`/`Update`/`Remove`, returns `{status}` |
| `zitadelAddUserGrant`/`Update`/`Remove` | `backend/internal/handlers/deps.go:147-164` | injectable closures over `zitadel.MgmtClient` |
| `UpsertDirectGrant` | `backend/internal/db/grants.go:15-32` | `INSERT … ON CONFLICT … RETURNING id` |
| `DirectGrant` model | `backend/internal/models/models.go:173-182` | no `source` |
| `InsertProvisioningIntent` / `ClaimPendingIntents` | `backend/internal/db/intents.go` | the pattern the outbox mirrors |
| `GetGrantIndex` / index table | `backend/internal/db/webhooks.go:174-224` | `zitadel_grants_index(grant_id,user_id,project_id,role_keys)` |
| `Scheduler` | `backend/internal/services/expiry/scheduler.go` | ticker + `Done()` graceful shutdown (drain/sweep template) |
| `ZitadelClient` iface | `backend/internal/zitadel/orchestrator.go:60-72` | `AddUserGrant`/`UpdateUserGrant`/`RemoveUserGrant`/`ListUserGrants`/`ListAllGrants` |
| migrations | `backend/db/migrations/` | highest is `000014`; next is `000015` |
| `useGovernanceSummary` | `ui/src/lib/queries/useGovernance.ts:31-43` | React Query over `/governance/summary` |
| `SidebarNav` | `ui/src/components/SidebarNav.tsx:56-92,117-121` | `NavSection[]` + `Badge` |
| grant mutations UI | `ui/src/app/zitadel/page.tsx:688-735` | `apiSend("POST", "zitadel/users/{id}/grants", …)` |

---

## Task 0 — Scaffolding baseline commit

Goal: land the OpenSpec scaffolding + this plan before touching source, so subsequent commits are clean.

**Files:**
- Create: `openspec/changes/wave-2-part-4-zitadel-state-projection-and-drift-control/{proposal,design,tasks}.md`
- Create: `…/specs/{access-governance,automation-policies,operational-readiness}/spec.md`
- Create: `docs/superpowers/plans/2026-06-09-wave-2-part-4-phase-1-outbox.md` (this file)

- [ ] **Step 0.1: Validate the change shape**

Run:
```bash
cd /Users/notkanishk/Documents/Mkrspc/Projects/Syndra
openspec validate wave-2-part-4-zitadel-state-projection-and-drift-control --strict
```
Expected: `Change '…' is valid`.

- [ ] **Step 0.2: Commit scaffolding only**

```bash
git checkout -b wave-2-part-4-outbox
git add openspec/changes/wave-2-part-4-zitadel-state-projection-and-drift-control docs/superpowers/plans/2026-06-09-wave-2-part-4-phase-1-outbox.md
git commit -m "docs(openspec): scaffold wave-2-part-4 zitadel state projection & drift control + sub-phase 1 plan"
```

---

## Task 1 — Migration `000015`: outbox table + `direct_role_grants` source columns

**Files:**
- Create: `backend/db/migrations/000015_zitadel_propagation_outbox.up.sql`
- Create: `backend/db/migrations/000015_zitadel_propagation_outbox.down.sql`

- [ ] **Step 1.1: Write the up migration**

```sql
-- 000015_zitadel_propagation_outbox.up.sql
-- Wave 2 · Part 4 (B4/D3): the outbox buffer for Syndra-mediated Zitadel grant
-- mutations, plus source attribution on direct_role_grants. The full 5-value
-- source enum is installed now so sub-phase 3 (cascade) needs no further ALTER.
-- `applied` is terminal success; there is no `confirmed` state (design Decision 1).

CREATE TABLE IF NOT EXISTS pending_zitadel_propagations (
    id               UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    op_type          TEXT NOT NULL CHECK (op_type IN ('add', 'revoke', 'replace')),
    user_id          VARCHAR(255) NOT NULL,
    project_id       VARCHAR(255) NOT NULL,
    role_keys        TEXT[] NOT NULL,
    zitadel_grant_id TEXT,
    payload_json     JSONB NOT NULL,
    idempotency_key  UUID NOT NULL UNIQUE,
    status           TEXT NOT NULL DEFAULT 'pending'
                         CHECK (status IN ('pending', 'in_flight', 'applied', 'failed')),
    attempts         INT NOT NULL DEFAULT 0,
    last_error       TEXT,
    initiated_by     VARCHAR(255) NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at       TIMESTAMPTZ,
    completed_at     TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_pending_zitadel_propagations_status
    ON pending_zitadel_propagations(status, created_at);

ALTER TABLE direct_role_grants
    ADD COLUMN IF NOT EXISTS source TEXT NOT NULL DEFAULT 'direct'
        CHECK (source IN ('direct', 'bundle', 'rule', 'external_backfill', 'lifecycle_cascade'));

ALTER TABLE direct_role_grants
    ADD COLUMN IF NOT EXISTS source_ref TEXT;
```

- [ ] **Step 1.2: Write the down migration**

```sql
-- 000015_zitadel_propagation_outbox.down.sql
-- Reverses 000015. Outbox rows are workflow state, not historical record;
-- dropping the table loses only un-drained buffer entries (re-creatable by the
-- operator). source/source_ref backfill to 'direct'/NULL on restore is lossless
-- for sub-phase-1 data (all rows are source='direct' until sub-phase 3 ships).

ALTER TABLE direct_role_grants DROP COLUMN IF EXISTS source_ref;
ALTER TABLE direct_role_grants DROP COLUMN IF EXISTS source;
DROP TABLE IF EXISTS pending_zitadel_propagations;
```

- [ ] **Step 1.3: Verify migration applies (round-trip)**

Run (requires a throwaway Postgres reachable via `DB_DSN`; if none is available in the implementation environment, statically validate file naming + SQL and note it, as Wave 2 · Part 3 Task 12 did):
```bash
cd backend
migrate -path db/migrations -database "$DB_DSN" up
migrate -path db/migrations -database "$DB_DSN" down 1
migrate -path db/migrations -database "$DB_DSN" up
```
Expected: no error; `\d pending_zitadel_propagations` shows the table after the final `up`.

- [ ] **Step 1.4: Commit**

```bash
git add backend/db/migrations/000015_zitadel_propagation_outbox.*.sql
git commit -m "feat(db): add zitadel propagation outbox + direct_role_grants source attribution (000015)"
```

---

## Task 2 — Models

**Files:**
- Modify: `backend/internal/models/models.go` (add `PendingPropagation`; extend `DirectGrant`)

- [ ] **Step 2.1: Add the `PendingPropagation` struct and extend `DirectGrant`**

Add near the other access structs in `models.go`:
```go
// PendingPropagation is one buffered Syndra-mediated Zitadel grant mutation.
// `applied` is terminal success (synchronous 2xx); there is no `confirmed` state.
type PendingPropagation struct {
	ID             string     `json:"id"`
	OpType         string     `json:"op_type"` // add | revoke | replace
	UserID         string     `json:"user_id"`
	ProjectID      string     `json:"project_id"`
	RoleKeys       []string   `json:"role_keys"`
	ZitadelGrantID string     `json:"zitadel_grant_id,omitempty"`
	Status         string     `json:"status"` // pending | in_flight | applied | failed
	Attempts       int        `json:"attempts"`
	LastError      string     `json:"last_error,omitempty"`
	InitiatedBy    string     `json:"initiated_by"`
	CreatedAt      time.Time  `json:"created_at"`
	StartedAt      *time.Time `json:"started_at,omitempty"`
	CompletedAt    *time.Time `json:"completed_at,omitempty"`
}
```

Extend `DirectGrant` (`models.go:173-182`) with two trailing fields:
```go
	Source    string     `json:"source"`               // direct | bundle | rule | external_backfill | lifecycle_cascade
	SourceRef string     `json:"source_ref,omitempty"` // bundle_id / rule_id when source ∈ {bundle, rule}
```

- [ ] **Step 2.2: Build to confirm it compiles**

Run: `cd backend && go build ./internal/models/`
Expected: no output (success).

- [ ] **Step 2.3: Commit**

```bash
git add backend/internal/models/models.go
git commit -m "feat(models): PendingPropagation + DirectGrant source attribution"
```

---

## Task 3 — `db/propagations.go` repository

**Files:**
- Create: `backend/internal/db/propagations.go`
- Create: `backend/internal/db/propagations_test.go`

This file follows the `db` package idiom exactly: package-level `PG *pgxpool.Pool`, `ctx` first arg, `fmt.Errorf("…: %w", err)` wrapping. It mirrors `intents.go`'s claim-and-process pattern.

- [ ] **Step 3.1: Write the failing test for claim semantics**

```go
// propagations_test.go
package db

import (
	"context"
	"testing"
)

// These tests run against a live test Postgres (same harness intents_test.go uses).
// Skip cleanly when DB_DSN is unset so `go test ./...` stays green in CI-less envs.
func TestClaimPendingPropagations_TransitionsToInFlight(t *testing.T) {
	ctx := requireTestDB(t) // helper that skips if PG == nil; see intents_test.go
	id := mustInsertPropagation(t, ctx, "add", "u1", "p1", []string{"r1"})

	claimed, err := ClaimPendingPropagations(ctx, 10)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if len(claimed) != 1 || claimed[0].ID != id {
		t.Fatalf("want 1 claimed row %s, got %+v", id, claimed)
	}
	if claimed[0].Status != "in_flight" {
		t.Fatalf("want status in_flight, got %s", claimed[0].Status)
	}
	// A second claim must not re-return the in_flight row by default selection.
	again, _ := ClaimPendingPropagations(ctx, 10)
	for _, r := range again {
		if r.ID == id && r.Status == "pending" {
			t.Fatalf("row %s should not be re-claimable as pending", id)
		}
	}
}
```

(Use the same `requireTestDB` / insert-helper conventions already present in `intents_test.go`; if that helper is named differently, match it. Add `mustInsertPropagation` as a local helper that calls `InsertPendingPropagation`.)

- [ ] **Step 3.2: Run it — expect compile failure (functions undefined)**

Run: `cd backend && go test ./internal/db/ -run TestClaimPendingPropagations -v`
Expected: FAIL — `undefined: ClaimPendingPropagations` / `InsertPendingPropagation`.

- [ ] **Step 3.3: Implement `propagations.go`**

```go
// propagations.go
package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"syndra/internal/models" // match the module path used elsewhere in db/*.go
)

// InsertPendingPropagation inserts one outbox row and returns its id. Used both
// by the transactional enqueue (via the tx variant below) and by the drift
// re-enqueue path (sub-phase 2). idempotencyKey must be a fresh UUID string.
func InsertPendingPropagation(ctx context.Context, opType, userID, projectID string,
	roleKeys []string, zitadelGrantID, payloadJSON, idempotencyKey, initiatedBy string) (string, error) {
	const q = `
		INSERT INTO pending_zitadel_propagations
			(op_type, user_id, project_id, role_keys, zitadel_grant_id, payload_json, idempotency_key, initiated_by)
		VALUES ($1,$2,$3,$4,NULLIF($5,''),$6,$7,$8)
		RETURNING id`
	var id string
	if err := PG.QueryRow(ctx, q, opType, userID, projectID, roleKeys, zitadelGrantID,
		payloadJSON, idempotencyKey, initiatedBy).Scan(&id); err != nil {
		return "", fmt.Errorf("insert propagation: %w", err)
	}
	return id, nil
}

// ClaimPendingPropagations atomically transitions up to `limit` pending rows to
// in_flight and returns them in created_at order. FOR UPDATE SKIP LOCKED makes
// concurrent drains safe (mirrors ClaimPendingIntents).
func ClaimPendingPropagations(ctx context.Context, limit int) ([]models.PendingPropagation, error) {
	const q = `
		WITH claimed AS (
			SELECT id FROM pending_zitadel_propagations
			WHERE status = 'pending'
			ORDER BY created_at
			LIMIT $1
			FOR UPDATE SKIP LOCKED
		)
		UPDATE pending_zitadel_propagations p
		SET status = 'in_flight', started_at = NOW()
		FROM claimed
		WHERE p.id = claimed.id
		RETURNING p.id, p.op_type, p.user_id, p.project_id, p.role_keys,
		          COALESCE(p.zitadel_grant_id,''), p.status, p.attempts,
		          COALESCE(p.last_error,''), p.initiated_by, p.created_at, p.started_at, p.completed_at`
	rows, err := PG.Query(ctx, q, limit)
	if err != nil {
		return nil, fmt.Errorf("claim propagations: %w", err)
	}
	defer rows.Close()
	return scanPropagations(rows)
}

func MarkPropagationApplied(ctx context.Context, id string) error {
	return execPropagation(ctx, id,
		`UPDATE pending_zitadel_propagations SET status='applied', completed_at=NOW(), last_error=NULL WHERE id=$1`)
}

func MarkPropagationFailed(ctx context.Context, id, errMsg string) error {
	const q = `UPDATE pending_zitadel_propagations
		SET status='failed', completed_at=NOW(), last_error=$2 WHERE id=$1`
	if _, err := PG.Exec(ctx, q, id, errMsg); err != nil {
		return fmt.Errorf("mark propagation failed: %w", err)
	}
	return nil
}

// RequeuePropagation returns a row to pending after a transient error and bumps
// attempts. Caller decides (via attempts vs OUTBOX_MAX_RETRIES) whether to halt.
func RequeuePropagation(ctx context.Context, id, errMsg string) (int, error) {
	const q = `UPDATE pending_zitadel_propagations
		SET status='pending', attempts=attempts+1, last_error=$2, started_at=NULL
		WHERE id=$1 RETURNING attempts`
	var attempts int
	if err := PG.QueryRow(ctx, q, id, errMsg).Scan(&attempts); err != nil {
		return 0, fmt.Errorf("requeue propagation: %w", err)
	}
	return attempts, nil
}

func GetPendingPropagations(ctx context.Context) ([]models.PendingPropagation, error) {
	const q = `
		SELECT id, op_type, user_id, project_id, role_keys, COALESCE(zitadel_grant_id,''),
		       status, attempts, COALESCE(last_error,''), initiated_by, created_at, started_at, completed_at
		FROM pending_zitadel_propagations
		WHERE status IN ('pending','in_flight')
		ORDER BY created_at`
	rows, err := PG.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("get pending propagations: %w", err)
	}
	defer rows.Close()
	return scanPropagations(rows)
}

func CountPendingPropagations(ctx context.Context) (int, error) {
	const q = `SELECT COUNT(*) FROM pending_zitadel_propagations WHERE status IN ('pending','in_flight')`
	var n int
	if err := PG.QueryRow(ctx, q).Scan(&n); err != nil {
		return 0, fmt.Errorf("count pending propagations: %w", err)
	}
	return n, nil
}

// PruneTerminalPropagations deletes applied/failed rows older than retentionDays.
// The outbox is ephemeral workflow state — canonical intent lives in
// direct_role_grants — so terminal rows are safe to drop after the window.
// `failed` rows are kept the full window as the audit trail of attention-needing
// mutations. Returns the number of rows pruned.
func PruneTerminalPropagations(ctx context.Context, retentionDays int) (int64, error) {
	const q = `DELETE FROM pending_zitadel_propagations
		WHERE status IN ('applied','failed')
		  AND completed_at IS NOT NULL
		  AND completed_at < NOW() - ($1 || ' days')::interval`
	tag, err := PG.Exec(ctx, q, retentionDays)
	if err != nil {
		return 0, fmt.Errorf("prune terminal propagations: %w", err)
	}
	return tag.RowsAffected(), nil
}

func execPropagation(ctx context.Context, id, q string) error {
	if _, err := PG.Exec(ctx, q, id); err != nil {
		return fmt.Errorf("update propagation %s: %w", id, err)
	}
	return nil
}

func scanPropagations(rows pgx.Rows) ([]models.PendingPropagation, error) {
	var out []models.PendingPropagation
	for rows.Next() {
		var p models.PendingPropagation
		if err := rows.Scan(&p.ID, &p.OpType, &p.UserID, &p.ProjectID, &p.RoleKeys,
			&p.ZitadelGrantID, &p.Status, &p.Attempts, &p.LastError, &p.InitiatedBy,
			&p.CreatedAt, &p.StartedAt, &p.CompletedAt); err != nil {
			return nil, fmt.Errorf("scan propagation: %w", err)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

var errNoPropagationRow = errors.New("no propagation row")
```

- [ ] **Step 3.4: Run the test — expect PASS (or SKIP without DB)**

Run: `cd backend && go test ./internal/db/ -run TestClaimPendingPropagations -v`
Expected: PASS against a live test DB; SKIP message when `DB_DSN` is unset. Either is acceptable for this step; the wave gate (Task 10) runs the full suite.

- [ ] **Step 3.5: Commit**

```bash
git add backend/internal/db/propagations.go backend/internal/db/propagations_test.go
git commit -m "feat(db): pending_zitadel_propagations repository (insert/claim/transition)"
```

---

## Task 4 — Transactional enqueue `db.EnqueueDirectGrantPropagation`

**Files:**
- Create: `backend/internal/db/propagation_enqueue.go`
- Create: `backend/internal/db/propagation_enqueue_test.go`

- [ ] **Step 4.1: Write the failing rollback test**

```go
// propagation_enqueue_test.go
package db

import (
	"context"
	"testing"
)

// Enqueue must be all-or-nothing: if the outbox insert fails, the direct-grant
// upsert and audit insert must roll back too.
func TestEnqueueDirectGrantPropagation_RollsBackOnOutboxFailure(t *testing.T) {
	ctx := requireTestDB(t)
	// Force the outbox insert to fail by supplying a duplicate idempotency key.
	dupKey := mustReserveIdempotencyKey(t, ctx) // inserts a row with a known key
	_, err := enqueueWithFixedKey(ctx, EnqueueParams{
		UserID: "rollback-user", ProjectID: "p1", RoleKeys: []string{"r1"},
		GrantedBy: "op", Reason: "x", OpType: "add", PayloadJSON: "{}",
	}, dupKey)
	if err == nil {
		t.Fatal("expected enqueue to fail on duplicate idempotency key")
	}
	if grantExists(t, ctx, "rollback-user", "p1", "r1") {
		t.Fatal("direct_role_grants row must not survive a rolled-back enqueue")
	}
}
```

(`enqueueWithFixedKey` is a test-only seam that injects a fixed key; in production the key is generated. Implement it in the test file by calling the same tx body with a provided key, or expose an unexported `enqueueTx(ctx, params, key)` that the public function wraps — see Step 4.2.)

- [ ] **Step 4.2: Implement the enqueue**

```go
// propagation_enqueue.go
package db

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type EnqueueParams struct {
	UserID      string
	ProjectID   string
	RoleKeys    []string
	GrantedBy   string
	Reason      string
	ExpiresAt   *time.Time
	Source      string // defaults to "direct" when empty
	SourceRef   string
	OpType      string // add | revoke | replace
	ZitadelGrantID string
	PayloadJSON string
}

type EnqueueResult struct {
	OutboxID       string `json:"outbox_id"`
	IdempotencyKey string `json:"idempotency_key"`
	Status         string `json:"status"`
}

// EnqueueDirectGrantPropagation writes the intent ledger rows (one direct_role_grants
// row per role), one audit row, and one outbox row in a single transaction, then
// returns the outbox handle. The Zitadel call happens later, during the drain.
func EnqueueDirectGrantPropagation(ctx context.Context, p EnqueueParams) (EnqueueResult, error) {
	return enqueueTx(ctx, p, uuid.NewString())
}

func enqueueTx(ctx context.Context, p EnqueueParams, key string) (EnqueueResult, error) {
	source := p.Source
	if source == "" {
		source = "direct"
	}
	tx, err := PG.Begin(ctx)
	if err != nil {
		return EnqueueResult{}, fmt.Errorf("begin enqueue tx: %w", err)
	}
	defer tx.Rollback(ctx) // no-op after a successful Commit

	const upsertGrant = `
		INSERT INTO direct_role_grants
			(user_id, zitadel_project_id, zitadel_role_key, granted_by, reason, expires_at, source, source_ref)
		VALUES ($1,$2,$3,$4,$5,$6,$7,NULLIF($8,''))
		ON CONFLICT (user_id, zitadel_project_id, zitadel_role_key)
		DO UPDATE SET granted_by=EXCLUDED.granted_by, reason=EXCLUDED.reason,
		              expires_at=EXCLUDED.expires_at, source=EXCLUDED.source,
		              source_ref=EXCLUDED.source_ref, updated_at=CURRENT_TIMESTAMP
		RETURNING id`
	var firstGrantID string
	for i, role := range p.RoleKeys {
		var id string
		if err := tx.QueryRow(ctx, upsertGrant, p.UserID, p.ProjectID, role,
			p.GrantedBy, p.Reason, p.ExpiresAt, source, p.SourceRef).Scan(&id); err != nil {
			return EnqueueResult{}, fmt.Errorf("upsert direct grant (%s): %w", role, err)
		}
		if i == 0 {
			firstGrantID = id
		}
	}

	const insertAudit = `INSERT INTO audit_logs
		(actor_zitadel_user_id, target_zitadel_user_id, action, resource_id) VALUES ($1,$2,$3,$4)`
	if _, err := tx.Exec(ctx, insertAudit, p.GrantedBy, p.UserID, "direct_grant.upserted", firstGrantID); err != nil {
		return EnqueueResult{}, fmt.Errorf("insert audit: %w", err)
	}

	const insertOutbox = `
		INSERT INTO pending_zitadel_propagations
			(op_type, user_id, project_id, role_keys, zitadel_grant_id, payload_json, idempotency_key, initiated_by)
		VALUES ($1,$2,$3,$4,NULLIF($5,''),$6,$7,$8)
		RETURNING id`
	var outboxID string
	if err := tx.QueryRow(ctx, insertOutbox, p.OpType, p.UserID, p.ProjectID, p.RoleKeys,
		p.ZitadelGrantID, p.PayloadJSON, key, p.GrantedBy).Scan(&outboxID); err != nil {
		return EnqueueResult{}, fmt.Errorf("insert outbox: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return EnqueueResult{}, fmt.Errorf("commit enqueue tx: %w", err)
	}
	return EnqueueResult{OutboxID: outboxID, IdempotencyKey: key, Status: "pending"}, nil
}
```

> **Module-path note:** confirm the `uuid` import path the repo already uses (`github.com/google/uuid` is conventional and is the dependency `intents.go` uses for `idempotency_key`; if `intents.go` uses a different uuid lib, match it).

- [ ] **Step 4.3: Run the test — expect PASS (or SKIP without DB)**

Run: `cd backend && go test ./internal/db/ -run TestEnqueueDirectGrantPropagation -v`
Expected: PASS against a live DB; SKIP without `DB_DSN`.

- [ ] **Step 4.4: Update the two `direct_role_grants` SELECTs to surface `source`**

In `backend/internal/db/grants.go`, extend the column list + `Scan` of `GetDirectGrantsForUser` and `GetAllDirectGrants` to include `COALESCE(source,'direct')` and `COALESCE(source_ref,'')`, appending `&grant.Source, &grant.SourceRef` to each `Scan`. Leave `GetExpiring*`/`GetExpired*`/`DeleteExpired*` untouched (they don't need source).

- [ ] **Step 4.5: Build + commit**

Run: `cd backend && go build ./internal/db/`
```bash
git add backend/internal/db/propagation_enqueue.go backend/internal/db/propagation_enqueue_test.go backend/internal/db/grants.go
git commit -m "feat(db): transactional EnqueueDirectGrantPropagation (ledger+audit+outbox)"
```

---

## Task 5 — `services/propagation` drain

**Files:**
- Create: `backend/internal/services/propagation/drain.go`
- Create: `backend/internal/services/propagation/deps.go`
- Create: `backend/internal/services/propagation/drain_test.go`

Mirror `services/expiry/`: a small package with injectable function vars in `deps.go` so the drain is testable without a live Zitadel or DB.

- [ ] **Step 5.1: Write `deps.go` (injectables)**

```go
// deps.go
package propagation

import (
	"context"

	"syndra/internal/db"
	"syndra/internal/models"
	"syndra/internal/zitadel"
)

var (
	claimPending  = db.ClaimPendingPropagations
	markApplied   = db.MarkPropagationApplied
	markFailed    = db.MarkPropagationFailed
	requeue       = db.RequeuePropagation

	// Reachability pre-flight: a cheap real call (limit-1 grant list) doubles as a probe.
	zitadelReachable = func(ctx context.Context) bool {
		if zitadel.MgmtClient == nil {
			return false
		}
		_, err := zitadel.MgmtClient.ListAllGrants(ctx, zitadel.SearchParams{Limit: 1})
		return err == nil
	}

	zitadelAddUserGrant = func(ctx context.Context, userID, projectID string, roleKeys []string) error {
		return zitadel.MgmtClient.AddUserGrant(ctx, userID, projectID, roleKeys)
	}
	zitadelUpdateUserGrant = func(ctx context.Context, userID, grantID string, roleKeys []string) error {
		return zitadel.MgmtClient.UpdateUserGrant(ctx, userID, grantID, roleKeys)
	}
	zitadelRemoveUserGrant = func(ctx context.Context, userID, grantID string) error {
		return zitadel.MgmtClient.RemoveUserGrant(ctx, userID, grantID)
	}

	// already-exists check (latency optimization only — see classifyZitadelError):
	// index first; on miss, ONE live grant list per row (not per role).
	grantIndexHasRole = db.GrantIndexHasRole // (ctx, user, project, role) (bool, error)
	liveUserGrantRoles = func(ctx context.Context, userID, projectID string) (map[string]bool, error) {
		res, err := zitadel.MgmtClient.ListUserGrants(ctx, userID, zitadel.SearchParams{Limit: 100})
		if err != nil {
			return nil, err
		}
		out := map[string]bool{}
		for _, g := range res.Items {
			if g.ProjectID != projectID {
				continue
			}
			for _, rk := range g.RoleKeys {
				out[rk] = true
			}
		}
		return out, nil
	}

	pruneTerminal = db.PruneTerminalPropagations

	maxRetries    = outboxMaxRetries()      // OUTBOX_MAX_RETRIES (default 5)
	retentionDays = outboxRetentionDays()   // OUTBOX_RETENTION_DAYS (default 30)
)

type ackClass int

const (
	ackApplied   ackClass = iota
	ackFailed             // terminal — operator must inspect
	ackTransient          // 5xx / timeout / network / 429 / 408 — retry
)

// classifyZitadelError maps a Zitadel client error to an ACK class by HTTP status,
// NOT by string-sniffing. Three review-hardened cases (design Decision 1):
//   - 409 AlreadyExists on add/replace  → ackApplied (idempotent success)
//   - 429 Too Many Requests, 408 Timeout → ackTransient (despite being 4xx)
//   - all other 4xx                      → ackFailed (terminal)
//   - 5xx / network / no status          → ackTransient
func classifyZitadelError(err error) ackClass {
	code := zitadelStatusCode(err) // reads zitadel.StatusError.Code; 0 if none
	switch {
	case code == 409:
		return ackApplied
	case code == 429 || code == 408:
		return ackTransient
	case code >= 400 && code < 500:
		return ackFailed
	default: // 5xx, network errors, or unknown
		return ackTransient
	}
}

var _ = models.PendingPropagation{} // keep the import even if unused after edits
```

> **Implement these supporting pieces:**
> - `db.GrantIndexHasRole(ctx, userID, projectID, role) (bool, error)` in `db/webhooks.go` (beside the grant index): `SELECT EXISTS(SELECT 1 FROM zitadel_grants_index WHERE user_id=$1 AND project_id=$2 AND $3 = ANY(role_keys))`.
> - `outboxMaxRetries()` (default `5`) and `outboxRetentionDays()` (default `30`) using the same env helper the expiry scheduler uses (`getEnvInt`-style).
> - `zitadelStatusCode(err) int` + a `zitadel.StatusError{Code int}` type **if the client does not already expose status**. The exploration confirmed the client wraps 429/503 with internal backoff and 401 with refresh — find where it surfaces the final status after retries and thread that through, rather than adding a parallel status path. This is the load-bearing seam for finding C (429 must be transient); pin it with the `TestDrain_RequeuesOn429` test in Step 5.2.

- [ ] **Step 5.2: Write failing drain tests (ACK classes + already-exists)**

```go
// drain_test.go
package propagation

import (
	"context"
	"testing"

	"syndra/internal/models"
	"syndra/internal/zitadel"
)

func swap[T any](dst *T, v T) func() { o := *dst; *dst = v; return func() { *dst = o } }

// NOTE: Drain() ends with a pruneTerminal() call and (on index miss) a
// liveUserGrantRoles() call. Every test that runs Drain to completion must stub
// both so they don't hit the nil PG pool / nil MgmtClient. Consider a
// stubDrainDeps(t) helper that sets safe no-op defaults (pruneTerminal→0,
// liveUserGrantRoles→empty, zitadelReachable→true) and let each test override
// only what it asserts. Shown inline below for explicitness.

func TestDrain_AppliedOn2xx(t *testing.T) {
	defer swap(&zitadelReachable, func(context.Context) bool { return true })()
	defer swap(&claimPending, func(_ context.Context, _ int) ([]models.PendingPropagation, error) {
		return []models.PendingPropagation{{ID: "o1", OpType: "add", UserID: "u", ProjectID: "p", RoleKeys: []string{"r"}}}, nil
	})()
	defer swap(&grantIndexHasRole, func(context.Context, string, string, string) (bool, error) { return false, nil })()
	defer swap(&liveUserGrantRoles, func(context.Context, string, string) (map[string]bool, error) { return map[string]bool{}, nil })()
	defer swap(&pruneTerminal, func(context.Context, int) (int64, error) { return 0, nil })()
	var addCalled bool
	defer swap(&zitadelAddUserGrant, func(context.Context, string, string, []string) error { addCalled = true; return nil })()
	var appliedID string
	defer swap(&markApplied, func(_ context.Context, id string) error { appliedID = id; return nil })()

	res, err := Drain(context.Background())
	if err != nil { t.Fatal(err) }
	if !addCalled || appliedID != "o1" || res.Applied != 1 {
		t.Fatalf("want add called + o1 applied, got addCalled=%v applied=%q res=%+v", addCalled, appliedID, res)
	}
}

func TestDrain_HaltsWhenZitadelOffline(t *testing.T) {
	defer swap(&zitadelReachable, func(context.Context) bool { return false })()
	res, err := Drain(context.Background())
	if err != nil { t.Fatal(err) }
	if !res.Halted || res.Reason != "zitadel_offline" {
		t.Fatalf("want halted/zitadel_offline, got %+v", res)
	}
}

func TestDrain_AlreadyExistsShortCircuits(t *testing.T) {
	defer swap(&zitadelReachable, func(context.Context) bool { return true })()
	defer swap(&claimPending, func(_ context.Context, _ int) ([]models.PendingPropagation, error) {
		return []models.PendingPropagation{{ID: "o2", OpType: "add", UserID: "u", ProjectID: "p", RoleKeys: []string{"r"}}}, nil
	})()
	defer swap(&grantIndexHasRole, func(context.Context, string, string, string) (bool, error) { return true, nil })()
	defer swap(&pruneTerminal, func(context.Context, int) (int64, error) { return 0, nil })()
	var addCalled bool
	defer swap(&zitadelAddUserGrant, func(context.Context, string, string, []string) error { addCalled = true; return nil })()
	defer swap(&markApplied, func(context.Context, string) error { return nil })()

	res, _ := Drain(context.Background())
	if addCalled { t.Fatal("add must be skipped when grant already exists") }
	if res.Applied != 1 { t.Fatalf("want 1 applied via short-circuit, got %+v", res) }
}

func TestDrain_FailedOn4xx_DoesNotHaltOthers(t *testing.T) {
	defer swap(&zitadelReachable, func(context.Context) bool { return true })()
	defer swap(&claimPending, func(_ context.Context, _ int) ([]models.PendingPropagation, error) {
		return []models.PendingPropagation{
			{ID: "bad", OpType: "add", UserID: "u", ProjectID: "p", RoleKeys: []string{"r1"}},
			{ID: "ok", OpType: "add", UserID: "u", ProjectID: "p", RoleKeys: []string{"r2"}},
		}, nil
	})()
	defer swap(&grantIndexHasRole, func(context.Context, string, string, string) (bool, error) { return false, nil })()
	defer swap(&liveUserGrantRoles, func(context.Context, string, string) (map[string]bool, error) { return map[string]bool{}, nil })()
	defer swap(&pruneTerminal, func(context.Context, int) (int64, error) { return 0, nil })()
	defer swap(&zitadelAddUserGrant, func(_ context.Context, _ , _ string, roles []string) error {
		if roles[0] == "r1" { return statusErr(400) }
		return nil
	})()
	var failedID, appliedID string
	defer swap(&markFailed, func(_ context.Context, id, _ string) error { failedID = id; return nil })()
	defer swap(&markApplied, func(_ context.Context, id string) error { appliedID = id; return nil })()

	res, _ := Drain(context.Background())
	if failedID != "bad" || appliedID != "ok" || res.Failed != 1 || res.Applied != 1 {
		t.Fatalf("4xx must fail its row but not halt others: %+v", res)
	}
}

func TestDrain_RequeuesOn429(t *testing.T) {
	defer swap(&zitadelReachable, func(context.Context) bool { return true })()
	defer swap(&claimPending, func(_ context.Context, _ int) ([]models.PendingPropagation, error) {
		return []models.PendingPropagation{{ID: "t1", OpType: "add", UserID: "u", ProjectID: "p", RoleKeys: []string{"r"}}}, nil
	})()
	defer swap(&grantIndexHasRole, func(context.Context, string, string, string) (bool, error) { return false, nil })()
	defer swap(&liveUserGrantRoles, func(context.Context, string, string) (map[string]bool, error) { return map[string]bool{}, nil })()
	defer swap(&pruneTerminal, func(context.Context, int) (int64, error) { return 0, nil })()
	defer swap(&zitadelAddUserGrant, func(context.Context, string, string, []string) error { return statusErr(429) })()
	var requeued bool
	defer swap(&requeue, func(_ context.Context, _, _ string) (int, error) { requeued = true; return 1, nil })()
	defer swap(&markFailed, func(context.Context, string, string) error { t.Fatal("429 must NOT mark failed"); return nil })()

	res, _ := Drain(context.Background())
	if !requeued || res.Requeued != 1 {
		t.Fatalf("429 must requeue (transient), got %+v", res)
	}
}

func TestDrain_AppliedOnAlreadyExists409(t *testing.T) {
	defer swap(&zitadelReachable, func(context.Context) bool { return true })()
	defer swap(&claimPending, func(_ context.Context, _ int) ([]models.PendingPropagation, error) {
		return []models.PendingPropagation{{ID: "e1", OpType: "add", UserID: "u", ProjectID: "p", RoleKeys: []string{"r"}}}, nil
	})()
	// Index says "not present" (stale-positive avoided): the live check also misses,
	// so we DO call Zitadel — which returns 409. That must resolve as applied.
	defer swap(&grantIndexHasRole, func(context.Context, string, string, string) (bool, error) { return false, nil })()
	defer swap(&liveUserGrantRoles, func(context.Context, string, string) (map[string]bool, error) { return map[string]bool{}, nil })()
	defer swap(&pruneTerminal, func(context.Context, int) (int64, error) { return 0, nil })()
	defer swap(&zitadelAddUserGrant, func(context.Context, string, string, []string) error { return statusErr(409) })()
	var appliedID string
	defer swap(&markApplied, func(_ context.Context, id string) error { appliedID = id; return nil })()
	defer swap(&markFailed, func(context.Context, string, string) error { t.Fatal("409 must NOT mark failed"); return nil })()

	res, _ := Drain(context.Background())
	if appliedID != "e1" || res.Applied != 1 {
		t.Fatalf("409 AlreadyExists must be idempotent success, got %+v", res)
	}
}

func TestDrain_PrunesTerminalRowsAtTail(t *testing.T) {
	defer swap(&zitadelReachable, func(context.Context) bool { return true })()
	defer swap(&claimPending, func(_ context.Context, _ int) ([]models.PendingPropagation, error) { return nil, nil })()
	var prunedWith int
	defer swap(&pruneTerminal, func(_ context.Context, days int) (int64, error) { prunedWith = days; return 4, nil })()

	if _, err := Drain(context.Background()); err != nil { t.Fatal(err) }
	if prunedWith != retentionDays {
		t.Fatalf("drain must prune with retentionDays=%d, got %d", retentionDays, prunedWith)
	}
}

// statusErr returns a typed status error the classifier reads by code (NOT by
// string). Use the same zitadel.StatusError the production client surfaces.
func statusErr(code int) error { return &zitadel.StatusError{Code: code} }
```

- [ ] **Step 5.3: Run — expect compile failure (`Drain` undefined)**

Run: `cd backend && go test ./internal/services/propagation/ -v`
Expected: FAIL — `undefined: Drain`.

- [ ] **Step 5.4: Implement `drain.go`**

```go
// drain.go
package propagation

import (
	"context"
	"fmt"
	"log"

	"syndra/internal/models"
)

type DrainResult struct {
	Applied int    `json:"applied"`
	Failed  int    `json:"failed"`
	Requeued int   `json:"requeued"`
	Halted  bool   `json:"halted"`
	Reason  string `json:"reason,omitempty"`
}

const claimBatch = 100

// Drain processes pending outbox rows in created_at order. Operator-triggered.
// `applied` (synchronous 2xx) is terminal success — no webhook round-trip.
func Drain(ctx context.Context) (DrainResult, error) {
	if !zitadelReachable(ctx) {
		return DrainResult{Halted: true, Reason: "zitadel_offline"}, nil
	}
	rows, err := claimPending(ctx, claimBatch)
	if err != nil {
		return DrainResult{}, fmt.Errorf("claim pending: %w", err)
	}
	var res DrainResult
	for _, row := range rows {
		if exists, _ := alreadyExists(ctx, row); exists {
			if err := markApplied(ctx, row.ID); err != nil {
				return res, err
			}
			res.Applied++
			continue
		}
		switch classifyDispatch(ctx, row) {
		case ackApplied:
			_ = markApplied(ctx, row.ID)
			res.Applied++
		case ackFailed:
			_ = markFailed(ctx, row.ID, lastDispatchErr(row))
			res.Failed++
		case ackTransient:
			attempts, _ := requeue(ctx, row.ID, lastDispatchErr(row))
			res.Requeued++
			if attempts > maxRetries {
				res.Halted = true
				res.Reason = "max_retries_exceeded"
				return res, nil
			}
		}
	}
	// Opportunistic retention prune (design §3.1). Non-fatal; canonical intent
	// lives in direct_role_grants, so a failed prune never loses real data.
	if n, err := pruneTerminal(ctx, retentionDays); err != nil {
		log.Printf("[PROPAGATION] retention prune failed: %v (non-fatal)", err)
	} else if n > 0 {
		log.Printf("[PROPAGATION] pruned %d terminal outbox rows older than %dd", n, retentionDays)
	}
	return res, nil
}

// alreadyExists is a latency optimization, NOT a correctness gate — Zitadel's
// 409 AlreadyExists (classified ackApplied) is the real safety net, so a false
// "no" here is harmless (we call, Zitadel absorbs the dup idempotently). It uses
// the webhook index first; on any miss it does ONE live grant list per row.
func alreadyExists(ctx context.Context, row models.PendingPropagation) (bool, error) {
	switch row.OpType {
	case "add", "replace":
		allIndexed := true
		for _, role := range row.RoleKeys {
			if ok, err := grantIndexHasRole(ctx, row.UserID, row.ProjectID, role); err != nil || !ok {
				allIndexed = false
				break
			}
		}
		if allIndexed {
			return true, nil // index covers every role; skip the API call
		}
		live, err := liveUserGrantRoles(ctx, row.UserID, row.ProjectID) // one list, not per-role
		if err != nil {
			return false, nil // can't confirm → proceed; 409 absorbs any dup
		}
		for _, role := range row.RoleKeys {
			if !live[role] {
				return false, nil
			}
		}
		return true, nil
	case "revoke":
		live, err := liveUserGrantRoles(ctx, row.UserID, row.ProjectID)
		if err != nil {
			return false, nil // can't confirm absence → let the revoke run
		}
		for _, role := range row.RoleKeys {
			if live[role] {
				return false, nil // still present → revoke must run
			}
		}
		return true, nil // nothing left to revoke → already in desired state
	}
	return false, nil
}

// classifyDispatch issues the Zitadel call and classifies the result. The last
// error is stashed per-row so markFailed/requeue can record it.
func classifyDispatch(ctx context.Context, row models.PendingPropagation) ackClass {
	var err error
	switch row.OpType {
	case "add":
		err = zitadelAddUserGrant(ctx, row.UserID, row.ProjectID, row.RoleKeys)
	case "replace":
		err = zitadelUpdateUserGrant(ctx, row.UserID, row.ZitadelGrantID, row.RoleKeys)
	case "revoke":
		err = zitadelRemoveUserGrant(ctx, row.UserID, row.ZitadelGrantID)
	default:
		log.Printf("[PROPAGATION] unknown op_type=%s row=%s", row.OpType, row.ID)
		stashErr(row.ID, fmt.Sprintf("unknown op_type %q", row.OpType))
		return ackFailed
	}
	if err == nil {
		return ackApplied
	}
	stashErr(row.ID, err.Error())
	return classifyZitadelError(err) // ackFailed for 4xx, ackTransient otherwise
}
```

Add a tiny per-drain error map (`stashErr`/`lastDispatchErr`) and `classifyZitadelError` (maps the zitadel client's status to `ackFailed`/`ackTransient`) in the same file. Keep `classifyZitadelError` honest about the real client error type — do not string-sniff if a typed status is available.

- [ ] **Step 5.5: Run drain tests — expect PASS**

Run: `cd backend && go test ./internal/services/propagation/ -v`
Expected: PASS (all four drain tests).

- [ ] **Step 5.6: Commit**

```bash
git add backend/internal/services/propagation/ backend/internal/db/webhooks.go
git commit -m "feat(propagation): operator-confirmed drain with ACK classification + already-exists short-circuit"
```

---

## Task 6 — Rewire `POST /api/v1/users/{id}/grants`

**Files:**
- Modify: `backend/internal/handlers/access.go:62-106`
- Modify: `backend/internal/handlers/deps.go` (add `dbEnqueueDirectGrantPropagation`, `svcDrain` injectables)
- Modify: `backend/internal/handlers/access_test.go` (or the relevant grant handler test file)

- [ ] **Step 6.1: Add injectables in `deps.go`**

```go
	dbEnqueueDirectGrantPropagation = db.EnqueueDirectGrantPropagation
	svcDrainPropagations            = propagation.Drain
```
(import `syndra/internal/services/propagation`.)

- [ ] **Step 6.2: Write the failing handler test (202 + enqueue, no direct Zitadel call)**

```go
func TestUpsertUserDirectGrant_EnqueuesAndReturns202(t *testing.T) {
	resetAccessDeps(t) // save/restore injectables; mirror resetRulesDeps
	var gotParams db.EnqueueParams
	dbEnqueueDirectGrantPropagation = func(_ context.Context, p db.EnqueueParams) (db.EnqueueResult, error) {
		gotParams = p
		return db.EnqueueResult{OutboxID: "ob1", IdempotencyKey: "key1", Status: "pending"}, nil
	}

	req := httptest.NewRequest("POST", "/api/v1/users/u1/grants",
		strings.NewReader(`{"project_id":"p1","role_key":"r1","reason":"lab access","duration_days":30}`))
	req.SetPathValue("id", "u1")
	w := httptest.NewRecorder()
	handleUpsertUserDirectGrant(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("want 202, got %d: %s", w.Code, w.Body)
	}
	if gotParams.OpType != "add" || gotParams.RoleKeys[0] != "r1" || gotParams.Source != "direct" {
		t.Fatalf("unexpected enqueue params: %+v", gotParams)
	}
	var body map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body["outbox_id"] != "ob1" || body["status"] != "pending" {
		t.Fatalf("unexpected body: %v", body)
	}
}
```

- [ ] **Step 6.3: Run — expect FAIL (still old behavior)**

Run: `cd backend && go test ./internal/handlers/ -run TestUpsertUserDirectGrant_EnqueuesAndReturns202 -v`
Expected: FAIL (200 + `{id,message}`).

- [ ] **Step 6.4: Rewrite the handler body**

Replace the `dbUpsertDirectGrant` call + audit + response (`access.go:96-105`) with:
```go
	payload, _ := json.Marshal(req)
	res, err := dbEnqueueDirectGrantPropagation(r.Context(), db.EnqueueParams{
		UserID:    userID,
		ProjectID: req.ProjectID,
		RoleKeys:  []string{req.RoleKey},
		GrantedBy: grantedBy,
		Reason:    req.Reason,
		ExpiresAt: expiresAt,
		Source:    "direct",
		OpType:    "add",
		PayloadJSON: string(payload),
	})
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	// Inline "apply now" for single mutations from inline forms (design §7 Q1).
	if r.URL.Query().Get("apply") == "true" {
		if dr, derr := svcDrainPropagations(r.Context()); derr == nil && dr.Applied > 0 {
			res.Status = "applied"
		}
	}
	rebuildUserCacheOrSkip(r.Context(), userID)
	jsonResponse(w, http.StatusAccepted, res)
```
(The `audit_logs` write now happens inside the transactional enqueue, so the standalone `dbInsertAuditLog` call is removed from this handler.)

- [ ] **Step 6.5: Run — expect PASS; then full handlers package**

Run: `cd backend && go test ./internal/handlers/ -run TestUpsertUserDirectGrant -v && go test ./internal/handlers/`
Expected: PASS. Fix any sibling test that asserted the old `{id,message}`/200 shape (update it to the 202 enqueue contract — this is an intended contract change, document it in the test rename).

- [ ] **Step 6.6: Commit**

```bash
git add backend/internal/handlers/access.go backend/internal/handlers/deps.go backend/internal/handlers/access_test.go
git commit -m "feat(handlers): route /users/{id}/grants through the outbox (202 + apply-now)"
```

---

## Task 7 — Rewire `/api/v1/zitadel/*` CRUD through the canonical path (B4/D3)

**Files:**
- Modify: `backend/internal/handlers/discovery.go:218-282`
- Modify: `backend/internal/handlers/deps.go` (add `dbGetGrantIndex` injectable if not present; `dbListUserGrantsLive`)
- Modify: `backend/internal/handlers/discovery_test.go` (or the file holding the existing zitadel-grant handler tests)

- [ ] **Step 7.1: Write the failing test — assign goes through enqueue, not direct Zitadel**

```go
func TestHandleAssignZitadelGrant_GoesThroughOutbox(t *testing.T) {
	resetDiscoveryDeps(t)
	var enqueued bool
	dbEnqueueDirectGrantPropagation = func(_ context.Context, p db.EnqueueParams) (db.EnqueueResult, error) {
		enqueued = true
		return db.EnqueueResult{OutboxID: "obz", IdempotencyKey: "k", Status: "pending"}, nil
	}
	var directCalled bool
	zitadelAddUserGrant = func(context.Context, string, string, []string) error { directCalled = true; return nil }

	req := httptest.NewRequest("POST", "/api/v1/zitadel/users/u1/grants",
		strings.NewReader(`{"projectId":"p1","roleKeys":["r1","r2"]}`))
	req.SetPathValue("id", "u1")
	w := httptest.NewRecorder()
	handleAssignZitadelGrant(w, req)

	if w.Code != http.StatusAccepted { t.Fatalf("want 202, got %d", w.Code) }
	if !enqueued { t.Fatal("assign must enqueue") }
	if directCalled { t.Fatal("assign must NOT call Zitadel directly anymore") }
}
```

- [ ] **Step 7.2: Run — expect FAIL**

Run: `cd backend && go test ./internal/handlers/ -run TestHandleAssignZitadelGrant_GoesThroughOutbox -v`
Expected: FAIL (direct call still happens, response is `{status:"granted"}`).

- [ ] **Step 7.3: Rewrite the three handlers**

`handleAssignZitadelGrant`: after validation, call `dbEnqueueDirectGrantPropagation` with `OpType:"add"`, `RoleKeys: req.RoleKeys`, `Source:"direct"`, `GrantedBy: resolveActor(r, "")`, `PayloadJSON` = marshaled body; respond `202 res`.

`handleUpdateZitadelGrant` (has `grantId` + `roleKeys`): resolve `grantId → (userID, projectID, roleKeys-existing)` via `dbGetGrantIndex(r.Context(), grantID)` (fallback `dbListUserGrantsLive`); enqueue `OpType:"replace"`, `ZitadelGrantID: grantID`, `RoleKeys: req.RoleKeys`. Respond `202`.

`handleRemoveZitadelGrant` (has `grantId`): resolve `grantId → (userID, projectID, roleKeys)` via index/live; enqueue `OpType:"revoke"`, `ZitadelGrantID: grantID`, `RoleKeys: <resolved roles>`. Respond `202`.

Each keeps its existing input validation; only the action + response change. The `zitadelAddUserGrant`/`Update`/`Remove` injectables in `handlers/deps.go` are now used **only** by the drain package's copies — the handlers no longer call them directly. (Leave the handler-package injectables in place if other code references them; otherwise delete to avoid dead code — `go vet` will tell you.)

- [ ] **Step 7.4: Run handler tests + vet**

Run: `cd backend && go test ./internal/handlers/ && go vet ./internal/handlers/`
Expected: PASS, no vet warnings. Update any test asserting the old `{status}` response.

- [ ] **Step 7.5: Commit**

```bash
git add backend/internal/handlers/discovery.go backend/internal/handlers/deps.go backend/internal/handlers/discovery_test.go
git commit -m "feat(handlers): rewire /zitadel/* grant CRUD through the canonical outbox path (B4/D3)"
```

---

## Task 8 — Drain endpoint, pending list, governance summary

**Files:**
- Create: `backend/internal/handlers/propagations.go`
- Modify: `backend/internal/handlers/router.go` (register routes)
- Modify: `backend/internal/services/views.go` (governance summary `pending_propagation` block)
- Create: `backend/internal/handlers/propagations_test.go`

- [ ] **Step 8.1: Write the failing endpoint test**

```go
func TestHandleDrainPropagations_ReturnsResult(t *testing.T) {
	resetPropagationDeps(t)
	svcDrainPropagations = func(context.Context) (propagation.DrainResult, error) {
		return propagation.DrainResult{Applied: 2}, nil
	}
	req := httptest.NewRequest("POST", "/api/v1/propagations/drain", nil)
	w := httptest.NewRecorder()
	handleDrainPropagations(w, req)
	if w.Code != http.StatusOK { t.Fatalf("want 200, got %d", w.Code) }
	if !strings.Contains(w.Body.String(), `"applied":2`) { t.Fatalf("body: %s", w.Body) }
}
```

- [ ] **Step 8.2: Implement handlers**

```go
// propagations.go
func handleDrainPropagations(w http.ResponseWriter, r *http.Request) {
	res, err := svcDrainPropagations(r.Context())
	if err != nil {
		jsonErrorResponse(w, http.StatusBadGateway, "DRAIN_ERROR", err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, res)
}

func handleListPendingPropagations(w http.ResponseWriter, r *http.Request) {
	rows, err := dbGetPendingPropagations(r.Context())
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, map[string]any{"pending": rows})
}
```
Add injectables `dbGetPendingPropagations = db.GetPendingPropagations` and (for the summary) `svcCountPendingPropagations = db.CountPendingPropagations`, `svcZitadelReachable`.

- [ ] **Step 8.3: Register routes (operator-auth)**

In `router.go`, beside the reconciliation route:
```go
mux.HandleFunc("POST /api/v1/propagations/drain", withCORS(withOperatorAuth(handleDrainPropagations)))
mux.HandleFunc("GET /api/v1/propagations", withCORS(withOperatorAuth(handleListPendingPropagations)))
```

- [ ] **Step 8.4: Extend governance summary**

In `services/views.go:Governance(ctx)`, add a `pending_propagation` field to the returned summary:
```go
count, _ := svcCountPendingPropagations(ctx)
summary.PendingPropagation = models.PendingPropagationSummary{
	Count:           count,
	ZitadelReachable: svcZitadelReachable(ctx),
}
```
Add the `PendingPropagationSummary` struct (`Count int`, `ZitadelReachable bool`, optionally `LastQueuedAt *time.Time`) to `models.go` and the field to the governance summary struct.

- [ ] **Step 8.5: Run + commit**

Run: `cd backend && go test ./internal/handlers/ ./internal/services/ && go vet ./...`
```bash
git add backend/internal/handlers/propagations.go backend/internal/handlers/router.go backend/internal/handlers/propagations_test.go backend/internal/handlers/deps.go backend/internal/services/views.go backend/internal/models/models.go
git commit -m "feat(handlers): drain endpoint, pending list, governance pending_propagation block"
```

---

## Task 9 — Frontend: Pending Propagation surfaces (amber, in-layout)

**Files:**
- Create: `ui/src/lib/queries/usePropagation.ts`
- Modify: `ui/src/lib/queries/useGovernance.ts` (add `pending_propagation` to `GovernanceSummary`)
- Modify: `ui/src/components/SidebarNav.tsx` (nested `Pending [N]` item + amber dot)
- Modify: `ui/src/components/dashboard/AdminDashboard.tsx` (dismissible pending callout)
- Create: `ui/src/components/propagation/PendingCallout.tsx`
- Modify: `ui/src/app/zitadel/page.tsx:688-735` (handle 202 `{outbox_id,status}` response)
- Create: `ui/src/components/propagation/PendingCallout.test.tsx`

- [ ] **Step 9.1: Extend the governance summary type + add the propagation hooks**

In `useGovernance.ts`, add to `GovernanceSummary`:
```typescript
  pending_propagation?: { count: number; zitadel_reachable: boolean; last_queued_at?: string | null };
```
and default it in the `queryFn` mapper (`pending_propagation: data?.pending_propagation ?? { count: 0, zitadel_reachable: true }`).

Create `usePropagation.ts`:
```typescript
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { request } from "@/lib/api-client";

export interface PendingRow {
  id: string; op_type: string; user_id: string; project_id: string;
  role_keys: string[]; status: string; attempts: number; created_at: string;
}

export function usePendingPropagations() {
  return useQuery({
    queryKey: ["propagations", "pending"],
    queryFn: async () => (await request<{ pending: PendingRow[] }>("/propagations")).pending ?? [],
  });
}

export function useDrainPropagations() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async () =>
      request<{ applied: number; failed: number; halted: boolean; reason?: string }>(
        "/propagations/drain", { method: "POST" }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["propagations"] });
      qc.invalidateQueries({ queryKey: ["governance"] });
    },
  });
}
```

- [ ] **Step 9.2: Sidebar nested `Pending [N]` item**

In `SidebarNav.tsx`, read `summary.pending_propagation?.count` (via the existing governance fetch — reuse `useGovernanceSummary` rather than the raw `fetch`), and add an Operations-section nested item:
```typescript
{ href: "/governance/pending", label: "Pending", badge: pendingPropCount > 0 ? pendingPropCount : undefined }
```
The badge uses amber tokens (`bg-tertiary-container text-on-tertiary-container`) consistent with the bundles "Welcome" badge idiom. Add a small amber dot on the parent Operations header when `pendingPropCount > 0`.

- [ ] **Step 9.3: Write the failing callout render test**

```typescript
// PendingCallout.test.tsx
import { render, screen } from "@testing-library/react";
import { PendingCallout } from "./PendingCallout";

test("renders count + Resume now when count > 0 and reachable", () => {
  render(<PendingCallout count={3} reachable onResume={() => {}} dismissed={false} onDismiss={() => {}} />);
  expect(screen.getByText(/3/)).toBeInTheDocument();
  expect(screen.getByRole("button", { name: /resume now/i })).toBeEnabled();
});

test("disables Resume and shows offline when unreachable", () => {
  render(<PendingCallout count={3} reachable={false} onResume={() => {}} dismissed={false} onDismiss={() => {}} />);
  expect(screen.getByRole("button", { name: /resume now/i })).toBeDisabled();
  expect(screen.getByText(/offline/i)).toBeInTheDocument();
});

test("renders nothing when count is 0", () => {
  const { container } = render(<PendingCallout count={0} reachable onResume={() => {}} dismissed={false} onDismiss={() => {}} />);
  expect(container).toBeEmptyDOMElement();
});
```

- [ ] **Step 9.4: Implement `PendingCallout.tsx`**

```tsx
"use client";
import { Card } from "@/components/ui/Card";
import { Button } from "@/components/ui/Button";

interface Props {
  count: number; reachable: boolean; dismissed: boolean;
  onResume: () => void; onDismiss: () => void;
}

export function PendingCallout({ count, reachable, dismissed, onResume, onDismiss }: Props) {
  if (count <= 0 || dismissed) return null;
  return (
    <Card className="flex items-center justify-between gap-4 border border-tertiary/40 bg-[color-mix(in_srgb,var(--tertiary-container)_60%,transparent)] px-4 py-3">
      <div className="flex items-center gap-3 text-on-surface">
        <span aria-hidden>⏱</span>
        <span>
          <strong>{count}</strong> {count === 1 ? "change" : "changes"} awaiting Zitadel
          {!reachable && <span className="ml-2 text-warning">— Zitadel offline</span>}
        </span>
      </div>
      <div className="flex items-center gap-2">
        <Button onClick={onResume} disabled={!reachable}>Resume now</Button>
        <button aria-label="Dismiss" onClick={onDismiss} className="text-on-surface/60 hover:text-on-surface">×</button>
      </div>
    </Card>
  );
}
```
Wire it into `AdminDashboard.tsx` above the stat grid, with per-session dismiss state (`useState`, not persisted) and `onResume = () => drain.mutate()`. A single-pulse badge animation on count increase can reuse the dashboard's existing `Pulse` element; keep it to one pulse (no loop) and gate on `prefers-reduced-motion`.

- [ ] **Step 9.5: Update the zitadel page grant callers for the 202 shape**

In `app/zitadel/page.tsx`, the `onAssign`/`onUpdate`/`onRevoke` handlers (`:688-735`) currently expect a synchronous success. Update them to read `{outbox_id, status}` and set the flash to `status === "applied" ? "Grant applied" : "Queued — resume from the dashboard to send to Zitadel"`. Keep the existing `loadGrants` refresh. (This is the minimal change folded in here; the full U5 split is Theme 4 / Wave 3.)

- [ ] **Step 9.6: Run UI tests + lint + build**

Run: `cd ui && bun run test && bun run lint && bun run build`
Expected: PASS.

- [ ] **Step 9.7: Commit**

```bash
git add ui/src/lib/queries/usePropagation.ts ui/src/lib/queries/useGovernance.ts ui/src/components/SidebarNav.tsx ui/src/components/dashboard/AdminDashboard.tsx ui/src/components/propagation/ ui/src/app/zitadel/page.tsx
git commit -m "feat(ui): Pending Propagation surfaces (sidebar item, dashboard callout, 202 handling)"
```

---

## Task 10 — Sub-phase 1 verification gate

- [ ] **Step 10.1: Backend full suite + vet**

Run:
```bash
cd backend && go test ./... && go vet ./...
```
Expected: all PASS, no vet warnings.

- [ ] **Step 10.2: UI gate**

Run:
```bash
cd ui && bun run lint && bun run test && bun run build
```
Expected: all PASS.

- [ ] **Step 10.3: Migration round-trip for `000015`**

Run (against a throwaway DB; if unavailable, statically validate file naming + up/down SQL symmetry and note it explicitly, as Wave 2 · Part 3 Task 12 did):
```bash
cd backend
migrate -path db/migrations -database "$DB_DSN" up
migrate -path db/migrations -database "$DB_DSN" down 1
migrate -path db/migrations -database "$DB_DSN" up
```
Expected: clean up → down → up; `pending_zitadel_propagations` + `direct_role_grants.source` present after final `up`.

- [ ] **Step 10.4: gofmt scoped to the touch set**

Run:
```bash
cd backend && gofmt -d \
  internal/db/propagations.go internal/db/propagation_enqueue.go internal/db/propagations_test.go \
  internal/db/propagation_enqueue_test.go internal/db/grants.go internal/db/webhooks.go \
  internal/models/models.go internal/handlers/access.go internal/handlers/discovery.go \
  internal/handlers/deps.go internal/handlers/propagations.go internal/handlers/router.go \
  internal/services/views.go internal/services/propagation/*.go
```
Expected: zero diff.

- [ ] **Step 10.5: Codebase-memory refresh**

```
mcp__codebase-memory-mcp__detect_changes
mcp__codebase-memory-mcp__index_repository   # affected scope: backend/internal/{db,handlers,services}, ui/src
```

- [ ] **Step 10.6: OpenSpec validate + tick the ledger**

```bash
cd /Users/notkanishk/Documents/Mkrspc/Projects/Syndra
openspec validate wave-2-part-4-zitadel-state-projection-and-drift-control --strict
```
Then check off Sub-phase 1 Tasks 0–10 in `tasks.md` and commit:
```bash
git add openspec/changes/wave-2-part-4-zitadel-state-projection-and-drift-control/tasks.md
git commit -m "chore(openspec): tick wave-2-part-4 sub-phase 1 (outbox) tasks complete"
```

---

## Self-review checklist (run after implementation, before requesting review)

1. **Doctrine honored:** every Syndra-mediated mutation writes the ledger *before* any Zitadel call, in one transaction (Tasks 4, 6, 7). No handler calls `zitadelAddUserGrant`/`Update`/`Remove` directly anymore (Task 7).
2. **No `confirmed` state anywhere** — outbox CHECK is `('pending','in_flight','applied','failed')`; pending count counts `pending`+`in_flight` only (Tasks 1, 3, 8). Design Decision 1.
3. **Idempotency:** Zitadel's `409 AlreadyExists` is classified `applied` (Task 5), so a stale grant index in *either* direction is harmless — the already-exists check is a latency optimization, not a correctness gate. Crash-recovery replay and the `idempotency_key` UNIQUE constraint cover the rest (Tasks 3, 5).
3a. **Transient ≠ terminal:** `429`/`408` requeue (never operator-triage), `5xx`/timeout requeue, other `4xx` fail. Classifier reads typed status, not error strings (Task 5; review finding C).
3b. **Bounded growth:** the drain prunes `applied`/`failed` rows older than `OUTBOX_RETENTION_DAYS` (default 30) at its tail; `failed` rows survive the full window as the attention-needed audit trail (Tasks 3, 5; review finding A).
4. **Type consistency:** `EnqueueParams.RoleKeys []string` (plural) flows from both `/users` (single role wrapped) and `/zitadel` (multi-role) callers; `op_type` ∈ `{add,replace,revoke}` matches the CHECK; `DrainResult` field names match between Go (`Applied`/`Failed`/`Halted`/`Reason`) and the TS hook.
5. **Backward compatibility:** `/api/v1/zitadel/*` URLs still resolve (aliases); only the response shape changed, and the one frontend caller is updated (Task 9.5).
6. **Scope discipline:** no drift tables, no `confirmation_mode`, no cascade enqueueing — those are sub-phases 2 and 3. This plan ships operator point mutations through the buffer, end to end.

---

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-06-09-wave-2-part-4-phase-1-outbox.md`. Two execution options:

1. **Subagent-Driven (recommended)** — dispatch a fresh subagent per task, review between tasks (`superpowers:subagent-driven-development`).
2. **Inline Execution** — execute tasks in this session with checkpoints (`superpowers:executing-plans`).
