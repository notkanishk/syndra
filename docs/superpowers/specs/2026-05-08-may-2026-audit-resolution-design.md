# May 2026 Audit Resolution — Design

**Date:** 2026-05-08
**Source:** `AUDIT.md` § "Addendum — May 2026 Codebase Audit" (lines 476–660)
**Status:** Design approved; awaiting writing-plans handoff

---

## 1. Aim

Resolve every finding from the May 2026 audit addendum (overengineering, spec drift, bugs, test gaps) in a single coordinated cleanup, while honoring the project's stated principle: *"Zitadel is the absolute source of truth — whatever information is on Zitadel is the whole truth."*

The cleanup is decomposed into **five themed OpenSpec changes**, sequenced in three waves. Theme 2 carries the only doctrinal shift; the other four restore spec-code coherence, finish in-flight work, and cut bloat.

This document is the **meta-spec** that organizes the work. Each themed change gets its own `openspec/changes/<theme>/{proposal,design,tasks,IMPLEMENTATION}.md` produced in the writing-plans phase.

---

## 2. Doctrinal decisions

Four direction-setting questions are answered upfront because they shape everything else.

| Ref | Question | Decision |
|---|---|---|
| B4 + D3 | What does "Backend is the single mutation authority" mean in a system where Zitadel is source of truth? | **Two-layer model.** Zitadel mirrors current state for every grant. MkAuth holds the intent ledger (reason, expires_at, granted_by, source). MkAuth-mediated mutations (any path that goes through MkAuth backend) write the intent ledger before the Zitadel API call. Direct-Zitadel mutations (Zitadel admin UI, external tools) are detected as drift and acquire their intent record through operator attribution. Every grant ends with an intent record; the timing differs by origin. |
| D1 | `GetWelcomeBundle` behaviour when no name matches | **Error explicitly.** Drop the "first bundle by `created_at`" fallback. Onboarding fails loudly when no welcome bundle is configured. Operator must mark a bundle as the welcome bundle deliberately. |
| D2 | ⌘K command palette is in `design.md` §5 but unimplemented | **Strike from current spec; add Phase 6 roadmap stub.** Honest about the gap. |
| D4 | `mapping_rules.version` increments without rollback machinery | **Drop versioning.** Rely on `audit_logs` for replay. The `version` column and increment logic are removed. |
| D6 | `lib/types.ts:UserProfile.location` is in spec but never rendered | **Drop from spec and code.** |

These are load-bearing for every downstream design choice in this plan.

---

## 3. Theme decomposition

Each themed OpenSpec change is independently reviewable and (mostly) independently shippable.

### Theme 1 — `production-trust-hardening`

Ship-blocker bugs that erode operator trust. Smallest scope. Ships first.

| Audit ref | Item |
|---|---|
| C1 | Production refuses missing signing keys. When `ZITADEL_DOMAIN != ""`, the backend fails fast (exits with non-zero on startup) if `ZITADEL_EVENT_SIGNING_KEY` or `ZITADEL_ACTION_SIGNING_KEY` is empty. The current "log warning and pass through" silent fallthrough in `zitadel_action_auth.go:50-57` is removed |
| C2 / D5 | OIDC member dashboard metadata. Either fetch profile from `/api/v1/users/{self}` during the OIDC callback to populate the cookie, or drop the title/team/location render line for OIDC sessions. Decision: **fetch during callback** so OIDC and demo sessions render identically |
| D1 | `GetWelcomeBundle` errors explicitly when no welcome bundle is configured. Migration adds `is_welcome BOOLEAN DEFAULT FALSE` to bundles; UI adds "Set as welcome bundle" toggle (constraint: max one true at a time) |
| C3 | Vault dev-mode self-attribution. `enforceSelfOnly` in `vault.go:13-30` refuses mutations in dev mode, OR requires an explicit `?actor=` query parameter that is logged separately as the API-key holder, not the user. Decision: **refuse mutations in dev mode** unless `?actor=` is supplied |
| B1 | Delete `backend/cmd/test/main.go`. No callers. Destructive `DELETE FROM mapping_rules` |

### Theme 2 — `zitadel-state-projection-and-drift-control`

The marquee architectural change. Wires the two-layer doctrine into reality.

| Audit ref | Item |
|---|---|
| B4 + D3 | Resolves the mutation-authority contradiction. The `/api/v1/zitadel/users/{id}/grants` POST/PUT/DELETE handlers (`discovery.go:218-282`) are rewired internally to flow through the canonical intent-ledger path. Result: same operator UX, no separate code path |
| B2 | `reconciliationSafetyCap` drops from `10_000` to `2_000`. `OnlyInZitadel` bucket no longer lists every mapping-rule-derived grant — those are classified as `expected_via_rule` |
| C6 | `directory/zitadel.go:Users()` overlay cache is only set after a fully successful overlay; partial-result paths skip cache. The follow-up drift detection covers any cache-miss-then-drift case |

Plus the new architectural pieces (Section 5).

### Theme 3 — `backend-coherence`

Internal seams. Refactors that improve readability and reduce repeat work without changing observable behaviour.

| Audit ref | Item |
|---|---|
| B3 | `services/views.go` (797 lines): compute `(user → roles)` map once per request in a request-scoped helper; hand the same map to `ListUsers`, `ListApplications`, `ListProjects`, and `Topology`. Likely halves the file |
| B5 | `repositories.go` (1303 lines): split into `repositories/{bundles,grants,rules,webhooks,vault,intents,roles,onboarding,audit}.go`. Function bodies unchanged; package boundaries reorganized |
| C4 | Lift parsed JWT claims into request context. `withUserAuth` parses once and stuffs claims into `r.Context()`; `withOperatorAuth` reads from context instead of re-extracting the bearer token in `router.go:189-211` |
| C5 | Cache last-known `claim_failure_mode` per project in Redis next to the claim payload. `GetClaimFailureMode` returns the cached value on transient DB error rather than silently defaulting to `fail_closed` |
| B6 / C11 / D8 | Tighten silent defaults in `webhook.go:77-79` and `:104-109`. Missing `event_type` returns 400 (with structured error payload). Zitadel-shape with missing `source_project` returns 400 unless the event is one of the documented "no source project" types (e.g. `user.added`) |
| B7 | `vault.go:isComplexityError`: replace string-prefix sniffing with a typed sentinel `ErrComplexity` returned by the Zitadel client wrapper |

