# Wave 2 · Part 4 — Sub-phase 3: Bundle/Rule Cascade Projection Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make bundle and mapping-rule changes project their effective grants into Zitadel through the sub-phase-1 outbox — so a bundle/rule change no longer computes roles read-side-only but actually mirrors them as `user_grant` rows — with a per-source `confirmation_mode` (auto drains immediately, manual queues) and an other-source check that suppresses a revoke when another source still covers the triple.

**Architecture:** A thin cascade orchestrator (`services/cascade.go`) turns each operator action into a set of `(user, project, role)` add/revoke intents and commits the source mutation **and** its outbox rows in one atomic `db.*AndEnqueue` tx (mirroring sub-phase 1's `EnqueueDirectGrantPropagation`), so a committed bundle/rule change always has its projection rows. Cascades write **no** `direct_role_grants` rows — that table's `UNIQUE(user,project,role)` cannot represent multi-source grants, so attribution lives on the new outbox `source`/`source_ref` columns and access is computed from the bundle/rule tables (unchanged `collectUserRoles`). When the source is `confirmation_mode='auto'` the orchestrator drains only its own rows via a new targeted `propagation.DrainBatch` (one advisory lock + one reachability preflight for the whole batch, reusing `DrainOne`'s `processRow`); manual rows wait in the existing Pending tier. Adds rely on the drain's already-exists short-circuit (`409→applied`); revokes use an explicit **other-source check** (`OtherSourceCovers`) computed from the *other* sources. The 6th trigger (rule-matcher change) composes add-new-target + revoke-old-target using the handler-captured pre-update rule — no diff engine. Auto cascades surface in a new "Recent automated cascades" element so they are never invisible.

**Tech Stack:** Go (module `syndra`, `pgx/v5` pool, stdlib `testing`, injectable-deps pattern), PostgreSQL (golang-migrate). Next.js + TypeScript + React Query (Bun), Vitest + Testing-Library. Material (obsidian-clarity) tokens.

## Global Constraints

Every task's requirements implicitly include these. Values copied verbatim from the design/specs:

- **Go module path:** `syndra`. Imports are `syndra/internal/...`.
- **Migrations dir:** `backend/db/migrations/`; highest is `000016_drift_queue`; **next is `000017`**. Paired `.up.sql`/`.down.sql`, `IF EXISTS`/`IF NOT EXISTS` guards, `DO $$` for constraint blocks, real down migrations that drop what up added. `uuid_generate_v4()` is available (used by `000015`/`000016`).
- **The full `source` CHECK enum already exists** (`000015`): `('direct','bundle','rule','external_backfill','lifecycle_cascade')`. Sub-phase 3 writes `source='bundle'` and `source='rule'` — **no `ALTER` to that enum is needed.**
- **`db` package has NO live-DB test harness.** It is covered only by *migration-coherence guards* (assert the SQL `CHECK` enums / column names match the Go string literals — see `backend/internal/db/propagations_migration_test.go` and `drift` equivalent). Behavioral coverage lives in **injectable service tests** (`services`, `services/propagation`) and **handler tests**, never live SQL.
- **No new dependencies.** No uuid module — outbox idempotency keys come from `db.NewOutboxIdempotencyKey() (string, error)` (crypto/rand v4, `db/propagations.go`). **NEVER import `github.com/google/uuid`.** Env parsing is stdlib inline (`strconv.Atoi`), no `getEnvInt` helper.
- **Reuse, do not reinvent.** The atomic enqueue unit is `db.enqueueWrites(ctx, tx, EnqueueParams, key) (outboxID string, err error)` — ledger upsert (add/replace) + audit + outbox insert on an existing tx; revoke intents skip the ledger upsert. `EnqueueParams` fields: `UserID, ProjectID, RoleKeys []string, GrantedBy, Reason, ExpiresAt *time.Time, Source, SourceRef, OpType, ZitadelGrantID, PayloadJSON`. The drain's already-exists short-circuit and `409→applied` make a pre-enqueue existence check unnecessary for adds.
- **Injectable-deps pattern:** package-level `var fn = realImpl` blocks (`handlers/deps.go`, `services/deps.go`, `services/propagation/deps.go`). Tests swap the vars; add a `reset*Deps()` helper per new test file mirroring the existing ones.
- **Cascade batching:** the transactional enqueue MUST NOT hold one `pgx.Tx` open across an unbounded user set. Batch at **500 rows per transaction**, committing each batch before opening the next (design §5).
- **Scale premise:** single-LXC, ~200 users, ~10 bundles. A bundle cascade maxes ~200 rows. Bounded fan-out; the reconciliation cap is 2 000.
- **Confirmation mode:** `auto` | `manual`, defaulting from `config_settings.global.default_rule_confirmation_mode`. Auto drains immediately (targeted to its own rows); manual and operator point mutations wait for explicit resume. **Expiry sweeps and lifecycle cascades are hardcoded `auto`** — their authoring is the pre-authorization — and MUST surface in "Recent automated cascades".

---

## All six trigger points are in scope

Design §4.7 lists **six** trigger points. All six are implemented:

1. Add user to bundle (`POST /api/v1/users/{id}/bundles`) — endpoint exists (Task 20).
2. Remove user from bundle — **new** `DELETE /api/v1/users/{id}/bundles/{bundleId}` (Task 21).
3. Add role to bundle (`POST /api/v1/bundles/{id}/roles`) — endpoint exists (Task 20).
4. Remove role from bundle — **new** `DELETE /api/v1/bundles/{id}/roles/{projectId}/{roleKey}` (Task 21).
5. Mapping rule fires (on create) — `POST /api/v1/rules/mapping` exists (Task 20).
6. Mapping-rule matcher change — **new** `PUT /api/v1/rules/mapping/{id}` (rules are create/get/validate-only today) (Task 21, §21f).

**The 6th trigger needs no bespoke old-vs-new diff engine.** It composes from the two primitives Tasks 20/21 already build, with the handler capturing the pre-update rule:
- **Add pass** = re-run the rule's add projection (every user holding the *new* source gets the *new* target; the drain's `409→applied` no-ops those who already had it).
- **Revoke pass** = every user holding the *old* source loses the *old* target (unless the triple is unchanged-and-re-added, or another source covers it). The old rule is captured by the handler before the update — needed because cascades keep no per-source ledger row to query.

**Projected per-role, attributed per-source on the outbox.** Zitadel holds flat per-role `user_grants` (it has no bundle/rule concept); **Syndra does NOT mirror them into `direct_role_grants`** — that table's `UNIQUE(user,project,role)` collapses multi-source grants to one last-writer row, so a ledger mirror would destroy source attribution and clobber operator grants. Bundle/rule intent already lives in `user_bundle_assignments`/`bundle_roles`/`mapping_rules` (which `collectUserRoles` reads for access). The *outbox* row carries `source`/`source_ref` for attribution (both threaded onto `models.PendingPropagation` and `CascadeSummary`), so the Pending worklist and Recent-cascades UIs name the originating bundle/rule by `source_ref`. The per-role breakout is a projection detail the operator never thinks in.

**Why sub-phase 3 is required (not backstopped by phase 2):** the phase-2 sweep deliberately does **not** project bundle/rule grants — its own comment: *"Bundle/rule-derived expected roles absent from Zitadel are NOT drift in sub-phase 2 — cascade projection is sub-phase 3."* Its `syndra_only` replay iterates `direct_role_grants` only. And because the Actions v2 data plane gates claim emission on the projects present in Zitadel `user_grants` (`dedupProjectIDs(req.UserGrants)` → zero grants ⇒ empty claims), a bundle/rule that is the **sole** source of a user's access to a project produces no claims until sub-phase 3 projects it. So the add-side is load-bearing for access, not cosmetic.

---

## OpenSpec change scope

- `openspec/changes/wave-2-part-4-zitadel-state-projection-and-drift-control/tasks.md` (Sub-phase 3, Tasks 19–23; Task 24 docs).
- `.../specs/automation-policies/spec.md` — the two ADDED requirements + their five scenarios (all covered below).
- `.../design.md` — Decision 2 (000017), §4 (data model), §4.7 (trigger table), §5 (batching), §7 Q1/Q4 (inline/other-source).

---

## File Structure

**Backend — create:**
- `backend/db/migrations/000017_confirmation_mode.up.sql` / `.down.sql`
- `backend/internal/db/config_settings.go` — `config_settings` repo (Get/Set).
- `backend/internal/db/cascade.go` — `enqueueCascadeRows` (tx-scoped outbox insert, no ledger) + atomic `AssignBundleAndEnqueue`/`AddRoleToBundleAndEnqueue`/`RemoveBundleFromUserAndEnqueue`/`RemoveRoleFromBundleAndEnqueue`/`UpdateMappingRuleAndEnqueue`/`CreateMappingRuleAndEnqueue` + `GetUsersForBundle` + `GetUsersWithRole` + bulk confirmation-mode setters + `GetRecentCascades`.
- `backend/internal/db/config_settings_migration_test.go` + `cascade_migration_test.go` — coherence guards.
- `backend/internal/services/cascade.go` — the six cascade orchestrators + `OtherSourceCovers` (the other-source coverage check) + injectable deps.
- `backend/internal/services/cascade_test.go` — behavioral tests (injectable swap).
- `backend/internal/services/propagation/drainbatch.go` — `DrainBatch`.
- `backend/internal/services/propagation/drainbatch_test.go`.
- `backend/internal/handlers/cascade_surfaces.go` — `handleRemoveBundleFromUser`, `handleRemoveRoleFromBundle`, `handleGetRecentCascades`, `handleSetConfirmationMode` (bulk), `handleGetGlobalConfirmationDefault` / `handleSetGlobalConfirmationDefault`.
- `backend/internal/handlers/cascade_surfaces_test.go`.

**Backend — modify:**
- `backend/internal/models/*.go` — add `ConfirmationMode` to `Bundle` + `MappingRule`.
- `backend/internal/db/bundles.go` — select `confirmation_mode`; `CreateBundle` takes mode; add `RemoveRoleFromBundle`.
- `backend/internal/db/rules.go` — select `confirmation_mode`; `CreateMappingRule` takes mode; add `UpdateMappingRule`.
- `backend/internal/handlers/bundles.go` — `handleAssignBundleToUser` + `handleAddRoleToBundle` fire cascade; `handleCreateBundle` inherits default mode.
- `backend/internal/handlers/rules.go` — `handleCreateMappingRule` inherits default mode + fires cascade; add `handleUpdateMappingRule` (6th trigger).
- `backend/internal/handlers/deps.go` + `services/deps.go` — new injectables.
- `backend/internal/handlers/router.go` — 6 new routes (2 bundle-remove, 1 rule-update, config get/set, bulk-set, recent-cascades).
- `backend/cmd/api/main.go` (if expiry/lifecycle enqueue paths need the hardcoded-auto surface tag) — verify only.
- `.env.example` — no new env for sub-phase 3 (docs task adds `OUTBOX_*` from sub-phase 1 if still missing).

**UI — create:**
- `ui/src/lib/queries/useConfirmationMode.ts` — global-default get/set + bulk-set mutations + recent-cascades query.
- `ui/src/components/policies/ConfirmationModeControls.tsx` — bulk multi-select + mode picker (shared by Policies + Bundles).
- `ui/src/components/operations/RecentCascades.tsx` — the "Recent automated cascades" element.

**UI — modify:**
- `ui/src/app/policies/page.tsx` + `ui/src/app/bundles/page.tsx` — mount bulk controls + per-row mode badge + create-form mode field.
- `ui/src/lib/queries/useMappingRules.ts` + `useBundles.ts` — carry `confirmation_mode`.
- `ui/src/components/SidebarNav.tsx` — "Recent cascades" nav item under Operations.
- `ui/src/app/settings` or the existing settings surface — global-default dropdown (mount beside existing config; if no settings page, mount on `/operations`).

---

## Reference: exact current signatures this plan builds on

```go
// db/propagation_enqueue.go
type EnqueueParams struct {
    UserID, ProjectID string
    RoleKeys          []string
    GrantedBy, Reason string
    ExpiresAt         *time.Time
    Source, SourceRef string   // Source defaults to "direct" when empty
    OpType            string    // add | revoke | replace
    ZitadelGrantID    string
    PayloadJSON       string
}
func enqueueWrites(ctx context.Context, tx pgx.Tx, p EnqueueParams, key string) (string, error) // outboxID
func EnqueueDirectGrantPropagation(ctx context.Context, p EnqueueParams) (EnqueueResult, error)

// db/propagations.go
func NewOutboxIdempotencyKey() (string, error)

// services/propagation/drain.go
type DrainResult struct { Applied, Failed, Requeued, Errored int; Halted bool; Reason string }
func Drain(ctx context.Context) (DrainResult, error)
func DrainOne(ctx context.Context, outboxID string) (DrainResult, error)
// deps.go (pkg vars): claimOne=db.ClaimPropagationByID, acquireDrainLock=db.TryAcquireDrainLock,
//   zitadelReachable() bool, processRow(ctx, row) internal per-row classify/apply.

// services/views.go
func collectUserRoles(ctx, userID) (map[roleKey]*models.EffectiveRole, []models.Bundle, error)
//   roleKey is keyed by (projectID, roleKey); unions direct + bundle + rule (fixpoint).

// db/bundles.go
func GetRolesForBundle(ctx, bundleID) ([]models.BundleRole, error)  // BundleRole{BundleID, ProjectID, RoleKey}
func GetBundlesForUser(ctx, userID) ([]models.Bundle, error)
func AssignBundleToUser(ctx, userID, bundleID) error
func RemoveBundleFromUser(ctx, userID, bundleID) error
func AddRoleToBundle(ctx, bundleID, projectID, roleKey) error
// tables: bundles(id,name,description,is_welcome,created_at); bundle_roles(bundle_id,zitadel_project_id,zitadel_role_key);
//         user_bundle_assignments(user_id,bundle_id)

// db/rules.go
func CreateMappingRule(ctx, sourceProject, sourceRole, targetProject, targetRole string) (string, error)
func GetActiveMappingRules(ctx) ([]models.MappingRule, error)
// db/webhooks.go — zitadel_grants_index(grant_id,user_id,project_id,role_keys[],...)
```

---

## Task 19 — Migration `000017` + `config_settings` repo + `confirmation_mode` on models

**Files:**
- Create: `backend/db/migrations/000017_confirmation_mode.up.sql`, `.down.sql`
- Create: `backend/internal/db/config_settings.go`
- Create: `backend/internal/db/config_settings_migration_test.go`
- Modify: `backend/internal/models/` (`Bundle`, `MappingRule` structs — grep for their definitions), `backend/internal/db/bundles.go` (`GetAllBundles`, `GetBundlesForUser`, add `CreateBundle` mode param), `backend/internal/db/rules.go` (`GetActiveMappingRules`, `CreateMappingRule` mode param)

**Interfaces produced (later tasks rely on these):**
- `db.GetConfigSetting(ctx context.Context, key string) (string, error)` — returns `""` (not error) when key absent.
- `db.SetConfigSetting(ctx context.Context, key, value, updatedBy string) error` — upsert.
- `models.Bundle.ConfirmationMode string` and `models.MappingRule.ConfirmationMode string`.
- `db.CreateBundle(ctx, name, description, confirmationMode string) (string, error)` (signature grows by one arg).
- `db.CreateMappingRule(ctx, sourceProject, sourceRole, targetProject, targetRole, confirmationMode string) (string, error)`.

- [ ] **Step 1: Write the up migration**

`backend/db/migrations/000017_confirmation_mode.up.sql`:
```sql
-- Wave 2 · Part 4 — Sub-phase 3: per-source confirmation mode for cascade projection.
-- mapping_rules + bundles gain confirmation_mode (auto drains immediately, manual queues).
-- config_settings holds the global default new rules/bundles inherit.
-- pending_zitadel_propagations gains source/source_ref so a cascade outbox row records its
-- originating bundle/rule (000015 outbox had no source column) — used for "Recent automated
-- cascades" and for grouping the Pending UI by source.

ALTER TABLE mapping_rules
    ADD COLUMN IF NOT EXISTS confirmation_mode TEXT NOT NULL DEFAULT 'auto'
        CHECK (confirmation_mode IN ('auto', 'manual'));

ALTER TABLE bundles
    ADD COLUMN IF NOT EXISTS confirmation_mode TEXT NOT NULL DEFAULT 'auto'
        CHECK (confirmation_mode IN ('auto', 'manual'));

-- Attribution on the outbox row (NOT direct_role_grants — cascades do not write the ledger;
-- see design pivot). Default 'direct' keeps existing operator rows valid; source_ref is the
-- bundle/rule id for cascade rows, NULL otherwise. Full 5-value enum matches direct_role_grants.
ALTER TABLE pending_zitadel_propagations
    ADD COLUMN IF NOT EXISTS source TEXT NOT NULL DEFAULT 'direct'
        CHECK (source IN ('direct', 'bundle', 'rule', 'external_backfill', 'lifecycle_cascade'));
ALTER TABLE pending_zitadel_propagations
    ADD COLUMN IF NOT EXISTS source_ref TEXT;
CREATE INDEX IF NOT EXISTS idx_pending_zitadel_propagations_source
    ON pending_zitadel_propagations(source, created_at);

CREATE TABLE IF NOT EXISTS config_settings (
    key        TEXT PRIMARY KEY,
    value      TEXT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_by VARCHAR(255)
);

INSERT INTO config_settings (key, value, updated_by)
    VALUES ('global.default_rule_confirmation_mode', 'auto', 'migration')
    ON CONFLICT (key) DO NOTHING;
```

- [ ] **Step 2: Write the down migration**

`backend/db/migrations/000017_confirmation_mode.down.sql`:
```sql
DROP TABLE IF EXISTS config_settings;
DROP INDEX IF EXISTS idx_pending_zitadel_propagations_source;
ALTER TABLE pending_zitadel_propagations DROP COLUMN IF EXISTS source_ref;
ALTER TABLE pending_zitadel_propagations DROP COLUMN IF EXISTS source;
ALTER TABLE bundles       DROP COLUMN IF EXISTS confirmation_mode;
ALTER TABLE mapping_rules DROP COLUMN IF EXISTS confirmation_mode;
```

- [ ] **Step 3: Write the failing coherence guard test**

`backend/internal/db/config_settings_migration_test.go` — assert the migration SQL enum literals match the Go constants and the seeded key. Mirror `propagations_migration_test.go`'s file-reading pattern (read the `.up.sql`, assert substrings):
```go
package db

import (
	"os"
	"strings"
	"testing"
)

func TestMigration000017_ConfirmationModeEnumMatchesGo(t *testing.T) {
	sql, err := os.ReadFile("../../db/migrations/000017_confirmation_mode.up.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	s := string(sql)
	for _, want := range []string{
		"confirmation_mode IN ('auto', 'manual')",
		"config_settings",
		"global.default_rule_confirmation_mode",
		"ALTER TABLE pending_zitadel_propagations", // outbox source/source_ref attribution
		"ADD COLUMN IF NOT EXISTS source",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("000017 up migration missing %q", want)
		}
	}
	// The Go layer only ever writes these two mode literals:
	for _, mode := range []string{confirmationModeAuto, confirmationModeManual} {
		if !strings.Contains(s, "'"+mode+"'") {
			t.Errorf("migration CHECK does not cover Go literal %q", mode)
		}
	}
}
```

- [ ] **Step 4: Run it, verify it fails** (`confirmationModeAuto` undefined)

Run: `cd backend && go test ./internal/db/ -run TestMigration000017 -v`
Expected: FAIL — `undefined: confirmationModeAuto`.

- [ ] **Step 5: Create `config_settings.go` with the constants + repo**

`backend/internal/db/config_settings.go`:
```go
package db

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

const (
	confirmationModeAuto   = "auto"
	confirmationModeManual = "manual"

	ConfigKeyDefaultConfirmationMode = "global.default_rule_confirmation_mode"
)

// GetConfigSetting returns the value for key, or "" when the key is absent
// (absence is not an error — callers fall back to a compile-time default).
func GetConfigSetting(ctx context.Context, key string) (string, error) {
	var v string
	err := PG.QueryRow(ctx, `SELECT value FROM config_settings WHERE key = $1`, key).Scan(&v)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return v, nil
}

// SetConfigSetting upserts a config value.
func SetConfigSetting(ctx context.Context, key, value, updatedBy string) error {
	_, err := PG.Exec(ctx, `
		INSERT INTO config_settings (key, value, updated_by, updated_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value,
			updated_by = EXCLUDED.updated_by, updated_at = NOW()`,
		key, value, updatedBy)
	return err
}

// NormalizeConfirmationMode returns a valid mode, defaulting unknown/empty to auto.
func NormalizeConfirmationMode(m string) string {
	if m == confirmationModeManual {
		return confirmationModeManual
	}
	return confirmationModeAuto
}
```

- [ ] **Step 6: Run the guard test, verify it passes**

Run: `cd backend && go test ./internal/db/ -run TestMigration000017 -v`
Expected: PASS.

- [ ] **Step 7: Add `ConfirmationMode` to the models**

Grep `backend/internal/models/` for `type Bundle struct` and `type MappingRule struct`. Add to each:
```go
	ConfirmationMode string `json:"confirmation_mode"`
```
(Place it after the last existing field, before `CreatedAt` for consistency.)

- [ ] **Step 8: Thread `confirmation_mode` through the SELECTs and CREATEs**

In `db/bundles.go`:
- `GetAllBundles` + `GetBundlesForUser`: add `confirmation_mode` to the SELECT column list and `&b.ConfirmationMode` to the `Scan`. (These already `COALESCE`-free; the column is `NOT NULL DEFAULT 'auto'` so no COALESCE needed.)
- `CreateBundle(ctx, name, description string)` → `CreateBundle(ctx, name, description, confirmationMode string)`; add `confirmation_mode` to the INSERT column list + `$N` placeholder + arg. Use `NormalizeConfirmationMode(confirmationMode)`.

In `db/rules.go`:
- `GetActiveMappingRules`: add `confirmation_mode` to SELECT + `&r.ConfirmationMode` to Scan.
- `CreateMappingRule(...)` → add trailing `confirmationMode string` param; add to INSERT with `NormalizeConfirmationMode`.

Fix the two call sites (`handlers/bundles.go:handleCreateBundle`, `handlers/rules.go:handleCreateMappingRule`) to pass a mode — temporarily pass `""` (Step done properly in Task 22 where they read the global default). Compile only.

- [ ] **Step 9: Run the DB package + build**

Run: `cd backend && go build ./... && go test ./internal/db/ -run 'TestMigration000017|Bundle|MappingRule' 2>&1 | tail -20`
Expected: builds; guard passes.

- [ ] **Step 10: Commit**

```bash
git add backend/db/migrations/000017_confirmation_mode.*.sql backend/internal/db/config_settings.go backend/internal/db/config_settings_migration_test.go backend/internal/models/ backend/internal/db/bundles.go backend/internal/db/rules.go backend/internal/handlers/bundles.go backend/internal/handlers/rules.go
git commit -m "feat(db): 000017 confirmation_mode + config_settings repo (sub-phase 3, Task 19)"
```

---

## Task 20 — Cascade ADD machinery + auto/manual drain + add-side trigger points

Adds only (bundle assign, role-to-bundle, rule create). Adds need no other-source check; the drain's already-exists short-circuit (`409→applied`) makes them idempotent. This task builds the reusable machinery (`db.enqueueCascadeRows` + the atomic `*AndEnqueue` fns, `propagation.DrainBatch`, the `services/cascade.go` orchestrator with the auto/manual decision) and wires the three add triggers.

**Files:**
- Create: `backend/internal/db/cascade.go`, `cascade_migration_test.go`
- Create: `backend/internal/services/propagation/drainbatch.go`, `drainbatch_test.go`
- Create: `backend/internal/services/cascade.go`, `cascade_test.go`
- Modify: `backend/internal/handlers/bundles.go`, `rules.go`, `deps.go`; `services/deps.go`

**Interfaces produced:**
- `db.enqueueCascadeRows(ctx, tx, params []db.EnqueueParams) (outboxIDs []string, err error)` — tx-scoped outbox inserts (source/source_ref), no ledger write; used inside the atomic `*AndEnqueue` fns.
- `db.GetUsersForBundle(ctx, bundleID string) ([]string, error)`.
- `db.GetUsersWithRole(ctx, projectID, roleKey string) ([]string, error)` — from the webhook grant index.
- `db.AssignBundleAndEnqueue(ctx, userID, bundleID string, params []EnqueueParams) ([]string, error)` — one tx: assign + audit + outbox rows (**no `direct_role_grants` write**).
- `db.AddRoleToBundleAndEnqueue(ctx, bundleID, projectID, roleKey string, params []EnqueueParams) ([]string, error)`.
- `propagation.DrainBatch(ctx, ids []string) (DrainResult, error)`.
- `services.CascadeBundleAssignedToUser(ctx, actor, userID, bundleID string) (services.CascadeResult, error)`.
- `services.CascadeRoleAddedToBundle(ctx, actor, bundleID, projectID, roleKey string) (services.CascadeResult, error)`.
- `services.CascadeRuleCreated(ctx, actor, sourceProject, sourceRole, targetProject, targetRole, mode string) (ruleID string, res services.CascadeResult, err error)` — creates the rule + enqueues atomically.
- `type services.CascadeResult struct { Enqueued int; Mode string; Drain propagation.DrainResult }`.

> **Design pivot (review-driven, load-bearing):** `direct_role_grants` has `UNIQUE (user_id, zitadel_project_id, zitadel_role_key)` and `enqueueWrites` upserts `ON CONFLICT … source=EXCLUDED.source` — so the ledger holds ONE last-writer row per triple, and cannot represent "granted by bundle X *and* rule Y." Therefore **cascades do NOT write `direct_role_grants` rows.** Bundle/rule intent already lives in `user_bundle_assignments`/`bundle_roles`/`mapping_rules`, which `collectUserRoles` reads directly for access; the ledger row would be redundant and would clobber operator `source='direct'` rows (breaking the coverage check). Attribution moves to the **outbox** row's new `source`/`source_ref` (migration 000017). This means: (a) `OtherSourceCovers`' `source='direct'` filter is reliable (operator rows are never overwritten by a cascade); (b) an applied cascade revoke's `ReconcileLedgerOnApplied` deletes a triple only when no covering source exists (we suppressed the revoke otherwise), so it never strips an operator grant.

> **Atomicity (fixes review P1c):** the source mutation and its outbox rows commit in ONE tx (mirroring sub-phase 1's `db.EnqueueDirectGrantPropagation` / `ApproveRequestAndEnqueue`). A committed bundle/rule mutation therefore always has its outbox rows — no path leaves Zitadel permanently stale behind a `200 OK`. Because the fan-out is bounded (≤ ~200 rows at the stated scale, well under one tx's comfort), the whole cascade fits in a single tx; the 500-row `pgx.Tx` batching from design §5 only applies if a fan-out could ever exceed a few hundred, which it can't here. The **drain** stays best-effort after commit: an auto row that fails to drain remains a `pending` outbox row visible in the Pending Propagation worklist and recoverable via "Resume now."

### 20a — `db` atomic enqueue helper + membership queries

- [ ] **Step 1: Implement the shared outbox-insert helper + `GetUsersForBundle`/`GetUsersWithRole`**

`backend/internal/db/cascade.go`:
```go
package db

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// enqueueCascadeRows inserts one outbox row per param (with source/source_ref) on an EXISTING
// tx and returns the outbox ids. It writes NO direct_role_grants row — cascade intent lives in
// the bundle/rule tables (see design pivot). Each row still gets an idempotency key.
func enqueueCascadeRows(ctx context.Context, tx pgx.Tx, params []EnqueueParams) ([]string, error) {
	const insertOutbox = `
		INSERT INTO pending_zitadel_propagations
			(op_type, user_id, project_id, role_keys, zitadel_grant_id, payload_json,
			 idempotency_key, initiated_by, source, source_ref)
		VALUES ($1,$2,$3,$4,NULLIF($5,''),$6,$7,$8,$9,NULLIF($10,''))
		RETURNING id`
	ids := make([]string, 0, len(params))
	for _, p := range params {
		key, err := newOutboxIdempotencyKey()
		if err != nil {
			return nil, err
		}
		src := p.Source
		if src == "" {
			src = "direct"
		}
		var id string
		if err := tx.QueryRow(ctx, insertOutbox, p.OpType, p.UserID, p.ProjectID, p.RoleKeys,
			p.ZitadelGrantID, jsonOrEmpty(p.PayloadJSON), key, p.GrantedBy, src, p.SourceRef).Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func jsonOrEmpty(s string) string { if s == "" { return "{}" }; return s }

// GetUsersForBundle lists the user ids currently assigned to a bundle.
func GetUsersForBundle(ctx context.Context, bundleID string) ([]string, error) {
	return scanUserIDs(ctx, `SELECT user_id FROM user_bundle_assignments WHERE bundle_id = $1`, bundleID)
}

// GetUsersWithRole lists user ids that hold (projectID, roleKey) per the webhook-derived grant
// index (Zitadel's current-state mirror). The index may lag; the reconciliation sweep backstops.
func GetUsersWithRole(ctx context.Context, projectID, roleKey string) ([]string, error) {
	return scanUserIDs(ctx,
		`SELECT DISTINCT user_id FROM zitadel_grants_index WHERE project_id = $1 AND $2 = ANY(role_keys)`,
		projectID, roleKey)
}

func scanUserIDs(ctx context.Context, q string, args ...any) ([]string, error) {
	rows, err := PG.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []string
	for rows.Next() {
		var u string
		if err := rows.Scan(&u); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}
```
> Read `db/webhooks.go` for the exact `zitadel_grants_index` column names (`project_id`, `role_keys`) and match them; read `propagation_enqueue.go`'s `insertOutbox` to confirm the base column list before appending `source, source_ref`.

- [ ] **Step 2: Implement the atomic add-side `*AndEnqueue` functions** (mutation + outbox in one tx)

```go
// AssignBundleAndEnqueue assigns a bundle to a user and enqueues its cascade outbox rows in
// ONE tx, so a committed assignment always has its projection rows.
func AssignBundleAndEnqueue(ctx context.Context, actor, userID, bundleID string, params []EnqueueParams) ([]string, error) {
	tx, err := PG.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx,
		`INSERT INTO user_bundle_assignments (user_id, bundle_id) VALUES ($1,$2)
		 ON CONFLICT DO NOTHING`, userID, bundleID); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO audit_logs (actor_zitadel_user_id, target_zitadel_user_id, action, resource_id)
		 VALUES ($1,$2,'bundle.assigned',$3)`, actor, userID, bundleID); err != nil {
		return nil, err
	}
	ids, err := enqueueCascadeRows(ctx, tx, params)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return ids, nil
}

