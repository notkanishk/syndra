# Design: reconciliation as a three-way merge

## 1. The base is a read-back, not a belief

The merge base for `(subject, target)` is **the state the target reported after
the last successful apply** — not what Syndra intended to write.

The distinction is the whole design. Recording the intent would produce a base
that agrees with `OURS` by construction, and a base that always equals one side
can never produce a conflict: every difference would resolve as "the target is
wrong", which is exactly today's behaviour with more machinery.

**And `apply` does not currently perform that read on the path that matters.**
This section previously claimed it did, which was wrong and was the most
expensive kind of wrong: it made the base look like a storage change on a path
that already had the value.

`createAndConverge` reads back (`readBack(username)`) and fingerprints what the
target reported. `convergeExisting` does not: after `user.update` it builds
`applied := *current` and overwrites the managed fields with the REQUESTED
values, then fingerprints that projection (`addons/truenas/apply.go`). So the
dominant path — every update to an account that already exists — produces a
fingerprint of intent wearing the shape of an observation, and a base written
from it would be precisely the intent-as-base failure this change exists to
remove. The create path's own comment states the rule the update path breaks:

> a fingerprint computed from a state this add-on invented is a fingerprint the
> next plan verifies against nothing

That fix is a prerequisite rather than a consequence, and it shipped ahead of
this change as `apply-reads-back-what-it-wrote`. What it established, and what
this design may now rely on:

**Every successful mutation MUST be followed by a read of the account as the
target then holds it.** Both paths converge on `readBack`, and the fingerprint
is computed from the read on both. The plan path already reads every subject, so
this is the same call the add-on makes twice a pass, not a new class of work.

**The base travels in the apply response, as observed values.** The outcome
carries an `observed` map of managed field to the value the target reported,
beside the fingerprint it already carries. The backend stores that map as the
base for `(subject, target)`; it never derives one from what it asked for.

The field is additive, and its safety is asserted rather than assumed: the
request direction is decoded strictly by the add-on, the reply direction is not,
and the backend's own decoder is tested against an outcome carrying `observed`
and `unverified`. There is no `apply_response.json` artifact yet — the contract
fixtures cover the request direction only. Adding one belongs with the consumer
that reads these fields, and until then the decode test is what holds the two
sides together.

**A read-back that fails does not produce a base.** The write happened; that is
reported as it is today. What must not happen is a base recorded from intent
because the read failed — that is the failure mode this whole design is about,
arriving through the error path. The outcome is `applied` with no `observed`,
the backend records no base, and the subject is treated exactly as one that has
never been applied: the next pass converges it two-way, as §8 describes. An
add-on too old to send `observed` lands in the same state, which is why absence
is legible rather than an error.

## 2. Fields, not accounts

The merge is per **field**, not per account, because entitlements are per field
and a whole-account comparison manufactures conflicts. An operator changing a
group membership while a member sets their own password must not produce a
conflict: those are different fields and neither touched the other.

This also bounds what a conflict can be about. Only fields Syndra manages for
that subject participate — an unmanaged field is not "unchanged", it is out of
scope, and the distinction already exists (`desired.managed[...]`).

## 3. The six outcomes

For each managed field, with `B` the base, `O` ours, `T` theirs:

```
T == B, O == B   unchanged        write nothing
T == B, O != B   fast-forward     apply
T == O           already merged   record the base, write nothing
O == B, T != B   theirs-only      drift; operator triages
T != B, O != B, T != O   conflict operator resolves
T absent, O present      deleted upstream
```

`already merged` is checked before `theirs-only`, deliberately: somebody who
made the change Syndra was going to make has not drifted, and telling them they
have is how a system trains people to ignore it.

## 4. What a sweep may do

A sweep may apply `fast-forward` and record `already merged`. It may **not**
resolve `conflict`, `deleted upstream`, or `theirs-only` — all three become
**durable findings**, not sweep output.

Durable is the load-bearing word, and `theirs-only` is the case that nearly
escaped it: an earlier draft of this design said it "becomes drift for an
operator to triage" while the task list created findings only for `conflict` and
`deleted upstream`. A state that exists only in the return value of the pass
that found it is visible to whoever ran that pass and to nobody else — and
`theirs-only` is the single most common real state here, because it is what a
hand edit on the NAS looks like. It is deduplicated per `(subject, target,
field)`, so a sweep every six hours against an unresolved edit produces one
finding rather than four a day.

This is the existing rule stated once rather than case by case. The sweep
already refuses to triage drift, refuses to conclude from a capped read, and now
refuses to conclude from an ambiguous one.

## 5. Resolution is the operator's — and it is bounded by where desired state
## actually lives

"Take theirs" and "edit" were written here as though the desired state for one
subject were a thing one could set. It is not, and a resolution the model cannot
express is worse than one it does not offer.

