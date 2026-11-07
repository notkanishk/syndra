# Rules on the pipeline: tasks

## 1. One closure
- [x] 1.1 `models.RuleClosure` (generic over the key), unit-tested: a chain in any order, a cycle that ends, the base left alone
- [x] 1.2 The compiler, `effectiveClosure` (every cascade), `collectUserRoles` and `db.MappingHolders` (SQL → Go) call it; the "keep in sync" copy is gone
- [x] 1.3 Guard `TestTheMappingCohortCountsEverySourceOfTheRole` now requires `models.RuleClosure(` instead of `WITH RECURSIVE`

## 2. Rules on the outbox
- [x] 2.1 `db.FollowRules` hook in `enqueueWrites` (write split into `enqueueOwn`), wired by `services.followRules`; rule rows via `enqueueCascadeRows`, `source='rule'`, `source_ref` = the rule reaching the role
- [x] 2.2 `EnqueueResult.RuleOutboxIDs` / `OutboxIDs()`; the grant handler, approvals and bulk give/extend deliver them with their own row
- [x] 2.3 Add-on targets reached by derived roles converge in the same transaction
- [x] 2.4 Webhook fires no rules; `EnforceMappingRules`, `RevokeMappingRules`, `StillExpected`, `fetchAllUserGrants` deleted; repo guard's KNOWN GAP entry removed (`TestOnlyTheTracedPathWritesZitadelGrants` now allows only the drain)
- [x] 2.5 Tests (fakes): own grant queues the whole chain; removal follows the chain and keeps what a bundle still gives; nothing owed when the closure does not move; no rules costs no reads; a failed write queues nothing; derived roles converge; the hook is wired
- [x] 2.6 Tests (live DB): own grant's outbox adds == `cache.CompileFacts` (A→B→C), all pending before any send, no ledger row for rule rows; a retried give duplicates nothing; a crash mid-send is reclaimed once and never after applied; a failure after the write leaves no ledger or outbox row; removal revokes B and keeps directly-held C
- [ ] 2.7 [ready-for-human] Before the first production rule: create one on staging, give its source by hand, confirm the target lands in Zitadel and in Change history under the rule

## 3. confirmation_mode (reconcile-build 3.9)
- [x] 3.1 Traced every read; inert under roles (design.md table); `TestDeliver_UnderRolesTheOldModeChangedNothing`
- [x] 3.2 Removed: model fields, db reads/writes, `Set*ConfirmationMode`, `NormalizeConfirmationMode`, the global default and its endpoints, `POST /policies/confirmation-mode`, `ListWaitSuggestions` and the one-time move, `CascadeResult.Mode`; create bundle/rule refuse a `confirmation_mode` field (strict JSON)
- [x] 3.3 UI: types, the policies hint (role only), retire outcomes read `wait_reason`, Roles that wait always shows the list

## 4. app_claim_overrides (claims-shaping 4.9)
- [x] 4.1 `seedApp` folds nothing; `ListAppClaimOverrides` and its row type removed; `scripts/reset-data.sh` no longer names the table

## 5. Migration 000078
- [x] 5.1 Refuses unless `confirmation.model = 'roles'`; refuses while `app_claim_overrides` has a row; drops both columns, the default setting, the table; down restores the schema
- [x] 5.2 Text guard, no-query-names guard, live refusal test (four cases, rolled back); mutation-checked (guard removed → the live case fails)
- [ ] 5.3 [ready-for-human] Staging runs 000078 on last night's copy of production before production does

## 6. Next wave: the other "who holds what" readers (audit family 3, not in this change)
- [x] 6.1 (one-path W2a) Compiler forces rule-derived roles endless (`cache/compiler.go`, `ponytail:` note): the token says a rule role never ends when its source does
- [ ] 6.2 [needs-triage] Seeds differ: the cascade subtracts queued revocations, the compiler and `collectUserRoles` do not (one `BaseHoldings` with options, per the audit)
- [x] 6.3 (one-path W2a; departed only, queued removals stay a cascade concern, see one-path design) `MappingHolders` counts departed people and people whose removal is queued
- [x] 6.4 (one-path W2a, `services.RuleDerived`) `ExpectedViaRule` (drift) seeds observed Zitadel grants while the person view uses records only; a reverse walk, not moved onto `RuleClosure`
- [x] 6.5 (one-path W2a, the what-if) UI one-hop previews: `app/policies/page.tsx` (~325), `components/people/ManageBundles.tsx` (~210, first matching rule only), `components/bundles/BundlePage.tsx` (~903); replace with one backend preview
  - [x] 6.5a The backend preview exists (coherence audit C1, C2, 5 Nov): `services.RulesFollowing` asks `followRules` in advance through the same `ruleDelta` (same reads, same closure: `internal/access` since release/coherence), each row named "Also gives {role} in {project}, by the automatic rule from {source}". `GET /users/{id}/rule-preview` serves it; a parity test holds preview == queued rows (8 Nov: deleted with `RulesFollowing`; the planner's `rules` are the one source, `GET /users/{id}/plan`, architecture-deepening 1.2b.)
  - [x] 6.5b The consent surfaces that give a role read it before the confirm: give (`GrantDirectAccess`, footer note and result), approve (`requests/Holding.tsx` `HoldingNote`, on Home and Requests), drift adopt (`UnexplainedAccess` ResolutionDialog: lede, rows, confirm held while it loads, result counts the rule rows), bulk give and bulk bundle assign (`services.RehearseBulk` consequence + fingerprint, so a rule added after the review moves the row), bulk drift adopt (`drift_rehearsal.go` consequence + fingerprint, apply line counts what was queued)
  - [x] 6.5c Adopt returns `rule_outbox_ids` (`db.AttributeDriftTx` returns the rule rows `enqueueWrites` queued); they are adds and wait in Pending changes for the operator (ADR-006)
  - [x] 6.5d The rule editor (`app/policies/page.tsx`) names the whole chain, not one hop (audit W12), from the backend: the what-if `rule_added` takes the unsaved rule (source, target, and `rule_id` for an edit) and computes it with `internal/access` (`services/insights.go`). The browser walk is gone (release/coherence); `saveRule.test.tsx` "never walks the rule list in the browser"
  - [x] 6.5e No surface left one-hop or silent: `ManageBundles.tsx` and `BundlePage.tsx` ask the what-if (one-path W2a); Zitadel › Users give dialog reads `useRulePreview`, the endpoint the person page's give reads, holds the give while it loads (release/coherence, `grantOutcome.test.tsx`); the approve result line reads `GET /users/{id}/rule-preview` before the write (group B) (8 Nov: Give, Approve and Manage bundles now say the planner's rule sentence, `GET /users/{id}/plan`, architecture-deepening 1.2.) (8 Nov: Drift adopt and Zitadel 'Users' read the plan too; `/rule-preview` is deleted, architecture-deepening 1.2b.)
- [ ] 6.6 [needs-triage] `RequestHolding` reads no bundles or rules; `GrantDirectAccess` `noteOf` ignores queued revocations; `observedRolesByProject` overwrites two grants in one project; working-copy bundle readers ignore includes and pins
- [ ] 6.7 [needs-triage] `?apply=true` and `switchedOver` (the legacy direct-grant wait) and `PUT /config/confirmation-model` are dead under roles; remove with the switch-over UI's last trace