// AddRoleToBundleAndEnqueue adds a role to a bundle and enqueues the per-member cascade in one tx.
func AddRoleToBundleAndEnqueue(ctx context.Context, actor, bundleID, projectID, roleKey string, params []EnqueueParams) ([]string, error) {
	tx, err := PG.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx,
		`INSERT INTO bundle_roles (bundle_id, zitadel_project_id, zitadel_role_key) VALUES ($1,$2,$3)
		 ON CONFLICT DO NOTHING`, bundleID, projectID, roleKey); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO audit_logs (actor_zitadel_user_id, target_zitadel_user_id, action, resource_id)
		 VALUES ($1,$2,'bundle.role_added',$3)`, actor, bundleID, projectID+"/"+roleKey); err != nil {
		return nil, err
	}
	ids, err := enqueueCascadeRows(ctx, tx, params)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return ids, nil
}
```
> These take `actor` explicitly (all cascade params in a batch share it) and pass it to the audit write. `CreateMappingRuleAndEnqueue` (Task 21f uses the update sibling) follows the same shape: one tx = insert rule + per-holder outbox + audit.

- [ ] **Step 4: `go build ./...`**, commit db layer.

### 20b — `propagation.DrainBatch`

- [ ] **Step 5: Write the failing test** (`drainbatch_test.go`)