### Theme 4 — `frontend-coherence`

The UI half — finishes the obsidian-clarity migration, kills dead code, splits the 947-line zitadel page, settles two design questions, adds the missing security tests.

| Audit ref | Item |
|---|---|
| U1 | `useNameResolver.tsx` (409 lines): replace rAF-batched resolver with single full-catalog fetch on `<NameResolverProvider>` mount. Resolution becomes synchronous lookup against an in-memory `Map`. Catalog refetch on user create/delete events |
| U2 + D10 | Finish obsidian-clarity-redesign palette migration. Move `Sidebar.tsx`, `ThemeToggle.tsx`, `RequestAccessButton.tsx`, `ErrorBoundary.tsx`, and the legacy parts of `app/zitadel/page.tsx` to Material tokens. Delete the legacy palette tokens from `globals.css` after. Move corresponding tasks in `obsidian-clarity-redesign/tasks.md` from `[x]` to a Stage-2 list and check them off here |
| U3 | Dedup `/governance/summary` fetch. `Sidebar.tsx` and `AdminDashboard.tsx` use a single shared React Query (`useGovernanceSummary`); `Sidebar.tsx` drops its raw `fetch()` call |
| U4 | Delete dead exports from `lib/api.ts` (`fetchBundles`, `fetchMappingRules`, `fetchProjects`, `fetchAudit`). Consolidate `lib/types.ts` into per-resource shapes inside each `useX.ts` |
| U5 | Split `app/zitadel/page.tsx` (947 lines) into `components/zitadel/{Health,Rotation,Projects,Users,AllGrants}.tsx`. Replace `apiGet`/`apiSend`/`apiGetDiagnostic` with `request<T>` from `api-client.ts` plus a `{ preserveErrorBody: boolean }` flag |
| U6 | `session.ts:206`: avatar fallback uses `userId`/`email` before falling through to `nameToAvatar(name)` |
| D2 | Strike ⌘K from `design.md §5`. Add stub line to `ROADMAP.md` Phase 6: "Optional: ⌘K command palette." |
| D6 | Drop `location` field from `lib/types.ts:UserProfile`, demo data, `live-directory-identity-completeness` spec, and the OIDC session profile shape |
| U7 | Add tests for `middleware.ts` (admin redirect, stale demo cookie clearing) and `api/proxy/[...path]/route.ts` (member self-scoping, allowlist enforcement). These enforce the admin/user split; explicit coverage matters |

### Theme 5 — `operational-polish`

Install/dev experience + tiny doc/script consolidations + small sync/LDAP fixes.

| Audit ref | Item |
|---|---|
| D4 | Drop versioning. Migration removes `mapping_rules.version` column. `UpdateMappingRule` no longer increments. UI surfaces audit log entries instead of a version number |
| D7 | Document sync-service env surface. `.env.example` gains a `--- Sync Service / LLDAP ---` block with `BACKEND_URL`, `MKAUTH_API_KEY`, `LLDAP_URL`, `LLDAP_BIND_DN`, `LLDAP_BIND_PASSWORD`, `LLDAP_BASE_DN`, `LLDAP_INSECURE_SKIP_VERIFY`, `SYNC_POLL_INTERVAL`, `SYNC_WORKER_COUNT`, `SYNC_INTENT_LIMIT`, `SYNC_RETRY_ATTEMPTS`, `SYNC_RETRY_BACKOFF` (the latter two newly wired by S4). The backend-side block in `.env.example` also adds `MKAUTH_EXTERNAL_URL` and `ZITADEL_M2M_TOKEN`, which are required at runtime but missing from the template today. Each entry includes a default-value comment and a one-line description |
| D9 | Drop "N>1 replicas" framing for `EXPIRY_SCHEDULER_*` in `.env.example:27-30`. Single-LXC deployment doesn't have multi-replica |
| S1 + S2 | Extract `scripts/lib/load-env.sh` (`_ENV_FILE` loader) and `scripts/lib/zitadel-api.sh` (`zitadel_api()` helper with PERMISSIONS hint). Source from `register.sh`, `rotate.sh`, `smoke-test-action-v2.sh`, `smoke-test-event-listener.sh`. ~120 lines of duplicate bash deleted |
| S3 | Fold `zitadel/actions/{PERMISSIONS,SIGNING_KEY}.md` into `README.md`. 4 docs → 2 (README + EVENTS) |
| S4 | Wire `SYNC_RETRY_ATTEMPTS` and `SYNC_RETRY_BACKOFF` env vars in `sync/internal/config/config.go:42-43`, OR remove the fields and inline constants. Decision: **wire env vars** for consistency with the rest of the sync config surface |
| C7 | Sync LDAP: replace `member: [""]` placeholder in `groupOfNames` with the bind DN. Survives strict OpenLDAP if MkAuth ever migrates from LLDAP |
| C8 | Plumb context through `withConn` in `sync/internal/ldap/client.go:185`. Worker shutdown propagates cancellation to in-flight LDAP ops |
| C9 | `scripts/smoke-test-lxc.sh:11-13` probes `/healthz` instead of `/api/v1/bundles`. Works on real OIDC LXC deployments |
| C10 | Shadow-password zero buffer (`worker.go:190-194`): drop the `defer zero(hashBytes)` line; the comment at the deletion site documents the limitation (Go GC immutability of the underlying string) |
| B8 | Drop `grantLookupMaxPages` from `100` to `10` in webhook lookup helper. 200-user org never paginates beyond ~2 pages |

