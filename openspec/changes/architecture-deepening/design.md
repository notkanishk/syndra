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
- **Apply** (`services.ApplyPlannedBulk`) runs in one transaction under the lock: claim the plan (`db.ClaimPlanJoined`), read the snapshot again, plan every row again, and refuse the whole plan as stale (`services.StalePlan`, answered as the usual 409 `PLAN_STALE`) if any row's fingerprint or cancel ids differ. Rolled back, so nothing is written and the plan is not spent. Otherwise it admits only the stored cancel ids at the outbox (`db.CancellationsNamed`; the old `CancellationsReviewed`, which admitted every cancellation, is no longer called for these ops) and writes each row through `AssignBundleAndEnqueue` / `RemoveBundleFromUserAndEnqueue`. Delivery follows the commit, per row, through the same `projectUpstream` → `Deliver` as every other bulk op.
- **The plan row stores** `cancel_ids` in `plan_subjects.outcome_json` (no migration), shape-checked and provenance-checked like grant ids: each must be a queued change of that subject's.
- **Words.** Assign: "Joins the Safety bundle." then "Gains Member in Laser Lab and Trained in Laser Lab." (alphabetical), "Also gives … by the automatic rule from …", "Giving … waits for you to send.", and the cancels line last. Remove: "Leaves the Safety bundle." then "Loses …", "Keeps … another way.", "Also loses …, which the automatic rule from … gave.", "Removing … waits for you to send.". Nothing to send is said: "Already holds every role it gives, so nothing is sent to Zitadel." / "Keeps every role it gave another way, so nothing is sent to Zitadel." A queued removal row now says "A role it removes always waits for you" instead of "gives".

Not in this slice, and listed as tasks: delivery decided only by the planner (apply still asks `Deliver`, which reads the same `role_waits` table, and a change to it between plan and apply makes the plan stale); one outbox insert (bundle ops only use `enqueueCascadeRows`, so the merge was not needed here); rule-derived rows carrying `source='rule'` and the triggering role (decision 7: a bundle's rule rows are still attributed to the bundle, as before); plans grouped by delivery on screen; the bulk reason recorded on bundle ops; and every other action.