Mirror `DrainOne`'s test setup (`resetPropagationDeps()` swap). Assert: `DrainBatch` takes the advisory lock once, preflights reachability once, and calls `processRow` once per id; a Halted lock returns `Halted{Reason:"drain_in_progress"}`; offline returns `Halted{Reason:"zitadel_offline"}`; only the passed ids are claimed (a queued *manual* row with a different id is never touched).
```go
func TestDrainBatch_ProcessesOnlyGivenIDs(t *testing.T) {
	defer resetPropagationDeps()
	var claimed []string
	acquireDrainLock = func(ctx context.Context) (func(), bool, error) { return func() {}, true, nil }
	zitadelReachable = func(ctx context.Context) bool { return true }
	claimOne = func(ctx context.Context, id string) (*models.PendingPropagation, bool, error) {
		claimed = append(claimed, id)
		return &models.PendingPropagation{ID: id, OpType: "add"}, true, nil
	}
	// with empty RoleKeys, alreadyExists short-circuits to applied; markApplied no-ops
	markApplied = func(ctx context.Context, id string) error { return nil }
	grantIndexHasRole = func(ctx context.Context, u, p, r string) (bool, error) { return true, nil }

	res, err := DrainBatch(context.Background(), []string{"a", "b"})
	if err != nil { t.Fatal(err) }
	if got := strings.Join(claimed, ","); got != "a,b" {
		t.Fatalf("claimed = %q, want a,b", got)
	}
	if res.Applied != 2 { t.Fatalf("applied = %d, want 2", res.Applied) }
}
```
(Imports: `models` for `models.PendingPropagation`. Match the real dep signatures — see `deps.go`: `acquireDrainLock` returns `(func(), bool, error)`, `claimOne` returns `(*models.PendingPropagation, bool, error)`.)

- [ ] **Step 6: Run, verify fail** (`DrainBatch` undefined).

- [ ] **Step 7: Implement `DrainBatch`** — copy `DrainOne`'s lock/preflight/claim structure exactly, looping over ids:

`backend/internal/services/propagation/drainbatch.go`:
```go
package propagation

import (
	"context"
	"fmt"
)

// DrainBatch applies ONLY the outbox rows whose ids are given, under one advisory lock and one
// reachability preflight (a per-id DrainOne loop would re-lock and re-preflight every row —
// wasteful on a 200-row cascade). It reuses Drain/DrainOne's per-row processing verbatim. Rows
// not in `ids` (e.g. queued manual-mode rows) are never claimed, so an auto cascade drains its
// own rows without touching a manual operator's queued mutations. Mirrors DrainOne's real
// signatures: acquireDrainLock → (release, acquired, err); claimOne → (*PendingPropagation, found, err);
// processRow is a method on *DrainResult returning halt.
func DrainBatch(ctx context.Context, ids []string) (DrainResult, error) {
	var res DrainResult
	if len(ids) == 0 {
		return res, nil
	}
	release, acquired, err := acquireDrainLock(ctx)
	if err != nil {
		return DrainResult{}, fmt.Errorf("acquire drain lock: %w", err)
	}
	if !acquired {
		return DrainResult{Halted: true, Reason: "drain_in_progress"}, nil
	}
	defer release()
	if !zitadelReachable(ctx) {
		return DrainResult{Halted: true, Reason: "zitadel_offline"}, nil
	}
	for _, id := range ids {
		row, found, err := claimOne(ctx, id)
		if err != nil || !found {
			continue // already terminal, gone, or unclaimable — skip
		}
		if halt := res.processRow(ctx, *row); halt {
			break // retry budget exceeded (same halt semantics as Drain)
		}
	}
	return res, nil
}
```

- [ ] **Step 8: Run test, verify pass. `go test ./internal/services/propagation/...`. Commit.**

### 20b′ — Source-aware drain reconcile (fixes review P1: cascade revoke must not delete operator grants)

The drain's `applyRow` reconciles the ledger on every applied revoke via `reconcileLedger` (= `db.ReconcileLedgerOnApplied`), and its **revoke** branch deletes `direct_role_grants` by `(user, project, role)` **without a source filter** (`db/propagations.go` — the `replace` branch is already scoped to `source='direct'`, but `revoke` is not). Since cascades write no ledger rows, a cascade-source revoke owns nothing to clean — but the current unscoped delete would strip an **operator** `source='direct'` row that happens to share the triple (e.g. an operator direct grant added after the coverage check, before the drain). Fix: carry the outbox row's `source` into the drain and scope the revoke delete by it.

