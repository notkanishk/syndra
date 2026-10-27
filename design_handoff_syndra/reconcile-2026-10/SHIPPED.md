# Shipped: the October reconcile, TrueNAS slice

From Claude Code, 15 Oct 2026. What is built against the boards, and where code departed from them and why. Branch `feat/truenas-reach`.

## Built as drawn

**route-storage (A-02, Q-01 states)**
- Every state on the board: no role, on its way, set a password, old password gone, access given back, ready, a role that opens nothing, no address set up, on hold, paused (both), offline, loading, empty, error.
- Your folders, A (the folder as the name): home first, then A to Z.
  - Each block has Can change files or Read only, and "In main · from your Fabrication lead role".
  - Usage only where the member can save: "Nothing saved here yet", nearly full from 90%, "no limit set".
  - Both addresses come from the backend, built once from the registered host.
- The password panel, with the folders locked under it (NotOpenYet). No form while offline or paused, first password included.
- The connection guide (4b): five devices, from the browser, the folder chosen from the member's own.
- Phone: the step bar, the full-width guide button, and a password field at least 220px.

**route-systems, route-system-targets-id (A-01)**
- One ranked status from the backend decides the row and the system page.
- Check pips appear only when a check was missed or none has run yet.
- "Waiting to retry"; "Sending waits until Zitadel answers"; "The queue stops at that change".
- The project step (3a): Step 1 of 2, with taken projects disabled and named and role counts.
  - What waits on the project is fenced in NotOpenYet.
  - The result shows for that visit; the setup strip on later visits.
- Connected systems shows the project, or "Choose its Zitadel project →".
- #57: Assign is a dialog. People search, the selection seat, "Already has {username} on {target}", Changes / Stays, ConfirmByTyping, and "Assign {account} to {name}".

**route-system-targets-id-mappings (A-01 §3, A-02 §3/§4)**
- The role picker offers the system's one project's roles, unmapped only.
- The group picker comes from the add-on's own list, with the states `add-nogroups`, `add-unread`, `add-unchecked` and `add-refused`.
- The opens line sits under the picked group.
- Rows name the role without the project and say what their group opens.
- The read's age is said once in the card header.
- "Opens nothing yet" is amber, with the people count and where to look.
- The no-project frame.

**New parts, for `/design-sync` to bring into the design system:** `Notice`, `NotOpenYet`, `ConnectionGuide`.

## Where code departed, and why

- **Home kept (A-02 §6).** It reads "None of the roles you hold gives a home folder any more", not "You no longer hold Lab Tech". Syndra doesn't keep which lost role made the folder, so it can't name it.
- **What every account opens.** Not drawn. Syndra now considers only the `main` share (owner, 15 Oct), and nothing inside it grants everyone.
- **The operator's own storage.** The operator reaches it from the Account sheet: a "Network storage" row, "Your password and folders". Drawn in no board; Q-03 asks to confirm it.
- **Two roles naming two homes (A-02 §7).** Retired by the owner's ruling of 15 Oct: one home location per system.

## A-03, built (15 Oct)

- **route-system-targets-id**
  - The setup card is one row per setting, and Home folders is its one write place.
  - The home-folder states: unset, choose, none, and change (with the count that stays).
  - Every managed account's home line, and its first verb: Set a limit, Change limit, Review limit.
  - The person's own limit through set, on, review and ended.
- **route-system-targets-id-mappings**
  - Home folder as No home folder | Gives a home folder, with the limit as Up to a size | No limit.
  - With no location chosen yet (add-home-unset), the sentence and its link take the control's place.
  - The row's read-only home line, and the preview's count of the homes it makes.
- **shell-account:** option B, My access → then Network storage →, between hairlines.
- **Holds due:** a limit under review, read-only, linking to where it's decided.
- **route-storage:** the folder's "Full" line, for a limit set below what is saved.

Departed: the limit dialog says "they" rather than "her", because Syndra doesn't know anyone's pronouns. See BUILD-NOTES, "A-03, read against the code".

## A-08 · Motion coverage (24 Oct 2026)

Built as answered (task 6.103): press to `--tint-3`, link hover 2a (`link-hover`), Basic ↔ Advanced by Unfold in the rail and on the page, narrowing a list (returning rows open, then tint in; a row back mid-fade turns round), seats 16 with a mouse and 24 on touch, Discard in place, a late Notice by Unfold, Read again's short pill.

Departed:
- Touch is "below the desktop breakpoint" (1080px), the product's one definition, not `pointer: coarse`. A tablet in landscape gets the 16px seat.
- Ten text-styled buttons that sit beside links also take `link-hover`, so a link and its neighbour hover alike.
- An inline Advanced id (in a sentence or a header line) tints in and out (`Tinted`): it has no height to unfold.
- Home's calm panel and its block grid still swap at once when the view changes on a quiet day; animating it would also animate the data-driven swap, which A-08 doesn't settle.

