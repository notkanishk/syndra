<!--
Landed 15 Oct 2026 from the design project, handoff/2026-10-reconcile/from-design/A-03-one-home-location.md, verbatim.
Boards it names live in the design project at handoff/2026-10-reconcile/design/
(route-system-targets-id, route-system-targets-id-mappings, shell-account).
Code's reading of it: BUILD-NOTES.md, "A-03, read against the code".
-->
# A-03 · One place for home folders, one person's own limit, an operator's own storage

Answer to `Q-03-one-home-location.md`, 15 Oct 2026. Boards in `design/` beside this folder, frames marked **Q-03**:
- `route-system-targets-id`: default, home-unset, home-choose, home-none, home-change · limit-set, limit-on, limit-review, limit-ended
- `route-system-targets-id-mappings`: default (row line), add-home, add-home-unset
- `shell-account`: proposed-storage (A), proposed-both (B, recommended)

Your Locked points are followed as written. A-02 §7 and its `homes-disagree` frame are removed. A-02 `add-home-none` moves to the system page as `home-none`. Fixtures stay within `main`.

## 1 · Where the location is chosen (route-system-targets-id)

The setup card under the title becomes one row per setting, laid out like every other card row: the label in a 170px seat, then the value, then its way on in the 200px right seat.
- **Zitadel project**: Network storage (a link). Its right seat is empty, since it is changed only by removing mappings.
- **Role mappings**: "2 roles to TrueNAS groups · version 4" · **Mappings →**
- **Home folders**: "In `main › home_dir`" · **Change folder**, an outline sm button that names its verb, as row actions do (3b). No ellipsis: no button in Syndra uses one.

This is the one write place. Mappings only show it.

- **Not chosen yet** (`home-unset`): "Not chosen yet" in muted, with one faint line under it ("Mappings work without it; none can give a home folder until it is chosen."), and **Choose folder**. Nothing else on the page changes, because mappings work without it.
- **Choosing** (`home-choose`): Modal md, "Choose where home folders go".
  - Lede: "Everyone whose role gives a home folder gets one of their own in it, named after their account and private to them. It's one folder for every role on TrueNAS."
  - Field "Folder in main": a Select listing the folders at the top of `main`. Hint: "The folders at the top of the main share, read from TrueNAS 2 seconds ago."
  - Footer note: "Takes effect now. Nothing is made until a mapping gives home folders." Button: **Use main › home_dir**.
- **No folder to offer** (`home-none`): the Select is disabled with "No folder to offer", and the hint reads "Read from TrueNAS 2 seconds ago."
  - A neutral panel: **"main has no folder to make homes in."** "Homes go inside a folder at the top of the main share, and main has none yet. Make one on TrueNAS, then read again. Mappings work without it; none can give a home folder until one is chosen." Then **Read again**.
  - The confirming button is off. Footer: "Nothing to choose until main has a folder."
- **Changing it** (`home-change`): the same dialog, titled "Change where home folders go". Picking a different folder adds Changes / Stays, built like Assign:
  - Changes: "New home folders are made in `main › homes`."
  - Stays: "The 36 home folders already in `main › home_dir` stay there, with their files, and their people keep connecting to them where they are. Syndra never moves or deletes a home folder."
  - Footer: "Takes effect now. Nothing is moved, and nobody's access changes." Button: **Make new homes in main › homes**.
  - The 36 is the count of managed accounts whose home address, read from TrueNAS, is in the old folder. With no current read, the Stays line says "The home folders already in …" with no count.

## 2 · The mapping dialog (route-system-targets-id-mappings)

- **Home folder** is a Segmented control, **No home folder** | **Gives a home folder**, with No first and the default. A Segmented rather than a Select, because there are two options.
  - Hint when off: "Home folders go in main › home_dir, set on TrueNAS's page for every role."
  - Hint when on: "Each person who holds the role gets a folder of their own in main › home_dir, named after their account and private to them. Syndra never moves or deletes one, even when the role is taken away."
