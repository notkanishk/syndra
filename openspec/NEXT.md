# Syndra — Next Steps

> Single pickup point. Everything open, in one place, as of **2026-07-31**.
> Sources consolidated here: `ROADMAP.md` phases 4–6, `docs/AUDIT.md` deferrals, open `changes/*/tasks.md`, and tooling debt.
> [< Index](INDEX.md) · [Architecture](changes/syndra-core-architecture/design.md) · [Roadmap](changes/syndra-core-architecture/ROADMAP.md)

## How to read this

Four buckets, in the order they actually unblock each other:

1. **Code** — write it whenever.
2. **Operator-gated** — needs a live Zitadel/LLDAP and a human at the console. Nothing in code blocks these; they block *declaring things done*.
3. **Spec & docs debt** — why `openspec/specs/` was empty for so long, and what still can't be archived.
4. **Tooling debt** — OpenLore's UI blind spot.

---

## 1. Code

### LDAP / sync (the known parked track) — **superseded, not resuming**

**Abandoned by [`changes/addon-platform`](changes/addon-platform/proposal.md)** (2026-08-08), which reaches TrueNAS SCALE and UniFi Access through their own management APIs instead of an intermediate directory. Nothing in this subsection is picked up again; `sync/` is deleted in that change's group 11, which is deliberately last because the vault reduction inside it is the point of no return. The items stay listed because a parked track that vanishes reads as forgotten rather than decided.

**Active pickup point for that work is [`changes/addon-platform/tasks.md`](changes/addon-platform/tasks.md).** Group 1 (the target dimension: `targets` registry, `propagation_outbox` rename and reshape, `desired_state_snapshots`, plan storage, drift target dimension — migration `000026`) is done and stands alone. Group 2 begins at 2.1; 2.17 onward and all of group 3 depend on group 1's plan storage, which now exists.

Paused pending research on real LLDAP password-propagation and credential semantics. Listed for completeness — you already know this one.

- **SC2 — top priority when sync resumes.** `IsConnectionError` doesn't classify `ErrorNetwork`, so a network-class LDAP failure isn't treated as a reconnect trigger. [`sync/internal/ldap/client.go`]
- **SC6** — per-UID worker routing (same-UID ordering guarantee).
- **SC7** — fresh bounded context for drain-time backend calls.
- **LLDAP Integration** — end-to-end wiring against the real external LLDAP (separate Proxmox LXC), production connectivity validation.
- **LLDAP Reconciliation** — periodic full sync comparing Syndra provisioning state to LLDAP group membership, overwriting drift per the one-way authority rule.
- **OE7 / OE13 / OE14** — cosmetic sync cleanups; do them while you're in there, not as their own errand.

### Basic / Advanced IA — what the redesign left open

- **BIA-35** — exercise the claim editor, the simulator, role → members and `DELETE /users/{id}/grants/{grantId}` against a live Postgres + Redis + Zitadel. Unit-tested only; the write paths have never run against a real database.
- **BIA-36** — the per-route manual a11y checklist (empty / loading / error / dense / ultra-wide / narrow / keyboard / light-theme contrast). The components are built to pass it; nobody has walked it.
- **BIA-37** — legacy path alignment (`/policies` → `/automation/rules`, `/graph` → `/automation/access-map`, `/operations` → `/system/events`) behind redirects. Cosmetic, deliberately deferred: the nav labels are already right, only the URLs lag.
- **Claim template versioning (C5)** — a claim profile edit takes effect on the next token with no record of what the previous shape was. If an app breaks after an edit, the audit row says who changed it, not what it was. **Trigger:** the first production claim change that needs comparison or reversal. Build it as an immutable snapshot tied to the application and the audit event, not a draft/publish lifecycle — a bundle edit reaches people, a claim edit reaches a token shape, and the question asked of it is "what was it before".

### IA screen completion — what the second handoff drop left open

