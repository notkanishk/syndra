# Reconciliation is a three-way merge, and Syndra only keeps two of the three

## What is wrong

Reconciliation compares two things: what Syndra wants (`OURS`) and what the
target has (`THEIRS`). Every difference between them is therefore ambiguous, and
the system resolves that ambiguity by guessing — consistently, in one direction,
and without saying that it guessed.

A difference has four possible causes and the two-way diff cannot tell them
apart:

| What happened | What Syndra concludes today |
|---|---|
| Syndra changed it and the target has not caught up | converge — correct |
| Somebody changed it **on the target** | converge — **overwrites their change** |
| Both changed it, to the same value | converge — a write that changes nothing |
| Both changed it, differently | converge — **silently discards one side** |

Rows two and four are the same operation as `git push --force`. The system's own
design forbids exactly this reasoning elsewhere: a capped read must not conclude
absence, a stale read must not produce findings, an unreachable target must not
look clean. Concluding "the target is wrong" from a difference whose cause is
unknown is the same error, and it is currently the default.

The consequences are not theoretical. A binding whose account had been deleted
on the target planned as *create*, and the sweep queued it every six hours — an
account somebody deliberately removed, waiting to be recreated. That was fixed
by refusing to converge it, which is right, but it is one hand-cut case of a
general shape.

## The shape it already has

Syndra is most of the way to the merge model and does not name it:

| Merge concept | What it already is |
|---|---|
| `OURS` | the desired-state snapshot, versioned per `(subject, target)` |
| `THEIRS` | the add-on's current read |
| dry-run merge | `/plan` |
| non-fast-forward rejection | the plan fingerprint, refused if the subject moved |
| commit history | the add-on's mutation log |
| a ref that may only fast-forward | the log anchor, which refuses a rewritten head |

What is missing is the **merge base**: the state Syndra last applied *and
observed*. Without it every difference is a two-way diff, and a two-way diff
cannot produce a conflict — only a winner.

## What changes

Record the base — and **first, make there be one to record.** This proposal
originally said the add-on already reads the account back at every successful
apply. It does so when it CREATES an account; when it updates one that exists it
builds the answer from the values it requested and fingerprints that, so the
dominant path reports intent in the shape of an observation. Reading back every
successful mutation is therefore part of this change rather than an assumption
of it, and until it lands there is nothing here worth storing. The base is then
one column, written from the observed values the response carries.

Then classify instead of overwrite:

- **unchanged** — `THEIRS == BASE`, `OURS == BASE`. Nothing to do, and no write.
- **fast-forward** — `THEIRS == BASE`, `OURS` moved. Apply. This is the ordinary case.
- **already merged** — `THEIRS == OURS`. Somebody did it by hand and got it right; record the base and write nothing.
- **theirs-only** — `OURS == BASE`, `THEIRS` moved. This is drift, and it is the case the drift queue exists for. A **durable finding**, deduplicated per field: it is what a hand edit on the NAS looks like, and left as the output of the pass that found it, it is visible to whoever ran that pass and to nobody else.
- **conflict** — both moved, differently. **Never resolved by a sweep.** An operator chooses, and the choice is bounded by where desired state can actually live: keep Syndra's intent always; adopt the target's value only where a per-subject decision can express it — a restrictive lifecycle value becomes a deny allowance, which gives the hand edit an author, a reason and an end date; otherwise the finding names the role mapping that produces the value and how many people it reaches, and the resolution is an edit to that policy. No per-subject additive grant is invented to make the dialog convenient.
- **deleted upstream** — `THEIRS` is gone and `OURS` still names it. Today's stale binding, now a named state rather than a special case.

A conflict is not a failure. It is the system declining to guess, which is what
every other rule in this design already does.

## Why this is worth doing

It replaces a silent policy with a visible one. Today an operator cannot tell,
from any surface, whether a convergence is about to apply their decision or
overwrite somebody else's — and the audit record afterwards says only that a
value changed.

It also makes the manual-change case legitimate. Somebody fixing a share by hand
during an incident is currently in a race with the sweep; under a merge model
their change is either adopted, or raised as a conflict, and never quietly
reverted six hours later.
