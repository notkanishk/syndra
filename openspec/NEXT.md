# Syndra — Next Steps

> Single pickup point. Everything open, in one place, as of **2026-08-10**.
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

### LDAP / sync — **gone**

Deleted on 2026-08-10 by [`changes/addon-platform`](changes/addon-platform/proposal.md),
group 11. `sync/`, the `provisioning_intents` queue, the four intent routes and
the Argon2id password vault are removed; migration `000034` drops the tables and
the credential columns. The items that used to be listed here — SC2's error
classification, SC6's per-UID routing, SC7's bounded context, LLDAP integration,
LLDAP reconciliation, OE7/OE13/OE14 — are not open work any more. There is
nothing to resume.

**One consequence outlives it:** every member who enrolled before the cutover
must set a new storage credential. Their hash went with the vault, and it could
not have been converted anyway — TrueNAS takes plaintext and nothing else. The
member view renders the re-enrolment state; the operator communication is the
thing to do before this deploys.

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

- **~~Deploy `reconciliation-as-merge`~~ Done, 2026-08-19.** Four migrations (`000041`–`000044`) applied to the dev deployment at `2085e91`; `schema_migrations` reads `44 | not dirty`, database dumped to `/root/syndra/backups/pre-000044-*.sql.gz` first. Backend, UI and the TrueNAS add-on rebuilt — the add-on had to be, because `/subjects` now serves each account's state in entitlement vocabulary and without it every subject classifies as baseless. Target `callable`, eight operations, a live reconcile read the real NAS and concluded `bound 0 · queued 0 · current`, and the NAS is untouched.

  **~~What the deployment has NOT exercised: the classifier's own comparison.~~ Done, 2026-08-24.** An account was provisioned through the whole path against the real NAS (rehearse → apply → drain → `user.create`), then changed behind Syndra's back by removing it from its managed group. The sweep classified it `theirs_only` with `base`/`ours`/`theirs` correct, `adoptable: false`, the reason given, and the owning mapping named with its holder count. `keep_ours` recorded the decision and honestly reported `resolved: false` — the finding stays open until a reconciliation sees the target agree — and the next apply restored the group and cleared it. A plan approved before the restore was refused as `PLAN_STALE`, which is the fingerprint gate working. Everything created was removed afterwards and the NAS is byte-for-byte as it was.

- **Add-on shutdown drain** (`addon-shutdown-grace-period` 3.3) — stop the add-on with a mutation in flight and confirm the settle completes and the terminal status is written. The fix and its guard are in: `stop_grace_period: 30s` now exceeds the add-on's `shutdownTimeout`, and a test fails if that inverts. But everything asserted so far is *numbers*. Only this proves the drain actually survives a real stop, which it had not done for the whole life of the add-on — Docker's 10s default was cutting a 20s drain in half, invisibly, because a truncated drain and a clean stop look identical from outside.
- **~~One TrueNAS API key~~ Done, 2026-08-13.** The full chain is live against the real NAS: key minted on a dedicated user, `wss://` authenticated, version read (`TrueNAS-25.10.5`), all four health reads answering, and the real account inventory returned. The one thing it immediately broke is recorded as 7.4b — `system.version` returns a `TrueNAS-` prefix the recorded fixture never had, and the version gate silently refused every mutation on a supported release. What remains unproven is `audit.query` / `sharing.smb.query`: they belong to `activity.get`, which has no backend route yet, so those two roles are configured and untested. Previous entry:
- **A drain with a mutation actually in flight** (`addon-shutdown-grace-period` 3.3) — the stop itself is observed live: the drain runs, the process exits itself in ~1s against a 30s grace, exit 0, no SIGKILL. **The behaviour is now covered in process** (3.3a): the real server, routes, authenticator and lifecycle, with a target call that has not answered yet — `Shutdown` does not return while the handler is inside it, a second mutation mid-drain is refused, and the released one completes and writes its terminal record. Mutation-verified with `Close`, which abandons it. What is left is narrow and needs hardware: the same thing against a real NAS whose call is genuinely slow. The stub answers in milliseconds, so the window has to be manufactured — which is fine for asserting this side of the call and proves nothing about the NAS side.
- **Enable SMB auditing on the shares** (`addon-platform` 34.12) — it is off on both `gitlab_data` and `main`, so a member's activity report can only ever contain authentication events, never file access and never a share name. Syndra cannot do this itself and should not be able to: `sharing.smb.update` answers `EACCES` because the add-on's credential holds `SHARING_SMB_READ` and not write. Shares → SMB → Edit → Advanced, per share. Until then `activity.get` is correct and nearly empty, and says so.
- **A failing disk on the NAS** — `1 uncorrectable errors reported for sde (SERIAL0000)`, standing since 2026-07-06. Surfaced on the target page now that `health.get` has a caller (34.8), which is how it was found. Not a Syndra problem; it is the first thing that surface was built to show.
- **The live deployment has no add-on at all.** `syndra.example.org` → caddy `198.51.100.15` → app `198.51.100.12:3000`, and `/opt/syndra` there carries zero `TRUENAS_*`/`ADDON_*` variables, no add-on container, and a `sync/` directory — it predates `addon-platform` entirely. Everything in this change is dev-only until that is deployed, and the deploy is a separate decision with its own migrations.
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
| `provisioning-intents` | no `specs/` — **and now superseded**: the pipeline it describes is deleted. Archive it with a supersession note rather than authoring a spec for a subsystem that no longer exists. |
| `shadow-password-vault` | no `specs/` — **and now partly superseded**: the hash, the algorithm and the salt parameters are gone (migration 000034); existence and rotation metadata survive. |
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

