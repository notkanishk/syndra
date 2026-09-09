> **Status:** bundle-lifecycle-repair — design | [< Index](../../INDEX.md) · [Proposal](proposal.md) · [Tasks](tasks.md)

# Bundle Lifecycle Repair — design

## The one distinction all of this turns on

A bundle has two role sets, and they answer different questions:

| | Table | Question | Who should read it |
|---|---|---|---|
| **Working copy** | `bundle_roles` | What will the NEXT version contain? | the bundle editor |
| **Published version** | `bundle_version_roles` | What does this bundle GRANT today? | anything previewing or performing an assignment |

`bundle-versioning` established this and its audit passes found four places
reading the wrong one. This pass found the fifth (the assign panel) and one
place where the *product* had no answer at all: a bundle created empty grants
nothing while containing everything the operator has typed.

`GET /bundles/{id}/roles` answered only the first question, to every caller.
Rather than a second route, it takes `?published=true` — the two answers come
from one place, so a reader of the handler sees both questions side by side, and
the comment there says which callers must ask which. The client caches them
under different keys (`["bundles", id, "roles"]` and `[…, "roles", "published"]`)
because sharing one entry would make whichever screen asked last correct.

## Why creation requires roles rather than allowing versionless bundles

Two ways to stop a bundle being born empty:

1. **No v1 until the first publish.** Honest, and it puts `pgx.ErrNoRows` on the
   path of `LatestVersion`, `LatestVersionRoles`, the assign cascade, the draft
   diff and the version list — five readers that currently rely on a version
   always existing, each needing its own "not published yet" state.
2. **Require the roles, publish v1 with them.** The invariant every reader
   depends on holds unchanged, and the bundle is born as the thing it was
   described as.

The second is chosen. The first is not more correct — it is the same rule
enforced later, in five places instead of one, and it invents a new bundle state
("exists, unpublishable, unassignable") that has no operator meaning.

The working copy and v1 are written from **one slice** inside one transaction.
Publishing diffs the working copy against the latest version, so a v1
disagreeing with `bundle_roles` by a single row would appear as an unpublished
change nobody made — which is the exact defect being repaired, reintroduced at
the moment of creation.

## The plan gate on the two version surfaces

`addon-platform` group 3 made every rehearsed mutation persist what it showed
and made the apply cite it (§8). Bundle publish and holder-move never joined,
and the visible consequence was that they could not be applied at all.

### What the citation has to bind

A bulk grant names its `user_ids`, so `FingerprintBulkRequest` is computable
from the request and the cohort cannot move under the approval. A publish
cannot: its cohort is *whoever holds the bundle*. `claimPlan` verifies the
subjects **on** the approval, so a person assigned the bundle after the review is
invisible to it — not stale, absent — and would be moved under an approval that
never mentioned them.

So the plan carries `RequestFingerprint`, set by the rehearsal from what it read:

| Bound | Why |
|---|---|
| bundle id, `migrate` | Moving fourteen people and leaving them alone are different acts. |
| version contents | A working-copy edit can change what v_next *is* without moving anybody — adding a role every holder already has from a rule leaves every delta identical. |
| the holder set, with each holder's version | A holder joining, leaving, or being moved to another version by somebody else is a different cohort. |

A mismatch surfaces as `PLAN_REQUEST_MISMATCH`, which the dialog already
recovers from as a stale plan: fresh preview, banner, read it again.

Per holder, the fingerprint binds the verdict's own inputs — the version they
stand on, and the adds and revokes the move would actually perform. Both move
without the bundle changing: a direct grant landing on somebody turns "LOSES
laser" into "no change to their access", and an approval read as the first must
not apply as the second.

### What the claim does NOT do

It does not drive the write. `PublishBundleVersion` is a single atomic operation
over a cohort, not a per-subject walk, and it recomputes under the access lock
as it always has. The claim's contribution is a refusal *before* the lock is
taken when the reviewed world has moved. That is the honest division: the lock
makes the write atomic, the citation makes it the write that was approved, and
those were never the same guarantee.

### The empty cohort

`issuePlan` returns without an id when the rehearsal had no subjects, so a
bundle nobody holds gets no approval — and demanding one that cannot exist would
be the same dead end from the other side. The apply skips the claim in exactly
that case, which is the shape `issueRollbackPlan` already uses for a mapping
version whose roles reach nobody.

## Why "moves nobody" is not "nothing to do"

`RehearsalDialog` blocked Apply on `plan.summary.apply === 0`. For a bulk grant
that is right — a plan for nobody means nobody was selected. For a surface whose
act is not the rows it is wrong, and two publishes proved it: a bundle nothing
holds yet, and holders deliberately left behind.

The escape hatch already existed (`definitionLabel`) but was gated on the plan
reaching *nobody*, guarding a real hazard — a plan of forty unmoved people must
not take that label and submit with no citation. The hazard is now closed at its
cause instead: whether a citation is required comes from whether the plan **has
rows**, which is precisely what the backend derives it from. So the gate can be
what it always meant, and a mapping whose forty subjects all hold the target
role from somewhere else becomes savable too — the same dead end, one surface
over.

What stays refused is a plan the dialog cannot read: no `outcomes` array, or an
empty one alongside a non-zero total. Those cannot be reviewed, so they cannot
be approved.

## Disabled controls state their reason

`Button` renders a visible `reason` under a disabled control, and `Button.tsx`
says why it is visible copy rather than a `title`: *"hover does not exist on
touch and does not survive a screenshot sent to a colleague."* The rehearsal
dialog — where an operator has just read a page of consequences and is looking
for the one control that acts on them — passed none, on any of its routes.

`applyBlocked` is one string, and `disabled` is `Boolean(applyBlocked)`. Deriving
the state from the explanation is the point: the previous code had three
conditions inline and no explanation at all, and any future condition added to
one would have to be remembered in the other.

The compose step gets `notReadyReason` for the same reason. On the publish
dialog the unanswered question sits above the fold and the disabled button below
it, so "greyed out for no visible reason" was the whole experience.

## What the assign panel now says

It reads the published version, which makes it correct and makes the bundle
screen look like it disagrees — the operator has just added those roles and
sees them absent here. So the panel names the discrepancy itself: *"Lab Tech has
2 unpublished changes. Those are not part of this — an assignment gives v2, and
publishing is what decides whether the people holding it move."*

A screen being right is not enough when a second screen shows a different set.
The reconciling sentence belongs on whichever of the two the operator reaches
with the wrong expectation, which is this one.