- **ISC-42** — exercise migrations `000019` and `000023`, the enriched drift read and the restored upstream console against a live Postgres + Redis + Zitadel. Unit-tested only. `cascade_id` in particular has never been written by a real cascade — on the outbox or, now, on `audit_logs`.
- **ISC-43** — per-route manual a11y checklist for the twelve screens this change touched. Same checklist as BIA-36; these routes were rebuilt after it was written.
- ~~**ISC-44**~~ — **closed.** `audit_logs.cascade_id` (migration `000023`) is stamped inside `enqueueCascadeRows`, which already minted the id and discarded it; the eleven audit inserts that sat one line above it moved inward, and a source guard fails if a twelfth appears. Rows written before the column keep their object id, unlinked, and are deliberately not backfilled. Change history answers `?cascade=`.
- ~~**ISC-45**~~ — **decided: one app lives in one project**, matching Zitadel and the UNIQUE constraint the schema already carries. The design diagram showing one app reading four projects was the thing that was wrong. Reopens on a real integration needing roles from two projects in one token — that is a product event, not a refactor. See `ui-capability-gap-closure` design Decision 14.
- **Hardware sync state on the person page (C9b)** — blocked on the same contract E1 needs: per user and device, the desired version, last attempt, result, error and timestamp. Raw grant ids (C9a) shipped; this half cannot until the bridge defines that shape.
- ~~**Expiring-access acknowledgement (C4)**~~ — **built** (migration `000024`), with the **reopens-when-the-grant-changes** rule. Implemented as a stored `acknowledged_expires_at` plus one join condition comparing it to the grant's current date: no trigger, no sweep, nothing a future write path can forget, and verifiable without a live database. Per-row only, grouped rather than hidden. See `ui-capability-gap-closure` design Decisions 18–19.
- **Drift evidence for sweep-detected rows (C8)** — the reconciliation sweep cannot name an actor because it compares grant sets. "Unknown actor" is the honest rendering — the sweep is an automated observation, not an action somebody took. Reading Zitadel's event stream for the grant's creation event would close it, at the cost of a second API path. Only worth it if operators routinely hit rows the webhook missed, and nothing counts that today.

### Operator runbook surfaces — what the reset work left open

- **ORS-18** — `reset-data.sh all --apply` has never committed. `demo --apply` has now run for real against the live deployment (48 rows, one transaction, real accounts untouched), and `all`'s plan rehearses and rolls back clean, but the `TRUNCATE ... RESTART IDENTITY CASCADE` has never been allowed to commit. Worth a `pg_dump` before the first one.
- **ORS-19** — `CountDemoResidue` matches audit rows on actor and target user id, and the seeder writes audit actors as display names (`alice.rivera`, `maya.chen`), so two of its four seeded audit rows are invisible to the count. Harmless while any other table carries residue — the banner still fires — but it would under-report on a database where audit rows are all that remain. Fix by matching the seeder's actor strings, or by giving seeded audit rows a marker.
- **Wider fixture drift** — the residue check and the reset script both read the demo catalog's project and user ids, and a `demo` package test fails when the script's copy diverges. Nothing guards a *new table* the seeder starts writing: it would be missed by both, silently. Worth revisiting if the seeder grows.

### Login doorway — what the brand handoff left open

- ~~**App-wide violet**~~ — **shipped.** Both themes now point at the violet ramp, and lime returned as the `Healthy` role rather than being retired. The token swap was the easy half; the half that could not be mechanical was finding every place the old accent stood in for "this is fine", since violet must never mean good or safe. Also brought the six motion roles, the contained-orb mark and the first favicon. See `changes/violet-and-motion/`.
- **The one thing left from it:** the GitHub social preview. GitHub exposes no REST endpoint, so `gh` cannot set it — the file is staged at `docs/assets/social-preview.png` and wants a manual upload at `github.com/notkanishk/syndra/settings`.
- ~~**Unauthenticated pages fetch authenticated data**~~ — **fixed.** `NameResolverProvider` takes an `enabled` gate fed by the session the root layout already resolves, covering all three of its requests including the per-miss `POST /lookup`; `/login` now issues zero proxy requests and logs nothing. Both `enabled` and `hasSession` default to `true`, so a caller who forgets the prop gets working name resolution rather than blank names.
- ~~**Ambience toggles**~~ — **shipped on.** Breathing pool and animated grain are CSS `@keyframes` inside `@media (prefers-reduced-motion: no-preference)`, animating the two properties the choreography never touches. Measured 120fps, worst frame 9.3ms.

### Phase 5 — Automation & Governance

- **Service Catalog Abstraction** — the spec'd service→bundle request mapping still falls back to project/role. [`specs/service-catalog`]
- **Partial Failure Rollback** — `EnforceMappingRules` / `RevokeMappingRules` are best-effort log-and-continue. No compensating revocation when a Zitadel call partially fails, so a half-applied mapping rule stays half-applied silently.
- **Rate Limiting** — webhook, action-injection, and shadow-password endpoints are unthrottled. **Blocked on one decision: in-process vs Redis-backed.** Redis is already a dependency, so the cost difference is small; the real question is whether limits must hold across replicas.
- **Advanced Filters** — multi-dimensional user search (project, role, account age, grant staleness).
- **Bulk Operations** — mass grant/revoke with preview, per-user outcomes, idempotent retry.
- **Observability** — metrics, alert thresholds, dashboard integration beyond structured logs.
- **CI/CD** — automated test runs, migration validation, container build verification. Nothing exists today; every check in this repo is run by hand.

### Phase 6 — IdP Lifecycle (the big one)

