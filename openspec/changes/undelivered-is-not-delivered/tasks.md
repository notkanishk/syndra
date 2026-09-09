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
