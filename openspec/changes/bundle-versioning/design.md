# Design — Bundle Versioning

## Decision 1 · The draft is not stored, it is computed

Three tables, and deliberately no fourth:

```
bundle_roles          the WORKING COPY — edits land here, reach nobody
bundle_versions       published snapshots — immutable once written
bundle_version_roles  what each snapshot contained
user_bundle_assignments.version_id   the pin
```

There is no draft table and no `is_draft` column. A draft is not a state
something is in — it is the difference between the working copy and the latest
published version, computed by `BundleDraft()` when asked. A stored draft flag
would be a second thing to keep true, and the first time it disagreed with the
tables the screen would be lying about what publishing does.

This is also why `bundle_roles` survived rather than being folded into the
version tables. It already *was* the working copy; the nine call sites that read
"what does this bundle hold" still mean the working copy and did not have to
change. Only the sites that meant "what does this PERSON get from it" moved.

## Decision 2 · Two questions that used to be one

`GetRolesForBundle(bundleID)` answered both of these, and versioning splits them:

| Question | Resolved by | Callers |
|---|---|---|
| What does this bundle grant *now*? | `bundle_roles` | editor, impact preview, topology, project role counts |
| What does *this person* get from it? | their pinned version | every closure computation |

`GetUserBundleRolesGrouped(userID)` is the second question as one query. It
replaced the "list their bundles, then read each bundle's roles" loop that every
closure ran — and that loop is precisely what made an edit reach everybody: it
asked what the bundle holds, when the question was what this person's version
holds.

Every closure path now goes through it: `userBaseHoldings`,
`userBaseHoldingsExcludingBundle`, `userBaseHoldingsExcludingGrant`, and
`collectUserRoles`. Missing one would leave a surface computing access from the
working copy — showing somebody roles they do not have.

## Decision 3 · Migration is a question with two right answers

Publishing asks whether current holders come along, and neither option is
pre-selected. A default here would be the product making the decision the
operator opened the dialog to make.

- **Move everyone.** The common case: the bundle was wrong and is now right.
- **Leave them.** A real answer, not a deferral. A bundle reshaped for the next
  intake should not change what the current cohort holds mid-term. The version
  is still written; it applies to new assignments.

The choice sits in the **compose** step rather than as a toggle on the result,
because it changes the plan. Rehearsing tells you what moving fourteen people
would do; flipping the answer afterwards would leave a plan on screen that
describes something else.

With zero holders the question is not asked at all — there is nothing to decide,
and asking anyway would train people to click past it.

## Decision 4 · The plan is per-holder, from each holder's own version

Holders are not all in the same place. After two publishes with "leave them",
one bundle can have people on v2, v3 and v4 simultaneously. A plan computed from
"the latest version" would be wrong for everyone not on it.

So `RehearseBundlePublish` walks `GetBundleHoldersByVersion` and computes each
person's closure delta from *their* pin. Three outcomes per row:

- **gains X** — straightforward.
- **LOSES X** — the row an operator is scanning for, called out in caps and
  carrying the consequence sentence.
- **no change** — the version moves but their access does not, because another
  bundle, a direct grant or a mapping rule already covers everything that
  changed. Saying so is the difference between a plan somebody reads and a plan
  somebody scrolls past.

The revoke-suppression property that used to be tested against
`CascadeRoleRemovedFromBundle` moved here with the behaviour, and the tests
moved with it: a revoke is only projected when *nothing else* still grants the
role.

## Decision 5 · A new bundle publishes an empty v1

Every assignment pins a version, so a bundle with no published version could not
be assigned at all. The alternative was blocking assignment on "publish
something first" — a new failure mode in three places, invented to avoid an
uninteresting row in a history.

v1 is honest: the bundle existed and granted nothing, which is what the
empty-bundle copy has always said. `CreateBundle` writes the bundle and its v1
in one transaction.

## Decision 6 · Version-numbering is per bundle, and allocated inside the tx

`UNIQUE (bundle_id, version)`, starting at 1. Not a global sequence: "Lab Tech
v2" is a sentence somebody says out loud, and a global counter would make it
meaningless.

