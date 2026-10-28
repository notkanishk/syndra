# Syndra feature map — 2026-10-27

Read-only analysis of the product and its subsystems. No source code was
changed.

This document has two layers:

1. **User-exposed feature sets** — the short list of jobs a member or operator
   actually does. These are the things to design, demo, and accept from a
   user's point of view.
2. **Technical feature map** — the longer list of independently testable
   subsystems behind those jobs. These are the things to build, fixture, and
   harden one at a time.

Use the IDs as stable work items, branch names, test names, or task labels.

---

## 1. User-exposed feature sets

A feature set is a **job to be done**, not a screen and not a table. Its
acceptance test is observable behavior: "I can do this, I can see what
happened, and I can recover when it fails."

### Member feature sets

#### U-M1 Sign in and return to where I was

- **Job:** authenticate, land on my own access, stay signed in, sign out.
- **Surfaces:** `/login`, `/auth/zitadel`, `/auth/callback`, `/auth/logout`,
  session cookie, middleware, member navigation.
- **Built from:** `MEM-01`, `PLT-12`.
- **Acceptance:** a member signs in with OIDC, lands on their own page, sees
  their name, can sign out; an expired session returns them to login and then
  back to the page they wanted; a demo deployment never issues a live session.
- **Watch:** open-redirect `next` values, forwarded-host trust, cookie
  `Secure`, and the backend being more permissive than the proxy.

#### U-M2 See what I can use

- **Job:** understand my roles, projects, what is recorded for me, what Zitadel
  actually shows, what is on hold, and what nobody can explain.
- **Surfaces:** `/`, `/me`, `/storage`, person page in member voice.
- **Built from:** `MEM-02`, `MEM-06`, `PLT-02`, `PLT-04`, `GOV-05`.
- **Acceptance:** for a fixture member, every role row says whether it is
  recorded, observed, queued, withheld, or unexplained; an empty read is not
  rendered as "no access"; a failed read is not rendered as "no end date".
- **Watch:** recorded vs observed counts, `observed_role_keys` null vs empty,
  and grants-read failures.

#### U-M3 Ask for access, keep it, or withdraw

- **Job:** request a role I do not have, ask to keep a role that is ending, or
  take my ask back.
- **Surfaces:** request dialog, `/requests`, ask/keep links, withdraw.
- **Built from:** `MEM-03`, `MEM-04`, `OPS-07`, `PLT-05`.
- **Acceptance:** none/until/for-good/on-the-way states are distinct; a
  duplicate ask is refused with a useful sentence; a pending ask can be
  withdrawn; an operator decision is visible to the member.
- **Watch:** the member catalog's held set and the ask dialog's held set are
  computed from different sources.

#### U-M4 Track my requests

- **Job:** see what I asked for, what was decided, and what is still waiting.
- **Surfaces:** `/requests`, person requests tab.
- **Built from:** `MEM-04`, `OPS-07`.
- **Acceptance:** pending, approved, rejected, withdrawn, fulfilled, and
  expired-before-decision all render distinctly; a member only sees their own.
- **Watch:** client-side filtering of an already server-filtered response.

#### U-M5 Set up and use network storage

- **Job:** see whether I have an account, set or rotate my password, open my
  folders, see my quota, and copy the connection instructions.
- **Surfaces:** `/storage`, connection guide, password form, folders, quota.
- **Built from:** `MEM-05`, `TGT-08`, `TGT-09`, `TGT-10`.
- **Acceptance:** no entitlement, entitled-no-account, waiting, needs-password,
  re-enrolment, ready, no-host, paused, and offline are all distinct; a
  password is refused when the target cannot receive it; a failed target read
  is not rendered as "ready".
- **Watch:** missing storage/manifest is rendered as ready or "address not set
  up"; `usable` is ignored.

#### U-M6 See what changed in my access

- **Job:** understand what changed recently, why, and what happened on a
  connected system.
- **Surfaces:** `/me/changes`, person Activity tab.
- **Built from:** `MEM-06`, `OPS-14`, `TGT-07`.
- **Acceptance:** direct, bundle, rule, revoke, expiry, and target events are
  distinguishable; an empty activity report says whether it is "no events" or
  "nobody was watching".
- **Watch:** target activity failure currently hides the whole card.

### Operator feature sets

#### U-O1 Start the day and see what needs me

