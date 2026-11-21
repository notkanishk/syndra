# Reachability survey: add-on contract 2 (read-only, 21 Nov 2026)

Repo: this repository, branch feat/addon-contract.
All paths are relative to that root. `D:` = `openspec/changes/addon-contract/design.md`, `T:` = `openspec/changes/addon-contract/tasks.md`, `A:` = `addons/AUTHORING.md`, `BE:` = `backend/internal`.
Facts marked (inferred) come from reading code, not from running it.

Vocabulary used below (the code's own):
- **add-on down** = Syndra cannot reach the add-on container (dial failure, breaker open, timeout, pin mismatch).
- **app down** = the add-on answers, but its app (TrueNAS) cannot be read. The add-on reports it (`health.reachable:false`, `stale.reason: unreachable`, `TARGET_UNREACHABLE`/`NO_CURRENT_READ`).
- **kept copy** = a last-known answer served with a date. **no copy** = nothing to serve.

There is NO `specs/` directory under `addon-contract` (only design.md, proposal.md, tasks.md, research*). All contract text is in D, in `addons/contract/v2/*` (schemas, registry, fixtures) and in A.

---

## 1. What the contract already says

### design.md (D)

| Line | Quote | Topic |
|---|---|---|
| D:295-297 | "A revocation (`handlers/revocation.go:99`) names `enabled` as a constant today and so works with the add-on unreachable; by role it must keep working after a backend restart while the add-on is down ... Revocation never depends on reachability." | role map persisted, revocation |
| D:298-307 | "A restart with the add-on down or its manifest refused keeps the member card and rail row, the person page and the home limits ... Write paths that check a value against the schema (mappings, allowances) still need the held manifest." | accepted shape saved (7l) |
| D:471-472 | "`unreached` means never sent; 'sent, outcome unknown' is `indeterminate`" | outcome classes (physical rules) |
| D:482-486 | "An `indeterminate` first attempt is never re-executed by anyone ... `CALL_UNSETTLED`" | at-most-once |
| D:535-545 (table) | per-reply `taken_at`/`current`/`truncated`/`stale`; health: `snapshot_taken_at`, `last_read_at`; stats: no `current` ("the watermark says it") | freshness table |
| D:550-563 | "`current`, defined once: 'this answer comes from a read of the app that began after this request arrived' ... **`current == false` ⇔ `stale` present**: `stale: {reason: cached \| cooldown \| unreachable \| draining \| never_read ...}`" | stale vocabulary |
| D:585-594 | "A read answers with an error only when the add-on has nothing to serve at all: `FRESH_READ_COOLDOWN` (429), `TARGET_UNREACHABLE` (503, `Retry-After`) when no request reached the app, or `NO_CURRENT_READ` (503) when the app was asked and failed (B3) ... the backend reads `TARGET_UNREACHABLE` as `unreached`, never as a lifecycle refusal" | no-copy errors (add-on side) |
| D:604-608 | "A reply made of several reads, answered later from the add-on's copy, carries the copy's own reason, dated by its oldest part" | composite copies |
| D:627-632 | "A kept copy is dated by its age on the add-on's own clock, as an HTTP cache does (RFC 9111 §4.2.3; 7c): age_value = the reply's `Date` less `taken_at` ... a `taken_at` more than two minutes ahead ... is read as now ... one dated the zero time is not a reading" | RFC 9111 age; zero time |
| D:611-616 | "'Check now' ... and the dormant removal ... honour `Retry-After` (RFC 9110 §10.2.3) ... `useRetryOnce`" | cooldown UI |
| D:734-766 | `CALL_IN_PROGRESS` "classed `unreached` by the backend"; only 2xx recorded; replay answered before fingerprint | write idempotency |
| D:823-831 | "'the PC is off' is `health.reachable: false`, target-level. Health keeps the target-level states: `reachable`, `lifecycle`, `version_tested`, `circuit_open`" | health |
| D:876-881 | "one accepted before and unreachable now keeps its member row and card from the saved kind, the card saying what it could not read (7l)" | member page, saved shape |
| D:1003 | kit rule: "`fresh` honoured within your cooldown, the last known answer with `stale` rather than an error" | add-on MUST |
| D:1083-1084 | "the laser PC is off: `laser.job_seconds` stays at the last day it could read" (watermark held back) | stats |
| D:1090-1094 | "A coded `/stats` answer (`NO_CURRENT_READ` ...) neither opens nor clears the breaker; a transport failure, a timeout or an uncoded 5xx counts" | breaker rule, one route |
| D:1116-1117 | "The add-on's buffer ... answers a collection while the app is off: the points are already there, the app's absence is a gap, and nothing waits." | stats buffer |
| D:1253-1256 | "A window the add-on could not observe is a `gap` with a `reason` in the member's words, never a zero. 'The PC was off' and 'nobody used it' are different sentences" | gaps, no false zero |
| D:1313-1316 | "live reads stay ... served by the add-on from its own most recent read of the NAS (its mirror) with `taken_at` and `current` ... never a stored sample as 'now'" | storage reads |
| D:1509-1516 | "a halted pass answers 503 with the inventory as its body, which the reader takes as data (`readInventory` ...), and the card shows its `detail`, not an empty list" ; "A refused read is not an outage. 500 `STORE_UNREADABLE` is `read_refused` ...; a refusal inside the add-on's cooldown is `read_stale`; every other failure is `target_unreachable` (`SubjectsResult.Halt()` is the one mapping)" | halt reasons (backend) |
| D:1519-1523 | "the add-on's kept copy, which can be older than the mirror" | why stats keeps a mirror |
| D:1894-1896 | "`item.checkout` is refused by the add-on whenever `health.lifecycle` is not `active` or the target is not `reachable` (never a queued checkout)" | physical op |
| D:1997-1998 | "A 429 with `Retry-After` is `unreached` as today" | many targets |
| D:2000-2002 | "`/plan` may answer from the mirror (`current: false`) ... `/apply` may not. Every read in the freshness scope may, and says so with `taken_at` and `stale`" | slow targets |
| D:2005-2023 | shape refusals (`CONTRACT_VERSION_MISMATCH`, `UNKNOWN_REQUEST_FIELD`, 404 on a feature route) are `unreached`, never `target_refused`; v1 `error` key read "for classification only" | classification |
| D:2186-2189 | decision 18: physical ops "refused unless the target is reachable and `active`" | decision |
| D:2210-2218 | decision 21: "`current == false` ⇔ `stale` on every reply the §4 table marks" | decision |

What D does not contain: any sentence on what the **backend** does when the add-on itself is down (as opposed to the app), the words "add-on down", the DNS name of the add-on, the dial/handshake timeouts, what a page shows with no copy, or `draining`/`never_read` semantics (they are listed at D:558 and in `addons/contract/v2/schema/common/stale.json:13-17` and nothing emits them).

### tasks.md (T), selected
- T:195-196 `/reach` cache is `cached`, "the failed-read fallback as `unreachable`; `/homes` never caches".
- T:351 TrueNAS mirror answer on a failed live read is `stale.reason: unreachable`, "not `cached`".
- T:1616-1617 `FRESH_READ_COOLDOWN` (429) and `TARGET_UNREACHABLE` (503, classified `unreached`) registered with `Retry-After`.
- T:1647 "With nothing to serve, `/subjects` answers 503 `TARGET_UNREACHABLE` when nothing was sent and `NO_CURRENT_READ` otherwise (B3)."
- T:1652 "a fresh read inside the cooldown of a failed read answers through `fallback` (the copy `unreachable`, `/subjects`' disk mirror, or the error)".
- T:1933 "watermark per metric held back while the NAS is unreachable".
- T:2842, 2853, 2865 `STORE_UNREADABLE` is not an outage; `read_refused`; `SubjectsResult.Halt()` the one mapping; "`NO_CURRENT_READ` and `TARGET_UNREADABLE` included, stays `target_unreachable`".
- T:1147, 1208, 1280 write `TARGET_UNREACHABLE` vs `SETTING_NOT_SAVED` ("the NAS was reachable").

### Contract artefacts and the guide
- `addons/contract/v2/schema/common/stale.json:13-17`: reason enum `cached|cooldown|unreachable|draining|never_read` with one-line meanings. `registry/errors.json:130,185,241,249`: `NO_CURRENT_READ`, `REACH_NEVER_READ`, `TARGET_UNREACHABLE`, `TARGET_UNREADABLE` (all 503; `TARGET_UNREADABLE` also 429/502). `schema/health.json:8,104`: `reachable` required.
- A:351-353 manifest "served while the app is unreachable, and a version is never invented". A:392 manifest MUST be served while the app is unreachable.
- A:560-566 "A slow or failing app yields `stale`, not an error" (answer last value, `current:false`, `stale.reason: unreachable`, that read's `taken_at`). A:567-575 "Nothing to serve is an error, not an empty list" (503 `TARGET_UNREACHABLE` / `NO_CURRENT_READ`; `/values` -> `TARGET_UNREADABLE`).
- A:606-654 freshness rules; A:629 "503 `TARGET_UNREACHABLE`, only when it has nothing at all to serve".
- A:704-710 health: MUST be served while the app is unreachable; "`reachable` MUST say whether a real call to the app just succeeded, not whether the add-on is up"; `circuit_open` is the add-on's own breaker.
- A:1796-1800 retry-class table; A:1808-1837 "Writes while the app is unreachable" (`TARGET_UNREACHABLE` only when certain nothing was sent; otherwise `APPLY_FAILED`/`OPERATION_FAILED` 5xx).
- A:1946-1950 `account.release` "MUST answer while the app is unreachable".
- A:2343-2347 "When the app is down: `/health` says `reachable: false`; reads answer their last value as `stale`; writes answer `TARGET_UNREACHABLE`".
- A "add-on down" (Syndra cannot reach the add-on): not covered anywhere (grep for add-on down/stopped/container: no hit).

---

## 2. How the backend detects it

### Probe and cadence
- Health probe: `addons.ProbeAll` (`BE/addons/health_monitor.go:154`) runs every 30 s (`backend/cmd/api/main.go:535`), serially over targets; each probe is `GET /health` through the pipe (`BE/addons/read.go:721`, `Health` at :712).
- Manifest refresh: `RefreshAll` (`BE/addons/registry.go:623`) every 15 min (`main.go:522`, `addonRefreshInterval` :911), 5 s per target (`BE/addons/deps.go:22`). On failure only `lastErr` is set; the held manifest stays (registry.go:385-395). On-demand re-read when none held, at most one per 30 s (`deps.go:44`, `registry.go:841`).
- Drift sweep every `driftInterval()` (`main.go:472`); stats collection hourly (`main.go:849`).
- Per-call timeout: `callTimeout = 30s` (`deps.go:28`), applied to every request incl. GETs (`pipe.go:106`, `transport.go:395`).

### Health monitor ("offline")
- `HealthMonitor.observe` (`health_monitor.go:90`) counts a probe as answered only if `h.Outcome == succeeded && h.Reachable` (`observeHealth` :145-150). So **add-on down and app down both count as a miss**; the monitor cannot tell them apart.
- `offlineThreshold = 3` (:14): `offline` after 3 consecutive misses (~90 s); before any success `unknown`; one success clears. A probe the breaker refused (`ErrCircuitOpen`) is not counted (:146).
- State is in memory; a restart forgets it (`health_monitor.go:57-60`, ponytail comment); pages then show `unknown`.
- Roster status (`handlers/targets.go:300` `statusOf`): `cannot_connect` > `offline` > `waiting_to_retry` (breaker open) > `checking` (unknown) > `answering`.

### Circuit breaker (backend against add-on)
- `breakerThreshold = 5` consecutive failures, `breakerCooldown = 30 s` (`deps.go:33,37`); half-open without a state (`transport.go:664`, comment :655-662).
- `breaker.record` (`transport.go:675-712`): 2xx and 4xx clear it; `LifecycleRefusal`, `FRESH_READ_COOLDOWN`, and coded `/stats` refusals neither count nor clear; everything else counts (so `unreached` dial failures and `indeterminate` timeouts both count).
- Checked in `Addon.send` before the credential is loaded (`pipe.go:93-96`); returns `ErrCircuitOpen`, outcome `unreached`. `SkipBreaker` exists only for lifecycle calls (`pipe.go:50`).
- Exposed: `CircuitOpen()` (`transport.go:719`) in `GET /targets` (`targets.go:277`) and `statusOf`. Distinct from the add-on's own `health.circuit_open` (`read.go:666-669` comment).

### How each failure is classed (`doAuthenticated`, `transport.go:395-480`; `classifyStatus` :586-606; `dialFailed` `credentials.go:261-286`)
| Failure | Class | Where |
|---|---|---|
| ctx already cancelled | `unreached` | transport.go:396-399 |
| breaker open | `unreached` | pipe.go:93-96 |
| no manifest / feature not declared / no credential / no call record | `unreached`, nothing sent | pipe.go:68-74, `Call` :256-291 |
| `net.OpError{Op:"dial"}` (refused, no route, DNS fail, dial timeout) | `unreached` | credentials.go:262-265 |
| TLS cert/alert/record-header errors, **pinned-key mismatch** (`pinMismatchError`), "TLS handshake timeout" string | `unreached` | credentials.go:266-285, derive.go:87-118 |
| any other transport error: timeout mid-response, reset, truncated | `indeterminate` (pessimistic, for reads too) | transport.go:439-444 |
| 2xx but body cut off / over 1 MiB / undecodable / unknown `outcome` | `indeterminate` | transport.go:455-462, :330-358 |
| `LIFECYCLE_REFUSAL`, shape refusals, `CALL_IN_PROGRESS`, `TARGET_UNREACHABLE`, `TARGET_CHANGED`, 429 | `unreached` | transport.go:586-600 |
| 408, any 5xx (incl. `NO_CURRENT_READ`, `TARGET_UNREADABLE`, `STORE_UNREADABLE`) | `indeterminate` | transport.go:601-602 |
| 3xx | `rejected` (redirects not followed) | transport.go:603-606, credentials.go:201 |
| other 4xx | `rejected` | transport.go:605 |

### Retry policy
- `addonAttempts = 1` (`transport.go:381`): the transport never retries (reads or writes). `extcall` would retry reads on 408/429/5xx if raised (transport.go:374-379). Backoff var exists, unused at 1.
- Reconcile sweep waits out one cooldown then re-reads (`drift/addon.go:228-232`, ceiling 60 s `sweepWaitCeiling`); "Check now" never waits (`ReconcileAddonNow`). UI retries a cooldown press once (`useRetryOnce`, `ui/src/lib/queries/fresh.ts:124`).
- Drain/outbox: `unreached` rows requeue, terminal after `maxRetries` (default 5, `OUTBOX_MAX_RETRIES`) as "out of retries" (`propagation/addon.go:262-277`); `indeterminate` rows stay claimable (`addon.go:284-296`). Pre-flight `addonReachable` (`propagation/deps.go:178-181`) halts before claiming, so an outage does not spend attempts.

### How "unreached"/"indeterminate" become one word for pages
`SubjectsResult.Halt()` (`read.go:475-500`): `Outdated` -> `addon_outdated`; succeeded -> not halted; `STORE_UNREADABLE` -> `read_refused`; cooldown -> `read_stale`; **everything else, whether the add-on answered or not (NO_CURRENT_READ, TARGET_UNREADABLE, timeout, dial failure, breaker open, uncoded 5xx) -> `target_unreachable`** (`db/target_reconciliation.go:18-19`). UI word: `whyNotChecked("target_unreachable")` = "It did not answer when Syndra last tried." (`ui/src/lib/format.ts:313`).

### The add-on's DNS name
- Registration: `ADDON_<T>_BASE_URL` e.g. `https://truenas-addon:8443` (`.env.example:127`, `registry.go:150`, `docker-compose.yml:201`). `requireHTTPS` (`registry.go:306-331`) checks scheme/host/userinfo/query only: **no FQDN requirement, no IP requirement**.
- The client is `&http.Transport{TLSClientConfig: tlsCfg}` (`credentials.go:199`): no `DialContext`, no resolver, no `ServerName`, no `TLSHandshakeTimeout`; bounded only by the 30 s attempt context (inferred: `extcall.Do` `extcall.go:186-189`). Name resolution is therefore whatever the container's resolver does (Docker embedded DNS, then host resolv.conf with its search domains) (inferred).
- The only guard against reaching the wrong host is the pinned ed25519 key: `InsecureSkipVerify:true` + `VerifyPeerCertificate` exact key compare, no hostname check (`derive.go:75-118`). A bare name that resolves (via search domain) to another host fails the handshake with `pinMismatchError`, classed `unreached` (`credentials.go:273-281`). Its message names three causes (secret mismatch, wrong host, add-on not yet restarted) (derive.go:104-115 comment).
- Backend sits on networks `edge, data, addons_truenas`; the add-on only on `addons_truenas` (`docker-compose.yml:259,496`); a stopped container's service name drops out of Docker DNS (inferred), which is the staging fall-through.
- Nothing in D, A, or the kit mentions the name, FQDN, search domains or what the backend does when the name resolves elsewhere.

---

## 3. Every read endpoint that proxies or depends on an add-on

Legend: **A-down** = add-on down (Syndra cannot reach it). **app-down** = add-on up, app unreachable. Zero-valued means present with Go zero (false/0/""/null/"0001-01-01T00:00:00Z"), absent means omitted by the JSON tag.

Backend has NO kept copy of `/subjects`, `/reach`, `/homes`, `/settings`, health, system-health, activity, or any `storage.status` except the two in-memory ones noted (mappable values, per-person storage). With A-down, all other endpoints below have **no copy**. In app-down the add-on serves its own kept copy (TrueNAS `fresher` keeps the last value per key, `addons/truenas/fresh.go:236-253`; `/subjects` also a disk mirror, `server.go:507-534`).

| # | Endpoint (handler) | A-down / no copy | app-down, copy exists | app-down, no copy | False zero / empty / date risk |
|---|---|---|---|---|---|
| 1 | `GET /api/v1/targets/{t}/inventory` (`handlers/inventory.go:33`; `drift.Inventory` `services/drift/addon.go:132`) | 503 with body = `AddonReconcileResult` (`halted:true, reason:target_unreachable`): `bound:0` (not omitempty), `unmanaged:null` (not omitempty), `read_at:"0001-01-01T00:00:00Z"`, `current:false`, `accounts` omitted, `queued:0`. `Inventory` returns before `listBindings` (addon.go:141-148), so `bound` (a DB fact) is never read. 429 `FRESH_READ_COOLDOWN` only on read-again in cooldown (inventory.go:36-46) | 200, data + `current:false`, `read_stale`, `reason:read_stale`, `read_at` = add-on `taken_at` (addon.go:152-164) | same as A-down (503 halted body): add-on 503 `TARGET_UNREACHABLE`/`NO_CURRENT_READ` -> `Halt()` -> `target_unreachable` | **Yes, all three**: 0 count, empty list, 1 Jan 1 date. This is the staging trigger. Same zero-valued shape comes out of `POST /targets/{t}/reconcile` when halted (#4 below) |
| 2 | `GET /targets/{t}/reach` (`targets.go:187`; `addons.ReadReach` `read.go:1166`) | 200 `{read:false, read_at:"0001-...", current:false}` (`ReadAt time.Time json:"read_at,omitempty"`: omitempty does not drop a struct, read.go:1150); `groups`/`everyone` omitted; `retry_after` only for cooldown | 200 `read:true, current:false, stale:{unreachable}, read_at` = add-on `taken_at` | add-on 503 -> `Reach{}` same as A-down | **Date**: zero `read_at` shipped with `read:false`; harmless only while the UI checks `read` first (see section 5) |
| 3 | `GET /targets/{t}/homes` (`target_homes.go:59`; `ReadHomes` `read.go:1108`) | 200 `{read:false, usage_read:false, accounts:[...from DB bindings...]}`; rows carry `subject_id`, `username` from the binding, `home`/`used_bytes`/`quota_bytes` absent (pointers, omitempty); no `taken_at/current/stale` (Observation nil) | 200 `read:true`, `taken_at`, `current:false`, `stale` | add-on 503 -> `Homes{}` | None: usage omitted, not 0. `reason` (`addon_outdated`/unreachable via `Attribution`) shown when answered but unattributable. `/homes` "never caches" at the add-on (T:195) |
| 4 | `POST /targets/{t}/reconcile` (Check now) (`targets.go:415`; `reconcileAddon` `drift/addon.go:212`) | 200 `AddonReconcileResult`, `halted:true`, `reason`, `detail`; `bound:0`, `read_at` zero; audit row `target.checked` | with `!Current`: halts `read_stale` (addon.go:~300) | same as A-down | Zero `bound`/`read_at` in a halted body, but the card branches on `halted` first (`TargetOverview.tsx:575-665`) |
| 5 | `GET /targets/{t}/health` (`targets.go:206`; `addons.Health` `read.go:712`) | 200 `{target, reachable:false, detail:<err text>, health:<monitor>, status}`; **none of** product/version/lifecycle/`snapshot_taken_at`/`checked_at` | 200 merged `TargetHealth` with `reachable:false` (add-on's own), `snapshot_taken_at` = mirror date (`dated`, `read.go:771-789`), `circuit_open` | n/a | `detail` is the raw Go error text for the operator (`errText`, targets.go:331). `Reachable` is a plain bool: a body missing it reads false (safe direction) |
| 6 | `GET /targets` roster (`targets.go:143`; `describeTarget` :265) | No add-on call. 200 from registry + monitor (`health.status`, `last_answered_at` null until a success, `recent`, `reachable_since`), `circuit_open`, `last_error` (manifest), saved `kind`/`entitlement_schema` | same | same | None from zero values. Health is "unknown" after a backend restart |
| 7 | `GET /me/systems` (`target_name.go:84`) | No add-on call, DB + registry only (comment :80-83) | same | same | None |
| 8 | `GET /me/targets` (`my_storage.go:203`, `describeTargetFor` :241) | Entitlement/binding/credential from DB. If manifest never fetched or breaker open: `addonsCallable` false -> returns early with **`health` "" and `reachable` false, no lifecycle** (:323-325). Else one live `Health` read per page load (:329), `health`/`reachable`/`checked_at`/`reachable_since` from the monitor, `storage` and `standing` **absent**, `reads[]` entries `readable:false` (`ReadReply.View` `results.go:473-479`). Per-target, per request, sequential | `storage` with `taken_at/current/stale` (Observation), places | `storage` absent | Absent, not zero. Member page state machine can still claim a wrong state (section 5, item 8). `usage` is from Syndra's own store (`personalUsage` `stats.go:357`) |
| 9 | `GET /users/{id}/targets` (person page; `person_storage.go:99`, `describePersonTarget` :123) | As #8, plus operator extras. **Backend-held copy**: `view.last_known = recalledStorage` only when `storage==nil && health==offline` (:157-163), `lastStorage` in memory (`person_storage.go:75`), dated by the add-on's `taken_at` at store time (`rememberStorage` :81). `home.home/used_bytes/quota_bytes` filled from it, and `view.preview` (reach) only if `reach.read` | n/a | no `last_known` before the first success since restart | Usage in `home` is from the kept copy; its date is carried only by `last_known.read_at`, not by `home` |
| 10 | `GET /targets/{t}/mappable-values` (`mappable_values.go:35`) | No manifest: 503 `TARGET_CAPABILITIES_UNKNOWN`. Else **backend-held copy** `lastMappableValues` (:30) served with `current:false` and `read_at` = backend `time.Now()` at the last success | n/a (`/values` has no `current`) | 409 `VALUES_NEVER_READ` | `read_at` is the backend's clock, not the add-on's (different rule from RFC 9111 age, D:627) |
| 11 | `GET /targets/{t}/settings` (`target_settings.go:16`; `ReadSettings` `read.go:1004`) | 200 `{read:false}` (no `settings`) | n/a (no copy at add-on, `/settings` not in freshness scope) | same | None |
| 12 | `GET /targets/{t}/system-health` (`targets.go:460`; `SystemHealth` `read.go:1458`) | 200 `{target, readable:false, detail}` | n/a (not a freshness read) | same | None; per-source `degraded[]` names unread sources (targets.go:413 region) |
| 13 | `GET /targets/{t}/activity?subject=` (`targets.go:486`) | 200 `{readable:false, reason, detail}` (`cooldown` with `retry_after`; `no_account` for add-on 422; else generic) | `readable:true`, `taken_at/current/stale` | add-on error -> `readable:false` | None; "no activity" vs "could not read" separated by `readable` |
| 14 | `GET /targets/{t}/stats`, `GET /insights/stats`, usage blocks (`stats.go:369,492,357`) | **No add-on call**; Syndra's own tables (`addon_metrics`, points, gaps). Collector (`stats/collect.go:72-92`) logs "not collected, its watermark kept" on failure; nothing is persisted about the failed attempt | same | n/a | `watermark`, `counted_through`, `gaps` carry the dating; a day after `counted_through` is "not counted yet, never a zero" (`ui/src/lib/usage.ts:161`). `insights/stats` gives counters a 0 when "they counted none there" (stats.go:541-543); not checked here against gaps |
| 15 | `GET /targets/{t}/accounts/dormant` (`inventory.go:453`; `services/dormant.go:100`) | 503 `TARGET_UNREADABLE` "could not be read, so nothing can be said" (inventory.go:465); cooldown 429. Read always fresh, never from a copy (dormant.go:95-99) | not served (owner Q1) | same | None: refuses rather than empty |
| 16 | `POST /targets/{t}/entitlements/rehearse` (plan) (`entitlements.go:43`, `services/entitlement_rehearsal.go:128`) | 503 `ADDON_UNREACHABLE` (writeEntitlementError `entitlements.go:211`) | 200 plan with `provisional: !Current`, `state_read_at`, `truncated` (:133-136) | n/a | None; a subject the add-on did not answer is `blocked`, not dropped (:145-158) |
| 17 | `GET /targets/{t}/unconfirmed` (`unconfirmed.go:71`) | `settleByCall` (`:132`) and `settleFromRead` (`:197`) conclude nothing on error and leave the rows (comments :197-203) | n/a | n/a | None: "unreadable now: nothing is concluded, and the rows stay" |
| 18 | `GET /targets/{t}/merge-findings`, `GET /me/changes`, role holders/`HolderFacts`, `GET /propagations` | DB only; no add-on call (grep: no add-on symbol in these handlers) | n/a | n/a | None. Home/Makerspace read `unreconciled_targets` from `services/views.go:1340` |
| 19 | Capabilities/manifest: no proxying endpoint. Held manifest + saved `targets.accepted_shape`/`field_roles` | Held manifest kept after a failed refresh (registry.go:385-395); `acceptedShape` survives a restart (D:298-307) | same | n/a | `GET /targets` carries `manifest_fetched_at`, `last_error` |
| 20 | `GET /me/targets/{t}/operations/{id}` (`member_operations.go:273`) | State from the DB row; `settleByCall` error ignored (`settled, s, _` :306); `view` only when recorded | n/a | n/a | None |
| 21 | Mapping/allowance validation `ResolvesValue` (`read.go:866`, called via `handlers/mappings.go:342`) | Returns `Resolution{}` and **nil error** on dial failure/any non-success (read.go:871-889) = "not checked", not refused | same | same | A write path that fails open on unreachability (callers not traced past `resolveValue`) |

Cross-cutting facts for the section:
- Per page, `/me/targets` and `/users/{id}/targets` cost one `Health` read plus one read per member read op, sequentially (my_storage.go:329, :367). Until the breaker opens (5 failures), each can wait up to 30 s against a blackholed host (inferred).
- `view.Health` is "" when `addonsCallable` is false (no manifest yet or breaker open), which the UI reads as `!view.health` / not offline (`ui/src/components/kinds/storage/MyStorage.tsx:335`).
- `view.Reachable = (monitor status == online)` (my_storage.go:332): "online" needs at least one success since start and fewer than 3 consecutive misses, so one or two misses still say reachable; after a restart it is false until the first success.

---

## 4. Every write path

| Path (file:line) | Gate before sending | Add-on down / unreached | Indeterminate | What operator/member is told |
|---|---|---|---|---|
| Drain, all targets / one target (`handlers/propagations.go:19,34`; `propagation/addon.go:75`) | `addonReachable` = live `Health` with `Reachable` (`propagation/deps.go:178-181`); not claimed if false | `unanswered` (`drain.go:275`): empty pass if nothing queued else `Halted:true, reason:"target_unreachable", waiting:n`; rows stay queued (attempts not spent) | row left claimable, `errIndeterminate` (addon.go:284-296) | 200 with `DrainResult`; row `last_error` "{system} could not be reached" (`unreachedWhy` addon.go:314-319) |
| Background revocation runner (`propagation/revocations.go:90-125`) | same probe per target | `merge(target, unanswered(...))`; other targets continue | as above | queued revocations wait; row-level text as above |
| Fingerprint re-plan before dispatch (`propagation/addon.go:440-470`) | `planOne` must be `Current` and not `Truncated` | row left claimable, "provisional fingerprint cannot gate an apply" (:466) | same | `persistErr`, no operator text beyond last_error |
| Apply entitlements (`entitlements.go:73`) | plan citation; target active (`TARGET_NOT_ACTIVE`) | queues (nothing sent here); 503 `ADDON_UNREACHABLE` only from the plan step | n/a | queued != applied |
| Revoke target access (`handlers/revocation.go:66`) | persisted role map (409 `ACCOUNT_FIELD_UNKNOWN` if none accepted, :88); **never needs the add-on** to record the deny + queue the lock | `password.rotate` half: 202 `partially_revoked`, `partialRevokeCopy` (:219-236) says "could not be reached. Try the rotation again" | "did not confirm ... Check the account before rotating again" | operator told exactly which half is outstanding |
| Adopt (`inventory.go:77`) | `Confirmed` | **default branch lumps unreached and indeterminate**: 202 `{status:"unconfirmed", outcome, detail:"The target did not confirm the adoption. Nothing was recorded here; check the inventory before trying again."}` (:119-130). `writeOperationError` (:229) maps a Go error to 502 via `writeUpstream` | same body | `outcome` field carries the class; sentence does not |
| Release (`inventory.go:271`) | `Confirmed`, `Version` (If-Match) checked against DB only | same lumping (:324-335), backend binding row not dropped | same | same |
| Dormant sweep/purge (`inventory.go:500`) | re-reads fresh; cooldown -> 429; unread -> 503 `TARGET_UNREADABLE` "nothing was removed" (:533) | per-account outcomes | per-account | per-account detail |
| Set lifecycle (`inventory.go:401`; `SetLifecycle` `read.go:791`) | `SkipBreaker`; audited | 502 via `writeUpstream` (:415-420 region) | n/a | generic upstream sentence |
| Member password set (`my_storage.go:507`) | `view.Reachable` (monitor online) else 503 `TARGET_UNREACHABLE` "Nothing was changed" (:550); also `TARGET_PAUSED` (draining/read_only) | `res.Outcome != succeeded`: 202 with `unconfirmedCredentialCopy[outcome]` (distinct unreached/indeterminate texts) | + `retry` state | member told which; one resend allowed |
| Member physical action (`member_operations.go:71`) | same `view.Reachable` gate, 503 `TARGET_UNREACHABLE` "That system is not answering right now. Nothing went out." (:125); preview unanswered -> 503 `PREVIEW_UNANSWERED` (:171) | confirm unreached -> 503 `TARGET_UNREACHABLE` (:204) or `TARGET_CHANGED` | 202 `checking`, page polls | member told "nothing went out" vs "checking" |
| Put setting (`target_settings.go:31`) | none besides registration | 503 `TARGET_UNREACHABLE` "didn't answer, so nothing was changed" (:49) | 503 `TARGET_UNCONFIRMED` (:44) | any other refusal 503 `TARGET_UNREADABLE` (:71) |
| Mapping/allowance create (`mappings.go:342`) | `ResolvesValue` (fails open, row 21 above) + held manifest | still saves (unchecked) | n/a | `errMappingTargetSilent` exists (mappings.go ~355) for "has not published its capabilities" |
| Settle unconfirmed (`unconfirmed.go:250`) | asks add-on `GET /calls/{id}` / `/settings` | nothing concluded; rows stay for a person | n/a | row stays |
| Stats retention, name, project (`stats.go`, `target_name.go`) | DB writes; no add-on | n/a | n/a | n/a |

Add-on side, write answers when the app is unreachable: A:1808-1837 (503 `TARGET_UNREACHABLE` only when certain nothing was sent, else 5xx `APPLY_FAILED`/`OPERATION_FAILED`); TrueNAS `apply.go:659`, `operations.go:171`, `settings.go:115,120`; `ErrNotSent` is the discriminator (`truenas/nas.go:92`).

---

## 5. UI

### The mechanism
- `ReadState{readAt, current, truncated, stale}` (`ui/src/components/ui/ReadFreshness.tsx:36-50`). `observed()` (:62) builds it from an add-on answer. `readStateOfQuery` (:57) builds one for Syndra's own lists (`current: !isError`).
- `classifyRead` (:73-82): `current===false` -> `provisional`; **falsy `readAt` -> `stale`; a truthy `readAt` of any value, including `"0001-01-01T00:00:00Z"`, is aged by `now - readAt`** -> `stale` (age >= 10 min).
- Sentence (`sentence()` :368-373): provisional with an `age` -> "Could not reach the connected system. This is the last state seen, {age}"; with no `age` -> "...and there is no earlier copy to show". `age = formatRelative(readAt)`. `formatRelative` (`lib/format.ts:72-86`) guards only null/NaN, so a year-1 date becomes `formatShortDate` -> "1 Jan 1". **No zero-time guard exists anywhere in `ui/src`** (grep for 0001/zero time: none).
- `STALE` knows only `cached|cooldown|unreachable` (`lib/queries/fresh.ts:34`); `draining`/`never_read` draw "as any copy does" (comment :29-33).
- `cannotReach` = `stale.reason==="unreachable" && readAt` (:284) -> `unreachableLine`. Add-on-down is a different path: an HTTP error or `read:false`, never a `stale.reason`.
- `blocksIrreversibleAction` (:94): anything other than live/ageing blocks adopt (TargetOverview.tsx `tooOldToAdopt = !inventory.data || blocksIrreversibleAction(read)` :1412).

### Consumers (query -> render)
| Consumer (file:line) | Reads | Add-on down / no copy | app-down with copy | Zero / empty / date from a non-read |
|---|---|---|---|---|
| Inventory card "Not created by Syndra" (`components/targets/TargetOverview.tsx:1390-1470`; `useTargetInventory` `lib/queries/useTargets.ts:299`, `readInventory` :286) | #1 | `readInventory` turns a 503 whose body says `halted:true` into data (:286-298). Card: `ReadFreshness` gets `readAt:"0001-..."`, `current:false` -> provisional "last state seen, 1 Jan 1"; `halted` row shows `inventoryNotRead` (`lib/format.ts:341`); `ListStates` suppressed (:1450); **header `count={inventory.data ? leaving.rows.length : undefined}` = 0** (:1429, `unmanaged ?? []` :1414) | copy shown with `read_stale`, "Read again" | `unmanaged:null`->`[]` (0), `read_at` zero date |
| Section strip "Accounts" count (`TargetOverview.tsx:135`) | #1 | `count: inventory.data?.bound` = **0** (data present, halted) | real bound | 0 from a non-read |
| People card "Accounts Syndra manages" (`components/targets/PeopleOnTarget.tsx:58-125`) | #1 + #3 | `accounts ?? []` = [] (:63); header `count={inventory.data?.bound}` = 0 (:101); `ListStates` empty -> **"Nobody yet. No role is mapped ... or the changes ... are still waiting"** (:119); no `ReadFreshness`, no `halted` check; `Card stale` only when `current === false` (:93) | rows shown, card recedes | 0, empty state, false guidance |
| Roster row count (`app/system/targets/page.tsx:178-186,285-293`) | #1 | `bound` = 0 shown as "0" with "last known" when not answering (:292); `unmanaged?.length ?? 0` (:186) | real count + "last known" | 0 labelled "last known" from a non-read |
| System page header read line `TargetRead` (`TargetOverview.tsx:1012-1022`) | #5 | `!health.reachable` -> `ReadFreshness{readAt: snapshot_taken_at, current:false}`; add-on-down body has no `snapshot_taken_at` -> "no earlier copy to show" | `snapshot_taken_at` = mirror date, labelled "{name}'s last known state" for the whole page | the single date is the /subjects mirror's, applied to every card under it |
| Mappings page "What each group opens" (`MappingManagement.tsx:84-140`; `useTargetReach` useTargets.ts:644) | #2 | `reach.data.read===false` -> no `ReadFreshness` unless `stale`/`retry_after` (`reachCopy` :87); `OpensNote` words it; with a cooldown, `readAt: reach.data.read_at` = zero date goes into `ReadFreshness` (:126-131) | `ReadFreshness` + Read again | possible 1 Jan 1 once a cooldown countdown ends (inferred: `aCopy && age`) |
| Add mapping dialog (`AddMappingDialog.tsx:92-124,240`) | #10, #2, #11 | `values.error.code==="VALUES_NEVER_READ"` handled (:123); reach used only if `reach.data?.read` (:240); settings only if `settings.data?.read` (:251) | kept values with `current:false` | none |
| Home folders (`components/kinds/storage/HomeLocation.tsx:39,135`) | #11, #3 | `read:false` handled by `whyNotChecked`/reason | n/a | none |
| Person activity (`components/people/PersonActivity.tsx:182-260`) | #13 | `data.readable ? events : []`; `no_account` vs unreadable sentences (:252-257) | `observed({taken_at,current,stale})` | none |
| System health card (`TargetOverview.tsx:381,413-447`) | #12 | `!data.readable` message; `degraded[]` list | n/a | none |
| Dormant list (`components/targets/DormantAccounts.tsx:50-170`) | #15 | cooldown via `cooldownOf`; 503 -> `ListStates` error "This list could not be read" | not served | none |
| Convergence plan (`components/targets/ConvergeEntitlements.tsx:140-145`) | #16 | 503 -> error; plan: `ReadFreshness{readAt: plan.state_read_at, current: !plan.provisional, truncated}` | provisional strip | none |
| Reconcile "Check now" result (`TargetOverview.tsx:575-683`) | #4 | branches on `result.halted` first (detail, `addon_outdated`, generic); skipped pass `retry_after` | `!result.current && !result.halted` line | none |
| Home "unreconciled targets" (`components/home/Home.tsx:135`, `Makerspace.tsx:91`, `UnexplainedAccess.tsx:356`; `useGovernance.ts:71`) | governance summary | `whyNotChecked(reason)`: "It did not answer when Syndra last tried." for **every** `target_unreachable`, including add-on-answered `NO_CURRENT_READ` | same | none |
| Member Network storage page (`components/kinds/storage/StoragePage.tsx:18-60`; `MyStorage.tsx`; `state.ts`) | #8 | `view.health`: offline (>=3 misses) -> Notice "{label} is offline", usage "unavailable while offline" (`MyStorage.tsx:82,107,528`); **no kept copy for a member** (`last_known` is operator-only); not offline and `storage` absent -> state `unread` (`state.ts:44`). **But `state.ts:40` returns `needs-password` first when `!storage && !view.credential.set`**, i.e. a member with no recorded credential is told to set a password while the system cannot be read (the form is then refused by the backend gate, `my_storage.go:549`) | storage + places with `stale` strip | the 3-miss window and `Health:""` (not callable) behave differently from offline |
| Person page storage card (`PersonStorage.tsx:76-79,215`; `PersonSystems.tsx:157`) | #9 | `view.storage ?? view.last_known?.storage`; "what it said at {clock}, {relative}" | storage | none; usage figures from `last_known` dated by one line |
| Usage cards (`components/usage/*`, `lib/usage.ts`) | #14 | Syndra's own store; `metric.watermark === null` -> "not counted"; `NOT_COUNTED` | same | counter 0 only after `counted_through` rules (`usage.ts:161,181`) |
| Section nav counts (`components/ui/SectionNav.tsx`, `FilterBar.tsx`) | take `count` props | a count is left off only when `undefined`; a 0 from a halted body is shown | | see inventory rows |
| Other `ReadFreshness` users (Bundles, Roles, Policies, Users, Requests, Audit ...) | Syndra/Zitadel reads, `readStateOfQuery` | not add-on reads | | out of scope |

### Where a 0, an empty state or a date can come from a non-read (list)
1. `TargetOverview.tsx:135` Accounts chip: `bound` 0.
2. `TargetOverview.tsx:1429` card count 0 (`unmanaged ?? []`).
3. `PeopleOnTarget.tsx:63,101,119` list `[]`, count 0, "Nobody yet." with guidance about mappings.
4. `app/system/targets/page.tsx:178,292` roster "0 ... last known".
5. `ReadFreshness` `sentence()` age from `inventory.read_at` zero time (`ReadFreshness.tsx:368-373` + `format.ts:72`).
6. `MappingManagement.tsx:126-131` reach zero `read_at` while a cooldown is shown (inferred).
7. `state.ts:40` `needs-password` while `storage` is absent because of an outage.
8. `Reach.read_at`, `lastKnownStorage.read_at`, `TargetInventory.read_at` are Go `time.Time`; `omitempty` on `time.Time` does not omit (read.go:1150).

---

## 6. The add-on side and the conformance kit

### What an add-on must answer (A + contract)
| Situation | Required (source) |
|---|---|
| App down, a read with `current` and a kept copy | last value, `current:false`, `stale.reason: unreachable`, that read's `taken_at` (A:560-566; D:550-563) |
| App down, no copy | 503 `TARGET_UNREACHABLE` (nothing sent) or 503 `NO_CURRENT_READ` (asked and failed); never an empty list (A:567-575). `/values`: `TARGET_UNREADABLE` (A:572-574). `/reach` never read: `REACH_NEVER_READ` (A:2378; `truenas/reach.go:526`). `/stats` before first read: `NO_CURRENT_READ` (`truenas/stats.go:439`) |
| App down, fresh read in cooldown of a failed read | the failure's answer: copy as `unreachable`, or the error; never "read moments ago" (A:609-613) |
| App down, `/health` | MUST answer; `reachable:false` (A:704-707); `circuit_open` separate (A:708-710) |
| App down, manifest | MUST be served; `product_version:""` not invented (A:351-353, A:392) |
| App down, `account.release` | MUST answer (A:1946-1950) |
| App down, write | 503 `TARGET_UNREACHABLE` + `Retry-After` only if certain nothing was sent; else `APPLY_FAILED`/`OPERATION_FAILED` 5xx (A:1808-1837; `ErrNotSent`) |
| App down, stats | watermark held; buffer answers; gap not zero (D:1083, 1116, 1253) |
| App down, physical op | refused unless reachable and `active` (D:1894, 2186) |
| **Add-on itself down** (Syndra cannot reach it) | **No text in D, A or the kit.** Behaviour exists only in backend code (section 2) and its tests |
| `draining`, `never_read` stale reasons | In schema and design vocabulary (D:558, `stale.json`); the TrueNAS add-on emits only `unreachable` and `cached`/`cooldown` (`truenas/server.go:342`, grep: no `never_read`); backend and UI draw them as "any copy" |

TrueNAS evidence: `server.go:486-534` (`handleSubjects`: live -> `STORE_UNREADABLE` -> snapshot with `unreachableSince` -> 503 `TARGET_UNREACHABLE` if `ErrNotSent` else `NO_CURRENT_READ`); `fresh.go:236-253` (`fallback`); `home.go:323` (`/homes` `TARGET_UNREADABLE`, no copy); tests `fresh_test.go:157,303`, `subjects_test.go:440` (capabilities while down), `stats_test.go:320`, `apply_test.go:1591,1615`, `system_health_test.go:217`.

### The conformance kit (`addons/conformance`)
- `freshness.unreachable` is **always skipped**: `conformance/fresh.go:52-53` "a black-box kit cannot take the app offline; the add-on's own test holds it (TestAnUnreachableAppAnswersTheLastValueNotAnError in addons/reference)". Declared in `report.go:71` and `freshChecks` (`fresh.go:22`).
- A:2457-2458: "`Config.Down` (test-only) holds the unreachable rule the kit cannot" (reference add-on `reference.go:125`, `fresh.go:52`, `checkout.go:141,254`, `stats.go:169`).
- No kit check reads `health.reachable` semantics (grep `reachable` in `addons/conformance/*.go` non-test: no hit); the `health` check validates the schema and the log head (`report.go:24`).
- No kit check asserts that a read answers a zero/empty list when the app is down, and none can: the kit never takes the app or the add-on down.
- Add-on-down is outside any possible black-box check (the kit is a client of a running add-on).
- Covered only by add-on-owned unit tests: reference `fresh_test.go:18,153`, `stats_test.go:68`; TrueNAS tests above. Backend side: `addons/outdated_test.go`, `fresh_cooldown_test.go`, `health_monitor_test.go`, `pipe_test.go`, `transport_test.go` (names only; contents not read in this survey).

---

## 7. Inconsistencies, ranked

1. **A halted/unreached read is answered with zero-valued data fields, and the UI reads it as data.** Inventory 503 body ships `bound:0`, `unmanaged:null`, `read_at:"0001-01-01..."` (`drift/addon.go:141-148` returns before bindings are read; fields not omitempty, :56-93); `readInventory` (`useTargets.ts:286-298`) is written to take it as data; five consumers then draw 0 / empty / 1 Jan 1 (section 5 list items 1-5). The same zero `read_at` is shipped by `/reach` (`read.go:1150`). D:1509-1516 states the intent ("not an empty list") but the body shape contradicts it. No UI zero-time guard.
2. **"No copy, cannot reach" has about nine different answers.** inventory 503 body `halted`; reach/homes/settings/system-health/activity 200 `read:false`/`readable:false`; mappable values 409 `VALUES_NEVER_READ`; dormant 503 `TARGET_UNREADABLE`; plan 503 `ADDON_UNREACHABLE`; put-setting 503 `TARGET_UNREACHABLE`; member 503 `TARGET_UNREACHABLE`; person/member view 200 with the field absent; health 200 `reachable:false`. Code names: `TARGET_UNREACHABLE`, `ADDON_UNREACHABLE`, `TARGET_UNREADABLE`, `VALUES_NEVER_READ`, `TARGET_CAPABILITIES_UNKNOWN`, `PREVIEW_UNANSWERED`, `target_unreachable`. D registers `TARGET_UNREACHABLE` for the add-on's 503 only (D:588-594) and is silent on the backend's own.
3. **"Reachable" means four things.** add-on `health.reachable` (app answered a real call, A:704); monitor `online/offline/unknown` (3 misses; add-on-down and app-down both a miss, `health_monitor.go:145-150`; resets on restart); member `view.Reachable` (`my_storage.go:332`, gates member writes; false after restart until first success); drain `addonReachable` (a single live probe, `propagation/deps.go:178`). Gating differs by path: a member is refused after 1 success then silence only once 3 misses accrue, while the drain refuses on the first miss. App-down vs add-on-down is not distinguishable in the roster, `offline`, or the halted reason; the contract says "an operator does not look at the network for what is the add-on's choice" (A:708) for `circuit_open` but nothing separates the two outages.
4. **`target_unreachable` is one word for seven causes**: dial failure, pin mismatch, breaker open, timeout, `NO_CURRENT_READ`, `TARGET_UNREADABLE`, uncoded 5xx (`read.go:475-500`). The only sentence is "It did not answer when Syndra last tried" (`format.ts:313`), untrue when the add-on answered (D:1509-1516 and T:2853 record the merge as intended).
5. **Kept copies live in three places with three dating rules, and the design only describes one.** Add-on copy dated by RFC 9111 age on `taken_at` (D:627-632); backend in-memory `lastStorage` (operator only, offline only, `person_storage.go:75,157-163`) dated by add-on `taken_at` at store time; backend `lastMappableValues` dated by backend `time.Now()` (`mappable_values.go:56`). Counts from the DB (`bound`, roster count) carry no age at all and are labelled "last known" without a date. A member gets no kept copy.
6. **Write outcome words differ by path for the same situation.** Adopt and release merge unreached and indeterminate into one "unconfirmed" 202 sentence (`inventory.go:119-130,324-335`); password set, member actions, revocation and settings each distinguish "nothing went out" from "may have applied" (`my_storage.go` copy map, `member_operations.go:171,204`, `revocation.go:219-236`, `target_settings.go:44,49`). D:471-472 defines the two classes once.
7. **"Revocation never depends on reachability" (D:297) is true of recording, not landing.** The deny and queue need no add-on (`revocation.go:79-90`), but the lock lands only when `addonReachable` passes (`revocations.go:91`), and the contract says nothing about how long a queued revocation may wait, whether the member keeps access meanwhile, or what the operator sees besides `Halted/Waiting` (`drain.go:275-281`). `unreached` rows also go terminal after 5 attempts (`addon.go:262-277`).
8. **The member state machine can claim a wrong state during an outage.** `state.ts:40` yields `needs-password` for `!storage && !credential.set` before `unread` (:44); `Health:""` (not callable) and the pre-3-miss window are not "offline", so offline copy does not show (`MyStorage.tsx:82`). The backend gate then refuses the password with `TARGET_UNREACHABLE`.
9. **The contract's stale vocabulary is wider than anything emitted or drawn.** `draining` and `never_read` (D:558, `stale.json`) have no emitter in the reference or TrueNAS add-on, no UI draw, and overlap three error names for "nothing read yet" (`NO_CURRENT_READ`, `REACH_NEVER_READ`, `never_read`).
10. **The kit cannot test the outage rules it requires.** `freshness.unreachable` is a permanent skip (`fresh.go:52`); no check on `health.reachable`, `TARGET_UNREACHABLE`, no-copy errors, or "no read answers a zero"; add-on-down is untestable by a black box. Only add-on-owned unit tests hold them.
11. **Contract is silent on the add-on's address.** No FQDN rule, no resolver/search-domain statement, no dial or TLS-handshake timeout (the transport sets none, `credentials.go:199`; 30 s is the context), no definition of what a pin mismatch means to an operator beyond the log (`derive.go:104-115`). A stopped container's bare name can resolve elsewhere; it is caught only by the key pin and classed `unreached`.
12. **Different shapes for the same cooldown.** inventory/dormant 429 `FRESH_READ_COOLDOWN` with `Retry-After` (`targets.go:201`); reach/homes/activity 200 with `retry_after` or `readable:false, reason:"cooldown"`.
13. **Fail-open write validation.** `ResolvesValue` returns "not checked" with no error on any unreachability (`read.go:871-889`); D:305 says mapping/allowance writes "still need the held manifest", but not what they do when `/values` cannot be read.
14. **Stats failures are only logged.** `collect.go:86` keeps the watermark and logs; nothing persisted says "last attempt failed", so a UI cannot say "not collected since"; `collected.at` is the last success only (`stats.go:122-127`).
15. **Health reads in page loads are unbounded in cost during an outage.** `/me/targets` and the person page issue one health read plus N read ops per target per request, serial, 30 s each until the breaker opens after 5 failures (inferred); `ProbeAll` is serial too (`health_monitor.go:154-160`).