### Coverage check

All audit findings have a home:

```
Backend:    B1 → T1   B2 → T2   B3 → T3   B4 → T2   B5 → T3   B6 → T3   B7 → T3   B8 → T5
UI:         U1 → T4   U2 → T4   U3 → T4   U4 → T4   U5 → T4   U6 → T4   U7 → T4
Sync:       S1 → T5   S2 → T5   S3 → T5   S4 → T5
Spec drift: D1 → T1   D2 → T4   D3 → T2   D4 → T5   D5 → T1   D6 → T4   D7 → T5
            D8 → T3   D9 → T5   D10 → T4
Bugs:       C1 → T1   C2 → T1   C3 → T1   C4 → T3   C5 → T3   C6 → T2   C7 → T5
            C8 → T5   C9 → T5   C10 → T5  C11 → T3
```

---

## 4. Architecture: Theme 2 (zitadel-state-projection-and-drift-control)

### 4.1 Two-layer model made real

**Zitadel = current-state mirror.** Every grant — regardless of origin (operator point mutation, bundle expansion, mapping rule output, lifecycle cascade, drift back-fill) — exists as a `user_grant` row in Zitadel. The Zitadel admin UI shows the complete picture.

**MkAuth = intent ledger.** For every grant in Zitadel, MkAuth stores rich context: `reason`, `expires_at`, `granted_by`, `source` (one of: `direct`, `bundle`, `rule`, `external_backfill`, `lifecycle_cascade`), `source_ref` (UUID pointing to the originating bundle or rule when `source ∈ {bundle, rule}`; NULL otherwise), `idempotency_key`, audit chain. Apps that need richer-than-Zitadel data ask MkAuth.