- **Job:** open the product and know the few things that need a human now.
- **Surfaces:** `/` Home, queue blocks, counts, Makerspace summary.
- **Built from:** `OPS-01`, `OPS-12`, `OPS-17`, `GOV-12`, `GOV-09`.
- **Acceptance:** every number links to the thing it counts; a failed read
  produces "couldn't check", never "nothing needs you"; a queue that empties
  while you are there fades instead of jumping.
- **Watch:** Home's loading/error aggregation omits service accounts and
  indicators; "last checked" can show 1970 before the first read.

#### U-O2 Find a person and understand their access

- **Job:** search for someone, read what they can use, why they have it, and
  what is wrong.
- **Surfaces:** `/users`, `/users/[id]`, access/requests/activity tabs.
- **Built from:** `OPS-02`, `OPS-03`, `MEM-02`, `PLT-02`, `PLT-04`, `GOV-05`.
- **Acceptance:** every role row carries its source, its Zitadel standing, its
  end, and its withheld state; a person with a bundle, rule, direct grant, and
  allowance can be read without opening another screen.
- **Watch:** per-role standing is read from a second endpoint; grants-read
  failure looks like permanent access.

#### U-O3 Give or remove access for one person

- **Job:** give a role, set an end, extend it, or revoke it, and know whether it
  has actually landed.
- **Surfaces:** person page, grant dialog, removal dialog, pending changes.
- **Built from:** `OPS-08`, `OPS-03`, `GOV-09`, `GOV-14`, `GOV-15`.
- **Acceptance:** "queued" and "applied" are distinct; a custom end date is the
  date the operator chose; a revoke says which source it removes; a failed
  delivery stays visible and retryable.
- **Watch:** custom date becomes a duration from now; cache invalidation after
  drain.

#### U-O4 Approve or decline access requests

- **Job:** read the queue, see what approving would change, decide, and handle
  bulk decisions.
- **Surfaces:** `/requests`, request dialog, bulk decisions.
- **Built from:** `OPS-07`, `GOV-14`, `MEM-03`, `GOV-09`.
- **Acceptance:** the queue shows what the person already holds; approve,
  decline, self-approve, and bulk mixed outcomes are explicit; a decided row
  stays long enough to read the result.
- **Watch:** approval and grant are separate writes; failure modes must be
  visible.

#### U-O5 Understand roles and who holds them

- **Job:** see the role catalog, which roles exist, who holds each, and how they
  came to hold it.
- **Surfaces:** `/roles`, `/projects`, `/projects/[id]`, role members page.
- **Built from:** `OPS-04`, `OPS-05`, `PLT-02`, `PLT-04`.
- **Acceptance:** the role index and role page agree on the headline count;
  direct, bundle, automatic, observed-only, and withheld holders are
  distinguishable; a role with no holders is not confused with "not read".
- **Watch:** index uses observed count, detail uses recorded + observed-only;
  project role counts come from two endpoints.

#### U-O6 Fix "my app isn't seeing the roles it expects"

- **Job:** open an app, see the token it receives, edit the claim shape, preview
  the result, and know whether the app will read the key.
- **Surfaces:** `/applications`, `/applications/[id]`, Token format, Preview
  token, key inventory.
- **Built from:** `OPS-06`, `CLM-01`–`CLM-08`, `PLT-03`, `PLT-05`.
- **Acceptance:** the preview and the live token agree for the same user and
  profile; a draft can be previewed before save; the app index shows the
  effective claim (override or default); a conflict names both owners.
- **Watch:** preview computes from ledger while live reads Redis and is gated
  by `user_grants`; multi-project keys change based on the user's other grants.
  Full review: `docs/from another side/ACTIONS-V2-REVIEW-2026-10-27.md`.

#### U-O7 Give access through a bundle

- **Job:** create a bundle, add roles, publish a version, assign it, move
  holders, and retire it.
- **Surfaces:** `/bundles`, `/bundles/[id]`, People bulk give-bundle.
- **Built from:** `GOV-01`, `GOV-02`, `GOV-14`, `GOV-09`.
- **Acceptance:** create requires at least one role; editing reaches nobody;
  publishing is rehearsed; assigning pins the published version; moving
  holders is explicit; retiring says what each holder loses.
- **Watch:** list role count is published, detail count is working copy;
  assigning does not invalidate bundle/holder queries.

#### U-O8 Automate access with a rule

- **Job:** create a rule "if someone holds A, they also get B", preview the
  sentence, and understand the cascade.
