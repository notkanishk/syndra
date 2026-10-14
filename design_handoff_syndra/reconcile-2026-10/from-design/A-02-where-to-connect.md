<!--
Landed 14 Oct 2026 from the design project, handoff/2026-10-reconcile/from-design/A-02-where-to-connect.md, verbatim.
Boards it names live in the design project at handoff/2026-10-reconcile/design/
(route-storage, route-system-targets-id, route-system-targets-id-mappings) and are read from there when each screen is built.
Code's reading of it: BUILD-NOTES.md, "A-02, read against the code".
-->
# A-02 · Where a member connects, what a group opens, and home folders

Answer to `Q-02-where-to-connect.md`, 14 Oct 2026. Boards in `design/` beside this folder, each with frames marked **Q-02**:
- `route-storage`: ready, ready-path, restored, home-kept, opens-nothing
- `route-system-targets-id-mappings`: default, opens-nothing, add, add-home, add-home-none, paused
- `route-system-targets-id`: homes-disagree

Your Locked points are followed as written. Fixtures use `drive.makerspace.tools` and `main › waddle_share`.

## 1 · One block per place (route-storage, A recommended)

The section is renamed **Your folders**. The member's places are listed in this order: their home first, then the others A to Z by folder.

**Naming: A, the folder (recommended).**
- The block's name is the folder: `waddle_share`. Beside it, one muted word: **Can change files** or **Read only**.
- Faint beneath: "In main · from your Fabrication lead role". The role is named because the page's lede promises that access follows roles.
- The full path is in the two addresses below it.
- B (frame `ready-path`) uses the path as the name (`main › waddle_share`). It's exact, but it puts the share, which nobody chose to think about, in front of every name.

**Read-only versus change shows.** It's the first thing a member gets wrong ("why can't I save here?").
- A read-only place shows no usage: they can't save there.
- A place they can change but haven't used shows **"Nothing saved here yet"**, not "0 GB". This follows your "0, not missing" rule, worded for a person.
- A quota and its bar show only where one is set. The bar turns amber from 90%, as before.

**Two folders in one share: two blocks.** Each has its own address and its own usage, and the guide asks for one address at a time.

**The connection guide** picks a folder, not a share: "Your home folder", `waddle_share`, `quack_share`. On Android, the server and the share are entered separately ("share: main. Then open waddle_share inside it").

## 2 · Access given back (frame `restored`)

The same violet step panel as a first visit, with the step line back on step 3.

- **Headline:** "Set a new password to open your folders again"
- **Body:** "Your access was given back on 12 Oct. TrueNAS opens your folders again only with a new password, so the one you had before no longer works. Your files are as you left them."
- **Button:** **Set new password**. Phone step bar: "Step 3 of 3 · you set a new password".
- **The folders are listed** under the violet-edged line: "Set a new password to open these" · "Until then, TrueNAS turns every sign-in away." · "Set it above ↑".
- **The date** is when the deny hold was lifted.
- **Signal:** `GET /me/targets` needs one flag for "the account exists, has had a password, and SMB is off until a new one" (e.g. `needs_new_password: true`). It's distinct from `reenrol` (the NAS was rebuilt) and from a first password.

## 3 · What a group opens (route-system-targets-id-mappings)

Per mapping row, under `group → fabrication`, one line per place, then the home folder if the mapping sets one:
- "Opens `main › waddle_share` can change files"
- "and `main › quack_share` read only"
- "Home folders `main › home_dir` one each, private"

**Freshness is said once, in the card header's note**, not per row: "What each group opens, read from TrueNAS's folder permissions 3 min ago".
- When the NAS can't be read: "… as last read from TrueNAS's folder permissions at 09:02", and those lines dim (0.62).
- The mapping itself stays at full strength: it's Syndra's own record (frame `paused`).
- With no last read, the lines are omitted and the note says "What each group opens: not read from TrueNAS yet".

**In the Add/Edit dialog**, once a group is picked, the same sentence sits under it: "Opens `main › waddle_share`, can change files, and `main › quack_share`, read only." The operator sees what a mapping gives before Preview.

## 4 · A group that opens nothing

**On the mappings page** (frame `opens-nothing`), in amber under the row (warn dot):
- "Opens nothing yet. No folder on TrueNAS gives laser-users anything, so its 12 people get an account and nowhere to connect."
- Then, in ink: "Look at the folder's permissions on TrueNAS."
- It's allowed, and nothing blocks it. It's amber because it's a broken assumption with nothing lost.
- The dialog line says the same: "Opens nothing on TrueNAS yet."

**On the member page** (frame `opens-nothing`), when none of the member's groups opens anything:
- **Headline:** "Your account is ready, but no folder is open to you yet"
- **Body:** "Your account exists and your password is set. There's just nowhere to connect to yet, and nothing for you to do."
- **Where the folders would be**, one dashed line: "None yet. Your Laser operator role reaches TrueNAS, but no folder there is open to it. Makerspace staff open folders on TrueNAS itself; this list fills in by itself when they do."
- No addresses and no guide. Sign in as and the password row stay.
- If only some of their roles open nothing, the member page says nothing about those roles. The member can't act on it, and the operator already sees it in amber.

## 5 · Home folder on the mapping screen

**On the row:** read only, as the line "Home folders `main › home_dir` one each, private". No line when none. It's set in exactly one place, the Add/Edit dialog (frame `add-home`). That's the one write place, and **Change** on the row opens the dialog.

**In the dialog:** "Home folder", a Select under "Group on TrueNAS".
- Options: **None** (default, first), then the folders at the top of TrueNAS's SMB shares, as `share › folder`.
- Hint: "Everyone who holds the role gets a folder of their own inside it, named after their account and private to them. Syndra never moves or deletes one, even when the role is taken away."

**No suitable place** (frame `add-home-none`):
- The Select is disabled: "None · no folder to offer". Hint: "Read from TrueNAS 2 seconds ago."
- Neutral panel: **"TrueNAS has no folder to make homes in."** "Homes go inside a folder at the top of a file share, and TrueNAS has no file share turned on yet. The mapping works without one; add a home later by changing it."
- Preview stays on.

**The preview** should count homes it will create: "Makes 34 home folders in main › home_dir; 2 people already have one there."

## 6 · The member's home block

**First among their folders** (frame `ready`):
- **Name:** "Your home folder" · **Can change files**
- Faint: "In main › home_dir · private to you"
- Usage and its bar, then both addresses (`smb://drive.makerspace.tools/main/home_dir/aanya.rao`)

**Role gone, folder kept** (frame `home-kept`): the same block, still first, with one line in ink:
- "You no longer hold Lab Tech, the role that gave you this folder. It stays, with your files in it, and only you can open it. Syndra never moves or deletes a home folder."
- This shows while the account still works through another role.
- If the person has no role left on TrueNAS, the page is in "None of your roles reaches TrueNAS" and the account is off. The home block isn't shown there, because nothing can open it.

## 7 · Two roles disagree (route-system-targets-id, frame `homes-disagree`)

A second row in **Waiting on you**, and the chip counts 2:
- **Title:** "[Priya Nair]'s roles name two home folders". Her name links to her page.
- **Body:** "Fabrication lead puts homes in `main › home_dir`, Archive admin in `archive › homes`. Syndra left her home where it is, in main › home_dir, and makes no second one. Give one of the two mappings no home folder, or the same one."
- **Link, right:** "Change it on Mappings →". A link, not a button: the fix is a mapping change, made in its one place.
- It clears by itself once the mappings agree.
- If several people are in this state from the same pair of mappings, it's one row: "12 people's roles name two home folders", with the names one tap away, filtered.

Nothing else changes.