The contract — **every grant in Zitadel ends with a corresponding MkAuth intent record.** For MkAuth-mediated mutations (any operator action through MkAuth UI, any bundle/rule cascade, any lifecycle propagation), the record is written before the Zitadel API call within the same database transaction. For direct-Zitadel mutations (Zitadel admin UI, external tools, scripted bypasses), the record is written post-hoc via the drift triage flow (Sections 4.5–4.6). The /api/v1/zitadel/* CRUD routes that today bypass this contract are rewired to honor the MkAuth-mediated timing.

### 4.2 Data model additions

```sql
-- Outbox for human-confirmed propagation
CREATE TABLE pending_zitadel_propagations (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    op_type         TEXT NOT NULL CHECK (op_type IN ('add', 'revoke', 'replace')),
    user_id         TEXT NOT NULL,
    project_id      TEXT NOT NULL,
    role_keys       TEXT[] NOT NULL,
    payload_json    JSONB NOT NULL,        -- full request body, for retry
    idempotency_key UUID NOT NULL UNIQUE,  -- stamped into Zitadel call metadata
    status          TEXT NOT NULL CHECK (status IN ('pending', 'in_flight', 'applied', 'confirmed', 'failed')),
    attempts        INT NOT NULL DEFAULT 0,
    last_error      TEXT,
    initiated_by    TEXT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at      TIMESTAMPTZ,
    completed_at    TIMESTAMPTZ
);
CREATE INDEX idx_pending_zitadel_propagations_status ON pending_zitadel_propagations(status, created_at);

-- Drift queue
CREATE TABLE drift_items (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id           TEXT NOT NULL,
    project_id        TEXT NOT NULL,
    role_keys         TEXT[] NOT NULL,
    zitadel_grant_id  TEXT,                 -- nullable for revoke-drift (grant existed in MkAuth, missing in Zitadel)
    detected_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    detection_source  TEXT NOT NULL CHECK (detection_source IN ('webhook', 'reconciliation_sweep')),
    drift_type        TEXT NOT NULL CHECK (drift_type IN ('zitadel_only', 'mkauth_only')),
    status            TEXT NOT NULL DEFAULT 'pending_triage' CHECK (status IN ('pending_triage', 'attributed', 'revoked', 'marked_external')),
    resolved_at       TIMESTAMPTZ,
    resolved_by       TEXT,
    resolution_payload_json JSONB
);
CREATE INDEX idx_drift_items_status ON drift_items(status, detected_at);

-- Operator-marked legitimate external grants
CREATE TABLE external_grant_exclusions (
    user_id     TEXT NOT NULL,
    project_id  TEXT NOT NULL,
    role_key    TEXT NOT NULL,
    marked_by   TEXT NOT NULL,
    marked_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    reason      TEXT,
    PRIMARY KEY (user_id, project_id, role_key)
);

-- Confirmation mode for autonomous propagation
ALTER TABLE mapping_rules
    ADD COLUMN confirmation_mode TEXT NOT NULL DEFAULT 'auto'
        CHECK (confirmation_mode IN ('auto', 'manual'));

ALTER TABLE bundles
    ADD COLUMN confirmation_mode TEXT NOT NULL DEFAULT 'auto'
        CHECK (confirmation_mode IN ('auto', 'manual'));

CREATE TABLE config_settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_by TEXT
);
INSERT INTO config_settings(key, value, updated_by)
    VALUES ('global.default_rule_confirmation_mode', 'auto', 'migration');

-- Extend direct_role_grants with source attribution
ALTER TABLE direct_role_grants
    ADD COLUMN source TEXT NOT NULL DEFAULT 'direct'
        CHECK (source IN ('direct', 'bundle', 'rule', 'external_backfill', 'lifecycle_cascade'));
ALTER TABLE direct_role_grants
    ADD COLUMN source_ref TEXT;  -- bundle_id or rule_id when source != 'direct' / 'external_backfill'
```

Migration ordering: drift queue + outbox + exclusions first (additive), confirmation_mode columns next (default `auto`), source columns last (backfill existing rows to `source='direct'`).

### 4.3 Steady-state propagation flow

```
1. Operator: POST /api/v1/users/{id}/grants  (or /api/v1/zitadel/users/{id}/grants — same handler now)

2. Backend:
   BEGIN TRANSACTION
     INSERT direct_role_grants (..., source='direct')
     INSERT audit_logs ('direct_grant.upserted', ...)
     INSERT pending_zitadel_propagations (status='pending', idempotency_key=uuid)
   COMMIT

3. Response: 202 Accepted + { outbox_id, idempotency_key }

4. Frontend: invalidates governance summary; sidebar Pending count increments
   with single-pulse animation; dashboard pending callout appears or updates

5. Operator: clicks "Resume now" on dashboard pending callout
   (or — for grants flowing from auto-mode rules — drain fires automatically)

6. Drain loop, per outbox row in age order:
   pre-flight: GET /zitadel/health
     unreachable → halt drain; UI updates pending callout to "Zitadel offline"
                   (resume button disabled but callout stays visible)
     reachable   → status='in_flight'; started_at=NOW()
                   call Zitadel ManagementAPI with metadata={idempotency_key}
                   per-row ACK:
                     2xx          → status='applied'; completed_at=NOW()
                     4xx          → status='failed'; last_error; halt drain
                     5xx/timeout  → status='pending'; attempts++; continue or backoff

7. Webhook return-trip: Zitadel emits user.grant.added with metadata
   webhook handler:
     extract idempotency_key from event metadata
     UPDATE pending_zitadel_propagations SET status='confirmed' WHERE idempotency_key=$1
     close the loop; UI pending count decrements
   missed webhook (rare): reconciliation sweep is the backstop (Section 4.5)
```

### 4.4 Auto vs manual confirmation (Hybrid model)

**Per-rule flag.** Both `mapping_rules` and `bundles` carry a `confirmation_mode` column. Default value is read from `config_settings.global.default_rule_confirmation_mode`.

**Auto mode behaviour.** Rule fires → write intent ledger row + outbox row → drain immediately without operator intervention. The outbox row exists for crash recovery, audit, and uniformity (every Zitadel mutation has an outbox provenance).

**Manual mode behaviour.** Rule fires → write intent ledger row + outbox row (status=`pending`) → stop. Surfaces in the Pending Propagation tier alongside operator point mutations. Operator must explicitly resume.

**Bulk toggle.** Policies UI gains a multi-select with a "Set confirmation mode" action: `auto` / `manual`. Affects selected rules in one transaction.

**Global default.** Settings page exposes a dropdown bound to `config_settings.global.default_rule_confirmation_mode`. New rules created via the UI inherit this default unless overridden in the create form.

**Hardcoded auto cases (intentionally not configurable).**
- **Expiry sweeps.** When an operator set `expires_at` on a grant, that act IS the pre-authorization for future revocation. Sweep runs auto. Documented explicitly.
- **Lifecycle event cascades** (zitadel-event-trigger-propagation flow). Shipping the spec IS the pre-authorization. Cascade fires auto. Surface fired cascades in a sidebar "Recent automated cascades" element so they're never invisible.

### 4.5 Drift detection

**Real-time via webhook.** When `webhook_translate_enrich.go` processes a `user.grant.added` or `user.grant.removed` event:

```
extract grant identifiers
look up matching outbox row by idempotency_key
  match found → confirm; close loop
  no match    → check external_grant_exclusions for (user_id, project_id, role_key)
                  excluded     → silently ignore
                  not excluded → INSERT drift_items (detection_source='webhook')
                                 fire UI badge update
```

**Backstop via reconciliation sweep.** Every 6 hours (configurable via `DRIFT_RECONCILIATION_INTERVAL_HOURS`, default `6`), or operator-triggered:

```
list Zitadel user_grants  (paginated, cap 2000 not 10000)
list MkAuth expected set  = direct_role_grants ∪ bundle_expansions ∪ rule_outputs ∪ exclusions
diff:
  zitadel ∖ expected → drift_items.upsert (drift_type='zitadel_only', detection_source='reconciliation_sweep')
  expected ∖ zitadel → re-enqueue outbox row (status='pending', drift_type='mkauth_only')
                       (the webhook for our original mutation was missed — replay it)
```

The reconciliation sweep was previously sized for SaaS scale (10k cap). Right-sized to 2k for the academic-makerspace audience.

### 4.6 Drift triage queue

`/governance/drift` is the operator-facing surface. Each row offers three actions:

**Attribute.** Modal opens with:
- `reason` (textarea, required)
- `expires_at` (date picker, optional)
- `granted_by` (auto-filled with current operator)
- `source` (radio):
  - `external_backfill` (default)
  - `bundle` → select existing bundle
  - `rule` → select existing rule

On submit:
- For `external_backfill`: write `direct_role_grants` row (`source='external_backfill'`, `source_ref=NULL`); write `audit_logs`; mark drift_item `attributed`.
- For `bundle`: add user to bundle membership table; the existing Zitadel grant matches the bundle expansion; reconciliation will classify as `expected_via_bundle`. Write `audit_logs`. Mark drift_item `attributed`.
- For `rule`: write `direct_role_grants` row (`source='rule'`, `source_ref=rule.id`); rule is presumed to have matched at firing time. Write `audit_logs`. Mark drift_item `attributed`.

**Revoke.** Treats the out-of-band mutation as a mistake to undo. Enqueues an outbox row with `op_type='revoke'`. Does not auto-drain — the drain still requires operator confirmation per the propagation flow. Marks drift_item `revoked` once outbox row reaches `confirmed`.

**Mark external.** Persistent declaration that this grant is managed by another system. Inserts `external_grant_exclusions` row keyed by `(user_id, project_id, role_key)`. Marks drift_item `marked_external`. Future detections for the same triple are silently filtered.

**Bulk attribution.** Top of page has multi-select checkboxes + a "Bulk Attribute" button. Modal applies the same context to all selected items in one transaction. Critical for the bootstrap case (initial deployment with N pre-existing grants) and for batch operator interventions.

### 4.7 Cascade projection scope

The two-layer model is followed for **the full effective grant set**: direct_role_grants, bundle expansions, and mapping rule outputs all land as Zitadel user_grants.

**Trigger points for cascade enqueueing:**

| Operator action | Cascade |
|---|---|
| Add user to bundle (`POST /api/v1/users/{id}/bundles`) | For each role in the bundle's roles, enqueue outbox row with `source='bundle'`, `source_ref=bundle.id` |
| Remove user from bundle | For each role in the bundle, enqueue outbox row `op_type='revoke'` (only if no other source covers it; checked by intent-ledger lookup) |
| Add role to bundle (`POST /api/v1/bundles/{id}/roles`) | For each user currently assigned to the bundle, enqueue outbox row with `source='bundle'` |
| Remove role from bundle | For each user assigned, enqueue revoke (with the same other-source check) |
| Mapping rule fires (matches user) | For each rule output role, enqueue outbox row with `source='rule'`, `source_ref=rule.id` |
| Mapping rule changes matchers | Re-evaluate against user set; diff against current rule-derived projections; enqueue add/revoke accordingly |

**Other-source check (revokes).** Before enqueueing a revoke, the system checks if the same `(user, project, role)` is covered by another source (different bundle, mapping rule, or direct grant). If yes, the revoke is suppressed — the role legitimately persists from the other source. The intent ledger gains a row tracking that the original source was removed.

**Already-exists check (adds).** Before issuing an add to Zitadel, the drain checks whether the `(user, project, role)` already exists in Zitadel (via the existing webhook-derived grant index, or via a live `GetUserGrants` query when the index is stale). If yes, the outbox row is marked `applied` without an API call and the intent ledger is updated to record the source. This handles two cases cleanly:

- Drift-attribution-to-bundle: operator attributes drift D to bundle X. Adding the user to bundle X enqueues cascade rows for each bundle role; the rows targeting roles that already exist in Zitadel (including D's role) self-resolve without redundant API calls. Other bundle roles enqueue normally.
- Crash recovery: a process crash between Zitadel ACK and outbox status update leaves the row as `in_flight`. On restart, the drain replays; the already-exists check converts `in_flight` to `applied` without double-write.

**Cascade efficiency.** A bundle composition change for a 50-member bundle generates 50 outbox rows. A mapping-rule matcher change against 200 users generates up to 200 evaluations. The drain loop processes these in created-at order; the operator sees one banner "N pending propagations from bundle change" in the dashboard pending callout.

### 4.8 Migration of existing /api/v1/zitadel/* CRUD

The handlers in `discovery.go:218-282` (`handleAssignZitadelGrant`, `handleUpdateZitadelGrant`, `handleRemoveZitadelGrant`) are rewired internally to call the canonical intent-ledger flow. Steps:

1. Each handler validates request as today.
2. Instead of calling `zitadelAddUserGrant` / `zitadelUpdateUserGrant` / `zitadelRemoveUserGrant` directly, the handler calls the canonical service function (`UpsertDirectGrant` + outbox enqueue + drain).
3. Response shape changes from `{status: "granted"}` to `{outbox_id, idempotency_key, status: "pending"|"applied"}` (per the steady-state flow).
4. The frontend caller (`app/zitadel/page.tsx`) is updated to handle the new response shape — it already needs updates as part of Theme 4 (U5 split), so this is folded in.

Result: one mutation pathway in code, two URL-space aliases for the same operation. The `/zitadel/*` URL stays for backward compatibility with operators' bookmarks; both routes go through the same handler internally.

---

## 5. UI urgency tiers

Two distinct surfaces — **Pending Propagation** and **Drift** — represent fundamentally different operator concerns. Their UI weight is calibrated as: **drift > pending > steady state**, with no overlap.

### 5.1 Tier definitions

| | Pending Propagation | Drift |
|---|---|---|
| **What it is** | Operator-initiated change buffered in MkAuth, awaiting Zitadel send | Grant exists in Zitadel that MkAuth has no context for |
| **Caused by** | The operator, just now, intentionally | Unknown — admin emergency, missed webhook, unauthorized actor |
| **Risk** | None. Workflow state | Possibly a security incident or system fault |
| **Resolution** | Operator resumes propagation | Operator triages: Attribute / Revoke / Mark external |
| **Tone** | Interrupted task — pulls operator back to unfinished work | Anomaly — demands investigation |
| **Mental model** | "Your work is half-done" | "Something is off" |

### 5.2 Pending Propagation visual specification

| Surface | Treatment |
|---|---|
| Sidebar | Nested item under Operations: `Pending [N]`. Badge filled, `bg-tertiary` `text-on-tertiary`. Parent Operations gets a small amber dot when count > 0 |
| Dashboard banner | Callout strip below any drift callout, or topmost if no drift. Dismissible × per-session (reappears next visit). Shows count, last-queued time, current Zitadel reachability, and a `Resume now` button |
| Cross-page banner | None |
| Inline (user grants page) | Bordered tag next to the affected grant: `[⏱ Awaiting Zitadel] [Resume]` |
| Motion | Single pulse on count badge when count increases (operator acknowledgement). No repeated pulse |
| Audio | None |
| Persistence | Auto-resolves when count hits 0 |
| Language | "awaiting Zitadel", "needs propagation", "resume" — active, not patient |
| Color tone | Amber (Material `tertiary-container`) at higher saturation than ambient |
| Icon | ⏱ clock |

### 5.3 Drift visual specification

| Surface | Treatment |
|---|---|
| Sidebar | Dedicated top-level nav item `⚠ Drift`. Persistent red dot when count > 0. Sits above `/governance/*` |
| Dashboard banner | Full-width callout above stat grid. NOT dismissible. Shows count and top-3 preview with `Triage all →` link |
| Cross-page banner | Sticky top of content area on ALL admin pages. NOT dismissible. `⚠ N drift items detected — out-of-band changes need triage [Review →]` |
| Inline (user grants page) | Bordered alert above grants table: `⚠ This grant has no MkAuth context [Attribute] [Revoke] [Mark external]`. NOT dismissible without action |
| Motion | Pulse on sidebar item when count transitions 0→N or N→N+1 (3 pulses, ~1.2s each). Sticky banner slide-in (200ms ease-out) on first appearance. Count-up animation on numeric badge. New drift-queue rows brief-highlight on arrival (2s fade). All respect `prefers-reduced-motion` |
| Audio | One soft chime (~400ms single tone) on count increase per session. User-settings toggle "Drift alert sound" (default on). First chime preceded by one-time tooltip explaining the cue |
| Persistence | Each item persists until explicitly resolved (Attribute / Revoke / Mark external) |
| Language | "drift", "out-of-band", "needs triage", "untracked" |
| Color tone | Red (Material `error-container`) |
| Icon | ⚠ alert triangle |

### 5.4 Stacking when both are present

```
[sticky drift banner — full viewport width, always topmost]
─────────────────────────────────────────────────────────────
[content area, dashboard:]
  [drift callout — full-width, undismissible]
  [pending callout — full-width, dismissible per-session]
  [stat grid]
  [recent activity]
```

The geometry alone makes them distinct. Drift breaks out of layouts (sticky banner, undismissible callout, dedicated nav, audio); pending lives inside layouts (nested nav, dismissible callout, inline tags, no audio). At every dimension where they overlap, drift is louder.

### 5.5 Aesthetic constraint

Both surfaces use Material tokens from the obsidian-clarity-redesign palette (`bg-error-container`, `bg-tertiary-container`, `text-on-error-container`, etc.). In dark mode these auto-flip to deeper, less retina-burning variants — drift remains prominent without being garish. The Linear/Stripe aesthetic is preserved.

---

## 6. Sequencing

### 6.1 Wave structure

```
Wave 1 — critical path
└── Theme 1: production-trust-hardening
    Effort: ~1-2 days
    Why first: ship-blockers. C1 is a production timebomb. Architectural work
    must not land while signing keys can be silently bypassed.

Wave 2 — parallelizable, with one ordering edge
├── Theme 4 (palette portion): finish obsidian-clarity migration
│   Effort: ~2-3 days
│   Why: Theme 2's drift UI must use Material tokens from day one. Don't
│   build drift UI on legacy palette and rewrite it later.
│
├── Theme 3: backend-coherence
│   Effort: ~3-5 days
│   Why before Theme 2: B3 (views.go refactor) cleans the file Theme 2 will
│   add projection lookups to. Working on services/views.go from a clean
│   baseline avoids merge churn.
│
├── Theme 5: operational-polish
│   Effort: ~2-3 days
│   Why parallel: independent of all others. Pure polish; can ship anytime.
│
└── Theme 2: zitadel-state-projection-and-drift-control
    Effort: ~2-3 weeks
    Why last: depends on clean palette (Theme 4 part 1) and clean views.go
    (Theme 3). Largest scope; needs the foundation under it.

Wave 3 — final consolidation pass
└── Theme 4 (remainder) + spec consolidation
    Effort: ~3-4 days
    Non-palette parts of Theme 4 (useNameResolver trim, lib/api dead code,
    zitadel/page split, ⌘K strike, location drop, U7 tests) can interleave
    Wave 2. The consolidation step updates INDEX.md, ROADMAP.md, feature-
    coverage.md, and design.md to reflect post-cleanup reality.
```

### 6.2 Cross-theme dependencies

```
Theme 1 ──┬──▶ Theme 4 (palette)  ──▶ Theme 2 (drift UI)
          ├──▶ Theme 3 (views.go) ──▶ Theme 2 (projection lookups)
          ├──▶ Theme 5 (independent)
          └──▶ Theme 4 (remainder, can interleave)

Theme 2 ──▶ Theme 4 (consolidation pass) — INDEX/feature-coverage updates
```

### 6.3 Realistic timeline

- **Single developer, sequential:** ~5-6 weeks
- **Single developer, smart interleaving:** ~4-5 weeks
- **Two developers (Theme 2 + Themes 3/4/5 parallel):** ~3 weeks
- **Subagent-dispatched parallel execution** (Theme 1 sequential, Themes 3/4/5 in parallel agents per `superpowers:dispatching-parallel-agents`, then Theme 2 with cleaned foundation): ~2-3 weeks

### 6.4 Ship-anywhere properties

Each theme except Theme 2 produces a shippable mini-release. If priorities shift:

- **Theme 1 alone** removes production timebombs.
- **Themes 3, 4, 5** independently improve the codebase without architectural commitment.
- **Theme 2** is the only theme that introduces the new doctrine; it can be deferred indefinitely without rotting the others.

This gives the option of "audit-resolved minus marquee" as a 2-week milestone (Themes 1+3+4+5), with Theme 2 taken deliberately when ready.

### 6.5 Internal split-point for Theme 2

Theme 2 itself can be sub-phased if its scope hits unexpected complexity:

1. **Outbox + operator-confirmed flow** (~1.5 weeks). Single point mutations work end-to-end through the buffer. No drift detection yet.
2. **Drift detection + triage queue** (~1 week). Webhook + reconciliation; UI surfaces; three actions.
3. **Bundle/rule cascade projection** (~3-5 days). Cascade enqueueing on bundle/rule changes.

Sub-phases 1 and 2 ship a working slice (operator point mutations + drift visibility for direct grants) without sub-phase 3. Whether to actually split is a writing-plans-phase decision.

### 6.6 Risk callouts

- **Theme 5's `D4 drop versioning`** is destructive (removes a column). Lands in a maintenance window with backup verified. Trivial migration but worth treating with care.
- **Theme 4 palette migration** can fight with in-flight UI work. No in-flight UI work today, so safe — but the longer it waits, the more new code accumulates on the legacy palette.
- **Theme 2 cascade projection** generates outbox rows proportional to bundle/rule fan-out. For a 200-user org with ~10 bundles, worst-case bundle-roles-mass-edit is bounded; still the largest single source of outbox volume. Drain rate must accommodate.

---

## 7. Doctrinal documentation updates

Most spec updates live inside their themed change directory under `openspec/changes/<theme>/specs/`. Cross-cutting docs:

| Document | Change | Owning theme |
|---|---|---|
| `openspec/changes/mkauth-core-architecture/design.md` | Replace single-mutation-authority section with two-layer model. Add Hybrid Project-to-Zitadel architecture, outbox pattern, drift triage. Add per-rule confirmation-mode doctrine | Theme 2 |
| `CLAUDE.md` | Replace "Backend is single mutation authority" line in Key Conventions with the new doctrine sentence (Section 7.1 below). Three-line edit | Theme 2 |
| `openspec/changes/mkauth-core-architecture/specs/feature-coverage.md` | Add Drift Control capability row (Integrated, post-cleanup). Update Welcome Bundle row ("convention-based" → "explicit; errors when not configured"). Update Versioned Policies row ("Partial; version column" → "Removed; rely on audit_logs"). Update LDAP Sync row for S4 env wiring | Theme 2 (Drift Control row) + Theme 5 (welcome bundle, versioning, LDAP) |
| `openspec/changes/mkauth-core-architecture/ROADMAP.md` | Add Phase 5.5: Audit Resolution. Add Phase 6 stub for ⌘K. Move "reconciliation deferred P5" line to Phase 5.5 closure | Theme 5 |
| `openspec/INDEX.md` | Add 5 new change rows. Add Drift Control capability row. Update Phase mapping for 5.5 | Theme 5 |
| `.env.example` | Add `--- Sync Service / LLDAP ---` block. Drop "N>1 replicas" framing. Add Theme 2 vars (`DRIFT_RECONCILIATION_INTERVAL_HOURS`, `OUTBOX_MAX_RETRIES`) | Theme 5 |
| `openspec/changes/mkauth-core-architecture/specs/access-governance/spec.md` | Add Drift Triage workflow: detection sources, three actions, source remap, bulk attribution | Theme 2 |
| `openspec/changes/mkauth-core-architecture/specs/automation-policies/spec.md` | Per-rule confirmation_mode flag. Global default. Bulk toggle UX. Welcome bundle errors when not configured. Drop versioning | Theme 2 (rule confirmation) + Theme 1 (welcome bundle) + Theme 5 (versioning) |
| `openspec/changes/mkauth-core-architecture/specs/operational-readiness/spec.md` | Visual urgency tiers (drift > pending > steady). Motion/audio cues. Dismissibility rules | Theme 2 |

### 7.1 The single load-bearing doctrinal sentence

Current `design.md`: *"Backend is the single mutation authority — frontend and triggers signal, backend decides."*

After this cleanup:

> **Backend is the single mutation authority for Zitadel state. Every Zitadel grant — whether issued via MkAuth UI, derived from a bundle/rule, or detected as drift and back-filled — ends with a MkAuth-side intent record (reason, expires_at, granted_by, source). MkAuth-mediated mutations write the intent ledger before the Zitadel API call. Direct-Zitadel mutations are detected as drift and acquire their intent record through operator attribution. Operator point mutations require fresh confirmation before propagation; rule firings and expiry sweeps treat their authoring as pre-authorization. Drift detection is real-time via webhook with reconciliation as backstop; drift items are triaged via Attribute / Revoke / Mark external.**

That paragraph is the doctrinal change. Everything else in this plan implements it.

---

## 8. Out of scope / explicit non-goals

- **No new IdP integrations.** Phase 6 territory.
- **No multi-tenant generalization.** Single-LXC, one-operator audience.
- **No horizontal scale-out features.** The N>1 replicas framing is being dropped, not extended.
- **No mapping-rule-version rollback machinery.** Versioning is removed (D4); audit_logs replaces the use case.
- **No automatic resumption of buffered propagation when Zitadel returns from offline.** Operator must explicitly trigger drain. Documented in design.md.
- **No drift auto-resolution heuristics.** Every drift item requires explicit operator action (Attribute, Revoke, or Mark external). No "if drift age > 30d, auto-mark external."
- **No ⌘K command palette.** Stub'd to Phase 6.
- **No location field rendering.** Dropped (D6).
- **No `mapping_rules.version` column.** Dropped (D4).

---

## 9. Open questions for writing-plans phase

These are decisions the writing-plans skill will need to resolve when producing each themed change's `tasks.md`:

1. **Outbox drain trigger.** Operator clicks "Resume now" on dashboard; should there also be a per-mutation "Apply now" affordance on the inline form (so a single grant POST can drain inline rather than queue + click)? Recommendation: yes for single mutations from inline forms; no for cascades (always go through buffer).
2. **Reconciliation sweep cadence configurability.** Default 6h; should the operator be able to trigger manually from `/governance/drift`? Recommendation: yes — add `[Reconcile now]` button on the page.
3. **Outbox UI: per-row vs aggregate view.** Pending callout shows count; should the operator be able to expand to see per-row detail before clicking Resume? Recommendation: yes — `Resume now` opens a confirmation modal listing each pending row.
4. **Bundle-removal cascade other-source check.** When a user is removed from a bundle, the cascade revoke checks for other sources before enqueueing. The check window: real-time at enqueue, or deferred to drain? Recommendation: real-time at enqueue for accuracy; tolerate a small race window resolved by reconciliation.
5. **Drift queue ordering.** Default sort by `detected_at DESC`. Should there be filters (by user, project, source)? Recommendation: yes for `/governance/drift` filtering; not needed inline.
6. **Source-remap constraint validation.** When operator attributes drift to bundle X, should MkAuth verify the bundle's roles include the drift role-key? Recommendation: yes — modal disables incompatible bundles and shows why.
7. **Theme 2 sub-phase split.** Should Theme 2 ship as one OpenSpec change with internal phases, or three sequential change directories? Recommendation: one change with phased tasks.md; archive together.
8. **U7 test scope.** Should middleware/proxy tests exercise stale demo cookies in production mode (with `ZITADEL_DOMAIN` set)? Recommendation: yes — that's the regression we're guarding against.
9. **User-settings infrastructure for the drift audio toggle.** No user-settings page exists today. Three options: (a) introduce a minimal `/settings` page in this cleanup with the audio toggle as its first row; (b) put the toggle in localStorage only with a small preferences popover from the avatar menu; (c) defer the toggle to Phase 6 and ship audio with no kill switch. Recommendation: (b) — localStorage + popover is the minimum viable kill switch, and a real settings page can land later without redesigning anything.

---

## 10. Finding-to-theme cross-reference

Full traceability so writing-plans can verify coverage.

| Audit ref | Description | Theme | Item ID in theme |
|---|---|---|---|
| B1 | Delete `cmd/test/main.go` | T1 | T1.5 |
| B2 | Drop reconciliation cap to 2k | T2 | T2.b2 |
| B3 | Refactor `services/views.go` | T3 | T3.1 |
| B4 | Mutation authority via /zitadel/* CRUD | T2 | T2.b4 |
| B5 | Split `repositories.go` | T3 | T3.2 |
| B6 | Webhook silent defaults (event_type) | T3 | T3.5 |
| B7 | Vault typed sentinel | T3 | T3.6 |
| B8 | `grantLookupMaxPages` to 10 | T5 | T5.12 |
| U1 | Trim `useNameResolver` | T4 | T4.1 |
| U2 | Palette migration finish | T4 | T4.2 |
| U3 | `useGovernanceSummary` dedup | T4 | T4.3 |
| U4 | Delete `lib/api.ts` dead code | T4 | T4.4 |
| U5 | Split `app/zitadel/page.tsx` | T4 | T4.5 |
| U6 | Avatar fallback | T4 | T4.6 |
| U7 | Middleware + proxy tests | T4 | T4.9 |
| S1 | Extract `scripts/lib/load-env.sh` | T5 | T5.4 |
| S2 | Extract `scripts/lib/zitadel-api.sh` | T5 | T5.5 |
| S3 | Fold zitadel/actions docs | T5 | T5.6 |
| S4 | Wire `SYNC_RETRY_*` env | T5 | T5.7 |
| D1 | Welcome bundle errors explicitly | T1 | T1.3 |
| D2 | Strike ⌘K from spec | T4 | T4.7 |
| D3 | (= B4) | T2 | T2.b4 |
| D4 | Drop `mapping_rules.version` | T5 | T5.1 |
| D5 | (= C2) | T1 | T1.2 |
| D6 | Drop `UserProfile.location` | T4 | T4.8 |
| D7 | Document sync env in `.env.example` | T5 | T5.2 |
| D8 | (≈ B6) | T3 | T3.5 |
| D9 | Drop "N>1 replicas" framing | T5 | T5.3 |
| D10 | (= U2) | T4 | T4.2 |
| C1 | Production refuses missing signing keys | T1 | T1.1 |
| C2 | OIDC member dashboard metadata | T1 | T1.2 |
| C3 | Vault dev-mode self-attribution | T1 | T1.4 |
| C4 | Lift JWT claims into context | T3 | T3.3 |
| C5 | Cache `claim_failure_mode` in Redis | T3 | T3.4 |
| C6 | Overlay cache half-overlay | T2 | T2.c6 |
| C7 | LDAP empty-DN placeholder | T5 | T5.8 |
| C8 | Sync ctx propagation | T5 | T5.9 |
| C9 | `smoke-test-lxc.sh` /healthz | T5 | T5.10 |
| C10 | Zero-buffer theatre | T5 | T5.11 |
| C11 | (≈ B6) | T3 | T3.5 |

(Item IDs are placeholders; writing-plans will assign canonical IDs in each theme's tasks.md.)

---

## 11. Handoff to writing-plans

Next step: invoke `superpowers:writing-plans` against this design doc to produce the per-theme `proposal.md` / `design.md` / `tasks.md` artifacts under `openspec/changes/<theme>/`. Each themed change is its own writing-plans pass; no single mega-plan for the entire cleanup.

Recommended order for writing-plans invocation (matches Wave structure):

1. Theme 1 (smallest, most urgent)
2. Theme 4 (palette portion only)
3. Theme 3
4. Theme 5
5. Theme 2 (largest, most architectural; informed by Themes 3 and 4 being plan'd already)
6. Theme 4 (remainder + consolidation)
