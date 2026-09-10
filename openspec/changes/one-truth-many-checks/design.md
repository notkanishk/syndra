# One truth, many checks

## What actually went wrong

A grant was reported delivered that had never been sent. Chasing it found a
stale cache, a skipped API call and a dead webhook pipeline — but those are
symptoms. The defect underneath is that **the system had no way to distinguish
what it had SEEN from what it had WORKED OUT**, and so it rendered the second as
the first.

Everything else follows from that. A cache could be treated as evidence because
nothing in the type said it was not. Two surfaces could disagree because each
inferred privately. A dead webhook could go unnoticed because nothing measured
the age of what it fed.

## Three kinds of fact, and only one of them is true

| Kind | Meaning | Can it be called true? |
|---|---|---|
| **Observed** | read from Zitadel at a stated time | Yes, as of that time |
| **Derived** | computed from observations and records by a pure function | Only as true as its inputs |
| **Recorded** | what Syndra decided | Never — it is intent |

The rule the product has to enforce, mechanically:

> **No path exists by which a Recorded or Derived fact becomes presentable as
> Observed.**

Not a convention. Not a review checklist. A property of the types, checked by a
test that enumerates every case.

## The verdict

One value, per (subject, project, role), carrying its own provenance:

```
Verdict {
  decided     Reason?      // direct grant | bundle vN | rule R — or nothing
  observed    Present | Absent | Unknown
  observedAt  Time?        // absent when Unknown
  owed        Queued | InFlight | None   // from the outbox
  state       State        // a TOTAL function of the four above
  basis       []Input      // what produced it, for the screen and the audit
}
```

`state` is a total function. Every combination has a name and there is no
default branch:

| decided | observed | owed | state |
|---|---|---|---|
| yes | Present | — | **In force** |
| yes | Absent | Queued/InFlight | **Not yet sent** |
| yes | Absent | None | **Undelivered** — a defect, raised as a finding |
| no | Present | — | **Unexplained** — drift |
| — | Unknown | — | **Unconfirmed** |
| no | Absent | None | *nothing* |

**No row yields "In force" without a fresh `Present` observation.** That single
property is the guarantee: the process cannot conclude somebody has access
unless Zitadel said so, whatever else is true, however stale the records.

`Undelivered` is the state today's failure occupied, and the product had no name
for it — which is why it rendered as "Granted".

## Freshness is an input, not a footnote

An observation without an age is a rumour. `observedAt` is required, and each
surface declares the age beyond which it will not rely on one. Past that, the
observation **degrades to `Unknown` automatically** and the verdict becomes
`Unconfirmed`.

This is what makes a stale cache harmless: it cannot be rendered as current,
because currency is computed rather than assumed. The whole class of defect that
produced today's failure ends here, structurally.

The vocabulary already exists — `ReadFreshness` has live / ageing / stale /
provisional and a rule about what may act on which. The change is to move it
from one component into the data model.

## Many checks, at the three moments that matter

"Deterministic inference with multiple live checks" means the same question is
asked of Zitadel at every point where being wrong is expensive:

**1 · Before a write.** Is this change still needed? The drain does this now.
Its only job is avoiding a redundant call, and it must never be able to
*conclude* a write happened.

**2 · After a write — the one that is missing.** A 2xx is an acknowledgement of
receipt, not evidence of state. The drain marks `applied` on the response. It
should mark `applied` only after a **read-back that observes the grant**, and
record that observation. Today's failure would have been caught in the same
second rather than a day later by a person looking at a screen.

This pattern is already in this codebase, one plane over: `apply.go` reads back
after `user.create`, because "a fingerprint computed from a state this add-on
invented is a fingerprint the next plan verifies against nothing." The Zitadel
plane never got the same treatment.

**3 · Periodically.** A sweep that refreshes observations so surfaces read
something recent without each one calling Zitadel. This is what makes a single
observation store affordable.

## Webhooks are a freshness hint, never a source

They were load-bearing and silent: the grant index was written by webhooks and
by nothing else, and none have arrived on this deployment in an hour — none
at all, ever, by the look of the events screen.

Under this design their absence is survivable and visible. Observations age;
aged observations degrade; degradation is stated on screen. And the silence
itself becomes a health fact worth reporting: *no event has arrived since X;
what you are reading was confirmed by the periodic read at Y*.

## Every degradation names itself

A fallback that does not say what broke is a lie with better manners.

| What broke | What the surface says |
|---|---|
| Zitadel did not answer | Unconfirmed since 04:12 — Zitadel last answered at 03:58 |
| Events stopped arriving | Live updates stopped at 07:43; this was confirmed by the sweep at 04:00 |
| The read hit its cap | 5000 of more than 5000 seen — absence cannot be concluded |
| Never observed at all | Not checked yet |

None of these is "unavailable".

## What proves it, mechanically

1. **Enumeration test.** Every (decided, observed, owed, age) tuple, asserting
   no tuple yields *In force* without a fresh `Present`. A total switch with no
   default, so a new state cannot be added without deciding its verdict.
