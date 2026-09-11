# Tasks

## 1. The rule

- [x] 1.1 Three kinds of fact written down — observed, derived, recorded — with
  the property that no path turns a recorded or derived fact into a
  presentable observed one
- [x] 1.2 State is INTERROGATED, never reconstructed. Events write nothing;
  they say a question is worth asking sooner, and carry attribution a query
  cannot see

## 2. Shipped

- [x] 2.1 A cache may not authorise skipping a mutation. The drain asks Zitadel,
  never the local index
- [x] 2.2 A revocation Syndra performs maintains Syndra's own index
- [x] 2.3 The person page answers each role from Zitadel, with three states —
  present, absent, unknown — where a failed read may never render as an absence
- [x] 2.4 One pipeline per fact. The duplicate holder count deleted, guarded per
  query so counting bundle ASSIGNMENTS stays legal
- [x] 2.5 The drain clears the claim envelope it invalidated. It was cleared
  only by webhooks, and the self-mutation guard drops Syndra's own changes — so
  a revoke left the old role in newly issued tokens for up to a day
- [x] 2.6 The directory's verdict disables the accounts Syndra manages, failing
  safe so one unreadable lookup cannot disable everybody
- [x] 2.7 A trigger's OUTCOME is verified, not its delivery. Onboarding
  propagates real faults, names "nothing to give" apart from "something broke",
  and answers "who joined and got nothing" from state
- [x] 2.8 Read-back after write. `confirmed_at` is a second fact; an unobserved
  write is never failed or retried; age at twenty minutes makes it a finding

## 3. Not done, in order

- [x] 3.1 The observation store and its periodic sweep — what a read saw about
  everything, not only about a write just made. `internal/observe`, `db.Observation`,
  wired in `cmd/api/main.go` at `OBSERVE_SWEEP_INTERVAL` (default 5m)
- [ ] 3.2 Counts and dashboard tiles read verdicts rather than records
- [x] 3.3 Only the observer may call Zitadel; delete the direct reads from
  surfaces and guard against their return. Done, 2026-09-11: the webhook no
  longer writes `zitadel_grants_index` from event payloads (it re-observes
  the affected person instead — `observeAfterEvent`, webhook.go), and
  discovery.go's two grant-listing routes (`/zitadel/grants`,
  `/zitadel/users/{id}/grants`) observe and answer from the store, carrying
  `observed_at`/`complete` through to the person page and the reconciliation
  list. `repoguard.TestOnlyTheObserverListsZitadelGrantsLive` guards it, with
  the outbox drain's PRE-FLIGHT read (`liveUserGrantRoles`, which concludes only
  presence) and the governance reachability probe exempted by argument.
  **Finished, 2026-09-11 ("The last two readers"):** the drift sweep
  (`drift/sweep.go`) no longer pages Zitadel at all — it reads
  `db.LatestOrgObservation` + `db.AllObservedGrants`, cites the org
  observation's `observed_at` on every finding it writes
  (`drift_items.zitadel_observed_at`, migration 000049), refuses a clean
  bill (`markReconciled`) from `ErrNoObservation` or an incomplete
  observation, and now runs on the observer's own cadence
  (`observeInterval()`, ~5m) instead of a dedicated 6h schedule, gated behind
  `awaitFirstObservation` so it never runs before the first sweep. Addon
  reconciliation (`drift/addon.go`, `ReconcileAddon`) is a different reader
  with different failure modes and keeps its own 6h `driftInterval()`.
  `handlers/reconciliation.go`'s on-demand diff now calls `observe.Org`
  (recording a fresh observation, shared rather than discarded) and reads
  `db.AllObservedGrants` back, same as discovery.go's routes.
  `repoguard.TestOnlyTheObserverListsZitadelGrantsLive` has one exemption
  left: `zitadel_grant_lookup.go`'s webhook grant-lookup fallback, a
  single-purpose read that enriches one event rather than answering "how
  many people hold this" — tracked in NEXT.md.
  **Also added:** a drift finding sourced from the sweep (no direct upstream
  evidence) derives, at read time (`db.driftItemSelect`, never stored),
  whether an event exists for its grant aggregate in `webhook_events` (a
  prefix match on `idempotency_key`) — `attribution_unavailable` when none
  does, and the stronger `event_possibly_missed` when the event log's own
  oldest held row is older than the finding's `detected_at` (otherwise the
  log simply does not reach back far enough to say so). Scoped to
  `drift_items` only — the self-mutation guard means Syndra's own grants
  never have an event, so running this over the observation store would
  flag every grant Syndra makes.