- **Surfaces:** `/policies`, rule editor, change history.
- **Built from:** `GOV-03`, `GOV-04`, `GOV-14`, `GOV-09`.
- **Acceptance:** invalid keys, self-edges, cycles, and duplicate rules are
  refused; the preview names the exact roles; the cascade shows who gained or
  lost what.
- **Watch:** holder count follows one hop while the resolver follows chains;
  cascade holds the global lock per user.

#### U-O9 Review access that is ending

- **Job:** see access that ends soon, decide to let it lapse, or extend it.
- **Surfaces:** `/review/expiring-access`, Home block, person page.
- **Built from:** `GOV-06`, `OPS-08`, `GOV-14`.
- **Acceptance:** 14-day and 30-day windows are labelled; acknowledged and
  undecided rows are distinct; extending changes the end and clears a stale
  acknowledgement; bulk extend reports per-person outcomes.
- **Watch:** Home/nav use 14 days, People uses 30; the same label does not say
  which window.

#### U-O10 Put access on hold or lift a hold

- **Job:** withhold one entitlement from one person, give it back, and see when
  it should be reviewed.
- **Surfaces:** `/review/holds`, person page, target revoke.
- **Built from:** `GOV-05`, `TGT-02`, `TGT-03`.
- **Acceptance:** a hold has an author, reason, and bound; it applies on the
  next resolution; it is visible wherever the role appears; lifting converges
  the target; a past date is refused.
- **Watch:** past dates accepted; target revocation can commit the allowance and
  fail to queue convergence.

#### U-O11 Investigate access nobody can explain

- **Job:** see access that exists in Zitadel or Syndra without an explanation,
  find out who made it, and decide what to do.
- **Surfaces:** `/governance/drift`, reconciliation tab, origin.
- **Built from:** `GOV-07`, `PLT-04`, `PLT-08`, `TGT-05`.
- **Acceptance:** only-in-Zitadel, only-in-Syndra, and compared differences are
  distinct; attribute/revoke/mark-external are explicit; a truncated or stale
  read is labelled and cannot conclude absence.
- **Watch:** truncated target reads can still create absence findings.

#### U-O12 See what Syndra is about to send, or has sent

- **Job:** inspect queued changes, send them, retry failures, and understand
  what a cascade did.
- **Surfaces:** `/governance/pending`, `/operations/cascades`, change history.
- **Built from:** `GOV-09`, `GOV-10`, `GOV-15`, `PLT-06`.
- **Acceptance:** pending, in-flight, applied, failed, retried, cancelled, and
  stale are distinct; a cascade groups its rows; a cancelled row says what it
  would have done.
- **Watch:** drain ignores cancellation; cache invalidation after apply;
  add-on replay.

#### U-O13 Recover access that was revoked but still exists

- **Job:** see revocations that did not land, retry them, and know which are
  stuck.
- **Surfaces:** `/governance/unconfirmed-revocations`.
- **Built from:** `GOV-08`, `PLT-06`, `PLT-07`.
- **Acceptance:** queued vs spent vs escalated is explicit; age is real; retry
  has an outcome; the count matches the badge.
- **Watch:** page recomputes from a 200-row cap and uses 30 minutes while the
  backend escalates at 24 hours.

#### U-O14 Onboard a new person

- **Job:** ensure a new member gets the welcome bundle, and find people who
  joined and got nothing.
- **Surfaces:** Home onboarding gaps, `/operations`, welcome bundle setting.
- **Built from:** `GOV-12`, `PLT-05`, `OPS-14`, `OPS-15`.
- **Acceptance:** no welcome bundle is a named state, not a silent failure; a
  transient failure is retryable; a missed person is found from state, not from
  a webhook log.
- **Watch:** failed triggers cannot retry because duplicate key returns success.

#### U-O15 Audit who changed what

- **Job:** answer "who changed this, when, and what did it cause" from one
  place.
- **Surfaces:** `/audit`, `/operations/cascades`, `/operations`.
- **Built from:** `OPS-13`, `OPS-14`, `GOV-10`, `PLT-09`.
- **Acceptance:** every mutation has an actor, target, action, and time; a
  cascade is traceable; unknown actions say "unknown" rather than inventing a
  description.
- **Watch:** audit writes that fail are logged and dropped; export label says
  "what is shown" but exports all filtered rows.

#### U-O16 Manage service accounts

- **Job:** mark an account as a service account, give it an owner and purpose,
  and keep it out of people flows.
- **Surfaces:** `/system/service-accounts`, person page, People filter.
- **Built from:** `OPS-17`, `PLT-01`, `PLT-02`.
- **Acceptance:** machine vs marked human is clear; no-owner is visible; the
  account is excluded from people counts; unmark restores it.