- [ ] **Step 9a: Add `Source` + `SourceRef` to `models.PendingPropagation`** (after `RoleKeys`):
```go
	Source    string `json:"source"`               // direct | bundle | rule | external_backfill | lifecycle_cascade
	SourceRef string `json:"source_ref,omitempty"`  // bundle/rule id for cascade rows; drives worklist attribution
```
`Source` drives the source-scoped reconcile below; `SourceRef` is surfaced so the Pending worklist and Recent-cascades UIs can name *which* bundle/rule caused a row (review P2 — attribution incomplete otherwise). The drain itself doesn't read `SourceRef`; it rides along for the UI.

- [ ] **Step 9b: Thread `source`/`source_ref` through the claim/scan SELECTs** in `db/propagations.go`: add `source, COALESCE(source_ref,'')` to the column lists of `ClaimPendingPropagations` (`RETURNING`), `ClaimPropagationByID` (`RETURNING`), and `GetPendingPropagations` (`SELECT`), and add `&p.Source, &p.SourceRef` to `scanPropagations`' `Scan` (in the same position). Pre-000017 rows read as `'direct'`/`''` (column default / NULL), so operator revokes keep their current behavior and the worklist gains bundle/rule attribution for free.

- [ ] **Step 9c: Make `ReconcileLedgerOnApplied` source-scoped on revoke.** Change its signature to `ReconcileLedgerOnApplied(ctx, opType, userID, projectID string, roleKeys []string, source string)` and the revoke SQL to:
```go
	case "revoke":
		const q = `DELETE FROM direct_role_grants
			WHERE user_id=$1 AND zitadel_project_id=$2 AND zitadel_role_key = ANY($3) AND source=$4`
		if _, err := PG.Exec(ctx, q, userID, projectID, roleKeys, source); err != nil { ... }
```
A cascade revoke (`source='bundle'|'rule'`) now deletes nothing (cascades own no ledger rows → operator rows untouched); an operator revoke (`source='direct'`) deletes exactly its own row, identical to today (pre-sub-phase-3 every row is `source='direct'`, so this is backward-compatible). Update `applyRow` to pass `row.Source`, and the `reconcileLedger` injectable signature + its `deps.go` binding + any drain tests that call it.

- [ ] **Step 9d: Tests** — a `processRow`/`applyRow` drain test: an applied `revoke` with `Source="bundle"` calls `reconcileLedger` with `source="bundle"` and does NOT delete a `source='direct'` row for the same triple; an applied `revoke` with `Source="direct"` still deletes its row. Run `go test ./internal/services/propagation/... ./internal/db/...`; commit.

### 20c — `services/cascade.go` orchestrator + auto/manual decision

- [ ] **Step 9: Write failing tests** (`cascade_test.go`) with a `resetCascadeDeps()` helper. Cover the three spec scenarios for adds:
  - *Adding a user to a bundle cascades one outbox row per bundle role* — assert `svcAssignBundleAndEnqueue` receives N params (one per role), each `Source="bundle"`, `SourceRef=bundleID`, `OpType="add"`.
  - *Auto-mode drains immediately* — bundle mode `auto` ⇒ `DrainBatch` called with the enqueued ids; result `Mode="auto"`.
  - *Manual-mode queues* — bundle mode `manual` ⇒ `DrainBatch` NOT called; rows left pending; `Mode="manual"`.

```go
func TestCascadeBundleAssignedToUser_AutoEnqueuesPerRoleAndDrains(t *testing.T) {
	defer resetCascadeDeps()
	svcGetBundleByID = func(ctx context.Context, id string) (models.Bundle, error) {
		return models.Bundle{ID: id, ConfirmationMode: "auto"}, nil
	}
	svcCascGetRolesForBundle = func(ctx context.Context, id string) ([]models.BundleRole, error) {
		return []models.BundleRole{
			{ProjectID: "p1", RoleKey: "r1"}, {ProjectID: "p1", RoleKey: "r2"},
		}, nil
	}
	var enqueued []db.EnqueueParams
	svcAssignBundleAndEnqueue = func(ctx context.Context, actor, userID, bundleID string, ps []db.EnqueueParams) ([]string, error) {
		enqueued = ps
		return []string{"o1", "o2"}, nil
	}
	var drainedIDs []string
	svcDrainBatch = func(ctx context.Context, ids []string) (propagation.DrainResult, error) {
		drainedIDs = ids
		return propagation.DrainResult{Applied: 2}, nil
	}

	res, err := CascadeBundleAssignedToUser(context.Background(), "admin", "u1", "b1")
	if err != nil { t.Fatal(err) }
	if len(enqueued) != 2 { t.Fatalf("enqueued %d params, want 2", len(enqueued)) }
	for _, p := range enqueued {
		if p.Source != "bundle" || p.SourceRef != "b1" || p.OpType != "add" || p.UserID != "u1" {
			t.Fatalf("bad param: %+v", p)
		}
	}
	if strings.Join(drainedIDs, ",") != "o1,o2" { t.Fatalf("drained %v", drainedIDs) }
	if res.Mode != "auto" { t.Fatalf("mode = %q", res.Mode) }
}

func TestCascadeBundleAssignedToUser_ManualQueuesWithoutDrain(t *testing.T) {
	defer resetCascadeDeps()
	svcGetBundleByID = func(ctx context.Context, id string) (models.Bundle, error) {
		return models.Bundle{ID: id, ConfirmationMode: "manual"}, nil
	}
	svcCascGetRolesForBundle = func(ctx context.Context, id string) ([]models.BundleRole, error) {
		return []models.BundleRole{{ProjectID: "p1", RoleKey: "r1"}}, nil
	}
	svcAssignBundleAndEnqueue = func(ctx context.Context, actor, userID, bundleID string, ps []db.EnqueueParams) ([]string, error) {
		return []string{"o1"}, nil
	}
	drainCalled := false
	svcDrainBatch = func(ctx context.Context, ids []string) (propagation.DrainResult, error) {
		drainCalled = true; return propagation.DrainResult{}, nil
	}
	res, err := CascadeBundleAssignedToUser(context.Background(), "admin", "u1", "b1")
	if err != nil { t.Fatal(err) }
	if drainCalled { t.Fatal("manual mode must not drain") }
	if res.Mode != "manual" { t.Fatalf("mode = %q", res.Mode) }
}
```

- [ ] **Step 10: Run, verify fail. Implement `services/cascade.go`**

```go
package services

import (
	"context"
	"log"

	"syndra/internal/db"
	"syndra/internal/models"
	"syndra/internal/services/propagation"
)

// CascadeResult reports what a cascade enqueued and (auto only) drained.
type CascadeResult struct {
	Enqueued int                      `json:"enqueued"`
	Mode     string                   `json:"mode"`
	Drain    propagation.DrainResult  `json:"drain"`
}

// --- injectable deps (swapped in cascade_test.go) ---
var (
	svcGetBundleByID = func(ctx context.Context, id string) (models.Bundle, error) {
		return db.GetBundleByID(ctx, id)
	}
	svcCascGetRolesForBundle = db.GetRolesForBundle
	svcGetUsersForBundle     = db.GetUsersForBundle
	svcGetUsersWithRole      = db.GetUsersWithRole
	svcGetMappingRuleByID    = db.GetMappingRuleByID
	svcDrainBatch            = propagation.DrainBatch

	// atomic mutation+enqueue (one tx each — see design pivot)
	svcAssignBundleAndEnqueue    = db.AssignBundleAndEnqueue
	svcAddRoleToBundleAndEnqueue = db.AddRoleToBundleAndEnqueue
	svcCreateRuleAndEnqueue      = db.CreateMappingRuleAndEnqueue
)

// applyMode drains the JUST-enqueued rows when the source is auto; manual leaves them pending.
// The rows were already persisted atomically with the source mutation by the caller, so a drain
// failure is NON-FATAL: it is captured in res.Drain (Halted/Reason/Errored) and applyMode returns
// nil, so the handler still responds 200 — the rows sit pending in the worklist, recoverable via
// "Resume now". (Returning derr here would 500 AFTER the mutation committed, inviting a retry that
// re-runs the whole cascade and mints duplicate outbox rows — review P2.)
func applyMode(ctx context.Context, mode string, ids []string) (CascadeResult, error) {
	mode = db.NormalizeConfirmationMode(mode)
	res := CascadeResult{Mode: mode, Enqueued: len(ids)}
	if mode == "auto" && len(ids) > 0 {
		dr, derr := svcDrainBatch(ctx, ids)
		res.Drain = dr
		if derr != nil {
			// non-fatal: log and surface via res.Drain; rows remain pending
			log.Printf("[CASCADE] auto-drain of %d row(s) failed (left pending): %v", len(ids), derr)
			res.Drain.Halted = true
			if res.Drain.Reason == "" {
				res.Drain.Reason = "drain_error"
			}
		}
	}
	return res, nil
}

// CascadeBundleAssignedToUser assigns the bundle AND enqueues one add per bundle role in one tx
// (atomic — no committed assignment without its outbox rows), then (auto) drains those rows.
func CascadeBundleAssignedToUser(ctx context.Context, actor, userID, bundleID string) (CascadeResult, error) {
	bundle, err := svcGetBundleByID(ctx, bundleID)
	if err != nil {
		return CascadeResult{}, err
	}
	roles, err := svcCascGetRolesForBundle(ctx, bundleID)
	if err != nil {
		return CascadeResult{}, err
	}
	params := make([]db.EnqueueParams, 0, len(roles))
	for _, ro := range roles {
		params = append(params, db.EnqueueParams{
			UserID: userID, ProjectID: ro.ProjectID, RoleKeys: []string{ro.RoleKey},
			GrantedBy: actor, Reason: "Bundle membership cascade",
			Source: "bundle", SourceRef: bundleID, OpType: "add", PayloadJSON: "{}",
		})
	}
	ids, err := svcAssignBundleAndEnqueue(ctx, actor, userID, bundleID, params)
	if err != nil {
		return CascadeResult{}, err // enqueue+assign rolled back together → handler returns 500
	}
	return applyMode(ctx, bundle.ConfirmationMode, ids)
}

// CascadeRoleAddedToBundle adds the role to the bundle AND enqueues one add per assigned user, in
// one tx, then (auto) drains.
func CascadeRoleAddedToBundle(ctx context.Context, actor, bundleID, projectID, roleKey string) (CascadeResult, error) {
	bundle, err := svcGetBundleByID(ctx, bundleID)
	if err != nil {
		return CascadeResult{}, err
	}
	users, err := svcGetUsersForBundle(ctx, bundleID)
	if err != nil {
		return CascadeResult{}, err
	}
	params := make([]db.EnqueueParams, 0, len(users))
	for _, u := range users {
		params = append(params, db.EnqueueParams{
			UserID: u, ProjectID: projectID, RoleKeys: []string{roleKey},
			GrantedBy: actor, Reason: "Bundle role-add cascade",
			Source: "bundle", SourceRef: bundleID, OpType: "add", PayloadJSON: "{}",
		})
	}
	ids, err := svcAddRoleToBundleAndEnqueue(ctx, actor, bundleID, projectID, roleKey, params)
	if err != nil {
		return CascadeResult{}, err
	}
	return applyMode(ctx, bundle.ConfirmationMode, ids)
}

// CascadeRuleCreated creates the rule AND enqueues the target for every user holding the source,
// in one tx, then (auto) drains. Source-holders are fetched before the tx (they do not depend on
// the new rule row); the tx inserts the rule, mints source_ref=<new id>, and enqueues. Returns
// the new rule id for the handler response. The handler does cycle/self-ref validation first.
func CascadeRuleCreated(ctx context.Context, actor, sourceProject, sourceRole, targetProject, targetRole, mode string) (string, CascadeResult, error) {
	holders, err := svcGetUsersWithRole(ctx, sourceProject, sourceRole)
	if err != nil {
		return "", CascadeResult{}, err
	}
	ruleID, ids, err := svcCreateRuleAndEnqueue(ctx, actor,
		sourceProject, sourceRole, targetProject, targetRole,
		db.NormalizeConfirmationMode(mode), holders)
	if err != nil {
		return "", CascadeResult{}, err
	}
	res, err := applyMode(ctx, mode, ids)
	return ruleID, res, err
}
```
> `db.CreateMappingRuleAndEnqueue(ctx, actor, sp, sr, tp, tr, mode string, holders []string) (ruleID string, outboxIDs []string, err error)` — one tx: `INSERT mapping_rules(... source_zitadel_project_id, source_zitadel_role_key, target_zitadel_project_id, target_zitadel_role_key, confirmation_mode ...) RETURNING id`, then one `add` outbox row per holder (`source='rule'`, `source_ref=<id>`, project/role = target), plus audit. This replaces a bare `db.CreateMappingRule` call in the create handler so a committed rule always has its projection rows (fixes review P1c for rules too).

