# Review of the 27 Oct audit

Every claim in `AUDIT-2026-10-27.md` was checked against `main` at the #52 merge, and against the recorded decisions (openspec, ADRs, SECURITY.md, DEPLOY.md, the design rules). Two checks were also run on production, read-only.

Verdicts:
- **Confirmed:** true as stated.
- **Partly:** real, but the scope, mechanism or severity is off.
- **Wrong:** not true.
- **Intended:** a recorded decision.
- **Known:** already tracked.

## The short version

- **The audit is mostly right.** Of 11 items in its §1, 7 are confirmed, 3 partly, and 1 is already tracked.
- **Severities are often inflated.** Several "critical" items are high or medium once the real exposure is known.
- **Some claims are wrong:** §1.9's convergence half, `EntitlementSet.Given` stale, and the bundle unpublished-count dedupe.
- **Some are recorded decisions:** demo mode, the read legs bypassing the gate, the cascade lock, the plan cohort, and the person project count.
- **One real problem dominates, §1.1.** The token path serves claims from a cache that every applied change deletes and nothing rebuilds. On production right now the cache holds **0** entries for **19** people with grants, and **0** claim profiles exist. So any app that reads Syndra's custom claims gets none today.
- **The checkers found about 20 more real issues nearby.** These are marked *new* below.

---

## 1. The audit's §1, re-ranked

| # | Claim | Verdict | Real severity | Why |
|---|---|---|---|---|
| 1.1 | Claims cache deleted after drain, never rebuilt | Confirmed | **Critical** | No compile on a miss. The token path falls to `fail_closed`, with empty claims. Drain, expiry sweep, background revoke and webhook remove all invalidate; only a few handlers rebuild. On production the cache is empty right now. |
| 1.2 | Proxy has no PATCH, drops DELETE bodies | Confirmed | **High** | Editing a mapping value returns 405. Deleting a mapping with holders fails with `PLAN_REQUIRED`. The tests mock both, so the suite can't see it. NEXT.md calls the feature "built". |
| 1.6 | `safeReturnPath` open redirect | Confirmed | **High** | `?next=/%09/evil.com` is decoded to a tab and passes the check. `new URL` then lands on evil.com. It works on the real OIDC path. Reproduced under Node. |
| 1.5 | Backend more permissive than the proxy | Confirmed (routes), partly (exposure) | **High** | Eight `withUserAuth` routes need operator, among them `POST /roles` (creates a Zitadel role), `simulate` with any `user_id`, and `/users`. The port is not on the internet: Caddy forwards only `/api/action` and `/api/webhooks`. On the LAN, though, production answers on 8080 and 3000 directly, around Caddy. Any member can get a token by PKCE against the public client. |
| 1.4 | Replayed blocked apply recorded as applied | Confirmed (replay only) | **High** | First answers are handled right. Only a replay is wrong: it is stored without its status, comes back as 200, and is bound. It needs a lost first response, which does happen. |
| 1.8 | Failed grants read shown as permanent access | Confirmed | **Medium** | "No end date" and "Given to you" appear on error, and also while loading. |
| 1.7 | "Pick a date" is a duration from now | Partly | **Medium** | In IST it lands on the right day except for grants made between 00:00 and 05:30. The real defects: a past date silently grants one day, "End of term" has the same flaw, and west-of-UTC display is skewed. Requests already use `until`. |
| 1.11 | 204 or empty body becomes "Backend unreachable" | Confirmed | **Medium** | Hits "Mark as a person" today. The same catch also turns any non-JSON error into a 502. |
| 1.10 | Add-on shutdown shorter than its call timeout | Partly | **Medium** | Worse than stated: one apply makes several calls, up to 90s. Shutdown never waits on `life.Drained()`, which already exists. The damage is bounded by re-drive and fingerprints. |
| 1.3 | Failed webhooks never retried | Confirmed (mechanism) | **Medium** | The comments assume Zitadel redelivers; the dedupe makes that impossible. Grant and drift work is repaired by the 5-minute sweeps. A failed `user_created` onboarding is lost for good; it only shows in the Missed report. |
| 1.9 | Truncated read draws absence and convergence | Partly | **Low** | Absence: true. Findings are written past the 5,000 cap, which needs more than 5,000 NAS users. Convergence: wrong; the add-on refuses it. |
| 1.12 | Mapping rules write Zitadel untraced | Known | Low today | NEXT.md:143. Dormant with 0 rules. *New:* once rules exist, they fire only for grants made outside Syndra, never for Syndra's own. |

## 2. Wrong, intended, or not worth doing

**Wrong:**
- `EntitlementSet.Given` stale: the reader iterates the post-override values. Only the doc comment is stale.
- Bundle unpublished count deduped on detail: both sides are the same sum.
- §1.9 convergence.
- "Target page re-renders every 30s" matters: a few dozen nodes, no cost.

