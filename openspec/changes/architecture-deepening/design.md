# Architecture deepening: decisions

## Candidate 1 · One access-change planner

### Settled (owner, 16 Oct 2026)

1. **Scope.** Every operator action that sends access changes to Zitadel goes through the planner: give, extend, remove a direct grant, approve a request, assign or remove a bundle, publish or move a bundle. System work (expiry, rules, Zitadel event triggers) joins later through the same entry.
2. **Shape.** Inside the access-mutation lock, Syndra reads one snapshot: the person's base holdings, the rule closure, the queued changes, and the role catalogue. The planner is a pure function of (action, snapshot) that returns a **Change**: the writes, a refusal if a guard fails, the queued changes it cancels, and the words for the plan row. Preview renders the Change; apply commits it. No database access inside the planner.
3. **Apply.** Apply runs the planner again on a fresh snapshot under the lock. If the Change's fingerprint (writes and cancel ids) differs from the reviewed one, the plan is stale and nothing is written.
4. **Cancellations a plan didn't name.** Its own small change, held with the planner (owner, 16 Oct): the plan row stores the exact queued-change ids it showed, and apply admits only those. Any other cancellation is refused and the plan goes stale. This replaces "a reviewed plan admits every cancellation".
5. **Location.** A planner module in `backend/internal/services/`. `db` keeps the snapshot read and the outbox insert; handlers only call the planner. Deciding a request moves out of `handlers/`.
6. **One set of checks.** Every action runs the same guards ("Zitadel still offers this role", "already held"). A bulk row that fails one reads "No change" with the reason; the other rows go on.
7. **Mapping rules inside the planner.** The planner computes the full rule closure (every hop) and writes the rule-derived roles to the outbox with the role that triggered them. This replaces the one-hop, untraced `zitadel.EnforceMappingRules` walk for operator actions (NEXT §4b), and must land before the first composed role.
8. **One outbox insert.** `enqueueWrites` and `enqueueCascadeRows` become one function: it keeps the dedupe of an identical pending add, and the Zitadel grant id and `cascade_id` stamps.
9. **Single actions ask from the planner.** A single action with no plan learns what it would cancel from the Change and asks before it writes anything. The check at the outbox insert stays as a second safety.
10. **Tests.** Planner tests run on plain data with no database. Each action has one live-database test proving that the outbox rows its apply writes equal the writes its plan showed.
11. **Straight away or waits for you.** For each write, the planner reads the role's `waits_for_send` mark from the snapshot and marks the write "Straight away" (sent in the same request) or "Waits for you" (stays in Pending changes, `wait_reason = role_waits`). This becomes the only place that decides delivery, replacing the four checks listed in `design_handoff_syndra/reconcile-2026-10/BUILD-LIST.md:160`. Plans group rows by it, as route-automation-settings-v5 draws.
12. **A record before every send.** A "Straight away" write is still written to the outbox first, then sent at once. It shows in Pending changes only if Zitadel doesn't answer, with that reason.
13. **Order of work.** The Q4 fix first, then the planner (deciding delivery from the settings of the time), then `waits_for_send` inside it.

### Deferred (owner, 16 Oct 2026)

The planner waits until the design packages already given are built (the reconcile build order in `design_handoff_syndra/reconcile-2026-10/README.md`, and the answers to Q-04 and Q-05). That changes decision 13: `waits_for_send` (reconcile step 3) will land first, checked separately on each path. When the planner is built, it absorbs those checks.

### Settled (owner, 5 Nov 2026)

The frontier the deferral left open, answered "all yes":

1. **Extend is a plan row with no Zitadel write.** Syndra enforces end dates; Zitadel has none. An extend is a Change whose writes are ledger edits only.
2. **One snapshot per plan.** A bulk plan reads every person's holdings in ONE locked pass, not one read per row.
3. **The plan row.** It stores the exact queued-change (outbox) ids it showed and a fingerprint, and keeps the `BulkOutcome` shape. Apply refuses on any difference: the plan goes stale and nothing is written.
4. **First slice:** bulk assign bundle and bulk remove bundle.
5. **Then** single actions, then system work (expiry, adopting drift).

Facts as of 7 Nov (refreshed before building): rules already run on the outbox with the full chain (`enqueueWrites` → `FollowRules`, PR 111, ADR-006); `confirmation_mode` is gone (migration 000078); the webhook no longer fires rules (outside changes are drift); production has 0 mapping rules and 2 bundles. The access core is `backend/internal/access` (`Ends`, `Closure`, `Fired`, `Left`); the planner builds on it.