**Actor** is passed explicitly to every cascade fn (the handler computes `actor := getAdminUserID(r.Context())` for its audit and passes it through) — avoids a `services`→handlers ctx-key tangle. All orchestrators use the `(ctx, actor, …)` shape.

- [ ] **Step 11: Add `db.GetBundleByID` and `db.GetMappingRuleByID` if absent**

Grep `db/bundles.go` / `db/rules.go`. If no single-row getter exists, add:
```go
// db/bundles.go
func GetBundleByID(ctx context.Context, id string) (models.Bundle, error) {
	var b models.Bundle
	err := PG.QueryRow(ctx,
		`SELECT id, name, description, is_welcome, confirmation_mode, created_at
		 FROM bundles WHERE id = $1`, id).
		Scan(&b.ID, &b.Name, &b.Description, &b.IsWelcome, &b.ConfirmationMode, &b.CreatedAt)
	return b, err
}
// db/rules.go — NOTE the real column names are source_zitadel_project_id / source_zitadel_role_key
// / target_zitadel_project_id / target_zitadel_role_key (the Go struct fields are Source*/Target*).
func GetMappingRuleByID(ctx context.Context, id string) (models.MappingRule, error) {
	var r models.MappingRule
	err := PG.QueryRow(ctx,
		`SELECT id, source_zitadel_project_id, source_zitadel_role_key,
		        target_zitadel_project_id, target_zitadel_role_key, confirmation_mode, created_at
		 FROM mapping_rules WHERE id = $1`, id).
		Scan(&r.ID, &r.SourceProject, &r.SourceRole, &r.TargetProject, &r.TargetRole, &r.ConfirmationMode, &r.CreatedAt)
	return r, err
}
```
> Confirm `bundles` columns for `GetBundleByID` against `db/bundles.go` (`GetAllBundles`' SELECT list) — the bundle columns are plain (`id,name,description,is_welcome,...`); only `mapping_rules` uses the `*_zitadel_*` names.

- [ ] **Step 12: Run cascade tests, verify pass.**

Run: `cd backend && go test ./internal/services/ -run Cascade -v`

### 20d — Wire the three add triggers into handlers

- [ ] **Step 13: Add injectables to `handlers/deps.go`**
```go
	svcCascadeBundleAssigned = services.CascadeBundleAssignedToUser // (ctx, actor, userID, bundleID)
	svcCascadeRoleAdded      = services.CascadeRoleAddedToBundle    // (ctx, actor, bundleID, projectID, roleKey)
	svcCascadeRuleCreated    = services.CascadeRuleCreated          // (ctx, actor, sp, sr, tp, tr, mode) → (ruleID, res, err)
```
The cascade **owns the source mutation now** (assign/add-role/create happen inside the atomic `*AndEnqueue` tx), so these handlers **no longer call** `dbAssignBundleToUser`/`dbAddRoleToBundle`/`dbCreateMappingRule` separately — remove those calls. The enqueue is fatal (the mutation rolled back with it); the drain is not.

- [ ] **Step 14: Rewrite `handleAssignBundleToUser`** — validate → resolve actor → call the cascade (which does assign+enqueue atomically, then drains if auto). Enqueue failure → 500 (nothing committed); drain failure is captured in `cascade.drain`, not the error:
```go
	actor := getAdminUserID(r.Context())
	if actor == "" { actor = "system" }
	cascade, err := svcCascadeBundleAssigned(r.Context(), actor, userID, req.BundleID)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "CASCADE_ERROR", err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, map[string]any{
		"message": "Bundle assigned to user",
		"cascade": cascade, // {enqueued, mode, drain}
	})
```

- [ ] **Step 15: Rewrite `handleAddRoleToBundle`** analogously: validate → `cascade, err := svcCascadeRoleAdded(r.Context(), actor, bundleID, req.ProjectID, req.RoleKey)`; 500 on `err`; respond `{message, cascade}`. Remove the separate `dbAddRoleToBundle` call.

- [ ] **Step 16: Rewrite `handleCreateMappingRule`** — keep its validation (self-ref, cycle via `dbDetectCycleOnInsert`), resolve `mode` from the request body or `db.GetConfigSetting(ctx, db.ConfigKeyDefaultConfirmationMode)` (Task 22 wires the default read), then:
```go
	ruleID, cascade, err := svcCascadeRuleCreated(r.Context(), actor, req.SourceProject, req.SourceRole, req.TargetProject, req.TargetRole, mode)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "CASCADE_ERROR", err.Error())
		return
	}
	jsonResponse(w, http.StatusCreated, map[string]any{"id": ruleID, "message": "Mapping rule created", "cascade": cascade})
```
Remove the separate `dbCreateMappingRule` call (the atomic `CreateMappingRuleAndEnqueue` inside the cascade replaces it).

- [ ] **Step 17: Handler tests** (`bundles_test.go`, `rules_test.go`): stub the `svcCascade*` injectables; assert (a) they receive the right args, (b) the response includes `cascade`/`id`, (c) an enqueue error → 500. Add `resetCascadeHandlerDeps()` if needed.

- [ ] **Step 18: Full backend gate + commit**

Run: `cd backend && go test ./... && go vet ./... && gofmt -l internal/ | head`
Expected: all pass, vet clean, gofmt empty.
```bash
git commit -am "feat(cascade): bundle/rule add-cascade machinery + auto/manual drain (sub-phase 3, Task 20)"
```

---

## Task 21 — Cascade REVOKE with other-source check (remove handlers + 6th trigger)

Revokes need an **other-source check** computed from the *other* sources explicitly (design §4.7 "intent-ledger lookup"): a triple is covered iff a `source='direct'` grant, a *different* bundle, or a *different* rule still yields it. This check is reliable **because cascades don't write `direct_role_grants` rows** (design pivot in Task 20) — so a `source='direct'` row is always a genuine operator grant, never a cascade's overwrite. The revoke path is atomic (mutation + revoke outbox rows in one tx). The small coverage race (a concurrent add between the check and the delete-commit) is reconciliation-resolved (design §7 Q4).

**Files:**
- Create: `backend/internal/handlers/cascade_surfaces.go` (remove handlers), `cascade_surfaces_test.go`
- Modify: `backend/internal/services/cascade.go` (`OtherSourceCovers` + revoke + rule-update orchestrators), `cascade_test.go`; `db/cascade.go` (atomic `RemoveBundleFromUserAndEnqueue`, `RemoveRoleFromBundleAndEnqueue`, `UpdateMappingRuleAndEnqueue`); `db/bundles.go` (`RemoveRoleFromBundle` used inside the atomic fn); `db/rules.go` (`UpdateMappingRule` used inside the atomic fn); `handlers/rules.go` (`handleUpdateMappingRule`); `handlers/deps.go`; `router.go`

**Interfaces produced:**
- `services.OtherSourceCovers(ctx, userID, projectID, role, excludeBundleID, excludeRuleID string) (bool, error)` — coverage by a source *other than* the excluded bundle/rule (single-hop rules; sweep backstops deeper chains).
- `services.CascadeBundleRemovedFromUser(ctx, actor, userID, bundleID string) (CascadeResult, error)` — deletes the assignment + enqueues revokes atomically.
- `services.CascadeRoleRemovedFromBundle(ctx, actor, bundleID, projectID, roleKey string) (CascadeResult, error)` — deletes the bundle_role + enqueues revokes atomically.
- `services.CascadeRuleUpdated(ctx, actor, old models.MappingRule, sp, sr, tp, tr string) (CascadeResult, error)` — 6th trigger; takes the **pre-update** rule (captured by the handler) + the new fields; updates + enqueues atomically.
- `db.RemoveBundleFromUserAndEnqueue(ctx, actor, userID, bundleID string, params []EnqueueParams) ([]string, error)` — one tx: delete assignment + audit + revoke outbox rows.
- `db.RemoveRoleFromBundleAndEnqueue(ctx, actor, bundleID, projectID, roleKey string, params []EnqueueParams) ([]string, error)`.
- `db.UpdateMappingRuleAndEnqueue(ctx, actor, id, sp, sr, tp, tr string, params []EnqueueParams) ([]string, error)` — one tx: update rule + audit + add/revoke outbox rows.
- Routes: `DELETE /api/v1/users/{id}/bundles/{bundleId}`, `DELETE /api/v1/bundles/{id}/roles/{projectId}/{roleKey}`, `PUT /api/v1/rules/mapping/{id}`.

### 21a — `OtherSourceCovers` (the explicit other-source check)

- [ ] **Step 1: Add the injectables + `OtherSourceCovers` to `services/cascade.go`**

```go
var (
	svcGetDirectGrantsForUser = db.GetDirectGrantsForUser // (ctx, userID, includeExpired bool) []models.DirectGrant
	svcGetBundlesForUser      = db.GetBundlesForUser
	svcGetActiveMappingRules  = db.GetActiveMappingRules
)

// OtherSourceCovers reports whether (projectID, role) is still granted to the user by a
// source OTHER than the excluded bundle/rule. Sources checked: an operator direct grant
// (source='direct'), any still-assigned bundle except excludeBundleID, and any active rule
// except excludeRuleID whose source the user holds (single hop). Because cascades do NOT
// write direct_role_grants rows (Task 20 pivot), a source='direct' row is always a genuine
// operator grant — the filter is reliable, never a cascade overwrite. Deeper rule chains are
// rare and caught by the reconciliation sweep (design §7 Q4: small race, reconciliation-resolved).
func OtherSourceCovers(ctx context.Context, userID, projectID, role, excludeBundleID, excludeRuleID string) (bool, error) {
	holder := map[HolderKey]bool{} // what the user holds from OTHER sources (for single-hop rule eval)

	// 1) operator direct grants (source='direct'; external_backfill/lifecycle also count as coverage)
	directs, err := svcGetDirectGrantsForUser(ctx, userID, false)
	if err != nil {
		return false, err
	}
	for _, g := range directs {
		// All direct_role_grants rows are genuine grants (cascades write none), so any row for
		// the triple is coverage; every row also feeds the single-hop rule holder set.
		if g.ProjectID == projectID && g.RoleKey == role {
			return true, nil
		}
		holder[HolderKey{UserID: userID, ProjectID: g.ProjectID, RoleKey: g.RoleKey}] = true
	}

	// 2) other bundles the user is still assigned to
	bundles, err := svcGetBundlesForUser(ctx, userID)
	if err != nil {
		return false, err
	}
	for _, b := range bundles {
		if b.ID == excludeBundleID {
			continue
		}
		roles, err := svcCascGetRolesForBundle(ctx, b.ID)
		if err != nil {
			return false, err
		}
		for _, ro := range roles {
			if ro.ProjectID == projectID && ro.RoleKey == role {
				return true, nil
			}
			holder[HolderKey{UserID: userID, ProjectID: ro.ProjectID, RoleKey: ro.RoleKey}] = true
		}
	}

	// 3) other active rules (single hop over what the user holds from 1+2)
	rules, err := svcGetActiveMappingRules(ctx)
	if err != nil {
		return false, err
	}
	for _, ru := range rules {
		if ru.ID == excludeRuleID {
			continue
		}
		if ru.TargetProject == projectID && ru.TargetRole == role &&
			holder[HolderKey{UserID: userID, ProjectID: ru.SourceProject, RoleKey: ru.SourceRole}] {
			return true, nil
		}
	}
	return false, nil
}
```
> Read `services/expected.go` for `HolderKey`'s exact field names (the digest shows `HolderKey{UserID, ProjectID, RoleKey}`) and `models.DirectGrant` for `ProjectID`/`RoleKey` field names; match them exactly.

### 21b — Revoke orchestrators

- [ ] **Step 2: Add the revoke orchestrators + `revokeParam` to `services/cascade.go`**

```go
var (
	svcRemoveBundleFromUserAndEnqueue = db.RemoveBundleFromUserAndEnqueue
	svcRemoveRoleFromBundleAndEnqueue = db.RemoveRoleFromBundleAndEnqueue
)

// CascadeBundleRemovedFromUser computes which of the bundle's roles are no longer covered by
// another source, then atomically deletes the assignment + enqueues those revokes, then (auto)
// drains. Coverage is computed with the bundle EXCLUDED, so it is correct to compute it before
// the delete (the exclude handles the not-yet-deleted assignment); any other concurrent change
// is a reconciliation-tolerated race (design §7 Q4).
func CascadeBundleRemovedFromUser(ctx context.Context, actor, userID, bundleID string) (CascadeResult, error) {
	bundle, err := svcGetBundleByID(ctx, bundleID)
	if err != nil {
		return CascadeResult{}, err
	}
	roles, err := svcCascGetRolesForBundle(ctx, bundleID)
	if err != nil {
		return CascadeResult{}, err
	}
	params := make([]db.EnqueueParams, 0, len(roles))
	for _, ro := range roles {
		covered, err := OtherSourceCovers(ctx, userID, ro.ProjectID, ro.RoleKey, bundleID, "")
		if err != nil {
			return CascadeResult{}, err
		}
		if covered {
			continue // another source keeps it — suppress revoke
		}
		params = append(params, revokeParam(actor, userID, bundleID, ro.ProjectID, ro.RoleKey))
	}
	ids, err := svcRemoveBundleFromUserAndEnqueue(ctx, actor, userID, bundleID, params)
	if err != nil {
		return CascadeResult{}, err
	}
	return applyMode(ctx, bundle.ConfirmationMode, ids)
}

// CascadeRoleRemovedFromBundle revokes (projectID, roleKey) for each still-assigned user no
// longer covered elsewhere, atomically deleting the bundle_role + enqueuing the revokes.
func CascadeRoleRemovedFromBundle(ctx context.Context, actor, bundleID, projectID, roleKey string) (CascadeResult, error) {
	bundle, err := svcGetBundleByID(ctx, bundleID)
	if err != nil {
		return CascadeResult{}, err
	}
	users, err := svcGetUsersForBundle(ctx, bundleID)
	if err != nil {
		return CascadeResult{}, err
	}
	params := make([]db.EnqueueParams, 0, len(users))
	for _, u := range users {
		covered, err := OtherSourceCovers(ctx, u, projectID, roleKey, bundleID, "")
		if err != nil {
			return CascadeResult{}, err
		}
		if covered {
			continue
		}
		params = append(params, revokeParam(actor, u, bundleID, projectID, roleKey))
	}
	ids, err := svcRemoveRoleFromBundleAndEnqueue(ctx, actor, bundleID, projectID, roleKey, params)
	if err != nil {
		return CascadeResult{}, err
	}
	return applyMode(ctx, bundle.ConfirmationMode, ids)
}

func revokeParam(actor, userID, bundleID, projectID, roleKey string) db.EnqueueParams {
	return db.EnqueueParams{
		UserID: userID, ProjectID: projectID, RoleKeys: []string{roleKey},
		GrantedBy: actor, Reason: "Bundle removal cascade",
		Source: "bundle", SourceRef: bundleID, OpType: "revoke", PayloadJSON: "{}",
	}
}
```
> `revoke` outbox rows resolve `zitadel_grant_id` in the drain (from the grant index); leaving `EnqueueParams.ZitadelGrantID` empty is fine. Confirm against `drain.go`'s revoke path; if it needs the id at enqueue, resolve via `db.GetGrantIndex(ctx, userID, projectID)` inside the atomic fn. Note the atomic remove fns delete even when `params` is empty (all roles covered) — the assignment/role deletion must still happen; the empty enqueue is a no-op and `applyMode` drains nothing.

- [ ] **Step 3: Write failing tests** (`cascade_test.go`) for the coverage scenario, stubbing `OtherSourceCovers`' injectables (NOT `collectUserRoles`):
```go
func TestCascadeBundleRemoved_SuppressesRevokeWhenAnotherSourceCovers(t *testing.T) {
	defer resetCascadeDeps()
	svcGetBundleByID = func(ctx context.Context, id string) (models.Bundle, error) {
		return models.Bundle{ID: id, ConfirmationMode: "auto"}, nil
	}
	svcCascGetRolesForBundle = func(ctx context.Context, id string) ([]models.BundleRole, error) {
		if id == "b1" { return []models.BundleRole{{ProjectID: "p1", RoleKey: "r1"}}, nil }
		return []models.BundleRole{{ProjectID: "p1", RoleKey: "r1"}}, nil // b2 also grants r1
	}
	svcGetDirectGrantsForUser = func(ctx context.Context, u string, inc bool) ([]models.DirectGrant, error) {
		return nil, nil
	}
	svcGetBundlesForUser = func(ctx context.Context, u string) ([]models.Bundle, error) {
		return []models.Bundle{{ID: "b2"}}, nil // still in b2, which covers r1
	}
	svcGetActiveMappingRules = func(ctx context.Context) ([]models.MappingRule, error) { return nil, nil }
	var passed []db.EnqueueParams
	svcRemoveBundleFromUserAndEnqueue = func(ctx context.Context, actor, userID, bundleID string, ps []db.EnqueueParams) ([]string, error) {
		passed = ps // the atomic fn still deletes the assignment even when ps is empty
		return nil, nil
	}
	res, err := CascadeBundleRemovedFromUser(context.Background(), "admin", "u1", "b1")
	if err != nil { t.Fatal(err) }
	if len(passed) != 0 { t.Fatalf("covered role must not enqueue a revoke, got %+v", passed) }
	if res.Enqueued != 0 { t.Fatalf("enqueued = %d, want 0", res.Enqueued) }
}

func TestCascadeBundleRemoved_RevokesWhenUncovered(t *testing.T) {
	defer resetCascadeDeps()
	svcGetBundleByID = func(ctx context.Context, id string) (models.Bundle, error) {
		return models.Bundle{ID: id, ConfirmationMode: "auto"}, nil
	}
	svcCascGetRolesForBundle = func(ctx context.Context, id string) ([]models.BundleRole, error) {
		return []models.BundleRole{{ProjectID: "p1", RoleKey: "r1"}}, nil
	}
	svcGetDirectGrantsForUser = func(ctx context.Context, u string, inc bool) ([]models.DirectGrant, error) { return nil, nil }
	svcGetBundlesForUser = func(ctx context.Context, u string) ([]models.Bundle, error) { return nil, nil } // no other bundle
	svcGetActiveMappingRules = func(ctx context.Context) ([]models.MappingRule, error) { return nil, nil }
	var got []db.EnqueueParams
	svcRemoveBundleFromUserAndEnqueue = func(ctx context.Context, actor, userID, bundleID string, ps []db.EnqueueParams) ([]string, error) { got = ps; return []string{"o1"}, nil }
	svcDrainBatch = func(ctx context.Context, ids []string) (propagation.DrainResult, error) { return propagation.DrainResult{}, nil }
	res, err := CascadeBundleRemovedFromUser(context.Background(), "admin", "u1", "b1")
	if err != nil { t.Fatal(err) }
	if len(got) != 1 || got[0].OpType != "revoke" || got[0].Source != "bundle" || got[0].SourceRef != "b1" {
		t.Fatalf("bad revoke params: %+v", got)
	}
	if res.Enqueued != 1 { t.Fatalf("enqueued = %d, want 1", res.Enqueued) }
}
```

- [ ] **Step 4: Run/fail, implement, run/pass.** `go test ./internal/services/ -run Cascade`.

### 21c — DB helpers + remove handlers

- [ ] **Step 5: Add the atomic remove functions + `RemoveRoleFromBundle`** (`db/cascade.go` / `db/bundles.go`):
```go
// db/bundles.go — the single-row delete, called inside the atomic fn.
func RemoveRoleFromBundle(ctx context.Context, bundleID, projectID, roleKey string) error {
	_, err := PG.Exec(ctx,
		`DELETE FROM bundle_roles
		 WHERE bundle_id = $1 AND zitadel_project_id = $2 AND zitadel_role_key = $3`,
		bundleID, projectID, roleKey)
	return err
}

// db/cascade.go — delete assignment + audit + revoke outbox rows in ONE tx (fixes review P1c:
// a committed removal always has its revoke rows).
func RemoveBundleFromUserAndEnqueue(ctx context.Context, actor, userID, bundleID string, params []EnqueueParams) ([]string, error) {
	tx, err := PG.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx,
		`DELETE FROM user_bundle_assignments WHERE user_id=$1 AND bundle_id=$2`, userID, bundleID); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO audit_logs (actor_zitadel_user_id, target_zitadel_user_id, action, resource_id)
		 VALUES ($1,$2,'bundle.unassigned',$3)`, actor, userID, bundleID); err != nil {
		return nil, err
	}
	ids, err := enqueueCascadeRows(ctx, tx, params)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return ids, nil
}

