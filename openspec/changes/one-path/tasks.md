# One path — tasks

## W1a · Zitadel screen pipes (branch `fix/zitadel-screen-pipes`)

- [x] 1.1 `services/roles.go` `writeRole`: trace (local row + audit, one tx, access lock) before the Zitadel call
- [x] 1.2 Read back on any error before undoing; undo audited as `role.not_changed`
- [x] 1.3 `CreateRole` on the pipe (key validated first; display name falls back to the key)
- [x] 1.4 `UpdateRole` keeps the local row in step (when there is one); `db.UpdateRole`
- [x] 1.5 `DeleteRole` refuses while Syndra gives the role, while holders are unknown, and until the observed-holder count is confirmed
- [x] 1.6 `handlers/discovery.go` role routes are thin callers; `zitadelAdd/Update/DeleteProjectRole` removed from `handlers/deps.go`
- [x] 1.7 Zitadel › People assign: `checkRoleOffered` + already-held on every role first, one direct grant per role, `services.Deliver`
- [x] 1.8 Zitadel › People remove: `services.DeleteDirectGrant` for a direct grant; a role a bundle or a rule gives is kept; only a role nothing gives is revoked directly
- [x] 1.9 Zitadel › People replace: remove-then-give over the difference; a refused give changes nothing
- [x] 1.10 UI: delete dialog shows who loses the role and confirms with that count; copy on both screens and the hub says they are recorded; `useUpstream` invalidates `users`, `roles`, `governance`, `propagations`
- [x] 1.11 Audit vocabulary: `role.updated`, `role.deleted`, `role.not_changed`
- [x] 1.12 Tests:
  - `services/roles_pipe_test.go`: trace before each write, no trace means no call, a 503 or timeout that landed is kept, a refusal is undone and audited, delete refusals;
  - `handlers/discovery_grants_test.go`: a role a bundle gives is kept, an unoffered or already-held role is refused, replace;
  - `handlers/discovery_roles_test.go`;
  - UI: `deleteRole.test.tsx`, `revokeGrant.test.tsx`.
- [x] 1.13 `handlers/roles.go` (`POST /api/v1/roles`) answers through `writeRoleWriteError`, so both screens answer refusals alike: a bad key is a 400 (was a 500), a lost reply is OUTCOME_UNKNOWN (was "Something went wrong"). `TestCreateRole_AnswersAsTheProjectsScreenDoes` (release/coherence)
- [ ] 1.14 Deploy, then delete a role nobody holds on the dev box and confirm both audit rows and the read-back (operator-gated) #ready-for-human

## W1d · Retries (branch `fix/one-retry-core`)

