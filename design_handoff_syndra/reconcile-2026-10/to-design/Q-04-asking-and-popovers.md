# Q-04 · Asking for access, links that go where they say, and one way to draw each control

From Claude Code, 15 Oct 2026. Follows A-03. Nothing here retires an earlier answer. Things found while testing on production, and in a sweep of every link and control after it, each fixed in code. §9–13 were Q-05, merged here before either was sent. The fixes are yours to confirm or redraw.

**Answer in the design project at** `handoff/2026-10-reconcile/from-design/A-04-asking-and-popovers.md`, with boards beside it in `handoff/2026-10-reconcile/design/`.

## 1 · Ask for access offered a member nothing to pick (route-requests `ask`)

**Found (Locked).**
- The board's picker is the system's native Select, and the code drew the same one. No new picker was involved.
- The list was empty because of where it read from. It used the operator's lists of projects and roles, which a member may not read. So a member opened the dialog and saw only "Choose…".
- The menu itself is the browser's own, coloured by the person's system accent. That's why it looked unlike anything in Syndra.

**Built (Locked).**
- The dialog reads the member catalogue, the same list My access shows, so the two can never offer different places.
- "Ask for this →" in the catalogue opens the dialog filled in, as before.
- Every other "Ask for access" link opens the dialog on arrival:
  - the no-access state on My access
  - its header button
  - "Ask for access in Requests →" on Network storage
- Before, the person landed on Requests and had to find the button again.

**Question.**
- "What do you need to use?" is a native Select over every place.
- With a dozen places and several roles each, should it become the role picker you drew for bundles (BUILD-LIST 3)? That would mean one search over place and role, with the catalogue's folds.
- Or does the two-step Select stand, because the catalogue on My access already does the browsing?
- Draw whichever you pick, with the place that has nothing defined (`ask-nothing`).

## 2 · An operator asking for their own access (route-requests, route-me)

**Found (Locked).**
- A-03 §4 gave operators My access, where they ask like anyone. Its "Ask for this →" goes to Requests.
- For an operator, Requests is the queue. The queue never read the link, so they landed on it with the dialog shut and their choice lost.

**Built (Locked).**
- The queue reads the link and opens Ask for access over itself, filled in.
- Sent, the request waits in the queue with everyone's.
- Its row says it's their own before approving: "Your own request. Approving it is allowed…" (A-03 §4, owner: allowed and marked).

**Questions.**
1. **Where an operator follows their own ask.**
   - A member has "Your requests" on Requests. An operator has only the queue, which filters to waiting by default.
   - Once their ask is decided, it leaves that view, and nothing on their own pages says what happened.
   - Should "Your requests" also show for an operator, under the queue? Or should My access's Recent changes carry it, since it already quotes decisions?
2. **Asking from the queue itself.** The queue has no "Ask for access" button, on purpose: it's for deciding. Confirm that an operator asks only from My access.
3. **Self-approval in the dialog.**
   - Should the dialog's footer say, before sending, that the operator will be the one deciding it unless another operator does?
   - Or is the queue row's note enough?

## 3 · Dialogs and popovers drew inside the card they came from (system-wide)

**Found (Locked).** Two shapes of one bug.
- **Dialogs.** Reconcile accounts, opened from a mapping row, drew inside its card: clipped by it and over its rows, with the page behind uncovered. A card that has just arrived on the screen still holds its rise animation, and a fixed-position dialog inside it is fixed to the card, not the window.
- **Popovers.** The source popover ("Granted directly", "Carried by a bundle") sat inside cards that clip their contents. On a table's last rows it lost its bottom half, and near the right edge its side. That covers a person's roles, a role's members, expiring access, and inside the removal dialog.

**Built (Locked).**
- Every dialog, sheet and docked note now draws above the page, never inside what opened it.
- Every popover draws above the page too, at its trigger:
  - under it, nudged back inside the window
  - above it when there's more room up than down
  - right-aligned for the Filters panel, whose button ends its row
  - the access map's search results match the search box's width
- Glossary terms (Term) use the same placement. Their upward flip used to cover the word they define.
- A guard test fails any dialog or popover that draws inside the page again.

**Questions.**
1. **Popover placement in the design system.** The rules above aren't written anywhere in yours. Should the design system get a Popover entry with them, so new boards draw popovers as code places them? In particular: the 7px gap, the 16px window gutter, and the flip.
2. **Width at phone sizes.** The source popover is 380px and shrinks to the window less 32px. On a phone, is that right, or should every popover there be a bottom sheet?

