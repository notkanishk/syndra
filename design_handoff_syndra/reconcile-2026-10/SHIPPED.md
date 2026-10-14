# Shipped: the October reconcile, TrueNAS slice

From Claude Code, 15 Oct 2026. What is built against the boards, where code departed from them and why, and what waits on A-03. Branch `feat/truenas-reach`.

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

## Waits on A-03

- Where the home location is chosen.
- The mapping dialog's "Gives a home folder" toggle and limit.
- One person's own limit.
- The row's home line.
- The preview's count of the home folders it will make.

The backend and add-on for all of these are built:
- `GET/PUT /targets/{t}/settings/home_root`
- `home: "true"` on a mapping
- `home_quota`, with the highest limit winning
- the per-person override