- [x] 1.1 `internal/extcall`: Policy, Backoff (jittered, capped), RetryableStatus, NoAnswer, Do with per-try timeout, read-back and request-id logging
- [x] 1.2 Zitadel client on `extcall.Do`; reads retried, writes resent only after 429; 401 refresh once per call on any try
- [x] 1.3 Read-backs on `AddUserGrant` and `AddProjectRole`
- [x] 1.4 `classifyZitadelError` uses `extcall.RetryableStatus` (truth table unchanged)
- [x] 1.5 Migration 000080 `propagation_outbox.not_before`; `RequeuePropagation` sets it; both batch claims respect it
- [x] 1.6 One start-up wait schedule in `cmd/api/main.go`
- [x] 1.7 Tests: backoff spacing and cap, retryable vs terminal, read-back on ambiguous write, outbox respects `not_before`, max attempts to failed (live), down migration (text and live, in a rolled-back transaction)
- [x] 1.8 Show "next try at" (`not_before`) on the pending worklist: `GetPendingPropagations` returns `not_before`, `attempts` and `retry`; `RetryStatus` says "Zitadel didn't answer. Syndra sends it again at 14:05 (try 2 of 6)." in IST (branch `fix/coherence-delivery`)
- [x] 1.8a One retry model (`extcall/retry.go`: `OutboxRetry`, `AddonResend`, `Policy.Retrying`, `AuditMark`), parity test across both (`extcall/retry_test.go`); `ui/src/lib/retry.ts` + `components/ui/RetryStatus.tsx` the one set of words and the one part
- [x] 1.8b Background drain retries any change already sent that Zitadel didn't take, past its backoff, gives included (`db.ClaimDueRetries`, live test `retry_claims_live_test.go`); never `role_waits`/`manual`; every automatic try audited `propagation.retried`; tick 1m
- [x] 1.8c Send takes rows in backoff (`ClaimPendingPropagations` ungated); `Delivery.WaitReason()` = `halted:retrying` on Requeued/Errored, with `Drain.Retry`; approve, give, remove, bundle retire carry `retry`
- [x] 1.8d Vocal: Home's Pending changes block says retrying and gave-up counts (`/governance/indicators` `retrying`, `gave_up`); member storage page says "Not confirmed yet. Syndra is trying again at 14:05." and waits for the server-named next try
- [x] 1.8e Rule retire (`app/policies/page.tsx` `DeleteRuleConfirm`) uses `retireOutcome` from `lib/retry.ts` with the cascade's `drain.retry`; "applied without review" gone (release/coherence; `deleteRule.test.tsx`)
- [x] 1.8f The rail carries a retry pill (`components/shell/RetryPill.tsx`, `retryBadge`): amber "↻ n" while retrying, danger "! n" once given up, on Home (Basic and Advanced) and on Pending changes (Advanced); a collapsed Changes group lights on a phone (`touch-nav.ts`). `RetryPill.test.tsx` (release/coherence)
- [x] 1.8h A straight-away send that halted before it went (Zitadel unreachable, a drain error) is retried on its own like a 503 after a send (owner-approved, 5 Nov): `services.Deliver` → `db.ScheduleRetry` counts try 1, sets the next try and `halted:retrying`, never on a row that waits; the outcome carries `retry`. If the schedule can't be written, the halt reason stays and the rows wait for Send. Tests: `TestDeliver_AHaltedSendIsRetriedOnItsOwn`, `TestDeliver_AnUnscheduledHaltSaysWhy`, live `TestScheduleRetry_AHaltedSendGetsANextTry` (release/coherence)
- [x] 1.8i Retries in Basic too: Home's *Sent again on its own* block (both views) and the rail pill; Home's Pending changes counts only what waits for a send (retrying rows are not "waiting for you"). A give-up stays until **Mark seen** (`POST /propagations/gave-up/seen`, `db.MarkGaveUpSeen`, per person since 6 Nov: table `gave_up_seen`, migration 000082, the old shared config key is dropped; before a person's first mark, the last 7 days count for them). Tests: `HomeRetries.test.tsx` (both views), `TestHandleMarkGaveUpSeen`, live `TestCountRetries_AGiveUpStaysUntilSeenByEachPerson` (two operators), `TestGaveUpSeenMigration_CoversTheQueries` (release/coherence)
- [x] 1.8j Next step or none on every outcome and retry state (owner, 5 Nov): `nextStepOf` + `next` on `ActionOutcome`, `retryNextStep`, drawn by `ActionOutcome` (all placements) and `RetryStatus`; guard `next-step.test.tsx`; copy guide §5 (release/coherence)
- [x] 1.8k `useWaitsBeforeLine(op)` is exact per act: a give or a removal while Zitadel is down goes on its own; only a role marked to wait waits for Send (was "will wait for you to send" for everything). `WaitsBeforeLine.test.tsx` (release/coherence)
- [x] 1.8l `Notice` requires `next` or `nothingToDo` through its types (`NoticeNextStep`); every caller states one; guard `notice-next-step.test.tsx` (types, runtime, copy rules) (release/coherence, 6 Nov)
- [x] 1.8m A refused role write whose undo failed says so: `services.ErrRoleRecordNotRestored` (`RoleRecordNotRestoredError{Reference}`), audit `role.not_restored`, 502 `RECORD_NOT_RESTORED` with a next step; never "Syndra put its record back". Tests: `TestCreateRole_RefusedAndUndoFails_SaysTheRecordIsNotRestored`, `TestRoleRecordNotRestored_IsHonestAndEndsOnANextStep` (release/coherence, 6 Nov)
- [x] 1.8n Re-audit (6 Nov, `fix/reaudit`): every next step says what really happens.
  - Give-ups: `db.CountRetries` counts every `failed` row per target (a retry Zitadel then refused no longer vanishes); `db.ListGaveUp` feeds Pending changes' **Syndra stopped sending** (filter `?gave_up=1`), which Home's card links to; a given-up revocation goes to Drift (its ledger row is gone), never "revoke again on their page". Indicators carry `retrying_by`, `gave_up_by`, `auto_retry`.
  - `REVOCATION_DRAIN_ENABLED=false`: `propagation.SetAutoRetry`; `RowRetry` answers `waiting` ("Press Send on Pending changes to try again"), and Home, Pending changes and the Zitadel offline Notice say so.
  - Retries that go on their own say so with their next try: bulk rows (`BulkOutcome.wait_reason/retry`, plain-language `queuedDetail`), bulk approve (now queued when it didn't land), Zitadel › Users (`retry`), drift revoke (`propagation`, `retry`), extend (`outcomeFromExtend`). A role marked to wait after its change was sent shows the wait, not a next try (R15).
  - Approve and give split their own row from the rules' rows (`Delivery.ReasonFor`, `WaitingBesides`; `rule_waiting`); bulk approve plan names rule gifts; a person's Requests tab shows the rule preview.
  - Exact failure classes on Zitadel › Users: `GRANT_NOT_READ` (nothing written), `PARTLY_RECORDED` (roles named in `details.recorded`); `OUTCOME_UNKNOWN` is partial ("Check Zitadel › Projects"). The revoke dialog's count comes from `GET …/grants/{grantId}/removal`, the removal's own walk with nothing written.
  - Shared defaults: `waitsForSend` (a queued outcome with another named step draws no Send link), a refusal says "Nothing was changed." once, `RetryStateName` is `retrying | waiting` only, dead `max_retries_exceeded` words gone, a lost reply says "Check this page after a reload".
  - Guards: `next-step-places.test.ts` (every "A › B" and every shared next step names a real place), `reaudit-outcomes.test.tsx`, `reaudit_test.go`, `TestCountRetries_ARefusedRetryIsGivenUpUnderItsTarget` (live DB).
- [x] 1.8o Revocation copy when `REVOCATION_DRAIN_ENABLED=false`: `REVOKE_SENDS_ITSELF`, `retireOutcome` (halted without a retry) and bulk `queuedNote` still say revocations send on their own; read `auto_retry` there too (done: `auto` threaded from `useAutoRetry`; tests `lib/__tests__/drain-off-copy.test.tsx`; PersonStorage offline Notice now `offlineNext`, traced to propagation/addon.go:81 and revocations.go:92) #ready-for-agent
- [x] 1.8p Final verification (6 Nov, `fix/final-n`), N1 to N7 and the MyStorage pause line.
  - N1, a change Zitadel refused outright: the drain names each refused row with its plain cause (`DrainResult.Refused`, `propagation.RefusalCause`: `not_allowed` 401/403, `not_found` 404/410, `invalid` 400/412/422, `other`); `services.Deliver` carries it as `refused:<cause>` (`db.WaitReasonRefused`), first in `WaitReason()` and per row in `ReasonFor`. Every result says "Zitadel refused this", the cause and the true step (`lib/retry.ts refusedOutcome`/`refusedNext`, kind `failed` with `recorded`, so never "Nothing was changed." and the act's button is spent): give, approve (Home, Requests, a person's Requests tab), removal (direct and bundle), drift revoke, retire (bundle and rule), bulk rows (`failed` with `refusedDetail`), extend, Zitadel › Users (per role `reason`), ManageBundles. Tests: `delivery_refused_test.go`, `propagation/refused_test.go`, `handlers/final_n_test.go`, `ui/src/lib/__tests__/final-verification.test.tsx`, GrantDirectAccess, ManageBundlesPublished, grantOutcome.
  - N2: the rail pill says "Syndra stopped sending" and follows `auto_retry`; "↻" only when the drain is on.
  - N3: `useAutoRetry` is null until the indicators answer; every drain-dependent sentence has a neutral branch ending on `DRAIN_UNKNOWN_NEXT`; `retryingOutcome` with no retry is neutral too.
  - N4: `BulkOutcome.rule_waiting`; bulk approve uses the approval's own `ReasonFor` and stays applied when only a rule's row waits; bulk adopt stays applied; both end on Send.
  - N5: `ApproveButton` (`components/requests/Holding.tsx`) holds Approve while the rule preview loads or fails, with the reason; `RulesLine` says when it can't read the rules.
  - N6: `PARTLY_RECORDED` says a recorded revocation sends on its own with the drain on and waits for Send with it off (`partlyRecordedMessage`).
  - N7: em dashes out of `services/bulk.go` plan text; copy guide drift revoke row current.
  - One consent rule (6 Nov): Approve, Give, Zitadel › Users assign and Drift adopt take their hold from one hook, `useRuleConsent` / `RuleConsentFailed` (`components/requests/RuleConsent.tsx`): held while the rule preview loads, held when it fails, with the reason and Try again. Bulk give and approve are held by `RehearsalDialog` (no apply without a plan). (8 Nov: Give and Approve now take it from the planner, `usePlanConsent`, architecture-deepening 1.2; Zitadel › Users and Drift adopt keep `useRuleConsent`.)
  - Stopped sending says why (6 Nov): a refused row keeps `wait_reason = refused:<cause>` (`db.MarkPropagationRefused`, migration 000083 widens the 000056 check). Pending changes names the cause per row; Home's card counts per cause; add-on failures and older rows keep the general sentence.
  - MyStorage: a pause never ends by itself (an operator sets the add-on back to active) and nothing resumes on its end alone, so "by itself" and "a few minutes" are gone (`handlers/my_storage.go` Lifecycle doc).
- [ ] 1.8g Deploy, then 503 one give on the dev box (stop Zitadel briefly) and watch it go on its own at the named time; audit shows `propagation.retried` #ready-for-human
- [x] 1.9 Move the add-on transport onto `extcall` (W1e): `addons/transport.go` (`doAuthenticatedWith` is one `extcall.Do`, `addonPolicy`, `addonAttempts = 1` keeps the old send-once rule); tests and guard in `addons/extcall_path_test.go`
- [ ] 1.10 Owner: after deploy, watch `[ZITADEL] ... req=` lines during the next Zitadel blip #ready-for-human

## W1b. Error envelope
- [x] Move the envelope to `handlers/errors.go`; sanitise raw upstream text at 502/503/504 `ready-for-agent`
- [x] Proxy errors use the envelope; gateway text never reaches the screen
- [x] `merge_findings.go` and `claim_shape.go` bodies use the envelope
- [x] `writeUpstream` at every raw call site, `rawUpstreamCodes` deleted; guard `TestNoErrorTextReachesAFiveHundredBody` (branch `fix/one-name-path`)
- [ ] Route `bundles.go`, `access.go`, `claimserve/door.go`, the add-on writer through one envelope type `needs-triage`

Branch `fix/one-addon-pipe`, 5 Nov 2026. Proposal: [proposal.md](proposal.md).

## W1e: backend add-on pipe

- [x] `addons/pipe.go`: `send`, `operate`, `StatusFor`. `read.go`, `entitlement.go` and `transport.go` `Call` have no direct breaker, credential or transport call left. Proof: `TestEveryAddonCallGoesThroughThePipe` (parses the package; fails if any file but `pipe.go` calls `doAuthenticated`, `br.allow`, `br.record` or the record writers; `registry.go` keeps the registration-time manifest read).
- [x] A trace before each write. Proof: `TestAWriteWithNoTraceIsRefusedBeforeItLeaves`, `TestTheTraceIsWrittenBeforeTheCallAndSettledAfter` (trace, call, settle, in that order, for `setting.put` and `lifecycle.set`), `TestLifecycleLeavesNoCallWithoutItsRecord`.
- [x] An idempotency key on every write, as the `Idempotency-Key` header (the row id; the outbox call id for apply). Proof: `TestOneIdempotencyKeyReachesTheTargetOnce`, `TestApplyCarriesItsCallIdAsTheKey`.
- [x] Lifecycle trace and audit row. `SetLifecycle` takes the actor; the handler passes `resolveActor`. Proof: `TestTheTraceIsWrittenBeforeTheCallAndSettledAfter`; the audit line comes from `db.BeginAddonOperation` (same transaction as the row).
- [x] Lifecycle still skips the breaker, neither consulted nor fed. Proof: `TestBreakerIsConsultedByThePipeAndLifecycleIsExempt`.
- [x] `addonop.statusFor` deleted for `addons.StatusFor`. `TestEveryDispatchOutcomeHasAnAcceptedStatus` runs against it, and now also checks an unknown outcome is indeterminate.

## W1f: one `decide` in the TrueNAS add-on

- [x] `decide`, `change`, `writes`, `applyTo` in `addons/truenas/apply.go`; `projected` (new accounts only) and `planOne` in `plan.go` read them. Proof: `TestPlanAndApplyAgreeOnEveryConvergence` (7 cases through both handlers: same effect, same write or no write, plan's consequence equals what apply left), `TestDecideIsPureAndWritesOnlyWhatDiffers`. The existing `apply_test.go`, `contract_test.go` and recorded-fixture tests pass unchanged.

## Open

- [ ] The add-on's `PUT /settings/{name}` does not read the `Idempotency-Key` header (`addons/truenas/settings.go` was outside this wave). Writing the same value twice is harmless today, so nothing breaks, but the key is not deduplicated add-on-side until it does. #ready-for-agent
- [ ] (architecture-deepening item 3's last gap) Create-path home: plan projects a new account's home from the desired state, apply asks `decideHome` (which reads the target). They can still disagree when the folder already exists. Fixing it makes plan read the filesystem for new accounts, which changes plan output, so it was left. #needs-triage
- [x] (release/coherence: `UnconfirmedChanges.tsx` words `lifecycle.set` as the system's own state, "Set TrueNAS to read-only"; the audit vocabulary already had "a maintenance change") The Unconfirmed changes card (`ui/.../UnconfirmedChanges.tsx`) and `ui/src/lib/audit-vocabulary.ts` have no wording for `lifecycle.set`; it falls back to the default sentence. A lifecycle row that settles `indeterminate` is settled by a person, not from a read. #ready-for-agent
- [ ] Outcome-to-HTTP mapping per handler (audit family 2, "differs per handler") and "reads drop the breaker reason" are not part of W1e. #needs-triage

## W2b · One cache refresh (branch `fix/one-refresh-path`)

- [x] `cache/refresh.go` `Refresh`; drain refreshes applied, failed and exhausted rows
- [x] Service-account mark/unmark and bundle rename refresh through it
- [x] Guard `cache/refresh_guard_test.go`; behavioural tests per path
- [x] `webhook.go`, `access.go`, `bulk.go`, `role_members.go`, `claim_delivery.go`, `expiry/sweep.go` reach `Refresh` through one seam per package, `cacheRefresh = cache.Refresh` (`handlers/deps.go`, `services/expiry/deps.go`); call sites name the people, and the best-effort ones log through `refreshOrLog` (`handlers/access.go`); `bundleRefresh` folded into `cacheRefresh`; `cache.RefreshUser` unexported (`refreshPerson`), so the compiler, not a scan, keeps every other refresh out; the guard (`refresh_guard_test.go`) reads the syntax tree: each seam bound to `cache.Refresh` under the file's own import name, each write-path function calling its refresh in its own body; `cache.RebuildUserCache` deleted, `seed/demo.go` uses `Refresh`; the guard has no allow-list
- [x] Webhook `processGrantAdded` refreshes after the onboarding write, not before (`TestProcessGrantAdded_RefreshesAfterTheOnboardingWrite`; the rule write the old text names is gone, the onboarding trigger is the write left)
- [x] Dead `directory.InvalidateAll` deleted (interface, both sources, test stubs); comment at `directory/zitadel.go:168` corrected. `InvalidateUsers` kept: no production caller, but `directory_test.go` uses it to expire the list and `TestZitadelSource_InvalidateDuringARebuildIsNotUndone` guards the `usersGen` race; its doc now says so. Removing it means rewriting those tests: `ready-for-human`

## W2c · One invalidation map

- [x] `lib/queries/invalidation.ts`; all mutation hooks and the two signing-key sites use it
- [x] Guard test: every mutation uses it; rail kinds hold governance
- [x] `HomeLimit.tsx`, `UnconfirmedChanges.tsx`, `DegradedBanner.tsx`, `useUpstream.ts` onto `invalidateAfter` (kinds `upstream`, `reconnected` added); `states/index.tsx` and `request-decisions.ts` re-read what is on screen (`refetchQueries`, `type: "active"`, as invalidation did), since they are not writes. The guard is a lint rule (`ui/eslint.config.mjs`, `no-restricted-syntax` on the syntax tree: `x.invalidateQueries`, `x["invalidateQueries"]`, destructuring) over every non-test file but `invalidation.ts`; CI's `bun run lint` runs it. Mutation-checked all three forms.
- [x] `["name-catalog"]` defined twice: now one `NAME_CATALOG_KEY` / `NAME_BUNDLES_KEY` in `useNameCatalog.ts` (see W2d)

## W2d: one path for a name (branch `fix/one-name-path`)

- [x] `ui/src/lib/names.ts` `roleName`; inline chains, local hooks and `shownNames.ts` routed through it
- [x] Ids out of the audit CSV name columns (id columns added), People on a target, Unexplained access picker, plan review sort, create-role picker
- [x] `backend/internal/textsort`; twelve backend name sorts routed through it
- [x] Guards: `names-one-path.test.ts`, `repoguard/name_sort_test.go`
- [x] One person fallback phrase across screens: `personOr` / `UNKNOWN_PERSON` in `lib/names.ts` ("this person", "This person" to open a sentence); ten sites routed; guard in `names-one-path.test.ts` (release/coherence)
- [x] No id passed off as a name: `directory.Source.ProjectName` returns `(name, ok, err)`, `("", false, nil)` when unlisted (`directory/directory.go`, `zitadel.go`, `demo.go`, `demo/catalog.go`); `claim_shape.go`, `role_members.go`, `views.go` (`upsertRole`, observed-only bucket), `insights_trends.go`, `insights_lens.go` (`lensNamer`) and `roles.go` (`GlobalRoleCatalog`, a seventh) send `""`; the UI renders it through `projectLabel` / `ResolvedProjectName` (`useUsers.ts` select, `Makerspace.tsx`, `CreateRoleDialog.tsx`, `ProjectClaimDefault.tsx`, `roles/[key]/page.tsx`); `services/project_name_test.go`, `directory_test.go`, `roles_test.go`
- [x] Go `roleLabel` (`services/views.go`) now is `humanizeKey` character for character, pinned by `ui/src/lib/__fixtures__/humanize-key.json`, read by `services/role_label_test.go` and `lib/__tests__/names.test.ts`
- [ ] SQL `ORDER BY name` (`db/bundle_includes.go`, `target_bindings.go`, `role_waits.go`): DB collation UNVERIFIED #needs-triage
- [ ] One envelope type for `bundles.go`, `access.go`, `claimserve/door.go`, add-on writer #needs-triage

## Coherence group D (branch `fix/coherence-remaining`)

- [x] Rate limits: the door counts only after every key and the refresh failed (`claimserve/door.go` `SignatureCheck`), shared by the event webhook; `TestRateLimit_VerifiedWebhookCallsAreNeverCounted` pins it. A 429 shows the server sentence through `describeFailure`/`outcomeFromError`.
- [x] `writeUpstream` sentences name what failed (Zitadel, that system) and `describeFailure` adds the next step and the full reference to a bare line; cards keep `failureMessage` plus their own reference.
- [x] Claims page: check 5 "unlisted" says access, not role (the gate is any grant); the not-listed notice ignores an unreadable call record; save, History restore and the save row say what holds a shape back (`heldBackLine`) instead of promising "next sign-in" while Zitadel names no app or lists no project.
- [x] Template sheet and chooser use `heldBackLine` instead of "keeps the project's default until you save" while Zitadel names no app
- [x] Partial-failure codes (`MAY_HAVE_GONE_THROUGH` in `ui/src/lib/outcome.ts`) say "Some of this may have gone through. Check <where> before trying again."; `partial-codes.test.tsx` reads the handlers and fails on an undecided 5xx code

- [x] A lost reply on a write (`LostReply` in `api-client.ts`, the proxy `BACKEND_UNREACHABLE`/`UPSTREAM_ERROR` on a non-GET) says "Check Pending changes or Audit", never "Nothing was changed."; `ANSWER_NOT_RECORDED` mapped, `CREDENTIAL_NOT_SET` and `PLAN_CLAIM_ERROR` verified as writing nothing

## W2a · One access core (branch `feat/one-closure-core`)

- [x] 2a.1 `internal/access`: `Ends`, `Closure`, `Fired`, `Later`, `Left`; `models.RuleClosure` deleted
- [x] 2a.2 Compiler on `access.Ends`: a rule role ends when its source ends (`cache/compiler_ends_test.go`)
- [x] 2a.3 `collectUserRoles` on `access.Ends` + `Fired`; `EffectiveRole.Ends`; role members' Ends from it; person page reads `role.ends`
- [x] 2a.4 `services.MappingHolders` (records + closure − people who left); `db.HeldRecords`; `ListMappingHistory` takes the holders func
- [x] 2a.5 Drift and reconciliation on `services.RuleDerived` (records only, forward); `ExpectedViaRule` deleted
- [x] 2a.6 Bundle readers on `bundleGivesSQL` (role usage, referenced roles, projects' bundle count, the access map); bundle impact on the latest version
- [x] 2a.7 `BundleHolding`, `RuleHolding`; `observed_holder_count` on bundle and rule lists
- [x] 2a.8 What-if kinds `rule_added`, `bundle_assigned`, `bundle_unassigned`; the three UI previews ask it
- [x] 2a.9 UI: observed holder numbers, recorded ones labelled; `holdersLine` never heads with a recorded number; six fallbacks sort by observed (`heldSortKey`); copy traced (see design)
- [x] 2a.10 One `Left` for bulk, insights, usage and the entitlement path
- [x] 2a.11 Parity and behaviour tests; two mutations checked
- [ ] 2a.12 `handlers/bundles.go` `GET /bundles/{id}/roles` (working copy, the editor's own list) and `BundlePage.tsx` re-deriving includes (`:164-167`) left: the editor shows what it edits #needs-triage
- [x] 2a.13 `RequestHolding` (`db/access_requests.go:84`), `GrantDirectAccess` `noteOf`, `MemberAccess` and `observedRolesByProject` overwrite: W3b's holding index. Done 10 Nov 2026: `RequestHolding` answers from the standing (`services/standing.go`), `noteOf`/`askState`/the member catalogue read `holdings`, `MemberAccess` reads `standings`, `observedRolesByProject` keeps every grant row (it overwrote). See 2a.17
- [x] 2a.14 `claim_shape.reachOf`, `insights_asof` (recorded fallback), the "Bundle … is unused" hint (assignments): onto the snapshot in W3b. Done 10 Nov 2026: `reachOf` already read it; `insights_asof` stores null before the first read (owner; migration 000084); the unused-bundle hint stays on assignments (owner), on its grouped count
- [x] 2a.17 **Role standing, built with W3b's holding index (owner, 10 Nov 2026; folds in architecture-deepening item 2).** One answer to "does this person hold this role, and why", computed once by the backend and sent with each role; the UI only renders it, never combines sources. Exactly five standings: **held** (seen in Zitadel), **sent, not seen back yet**, **waiting to be sent**, **being removed** (removal queued; still held, because holding is observed), **not held**; each with its reason (the bundle, rule or direct grant that explains it). Today the person page reads `collectUserRoles` without `svcQueuedRevocations` while the cascade and what-if subtract them, so mid-removal they disagree; the standing ends that. Do not close W3b without it. **Built 10 Nov 2026** (branch `feat/role-standing`; design.md W3b): `db.latestDeliveries`/`Deliveries` (one latest-row read and rule, shared with the planner's `deliveredFor` and the cascades' `queuedRevocationsFor`), `services.standingOf`, `personBasis`, `accessSnapshot.Standing`/`BasisFor`, `holdingFrom`; `/users/{id}/access` sends `standings` and `holdings`, role members `standing`; UI `lib/standing.ts` + `standing-guard.test.ts`. Tests: `standing_test.go`, `standing_view_test.go`, `request_holding_test.go`, `role_deliveries_live_test.go`; UI `standing.test.ts`, the surfaces' tests. `observed_only` holders carry a standing too (10 Nov 2026; the delete guard still counts a holder being removed, owner).
- [ ] 2a.15 Cost: bundle and rule lists now walk the snapshot per request, like the role catalogue. Watch on prod; cache the snapshot across requests if slow #needs-triage
- [ ] 2a.16 After deploy: drift may raise rule targets whose source is only in Zitadel; triage them with their source (operator) #ready-for-human
