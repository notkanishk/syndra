# Architecture deepening: candidates from the 16 Oct review

Each is a card in `review-2026-10-16.html`. Take one at a time: grill it, record the decisions in `design.md`, then build.

- [ ] 1 **One access-change planner** (Strong) [needs-triage]. An operation returns its outbox writes; rehearsal runs it dry, apply commits the same writes, and the guards (role offered, already held) live inside it. Replaces `cancellingWrites` (`services/bulk.go`) and the hand-built decision params (`handlers/requests_bulk.go`, `handlers/access.go`). A plan carries the exact queued changes it names, so apply admits only those. Overlaps NEXT §4b "untraced rule propagation" (enqueue the closure delta).
- [ ] 2 **One role-standing module** (Strong) [needs-triage]. Merge `effectiveClosure` and `collectUserRoles`; one standing per role (held, waiting to be sent, removal queued, not held) with its reasons; the UI reads it through one helper. Fix `sourcePhrase` testing "rule" where the producer emits "mapping".
- [ ] 3 **TrueNAS add-on: apply executes the change plan decided** (Strong) [needs-triage]. A pure decide returns a Change; plan serialises it, apply executes it through a NAS module of named operations over the existing rpc seam. Add a plan-equals-apply parity test.
- [ ] 4 **The plan gate as one surface interface** (Worth exploring) [needs-triage]. Gate owns fingerprint, issue, cohort guard, claim, staleness and the apply loop; each surface fills Rehearse and Apply. One apply protocol.
- [ ] 5 **A few store interfaces instead of ~290 function vars** (Worth exploring) [needs-triage]. Postgres and in-memory adapters; safety checks stay on in tests. After 1 and 4.
- [ ] 6 **Declare the backend and add-on contract once** (Worth exploring) [needs-triage]. Shared message types, a golden fixture per message, field meaning read from the manifest.
- [ ] 7 **One row-action module for decide, mark, roll back** (Speculative) [needs-triage]. Four copies in JSX today; the Motion 2a sweep would land once.