// RemoveRoleFromBundleAndEnqueue: delete bundle_role + audit + revoke outbox rows in one tx.
func RemoveRoleFromBundleAndEnqueue(ctx context.Context, actor, bundleID, projectID, roleKey string, params []EnqueueParams) ([]string, error) {
	tx, err := PG.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx,
		`DELETE FROM bundle_roles WHERE bundle_id=$1 AND zitadel_project_id=$2 AND zitadel_role_key=$3`,
		bundleID, projectID, roleKey); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO audit_logs (actor_zitadel_user_id, target_zitadel_user_id, action, resource_id)
		 VALUES ($1,$2,'bundle.role_removed',$3)`, actor, bundleID, projectID+"/"+roleKey); err != nil {
		return nil, err
	}
	ids, err := enqueueCascadeRows(ctx, tx, params)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return ids, nil
}
```

- [ ] **Step 6: Write the two remove handlers** in `handlers/cascade_surfaces.go`. The cascade OWNS the delete (inside the atomic fn), so the handler just validates → calls the cascade → 500 on error → responds:
```go
func handleRemoveBundleFromUser(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("id")
	bundleID := r.PathValue("bundleId")
	if strings.TrimSpace(userID) == "" || strings.TrimSpace(bundleID) == "" {
		jsonValidationErrorResponse(w, "id and bundleId are required", map[string]string{"path": "required"})
		return
	}
	actor := getAdminUserID(r.Context())
	if actor == "" { actor = "system" }
	cascade, err := svcCascadeBundleRemoved(r.Context(), actor, userID, bundleID)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "CASCADE_ERROR", err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, map[string]any{"message": "Bundle removed from user", "cascade": cascade})
}
```
(Write `handleRemoveRoleFromBundle` analogously with `r.PathValue("projectId")`/`r.PathValue("roleKey")` and `svcCascadeRoleRemoved`.)

- [ ] **Step 7: Register routes + injectables**

`router.go`:
```go
	mux.HandleFunc("DELETE /api/v1/users/{id}/bundles/{bundleId}", withCORS(withUserAuth(handleRemoveBundleFromUser)))
	mux.HandleFunc("DELETE /api/v1/bundles/{id}/roles/{projectId}/{roleKey}", withCORS(withUserAuth(handleRemoveRoleFromBundle)))
```
`deps.go`: `svcCascadeBundleRemoved=services.CascadeBundleRemovedFromUser`, `svcCascadeRoleRemoved=services.CascadeRoleRemovedFromBundle`. (No separate `dbRemove*` injectables in the handler — the delete lives inside the atomic cascade.)

- [ ] **Step 8: Handler tests** (`cascade_surfaces_test.go`): each remove handler deletes then cascades; a covered role yields `cascade.enqueued==0`; empty-path-param → 400.

- [ ] **Step 9: Backend gate + commit.**
```bash
git commit -am "feat(cascade): revoke cascade + explicit other-source check + remove handlers (sub-phase 3, Task 21a-c)"
```

### 21f — 6th trigger: mapping-rule matcher change (composition, no diff engine)

Because cascades write no `direct_role_grants` rows, there is no per-source ledger to query for "who has the target via this rule" — and the *old* target is not in the updated rule. The correct composition therefore captures the **pre-update rule** in the handler and passes it in:

- **Add pass** — every user holding the *new* source gets the *new* target (drain `409→applied` no-ops those who already had it).
- **Revoke pass** — every user holding the *old* source (`GetUsersWithRole(old.Source…)`, stable across the Syndra-side update since it reads the Zitadel grant index) should lose the *old* target, **unless** (a) the old target equals the new target and they are in the add set (re-added identically — skip), or (b) another source still covers the old target (`OtherSourceCovers(..., excludeRuleID=old.ID)`).

This handles source-only, target-only, and both changes correctly — the earlier "revoke uses the updated rule's new target" plan (review P1a) would have orphaned the old target and kept-plus-re-added users; capturing the old rule fixes both.