Desired state is resolved by `ResolveEntitlements` from exactly three inputs: the
roles a person holds, the **role mappings** for the target, and their
**allowances**. Which means:

- `group` comes only from `target_role_mappings`, keyed
  `UNIQUE (target, project_id, role_key, field)` with **no subject column**. Its
  own DDL says what editing it does: *"changing it silently changes what every
  holder of that role can reach."* Adopting one member's hand-made group into
  the mapping changes it for everybody who holds that role.
- `enabled` and `smb_enabled` are not mappable **at all**, and are refused as
  mapping fields at three separate layers. They are derived — a subject holding
  any non-lifecycle mapping is enabled — so there is no value to edit.
- The only per-subject overlay is `allowances`, and its additive arm is
  deliberately unimplemented (`ErrAllowanceAdditiveUnsupported`). It is
  subtractive in practice: a deny with an actor, a reason, and a bound in time.

So the resolutions are constrained by **whether the adopted value has an owner
that can hold it for one subject**:

**Keep ours** — always available. Apply Syndra's state over the target's. The
current default, now a decision somebody made.

**Take theirs**, offered only where the target's value is expressible for that
one subject in a source that already exists:

- A lifecycle field whose target value is the RESTRICTIVE one — somebody
  disabled the account, or turned SMB off, on the NAS — is adopted as a **deny
  allowance**. This is not a new mechanism; it is the mechanism that layer was
  built for, and adopting through it is a strict improvement on the hand edit:
  the suspension acquires an author, a reason, and an expiry or a review date,
  because the schema refuses one without them.
- Everything else — a group value, or a lifecycle field whose target value is
  the PERMISSIVE one — has no per-subject owner. It is not offered as a click.

**Change the policy** replaces the unowned half of "take theirs" and all of
"edit". The finding names the mapping that produces the value and **how many
people hold that role** (`GET /targets/mappings/{id}/holders` already answers
this), and the operator edits that mapping knowing the blast radius. An operator
who genuinely wants one person's group changed is being told the truth: in this
model that is a role question, and the answer is a role grant or a new mapping,
not a patch aimed at one row.

**Explicitly NOT built here: a per-subject additive override.** The `allowances`
schema has the column and the code refuses the arm, on purpose — an additive
per-subject grant is entitlement outside the role model, and 000030 defers it
until a second consumer makes the abstraction real. Introducing it so that a
conflict dialog has a convenient button would be the worst available reason to
introduce it, and it would put a grant somewhere no access review looks.

Each resolution records the base afterwards, which is what stops a resolved
conflict from returning. A finding whose only honest resolution is "change the
policy" stays open until the policy changes and the next pass agrees — it is not
dismissible, because dismissing it is the ignore flag §7 refuses.

## 6. Deleted upstream is its own state on purpose

`OURS` names an account the target no longer has. Re-creating it is one valid
answer and un-binding is another, and a sweep may take neither: the account may
have been deleted deliberately, in which case recreating it undoes a decision,
or accidentally, in which case unbinding erases the record of it.

This is the case that already bit — stub-era bindings queueing a create every
six hours against a production NAS. It is fixed today by refusing to converge
them; this change makes that refusal a named state with a resolution instead of
a special case with none.

## 7. What this does NOT change

- **The fingerprint stays.** It answers a different question: "has the subject
  moved since the plan the operator approved?" That is `git`'s
  non-fast-forward check, and it remains correct with a base in place.
- **The mutation log stays append-only.** History is not rewritten by a merge.
- **No automatic conflict resolution, ever.** Not behind a flag: a flag that
  turns conflict resolution back into "ours always wins" is the current
  behaviour with a name, and the flag would be set on the deployment where the
  cost of being wrong is highest.

## 8. Rollout

The base is absent for every existing binding. A missing base MUST behave as
today — converge — because inventing one would either fabricate agreement or
manufacture conflicts for every managed subject on the first pass.

**"As today" includes today's guards, and one of them must be said out loud: a
missing base selects the two-way behaviour only for an account that is
PRESENT.** An account the target no longer has is `deleted upstream` whether or
not a base exists, and is never queued for creation. Today's reconciler already
refuses those (`partitionByPresence` splits them out as stale bindings, and they
are excluded from planning) — this is the guard that stopped stub-era bindings
recreating accounts on a production NAS, and a rollout rule phrased as "no base,
converge" would hand it straight back for exactly the bindings that predate the
base. The required tests include the pair: no base with the account present
converges, no base with the account absent reports `deleted upstream` and writes
nothing.

It fills in as applies happen; a target can also be seeded by adopting the
current read as base for subjects where `OURS == THEIRS`, which is safe by
definition and is the "already merged" branch running once.