## 4a. Add-on platform — complete

`changes/addon-platform` is done — **every row is ticked**, including the four
handover screens this section used to list as owed. (It said "three rows
unticked" and "each has a backend endpoint and no screen"; both were true when
written and neither is now. The entry below is what the section looked like
before, kept because the reasoning about §13/§17 is still the thing to read
first.) The backend's IAM half, the TrueNAS add-on,
the dispatcher joining them, the lifecycle trigger that fires it, the unmanaged
inventory, provisional plans, the mutation-log anchor, and the retirement of the
LLDAP bridge are all in and green. What remains is a **visual pass on three
screens**, handed to the design agent with
[`HANDOVER-UI.md`](changes/addon-platform/HANDOVER-UI.md).

**Start at the header of [`tasks.md`](changes/addon-platform/tasks.md), then §13
and §17.** The header names the pattern behind nearly every defect on the branch
— two internally-consistent definitions of one thing — and says which rows are
still open and why. §13 and §17 record the same
class of defect, found twice: the add-on had never spoken to a real NAS (§13),
and the backend and the add-on had never spoken to each other (§17) — the
backend's envelopes carried fields the add-on's strict decoders never declared,
so every real `/apply` and `/operations/*` call would have been answered 400.
Neither suite could see it, because each was written against its own fake and
the two fakes agreed with each other. The contract is an artifact now
(`addons/contract/*.json`), asserted from both ends.

**~~What the handover covers, and nothing else does yet.~~ All four are built**
— `ConvergeEntitlements.tsx`, `MappingManagement.tsx`, `DormantAccounts.tsx`
and the allowance surfaces. Listed as they were scoped:

1. **Entitlement plan-then-apply UI** (9.3–9.6) — `POST /targets/{t}/entitlements/rehearse` and `.../apply`. The apply carries the plan id, never the original submission.
2. **Mapping management with version history** (9.7–9.8) — `rehearse-edit`, `rehearse-delete`, and `PATCH`/`DELETE` citing a plan id. The blast-radius acknowledgement is enforced by the backend; the UI has to show the number.
3. **Dormant-account housekeeping** (9.11–9.12) — the only one that also needs a listing endpoint.
4. **Allowance authoring and review-date surfacing** (9.22, 9.25).
5. **Connection instructions** (10.8) — the account name is already on the member page; the mount instructions are not.
6. **A button for the revocation composition** (6.17) — `POST /targets/{t}/users/{id}/revoke-access` exists and nothing calls it. Its copy is fixed by the backend and must be shown verbatim: this target cannot end a session.

**It has now run end to end — against a stand-in, not a NAS.** The dev LXC ran
the whole platform: migrations 25→34 against a clone of the live database, both
binaries in their own containers over mutual TLS, and a stand-in middleware
speaking TrueNAS's JSON-RPC on the far end. Rehearse → apply → drain creates the
account under the name the plan promised; a replayed plan is refused; a
provisional plan issued against the add-on's mirror is refused at dispatch as
`PLAN_STALE` when the subject has moved; the mutation log deleted is detected.
**Start at the header of `changes/addon-platform/tasks.md`, then §13 and §17.**
§19 records the fourteen defects the first deployment found, seven of which
every test in both suites passed straight through.
§23–§31 record a full audit of the branch afterwards, and the header of that
file is the part to read first: **the recurring defect is two
internally-consistent definitions of one thing** — two fakes agreeing with each
other, `btrim` against `TrimSpace`, the proxy allowlist against the router, a
comment against the code beside it. Each side correct, tested, agreeing with
itself. Look wherever two things have to agree and nothing makes them.