### Built: slice 1, bulk assign and remove bundle (7 Nov 2026, `feat/planner-bundles`)

- **The planner** is `services/planner.go`. `Plan(Action, PlanSnapshot) Change` is pure. A `Change` carries the writes (each a single-role outbox write with its delivery: `straight_away`, `waits_for_you` with `wait_reason = role_waits`, or `none` when it only cancels a queued change and nothing is left to send), the refusal (a short code: `held`, `not_held`, `not_offered:…`, `catalogue_unread`, `departed:…`, `absent`, `retired`), the queued changes it cancels, the version an assignment pins, and the plan row (`BulkOutcome`: effect, words, fingerprint, cancel ids). The writes are the closure delta (`access.Closure`, every hop), attributed to the bundle as the cascade always did.
- **The fingerprint** is the decision, not the words: op, person, bundle, effect, refusal code, pinned version, every write (op, role, source, ref, delivery) and every cancel id. A renamed role or an unrelated holding leaves it; anything that changes what is written or cancelled moves it.
- **The guards** are the same for every row (decision 6): already in the bundle (No change), not in it (No change), a give of a role Zitadel no longer offers (No change, with the step: take it out of the bundle and publish, or retire the rule), Zitadel's list unreadable (refused, try again shortly), departed or unknown account, retired bundle (refused). One failing row never stops the others.
- **The snapshot** is `db.ReadAccessSnapshot`, refused outside `InTxLockingAccess`: for everyone in the plan in one pass, their unexpired direct grants, assigned bundles, what each bundle's pinned version gives them, queued revocations, their pending queued changes, which roles stand in Zitadel (last applied delivery an add), the active rules and the roles that wait. The per-person reads (`GetDirectGrantsForUser`, `GetUserBundleRolesGrouped`, `QueuedRevocations`) are now the one-person case of the same batched statements, and `services.baseFrom` is the one base-holdings rule the cascades and the planner share. The directory's people and the role catalogue are read before the lock (it is never held across a call to Zitadel).
- **Which queued change a write cancels** is one Go predicate, `db.Answers` (with `db.OwedAfterCancel` for whether the write is still sent); `WouldCancel`, `withdrawUndelivered` and the planner all ask it. It replaced the SQL predicate `answeredWhere`.
- **Apply** (`services.ApplyPlannedBulk`) runs in one transaction under the lock: claim the plan (`db.ClaimPlanJoined`), read the snapshot again, plan every row again, and refuse the whole plan as stale (`services.StalePlan`, answered as the usual 409 `PLAN_STALE`) if any row's fingerprint or cancel ids differ. Rolled back, so nothing is written and the plan is not spent. Otherwise it admits only the stored cancel ids at the outbox (`db.CancellationsNamed`; the old `CancellationsReviewed`, which admitted every cancellation, is no longer called for these ops) and writes each row through `AssignBundleAndEnqueue` / `RemoveBundleFromUserAndEnqueue`. Delivery follows the commit, per row (since 7 Nov through `DeliverChange`, below; at first through the same `projectUpstream` → `Deliver` as every other bulk op).
- **The plan row stores** `cancel_ids` in `plan_subjects.outcome_json` (no migration), shape-checked and provenance-checked like grant ids: each must be a queued change of that subject's.
- **Words.** Assign: "Joins the Safety bundle." then "Gains Member in Laser Lab and Trained in Laser Lab." (alphabetical), "Also gives … by the automatic rule from …", "Giving … waits for you to send.", and the cancels line last. Remove: "Leaves the Safety bundle." then "Loses …", "Keeps … another way.", "Also loses …, which the automatic rule from … gave.", "Removing … waits for you to send.". Nothing to send is said: "Already holds every role it gives, so nothing is sent to Zitadel." / "Keeps every role it gave another way, so nothing is sent to Zitadel." A queued removal row now says "A role it removes always waits for you" instead of "gives".