New part for `/design-sync` to bring into the design system: `AnchoredPanel` (the popover's placement), beside `Notice`, `NotOpenYet` and `ConnectionGuide`.

## 4 · Links that went somewhere other than they said (system-wide)

**Found and built (Locked).** A sweep of every link in the product, for each role that can see it:
- **"Add people to this role"** opened People filtered to the people who already held the role. Those are the only ones nobody needs to add. It now lists everyone, says "Pick who should get {role}, then choose Give a role", and prefills that role in the dialog.
- **Home › Where access lives.** A role's number is what Zitadel holds, but the row opened People, which lists only people Syndra has a record of, so the unexplained holders it counted never appeared. It now opens the role's own page, which lists both.
- **"Request an extension"** opened an empty ask. It now names the role that's ending.
- **"Set it above ↑"** on Network storage jumped to the first system's password field, whichever card it was on, and pointed at nothing while the system was offline or paused. Each card now has its own field, and the link is gone when there's no field to reach.
- **"The full log is in Review › Audit"** on a person's activity opened everyone's log. It now opens theirs.
- **"Decide on Drift →"** on a person, and **"Review in drift →"** on a role, opened the whole project. They now open that person's rows, and that role's.
- **Home's expiring tile** counted the next 14 days and opened a list of the next 30. It's labelled "Ending in the next 14 days" and opens the same 14.
- **A session the server refused** sent the person to sign-in, which sent them straight back, and round. It now says "Your session ended. Sign in again and you'll be back where you were.", and signing in returns them there. A sign-in page opened while already signed in goes on to where it was headed.

**Question.**
- The expiring list has no control for its window: 14 from Home, 30 from the rail.
- Should it show a "Next 14 days | Next 30 days" Segmented, so the number on Home and the list are visibly one decision?
- Or should both use one window?

## 5 · One control, drawn one way (system-wide)

**Found and built (Locked).**
- **Checkboxes and radios.** Seven were hand-drawn, at three sizes: 16px everywhere in the role picker, 24px under a thumb in the selection bar, and a top nudge in some labels. They're now one Checkbox and one Radio: the selection bar's glyph, 24px on a phone, 16px with a mouse, in the accent.
- **Sign out** was a hand-styled pill in muted text. It's now the outline Button, so it reads like every other button.
- A guard now fails any select, textarea, checkbox, radio or pill-shaped button drawn outside the primitives.

**Questions.**
1. **Checkbox and Radio** aren't in the design system. Should they be? If so, draw the checked, disabled and focus states, and say whether a disabled-but-held row (a bundle's locked role in the picker) looks different from a disabled-empty one.
2. **Sign out** now has ink text instead of muted. Was the muted pill deliberate, to make leaving quieter than anything else in the sheet? If so, say so, and it becomes a variant rather than a one-off.

## 6 · Cards wider than a phone (route-me, member catalogue, bundles, expiring)

**Found and built (Locked).**
- Cards that sit side by side on a wide screen had a minimum width of 420px. On a 375px phone each card was wider than the screen, and the page scrolled sideways.
- That covers My access, the member catalogue, Bundles, and the Expiring access rows.
- Each minimum is now "420px, or the width there is, whichever is smaller". Wide screens are unchanged, and a phone gets one card per row at full width.

**Question.** None, unless the boards meant a different phone layout for these. If they did, draw it.

## 7 · One way to draw each thing, continued (system-wide)

**Built (Locked)**, where a primitive or a rule in RULES already decided it:
- **Cancel** in every dialog footer is the outline button. Seven were ghost.
- **Row actions** are outline sm, per the 15 Oct rule: Dismiss, the stop-tracking Cancel, and Withdraw.
- **A button's busy state** is the button's own spinner, never its label rewritten ("Revoking…", "Setting…").
- **"Somewhere else →"** is one link: ArrowLink, violet, semibold, 13.5px, with a 44px target and the arrow not read aloud. Eleven hand-written ones moved to it, at five sizes before.
- **Empty and loading states** on the access map and Home's makerspace cards use the shared EmptyState and the row skeleton.
- **The 404 page and the crash panel** use Button and ButtonLink.

