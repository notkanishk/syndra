# Bundle Versioning

**Status:** Implemented
**Phase:** 5.5
**Supersedes:** the immediate bundle-edit cascade introduced in `provisioning-intents`

## Why

Editing a bundle used to reach every holder the moment it saved. Adding one role
to a bundle fourteen people hold was fourteen grants, projected before anybody
had looked at what it would do — so the edit and its consequence were the same
click, and the only way to be careful was not to edit.

That also made two ordinary requests impossible. A bundle reshaped for the next
intake could not be reshaped without changing what the current cohort holds. And
nothing could answer "what did Lab Tech grant last term" — the bundle only ever
knew its present.

## What changes

Bundles are versioned the way a repository is tagged, and the model is git's:

- **`bundle_roles` is the working copy.** Edits land here and reach nobody.
- **`bundle_versions` are published snapshots.** Immutable once written.
- **Every assignment pins a version.** A holder's access resolves through the
  version they are on, not through the bundle's current contents.

Publishing is the step that has consequences, and it asks the question the whole
feature exists for: do the people already holding this come along, or stay where
they are. Both answers are legitimate and neither is a default.

Publishing and moving holders are both **rehearsed** — the same plan-then-apply
contract, the same `?apply=true`, the same `BulkPlan` every other bulk surface
in the product speaks. The plan is per-holder and built from each person's OWN
pinned version: somebody on v2 and somebody on v4 are moving different
distances.

## What becomes visible

- **The bundle list** marks a bundle with unpublished changes, and states how
  many holders an earlier publish left behind.
- **The bundle workspace** carries the unpublished diff as a strip, not a badge:
  a bundle can sit edited-but-unpublished indefinitely and look finished.
- **A version list** with per-version holder counts, and "move these N to v4"
  where somebody is behind.
- **A person's bundle chip** names their version, and says when a newer one
  exists.
- **The People filter** narrows by bundle, then by version — which is how you
  find the eleven people a publish left on v2.

## Capability deltas

- `access-governance` — a bundle edit no longer cascades. See the spec delta.
- `role-management` — unaffected.

## Out of scope

- Deleting a version. A version with holders cannot be removed, and an empty
  superseded version is history rather than clutter. Revisit if a real bundle
  accumulates enough noise to make the list unreadable.
- Naming versions (`v1.2-safety`). The number plus a required-in-practice note
  carries the same information; a second identifier would need a uniqueness rule
  and buys nothing yet.
