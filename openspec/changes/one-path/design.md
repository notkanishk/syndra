# One path — design

## W1a · Zitadel screen pipes

### Roles: `writeRole` (`backend/internal/services/roles.go`)

```
trace tx (access lock) ── local(ctx) + audit(role.created|updated|deleted)
        │ commit; no trace, no write
        ▼
send(ctx, zitadel.MgmtClient)
        │ error?
        ▼
readBackRole (detached ctx, 15s; first page of ListProjectRoles)
        │ landed(found)?  yes → done (log only)
        ▼ no, or the read-back failed
undo tx (access lock) ── undo(ctx) + audit(role.not_changed, {action, error, read_back_error?})
        ▼
ErrRoleNotChangedInZitadel
```

- **Trace.** The `roles` row is Syndra's record of the role, and the audit row is written in the same transaction. A role-only write has no outbox. The call is synchronous, so the transaction commits before the call, the same way key rotation does (`dbStartActionKeyRotation`).
- **Read-back on every error, not only on ones that look ambiguous.** A definite 4xx reads back as unchanged and is undone. A 503 or a timeout that landed reads back as landed and is kept. One rule is simpler than classifying errors, and it is correct for both.
- **Undo when the read-back itself fails.** Syndra cannot tell whether the change landed. It puts its own record back and says so in the `role.not_changed` details (`read_back_error`). The worst case is a Zitadel role with no local row. That case is visible: the catalogue lists it with source `zitadel`, and the next attempt reads it back as landed.
- **No client** (local-policy-only mode). The record is written and audited, and nothing is sent. This is what `CreateRole` already did, now true for all three operations.
- **Cache.** After any change, `directory.Default.InvalidateProject` runs, so the next catalogue read sees it. A role carries no per-person claims of its own: a delete is refused while Syndra gives the role, so no compiled claims cache holds it.

### Delete guard (`roleDeleteGuard`)

It runs inside the trace transaction, so the count and the write sit under one access lock.

| Condition | Answer |
|---|---|
| `RoleMembers(...).Members` non-empty, or a live bundle or rule names the role (`GetRoleUsageCounts`) | `RoleInUseError`, 409 `ROLE_IN_USE`. A confirmation never overrides it. |
| No observation, last observation failed, or it was capped | `RoleHeldError{Unknown}`, 409 `HOLDERS_UNKNOWN` |
| `len(ObservedOnly) != confirm_holders` | `RoleHeldError{Holders}`, 409 `ROLE_HELD` with `details.count` and `details.holders` (names, alphabetical) |
| otherwise | proceeds. The audit row's `holders_lost` lists their ids. |

The UI never counts the holders itself. It shows the refusal's names and count, and a tick (`AcknowledgeCount`) arms "Delete and revoke it for N people", which resends with `confirmHolders: N`.

### Grants: Zitadel › People (`backend/internal/handlers/discovery.go`)

| Route | Pipe |
|---|---|
| `POST …/grants` | `canGiveRoles` (`checkRoleOffered` + `dbRequestHolding`, all roles before any write) → `giveRoles` (one `grantDirect` add per role) |
| `PUT …/grants/{grantId}` | diff against the grant's roles → `canGiveRoles(adds)` → `removeRoles(removes)` → `giveRoles(adds)` |
| `DELETE …/grants/{grantId}` | `removeRoles(all roles on the grant)` |

`removeRoles` handles each role one of three ways:
- **direct grant** → `services.DeleteDirectGrant`, which uses the closure delta before and after;
- **no direct grant, but `UserExpectsRole`** (a bundle or a rule) → kept, reported in `retained_roles`;
- **nothing in Syndra gives it** → one traced `grantDirect` revoke.

`finishZitadelGrantChange` is the person page's tail: `rebuildUserCacheDetachedFn`, then `svcDeliver(ids, apply != "true")`. Remove is sent with `?apply=true`, as People › Access removes. Give and replace are not, as People › Access gives.

The response is `{outbox_ids, given_roles, revoked_roles, retained_roles, status, wait_reason}`.

**Known ceiling** (`ponytail:` in code): `UserExpectsRole` is read outside the access lock. W3a moves it under the lock.

## W1d · Retries (`backend/internal/extcall`)

**The rule.** Nothing is sent twice unless sending it twice is harmless.

| Outcome of a try | Read (`Idempotent`) | Write |
|---|---|---|
| 2xx, or any final status (4xx except 408/429) | returned | returned |
| 429 | retried with backoff | retried with backoff (the other side did not process it) |
| 408, 5xx, timeout, broken connection | retried with backoff | **ambiguous**: read-back if the caller gave one, otherwise returned unretried |
| caller's context cancelled | stop | stop |