**Questions.** Code chose nothing here. Each is two patterns in use, and yours to pick.
1. **Callouts.** Notice is used on one screen. Nineteen others draw their own amber, red or violet box, many with a hand-copied "!" badge.
   - Should every one become a Notice?
   - Does Notice need an informational (violet) tone for the explanations at Roles, Expiring access, Bundles, and a person's access page?
   - Or should those be plain prose under RULES ("ledes, notices and captions stay plain prose")?
   - Do the cautions inside dialogs (the "I understand" box, Withheld) stay their own pattern?
2. **Verbs per object.** Today it reads as Revoke for access, Remove for an assignment or a mapping, and Delete for a rule, bundle or role. The mapping dialog says "Remove this mapping" beside "Delete rule" and "Delete bundle". Confirm the scheme, and code will make every label follow it.
3. **"Waiting to be sent".** RULES says "Waiting for you to send →", never "waiting to be sent". The product says "Waiting to be sent" in five places, including a member's own page, where the member can't send anything. Is the RULES phrase for operators only, with members reading something else?
4. **Pagination.** RULES gives "Showing 10 of N · Show 25 more · Show all N · Back to 10". Five lists each do their own: "Load more", "Show all N", or a bare count. Confirm the pattern, including for the two lists that fetch more from the server rather than reveal more (Audit, a person's activity), and code builds it once.
5. **Buttons with an arrow that act rather than go.** "Manage bundles →" on a person's page, "Open the rule →" on a role page, and EmptyState's own action are buttons that open a dialog, but carry the arrow that means "goes somewhere". Drop the arrow, or make them links?
6. **The size of a destructive action on a page.** "Delete bundle" and "Delete rule" are full-size red, while every other destructive action is small. Which is right?
7. **Choosing among cards.** The removal dialog's source chooser, Manage bundles, Publish version and Assign each build their own selectable card row. Should Radio and Checkbox grow a card-row form?
8. **Three segmented controls.** The Basic/Advanced switch, the chime toggle and Tabs each draw their own pill group, deliberately. Keep them, or make them one Segmented?
9. **Dates for an end.** "Expires 2 Aug", "Ends 2 Aug", "until 2 Aug" and "expiring 2 Aug" all appear for the same thing, and two dialogs use the long date with the year. Pick one verb and one format for a list and one for a sentence.

## 8 · Asking for what you already have, and asking twice (route-requests, people, review)

**Found (Locked).**
- A member asked five times, from one dialog, for a role they held for good.
- Every copy reached the queue.
- Approving any of them would have written its end date over the access the member already had.

**Built (Locked, owner 15 Oct):** a request is for a role someone lacks, and one per role. Nothing merges into what's there.
- **The dialog** marks each role "you have this", "yours until 18 Dec" or "already asked", and won't send one of them (all its states: §10).
- **Once sent**, the dialog becomes its result, with Done only.
- **A request for a role they've come to hold since asking** shows "Already has this… decline it to close the request" in the queue, with Approve off.
- **Giving a role someone already has** is refused everywhere: People's grant dialog, and bulk assign, including through a bundle. Extend is the one act that moves an end, and only later.

**Questions.**
1. **Keeping a role that ends.** My access used to say "Request an extension", and a request for a held role is now refused. It now says to tell makerspace staff before it ends. Should a member have a way to ask for that inside Syndra, such as an "Ask to keep it" that lands in the queue as an extension, not a grant? Or is telling staff enough?
2. **A second source.** Bulk assign used to add a direct grant beside a bundle, so access survived the bundle changing. It's refused now. Is that ever needed? If it is, should it be its own action ("Keep this even without the bundle")?

## 9 · Asking and giving, drawn as one flow: the policy (owner, 15 Oct · Locked)

- A member asks only for a role they don't have, and only once while it's waiting.
- An operator gives only a role the person doesn't have.
- An existing end is changed only by Extend, and only to a later date.
- Nothing merges into access someone already has.

## 10 · Ask for access: every state of the one dialog (route-requests `ask`)

Built now, in code's own drawing. Please draw each one:

1. **Choosing.** Each role in the picker is marked: "you have this", "yours until 18 Dec", or "already asked". Marked roles can't be picked.
2. **A role you have, chosen from a link.** The hint replaces "Can't find it?":
   - "You already have this, with no end date, so there's nothing to ask for."
   - For a role that ends: "You have this until 18 Dec. To keep it longer, ask makerspace staff."
   - Send is off.
3. **A role already asked for.** "You've already asked for this. It's waiting, and the answer appears on this page." Send is off.
4. **Sent.**
   - The form goes, and the dialog becomes its result: "Asked for Staff on TrueNAS" with "You'll see the answer on this page", and Done.
   - Reopened, it starts clean. Until today it reopened on the last result.
5. **Refused by the server**: one of the above, reached another way, such as a second tab. It reads as an outcome in the sheet.

**Questions.**
- Is "becomes its result, Done only" right for an ask? Or should the sheet close and the new row arrive at the top of Your requests, with the result said there?
- Today the result card shows an **"Applied"** badge for something that has only been asked. What should an ask's result say?

## 11 · One way for a dialog to report what it did (system-wide)

The same kind of action reports its result in three different ways:
- **Ask for access** turns its sheet into a result card, with an "Applied" badge.
- **Grant direct access** closes, and the person's page shows the result. The owner saw "the correct outcome but in a different form".
- **Rehearsed dialogs** (mappings, bulk, revoke) end on their own result step, listing who changed.

**Question.**
- Draw one pattern for "a dialog did its thing", and say when each variant applies: a single direct change, a request someone else will decide, and a rehearsed change to many people.
- Cover what the badge says ("Asked", "Given", "Waiting to be sent"), where the result lives (in the sheet or on the page), and what the one button reads.

## 12 · The role picker, asking and giving (route-requests, people)

The same choice is drawn two ways:
- **Ask for access:** a native Select labelled "What do you need to do there?", showing role names only.
- **Grant direct access:** a native Select labelled "Role", reading "Community · community" (name, then key). Below it are "2 people hold it", the role's description, and now "test user already has this, with no end."

Both now mark the roles the person already has the same way, and won't let them be picked.

**Questions.**
1. Should both use one picker? Should it be the role picker you drew for bundles (BUILD-LIST 3: one search, the catalogue's folds, the selection seat), or stay a native Select?
2. Should the key ("· community") show to an operator and not to a member, as now, or to neither?
3. Where do "2 people hold it" and the description belong for an operator, and does a member see either?
4. How does a role the person already has read in each? Code says "· you have this" to a member and "· has this" to an operator, and disables it. Should it instead stay pickable and explain itself when chosen?

## 13 · A request overtaken by a grant, and a grant not yet sent (route-requests, route-me, people)

**Found (Locked).** Tested on production, 15 Oct:
- **Given directly while a request waited.** The request stayed open. The queue still asked for a decision, the person couldn't ask again once the role ended, and the only way to close it was Decline, which told them "Not approved" for access they had. The five requests Syndra closed itself earlier were closed the same wrong way.
- **Asking for, or giving, a role that's given but not yet sent.** Refused, which is right, but it said "You already have this" while it doesn't work yet.
- **Giving back a role whose removal hadn't been sent.** Refused as "already has this", because Zitadel still showed it. When it was allowed, it queued the add behind the removal: two changes to confirm that net to nothing.

**Built (Locked).**
- A grant, from any route (directly, by approving, through a bundle or a rule), closes a waiting request for the same role, in the same step. The request reads **"You have this"** to the member and **"Given another way"** to an operator, with a note saying how: "Given directly, so there was nothing left to decide." The five earlier closures now read the same, with "Already had this when it was asked".
- A role given but not yet sent still counts as held, so it's never asked for or given twice. The refusal adds "It's waiting to be sent, so it may not work yet."
- A role whose removal is queued counts as not held. Giving it back cancels the queued removal instead of adding a second change.
- A person's page showed a withdrawn request as "Waiting on a decision". It now uses the queue's words.

**Questions.**
1. Are "You have this" and "Given another way" the right words? Does the member's row also need the note, or only the word?
2. Should Change history and the member's Recent changes say a request was closed because the role came another way, or is the request row enough?
3. Draw the "waiting to be sent" state in Ask for access and Grant direct access: today the picker says "you have this" or "has this" for a role that's given and not yet sent.

## What code would welcome from design

Code moved the product toward one way of drawing each control. Where design sees a better one than code chose, redraw it and code will follow. The same goes for anywhere above where code picked between two plausible patterns.

## Also changed, no question

A member's pages no longer ask for three operator reads on every load. The access map's bundles, the systems list and the system mode were each refused, silently, and two were retried. Nothing a member sees changes.