The next number is computed inside the publish transaction
(`MAX(version) + 1` in the INSERT), not read first. Two operators publishing at
once would otherwise both read the same max and collide on the unique index —
which is the correct failure, but only one of them should see it.

## Decision 7 · Filtering is bundle, then version

The People filter gains `version`, meaningful only alongside `bundle`. A version
with no bundle spans unrelated bundles and answers nothing, so `parseFilters`
drops it — a hand-edited URL degrades to the bundle-less view rather than an
empty one.

The two clear independently, because dropping the version to see the whole
bundle is the move an operator makes constantly once they have found the
stragglers.

`UserListItem` carries `bundle_versions` (bundle name → pinned version) so the
row can render the chip and the filter can narrow, both without a second
request.

## What this changed about existing behaviour

`CascadeRoleAddedToBundle` and `CascadeRoleRemovedFromBundle` are gone, replaced
by `EditBundleWorkingCopy`, which enqueues nothing. The handlers return the
draft instead of a cascade result, and say so:

> "Role added to the bundle's working copy. Publish a version to apply it."

This is the single largest behavioural change in the product since the outbox
landed, and it is deliberate: it converts the most dangerous edit in MkAuth into
a free one, and puts the danger behind a rehearsal.

---

## Decision 8 · Every per-user read had to move, and two did not (post-audit)

The first cut of Decision 2 converted the closure paths and missed two places
that also read the working copy *for a person*. An audit found both, and they
were the most serious defects in the change:

- **The cache compiler.** `CompileUserCache` resolved a user's bundles through
  `bundle_roles`. That is the path a token is issued from, so any rebuild after
  a draft edit — a webhook, a rule change, a manual recompile — baked the
  unpublished edit into real tokens. Before publishing, without appearing in any
  plan, and invisible on every screen. It now calls
  `GetUserBundleRolesGrouped`, which structurally cannot return a role that is
  not in some published version.

- **Assigning a bundle.** `CascadeBundleAssignedToUser` computed its cascade
  from the working copy while `AssignBundleAndEnqueue` pinned the assignment to
  the latest published version, in the same transaction. A new member was
  pinned to v2 and simultaneously granted whatever sat unpublished in the
  working copy.

- **And one the audit did not name:** the bulk assign-bundle *rehearsal* counted
  working-copy roles, so its "Cascades 6 roles" promised roles the apply pass
  would never grant.

The last two share a fix: `LatestVersionRoles(bundleID)` — *what does this
bundle grant today* — which is by definition what a new assignment pins to.
Reading `bundle_roles` to answer that question is now wrong everywhere, and the
three questions are finally distinct:

| Question | Function |
|---|---|
| What is being edited? | `GetRolesForBundle` (working copy) |
| What does a new assignment get? | `LatestVersionRoles` |
| What does *this person* have? | `GetUserBundleRolesGrouped` |

**The lesson worth keeping:** splitting one function into two left the *third*
question unnamed, and the two misses were both code that needed it. A migration
like this is not done when the callers compile — it is done when every caller
has been re-asked which question it meant.

## Decision 9 · The publish snapshot is the caller's read, not a fresh one

Decision 1 originally had `PublishVersionAndEnqueue` snapshot the version with
`INSERT ... SELECT FROM bundle_roles` inside the transaction, on the reasoning
that a caller-supplied role list could disagree with what was on screen.

That reasoning was backwards. The deltas in `params` are computed *before* the
transaction from one read of the working copy; re-selecting inside it reads at a
different instant. A concurrent edit between the two leaves holders pinned to a
version whose contents the outbox never projected — the outbox and the pin
describing different things, which is precisely the failure versioning exists to
prevent.

The snapshot is now the role set the plan was built from, threaded through
`DraftDiff.Working` so the diff, the per-holder deltas and the version contents
all come from **one** read. An edit that lands mid-publish is simply not in this
version; it stays a draft for the next one, which is the honest outcome.

`draftRoleSet` — which read the working copy a second time and *discarded its
error* — is gone. A failed read returned nil, nil reads as an empty bundle, and
an empty bundle plans a revoke of every role for every holder. The error
propagates.