Not in this slice, and listed as tasks: delivery decided only by the planner (apply still asks `Deliver`, which reads the same `role_waits` table, and a change to it between plan and apply makes the plan stale); one outbox insert (bundle ops only use `enqueueCascadeRows`, so the merge was not needed here); rule-derived rows carrying `source='rule'` and the triggering role (decision 7: a bundle's rule rows are still attributed to the bundle, as before); plans grouped by delivery on screen; the bulk reason recorded on bundle ops; and every other action.

### Built: planner foundations, tasks 1.4, 1.5, 1.8 (7 Nov 2026, `feat/planner-foundations`)

- **One outbox insert (decision 8).** `db.enqueueWrites` is the one function every access change reaches the outbox through, and holds the one `INSERT INTO propagation_outbox` for Zitadel rows; `enqueueCascadeRows` is gone. A change is either a write a person named (a key is passed: its ledger and audit rows via `enqueueLedger`, its own row with that key and no cascade id, an identical waiting add answering it, the mapping rules following it through `FollowRules`) or a cascade (its audit rows, a key per row, the grant id taken from the index for a revoke). Every other row carries the change's one cascade id, every row passes `withdrawUndelivered` and the cancel gate. Each of these was pinned by live tests against the two old functions before they were merged (`db/enqueue_live_test.go`), and the merge changed none of them. One small difference: a named write with an empty payload is now stored as `{}` (as a cascade's always was) instead of failing the insert.
- **Delivery decided only by the planner (decisions 11, 12).** A write the Change says waits for you is written waiting: `wait_reason = role_waits` is in the insert (`EnqueueParams.WaitReason`), so no drain can send it before the reason is on the row and nothing after the commit decides it again. The apply hands back each Change with its rows (`Write.OutboxID`); if the outbox wrote other rows than the Change says, the apply refuses and rolls back. `services.DeliverChange(ctx, Change)` sends the "straight away" rows through the same send `Deliver` uses (halt, retry, refusal: the same `Delivery` words) and never reads `role_waits` again. That is the seam 1.2 and 1.3 hand their Change to; paths not yet on the planner keep `Deliver`.
- **The bulk reason (1.8).** The operator's reason on a bulk bundle assign or remove is on the audit row (`details.reason`), on every outbox row of the change (`propagation_outbox.reason`), and in Change history (`CascadeGroup.reason`, shown quoted on the card), which is what the dialog promises. It also shows where a queued change's reason was already read: the cancel step ("“term start”" instead of "… gave no reason."), the plan's cancels line ("… queued 7 Nov (term start)"), and a cancellation in the audit log. The labels "Bundle membership cascade" / "Bundle removal cascade" were never stored for a cascade's rows. The single ManageBundles path asks for no reason and records none, as before.

### Built: rule rows carry the rule, task 1.6 (8 Nov 2026, `feat/planner-rule-rows`)