2. **No surface reads Zitadel.** Only the observer may. Source guard.
3. **No surface renders `decided` without `observed`.** Source guard.
4. **One aggregation.** Counts are sums over verdicts, never their own query —
   the guard for this already exists and generalises.

## Order of work, by risk

1. The verdict type, the total function, the enumeration test. Pure addition,
   no behaviour change.
2. Read-back after write in the drain. Highest value: it closes the exact class
   that failed, at the moment it fails.
3. The person page reads verdicts. It is already close.
4. Counts become aggregations of verdicts.
5. The observation store and its periodic refresher; webhooks demoted to a hint.
6. Delete the direct Zitadel reads from surfaces, and guard against their return.

Each step is shippable and leaves the product more honest than it found it.

## Costs, stated

- A read-back doubles the calls on the write path. It is worth it; writes are
  rare and being wrong about one is expensive.
- A periodic sweep costs one paged listing per interval, and bounds how stale
  any surface can be.
- Absence still cannot be concluded from a truncated read. That stays a stated
  limit rather than something smoothed over, because smoothing it is how a
  capped read becomes a claim about everybody.
- Observing costs more than inferring. That is the trade being made deliberately:
  the cheaper answer is the one that was wrong.

---

# State is interrogated, never reconstructed

The owner, on reading the webhook safeguards:

> this essentially puts a lot of belief in zitadel just telling us something
> happened and that info builds our state rather than we explicitly asking
> zitadel what the state is

That is the distinction, and it is the one the earlier draft missed. Sequence
tracking, gap alarms and heartbeats all make a RECONSTRUCTION more trustworthy.
They do not make it an OBSERVATION.

## Reconstruction versus interrogation

| | Reconstruction (events) | Interrogation (query) |
|---|---|---|
| State is | rebuilt from a stream of notifications | the answer to a question we asked |
| Errors | accumulate and persist silently | cannot accumulate; the next answer replaces the last |
| A failure is | silent, permanent, indistinguishable from quiet | loud, transient, fixed by the next call |
| Requires trusting | the completeness of a stream we do not control | that Zitadel answers correctly about itself |
| Verifiable | only by interrogating | by construction |

The last row settles it. **A reconstruction cannot be verified except by
interrogating.** So if correctness requires interrogation anyway, the events
were never buying correctness — only latency.

## The inversion

**Events never write state.** At most they say a question is worth re-asking
sooner than scheduled.

That single rule removes whole categories of failure rather than instrumenting
them:

- A missed event costs LATENCY, never correctness.
- A misread or malformed event cannot corrupt anything, because nothing it
  carries is stored as fact.
- Sequence gaps and dedup stop being correctness machinery. They remain useful
  for observability and nothing depends on them.
- The dead webhook pipeline degrades to "we refresh on the schedule" — which is
  what is happening right now, and would have been harmless if state had been
  interrogated.

The defect that started all of this — a cache asserting a grant existed —
becomes unreachable. The store can only ever hold answers Zitadel gave to
questions Syndra asked, each stamped with when it asked.

## What events are still genuinely for

Two things a query cannot provide, and they are worth keeping:

**Attribution.** A poll sees state; it cannot see who changed it. The event
carries `EditorID` and `EventCreatedAt`, which is how a drift row can say
"created in the identity provider on 21 Jul by svc-badge-sync" instead of only
"found nine days ago". That is real value for triage and it is unreconstructable
from state alone.

**Latency.** A change upstream can prompt an immediate re-read instead of
waiting for the next sweep.

Both are enrichment. Neither is a source. An event that arrives is a reason to
ask; the answer is what gets stored.

## What this costs, and where it stops working

Interrogation must be affordable at the frequency correctness requires. At this
deployment's scale — a few hundred people, a handful of projects — a full grant
listing is one paged call, and doing it every minute is not worth optimising.
Per-person on demand is cheaper still.

The answer flips somewhere around the point where a full listing stops fitting
in a sweep interval — tens of thousands of grants. At that scale you are forced
back toward a maintained projection, and you pay for it with the machinery this
document just removed. Designing for it now would be buying that cost with no
benefit.

**The trust that remains, stated plainly:** that Zitadel answers correctly about
itself, and answers COMPLETELY. The second is not free — the current org-wide
read caps at 5000 and reports truncation. A truncated answer can confirm
presence and can never conclude absence, and that limit stays visible rather
than smoothed away.

---

# Sequencing, and what is deliberately not done yet

Done, in this order and for this reason:

1. **The claim cache is invalidated by the drain**, not by a notification. A
   cache is a projection of state; whatever changes the state clears it.
2. **The directory's verdict reaches the accounts Syndra manages.** A third
   cause of a disabled account, fail-safe in the direction where a failed
   lookup cannot disable everybody at once.