**Intended, and the decision still holds:**
- Demo mode selected by an empty `ZITADEL_DOMAIN` (SECURITY.md). Its guard is weak, though; see §3.5.
- Read legs not gated as operations.
- The cascade's global lock (sized for a makerspace).
- The plan cohort widening after review (documented). A cheap hardening exists.
- The person's project pill showing the recorded count.
- Usage "never report unused from silence".
- The transport record consumed before send.

**Ceilings, not problems, at a few hundred people:**
- The bundle list N+1.
- The requests `withHolding` N+1 (it only covers pending rows).
- The cascade per-user queries.
- The bundle pickers fan-out (cached).
- The add-on `shareUsage` and reach reads.
- The person's two reads (failure is already handled as "unreachable").
- The session check duplicated between middleware and `getSession`.

**Churn I wouldn't do:**
- Splitting every mega-component.
- A big-bang query-key registry.
- A codemod over 2,000 arbitrary utilities. A guard to stop new ones is enough.
- Merging `effectiveClosure` with the resolver; a parity test is cheaper.
- `BundlePage` loading via `useBundle` (it carries a delete hack).

## 3. What I'd improve, grouped by job

### 3.1 Tokens carry the right claims (the one critical)
- Recompile instead of invalidating. Rebuild the user's existing `mapping:<user>:*` keys plus the row's project, and drop a key only if its compile fails.
- Apply this in the drain, the expiry sweep, the background revoke and the webhook remove/deactivate paths.
- *New:* a Zitadel grant event for project A wipes the user's cache for every project but rebuilds only A. Fix it in the same change.
- *New:* the simulator compiles and writes Redis on every GET, so opening a preview silently "fixes" one project and hides the bug. Make the simulator read-only.
- Warm the cache at startup.
- Test: after an applied grant, the next token carries the claims.

### 3.2 The UI proxy keeps its contract
- `PATCH`, plus DELETE bodies.
- 204, empty and non-JSON responses pass through with their status.
- `Cache-Control: private, no-store`, and `Retry-After` forwarded.
- A fetch timeout.
- A 1 MB `Content-Length` cap before reading.
- *New:* reject encoded dot-segments (`..%2F`). They slip past the member allowlist, which matters in demo mode.
- An `Origin` check on mutations. The session cookie is `SameSite=Lax`, so a sibling subdomain can POST.
- A router-to-proxy method parity test, plus PATCH, DELETE-body and 204 cases.

### 3.3 Sign-in can't be bounced off-site
- `safeReturnPath` parses against a base, requires the same origin, and returns path and search only.
- Tests for `\t`, `%09` and `\n`.

### 3.4 The backend is its own authorization boundary
- `withOperatorAuth` on `POST /roles`, `simulate`, `/onboarding/*`, `/webhook/events`, `/bundles/{id}/impact`.
- Also `/users` and `/lookup`, after checking what member screens call.
- A route-gate test that lists every route and its gate.
- *Infra, yours:* allow ports 3000 and 8080 on the app container only from Caddy. Binding to localhost would cut Caddy off.

### 3.5 A broken config fails loudly, not open
- The deploy workflow requires `ZITADEL_DOMAIN` set, and requires 401/403 from the unsigned Action POST. Today `*) reachable` passes a demo-mode 200.
- Startup refuses an empty `ZITADEL_AUDIENCE`, and refuses the `dev_auth_token_secret` API key, when live.
- Remove the dead `PORT` and `NEXT_PUBLIC_API_URL`.
- Server timeouts: `ReadHeaderTimeout` and `IdleTimeout`. Leave `WriteTimeout` off, because drains are long.
- A 5s JWKS client.
- *New:* a negative cache or singleflight on unknown `kid`. Today a forged token makes the backend fetch JWKS from Zitadel on every request.

### 3.6 Work that fails can be retried
- Webhook and onboarding: re-take a row whose status is `failed`, on conflict.
- *New:* the fallback dedupe key has no sequence, so grant → remove → grant the same role is dropped forever.
- Target revocation: on `ALREADY_IN_FORCE` for the same deny, carry on and queue the convergence and the credential rotation. Today a retry is refused and the rotation never happens.

### 3.7 The add-on never records what didn't happen
- Remember only 2xx outcomes. The backend also treats `Effect == "blocked"` as rejected.
- One bolt transaction checks the username and puts the binding. Today two people can bind one NAS account.
- Truncate a torn mutation-log tail on open, and truncate back on a write or fsync error. Today one crash bricks `Verify`.
- `account.release` is available when the target is down.
- The self-account guard refuses purge when identity is unknown, and retries `readSelf`.
- Shutdown waits on `Drained()` past one apply's worst case. Raise the Compose grace with it.
- Purge says when a binding survived.
- Create adopts an empty orphan group, and binds when the read-back fails.
- `activity.get` says when it truncated.
- *New:* `PutSetting` is a target write with no call id, operation record or audit row, and a timeout after it applied is reported as "nothing changed". It needs the same trace as every other mutation.