- **Watch:** some screens use `/users`, others `/service-accounts`; unmark
  through the proxy can return 502 on a 204.

#### U-O17 Connect and operate a system

- **Job:** see a connected system, know whether it is reachable, read its
  inventory, adopt or release an account, and pause writes.
- **Surfaces:** `/system/targets`, `/system/targets/[target]`, inventory,
  lifecycle, health.
- **Built from:** `TGT-01`, `TGT-02`, `TGT-03`, `TGT-09`, `TGT-10`.
- **Acceptance:** registered/not-answering/callable/offline/draining/read-only
  are distinct; a truncated read is labelled; adopt and release are explicit;
  purge says what cannot be undone.
- **Watch:** roster status is 10 minutes stale; truncated read; apply replay;
  shutdown drain.

#### U-O18 Map a role to a system

- **Job:** say "this Zitadel role means this group/value on this system", with a
  blast-radius preview.
- **Surfaces:** target mappings, add/edit/delete, versions, rollback, holders.
- **Built from:** `TGT-04`, `GOV-14`, `TGT-09`.
- **Acceptance:** create/edit/delete is rehearsed; the holder count is real;
  a stale plan is refused; a version can be rolled back; a mapping edit shows
  who it reaches.
- **Watch:** proxy lacks PATCH and drops DELETE bodies, so edit/delete are
  currently broken through the UI.

#### U-O19 Reconcile a system

- **Job:** compare what Syndra thinks with what the target holds, and resolve
  the differences.
- **Surfaces:** target reconcile, merge findings, binding conflicts, log anchor.
- **Built from:** `TGT-05`, `TGT-03`, `PLT-08`.
- **Acceptance:** current/stale/truncated/unreachable are distinct; a conflict
  names both claimants; "keep ours" and "take theirs" are explicit; a resolved
  finding stays resolved only when a later read agrees.
- **Watch:** truncated pass persists absence; `TRUENAS_VERIFY_TLS` production
  exception; binding race.

#### U-O20 Manage storage homes and limits

- **Job:** choose where home folders live, set a default or per-person limit,
  and decide what happens to a home after someone leaves.
- **Surfaces:** target settings, home limit, person allowance, storage page.
- **Built from:** `TGT-08`, `MEM-05`, `GOV-05`.
- **Acceptance:** no home root is a named state; a limit is a whole number of
  bytes; a per-person override replaces the role value; a kept home is visible.
- **Watch:** mount instructions infer share name from group name; member
  storage ignores `usable`.

#### U-O21 Configure the makerspace

- **Job:** set term dates, choose the confirmation model, mark which roles wait,
  and set the welcome bundle.
- **Surfaces:** `/settings`, `/changes/roles-that-wait`, bundle welcome toggle.
- **Built from:** `OPS-15`, `GOV-11`, `GOV-12`.
- **Acceptance:** term dates validate; a role that waits is marked everywhere;
  switching models is explicit; the welcome bundle is one choice, not a
  convention.
- **Watch:** "Stop waiting" has no confirmation; global default changes affect
  every future grant.

#### U-O22 Use Zitadel directly when needed

- **Job:** inspect Zitadel health, users, projects, grants, roles, and perform a
  direct write when Syndra is not the right tool.
- **Surfaces:** `/zitadel`, `/zitadel/users`, `/zitadel/projects`,
  `/zitadel/grants`, rotation status.
- **Built from:** `OPS-16`, `PLT-12`, `CLM-05`.
- **Acceptance:** direct writes are visibly different from Syndra-mediated
  writes; the warning says no ledger/outbox/audit trace; rotation status is
  readable; failure is not silent.
- **Watch:** direct writes bypass the single-mutation-authority rule; backend
  auth is weaker than the proxy.

#### U-O23 Explore trends and usage

- **Job:** understand access shape, usage, endings, and "what if" questions.
- **Surfaces:** `/insights`, map, trends, usage, lens, as-of.
- **Built from:** `GOV-13`, `PLT-10`, `PLT-04`.
- **Acceptance:** no data is not zero; a snapshot has a date; usage says whether
  the source is receiving; a what-if is labelled as a simulation.
- **Watch:** "held not used" is empty when the window has no sign-ins; first-seen
  resets on observation.

---

## 2. Technical feature map

Use these as independently testable slices. Each has a fixture boundary and a
known failure mode.

