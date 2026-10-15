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

### Open: the frontier when this resumes

Ask these in order. Each needs the facts re-read first, because the design build will have moved the code.

1. **Facts to refresh.** Where `waits_for_send` and `wait_reason` were built, and how many places check them. Whether `confirmation_mode` (global, per bundle, per rule) and `?apply=true` are gone. The Q4 fix is held: confirm production still has no mapping rules, and ship it before the first one. Re-run the preview/apply table in `review-2026-10-16.html` card 1.
2. **The Change.** Its fields: the writes (each with delivery and wait reason), a refusal, the cancelled queued changes, the plan-row words, and the fingerprint. Does an extend (a new end date) count as a write or as a ledger edit?
3. **The snapshot.** Exactly what is read under the lock: base holdings, rule closure, queued changes, the role catalogue with `waits_for_send`, and the person's existing grants with their end rules. One read per action, or one per bulk plan?
4. **The plan row.** How it stores the cancel ids (decision 4) and the fingerprint inputs, and whether the `BulkOutcome` shape stays or each surface gets its own row type.
5. **The first slice.** Which action moves first. Recommended: bulk assign and remove bundle, the two with the highest drift.
6. **System work.** When expiry, rules from Zitadel events, and triggers join, and what replaces the webhook's `EnforceMappingRules` for grants made outside Syndra (drift, or a planned write).
