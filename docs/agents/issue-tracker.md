# Issue tracker: OpenSpec

Work for this repo is tracked in OpenSpec, not GitHub Issues. The GitHub repo is
public: nothing that names a private address, a real account, or a weakness of a
live system may be written to an issue, a PR, or any committed file.

## Where things live

- **Pickup point:** `openspec/NEXT.md` lists everything open, across all changes.
- **A body of work:** `openspec/changes/<change-slug>/` with `proposal.md` (why),
  `design.md` (decisions), `specs/*.md` (contract deltas), and `tasks.md` (tickets).
- **Index:** `openspec/INDEX.md` links every change and capability spec.
- **Done work:** `openspec/changes/archive/`.

## Tickets

A ticket is one checkbox item in a change's `tasks.md`, numbered by section:
`- [ ] 3.2 What has to be true when it's done`. Sub-bullets carry detail and the
tests it owes. `- [x]` means done; never delete a done item, so the history stays
readable. A ticket found while doing other work goes under a "Found on the way"
or a dated "Found in production" section, not silently into code.

Blocking is written in prose on the item: `Blocked by 2.1, 2.3.`

## When a skill says "publish to the issue tracker"

1. Pick the change it belongs to, or create `openspec/changes/<slug>/` with a
   `proposal.md` and `tasks.md` (use the `openspec-propose` skill for a new one).
2. Add the tickets to `tasks.md`.
3. Add a one-line pointer to `openspec/NEXT.md` if the work is open beyond this session.

A spec (from `to-spec`) is that change's `proposal.md` plus `design.md`.

## When a skill says "fetch the relevant ticket"

Read the `tasks.md` item the user names (change slug + number), its section, and
the change's `design.md`.

## Wayfinding

- **Map:** `openspec/changes/<effort>/design.md`, with a `## Decisions so far`
  section and a `## Fog` section for what's still unknown.
- **Child ticket:** a `tasks.md` item whose text is the question, tagged with its
  type: `(research)`, `(prototype)`, `(grilling)` or `(task)`.
- **Claim:** add `(claimed)` to the item before starting. **Resolve:** tick it,
  write the answer as a sub-bullet, and add a one-line pointer under Decisions so far.
- **Frontier:** open, unblocked, unclaimed items, lowest number first.

## PRs

PRs are not a request surface. They carry finished work and end with the deploy
note when merging would change production.
