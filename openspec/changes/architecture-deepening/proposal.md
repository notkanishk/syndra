## Why

An architecture review on 16 Oct 2026 walked the code that changed most since 15 Sep (access changes, plans, the TrueNAS add-on, the targets and access screens) looking for shallow modules: places where one question is answered by several copies, so a fix in one leaves the others wrong. The review is `review-2026-10-16.html` in this folder (open it in a browser). It uses the `codebase-design` vocabulary: module, interface, depth, seam, adapter, leverage, locality.

The common thread is the project's own rule, "a preview computed by different code from the thing it previews is a preview of nothing". It holds for the token shape, and not yet for access changes, plans or the add-on.

## What Changes

Candidate 1, the access-change planner, is designed (`design.md`) and its first slice is built: bulk assign and remove bundle are planned and applied by one pure planner over one locked snapshot (`tasks.md` 1.1). Each candidate in `tasks.md` is taken one at a time: grilled with the owner, then designed in `design.md` before any code. The order matters: 1 gives 4 its apply step, and 5 is cheaper after 1 and 4.

## Impact

Backend `handlers`, `services`, `db`; `addons/truenas`; `ui/src` screens that judge a person's holding. Slice 1 needs no migration: a plan row's `cancel_ids` live in `plan_subjects.outcome_json`. The bulk endpoint's contract is unchanged; a bundle row's words are new.