- **Google Workspace Account Poller.** Nothing built yet. Separate container, monthly Admin SDK Directory API poll, verifies every Zitadel user still has an active Google account, deactivates suspended/deleted ones via the Management API. The deactivation then cascades through the existing webhook pipeline (`user_deactivated` → cache invalidation → LLDAP membership revocation via provisioning intents), so the downstream half is already built and tested. Design: `changes/syndra-core-architecture/design.md` §10.
- **⌘K command palette** — optional, struck from spec in the May 2026 audit. Only if operator navigation demand shows up.

---

## 2. Operator-gated

None of these are code. All need a live instance and a human.

- **Actions v2 key lifecycle** — `make zitadel-actions-register`, then `make zitadel-actions-rotate-key`, verify the new key lands in both the response and `.action-signing-key` with the old one in `.action-signing-key.previous`; swap env var, restart, confirm `make zitadel-actions-verify` passes.
- **Actions v2 smoke** — `go run ./backend/cmd/api` + `scripts/smoke-test-action-v2.sh`, expect 200 with an `append_claims` array.
- **Live directory smoke** — confirm `[DIRECTORY] Source=zitadel` at startup; `/users`, `/projects`, `/bundles`, `/applications` show real Zitadel entities, not Alice/Sam/Laser-Lab. Then the demo-mode regression: unset `ZITADEL_MACHINE_KEY_PATH`, restart, expect `[DIRECTORY] Source=demo`.
- **Application/metadata smoke** — `/applications` shows real app names per project typed `OIDC Client` / `API` / `SAML SP`; setting a user's `title` metadata in the Zitadel console reflects on `/users` within ~30s.
- **Event-trigger end-to-end** — live verification of the lifecycle event path.
- **Grant expiry smoke** — seed a grant with `expires_at=NOW()+'10s'`, confirm the row is removed, audit + intent rows written, and the `[SCHEDULER] Sweep complete duration=...` line appears.
- **Session name on a live login** (`PBD-33`) — sign in against the live Zitadel and confirm the shell header and the Today greeting render the operator's *name*, not their subject id. This was the defect `people-bulk-and-dashboard-depth` fixed, and it cannot be confirmed from a test: the demo/fixture path never had it, because those sessions always carry a name.
- **Bulk rehearsal against real data** (`PBD-34`) — run one `POST /grants/bulk` rehearsal over a real cohort and confirm each per-person verdict matches what that person's own screen says. The rehearsal is the only thing standing between a bulk remove and an unrecoverable surprise, so it is worth checking once against data nobody wrote as a fixture.
- **Obsidian-clarity UI checklists** (`OCR-S2-19`, `OCR-S3-10`) — per-route manual pass over empty / loading / error / dense / ultra-wide / narrow / keyboard / light-theme contrast. Browser-driven, can't be automated as specified.

---

## 3. Spec & docs debt

**Fixed 2026-07-30:** `openspec/specs/` was empty — every spec lived as an unpromoted delta under `changes/*/specs/`, so OpenLore skipped its spec index entirely and `check_spec_drift` / `search_specs` / `orient`'s `specDomains` all returned nothing. Five changes are now archived and seven capabilities consolidated (`access-governance`, `automation-policies`, `backend-api-testing`, `contract-quality`, `operational-readiness`, `production-security-boundary`, `user-management`). Spec index builds: 46 sections.

**Still blocked — six changes are complete but cannot be archived.** They have no `specs/` directory at all, so there is no delta for `openspec archive` to merge. Their requirements were never written down anywhere:

| Change | Blocker |
|---|---|
| `advanced-role-crud` | no `specs/` |
| `codebase-audit-and-hardening` | no `specs/` |
| `live-webhook-listener` | no `specs/` |
| `provisioning-intents` | no `specs/` |
| `shadow-password-vault` | no `specs/` |
| `zitadel-management-client` | no `specs/` |

Archiving them with `--skip-specs` would clear `changes/` but permanently lose the requirements — don't. Each needs spec deltas authored from the shipped code first (`## ADDED Requirements` + `### Requirement:` + `#### Scenario:`). `backend-owned-onboarding-and-security-boundary` had this exact problem and was fixed by reformatting its existing spec to delta headers; the other six need the content written, not just reformatted.

**Status conflicts to resolve.** `INDEX.md` marks these "In progress" but their `tasks.md` shows zero open tasks. One of the two is lying:

- `wave-1-production-trust-hardening`
- `wave-2-part-1-frontend-palette-finalization`
- `wave-2-part-2-backend-coherence`
- `wave-2-part-3-operational-polish`

**Stale roadmap entry.** `ROADMAP.md` Phase 5 lists **Welcome Bundle Configuration** as open, describing `GetWelcomeBundle` as convention-based name matching to be replaced. It shipped: migration `000012_welcome_bundle_flag`, `is_welcome` column, operator-gated `PUT /api/v1/bundles/{id}/welcome`, UI on the bundles page. Tick it.