- [x] 3.3a A pending `target_only` row whose grant Zitadel no longer holds at
  all did not close — only `retractExplained` closed rows, and only when
  Syndra's own state explained the grant, so a grant removed out of band
  (`zitadel.grant_removed`) left its finding open forever (prod: "47 items"
  where Zitadel held 46 unexplained). Fixed 2026-09-11: `drift/sweep.go`'s
  `closeGoneDrift`, called only past the truncated-read guard, closes such a
  row to `db.DriftResolved` ("resolved") via the new `db.CloseGoneDrift`
  (migration 000050 adds the status). Tests:
  `TestSweep_ClosesAFindingWhoseGrantHasVanished`,
  `TestSweep_LeavesAFindingWhoseGrantIsStillPresent`,
  `TestSweep_TruncatedObservationClosesNothing` (mutation-sensitive on the
  completeness guard).
- [x] 3.4 The truncation limit is stated AND enforced where it decides
  something: a revoke may only be confirmed from a complete answer. The
  pre-flight stays capped because it concludes presence, where a miss costs a
  call that 409 absorbs; the confirmation concludes absence and cannot

## 4. Holding is observed (2026-09-11 production walk)

- [x] 4.1 `RoleHolderFacts` returns `HolderFacts{Given, Confirmed, Observed,
  Basis}` from one snapshot; `ObservedHolderCounts` counts whoever Zitadel
  shows. Users, projects, roles, apps and role-members responses carry
  `observed_*` fields and the observation basis. `is_unused` requires
  observed == 0. Test: `TestRoleHolderFacts_ObservedCountsWhoeverGaveIt`.
- [x] 4.2 Every UI holder/access number reads observed; recorded counts only
  explain (`ui/src/lib/holders.ts`, one helper for every surface).
- [x] 4.3 `db.ConfirmFromObservation` stamps `confirmed_at` from a complete
  listing (adds by presence, revokes by absence); called from
  `observe.Sweep` only past the completeness guard. Live test:
  `TestConfirmFromObservationStampsWhatTheIndexShows`.
- [x] 4.4 Migration 000050 down is `NOT VALID` so a rollback keeps rows the
  sweep closed as gone.
- [x] 4.5 A member's own page reads what Zitadel holds for them through the
  same observer route the operator page uses (`GET /zitadel/users/{id}/grants`
  is now self-or-operator). One reader, one answer, both audiences. Test:
  `TestZitadelUserGrantsRoute_SelfReadableByMember` (mutation-checked).
- [x] 4.6 A revoke overtaken by a later delivered add for the same roles is
  settled by that add, not "unseen": `ConfirmFromObservation` stamps it, so
  the Home queue stops listing withdrawals whose absence can never be seen
  again. Live test: `TestConfirmFromObservationSettlesARevokeOvertakenByALaterAdd`.
  The SQL also enforces the completeness guard itself (latest org
  observation complete and error-free) rather than trusting the caller.
- [x] 4.7 Found by the walk, proven by read-back: a direct grant to a person
  who already holds a grant on that project came back "applied" and "Not in
  Zitadel". The drain sent AddUserGrant, Zitadel answered 409 (one grant per
  user+project), and 409 is absorbed as idempotent success. An add now reads
  the live grant and UPDATES it with the union of roles (`liveUserGrant`, the
  one read behind add and revoke). Test:
  `TestDrain_AddMergesIntoAnExistingGrant` (mutation-checked).
- [x] 4.8 Grant dialog's "N people hold it" and Pending changes' CAUSED BY
  read observed holders and name a direct grant "Given by hand" — no more
  "Automatic rule" on a grant somebody chose by hand.