- **Home folder limit** appears only when it's on: a Segmented, **Up to a size** | **No limit**, then a numeric Input and "GB". Hint: "Someone with two roles that give a home folder gets the larger limit. One person can be given their own, on TrueNAS's page under Accounts."
- **No location yet** (`add-home-unset`): one sentence takes the control's place, with no NotOpenYet frame. That's what `proposed/NotOpenYet.md` already says for one control that can't act now. NotOpenYet is for a whole section, and this is one field.
  - "No mapping can give one until you choose where home folders go on TrueNAS. This one works without it; give it a home folder later by changing it."
  - Then the link **Choose where home folders go →**, to the system page. Preview stays on.
- **The row's read-only line**: "Home folder, up to 20 GB, in `main › home_dir`", or "Home folder, no limit, in `main › home_dir`". There's no line when the mapping gives none.
- The A-02 §5 preview count stands: "Makes 34 home folders in main › home_dir; 2 people already have one there."

## 3 · One person's own limit (route-system-targets-id, Accounts on TrueNAS)

It goes on the system's account list, not the person's page. The limit is a TrueNAS setting, and the person's page is about what Zitadel holds.

**Every managed row** gains a home line under its account: "Home folder up to 20 GB, from Fabrication lead". That's the largest limit among their roles, and the role that gives it. The row's first verb is **Set a limit**, and the actions seat widens to 380.

**Setting it** (`limit-set`): Modal md, "Give Aanya Rao her own home folder limit".
- Lede: "Her roles give her up to 20 GB, from Fabrication lead. Her own limit takes the place of that, larger or smaller, until it ends."
- **Limit**: Up to a size | No limit, then GB. Hint: "She has 18.4 GB in her home folder now."
- **Why**: a Textarea. Required. Hint: "Whoever looks at this limit next reads this and nothing else."
- **Until**: **Ends on** | **Review on**, then a date. Hint (Ends): "On that day her roles' 20 GB comes back by itself. If she has more than that in her folder then, she keeps it all and can't save more until she's under."
- Footer: "Syndra sends it to TrueNAS as soon as you save." Button: **Give her 50 GB**.

**In force** (`limit-on`): the home line reads "Home folder up to 50 GB, her own" in ink. Under it, faint: "Ends 30 Nov · “[reason]”". The verb becomes **Change limit**.

**Review due** (`limit-review`): nothing ends. The faint line turns amber with a warn dot: "Review was due 30 Nov. It stays at 50 GB until you keep it or end it." The verb becomes **Review limit**, which opens the same dialog with Keep and End. It also lists under Holds due. *Confirm that the backend's hold review feeds that list.*

**Ended** (`limit-ended`): an end date lapses by itself, and the line goes back to "up to 20 GB, from Fabrication lead". If her folder holds more than that, one amber line under it says: "Her own 50 GB ended on 30 Nov with 34 GB in her folder. She can open and delete files but can't save new ones until she's under 20 GB. Nothing was deleted." It clears when she's under, or when she's given a limit again. With no overage, there's no line, and the end is in Change history.

**Not drawn, for code:** the member's home block on route-storage already turns its bar amber from 90%. When a person is over their limit, the block should say "Full. You can open and delete files here, but not save new ones until you're under 20 GB." *Confirm TrueNAS behaves that way when a quota is set below usage.*

## 4 · The operator's Account sheet (shell-account)

Code's row is confirmed: **Network storage →** (violet, since it goes somewhere), with "Your password and folders" muted under it, above Appearance, between two hairlines. A member doesn't get it in the sheet.

**B, recommended (`proposed-both`):** also **My access →**, "What you can use, and asking for more", first. An operator's own access is asked for like anyone's, so they need the page that asks. Requests stays out of the sheet: the operator already reads every request from their rail.
- *Question for code:* when an operator asks for a role, does the request land in their own queue? If it does, it needs a second approver.

Nothing else in A-02 changes.
