# One standard path per operation

**Why.** The owner, 5 Nov 2026: "follow same pipe for same operation… core should remain same… consistent and safe behaviour… be elegant." The architecture audit of that day (job `146ae399`, `architecture-audit.md`) found operations that reach the same outcome by two pipes, where one pipe skips the other's guards. This change collects the waves that put each operation on one pipe. Each wave adds its own section here.

## W1a · Zitadel screen pipes (audit items 1 and 2; families 1c, 1d, 5, 11)

**Problem (verified on `main` at `4d668447`).**
- Zitadel › Projects created, renamed and deleted roles by calling Zitadel's Management API straight from `handlers/discovery.go` (`zitadelAddProjectRole`, `zitadelUpdateProjectRole`, `zitadelDeleteProjectRole`). There was no trace, no audit row, no local `roles` row, and no key check. Delete had no guard, and Zitadel takes a deleted role from every holder. Meanwhile `POST /api/v1/roles` (`services.CreateRole`) made roles another way: local row first, audit after with its error discarded, and a rollback on any error, which left an orphan role in Zitadel after a 503 or a timeout that had actually landed.
- Zitadel › People's grant remove (`handleRemoveZitadelGrant`) queued one revoke of every role on the Zitadel grant. It did not check whether a bundle or a rule still gave a role, which `services.DeleteDirectGrant` does. Assign skipped `checkRoleOffered`, the already-held check and `services.Deliver`.

