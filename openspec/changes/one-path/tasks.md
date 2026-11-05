# Tasks: one standard path per operation

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
