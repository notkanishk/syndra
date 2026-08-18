# Design: reconciliation as a three-way merge

## 1. The base is a read-back, not a belief

The merge base for `(subject, target)` is **the state the target reported after
the last successful apply** — not what Syndra intended to write.

The distinction is the whole design. Recording the intent would produce a base
that agrees with `OURS` by construction, and a base that always equals one side
can never produce a conflict: every difference would resolve as "the target is
wrong", which is exactly today's behaviour with more machinery.

`apply` already performs this read-back (`readBack(username)`), for the
fingerprint. Storing it is one write on a path that already reads.

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
resolve `conflict`, `deleted upstream`, or `theirs-only` — those become
findings.

This is the existing rule stated once rather than case by case. The sweep
already refuses to triage drift, refuses to conclude from a capped read, and now
refuses to conclude from an ambiguous one.

## 5. Resolution is the operator's, and it is three real choices

- **Keep ours** — apply Syndra's state over the target's. The current default,
  now a decision somebody made.
- **Take theirs** — adopt the target's value into the desired state. Not
  "ignore": the mapping changes, so the next sweep agrees rather than re-raising.
- **Edit** — set a third value, which is the case where both sides were wrong.

Each records the base afterwards, which is what stops a resolved conflict from
returning.

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

It fills in as applies happen; a target can also be seeded by adopting the
current read as base for subjects where `OURS == THEIRS`, which is safe by
definition and is the "already merged" branch running once.