3. **A trigger's outcome is verified rather than its delivery.** Onboarding
   propagates real faults, distinguishes "nothing to give" from "something
   broke", and answers "who joined and got nothing" from state.

**Read-back after write — DONE, 2026-09-10.** What follows is why it needed
its own change rather than being the tail of the last one. The naive
version is a hazard rather than a safeguard: Zitadel's read path is a
projection over its eventstore, so a read issued immediately after an accepted
write can legitimately not see it yet. A read-back that treats "not observed"
as "not applied" would fail good writes, retry them, and turn a lagging
projection into duplicate work.

So it needs two facts where the row currently holds one:

- **accepted** — Zitadel returned 2xx. What `applied` means today.
- **confirmed** — a read has since observed the grant. What nothing records.

A write becomes accepted immediately and confirmed when an observation catches
up, with the gap between them visible rather than assumed away. That is a
schema change and a vocabulary change on a screen operators already read, which
is why it is its own change rather than the tail of this one.

The failure that started all of this would have been caught either way: the
grant was never accepted, because no call was made.


## Read-back, as built

`propagation_outbox.confirmed_at` (migration `000047`) holds the second fact.
Deliberately nullable and deliberately not part of `status`: accepted and
unconfirmed is an ordinary, temporary state, not a fault.

The drain reads Zitadel back after every accepted write. An add is confirmed by
PRESENCE, a revoke by ABSENCE — transposing them would confirm exactly the
writes that failed, so each direction has a test that fails when it is.

**An unobserved write is never a failure.** It is not retried, not requeued,
not marked failed. It stays applied and waits to be looked at again. This is
the whole hazard the design named: a read-back that failed the row would retry
a good write every time Zitadel's projection lagged, and turn latency into
duplicate work.

**Age is what makes one a finding**, at twenty minutes — long enough that a
projection catching up, or a burst of onboarding writes, never reaches a
screen; short enough to bound how stale "in force" can be claimed. Surfaced on
Home as a block that says what happened, what it is NOT (neither a failed
change nor one still waiting to be sent), how old it is, and where to look.

Proven rather than asserted. Five unit tests and four live tests against real
Postgres, each mutation-checked: disabling the read-back fails two, confirming
a revoke by presence fails one, dropping the age filter fails one, confirming
regardless of status fails one, and applying after confirming fails one.

**That last test exists because the first version of this recorded nothing.**
`MarkPropagationConfirmed` is guarded on `status='applied'` so a confirmation can
never resurrect a failed or superseded row. It was called BEFORE `markApplied`,
while the row was still `in_flight`, so the guard matched nothing: zero rows
updated, no error returned, the observation made and thrown away.

Neither layer of tests could see it. The unit tests stub the seam, so the SQL
guard never ran; the live tests seeded a row that was already applied, so the
sequence never ran. Two halves, each correct about itself — the recurring defect
this codebase already names, met while fixing an instance of it. The order is
now asserted directly, which is the one thing neither half was checking.

The ordering that replaced it respects the rule it looked like it broke.
`markApplied` comes last AFTER DURABLE SIDE-EFFECTS: a row whose ledger or store
write did not land must stay in_flight and be reclaimed. Neither the confirmation
nor the claim invalidation is one of those — the first annotates the row itself
and the second is a cache, and losing either leaves a state that is honest and
self-correcting.

## What is left

- ~~**The observation store.**~~ Built: `internal/observe`, `db.Observation`,
  wired in `cmd/api/main.go` at `OBSERVE_SWEEP_INTERVAL` (default 5m).
- **Surfaces read verdicts.** The person page already answers per role from
  Zitadel; counts and tiles still derive from records alone.
- ~~**Delete the direct reads and guard them out.**~~ Partly built,
  2026-09-11: the webhook re-observes the affected person instead of writing
  `zitadel_grants_index` from event payloads, and discovery.go's two
  grant-listing routes (`/zitadel/grants`, `/zitadel/users/{id}/grants`)
  observe and answer from the store, carrying `observed_at`/`complete`
  through to the person page and the reconciliation list.
  `repoguard.TestOnlyTheObserverListsZitadelGrantsLive` enforces it from here
  on, with two exemptions argued in the guard itself — the outbox drain's
  PRE-FLIGHT read (`liveUserGrantRoles`: a capped single-page check asking only
  whether a call is still needed) and the governance reachability probes
  (`Limit:1`, result discarded).

  The pre-flight may stay capped because it concludes only PRESENCE: a page that
  misses a grant means the call proceeds, and a 409 absorbs it. The drain's
  READ-BACK is a different question and does not share the exemption — it
  concludes ABSENCE when confirming a revoke, so it goes through the observer,
  which reports whether the answer was whole. A capped read that happened not to
  include the grant would otherwise have confirmed a revocation that may never
  have happened. Still reading Zitadel live, as KNOWN GAPS in
  the same guard: reconciliation's own diff, the drift sweep, and the
  webhook's grant-lookup fallback — all pre-existing and out of this pass's
  scope.
