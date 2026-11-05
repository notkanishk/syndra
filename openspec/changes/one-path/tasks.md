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
- [ ] 1.13 `handlers/roles.go` (`POST /api/v1/roles`) to answer through `writeRoleWriteError`, so both screens answer refusals alike (it is not a W1a file) #ready-for-agent
- [ ] 1.14 Deploy, then delete a role nobody holds on the dev box and confirm both audit rows and the read-back (operator-gated) #ready-for-human

## W1d · Retries (branch `fix/one-retry-core`)

- [x] 1.1 `internal/extcall`: Policy, Backoff (jittered, capped), RetryableStatus, NoAnswer, Do with per-try timeout, read-back and request-id logging
- [x] 1.2 Zitadel client on `extcall.Do`; reads retried, writes resent only after 429; 401 refresh once per call on any try
- [x] 1.3 Read-backs on `AddUserGrant` and `AddProjectRole`
- [x] 1.4 `classifyZitadelError` uses `extcall.RetryableStatus` (truth table unchanged)
- [x] 1.5 Migration 000080 `propagation_outbox.not_before`; `RequeuePropagation` sets it; both batch claims respect it
- [x] 1.6 One start-up wait schedule in `cmd/api/main.go`
- [x] 1.7 Tests: backoff spacing and cap, retryable vs terminal, read-back on ambiguous write, outbox respects `not_before`, max attempts to failed (live), down migration (text and live, in a rolled-back transaction)
- [ ] 1.8 Show "next try at" (`not_before`) on the pending worklist. The models and UI are outside W1d's files #ready-for-agent
- [ ] 1.9 Move the add-on transport onto `extcall` (W1e) #ready-for-agent
- [ ] 1.10 Owner: after deploy, watch `[ZITADEL] ... req=` lines during the next Zitadel blip #ready-for-human

## W1b. Error envelope
- [x] Move the envelope to `handlers/errors.go`; sanitise raw upstream text at 502/503/504 `ready-for-agent`
- [x] Proxy errors use the envelope; gateway text never reaches the screen
- [x] `merge_findings.go` and `claim_shape.go` bodies use the envelope
- [ ] `writeUpstream` at every raw call site, then delete `rawUpstreamCodes` (needs W1a, W1e files) `needs-triage`
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
- [ ] Create-path home: plan projects a new account's home from the desired state, apply asks `decideHome` (which reads the target). They can still disagree when the folder already exists. Fixing it makes plan read the filesystem for new accounts, which changes plan output, so it was left. #needs-triage
- [ ] The Unconfirmed changes card (`ui/.../UnconfirmedChanges.tsx`) and `ui/src/lib/audit-vocabulary.ts` have no wording for `lifecycle.set`; it falls back to the default sentence. A lifecycle row that settles `indeterminate` is settled by a person, not from a read. #ready-for-agent
- [ ] Outcome-to-HTTP mapping per handler (audit family 2, "differs per handler") and "reads drop the breaker reason" are not part of W1e. #needs-triage

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
- [ ] 2a.13 `RequestHolding` (`db/access_requests.go:84`), `GrantDirectAccess` `noteOf`, `MemberAccess` and `observedRolesByProject` overwrite: W3b's holding index #ready-for-agent
- [ ] 2a.14 `claim_shape.reachOf`, `insights_asof` (recorded fallback), the "Bundle … is unused" hint (assignments): onto the snapshot in W3b #ready-for-agent
- [ ] 2a.15 Cost: bundle and rule lists now walk the snapshot per request, like the role catalogue. Watch on prod; cache the snapshot across requests if slow #needs-triage
- [ ] 2a.16 After deploy: drift may raise rule targets whose source is only in Zitadel; triage them with their source (operator) #ready-for-human