- [ ] **Step 10: Add `db.UpdateMappingRuleAndEnqueue`** (`db/rules.go` / `db/cascade.go`) — atomic update + enqueue, correct `*_zitadel_*` columns:
```go
// UpdateMappingRuleAndEnqueue updates the rule matcher/target AND enqueues the add/revoke
// cascade rows in ONE tx (fixes review P1c for rule updates). Note the real column names.
func UpdateMappingRuleAndEnqueue(ctx context.Context, actor, id, sp, sr, tp, tr string, params []EnqueueParams) ([]string, error) {
	tx, err := PG.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx,
		`UPDATE mapping_rules
		 SET source_zitadel_project_id=$2, source_zitadel_role_key=$3,
		     target_zitadel_project_id=$4, target_zitadel_role_key=$5
		 WHERE id=$1`, id, sp, sr, tp, tr); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO audit_logs (actor_zitadel_user_id, target_zitadel_user_id, action, resource_id)
		 VALUES ($1,'','mapping_rule.updated',$2)`, actor, id); err != nil {
		return nil, err
	}
	ids, err := enqueueCascadeRows(ctx, tx, params)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return ids, nil
}
```

- [ ] **Step 11: Add `CascadeRuleUpdated` to `services/cascade.go`** — takes the pre-update rule + new fields:
```go
var svcUpdateRuleAndEnqueue = db.UpdateMappingRuleAndEnqueue

// CascadeRuleUpdated re-projects a rule whose matcher/target changed. `old` is the rule as it
// was BEFORE the update (captured by the handler); sp/sr/tp/tr are the NEW fields. The update
// and the cascade commit atomically. Revoke targets the OLD (project, role); add targets the NEW.
func CascadeRuleUpdated(ctx context.Context, actor string, old models.MappingRule, sp, sr, tp, tr string) (CascadeResult, error) {
	// --- add pass: new-source holders get the new target ---
	addUsers, err := svcGetUsersWithRole(ctx, sp, sr)
	if err != nil {
		return CascadeResult{}, err
	}
	addSet := make(map[string]bool, len(addUsers))
	params := make([]db.EnqueueParams, 0, len(addUsers))
	for _, u := range addUsers {
		addSet[u] = true
		params = append(params, db.EnqueueParams{
			UserID: u, ProjectID: tp, RoleKeys: []string{tr},
			GrantedBy: actor, Reason: "Mapping-rule update cascade",
			Source: "rule", SourceRef: old.ID, OpType: "add", PayloadJSON: "{}",
		})
	}

	// --- revoke pass: old-source holders lose the OLD target unless kept/covered ---
	sameTriple := old.TargetProject == tp && old.TargetRole == tr
	oldUsers, err := svcGetUsersWithRole(ctx, old.SourceProject, old.SourceRole)
	if err != nil {
		return CascadeResult{}, err
	}
	for _, u := range oldUsers {
		if sameTriple && addSet[u] {
			continue // identical triple re-added for this user — no churn
		}
		covered, err := OtherSourceCovers(ctx, u, old.TargetProject, old.TargetRole, "", old.ID)
		if err != nil {
			return CascadeResult{}, err
		}
		if covered {
			continue
		}
		params = append(params, db.EnqueueParams{
			UserID: u, ProjectID: old.TargetProject, RoleKeys: []string{old.TargetRole},
			GrantedBy: actor, Reason: "Mapping-rule update cascade",
			Source: "rule", SourceRef: old.ID, OpType: "revoke", PayloadJSON: "{}",
		})
	}

	ids, err := svcUpdateRuleAndEnqueue(ctx, actor, old.ID, sp, sr, tp, tr, params)
	if err != nil {
		return CascadeResult{}, err
	}
	return applyMode(ctx, old.ConfirmationMode, ids)
}
```

- [ ] **Step 12: Write failing tests** (`cascade_test.go`): a **target change** (old `tp/trOld` → new `tp/trNew`) where a source-holder gets the new target added AND the old target revoked (not covered elsewhere); and a case where the old target is still covered by a bundle → no revoke.
```go
func TestCascadeRuleUpdated_TargetChange_AddsNewRevokesOld(t *testing.T) {
	defer resetCascadeDeps()
	old := models.MappingRule{ID: "rule1", SourceProject: "sp", SourceRole: "sr",
		TargetProject: "tp", TargetRole: "trOld", ConfirmationMode: "auto"}
	svcGetUsersWithRole = func(ctx context.Context, p, r string) ([]string, error) {
		return []string{"u1"}, nil // u1 holds both old and new source (same source here)
	}
	svcGetDirectGrantsForUser = func(ctx context.Context, u string, inc bool) ([]models.DirectGrant, error) { return nil, nil }
	svcGetBundlesForUser = func(ctx context.Context, u string) ([]models.Bundle, error) { return nil, nil }
	svcGetActiveMappingRules = func(ctx context.Context) ([]models.MappingRule, error) { return nil, nil }
	var got []db.EnqueueParams
	svcUpdateRuleAndEnqueue = func(ctx context.Context, actor, id, sp, sr, tp, tr string, ps []db.EnqueueParams) ([]string, error) {
		got = ps; return []string{"o1", "o2"}, nil
	}
	svcDrainBatch = func(ctx context.Context, ids []string) (propagation.DrainResult, error) { return propagation.DrainResult{}, nil }

	if _, err := CascadeRuleUpdated(context.Background(), "admin", old, "sp", "sr", "tp", "trNew"); err != nil { t.Fatal(err) }
	var addNew, revokeOld int
	for _, p := range got {
		if p.OpType == "add" && p.RoleKeys[0] == "trNew" { addNew++ }
		if p.OpType == "revoke" && p.RoleKeys[0] == "trOld" { revokeOld++ }
	}
	if addNew != 1 || revokeOld != 1 { t.Fatalf("addNew=%d revokeOld=%d, want 1/1 (%+v)", addNew, revokeOld, got) }
}
```

- [ ] **Step 13a: Add an update-aware cycle detector `db.DetectCycleOnUpdate`.** The existing `db.DetectCycleOnInsert` (`db/validation.go`) loads **all** current rules and adds the proposed edge — for an update it would still include the rule's *old* edge, so a valid retarget (e.g. break an existing chain) could be falsely rejected. Add:
```go
// DetectCycleOnUpdate is DetectCycleOnInsert but excludes the rule being edited from the graph
// before adding the proposed (new) edge, so re-pointing an existing rule is judged on the graph
// WITHOUT its old edge. Load rules WHERE id != excludeRuleID (or filter in Go), then run the same
// DFS as DetectCycleOnInsert with the new edge added.
func DetectCycleOnUpdate(ctx context.Context, excludeRuleID, sp, sr, tp, tr string) (bool, error)
```
Factor the DFS out of `DetectCycleOnInsert` so both share it (DRY); `DetectCycleOnInsert` = load-all + DFS, `DetectCycleOnUpdate` = load-all-minus-excludeRuleID + DFS.

- [ ] **Step 13b: `handleUpdateMappingRule`** (`handlers/rules.go`) — validate (strict JSON, non-self-referential, `dbDetectCycleOnUpdate(ctx, id, ...)`), then **read the old rule** and fire the cascade (which updates + enqueues atomically):
```go
	old, err := dbGetMappingRuleByID(r.Context(), id)
	if err != nil {
		jsonErrorResponse(w, http.StatusNotFound, "NOT_FOUND", "mapping rule not found")
		return
	}
	// validate req: self-ref → 400; if cyc, _ := dbDetectCycleOnUpdate(r.Context(), id, req.SourceProject, req.SourceRole, req.TargetProject, req.TargetRole); cyc → 400
	cascade, err := svcCascadeRuleUpdated(r.Context(), actor, old, req.SourceProject, req.SourceRole, req.TargetProject, req.TargetRole)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "CASCADE_ERROR", err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, map[string]any{"message": "Mapping rule updated", "cascade": cascade})