### Member

- **MEM-01 Sign-in and session** — OIDC PKCE, demo login, signed cookie, expiry,
  logout, return path, member/operator gate. Test fake callbacks, stale cookies,
  bad `next`, proxy 401/403.
- **MEM-02 My access** — effective roles, recorded vs observed, project
  grouping, unexplained roles, allowances. Test no grants, direct-only,
  bundle-only, rule-derived, observed-only, held, suspended, multi-project.
- **MEM-03 Ask for access / ask to keep it** — ask, extend, withdraw,
  none/until/for-good/on-the-way. Test duplicate ask, expired ask, self-scope.
- **MEM-04 My requests** — pending/decided history, current/requested end,
  withdraw. Test pending, approved, rejected, withdrawn, fulfilled, expired.
- **MEM-05 Network storage** — account, password, re-enrolment, folders, quota,
  connection guide. Test no entitlement, waiting, needs-password, re-enrol,
  ready, no-host, paused, offline.
- **MEM-06 My changes / activity** — recent access changes, target activity.
  Test direct/bundle/rule/revoke/expiry and empty/failed reads.

### Operator core

- **OPS-01 Home / Today** — queue, blocks, counts, onboarding gaps, service
  accounts, pending changes, drift, unvouched targets. Test each block alone,
  all empty, one read fails, placeholder counts.
- **OPS-02 People index** — search, filters, sort, paging, attention, no-bundle,
  service-account exclusion, bulk. Test empty, dense, multi-project, failed
  read.
- **OPS-03 Person detail** — access by project, sources, observed standing,
  extend, remove, requests, allowances, activity. Test each source, multiple
  sources, queued add/revoke, observed-only, failed grants read.
- **OPS-04 Roles catalog and role members** — global catalog, holder counts,
  role detail, source filters, withheld, roles that wait. Test zero, direct,
  bundle, rule, observed-only, recorded-not-observed.
- **OPS-05 Projects** — list/detail, roles, apps, member counts, role counts.
  Test zero roles, Zitadel-only, Syndra-only, multiple apps, mixed formats.
- **OPS-06 Apps index and app token screen** — app list, claim name/format,
  people, mixed-format warning, token editor, preview. Test no apps, siblings,
  override vs default, no roles.
- **OPS-07 Requests queue and decisions** — approve/decline, bulk, holding,
  self-approval, cancel. Test approve, decline, self-approve, already-held,
  extend, expired, bulk mixed.
- **OPS-08 Direct grants** — give, expiry/date, extend, revoke, queued vs
  applied. Test no end, days, custom date, past date, extend shorter, already
  held, failed drain.
- **OPS-09 Bulk operations** — selection, select-all-matching, rehearsal,
  per-person outcomes, partial failure. Test 0/1/N, mixed effects, stale plan.
- **OPS-10 Search, lookup, and name resolution** — `/catalog`, `/lookup`,
  name components, deleted ids. Test known/deleted/machine/missing, catalog
  failure.
- **OPS-11 Navigation, views, crumbs, mobile shell** — Basic/Advanced,
  operator/member, rail, tab bar, sheet, breadcrumbs, account sheet. Test each
  audience, breakpoint, deep link, back/forward.
- **OPS-12 Loading, empty, error, stale, outcome states** — ReadGate,
  ListStates, ReadFreshness, ActionOutcome, Notice, skeletons, retries. Test
  slow/fast/failure/stale/never-read/empty/partial.
- **OPS-13 Audit log** — filter, search, paging, export, vocabulary, links.
  Test empty, dense, filtered, paged, export, unknown action.
- **OPS-14 Incoming events** — webhook + onboarding stream, filters, details.
  Test webhook only, onboarding only, both, failed, dropped, duplicate.
- **OPS-15 Makerspace settings** — term dates, confirmation model, roles that
  wait, welcome bundle. Test unset, valid, invalid, overlap, one/multiple terms.
- **OPS-16 Zitadel console** — health, projects, users, grants, roles, direct
  writes, rotation status. Test healthy/unhealthy, direct write, rotation
  disabled/warn/stale.
- **OPS-17 Service accounts** — mark/unmark, kind, purpose, owner, list, page.
  Test machine/marked human, no owner, no roles, deleted owner, unmark.

### Access composition and governance

- **GOV-01 Bundles** — create with roles, working copy, includes, rename, retire.
  Test empty, one role, many, includes, included-by, retired, duplicate.
