# A-04 · Asking and giving, links that go where they say, one way to draw each control

Answer to `Q-04-asking-and-popovers.md`, 17 Oct 2026. **Owner, 17 Oct: every recommendation here is accepted** (4a, 7.1a, Ask to keep it, Retire, the pushed cancel step, and the rest). Every Locked point is followed as written. Code's guarantees in §14 are unchanged; only the drawing is new.

Boards in `design/` beside this folder:
- `route-asking`: ask-choose, ask-held, ask-keep, ask-asked, ask-onway, ask-nothing, ask-sent, ask-refused, op-ask, op-ask-sent, give-choose, give-done, op-own, op-closed, mine, phone-ask, and the §11 result specimens
- `flow-cancel-queued`: grant-one, remove-one, approve-one, several, longer, fold, no-reason, outcome, kept, plan-bulk, plan-bundle, history, member-history, phone-one, phone-outcome, and Motion
- `Controls`: Checkbox and Radio, ChoiceRow, AnchoredPanel, the expiring window (4a, 4b), callouts (7.1a, 7.1b), verbs, waiting words, pagination, arrows, destructive size, pill groups and Sign out, dates

New in `proposed/`: `AnchoredPanel.md`, `ChoiceRow.md`, `CancelQueuedStep.md`. `ActionOutcome-seat.md` gains the block form with a word (§11).

## 1 · What do you need? (route-asking ask-*)

**One RolePicker in single choice** (`mode="one"`, BUILD-LIST 3 and N17) replaces both Selects. It has one search over place and role, the chosen role as one chip, places as folds, and radio seats.
- The place you came from opens first, at the top. The rest are closed, each with its count ("3 roles", "1 chosen").
- A place with nothing defined (`ask-nothing`) says "Nothing yet" in its count before it's opened. Opened, it holds one sentence instead of rows: "Nothing is set up here yet, so there's nothing to ask for. Makerspace staff can set it up." Send stays off and the footer reads "Choose a role to ask for."
- Reason: a dozen places with several roles each is exactly what the picker was built for. My access still does the browsing. The dialog only needs to find one thing fast and say what you already have.
- A member sees name and description. No key, no counts, no Waits-for-you mark.

## 2 · An operator's own ask (op-ask, op-ask-sent, op-own)

1. **Following it.** "Your requests" appears on Requests under the queue once an operator has asked for anything. It's the member's card, in the member's place. My access's Recent changes keeps quoting the decision as it does today.
2. **Asking from the queue.** Confirmed: an operator asks only from My access. The queue stays for deciding.
3. **Self-approval.** Yes, the footer's before-line says it: "You'll decide this yourself, unless another operator does first." The queue row's note stays too. One says it before sending, the other before approving.

## 3 · Popovers (Controls, proposed/AnchoredPanel.md)

1. **Yes, `AnchoredPanel` goes in the design system** with code's numbers: a 7px gap, a 16px window gutter, under by default and above when there's more room up. It starts aligned, aligns its end when the trigger ends its row, and matches the width for search results. It always draws in its own layer above the page and returns focus to its anchor.
2. **Phone.** The rule: a popover you read stays a popover, and a popover you act in becomes a sheet. The source popover and Term stay anchored, at the window's width less 32. Filters (87c) and anything with a choice or a button open as a Modal sm sheet.

## 4 · Links

All fixes confirmed. **The expiring window: 4a, picked by the owner on 17 Oct.** FilterPills "14 days | 30 days" goes in the FilterBar's pill seat. It's a filter, not an action, so it isn't a Segmented. Home's tile and the rail both open on 14 days, so the number you tapped is the number you land on. 4b, one window with no control, is drawn beside it.

## 5 · Controls (Controls)