**Do not archive `syndra-core-architecture`.** Its `tasks.md` is empty, so it looks archivable, but it's the living architecture and roadmap hub — `CLAUDE.md` links directly into it and Phases 5–6 are still open.

---

## 4. Tooling debt

**OpenLore parses `.tsx` with the wrong grammar.** 94 of 94 `.tsx` files degrade with 5558 error regions; 0 of 46 `.ts` files degrade. Every symbol and call edge in the UI layer is a **lower bound** — this is why `orient` reports `fanIn: 0` on functions that demonstrably have callers, and why any dead-code claim about `ui/` needs a grep to confirm.

Located contributor: `getTSParser()` hardcodes `tsModule.default.typescript` and never selects `.tsx`, though `tree-sitter-typescript` exports both grammars (`dist/core/analyzer/call-graph.js:134`, openlore 2.1.7).

Patching that line is a measurable but **partial** win: function count 2743 → 2872, plus 24 more files with extracted bodies. `parse-health` still reports 94/5558, so the grammar is not the whole cause — the rest needs upstream triage.

**The patch is deliberately not applied.** It only ever lived in a package-manager cache, so it evaporates on any cache clear or version bump — an index that silently loses 129 functions when the cache rotates is worse than one that's consistently a lower bound. The current index is stock-parser output (2743 functions). File the upstream issue against openlore 2.1.7 with the line reference above; re-measure when it lands.

Everything else is healthy: index tracks HEAD, embeddings are local-semantic (`Xenova/all-MiniLM-L6-v2`, on-device, no API key), spec index builds (46 sections), MCP runs under `bunx` pinned to `openlore@2.1.7`.

> Runner note: the toolchain is `bunx`, not `npx` (`.mcp.json`, `.claude/settings.json`). Version is pinned because unpinned `bunx openlore` resolved to a stale 2.1.6 while `npx` fetched 2.1.7 — pin both or they drift apart.

---

## 4b. Test infrastructure debt

- **`internal/db` has no live-database harness.** Every assertion in that package is a migration-coherence or SQL-text guard, so anything that only manifests as an interleaving of two transactions is asserted structurally rather than executed. Three review findings in a row bottomed out here: apply-vs-deregistration, enqueue-vs-deregistration, and disable-during-dispatch. The fixes are in and guarded by source-level checks; what is missing is a test that actually runs them.

  `golang-migrate` is already a dependency, so the harness is small: connect to a `SYNDRA_TEST_DATABASE_URL`, migrate up once, skip every live test when the variable is unset so `go test ./...` stays green without a database. What it needs is a throwaway Postgres — there is none on the development machine (no Docker, no `psql`), which is why this is debt rather than done.

  Also blocked on it: the live-row half of 2.18 (a plan persists and expires), 2.20 (a fingerprint mismatch mutates nothing), 2.22 (scan plan rows for a submitted secret), 1.11's real interleavings, and 1.21/2.46's — a concurrent apply for one subject genuinely serializing, the settled state equalling the higher version, and a grant overtaken by a later revoke actually terminating `superseded` rather than being asserted to.

## 4c. Owed operator surfaces

- **The unreconciled-target record has no dashboard.** `target_reconciliation` (migration 000026, change `addon-platform` 1.14) records when Syndra last saw each target for itself and since when it has not. The on-demand sweep returns it on `DriftResult`, so [Reconcile now] shows it; the scheduled sweep writes it and nothing reads it back. `db.GetUnreconciledTargets` exists for that consumer and currently has none.

  This matters most in exactly the case it was built for: a nightly sweep that has been unable to reach a target for a week looks, on every surface an operator actually opens, like a week with no drift. The natural home is the governance summary beside the drift count — which needs `TargetReconciliation` moved to `internal/models` first, since `models` must not import `db`. Deliberately not done inline with 1.14: a backend field with no rendering is not "saying so" to anybody, and inventing the callout unprompted is a design decision the IA change owns (`basic-advanced-ia`).

## 5. Declined / deliberately kept

Don't re-litigate these without new information.

- **OE15** — `golang-migrate` kept. Its dirty-state guard on mid-migration failure is worth the one dependency; a stdlib runner would drop that recovery.
- **OE21** — `sortedKeys` → `slices.Sorted(maps.Keys(…))` declined. `sortedKeys` returns a non-nil empty slice, `slices.Sorted` returns `nil`, and `zitadelByPair[k]` is allocated before its `RoleKeys` loop — so a Zitadel grant with zero roles would flip `"role_keys": []` to `null` in the reconciliation response. Six lines is not worth a JSON contract change.
