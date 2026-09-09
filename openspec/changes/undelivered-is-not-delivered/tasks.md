# Tasks

## 1. The queue

- [x] 1.1 `withdrawUndelivered` (`db/withdraw.go`) cancels a `pending` add for
  the exact triple this revoke answers, from the same source, and reports
  whether a revoke is still owed. `superseded` with a stated reason — the row
  stays, because the record of what was asked for is not the queue of what is
  still owed
- [x] 1.2 Called from BOTH enqueue chokepoints — `enqueueCascadeRows` and
  `enqueueWrites` — rather than from the bundle handler that surfaced it. Every
  cascade routes through one of the two, so bundle removal, bundle deletion,
  removing a role from a bundle, a rule change and a direct revoke all get it
  without any caller having to remember
- [x] 1.3 `in_flight` is never cancelled. It may already be with Zitadel, and
  its effect still has to be undone
- [x] 1.4 A row carrying more roles than the one being taken away is left
  alone: cancelling it would withdraw deliveries nobody countermanded

## 2. The screens

- [x] 2.1 `RoleReason.queued` — per SOURCE, not per role. A role delivered
  directly and queued by a bundle is one the person genuinely has, and a
  per-role flag would have to be wrong about one of the two
- [x] 2.2 Marked in `ExplainUserAccess`, not in `collectUserRoles`. The drift
  detector and the entitlement resolver call the same collector and want the
  records exactly as recorded — and neither should pay a per-user query for a
  marker it never renders
- [x] 2.3 The person page shows "Waiting to be sent" in the slot that otherwise
  dates the access, when EVERY source of a role is queued. An expiry on access
  that does not exist yet answers a question nobody can ask
- [x] 2.4 The removal dialogs stop claiming a loss that cannot happen: "Nothing
  to take back · recorded, never sent", for both the bundle and direct paths
- [x] 2.5 Manage bundles reports what is actually WAITING, summed from the
  backend's `enqueued`, instead of assuming it from the number of boxes ticked.
  A removal that withdraws undelivered rows now says the queue is unaffected
  rather than instructing somebody to go and confirm an empty screen
- [x] 2.6 The project header stops sending unsent grants to Drift. "Not in
  Zitadel yet — these changes have not been sent" where every role is queued;
  the drift pointer stays for the case that IS drift. A bundle's own grant is
  not drift, so the old pointer ended in an empty screen and an operator
  concluding the drift report was broken

## 3. Three things found on the way

- [x] 3.1 Assigning or removing a bundle never invalidated the pending-changes
  queries, so the nav count and that screen stayed a step behind an edit made
  from the person page itself
- [x] 3.2 `Chip` is `inline-flex`, so the `{" "}` between a bundle's name and
  its version was collapsed by flex layout — "Ops Adminv1". A space cannot
  survive that boundary; a gap can
- [x] 3.3 Unfinished revocations rendered its rows in an unpadded `<ul>`, so
  names ran into the card's left border and the `c_` handles clipped on the
  right. The empty state one line above had the padding, so it only showed once
  the list had something in it

## 4. The harness this needed

- [x] 4.1 `internal/db` gains a live-database harness (NEXT.md §4b), skipped
  when `SYNDRA_TEST_DATABASE_URL` is unset so `go test ./...` stays green
  without a Postgres. It migrates the target database and truncates between
  cases
- [x] 4.2 Seven cases for the withdrawal rule, all written from the expensive
  direction: what must STILL be revoked
- [x] 4.3 Mutation-checked against the real database. Allowing `in_flight` to
  be cancelled, ignoring the source, and dropping the applied-delivery check
  each fail exactly one case and no others

## 5. The same defects, everywhere else they occur

Found by auditing all 77 mutation call sites rather than waiting for the next
screenshot. Five failure modes, each one first seen on the person page.

- [x] 5.1 **`["propagation"]` matched nothing.** Three `invalidateQueries` calls
  used the singular; the real key is `["propagations", "pending"]`. Each carried
  a comment stating the intent the code failed to achieve — "what changed is the
  pending count" — so Pending changes never refreshed after a converge apply, a
  mapping create or a version rollback. Guarded now by a source scan, because
  the failure is silent and the code reads correct
- [x] 5.2 Ten further mutations did not invalidate what they changed: a mapping
  rule that moves dozens of people refreshed only the rule list; drift
  resolutions left a stale unexplained badge on the People list; deleting a
  bundle skipped the queue its own single-holder sibling refreshes
- [x] 5.3 **Ten dialogs stayed armed after they succeeded.** Worst was
  `UnexplainedAccess`: "Resolved. It will not be listed again." printed above
  the item's own name with a live red Revoke underneath. Every fix keeps the
  control after a FAILURE — retiring on any outcome would leave a refusal on
  screen with no way to try again
- [x] 5.4 **Two shared components carried a defect twelve times each.**
  `PlanReview` rendered "Every selected person will change" beside a disabled
  button reading "Nothing to apply"; `BulkDialog` never passed
  `notReadyReason`, so five distinct causes of a dead submit all showed as
  silent grey — the dead end that prop's own doc comment exists to prevent
- [x] 5.5 About twenty-five disabled controls gave no reason, against
  `Button.tsx`'s stated rule. The asymmetric ones were the worst: only the
  first of three lifecycle buttons explained itself, from a condition all three
  shared
- [x] 5.6 `danger` red on buttons that GRANT access, beside revoke buttons
  styled identically. The `/zitadel/*` pages had quietly given the variant a
  second meaning — "this writes straight to Zitadel" — which is what made the
  pair unreadable
- [x] 5.7 **`Button` had two render roots.** A bare `<button>` without a
  reason, a wrapped one with: the element type at that position changed the
  moment a reason appeared, so React remounted the control while somebody was
  typing beside it — focus lost, pending press dropped. Four tests had
  independently worked around it by re-querying the node and none reported it.
  It matters more after this sweep, which adds a reason to twenty-five controls
- [x] 5.8 The member's storage page promised something the add-on refuses.
  Two banners disagreed about whether a password applies while a target is
  paused; the first reconciliation replaced one false sentence with another —
  that it would apply once changes resume. `draining` and `read_only` refuse
  every new mutation, so nothing is held and replayed. Somebody told to try is
  worse off than somebody told to wait: they read the refusal as their own
  mistake