## Decision 10 · A rehearsal validates what the write validates

`MoveHoldersAndEnqueue` rejected a version belonging to another bundle. The
*rehearsal* did not, so it produced a plan — and because the target's version
number was inferred from whoever was standing on it, one that read "v2 → v0".

A plan that renders nonsense and is refused only on apply is worse than no
plan: it is something an operator has already approved. Ownership is checked
before the plan is built, and the target version number comes from the version
list rather than from its holders — nobody is standing on a version the moment
it is published.

## Decision 11 · The version travels with the roles it was read from

Decision 8 fixed *which roles* an assignment projects. It left *which version*
resolved twice: the service read the latest version's roles, and
`AssignBundleAndEnqueue` independently selected the latest version to pin. A
publish committing between the two pinned the member to v3 while the outbox
carried v2's roles.

That mismatch is undetectable afterwards. Both rows are individually valid — a
real assignment to a real version, and real outbox rows for real roles — and
nothing in the system compares them.

`LatestVersionRoles` now returns `(version, roles)` and the caller passes the id
into the transaction, so the pin and the projection are the same version *by
construction* rather than by both being read at roughly the same time. A publish
that lands mid-assignment leaves the new member on the version they were
assigned, which is exactly where every un-migrated holder sits.

The same shape as Decision 9, one level down: any pair of reads that must agree
has to be one read.

## Decision 12 · Copy is part of the contract, and it did not move with the code

The removal panel said "Removing X affects 14 people **now**" and "14 people
**lose** it", next to a red `dangerConfirm` button. Every word was true when a
removal cascaded on save. None of it was true afterwards — removal edits the
working copy and reaches nobody — and the panel went on saying it.

That is the most dangerous kind of stale copy: it does not look broken. An
operator reads it, takes the edit for an applied revocation, and either believes
a door is locked while it is open, or goes looking for a change that never
happened.

The content was right and stays: who loses the role, who keeps it through a
rule, what cascades away with it, is exactly what you want before deciding to
make the edit. What changed is tense and register — every sentence conditional
on publishing *and* migrating, amber rather than red, and the destructive
confirm moved to Publish, where the revocation actually occurs. The row action
reads "Drop" and its toast says "Nobody loses it until you publish."

Worth stating as a rule: when a behaviour change makes an action safer, the copy
describing it is not decoration that can lag. It is the only thing on screen
that says which of the two behaviours is in effect.

## Decision 13 · A conflict has to be answered, not swallowed

`AssignBundleAndEnqueue` used `ON CONFLICT DO NOTHING` on
`(user_id, bundle_id)`, which is correct for making assignment idempotent and
was silently wrong for everything downstream of it.

Re-assigning a bundle to somebody already on v1 conflicted, so their v1 pin was
preserved — and the caller's params, computed against v2, were enqueued anyway.
They received v2's access while every record said v1. Both halves individually
plausible, the pair undetectable, and the audit log recorded an assignment that
never happened.

The insert now `RETURNING`s the pin. No row means they already held it, and in
that case the transaction writes **nothing**: no outbox rows, no audit row, no
repin. `CascadeResult.NoOp` carries the fact outward so the API can say "Already
holds this bundle — nothing changed" rather than reporting a successful
assignment.

Two things this deliberately does not do:

- **It does not re-pin them to the latest version.** Moving somebody between
  versions is a rehearsed action with a plan; doing it as a side effect of an
  idempotent assign would move people nobody planned to move, without a plan and
  without a record of the decision.
- **It does not pre-check.** Asking "do they already hold this?" before the
  transaction only moves the race. The conflict is authoritative exactly where
  it occurs.

`NoOp` is also kept distinct from `Enqueued == 0`. The latter means the write
*did* happen and changed nobody's effective access — something else already
granted every role. Collapsing the two is how an idempotent call gets reported
as a change.

**The pattern across Decisions 9, 11 and 13** is one thing said three ways: a
value read in one place and re-derived in another will eventually disagree, and
in this subsystem the disagreement is always between what somebody holds and
what the records say they hold — which is the one inconsistency the product
exists to prevent.