1. **Checkbox and Radio join the design system** as code drew them: the round seat, accent when on, 16px with a mouse and 24px on touch, a 44px hit area from the row. Checkbox ticks and Radio fills with a 6px dot (8px on touch). Focus is a 2px ring, offset 2. The two disabled forms do differ:
   - **Held** (a bundle's locked role, a role you have): a surface-2 fill, a faint tick, muted words, and why in the note seat.
   - **Can't be chosen** and not held (a role Zitadel no longer offers): an empty ring in `--line`, faint words, and why.
2. **Sign out.** Ink outline is right. The muted pill wasn't a deliberate variant. The sheet keeps leaving quiet through layout (last, under a hairline), not through a one-off colour.

## 6 · Phone cards

No change. One card per row at full width is the intended phone layout.

## 7 · One way to draw each thing (Controls)

1. **Callouts: 7.1a, picked by the owner on 17 Oct.** Every box that reports a dysfunction becomes a Notice, and the hand-copied "!" badges go. Explanations (Roles, Expiring access, Bundles, a person's access page) lose their box and become the page's lede or a caption under the heading. Notice gets no violet tone: violet means you can act. The cautions inside dialogs (I understand, ConfirmByTyping, Withheld) stay their own patterns, because they're about consent and withholding, not dysfunction. 7.1b, a violet Notice, is drawn for comparison.
2. **Verbs.** Code's scheme, with one change:
   - **Revoke**: a person loses access they hold.
   - **Remove**: a part comes out of a whole (a role from a bundle, a bundle from a person, a mapping from a system).
   - **Retire**: a bundle or a rule stops being given and keeps its history (#44). This replaces "Delete bundle" and "Delete rule".
   - **Delete**: gone everywhere, with nothing left to read (a role in Zitadel, an unpublished draft).
   - The verb is the label, the dialog's title and its confirm.
3. **Waiting words.** The RULES phrase is the operator's. A member reads **"On its way"** with a neutral dot, and under it "Given 15 Oct. It starts working once makerspace staff send it." "Waiting to be sent" goes from all five places.
4. **Pagination.** Confirmed for all five lists. Server-fetched lists use the same words:
   - With a total, the row is identical.
   - Without one: "Showing 35 · Show 25 more · Back to 10", with no Show all. At the end it reads "That's everything since 2 Sept · Showing all 61".
   - Show 25 more shows its own spinner while fetching.
5. **Arrows.** Buttons that open a dialog drop the arrow and name their verb: "Manage bundles", "Edit rule". EmptyState's action loses its arrow. Only ArrowLink ends in →.
6. **Destructive size.** Size follows the seat, not the tone. In a page header it's md outline danger, placed last. In a row it's sm. A page never gets the solid fill.
7. **Choosing among cards.** Yes: `ChoiceRow` (proposed/ChoiceRow.md), the picker's seat-led row as the card form of Radio and Checkbox. The source chooser, Manage bundles, Publish version and Assign move to it.
8. **Pill groups.** Tabs stays its own, because switching views is closer to going than to setting. The Basic/Advanced switch and the chime toggle become Segmented sm.
9. **Dates.**
   - The one verb for an end is **ends**: "Ends 2 Aug" and "Ended 2 Aug" in a list, "until 2 Aug" in a sentence.
   - The year appears only when it isn't this year. The time appears only where it matters (#111).
   - "Expires", "expiring" and long dates with the year go, including in the two dialogs.
   - The page keeps the name Expiring access.

## 8 · Asking for what you have

1. **Keeping a role that ends: yes, "Ask to keep it"** (`ask-keep`). A held role with an end stays pickable. Choosing it changes the form: "How much longer?" (Another month | To the end of next term), Why, and the button **Ask to keep it**. The footer reads "Makerspace staff decide. Until they do, it still ends 18 Dec."
   - In the queue it reads "Keep Laser cutter · Operator", with "Approving moves his end date to 18 Jan. Nothing else changes." (`op-own`).
   - Approving runs Extend, so the §9 policy holds.
   - *Backend:* a request kind `extend` that names the grant, with Approve running Extend to the later date. Refuse it if the new end isn't later than the current one.
2. **A second source.** Not needed. The refusal stands. Bundle changes already let the operator choose who keeps what.

## 10 · Every state of Ask for access (route-asking)

- **Choosing**: each row says it in the note seat.
  - Held for good: "You have this".
  - Held with an end: "Yours until 18 Dec", still pickable.
  - Already asked: "Asked 2h ago".
  - Given, not yet sent: "On its way".
  - Rows that can't be picked are ticked grey.
- **Arriving from a link** on one that can't be asked for: the row is outlined so the eye lands on it. A dot and one sentence replace the hint. How long and Why go, Send is off, and the footer says what to do instead.
  - Held: lime dot, "You already have Laser cutter · Member, with no end date, so there's nothing to ask for."
  - Asked: "You've already asked for Bandsaw · Member. It's waiting, and the answer appears on this page."
  - On its way: "Makerspace staff gave you Member on 15 Oct. It starts working once they send it, so there's nothing to ask for."
- **Sent**: "becomes its result" is right, with **Close** (96b), not Done. The result says **Asked**, neutral dot: "Staff on Network storage, for a month." Under it, faint: "Makerspace staff decide. The answer appears on this page, and nothing else is needed from you." Reopened, the dialog starts clean.
- **Refused**: **Not sent**, neutral, because nothing is wrong and it's already in hand. "You already asked for Staff on Network storage, from another tab. That request is waiting." Under it, faint: "Nothing was changed. Its answer appears on this page."

## 11 · One way for a dialog to report what it did (route-asking, Results)

Every dialog ends on its own result, in the sheet, with **Close**. The page beneath updates in place without moving (Motion 1, 2). The block has three parts: a dot and one word in the act's own vocabulary, one sentence about what it means for the person, and one faint line about where it goes next.
- **A single change, now**: lime **Given** / **Removed** / **Extended**. "Rohan can use the laser cutter on his own now."
- **A single change that waits**: the same word with an amber dot, then "Waiting for you to send →" and the faint why (NextStep).
- **A request someone else decides**: neutral **Asked**. Never "Applied".
- **A rehearsed change to many**: the count goes in the word ("Given to 12 people"), then RehearsalDialog's list of who changed, as today.

Grant direct access stops closing on success and stays on its result like the rest. ActionOutcome's block form takes a `word` prop, as the seat form already does (`proposed/ActionOutcome-seat.md`).

## 12 · The role picker (route-asking give-choose)

1. **One picker** for both: RolePicker `mode="one"`.
2. **The key** shows to an operator, in mono after the name, and never to a member.
3. **"N people"** goes in an operator's note seat whenever the row has no status. The description sits under the name for both readers. A member never sees counts.
4. **A role the person has.** Held for good, or given and not yet sent, is ticked grey and not pickable, with why in the note seat. To an operator that's "Has this", or "Given 2h ago" with Waiting for you to send →. Held with an end stays pickable: for a member it becomes Ask to keep it, and for an operator it offers Extend.

## 13 · Overtaken requests

1. **Words**: "You have this" (member, lime) and "Given another way" (operator, neutral) are right. The member's row needs the note too: "Makerspace staff gave it to you directly on 15 Oct, so there was nothing left to decide." Without it, a closed request looks like a lost one (`mine`, `op-closed`).
2. **Change history**: no separate entry. The grant's entry gains one faint line, "Closed Priya's request for it". The member's Recent changes needs nothing more than the request row.
3. **Not yet sent in the pickers**: "On its way" to a member, and "Given 2h ago · Waiting for you to send →" to an operator (`ask-onway`, `give-choose`).
4. **The cancel confirmation becomes a pushed step**, not a stacked sheet. It opens inside whichever sheet asked, so no route can skip it and sheets still never stack. See §14. Removing a bundle whose grants are still waiting drops its own line about them. The step is the one place this is said, because only the step carries who queued each change and why.

## 14 · Undoing a change that's still waiting (flow-cancel-queued, proposed/CancelQueuedStep.md)

1. **One change.** The host sheet turns one page.
   - The ModalHeader chip names the host ("Give Rohan Mehta a role").
   - The title names the kind: "This cancels a removal that's waiting".
   - The lede says what that means today: "It hasn't reached Zitadel, so Rohan still has Member today."
   - Then the change: **Removing** Network storage · Member from Rohan Mehta, then "Queued by Sam Iyer · 2h ago", then his reason as a quote.
   - **Keep it** comes first, is focused, and is what Escape does. The confirm names what goes ahead: "Cancel it and give Member" (accent), "Cancel it and remove" (dangerConfirm, as the removal dialog's own confirm), "Cancel it and approve".
   - Approve on a queue row has no sheet of its own, so a Modal sm opens holding only the step.
2. **Several.** Changes queued together (same person, same moment, same reason) share one who-line and one quote. Up to five changes show; past that, "Showing 5 of 9 · Show all 9" sits inside the list. The title and the confirm carry the count.
3. **A longer list.** The step stays open. One line above the list says: "One more since you looked. Lena queued it a minute ago, so it's added at the top and nothing was changed." The new group sits first, accent-soft, labelled "New since you looked", and flashes once. The confirm's count updates.
4. **No reason.** "Sam gave no reason." in faint, in the quote's place. The footer adds: "If you're unsure why Sam queued it, keep it and ask him."
5. **Outcome.** Yes. The host's usual result, plus one faint line: "The removal Sam queued 2h ago is cancelled, so nothing goes to Zitadel. Change history records both."
6. **Kept.** Back to the form exactly as it was, still open. The footer note reads "Nothing changed. Sam's removal is still waiting." with Waiting for you to send →. A queue row's Approve that's kept leaves the request waiting in its seat.
7. **Plan rows.** PlanReview's consequence line carries "Cancels a change waiting to be sent: …" as code wrote it. Under the plan, one amber line repeats it with the quote, because the plan goes stale if anyone queues another change first.
8. **Change history.**
   - Operator: "Kanishk Singh cancelled a change that was waiting to be sent", then the change, who queued it and when, the quote, and "cancelled when Kanishk gave it back". The act that caused it names it in return.
   - Syndra's own cancellations read "Syndra cancelled…", with "nobody was asked".
   - Member: "You keep Network storage · Member · A removal was waiting to be sent for you. Makerspace staff cancelled it, so nothing changes."
9. **Motion**, from the six movements only. Nothing slides sideways.
   - **The ask arrives**: tint, then settle, then arrive. Focus lands on Keep it.
   - **Kept**: the reverse, then the footer note tints.
   - **Asked again, longer**: settle, then flash once on the new group.
   - **Going ahead**: press, the button's own spinner, then Moment 1.
   - **Pending changes**: a cancelled row keeps its seat (Moment 2) with "Cancelled by Kanishk · nothing will be sent", and leaves on the next read.
   - **Reduced motion**: each state appears in place.

## Open for code

- §8.1 needs the `extend` request kind.
- §2: does Your requests have an endpoint scoped to the operator's own requests, or does it filter the queue's read?
- §14.3: confirm the second refusal returns which entries are new, or code diffs the two lists on the client.
