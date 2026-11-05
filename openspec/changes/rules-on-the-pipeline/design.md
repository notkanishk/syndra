# Rules on the pipeline: design

ADR-006 records the decision. This file covers how it works and the evidence.

## The path a rule takes now

```
direct grant / approval / bulk give / replace / revoke / drift adopt
  └─ db.enqueueWrites                     propagation_enqueue.go:152   (access lock, one tx)
       └─ db.FollowRules (services.followRules)   rule_follow.go:37
            closure before  = RuleClosure(userBaseHoldings, active rules)
            write()         = enqueueOwn: ledger + audit + own outbox row
            closure after   = RuleClosure(userBaseHoldings, active rules)
            derived         = closure delta − the write's own base change
            converge the add-on targets the derived roles reach (same tx)
       └─ enqueueCascadeRows(derived)     propagation_enqueue.go:175   source='rule', source_ref=rule id
  └─ EnqueueResult{OutboxID, RuleOutboxIDs}
handler delivers OutboxIDs() through services.Deliver (access.go:185, :647; bulk.go:365)
  └─ propagation drain: add merges into an existing grant (UpdateUserGrant), 409 is idempotent
```

Bundle and rule cascades were already on the outbox and already followed the closure (`services/cascade.go`), so they are unchanged.

**Why the chokepoint.** `enqueueWrites` is the second of the two enqueue chokepoints, and the only one direct writes pass. Hooking each caller would mean six hooks, and the next caller added would be the forgotten one. That is the argument `deltaParams` already makes for cascades.

**Why a hook from db to services.** The definition of what a person holds (direct grants ∪ pinned bundle versions − queued revocations) lives in services (`userBaseHoldings`), and services imports db. The pattern is the one `zitadel.StillExpected` used, wired in `init`. When the hook is nil (db's own unit tests), the write goes alone. The binary always links services.

**Why read after the write instead of simulating.** Both reads happen in the same transaction under the access lock, so the post-write read sees exactly the write and nothing else. It handles add, replace, revoke and adoption alike, with no per-op simulation to keep in step.

**No loop.** Derived rows are outbox rows with no ledger row. Delivering them changes nothing `followRules` reads, and their webhook echo is Syndra's own and is dropped. The webhook no longer fires rules at all.

**What the webhook does with a Zitadel-side grant.** It rebuilds the cache and raises drift, and fires no rule. The token does not count an untraced grant as held, so a rule fired from it would give Zitadel a role the token does not carry. Adopting the drift is a Syndra write, and adopting does fire the rules.

## One closure

`models.RuleClosure` (`internal/models/rule_closure.go:24`) is generic over the key type, so each caller keeps its own key and pays no copy. Its callers:
- `cache/compiler.go:230`: the token facts.
- `services/cascade.go:139` `effectiveClosure`: every cascade, `followRules`, insights, welcome claims and direct-grant removal.
- `services/views.go:1507` `collectUserRoles`: the closure, then one pass that names the rules reaching each role.
- `db/mappings.go:291` `MappingHolders`: moved from a `WITH RECURSIVE` query to Go.

The live test `TestRulesOnThePipeline_OwnGrantSendsZitadelExactlyTheCompiledChain` asserts that the outbox adds for a grant of A, with rules A→B→C, equal `cache.CompileFacts` over the three projects.

## confirmation_mode: inert under roles (finding)

The owner asked: under the roles model, does a bundle's or rule's own mode still change behaviour? **No.** Every read was traced:

| Read | Effect under `confirmation.model = roles` |
|---|---|
| `services/cascade.go` `applyMode` → `Deliver(ids, mode=="manual")` | `Deliver` computes `legacy := legacyWait && !switchedOver(ctx)`, which is false under roles. Only role waits hold a row. |
| `services/welcome_claims.go:53` | `manual && !switchedOver`, which is false under roles. |
| `db/role_waits.go` `ListWaitSuggestions` | Read only when not switched over (`handleListRolesThatWait`). |
| `handlers/cascade_surfaces.go`, `bundles.go:187`, `rules.go:117`, `POST /policies/confirmation-mode`, `GET/PUT /config/confirmation-mode-default` | These only write or read the mode itself. Nothing else consumes it. |
| UI policies `waitsForYou` | `!switched && …`, which is false under roles. |
| UI rule and bundle retire outcome, `cascade.mode === "auto"` | **Display only.** It guessed "sent" or "waiting" from the mode. Under roles it was wrong for a manual bundle, whose rows were in fact sent. Both production bundles are manual. It now reads the server's `wait_reason`. |

Two edge cases, neither a behaviour under roles:
- `switchedOver` treats an unreadable setting as "not switched", so during a failed read of `config_settings` a manual bundle's rows would have been held. That fallback has nothing left to fall back to. Migration 000078 refuses unless the model is `roles`.
- `?apply=true` on the grant endpoint is the same `legacyWait` path for direct grants, not a bundle or rule mode. It is left alone (NEXT.md 0c says so too).

`TestDeliver_UnderRolesTheOldModeChangedNothing` checks the claim: across every combination of role marks and send outcomes, `Deliver(ids, true)` and `Deliver(ids, false)` produce the same rows sent, the same rows waiting and the same reasons recorded.

## Migration 000078

- It refuses unless `config_settings['confirmation.model'] = 'roles'`, and refuses while `app_claim_overrides` has any row. Both checks run before any DROP.
- It drops `bundles.confirmation_mode`, `mapping_rules.confirmation_mode`, the `global.default_rule_confirmation_mode` setting and `app_claim_overrides`.
- The down migration restores the schema: the columns come back as `auto`, the table comes back empty, and both indexes and the CHECK return. It does not restore the values.
- Tests:
  - `TestMigration000078_RefusesBeforeItDrops` guards the text.
  - `TestNoQueryNamesWhat000078Dropped` checks that no db source names a dropped thing.
  - `TestMigration000078_RefusalsAgainstTheSchema` runs live. It runs the down, sets up the state and runs the up, inside a rolled-back transaction, for four cases.
- The numbering was coordinated: this change takes 000078, the time branch 000079 and the retries branch 000080.

## Deviation from the brief

The brief said rules should fire "not only on webhooks". Instead they **no longer fire on webhooks**. A webhook that survives the self-mutation guard is a Zitadel-side change with no Syndra trace. CLAUDE.md and `ConvergeProjectChange` both say such a change is drift to triage, never a decision to act on. The compiler does not count it, so firing a rule from it would put Zitadel ahead of the token. Rules fire on every change to Syndra's own record, including adoption of that drift.