- **GOV-02 Bundle versioning and holders** — publish, versions, pins, move
  holders, stale holders, welcome. Test no/one/many holders, old pins, publish
  with/without migration.
- **GOV-03 Automatic rules** — create/edit/delete/retire, sentence preview,
  cycles, chains, confirmation mode, waits. Test valid, self-edge, cycle,
  duplicate, retired, chain.
- **GOV-04 Mapping cascades and closure** — direct + bundles + rules, closure
  delta, outbox rows. Test one hop, multi-hop, multiple sources, revoke while
  covered/queued.
- **GOV-05 Allowances and holds** — deny, set, lift, review date, history,
  revoke target access. Test deny/set/lift/lapse/review due/already in force.
- **GOV-06 Expiring access review** — 14/30 windows, acknowledge, extend, bulk.
  Test 14 vs 30, acknowledged, renewed, expired, search, bulk.
- **GOV-07 Drift triage** — only-in-Zitadel/Syndra/compared; attribute, revoke,
  mark external, bulk, origin. Test each type, known rule, truncated/stale/no
  observation.
- **GOV-08 Unfinished revocations** — queued/spent/escalated, retry, revoke
  again. Test queued, spent, old, retried, succeeded, empty.
- **GOV-09 Pending changes and outbox** — adds/revokes/replaces, wait reasons,
  drain, cascade groups, retention. Test pending/in-flight/applied/failed/
  retry/halted/manual/role-wait/cancel.
- **GOV-10 Change history and cascades** — one change → cascade → outcomes;
  audit trace. Test no cascade, one, many, partial, cancelled, retired.
- **GOV-11 Roles that wait and confirmation model** — which roles wait, global
  default, switch over, stop waiting. Test role waits, global default, override,
  switch, stop.
- **GOV-12 Onboarding and welcome bundle** — welcome assignment, missed
  onboarding, trigger log, unconfigured, retries. Test no bundle, success,
  failure, duplicate, service account, deactivated.
- **GOV-13 Insights** — trends, map, usage, lens, as-of, what-if, findings. Test
  no data, one/many snapshots, term boundaries, unused roles, no sign-ins.
- **GOV-14 Rehearsal and plan gate** — plan id, fingerprint, cohort, effects,
  stale, apply. Test no/one/many effects, cohort changed, expired, reused, wrong
  surface.
- **GOV-15 Cancellation of queued changes** — preview, acknowledge, cancel,
  outcome, idempotency. Test one/many, already applied, already cancelled,
  partial.

### Claims and tokens

- **CLM-01 Claim profiles** — project default and app override; claim name,
  format, attributes, statics, conflicts. Test default/override/both/duplicate/
  invalid/unknown/empty.
- **CLM-02 Token format editor** — edit, save, discard, delete override, dirty,
  outcome. Test no changes, dirty, success, conflict, delete, auto-open, scope.
- **CLM-03 Token preview / simulator** — pick user, run, claims, owners, raw
  roles, copy. Test no roles, one, many, attributes, statics, sibling, missing
  user, cache miss.
- **CLM-04 Actions v2 claim injection** — trigger payload → projects → Redis
  facts → profiles → `append_claims`. Test canonical payload, single/multi
  project, override, formats, no/duplicate grants, distinct/colliding keys,
  cache miss, malformed, minimal_safe, one degraded.
- **CLM-05 Actions v2 deployment, signing, rotation** — targets.json,
  register/rotate, HMAC, status. Test target type, both executions, no v1,
  valid/invalid/stale signature, multiple `v1=`, rotation status.
- **CLM-06 Claim key inventory and conflicts** — every key, owner, source,
  conflict. Test no keys, one app, siblings, duplicate, override, unused.
- **CLM-07 v1 → v2 migration and parity** — map v1 claims to v2 profiles,
  compare fixed matrix. Test direct/bundle/rule/no roles/multi-project/
  deactivated; compare custom claims only.
- **CLM-08 Claim failure modes** — fail_closed, minimal_safe, cache miss,
  malformed, timeout, DB outage. Test each per project, sibling healthy, logs,
  latency.

### Target plane

- **TGT-01 Target registry and Connected systems index** — registered,
  transport, callable, operations, project, health, needs. Test no add-on,
  registered-not-answering, callable, transport error, offline, circuit open.
- **TGT-02 Target health, reach, lifecycle** — monitor, live reach, breaker,
  active/draining/read_only, key expiry, in-flight. Test online/offline/
  checking/waiting/cannot connect/draining/read-only/stale mirror.