- **Backoff.** `min(Max, Base·2^n)` with equal jitter, uniform in `[d/2, d]`. Half the wait is guaranteed, so spacing never collapses; half is random, so callers that failed together do not retry together.
- **Read-back.** It is called once, after an ambiguous outcome. "Landed" returns a synthetic 200, since the caller asked for an effect and the effect is there. "Not landed" or a read-back error returns the original failure, and the write is still not resent: a timed-out request may yet be processed.
- **Timeout.** A timeout applies to each try. Its cancel rides on the response body, so a caller can still read the answer after `Do` returns.
- **Logging.** A request id per call goes out as `X-Request-Id` and is logged under the caller's subsystem tag (`[ZITADEL]`), so the log vocabulary does not grow.

**Zitadel client.** `idempotent` covers GET plus the search POSTs (`/_search` in v1, `/search` in v2). The policy is three tries, 100ms doubling to 2s, and 10s per try. A 401 refreshes the token once per call, on any try. The read-backs are:
- `AddUserGrant`: does the user's grant on the project carry every role asked for?
- `AddProjectRole`: does the role key exist?

Both conclude only from a complete read.

**Outbox.** `propagation_outbox.not_before TIMESTAMPTZ NULL`, where NULL means claimable now. `RequeuePropagation` sets it in the same statement that increments `attempts`. The Go schedule (`extcall.Backoff` for 0..15 spent tries) goes in as an array and the row indexes it by its own count, so the count and the wait cannot disagree. The policy is 1m doubling to a 30m cap, roughly 1+2+4+8+16 minutes across the default budget of five. `ClaimPendingPropagations` and `ClaimPendingRevocations` apply `notBeforeDue`. `ClaimPropagationByID` does not: it is an operator's explicit apply of one row, and the budget still bounds it. The drain's existing exhausted branch (`drain.go:334`) is unchanged and lands the row in `failed`.

**The drain's classification** (`classifyZitadelError`) uses `extcall.RetryableStatus` for its transient set and keeps its own rule that a 409 on an add means applied. Its truth table is unchanged.

**Open (other waves).**
- `services/roles.go:80-84` still rolls back when the read-back cannot confirm the role. That is W1a's.
- The add-on transport's own retries are W1e's to move onto `extcall`.
- `PutSetting` has no idempotency key (family 2).

## W2a · One access core (`backend/internal/access`)

```
records (direct grants with ends, pinned bundle versions incl. includes)
   └─ access.Ends(base, rules)     ← the one fixpoint; Closure = Ends without dates
        ├─ cache/compiler.go       token roles + role_ends
        ├─ services.collectUserRoles  person view (+ Fired for reasons, + Ends per role)
        │     └─ accessSnapshot → HolderFacts, BundleHolding, RuleHolding, role members
        ├─ services/cascade.go effectiveClosure  every cascade, followRules, what-if, welcome
        ├─ services.RuleDerived    drift sweep + reconciliation (records only)
        └─ services.MappingHolders db.HeldRecords + Closure − access.Left
```

**Seed policy, stated once.** What a person holds in Syndra's records: unexpired direct grants and the roles of the bundle versions they are pinned to (own and included). Readers differ only in two deliberate ways: the cascades subtract queued revocations (they simulate the next state, so a queued revoke is already gone), and nothing seeds from observed grants (a rule never fires from a grant Syndra did not record, ADR-006). The audit's single `BaseHoldings(opts)` reader was not built: the compiler, the person view and the cohort read the same tables three ways for cost reasons (per person, per person with names, all at once), and the parity tests pin them together instead.

**Ends.** The zero time means never. `Later(a, b)` is the end of a role held two ways: never wins, else the later date. Along a rule the end is carried unchanged, so a role's end is the latest end among every base role that reaches it. Passes are bounded by `len(rules)`: the best path is simple.

**Departed.** Counts of who holds a role stay observed and include people who have left: they still hold it in Zitadel, and hiding them would hide real access. Only the mapping cohort drops them, because the entitlement path already disables their account.

**Counts.** A bundle is held when Zitadel shows every role the person's pinned version gives (own and included); a rule is held when Zitadel shows its target for a person it applies to. Both use `HolderFacts{Given, Confirmed, Observed, Basis}` from one snapshot; `Confirmed = Observed`, since both are measured among the given set.