**What.**
- **Roles: one pipe.** `services/roles.go` `writeRole` is now the only way a project role is written, from either screen. `CreateRole`, `UpdateRole` and `DeleteRole` each describe their write, and `writeRole` runs it the same way every time:
  1. writes the trace (Syndra's `roles` row plus an audit row, in one transaction under the access lock);
  2. calls Zitadel;
  3. on any error, reads the role back;
  4. undoes the record only when Zitadel is known not to be in the state asked for: the read-back shows it absent, or Zitadel answered a final refusal (a 4xx other than 408/429). That undo is audited as `role.not_changed`. If the undo itself fails, the audit says `role.not_restored`, the error is `ErrRoleRecordNotRestored` and the route answers 502 `RECORD_NOT_RESTORED` ("Syndra couldn't put its own record back ... check the role under Roles, or ask whoever runs the Syndra server to look at reference <project>/<role>"), never the "put its record back" sentence. When the answer was lost (a timeout, a broken connection, 408, 429, 5xx) AND the read-back failed, nobody knows: the record stays, the audit says `role.change_unknown`, and the route answers 502 `OUTCOME_UNKNOWN` with a sentence that says Syndra can't tell and how to check, never "Nothing was changed" (coherence audit C6 and W13, 5 Nov 2026). Before this, a failed read-back always undid, so a delete that may already have taken the role from every holder was reported as not having happened.
- **Delete never strips silently.** It is refused (409 `ROLE_IN_USE`) while Syndra gives the role through a person, a bundle or a rule. It is refused (409 `ROLE_HELD`) while anyone holds it in Zitadel, with the holders' names and their count, until the caller sends that exact count back as `?confirm_holders=N`. It is refused (409 `HOLDERS_UNKNOWN`) when there is no complete, current observation to count from.
- **Grants: the person page's pipe.** The three `/api/v1/zitadel/users/{id}/grants` routes call the same pieces as People › Access:
  - Assign runs `checkRoleOffered` and `dbRequestHolding` on every role before any write, then writes one direct grant per role.
  - Remove sends a role with a direct grant through `services.DeleteDirectGrant`. A role Syndra still gives (`UserExpectsRole`) is kept. Only a role nothing in Syndra gives is revoked directly.
  - Replace is remove-then-give over the difference.
  - All three rebuild the person's cache and deliver through `services.Deliver`. Each answers `roles[]`, every role with its change (given, revoked, kept) and its own status after delivery (applied, waiting, pending, failed, kept), and `status` is the least delivered of them (coherence audit W8). The screen reports each role, never "done" for one that waits or that Zitadel didn't take.
- **The legacy routes stay.** They are now thin callers.

**Not in scope.**
- Rule-target delivery for direct grants (audit 1b) belongs to W3a, the planner.
- `handlers/roles.go`'s error mapping for `POST /api/v1/roles` is unchanged. It is not a W1a file, and an `ErrRoleNotChangedInZitadel` there still answers 500.
- Target mappings and role waits that name a deleted role are not refused. They give nobody anything.
Source: owner, 5 Nov 2026, and the architecture audit of the same day (twelve families of operations that reach one outcome by more than one code path). Each wave gives one family a single core and moves its callers onto it. Sections are per wave agent; each owns a disjoint set of files.

## W1d · Retries, backoff and idempotency (audit family 11, ranked item 7)

**Why.** Every external call carried its own retry rules, and two of them were unsafe:

- The outbox requeued a row with no time gate (`db/propagations.go:663` on main), so a short Zitadel blip could spend the whole retry budget (`OUTBOX_MAX_RETRIES=5`, `services/propagation/deps.go:209`) in back-to-back passes. With the client's own three tries per call, one row could take up to 15 unspaced attempts.
- The Zitadel client resent any method on a 503 (`zitadel/client.go:518`), including a create whose first try had landed. Role create then saw a failure, rolled back its own row (`services/roles.go:80-84`) and left an orphan role in Zitadel.
- A 401 after a 429 failed without refreshing the token (`client.go:509`, refresh on the first try only).
- A lost answer on a read failed at once with no retry (`client.go:500`).
- The start-up wait schedule was written twice (`cmd/api/main.go:171`, `:209`).

**What changes.**

- New `backend/internal/extcall`: one per-try timeout, one classification (retryable: 408, 429, 5xx, timeout, broken connection; every other status is final, and what a 409 means stays the caller's), one jittered capped backoff, a read-back hook for an ambiguous write, and a request id on every log line.
- The Zitadel client calls through it. Reads are retried. A write is resent only after a 429. Any other failure of a write is ambiguous: `AddUserGrant` and `AddProjectRole` read back, and every other write returns the failure unretried for the outbox to handle.
- The outbox gains `not_before` (migration 000080). A requeue sets it from `extcall.Backoff`, and the background claims skip the row until then. A row past its budget still lands in `failed`, which the unfinished-revocations surface already shows.
- *Amended 5 Nov 2026 (owner, coherence audit W1, W2, S9).* "Handed to the outbox, which retries with a backoff" was true only for revocations: an add was retried only when somebody pressed Send after its backoff. Now:
  - **One retry model** (`extcall/retry.go`). Each system's rule is a `Policy`: `OutboxRetry` (one send plus `OUTBOX_MAX_RETRIES`, 1m doubling to 30m) and `AddonResend` (a member's storage password: one resend, by the page). Shared: the states (sent, retrying with next try and try n of N, waiting for you, failed), the backoff (`extcall.Backoff`), the audit mark (`retried_by: automatic`, `try`, `of`), and on screen one set of words (`ui/src/lib/retry.ts`) and one part (`RetryStatus`).
  - **The background drain retries what was already sent.** `ClaimDueRetries` takes a row that was sent once (straight away, or by Send), that Zitadel didn't take (503, no answer, 429), and whose `not_before` has passed — gives included, because the operator consented when it went straight away. Never a row that waits for the operator (`role_waits`, by stamp or by the role being marked; `manual`), and never a withdrawal (the revocation claim owns those). Every automatic try is audited (`propagation.retried`, with try n of N and how it went). The drain ticks every minute (`REVOCATION_DRAIN_INTERVAL`, default 1m) so the named next try is kept on time.
  - **Send is the operator asking now.** `ClaimPendingPropagations` no longer skips rows in backoff; only the background claims do.
  - **Truth in the words.** `Delivery.WaitReason()` returns `halted:retrying` when a send was requeued or errored, and `Drain.Retry` carries the first such row's next try and try number, read from the row. Approve, give, remove, bundle retire and rule retire say "Zitadel didn't answer. Syndra sends it again at 14:05 (try 2 of 6).", never "Sent". `GetPendingPropagations` returns `not_before`, `attempts` and `retry`; each retrying row on Pending changes says when, in IST. Home's Pending changes block counts retrying changes and changes Syndra gave up on this week (`/governance/indicators` `retrying`, `gave_up`).
- One start-up wait schedule in `cmd/api/main.go`.

**Not preserving, by intent:** retry timing, and writes are no longer resent inline on a 503.
Owner, 5 Nov 2026. Source: the architecture audit (family 8 and the wave plan). Each wave names the files it owns.

## W1b. Error envelope (audit item 6)

Problem, verified on main: `jsonErrorResponse` hid `err.Error()` only at exactly 500 (`handlers/router.go:685-700`), so raw Zitadel and add-on text reached the screen at 502 and 503 (`discovery.go:136-432`, `drift.go:417`, `propagations.go:21,46`, `targets.go:513`, `inventory.go:161,181,226`, `entitlements.go:211`, `profile.go:26,69`). The UI proxy put prose in `error` (`route.ts` 403, 401, 400, 413, 502), and passed a gateway page through as text. `merge_findings.go:395` sent `err.Error()` in the `error` slot; `claim_shape.go:203` sent the developer string `save again with acknowledge_breaking` as the message.

Done:
- `handlers/errors.go` holds the envelope, `jsonErrorResponse`, `supportReference`, `jsonValidationErrorResponse`. A 500, and a 502/503/504 whose code is in `rawUpstreamCodes`, logs the raw text with a reference and sends a plain sentence plus `details.request_id`.
- The proxy speaks the same shape (`{error: CODE, message, details?}`); a non-JSON gateway error keeps its status and Retry-After and loses its text.
- `ApiError` never shows a bare string body at 5xx and has a fallback sentence per status. `MergeFindings` and `ShapeChooser` show the reference.
- The two hand-built bodies now embed `ErrorResponse` with typed extras.

Not done (files owned by other waves, so left alone): 5xx call sites that pass an authored sentence are not touched on purpose, because some carry safety ("the change may have applied"). The code list is the ceiling. The upgrade is a `writeUpstream(w, status, code, err)` at each raw call site once W1a/W1e land, and then `rawUpstreamCodes` goes away. `bundles.go:65,339`, `access.go:335`, `claimserve/door.go:338` and `addons/truenas/server.go:416` are still hand-built (UNVERIFIED whether they matter to the screen). `outcome.ts` code map (`CODES`, `isCode`) is not built: the envelope now carries the sentence, so the UI has no code-to-copy table to keep.
Owner, 5 Nov 2026. Source: the architecture audit (families 1-11, ranked items, build order W1a-W3). Each wave gives one operation one path, so a rule, a trace or a guard added later lands in one place.

This file holds the waves that have landed. Tasks and evidence are in [tasks.md](tasks.md).

## W1e/W1f: one add-on pipe (audit family 2, item 11)

**Problems, verified on `main` before the fix.**

- A lifecycle change left no trace: `addons/read.go` `SetLifecycle` wrote no `addon_operations` row, and its handler (`handlers/inventory.go` `handleSetTargetLifecycle`) wrote no audit row.
- The breaker check (`allow`, `credentialFor`, `record`) was hand-copied in 12 places: `entitlement.go` Apply, `transport.go` Call, and ten functions in `read.go`. A new endpoint that forgot it never tripped the breaker.
- `PutSetting` ran its own record pipe and sent no idempotency key. It also kept its own copy of the outcome-to-status map, which `addonop.statusFor` had as well.
- The TrueNAS add-on decided twice: `plan.go` `projected` and the `changes` test, against `apply.go` `converge` building `update`.

**Fix.**

- `backend/internal/addons/pipe.go`: `(*Addon).send` is the one request path (breaker, credential, claim, signed call, record). `operate` adds the row and audit line before the call and settles after. `StatusFor` is the one outcome-to-status map.
- A write is refused unless it carries a trace made before the call. Every write carries an `Idempotency-Key` header.
- Lifecycle now begins an `addon_operations` row (`lifecycle.set`, subject = the target, about = the new state), which writes the audit line `addon.<target>.lifecycle.set.dispatched` in the same transaction. It still skips the breaker, on purpose.
- One-path 1.9: the signed HTTP call in `addons/transport.go` is one `extcall.Do` (per-attempt timeout is `Policy.Timeout`, retry/status rules are extcall's). The transport had no retry of its own, so `addonAttempts = 1` keeps send-once as an explicit option; `classifyStatus` and the breaker are untouched.
- `addons/truenas`: `decide(current, desired, home) change` is the one decision. `plan` reads it to say what would happen; `converge` and `createAndConverge` read it to do it.

**Behaviour.** The same outcomes for the same inputs, except two additions: a lifecycle flip leaves a row and an audit line, and writes carry the `Idempotency-Key` header. Wording is untouched: plan keeps `describePlannedChange`, apply keeps `describeChange`.

## W2b · One cache refresh (audit family 10, item 12, backend)

Verified on main before changing: `drain.go` refreshed the cache only after an applied row (`refreshClaims`, was `drain.go:516`); a failed row and a row out of retries reached `markFailed` with no refresh (was `drain.go:326-349`). `services/service_accounts.go` (`SaveServiceAccount`, `UnmarkServiceAccount`) had none, though `facts.IsServiceAccount` is compiled from the mark (`cache/compiler.go`). `handleUpdateBundle` had none, though token facts carry bundle names (`compiler.go` `bundleNames`).

Change: `cache.Refresh(ctx, people, projects...)` in `cache/refresh.go` is the one call a write path makes after its commit. Routed through it: the drain (applied, failed, exhausted via `refreshAfterRow`), service-account mark/unmark, and a bundle rename (every holder from `db.GetBundleHoldersByVersion`, on a detached bounded context). A cache failure is logged and never fails the write; the 12-hour warm stays the backstop. Guard: `cache/refresh_guard_test.go` fails if a listed write path stops calling it, or if new code calls `cache.RefreshUser` directly.

Not moved (outside W2b's file list, still on `RefreshUser`/`RebuildUserCache`): `handlers/webhook.go`, `access.go`, `bulk.go`, `role_members.go`, `claim_delivery.go`, `services/expiry/sweep.go`. They are allow-listed in the guard.

## W2c · One invalidation map (audit family 10, item 12, UI)

Change: `ui/src/lib/queries/invalidation.ts` maps each kind of write (`access`, `bundle`, `bundleRename`, `request`, `drift`, `role`, `serviceAccount`, `expiryNote`, `claimShape`, `signingKey`, `target`, `storageCredential`, `terms`) to the keys it makes stale; every mutation hook calls `invalidateAfter(qc, kind, scope)`. Every kind that can move a rail count includes `["governance"]`, which holds `["governance","indicators"]`. Hooks that were short now reach what the audit named: approving a request (people, pending changes), a drain (people), `useAnswerUnconfirmed` (had no invalidation at all). The signing-key pair is now one `signingKey` kind (`app/zitadel/page.tsx`, `SigningKeyGuide.tsx`). `useUpstream.ts` is W1a's and untouched.

Guards (`__tests__/invalidationMap.test.tsx`): every kind is classified and rail kinds hold `["governance"]`; every `useMutation` in `lib/queries` calls `invalidateAfter` unless it is a listed dry run or lookup; no raw `invalidateQueries` in `lib/queries`.

Left: raw `invalidateQueries` in `components/targets/HomeLimit.tsx`, `components/targets/UnconfirmedChanges.tsx` (writes) and `components/states/{DegradedBanner,index}.tsx` (global refetch), outside W2c's list.

## W1b follow-up: `writeUpstream` (branch `fix/one-name-path`, 5 Nov 2026)

`handlers/errors.go` gains `writeUpstream(w, err, code)` (502) and `writeUpstreamStatus(w, status, err, code)` (503): the raw error goes to the log with a reference, the caller gets the plain sentence and `details.request_id`. Every call site that sent an upstream's `err.Error()` at 502/503 now uses it: `discovery.go` (7), `drift.go`, `entitlements.go`, `inventory.go` (`writeOperationError` and the lifecycle call), `mappings.go`, `propagations.go` (2), `targets.go`, `merge_findings.go`, `profile.go` (2), `reconciliation.go` (the observation's `Error` text). `rawUpstreamCodes` is deleted; `jsonErrorResponse` now sanitises a 500 only, and a 5xx sentence passed to it is one a handler wrote ("the change may have applied" stays).

The guard found three more leaks, now closed: `/zitadel/health` returned `err.Error()` in its 502 body, `/health` returned the Postgres ping error in its 503 body, and the 501 for additive allowances returned `db: additive allowances are not supported yet`. Proof: `TestNoErrorTextReachesAFiveHundredBody` (parses the package; fails on `.Error()` inside any 5xx `jsonErrorResponse`/`jsonResponse` call except a plain 500 through `jsonErrorResponse`), `TestUpstreamFailureNeverShowsRawText`.

Still open: the hand-built bodies in `bundles.go`, `access.go`, `claimserve/door.go` and the add-on writer (one envelope type), and a 5xx whose message is a variable rather than a literal call (the guard reads `.Error()` calls only).

## W2d. One path for a name (branch `fix/one-name-path`, 5 Nov 2026; audit family 9)

Verified on main before changing: the `display_name || humanizeKey(key)` chain was inline in 25+ places (screens, components, `useRoles.ts`, `shownNames.ts`), with four local `useRoleName` hooks; `ServiceAccountPage` used the key alone; the audit CSV put `actor_id` and `target_id` in the who and target columns; `drift-filters.ts` sorted names with a bare `localeCompare`; six backend name sorts used byte order or `strings.ToLower` (`views.go`, `bundle_publish.go`, `target_homes.go`, `claim_delivery.go`, `service_accounts.go`, `role_members.go`), and five more were found by the guard (`insights.go` twice, `roles.go`, and two more in `views.go`).

Done:
- `ui/src/lib/names.ts`: `roleName(displayName, key)` and `humanizeKey` (re-exported from `format.ts`). Every inline chain, the local hooks (`PersonAccess`, `PersonStorage` now use `useShownNames().role`) and `shownNames.ts` go through it. `ServiceAccountPage` shows the role's own name.
- Raw ids out of name slots: the audit CSV (names in `who` and `target`, "Unknown account" when unresolved, ids in `who_id` and `target_id`), the People on a target dialogs, the Unexplained access person picker, the plan review sort key, the create-role clone picker (role name, not key).
- `backend/internal/textsort` (`x/text/collate`, `Loose` + `Numeric`, the same options as `Intl.Collator` base/numeric): `Compare`, `Less`, `Strings`, `ByName`. All name sorts above route through it.
- Guards: `ui/src/lib/__tests__/names-one-path.test.ts` (no inline chain outside `names.ts`; no `localeCompare` on names, time fields excepted), `backend/internal/repoguard/name_sort_test.go` (no `sort.Strings` on names, no name compared with `<`). Tests: `names.test.ts`, `textsort_test.go` (same cases as `sort.test.ts`), the Lab 2/Lab 10 case in `drift-filters.test.ts`, the CSV columns in `export.test.tsx`.

Not done: the person fallback phrase still differs by screen ("this person", "Somebody", "They"); the backend still returns the id as the name in six places (`directory/zitadel.go:430-436` and others) with the UI's `name !== id` check carrying it; `services/views.go` `roleLabel` is unchanged; SQL `ORDER BY b.name` and DB collation are UNVERIFIED; `machineName(id)` in the audit `Actor` still returns a raw id for a non-system actor; `ShapeChooser`/`ShapeCard` show role keys beside names on purpose.

## W2a · One access core (audit families 3 and 4, item 9)

**Problem (verified on `main` at `23bbc59a`).**
- The token compiler marked every rule target as never ending (`cache/compiler.go:239-241`, a `ponytail:` note), so `role_ends` said a rule role outlived the grant it came from.
- `db.MappingHolders` (`db/mappings.go:255`) counted people the directory says have left, whose accounts the entitlement path already disables (`services/entitlement.go:252`).
- Drift seeded rule derivation from observed Zitadel grants as well as records (`services/drift/sweep.go:153`, `handlers/reconciliation.go:186`), while the person view and the webhook seed from records only (`services/views.go:1432`). A target whose source was added by hand in Zitadel was "explained" to the sweep and unexplained on the person page. `ExpectedViaRule` (`services/expected.go:110`) was a second, reverse walk of the chain.
- The working-copy bundle readers ignored includes and held versions: role usage and referenced roles (`db/roles.go:122,192`), the projects' bundle count, bundle impact and the access map (`services/views.go:1017,1080,1384`). A bundle made only of includes gave "nothing", and a role only in an older held version was "unused".
- Bundle and rule holder counts were recorded numbers shown as holding (`handlers/bundles.go:123`, `handlers/rules.go:65`). Six UI sites fell back from observed to the recorded count. Three UI previews walked the rule list one hop (`app/policies/page.tsx:310-335`, `components/people/ManageBundles.tsx:210`, the first matching rule only; `components/bundles/BundlePage.tsx:909-926`).
- "Left" was defined twice (`services/bulk.go:708`, `services/insights_trends.go:98`), disagreeing on alumni and deactivated.

**What.**
- **`internal/access`**, pure: `Ends` (the one fixpoint, each role with when it ends; a rule role ends with its last source), `Closure` (the same fixpoint without dates), `Fired` (the rules whose source is held), `Later`, and `Left` (the one definition of a person who has gone). `models.RuleClosure` is deleted.
- Every closure goes through it: the compiler (`access.Ends`), `collectUserRoles` (now also carries each role's end, so the person page and the role page say what the token says), every cascade, welcome claims, the what-if, drift and reconciliation (`services.RuleDerived`, records only, forward), and the mapping cohort.
- **The mapping cohort** is `services.MappingHolders`: `db.HeldRecords` (one read) + `access.Closure`, minus people `access.Left` positively reports. `db.ListMappingHistory` takes it as a parameter.
- **Bundle readers** read `bundleGivesSQL`: the working copy (own and included), the latest version and every held version, each through `bundle_version_effective_roles`. Bundle impact counts the latest published version, as the bundle list does.
- **Counts.** `services.BundleHolding` and `services.RuleHolding` read the one snapshot (`HolderFacts` shape): Given is recorded, Observed is Zitadel's, nil when never read. The bundle and rule lists carry `observed_holder_count` beside the recorded `holder_count`.
- **UI.** Holder numbers are observed; recorded numbers are labelled ("assigned to", "given by Syndra", "in Syndra's records") or are the note. `holdersLine` never puts a recorded number in the headline. The previews ask the backend's what-if, which gains `rule_added`, `bundle_assigned` and `bundle_unassigned`.

**Not preserving, by intent (user-visible).**
- A rule-derived role's end in the token (`role_ends`), on the person page and on the role page is its source's end, not "no end". A role held by a dated grant and by a bundle now says no end on both pages (it said the grant's date).
- Mapping blast radius counts (merge findings, mapping plans, version diffs, holders endpoint) drop people who have left. The add-on's unbound pass no longer considers them (it already skipped them at resolve).
- Drift raises a rule target whose source is only in Zitadel, alongside that source.
- Role "in use by N bundles" and the delete guard count a bundle that gives the role through an include or a held older version.
- Bundle and rule lists show who Zitadel shows holding, with "not checked yet" before any read. Sorting by people puts "not checked" last.
- "Left" adds "alumni" and "deactivated" to the insights and usage reports' definition, and "departed", "alumni", "deactivated" to the entitlement path's. Zitadel emits none of the three, so nothing changes in production.