```
Route in `router.go`: `mux.HandleFunc("PUT /api/v1/rules/mapping/{id}", withCORS(withUserAuth(handleUpdateMappingRule)))`. Injectables: `dbGetMappingRuleByID=db.GetMappingRuleByID`, `dbDetectCycleOnUpdate=db.DetectCycleOnUpdate`, `svcCascadeRuleUpdated=services.CascadeRuleUpdated`.

- [ ] **Step 14: Handler test** (`rules_test.go`): (a) update reads old rule + fires `svcCascadeRuleUpdated`; (b) self-referential → 400; (c) a retarget that only cycles WITH the old edge present is **accepted** (guards the update-vs-insert cycle difference); (d) unknown id → 404.

- [ ] **Step 15: Backend gate + commit.**
```bash
git commit -am "feat(cascade): 6th trigger — rule-matcher-change via add-new-target + revoke-old-target (pre-update capture, atomic) (sub-phase 3, Task 21f)"
```

---

## Task 22 — Confirmation-mode UI + global default + Recent automated cascades

Backend read/write for the global default + bulk toggle; hardcoded-auto for expiry/lifecycle; then the UI.

**Files:**
- Modify: `backend/internal/db/cascade.go` (bulk setters + `GetRecentCascades`), `handlers/cascade_surfaces.go` (config + recent-cascades handlers), `router.go`, `deps.go`; `handlers/bundles.go`/`rules.go` create handlers (inherit default); expiry/lifecycle enqueue sites (verify hardcoded auto).
- Create UI: `useConfirmationMode.ts`, `ConfirmationModeControls.tsx`, `RecentCascades.tsx`.
- Modify UI: `policies/page.tsx`, `bundles/page.tsx`, `useMappingRules.ts`, `useBundles.ts`, `SidebarNav.tsx`.

**Interfaces produced:**
- `db.SetBundleConfirmationMode(ctx, ids []string, mode string) error` + `db.SetRuleConfirmationMode(ctx, ids []string, mode string) error` (one tx each).
- `db.GetRecentCascades(ctx, limit int) ([]models.CascadeSummary, error)` — recent **applied** outbox rows where `source IN ('bundle','rule','lifecycle_cascade')`.
- Routes: `GET/PUT /api/v1/config/confirmation-mode-default`, `POST /api/v1/policies/confirmation-mode` (bulk, body `{kind:"rule"|"bundle", ids:[], mode}`), `GET /api/v1/propagations/cascades`.

### 22a — Backend

- [ ] **Step 1: Create handlers inherit the global default.** Resolve `mode` = request body's optional `confirmation_mode` else `db.GetConfigSetting(ctx, db.ConfigKeyDefaultConfirmationMode)` (fallback `"auto"` on `""`). `handleCreateBundle` passes it to `db.CreateBundle(..., mode)` (creating a bundle triggers no cascade — no members/roles yet). `handleCreateMappingRule` passes it to `svcCascadeRuleCreated(ctx, actor, sp, sr, tp, tr, mode)` (Task 20 Step 16), which persists it via `CreateMappingRuleAndEnqueue`. Spec scenario: *Global default applies to new rules unless overridden.*

- [ ] **Step 2: Bulk setters in `db/cascade.go`:**
```go
func SetRuleConfirmationMode(ctx context.Context, ids []string, mode string) error {
	_, err := PG.Exec(ctx,
		`UPDATE mapping_rules SET confirmation_mode = $1 WHERE id = ANY($2)`,
		NormalizeConfirmationMode(mode), ids)
	return err
}
// SetBundleConfirmationMode — same shape against bundles.
```
(One statement = one implicit tx; spec's "in one transaction" satisfied.)

- [ ] **Step 3: `GetRecentCascades`** — recent **applied** cascade outbox rows (must filter `status='applied'`; a pending/failed row is not a completed cascade — review P2):
```go
func GetRecentCascades(ctx context.Context, limit int) ([]models.CascadeSummary, error) {
	rows, err := PG.Query(ctx,
		`SELECT id, op_type, user_id, project_id, role_keys, source, COALESCE(source_ref,''), status, completed_at
		 FROM pending_zitadel_propagations
		 WHERE source IN ('bundle','rule','lifecycle_cascade') AND status = 'applied'
		 ORDER BY completed_at DESC NULLS LAST LIMIT $1`, limit)
	// scan into models.CascadeSummary{ID, OpType, UserID, ProjectID, RoleKeys, Source, SourceRef, Status, CompletedAt}
	...
}
```
Add `models.CascadeSummary{ID, OpType, UserID, ProjectID string; RoleKeys []string; Source, SourceRef, Status string; CompletedAt *time.Time}` — `SourceRef` (bundle/rule id) is what lets the "Recent cascades" UI name the originating bundle/rule.
> **Naming/precision note:** the outbox row does NOT record whether it drained automatically (auto) vs by operator "Resume now" (a resumed manual row) — the confirmation decision isn't persisted per row. So this query surfaces *applied cascade projections* (source ∈ {bundle,rule,lifecycle_cascade}), which is a superset of the spec's "automated." That is the right thing to show operators (every cascade that reached Zitadel, never invisible); expiry/lifecycle rows (`source='lifecycle_cascade'`, always auto) are included. If a strict auto-only view is ever needed, persist the drain trigger (auto vs operator) on the outbox row and filter on it — out of scope now. Label the UI element "Recent cascade projections" if "automated" reads as over-claiming.

- [ ] **Step 4: Handlers + routes + injectables** for the three endpoints (config get/set with strict-JSON decode + operator auth; bulk with `decodeJSONStrict`; recent-cascades read). Tests in `cascade_surfaces_test.go`: bulk updates the selected ids' mode; global-default get returns seeded `auto`; set persists.

- [ ] **Step 5: Hardcoded-auto for expiry/lifecycle.** Grep `services/expiry/` and the lifecycle/zitadel-event-trigger path for where they revoke/enqueue. If they call `EnqueueDirectGrantPropagation` or the drain, ensure `Source` is set to `lifecycle_cascade` (or `direct` for expiry) and the drain runs immediately (auto) — they are pre-authorized. If expiry currently revokes directly (not via outbox), leave it; note in `tasks.md` that expiry projection is out of this task's scope unless it already routes through the outbox. **Verify only; do not rework expiry** — YAGNI.

- [ ] **Step 6: Backend gate + commit.**

### 22b — UI

- [ ] **Step 7: Carry `confirmation_mode`** in `useMappingRules.ts` `MappingRuleRow` + `useBundles.ts` `BundleRow` types.

- [ ] **Step 8: `useConfirmationMode.ts`** — `useGlobalConfirmationDefault()` (query `["config","confirmation-mode-default"]`), `useSetGlobalConfirmationDefault()` (PUT), `useBulkSetConfirmationMode()` (POST `/policies/confirmation-mode`, invalidates `["mapping-rules"]`/`["bundles"]`), `useRecentCascades()` (query `/propagations/cascades`).

- [ ] **Step 9: `ConfirmationModeControls.tsx`** — a bulk toolbar: a "Bulk edit" toggle that reveals per-row checkboxes (managed by the parent page via a selected-id set) and a `[auto | manual]` picker + "Apply to N selected" button calling `useBulkSetConfirmationMode`. Props: `{kind, selectedIds, onDone}`. Respect Material tokens; keep it presentational.

- [ ] **Step 10: Mount in `policies/page.tsx`** — bulk controls in the Card header; each rule article shows a small `confirmation_mode` badge (`Auto`/`Manual`) and, in bulk mode, a checkbox. Create-rule form gains a mode `<select>` defaulting to the global default (from `useGlobalConfirmationDefault`).

- [ ] **Step 11: Mount in `bundles/page.tsx`** — same bulk controls + per-bundle mode badge + create-bundle mode field.

- [ ] **Step 12: Global-default dropdown** — mount on the existing settings/operations surface (design §9 Q9 says no `/settings` page was built for the chime; reuse the same location — the sidebar footer/operations page). A single `<select>` bound to `useGlobalConfirmationDefault`/`useSetGlobalConfirmationDefault`.

- [ ] **Step 13: `RecentCascades.tsx` + sidebar item** — a compact list (source, op, user, role, when) from `useRecentCascades`; add a "Recent cascades" nav item under the Operations section in `SidebarNav.tsx` linking to where the element mounts (e.g. `/operations` or a small `/operations/cascades` route). This satisfies "surface fired cascades so they are never invisible."

- [ ] **Step 14: UI tests** (Vitest + jsdom, `QueryClientProvider`): `ConfirmationModeControls` — selecting rows + Apply POSTs the chosen mode for the selected ids; `RecentCascades` renders rows from a stubbed fetch; create-form defaults to the global default. Follow `DriftTriageClient.test.tsx` for the `makeProxyFetch` stub pattern.

- [ ] **Step 15: UI gate + commit.**
```bash
cd ui && bun run test && bun run lint && bun run build
git commit -am "feat(ui): confirmation-mode controls + global default + recent cascades (sub-phase 3, Task 22)"
```

---

## Task 23 — Sub-phase-3 verification gate + docs (Task 24)

- [ ] **Step 1: Backend full gate** — `cd backend && go test ./... && go vet ./... && gofmt -l internal/ cmd/`. All pass, vet clean, gofmt empty.
- [ ] **Step 2: UI full gate** — `cd ui && bun run test && bun run lint && bun run build`. All pass, build succeeds including any new route.
- [ ] **Step 3: Migration `000017` up/down symmetry** confirmed statically (no throwaway Postgres in this sandbox, as sub-phases 1/2 did): up adds two columns + `config_settings` + seed; down drops the table + both columns; nothing orphaned.
- [ ] **Step 4: `openspec validate wave-2-part-4-zitadel-state-projection-and-drift-control --strict`** from repo root — passes.
- [ ] **Step 5: Tick Tasks 19–23 in `tasks.md`** with the same detailed post-hoc notes style as sub-phases 1/2 (deviations; the 6th trigger built as add-reproject + `source_ref`-targeted revoke; the `OtherSourceCovers`-vs-`collectUserRoles` correctness note; expiry/lifecycle verification result). Note the new `PUT /api/v1/rules/mapping/{id}` endpoint under Task 20's "rule create/update path". Also tick the stale Tasks 11–13 checkboxes flagged in the Task 14 note.
- [ ] **Step 6: Task 24 cross-cutting docs** — adopt the doctrinal sentence in `CLAUDE.md` Key Conventions + `syndra-core-architecture/design.md`; add the **Drift Control** row + flip the reconciliation note in `feature-coverage.md`; Phase 5.5 closure line in `ROADMAP.md`; this change's row in `INDEX.md`; ensure `.env.example` documents `DRIFT_RECONCILIATION_INTERVAL_HOURS`, `OUTBOX_MAX_RETRIES`, `OUTBOX_RETENTION_DAYS`.
- [ ] **Step 7: codebase-memory refresh** — `mcp__codebase-memory-mcp__detect_changes` + reindex the affected scope (`services`, `services/propagation`, `db`, `handlers`).
- [ ] **Step 8: Final commit.**
```bash
git commit -am "docs(openspec): tick sub-phase 3 tasks + adopt drift-control doctrine (Task 23/24)"
```

---

## Self-Review

**Spec coverage (automation-policies/spec.md):**
- "Bundle/rule changes MUST project effective grants through the outbox" → Tasks 20 (adds) + 21 (revokes). ✅
- "Adding a user to a bundle cascades one outbox row per bundle role" + already-exists self-resolve → Task 20a/20c (drain's `409→applied`). ✅
- "Removing coverage checks for another source before revoking" + ledger records source removed → Task 21a/21b (explicit `OtherSourceCovers`; the revoke `source='bundle'`/`source_ref` records the originating source; the sub-phase-1 ledger reconcile on applied revoke prunes only the named triple). ✅
- "Each rule/bundle MUST carry confirmation_mode defaulting from config_settings" → Task 19 + Task 22a Step 1. ✅
- "Auto-mode drains without operator intervention; rows still persisted first" → Task 20c `applyMode` (enqueue THEN DrainBatch). ✅
- "Manual-mode waits in Pending tier" → Task 20c (no drain; rows are plain `pending` outbox rows the sub-phase-1 Pending UI already lists). ✅
- "Global default applies to new rules unless overridden" + "bulk Set confirmation mode in one transaction" → Task 22a Steps 1–2, Task 22b Steps 9–11. ✅
- "Expiry/lifecycle hardcoded-auto + Recent automated cascades" → Task 22a Step 5 + 22b Step 13. ✅

**All six §4.7 triggers implemented:** 1 add-user-bundle (T20), 2 remove-user-bundle (T21c), 3 add-role-bundle (T20), 4 remove-role-bundle (T21c), 5 rule-fire-on-create (T20), 6 rule-matcher-change (T21f, add-new-target + revoke-old-target via pre-update capture). None deferred.

**Review round 3 fixes (compile/consistency):**
- **DrainBatch signatures:** rewritten to match `DrainOne` exactly — `acquireDrainLock` → `(release, acquired, err)`, `claimOne` → `(*models.PendingPropagation, found, err)`, and `res.processRow(ctx, *row)` (method, returns `halt`); test uses `*models.PendingPropagation`, not a non-existent `pendingRow`.
- **Cascade test injectable name:** tests stub `svcCascGetRolesForBundle` (the actual cascade injectable), not `svcGetRolesForBundle`.
- **`source_ref` surfaced:** added `SourceRef` to `models.PendingPropagation` + `CascadeSummary`, threaded through the claim/scan SELECTs and `GetRecentCascades`, so the Pending worklist and Recent-cascades UIs can name the originating bundle/rule (not just the source *type*).

**Review round 2 fixes:**
- **P1 (cascade revoke could delete operator grants):** the drain's revoke reconcile was source-blind (`replace` was already `source='direct'`-scoped; `revoke` was not). Task 20b′ threads the outbox `source` through `models.PendingPropagation` + claim/scan + `applyRow`, and scopes `ReconcileLedgerOnApplied`'s revoke delete by the row's `source` — so a `source='bundle'/'rule'` revoke deletes no ledger rows (operator `source='direct'` rows are safe) and an operator revoke behaves exactly as today.
- **P2 (drain error was fatal):** `applyMode` no longer returns the drain error — it logs it and records `Halted`/`Reason` in `res.Drain`, returning nil, so a drain failure is a `200` with the rows left pending (the handler's `500` now fires only for the atomic enqueue failure, which rolls back).
- **P2 (rule-update cycle check):** added `db.DetectCycleOnUpdate` that excludes the edited rule's old edge before testing the new edge (the insert detector would falsely reject valid retargets); handler uses it; a test guards the difference.
- **P2 (recent cascades):** query now requires `status='applied'` and is documented as "applied cascade projections" (the outbox doesn't persist auto-vs-resumed, so precise "automated-only" would need an extra column — noted, out of scope).

**Review round 1 fixes (P1a/P1b/P1c/P2×2):**
- **P1b (ledger is per-triple, not per-source):** cascades write **no** `direct_role_grants` rows; attribution lives on the new outbox `source`/`source_ref`; `OtherSourceCovers` computes coverage from the source tables + genuine `source='direct'` grants (reliable because cascades never overwrite them).
- **P1a (rule update orphaned old target):** `CascadeRuleUpdated` takes the **pre-update** rule; revoke targets the OLD (project, role), add targets the NEW; unchanged-and-re-added users are skipped, so no keep-plus-re-add and no orphan.
- **P1c (mutation committed without outbox rows):** every trigger uses an atomic `*AndEnqueue` db fn (mutation + outbox in one tx, mirroring sub-phase 1); enqueue failure rolls back the mutation and returns 500; only the drain is best-effort (failed auto rows surface in the Pending worklist).
- **P2 (outbox source):** migration 000017 adds `source`/`source_ref` to `pending_zitadel_propagations`; `enqueueCascadeRows` writes them; `GetRecentCascades` filters on them.
- **P2 (rule columns):** all rule SQL uses `source_zitadel_project_id`/`source_zitadel_role_key`/`target_zitadel_project_id`/`target_zitadel_role_key` (Go struct fields stay `Source*/Target*`).

**Type consistency:** `CascadeResult{Enqueued, Mode, Drain}` identical across service + handler; all cascade fns use `(ctx, actor, …)`; `applyMode(ctx, mode, ids []string)` (drain-only) — enqueue is done by the atomic db fns which return the ids; `HolderKey{UserID, ProjectID, RoleKey}` reused from `services/expected.go`.

**Placeholder scan:** the "verify against source" items — `db.GetBundleByID` existence + bundle column list, `HolderKey`/`models.DirectGrant` field names, `zitadel_grants_index` columns, `processRow` signature, `drain.go` revoke grant-id resolution, factoring the DFS out of `DetectCycleOnInsert`, and threading `source` through the claim/scan SELECT column lists — each carries an explicit read-first note. No TBDs.
