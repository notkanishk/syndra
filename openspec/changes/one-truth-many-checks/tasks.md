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
- [x] 3.4 The truncation limit is stated AND enforced where it decides
  something: a revoke may only be confirmed from a complete answer. The
  pre-flight stays capped because it concludes presence, where a miss costs a
  call that 409 absorbs; the confirmation concludes absence and cannot