- **TGT-03 Target inventory, unmanaged, adopt/release/purge** — bound,
  unmanaged, adopt, release, purge, conflicts. Test empty, unmanaged, adopt,
  already bound, conflict, release, purge, truncated.
- **TGT-04 Target mappings** — role → field/value, create/edit/delete,
  rehearsal, versions, rollback, holders, reach. Test create/edit/delete, no/
  one/many holders, stale plan, conflict, retired.
- **TGT-05 Target reconciliation and merge findings** — reconcile, merge base,
  findings, conflict/deleted upstream, resolve. Test current/stale/truncated/
  unreachable/conflict/deleted/resolved.
- **TGT-06 Dormant accounts** — list, reason, sweep, remove. Test none, one,
  many, sweep success/failure, already removed.
- **TGT-07 Target activity / SMB audit** — per-person activity, covered/
  uncovered, event types, refusals. Test no events, events, uncovered, audit
  off, cap reached.
- **TGT-08 Home folders, allowances, settings** — home root, folder, quota,
  per-person, kept home. Test no home, home, quota, override, no limit, removed
  role, unreachable.
- **TGT-09 Add-on transport, derived keys, policy** — per-target secret,
  Ed25519, signed calls, manifest, policy, effective operation, breaker. Test
  secret load, signature, manifest, unavailable op, secret params, replay.
- **TGT-10 TrueNAS adapter operations** — create/update/delete, groups, SMB,
  home, password, purge, adopt, release, activity, health, storage. Test create/
  existing/conflict/rename/delete/purge/password/SMB/home/quota/truncated.
- **TGT-11 Mutation log and log anchor** — append-only record, chain, verify,
  recover, tamper, resolve. Test clean/modified/removed/truncated/partial/
  rotation/empty.
- **TGT-12 Add-on contract fixtures and versioning** — contract JSON,
  cross-binary tests, manifest, operations, lifecycle vocabulary. Test apply/
  operation/plan/read/error/version.

### Platform and foundations

- **PLT-01 Directory adapter** — users, projects, apps, roles, names, caches,
  live/demo. Test live/demo/partial/unreachable/cache hit/miss.
- **PLT-02 Access resolver and closure** — direct + bundles + rules +
  allowances → effective roles and reasons. Test one/multiple sources,
  multi-hop, queued revoke, override, lifecycle.
- **PLT-03 Claim compiler and Redis cache** — compile, store, refresh,
  invalidate, warm, TTL. Test compile/hit/miss/malformed/refresh/revoke/expiry/
  rebuild/multi-project.
- **PLT-04 Observation store** — grants index, complete/truncated, basis, first
  seen, latest. Test never read/complete/truncated/failed/stale/per-user/org/
  deleted.
- **PLT-05 Webhook and event pipeline** — translate, dedupe, self-mutation,
  dispatch, observe, sign-in. Test each event, duplicate, failed, retry,
  self-mutation, unknown, incomplete.
- **PLT-06 Outbox, drain, revocations** — enqueue, claim, dispatch, apply,
  retry, confirm, prune, background, cancel. Test pending/in-flight/applied/
  failed/retry/halt/cancel/stale claim.
- **PLT-07 Expiry and allowance scheduler** — direct/allowance expiry, review
  due, cache, cascade. Test not expired/expired/renewed/queued/lapse/review.
- **PLT-08 Drift and reconciliation engine** — expected vs observed, merge base,
  classify, findings, replay, resolve. Test fast-forward/already merged/
  theirs-only/conflict/deleted/truncated/stale.
- **PLT-09 Audit and event vocabulary** — audit rows, action names, cascade ids,
  trace, words. Test every action, unknown, cascade, links, deleted objects.
- **PLT-10 Insights data model** — snapshots, trends, usage, lens, as-of,
  what-if, findings. Test no data/one/many/term boundaries/backfill/no sign-ins.
- **PLT-11 API contract and error model** — strict decode, codes, statuses,
  limits, pagination, unknown fields. Test malformed/unknown/trailing/oversized/
  wrong method/invalid enum.
- **PLT-12 Auth, session, proxy, CSRF** — JWT, API key, demo/live, operator,
  member allowlist, cookie, headers, CSRF. Test no session/expired/wrong role/
  forged/stale demo/host/origin/204/PATCH/DELETE.
- **PLT-13 Data model and migrations** — schema, constraints, indexes,
  migrations, live DB, residue. Test fresh/upgrade/down-up/constraints/indexes/
  reset/residue.
