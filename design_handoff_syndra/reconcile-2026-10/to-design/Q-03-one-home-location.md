# Q-03 · One place for home folders, and an operator's own storage

From Claude Code, 15 Oct 2026. Follows A-02. Read `from-design/A-02-where-to-connect.md` first; this changes its §5 and §7 and leaves the rest standing.

**Answer in the design project at** `handoff/2026-10-reconcile/from-design/A-03-one-home-location.md`, with boards beside it in `handoff/2026-10-reconcile/design/`.

## What changed (owner, 15 Oct · Locked)

- **Home folders have one location per system, not one per role.** It is chosen once, from the same list A-02 §5 drew (`main › home_dir`).
- **A role only says whether it gives a home folder, and its default limit.** On the mapping, "Gives a home folder" is on or off, and the limit is a size or none.
- **The largest limit among a person's roles wins.** One person can still be given their own limit, with a reason and a review or expiry date, like a hold. That is already built in the backend.
- **Two roles can't disagree about where a home goes any more**, so A-02 §7 ("[Name]'s roles name two home folders") is retired.
- **Changing the location later moves nothing.** Syndra never moves or deletes a home folder. Existing homes stay where they are, and new ones are made in the new place. A home's own address is always read from the account on TrueNAS.

## What code found (Locked)

- **What every account opens.** On the live NAS, one share grants `everyone@` read. That covers every account, Syndra's included, whatever its roles. The add-on reports it apart from what each group opens: what holding a group adds is one list, and what every account opens anyway is another. A-02 §3 drew only the first.
- **An operator had no way to their own storage.** `/storage` is in the member's rail. The operator's rail is about running the makerspace, and an operator is a person too, with a password and folders of their own. Code built one row in the Account sheet for operators, following shell-account ("Everything about you and this device sits behind the account"):
  - "Network storage", with the muted note "Your password and folders", above Appearance.
  - A member doesn't get it there: they have it in their rail, and one row does one job.
  - The page names itself in the breadcrumb in either view.

## What needs drawing

1. **Where the location is chosen.** It's one write place per setting. Code's suggestion is the system page (route-system-targets-id), beside the Zitadel project step: "Home folders go in `main › home_dir`" · Change. Draw:
   - choosing it, from the list A-02 §5 drew
   - not chosen yet
   - no place to offer (A-02 `add-home-none`)
   - the change confirmation, saying existing homes stay where they are
2. **The mapping dialog, redrawn from A-02 `add-home`.**
   - "Gives a home folder" (on/off), and under it "Home folder limit": a size, or no limit.
   - When the system has no location yet, the toggle can't be used. Is that NotOpenYet, or one sentence with "Choose where home folders go →"?
   - The row's read-only line becomes "Home folder, up to 20 GB, in `main › home_dir`".
3. **One person's own limit,** on their page or the system's people list. Draw: setting it with a reason and a review or expiry date, how it reads while in force, and when it lapses.
4. **What every account opens,** on the mappings page. One line in the card header or under the list? For example: "Every account also opens `gitlab_data`, read only, whatever its roles." Is it amber? It's usually a NAS setting nobody meant.
5. **The operator's Account sheet** with the storage row: confirm it or redraw it. Should an operator also reach their own access (the member's My access page) from there?

Nothing else in A-02 changes.
