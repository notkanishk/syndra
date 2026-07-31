# MkAuth — Next Steps

> Single pickup point. Everything open, in one place, as of **2026-07-31**.
> Sources consolidated here: `ROADMAP.md` phases 4–6, `AUDIT.md` deferrals, open `changes/*/tasks.md`, and tooling debt.
> [< Index](INDEX.md) · [Architecture](changes/mkauth-core-architecture/design.md) · [Roadmap](changes/mkauth-core-architecture/ROADMAP.md)

## How to read this

Four buckets, in the order they actually unblock each other:

1. **Code** — write it whenever.
2. **Operator-gated** — needs a live Zitadel/LLDAP and a human at the console. Nothing in code blocks these; they block *declaring things done*.
3. **Spec & docs debt** — why `openspec/specs/` was empty for so long, and what still can't be archived.
4. **Tooling debt** — OpenLore's UI blind spot.

---

## 1. Code

### LDAP / sync (the known parked track)

Paused pending research on real LLDAP password-propagation and credential semantics. Listed for completeness — you already know this one.

- **SC2 — top priority when sync resumes.** `IsConnectionError` doesn't classify `ErrorNetwork`, so a network-class LDAP failure isn't treated as a reconnect trigger. [`sync/internal/ldap/client.go`]
- **SC6** — per-UID worker routing (same-UID ordering guarantee).
- **SC7** — fresh bounded context for drain-time backend calls.
- **LLDAP Integration** — end-to-end wiring against the real external LLDAP (separate Proxmox LXC), production connectivity validation.
- **LLDAP Reconciliation** — periodic full sync comparing MkAuth provisioning state to LLDAP group membership, overwriting drift per the one-way authority rule.
- **OE7 / OE13 / OE14** — cosmetic sync cleanups; do them while you're in there, not as their own errand.

### Basic / Advanced IA — what the redesign left open

- **BIA-35** — exercise the claim editor, the simulator, role → members and `DELETE /users/{id}/grants/{grantId}` against a live Postgres + Redis + Zitadel. Unit-tested only; the write paths have never run against a real database.
- **BIA-36** — the per-route manual a11y checklist (empty / loading / error / dense / ultra-wide / narrow / keyboard / light-theme contrast). The components are built to pass it; nobody has walked it.
- **BIA-37** — legacy path alignment (`/policies` → `/automation/rules`, `/graph` → `/automation/access-map`, `/operations` → `/system/events`) behind redirects. Cosmetic, deliberately deferred: the nav labels are already right, only the URLs lag.
- **Claim template versioning** — a claim profile edit takes effect on the next token with no record of what the previous shape was. If an app breaks after an edit, the audit row says who changed it, not what it was. Worth a `claim_profile_versions` table if this bites.

### Phase 5 — Automation & Governance

- **Service Catalog Abstraction** — the spec'd service→bundle request mapping still falls back to project/role. [`specs/service-catalog`]
- **Partial Failure Rollback** — `EnforceMappingRules` / `RevokeMappingRules` are best-effort log-and-continue. No compensating revocation when a Zitadel call partially fails, so a half-applied mapping rule stays half-applied silently.
- **Rate Limiting** — webhook, action-injection, and shadow-password endpoints are unthrottled. **Blocked on one decision: in-process vs Redis-backed.** Redis is already a dependency, so the cost difference is small; the real question is whether limits must hold across replicas.
- **Advanced Filters** — multi-dimensional user search (project, role, account age, grant staleness).
- **Bulk Operations** — mass grant/revoke with preview, per-user outcomes, idempotent retry.
- **Observability** — metrics, alert thresholds, dashboard integration beyond structured logs.
- **CI/CD** — automated test runs, migration validation, container build verification. Nothing exists today; every check in this repo is run by hand.

### Phase 6 — IdP Lifecycle (the big one)

- **Google Workspace Account Poller.** Nothing built yet. Separate container, monthly Admin SDK Directory API poll, verifies every Zitadel user still has an active Google account, deactivates suspended/deleted ones via the Management API. The deactivation then cascades through the existing webhook pipeline (`user_deactivated` → cache invalidation → LLDAP membership revocation via provisioning intents), so the downstream half is already built and tested. Design: `changes/mkauth-core-architecture/design.md` §10.
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

**Do not archive `mkauth-core-architecture`.** Its `tasks.md` is empty, so it looks archivable, but it's the living architecture and roadmap hub — `CLAUDE.md` links directly into it and Phases 5–6 are still open.

---

## 4. Tooling debt

**OpenLore parses `.tsx` with the wrong grammar.** 94 of 94 `.tsx` files degrade with 5558 error regions; 0 of 46 `.ts` files degrade. Every symbol and call edge in the UI layer is a **lower bound** — this is why `orient` reports `fanIn: 0` on functions that demonstrably have callers, and why any dead-code claim about `ui/` needs a grep to confirm.

Located contributor: `getTSParser()` hardcodes `tsModule.default.typescript` and never selects `.tsx`, though `tree-sitter-typescript` exports both grammars (`dist/core/analyzer/call-graph.js:134`, openlore 2.1.7).

Patching that line is a measurable but **partial** win: function count 2743 → 2872, plus 24 more files with extracted bodies. `parse-health` still reports 94/5558, so the grammar is not the whole cause — the rest needs upstream triage.

**The patch is deliberately not applied.** It only ever lived in a package-manager cache, so it evaporates on any cache clear or version bump — an index that silently loses 129 functions when the cache rotates is worse than one that's consistently a lower bound. The current index is stock-parser output (2743 functions). File the upstream issue against openlore 2.1.7 with the line reference above; re-measure when it lands.

Everything else is healthy: index tracks HEAD, embeddings are local-semantic (`Xenova/all-MiniLM-L6-v2`, on-device, no API key), spec index builds (46 sections), MCP runs under `bunx` pinned to `openlore@2.1.7`.

> Runner note: the toolchain is `bunx`, not `npx` (`.mcp.json`, `.claude/settings.json`). Version is pinned because unpinned `bunx openlore` resolved to a stale 2.1.6 while `npx` fetched 2.1.7 — pin both or they drift apart.

---

## 5. Declined / deliberately kept

Don't re-litigate these without new information.

- **OE15** — `golang-migrate` kept. Its dirty-state guard on mid-migration failure is worth the one dependency; a stdlib runner would drop that recovery.
- **OE21** — `sortedKeys` → `slices.Sorted(maps.Keys(…))` declined. `sortedKeys` returns a non-nil empty slice, `slices.Sorted` returns `nil`, and `zitadelByPair[k]` is allocated before its `RoleKeys` loop — so a Zitadel grant with zero roles would flip `"role_keys": []` to `null` in the reconciliation response. Six lines is not worth a JSON contract change.
