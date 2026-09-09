# Bundle Lifecycle Repair

**Status:** Implemented
**Phase:** 5.5 (fourth audit pass on [`bundle-versioning`](../bundle-versioning/proposal.md))
**Found by:** an operator creating a bundle on the live deployment, 2026-09-09

## Why

Someone tried to create a bundle, put roles in it, publish it and give it to a
person. Every one of those four steps misbehaved, and three of them for the same
underlying reason: the product could not consistently tell the difference
between what a bundle *contains* and what a bundle *grants*.

**Creating one published an empty v1.** The v1 is load-bearing — an assignment
pins a version, and `LatestVersion` is a `QueryRow` that errors for a bundle
with none — but its emptiness was not. It put the operator somewhere with no
good exit: assigning the bundle granted nothing, and the roles they then added
came back to them as "2 unpublished changes" against a version that had never
described anything. There is no point in that sequence where the screen is
telling the truth about a bundle the operator considers finished.

**Publishing it was impossible.** The shared rehearsal dialog disables Apply
without a `plan_id`, correctly, and `POST /bundles/{id}/publish` issued none —
so every publish that reached anybody could be previewed and never applied. The
identical defect was found and fixed for mapping rollback, which says so in
`mapping_plan.go`: *"A rehearsal that returns no approval is a rehearsal nothing
can spend."* Bundle publish and holder-move were the two surfaces that were
missed. Two further publishes were refused on a separate rule — a bundle nobody
holds, and holders deliberately left behind — because the dialog read "moves
nobody" as "nothing to do", which is right for a bulk grant and wrong for the
only screen that can cut a version.

**Nothing said why.** The button greyed out and offered no reason, on three
different routes, although `Button` has rendered a visible reason under a
disabled control since the touch work and `Button.tsx` states the rule.

**Assigning it promised roles it would not grant.** The assign panel read the
working copy; an assignment pins the published version. So it listed every
unpublished edit as a role the person was about to receive — the operator saw
the roles they had just added promised on the way in and called *unpublished* on
the way out, and the apply granted neither of the two sets they had been shown.

## What changes

- **A bundle is created with its roles.** `POST /bundles` requires at least one,
  and writes the working copy and v1 from one slice. "Created empty." remains a
  reachable note for the bundles that already have it.
- **`GET /bundles/{id}/roles?published=true`** answers what the bundle grants
  today. The default still answers the working copy, because that is what the
  bundle editor edits. One route, two questions, and the caller has to say which.
- **Publishing and moving holders join the plan gate.** Both rehearsals persist
  what they showed and return the `plan_id` their apply cites — the last two
  rehearsed mutations in the product that recomputed their own diff and wrote
  from it.
- **`BulkPlan.RequestFingerprint`**, for the surfaces whose cohort is derived
  from the world rather than named in the request body. A publish cannot list
  its subjects the way a bulk grant can, so a person assigned the bundle between
  the review and the apply is not on the approval and no per-subject check would
  ever notice them.
- **An apply that moves nobody can still be an act**, where the surface says so.
  The gate is now "no row will act" rather than "there are no rows".
- **Every disabled control in the rehearsal dialog states its reason**, from one
  string that `disabled` is derived from, so the two cannot disagree.
- **The assign panel reads the published version**, cached apart from the working
  copy, and names the unpublished edits that are *not* part of the assignment.

## Capability deltas

- `access-governance` — see the spec delta. Amends the bundle-versioning
  requirement that a new bundle publishes an *empty* v1, and adds the plan-gate
  requirement for the two version surfaces.
- `role-management` — unaffected.

## Out of scope

- **A migration for bundles that are already empty.** They exist on the live
  deployment and stay valid: the guard is on creation, and an existing empty
  bundle is not broken, only useless. Emptying one after the fact is still
  allowed — the delete dialog's own copy offers it as an alternative to deleting.
- **Removing the "leave them behind" answer.** It is deliberate and documented.
  What was wrong was that the product would not carry it out.
- **`definitionLabel` as a name.** It now covers two things — a definition being
  saved, and a version being cut — and the name only describes the first. Left
  alone rather than renamed across four mapping call sites and their tests in a
  change that is already wide; the prop's own comment says what it means.