**Partly operator-gated still, and §2 above has moved ahead of this
paragraph.** The full chain HAS touched the real NAS since 2026-08-13 — key
minted, `wss://` authenticated, version read, health reads answering, real
inventory returned — and this paragraph predates that. What remains untouched
is narrower: What that leaves untested is TrueNAS's own behaviour
rather than Syndra's: the filter syntax `user.query` actually accepts, what
`user.update` does with a `groups` list, the auth rate limiter and its ten-minute
lockout, and whether `builtin` is on every supported major. Point `TRUENAS_URL`
at the real one, run the same sequence, and read the mutation log afterwards.
The bring-up is now written down — DEPLOY.md step 5a for the proxy and
"Bringing up the TrueNAS add-on" for the NAS identity, the transport secret
(minted by the deployment itself — `truenas-addon-secret` in the compose file)
and the start order, which is now two `.env` lines and `docker compose up -d`.

**The transport under that bring-up changed after it was written.** The
certificate ceremony is gone: one secret per target, both keys derived from it
at both ends (`addon-transport-derived-keys`). Sequencing was meant to be the
other way round — the live bring-up first, so a NAS-side failure and a
transport-side failure could never be diagnosed together — and it was not, so
the first real bring-up carries a transport that has never handshaked outside a
test. Worth knowing while reading the failure, not a reason to redo it.

**`activity.get` now has a caller.** It was implemented by the add-on and
declared by the backend from the day the platform landed, and no route ever
dispatched it — which is why `audit.query` and `sharing.smb.query` sat
configured and unexercised. `GET /targets/{t}/activity` reaches it and the
person's Activity tab renders it beside Syndra's own feed, deliberately as a
second card (`addon-platform` §33). Whether the real NAS answers those two
methods as the fixture says is now a question somebody can ask from a screen.

**One of those questions is answered, and the answer contradicts what the branch
said.** The API key's permission set does cover `user.create`/`user.update`
without FULL_ADMIN — `ACCOUNT_WRITE` is enough — but the same role is what
`user.delete` requires, and TrueNAS publishes no narrower one. So the standing
key **can** delete an account, which `nas.go`, `.env.example` and the design all
denied. The injected purge key is an audit and blast-radius separation, not the
capability separation claimed. Corrected in place; recorded as
[`addon-platform` §32](changes/addon-platform/tasks.md) — including the design's
own account of the purge key (`design.md` line 224), which this entry used to
list as the one copy still unreworded and which now carries the correction.

**And the deployment manifest was carrying the branch's recurring defect.**
Four variables the add-on reads were passed by no Compose service, so no
deployment could set them — `TRUENAS_SHARE_HOST` worst of the four, because
unset it makes the manifest omit its connection block exactly as designed, so
the member page dropped its mount instructions and nothing reported a fault.
Wired, and guarded by a test that reads the add-on's own source against the
Compose service block (§32.3).

## 4b. Test infrastructure debt

- **`internal/db` has no live-database harness.** Every assertion in that package is a migration-coherence or SQL-text guard, so anything that only manifests as an interleaving of two transactions is asserted structurally rather than executed. Three review findings in a row bottomed out here: apply-vs-deregistration, enqueue-vs-deregistration, and disable-during-dispatch. The fixes are in and guarded by source-level checks; what is missing is a test that actually runs them.

  `golang-migrate` is already a dependency, so the harness is small: connect to a `SYNDRA_TEST_DATABASE_URL`, migrate up once, skip every live test when the variable is unset so `go test ./...` stays green without a database. What it needs is a throwaway Postgres — there is none on the development machine (no Docker, no `psql`), which is why this is debt rather than done.

  Also blocked on it: the live-row half of 2.18 (a plan persists and expires), 2.20 (a fingerprint mismatch mutates nothing), 2.22 (scan plan rows for a submitted secret), 1.11's real interleavings, and 1.21/2.46's — a concurrent apply for one subject genuinely serializing, the settled state equalling the higher version, and a grant overtaken by a later revoke actually terminating `superseded` rather than being asserted to.

## 4c. Owed operator surfaces

