# Q-02 · Where a member connects, and what a group opens

From Claude Code, 14 Oct 2026. Read `openspec/changes/truenas-reconcile/WALKTHROUGH.md` in the repo first. It is the whole flow, operator and member, Syndra and TrueNAS, with every gap marked.

**Answer in the design project at** `handoff/2026-10-reconcile/from-design/A-02-where-to-connect.md`, with boards beside it in `handoff/2026-10-reconcile/design/`.

## What code found (Locked)

- **The member page's addresses are wrong.** It builds `\\host\<group name>`, assuming a share is named after each group. None is.
  - On the real NAS, a member of Community reaches `\\drive.makerspace.tools\main\waddle_share`: one SMB share (`main`) with a folder per resource.
  - What a group can open is decided only by the folder permissions on TrueNAS.
- **Code will derive it from the NAS.** For the member's groups, the add-on reads each share and its first-level folders, and returns every place they can open, with read or modify and both addresses.
  - Nothing is configured twice in Syndra.
  - The same read answers "what does this group open" for the operator.
- **Usage follows it.** One block per place the member can open, with 0 used rather than missing when they haven't saved anything there yet.
- **Restoring access needs a new password.** When access is taken away and given back, TrueNAS turns SMB on again only together with a new password. Until the member sets one, their account is "ready" but share access stays off.

## What needs drawing

1. **route-storage, one block per place.** A place is a share plus a folder, not a share alone: `main › waddle_share`. How is it named?
   - Folder alone, share alone, or a path?
   - Does read-only versus can-change show?
   - A member who reaches two folders in one share: two blocks, or one block with two addresses?
2. **The restored-access state.** The account exists and has a password, but SMB is off until a new password is set. It is like "Set a password to switch your account on", but the member had one. Draw the line and the step panel.
3. **The operator's "what this group opens" line** on route-system-targets-id-mappings, per mapping row or per group:
   - the places
   - read or modify
   - and when the NAS can't be read: last known, with its age
4. **A group that opens nothing.** A mapping to a group that no folder grants anything is allowed, but it gives nobody anything. How is that said on the mappings page, and on the member page?

Everything else in the package is built to the boards and `RULES.md`.