- **PLT-14 Demo mode, seed, reset** — demo catalog, seed, reset, residue,
  production guard. Test demo/live/seed/reset/residue/unset domain.
- **PLT-15 Deployment and runtime** — compose, install/update, secrets, health,
  shutdown, profiles, proxy, ports. Test fresh/restart/stop/missing env/rotate/
  backup.
- **PLT-16 Observability and logging** — data-plane, cache, drain, health,
  rotation. Test hit/miss/degraded/drain/webhook/unreachable/audit failure.
- **PLT-17 Test harness and quality guards** — unit, contract, live DB, smoke,
  repoguard, golden fixtures. Test each feature has a fixture; failures are
  meaningful; guards catch drift.
- **PLT-18 Design system, motion, accessibility** — tokens, primitives, motion,
  focus, contrast, responsive, reduced motion. Test both themes, keyboard,
  screen reader, phone/tablet/desktop, dense/empty.
- **PLT-19 Performance and scale** — query counts, payloads, polling,
  memoization, pagination, cache. Test 200 users, 30 bundles, 1000 grants, 500
  drift rows, 10 targets.
- **PLT-20 Security hardening** — authorization, CSRF, headers, rate limits,
  secrets, TLS, audit. Test direct backend calls, member tokens, replay,
  signature, host, rate limit.

---

## 3. Cross-cutting test dimensions

Every feature set and technical feature should be exercised across:

- **Mode:** demo vs live.
- **Audience:** member vs operator.
- **View:** Basic vs Advanced.
- **Data:** empty, one, many, dense, multi-project, deleted, service account.
- **Read state:** loading, success, empty, stale, never read, partial/truncated,
  failed.
- **Write state:** pending, queued, applied, confirmed, failed, retried,
  cancelled, stale plan.
- **Device:** phone, tablet, desktop, ultra-wide.
- **Input:** keyboard only, screen reader, reduced motion, light theme.
- **Failure injection:** Redis down, DB down, Zitadel down, add-on down, NAS
  slow, malformed cache, expired signature.
- **Concurrency:** two operators, two tabs, duplicate webhook, duplicate apply,
  cancel while in flight.

---

## 4. Suggested stages

**Stage 0 — Foundations**
`PLT-11`, `PLT-12`, `PLT-13`, `PLT-15`, `PLT-17`, `PLT-18`, `PLT-20`.

**Stage 1 — Core operator and member**
`MEM-01`–`MEM-06`, `OPS-01`–`OPS-12`, `PLT-01`, `PLT-02`.

**Stage 2 — Access composition**
`GOV-01`–`GOV-04`, `GOV-09`–`GOV-12`, `GOV-14`, `GOV-15`.

**Stage 3 — Claims and tokens**
`CLM-01`–`CLM-08`, `PLT-03`.

**Stage 4 — Review and governance**
`GOV-05`–`GOV-08`, `GOV-13`, `PLT-04`, `PLT-08`, `PLT-09`.

**Stage 5 — Target plane**
`TGT-01`–`TGT-12`, `PLT-05`–`PLT-07`.

**Stage 6 — Polish and scale**
`OPS-13`–`OPS-17`, `PLT-10`, `PLT-16`, `PLT-19`.

---

## 5. First 12 slices

1. **PLT-03 Cache lifecycle** — rebuild after drain; otherwise Actions v2 is
   untrustworthy.
2. **CLM-04 Actions v2 exact-envelope tests** — canonical payload, seeded
   facts, exact `append_claims`.
3. **CLM-03 Preview/live parity** — same facts/profiles, compare simulator to
   handler.
4. **CLM-07 v1 → v2 parity** — fixed user/app matrix; compare only custom
   claims.
5. **GOV-09 Pending changes/outbox** — states, retries, cancellation, cache
   invalidation.
6. **GOV-07 Drift** — each drift type, truncated/stale observation, resolution.
7. **GOV-01/02 Bundles** — create → add role → publish → assign → move holders
   → retire.
8. **TGT-01/02 Target health** — roster vs live health, offline/checking/
   draining.
9. **TGT-04 Target mappings** — rehearsal, plan gate, edit/delete, stale plan.
10. **MEM-02/03 Member access and requests** — observed/recorded, ask/keep/
    withdraw.
11. **OPS-02/03 People and person detail** — source kinds, failed grants read,
    bulk selection.
12. **PLT-05/06 Webhooks and drain** — retry, dedupe, self-mutation, apply/
    confirm.