- **~~The unreconciled-target record has no dashboard.~~ Built.** It is in the governance summary and on the home queue, and it counts toward the "nothing needs you" decision — which was the point, since an unread target produces no findings and a blind week otherwise renders exactly like a quiet one (`addon-platform` 1.14a). The note below is the state before that.

  <details><summary>Previous entry</summary> `target_reconciliation` (migration 000026, change `addon-platform` 1.14) records when Syndra last saw each target for itself and since when it has not. The on-demand sweep returns it on `DriftResult`, so [Reconcile now] shows it; the scheduled sweep writes it and nothing reads it back. `db.GetUnreconciledTargets` exists for that consumer and currently has none.

  This mattered most in exactly the case it was built for: a nightly sweep that has been unable to reach a target for a week looks, on every surface an operator actually opens, like a week with no drift. The natural home is the governance summary beside the drift count — which needs `TargetReconciliation` moved to `internal/models` first, since `models` must not import `db`. Deliberately not done inline with 1.14: a backend field with no rendering is not "saying so" to anybody, and inventing the callout unprompted is a design decision the IA change owns (`basic-advanced-ia`).
  </details>

- **~~The log-integrity finding reaches one surface, and should reach the summary.~~ Closed by deletion, deliberately.** `db.ListCompromisedLogs` no longer exists: it extended `anchorSelect`, whose `WHERE ($1 = '' OR target = $1)` needs an argument, and passed none — so pgx refused every call it was ever given. Having no caller is what hid that. The finding still reaches the operator on the target's own health card, which is where they act on it, and the listing comes back when a surface wants it, with a test. The note below is the state before that.

  <details><summary>Previous entry</summary> `addon_log_anchors` (migration 000033) records where each add-on's mutation-log head was and refuses to move past a truncation or a rewrite. `GET /api/v1/targets/{target}/health` now carries the finding (§19.6), so an operator who opens that target sees it — but `db.ListCompromisedLogs` still has no consumer, so nothing tells them to open it. The governance summary is the home, beside the drift count and the unreconciled-target record above; all three are the same missing callout.

  </details>

- **~~A target whose first manifest read fails stays uncallable for a quarter of an hour.~~ Closed.** `ResolveOperation` makes one on-demand capability read when the manifest is missing — single-flighted per target, rate-limited by its own cooldown, and reached only from the missing-manifest path (`addon-transport-derived-keys` 11.7). Compose ordering was considered and rejected: the add-on is profile-gated, so a `depends_on` fails `docker compose up` on every deployment that runs no NAS. The note below is the state before that.

  <details><summary>Previous entry</summary> `cmd/api/main.go` calls `addons.RefreshAll` once at start-up and then on a `periodic.Runner` tick; a target whose add-on was still starting during that first pass has no accepted manifest, and every operation on it is refused with `ErrNoManifest` (`internal/addons/registry.go:90`) until the next tick. Met on the dev deployment: `docker compose up -d` raced the add-on's start, and `POST /targets/truenas/bindings/{subject}/release` answered `502 addon: no accepted manifest for this target: truenas` while the add-on was up, healthy and answering `/capabilities`. A `docker compose restart backend` cleared it.

  Availability rather than correctness — nothing is written, and the refusal is honest about what the backend knows. What makes it worth fixing is that the failure is invisible from the target's own side: the add-on is running, its health is green, and the operator surface says `callable: false` with a `last_error` from a read minutes old, so the machine an operator inspects is the one that is fine.

  The fix is a bounded refresh-and-retry on `ErrNoManifest` in the dispatch path, single-flighted per target so a burst of refused calls produces ONE capability read rather than one each — the retry must not become the thing that keeps a starting add-on down. Retried once, never in a loop: a target that genuinely has no manifest must still refuse quickly. Complemented by Compose ordering, which narrows the window rather than closing it — `depends_on` cannot promise the add-on has served a manifest by the time the backend asks, and a fix that relies on ordering alone reappears the first time a restart takes longer than expected.
  </details>

## 5. Declined / deliberately kept

Don't re-litigate these without new information.

- **OE15** — `golang-migrate` kept. Its dirty-state guard on mid-migration failure is worth the one dependency; a stdlib runner would drop that recovery.
- **OE21** — `sortedKeys` → `slices.Sorted(maps.Keys(…))` declined. `sortedKeys` returns a non-nil empty slice, `slices.Sorted` returns `nil`, and `zitadelByPair[k]` is allocated before its `RoleKeys` loop — so a Zitadel grant with zero roles would flip `"role_keys": []` to `null` in the reconciliation response. Six lines is not worth a JSON contract change.