**The stored shape.** One shape for a rule's row wherever it is written: `source='rule'`, `source_ref` = the rule that reaches the role (in the closure after for a give, before for a removal). That rule's first role is the role that set it off, so the existing columns carry both and nothing is migrated (production had 0 mapping rules on 7 Nov). This is what `FollowRules` already wrote; a bundle's cascade, publish and move, and the planner now write it too. A role in a change's delta is the rule's when the change moved it only through the rules (its base holding did not move); otherwise it is the change's own (the bundle's). `services.ruleRefs` decides it, `services.asRuleRows` stamps it (`services/rule_follow.go:87`, `:107`); `followRules` uses the same pair (`:75`). A rule's own edit (create, update, retire) still attributes every row to the edited rule, as before.

**Which queued change a write cancels** (`db.Answers`, `db/withdraw.go:261`), each case with its test:

| Case | Cancels? | Why | Tests |
|---|---|---|---|
| Bundle removal, its own queued give | yes | same bundle | `db/answers_test.go`, `TestPlanner_RemoveBundle_CancelsItsOwnAndItsRulesQueuedRows` |
| Bundle removal, the give the rule queued under the bundle | yes | the removal's revoke of that role is now the rule's row, and a rule revoke answers queued rule gives | same two |
| Bundle removal, another bundle's give of the same role | no | different bundle | same two |
| Bundle removal, a direct give of the role the rule gives | no | a rule revoke never answers a direct give | same two |
| Rule's own retirement, the give it queued under a bundle | yes (before 1.6: no, the removal was queued behind the give) | rule revoke, rule give | `TestPlanner_RuleRetiredCancelsTheRowItSetOffUnderABundle`, `TestARulesRemovalCancelsTheRulesQueuedGive` |
| Rule removal, a give another rule queued for the same role | yes | a rule row is only ever a closure difference: a rule revoke is queued only when no grant, bundle or rule reaches the role, so no queued rule give of it is wanted. Matching the rule id too would let two rules reaching one role, picked in a different order on the way in and out, queue the removal behind the give | `db/answers_test.go`, `TestARulesRemovalCancelsTheRulesQueuedGive` |
| Direct removal of a role a rule's give is queued for | no, as today for a bundle's give | a direct revoke answers direct gives only | `db/answers_test.go`, `TestADirectRemovalLeavesARulesQueuedGive` |
| Direct (or bundle) give of a role whose rule removal is queued | yes, as today for every source | any give answers a queued removal | `db/answers_test.go`, `TestADirectGiveCancelsARulesQueuedRemoval` |

`GetActiveMappingRules` is now ordered (`db/rules.go:34`), so the rule a row names is the same between a plan and its apply; without it two rules reaching one role could make a plan stale for nothing.

**Every reader of `propagation_outbox.source` / `source_ref`** (traced 8 Nov):

| Reader | file:line | Effect of rule rows under a bundle's change |
|---|---|---|
| `Answers` / `AnsweredBy`, the one cancel predicate | `db/withdraw.go:261`, `:276` | rule revoke matches on source (above) |
| `withdrawUndelivered` (via `queuedChangeColumns`) | `db/withdraw.go:72`, `:311` | asks `Answers`; unchanged |
| planner snapshot, queued changes | `db/access_snapshot.go:90` | same columns; unchanged |
| planner writes and fingerprint | `services/planner.go:166`, `decided` `:246` | writes carry rule/ref; the fingerprint has source and ref, so a plan issued before this deploy goes stale on apply (stale-plan behaviour unchanged) |
| `CancelledUnsentDetails` (audit `cause.via`) | `db/withdraw.go:207` | via is "rule" for the rule's row a bundle removal cancels; the audit line does not show via (`ui/src/components/audit/EventLine.tsx` `CancelledChange`) |
| bundle cascades | `services/cascade.go:368`, `:496`, `:568` | rule rows |
| bundle publish and move, rehearsal and apply | `services/bundle_publish.go:301`, `:533`, `:659`, `:732` | rule rows |
| rule cascades (create, update, retire) | `services/cascade.go:441`, `:626`, `:684`; stamp `db/cascade.go:258` | unchanged: the edited rule |
| `FollowRules` | `services/rule_follow.go:75` | unchanged shape, now through `asRuleRows` |
| outbox insert | `db/propagation_enqueue.go:273` | unchanged |
| Change history groups | `db/cascade.go:33` (`cascadeGroupSources`), `:58` (audit stamp), `:419` | a bundle's entry carried its rule rows already (one cascade id); the entry is now named by the non-rule row, since rows written in one transaction share `created_at` and the first read could be the rule's |
| Change history card | `ui/src/app/operations/cascades/page.tsx` `CascadeCard`, `RuleFrom` | names the entry by `group.source_ref` (was the first write's); a rule's write in a bundle's entry reads "by the automatic rule from Trained in Laser Lab" |
| Pending changes rows | `db/propagations.go:836`, `:936`, `scanPropagations` `:1102` | unchanged read |
| Pending changes "Caused by" | `ui/src/app/governance/pending/page.tsx` `CausedBy` | a rule's row reads "Rule from Trained in Laser Lab" (A-21 §Q3; was "Trained gives Advanced"); the batch band is still one per edit, "These N changes come from one edit", since they are sent together |
| ledger reconcile on applied | `db/propagations.go:1075` | a rule revoke deletes no ledger row (rules write none), as a bundle's never did |
| pending deliveries, "waiting to be sent" marks | `db/pending_delivery.go:39`, `services/views.go:1564` | a rule's queued give now marks the person's rule reason as waiting; before, under a bundle's change, it marked nothing |
| member feed | `db/my_changes.go:112`, `handlers/my_changes.go:83`, `:102` | a rule-given role reads "came automatically, because you have Trained" instead of "because you're in Safety", and names no operator (rows of a rule never do) |
| request joins | `db/my_changes.go:95`, `:133`, `db/insights_trends.go:139` | request rows only; unaffected |
| drift classification | `services/drift/sweep.go` | reads Syndra's records (ledger, bundles, rules), not the outbox source; unaffected |
| `internal/access` | | no outbox reads |

Not done: a rule's own edit attributing a chain hop to the rule that reaches it (it names the edited rule, so Pending changes says "Rule from" that rule's first role for a two-hop row); Pending changes' separate batch per rule (A-21 §4.3 "These 2 changes come from one rule"), which would split one send-together edit in two; recording the triggering role apart from the rule id, so a rule whose first role is later edited renames its old rows (the member feed and Change history already read the rule's current first role).