**Previews.** The three UI previews ask `POST /insights/whatif`, which already ran the cascade's readers and closure. New kinds: `rule_added` (project/role = source, `target_project_id`/`target_role_key`, optional `rule_id` replaced), `bundle_assigned` and `bundle_unassigned` (`user_id`, `bundle_id`). BundlePage uses the existing `bundle_gives_not`.

**Evidence.** `access/closure_test.go`; `cache/compiler_ends_test.go`; `services/access_parity_test.go` (cohort = person view minus people who left; person view = core closure; rule roles end with their source; bundle and rule counts observed; never read is nil); `services/expected_test.go` `TestRuleDerived_AgreesWithThePersonView`; `drift/intent_sources_test.go` `TestSweep_ARuleTargetFromAnUnrecordedSourceIsDrift`; `services/insights_whatif_preview_test.go`; `handlers/holder_counts_test.go`; UI `holders.test.ts`, `list.test.tsx`, `removal-impact.test.tsx`, `saveRule.test.tsx`, `ManageBundlesPublished.test.tsx`, `Makerspace.test.tsx`. Mutation-checked: dropping the departed filter fails three parity assertions; making rule targets endless in `access.Ends` fails three `Ends` tests.

## W3b · One holding index, and each role's standing (2a.13, 2a.14, 2a.17)

Owner, 10 Nov 2026: one answer to "does this person hold this role, and why", worked out once by the backend and sent with each role; the UI only renders it. Five standings.

**Today (mapped 10 Nov, before the change).** No reader gives that answer. `/users/{id}/access` sends the recorded closure, a per-source `queued` flag and `observed_role_keys` per project, and never reads queued removals or unconfirmed writes; `/projects/{id}/roles/{key}/members` sends `in_zitadel` and no `queued`. The UI then combines them in six places (`MemberAccess` `standing()`, `PersonAccess` `RoleGroup` with a second fetch of Zitadel's grants, `GrantDirectAccess` `noteOf`, `Holding.tsx` `askState`, the role page's `holdCount`, `answers.ts`), each with its own order. A role whose removal is queued shows as held, or as missing, depending on the page. `db.RequestHolding` is the only reader that already treats a queued removal as leaving. `db.PendingDeliveriesForUser` does not filter `target`, so an add-on row can mark a Zitadel role as not sent.

**The standing.** For one person and one role (`project:role`):

```
latest outbox row for this role (target zitadel; add naming it, revoke naming it,
or any replace on its project — a replace gives what it names and takes the rest),
by intent_seq
   pending / in_flight, gives      → waiting      "waiting to be sent"
   pending / in_flight, takes      → removing     "being removed" (still held)
   applied, not confirmed, takes   → removing if Zitadel still shows it, else not_held
   applied, not confirmed, gives   → held if Zitadel shows it, else sent
otherwise (no row, confirmed, failed, discarded)
   Zitadel shows it                → held
   Zitadel read, complete, absent  → not_held
   Zitadel not read / incomplete   → no standing (null): absence can't be concluded
```

The latest row decides, so an add a later revoke took back, or the other way round, is never read as two states. `held` with no reasons is access Syndra didn't give (unexplained); `not_held` with reasons is access given and not in Zitadel. Neither needs a sixth word.

**Where it lives.** `services.accessSnapshot` becomes the holding index: beside `For` (records) and `Observed` (Zitadel), `Standing(userID)` reads the person's outbox rows once (`db.RoleDeliveries`, latest row per role, batch for many people) and applies a pure `standingOf` (table-tested). `ExplainUserAccess` and `RoleMembers` read it from the snapshot and send `standing` on each `EffectiveRole` and member. The UI gets one helper, `standingWords`, and the six derivations call it. `in_zitadel` and the second fetch of Zitadel's grants on the person page go once nothing reads them.

**The rest of W3b.** `RequestHolding` answers from the standing plus the direct grant's end (the request guard); `MemberAccess`'s overwrite of `observed_role_keys` goes; `reachOf` already reads the snapshot (a role being removed is still held, so no change); `insights_asof` stores nothing for a day before Zitadel was ever read (no recorded fallback); the "unused bundle" hint stays a question about assignments, and stays on its one grouped count (`db.GetBundleHolderCounts`): reading it through the snapshot would walk every person's records on Home for the same answer.

**Owner's answers, 10 Nov 2026.** (1) Zitadel never read or read incompletely: no standing (null); the page says it couldn't check, from the view's `observation`. The four outbox-based standings still show. (2) A role sent and not seen back yet counts as already had: the request guard and the pickers say it is on its way, and no duplicate request is made. (3) Insights history says "not checked" for days before the first read. (4) "Unused bundle" stays about assignments.