### 3.8 Data reads never pass a partial answer off as whole
- `rows.Err()` in the 16 loops that lack it. Four feed the closure, where a truncated read can queue a revoke.
- *New:* mapping holders and `ExpectedViaRule` follow one rule hop, while chains are legal. The blast radius undercounts, and chained grants show as drift. Use a recursive CTE.
- A queued revoke counts as held on the Zitadel-side remove and on `replace`.
- Re-read the mapping and holders inside the access lock.
- Truncated read: no absence findings.
- Insights "first seen": drop the grants-index arm.

### 3.9 The screen never claims what it didn't read
- Grants not read: "End date unavailable", never "No end date" or "Given to you".
- The rail's placeholder zeros (`Sidebar` lacks `TouchNav`'s one-liner). Today a failing indicators read shows 0 forever.
- Home folds a failed service-accounts read into "Couldn't check". No 1970 "last checked".
- Project page: an error is not "isn't listed".
- Apps index: no stale dot while loading.
- MyStorage: "couldn't read" separate from "ready", and "not read yet" separate from "not set up".
- Reach card: no "Nothing in flight" from a failed live read.
- Health: zero timestamps omitted.

### 3.10 The same fact, one answer
- Indicator invalidation in four hooks.
- The role page counts observed holders only (it also feeds the "N people hold" sentence).
- Assigning a bundle refreshes bundles; creating a role refreshes projects.
- The name caches are refreshed on rename.
- The member's "already held" includes observed roles.
- Withdrawn access links service accounts to their own page.
- Source pills count people, not reasons.
- The People chip says "within 30 days".
- One `expiryLabel()` replaces five formats ("-2 days", "1 days").
- The drift chime plays once.
- Revocations use the endpoint's `summary` and `escalated`, not counts from a 200-row cap.
- The target index status is live, not cached for 10 minutes.
- Role names from the catalog on a person's page.

### 3.11 Keyboard and contrast
- A skip link.
- The filter panel takes focus and gives it back.
- `Modal` names itself from its header by default (closes 5 unnamed dialogs) and makes the background inert.
- `Segmented` and `FilterPills` as `group` with `aria-pressed`, or roving tabindex.
- A role for `Tabs`.
- The theme script reads the OS preference, so light-OS users get no dark flash.
- Esc in a bulk dialog doesn't drop the selection.
- People search re-syncs from the URL.
- A destination tap in the phone sheet leaves no extra history entry.
- The Zitadel grant dialogs keep their outcome.
- Confirm before deleting an app's claim override.
- Export says what it exports.
- *New:* CSV formula injection, where a name starting with `=` runs in Excel.
- **Contrast (needs Design):** `--label` (2.5–2.9:1) and `--faint` (2.8–3.8:1) fail AA at their 12.5–14px sizes in both themes, and light `--muted` fails on the canvas. That's a token change, so it goes to Claude Design as Q-09, with computed values.

### 3.12 Scale, where it's real
- Governance summary: one holder-count query instead of bundles × users (about 9,000 queries on Home today).
- *New:* the directory cache's 30s rebuild makes one metadata call per user (N+1 to Zitadel, about 10/s while Home is open). Add singleflight, and stop the per-user metadata fan-out.
- Home's onboarding poll: 30–60s, not 5s.
- Logs: `LIMIT` with a cursor, plus a retention prune.
- *New:* each page of `/zitadel/grants` runs a full org observation.
- The add-on read-back filters `user.query` by username: a 100-person cascade is about 400 NAS calls today.

### 3.13 Less code
- Delete the confirmed-dead items: `lib/api.ts`, the six dead types, `getSessionToken`, `LookupRequest`, the test-only format helpers with their tests, `UpsertGrantIndex`/`DeleteGrantIndex`, `GetAuditLogs`, `ActiveTargets`, `LatestObservation`, `RoleHolderConfirmation`, `NeedsWrite`. Also the dead `dehydrate` config, and the smoke scripts' copied env loader.
- **Keep** `ListUnresolvedAddonOperations` and `CountAddonOperations`. *New:* they're the unbuilt read side of tasks 2.15/2.16, which are ticked "built". Crash-mid-call secret operations are never shown to anyone, which is spec drift to fix.
- One `succeeded()`, one `EXTEND_DAYS`, StatCard via `next/link`.
- Lazy-load the four heavy dialogs.
- Split `TargetOverview`, `UnexplainedAccess` and `views.go`, since those have reuse value.
- `useMutationOutcome`, adopted incrementally.

## 4. Proposed order

1. **3.1–3.4**: claims, proxy, redirect, route gates. Small diffs, highest risk removed, one backend and UI deploy. The firewall change is yours.
2. **3.5–3.8**: config guards, retries, the add-on, data reads. The add-on changes ship in their own image.
3. **3.9–3.11**: screen truth, one answer, keyboard. Q-09 contrast goes to Design in parallel.
4. **3.12–3.13**: scale and less code.

Plus the two items already decided: Home's calm panel and blocks swap by Unfold on the view switch, and text-styled buttons tint without the underline.
