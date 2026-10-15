# Q-05 · Asking and giving, drawn as one flow

From Claude Code, 15 Oct 2026. Follows Q-04, which it doesn't replace (§8 there is the policy this builds on). It comes from the owner testing on production.

**Answer in the design project at** `handoff/2026-10-reconcile/from-design/A-05-asking-and-giving.md`, with boards beside it in `handoff/2026-10-reconcile/design/`.

## The policy the boards draw (owner, 15 Oct · Locked)

- A member asks only for a role they don't have, and only once while it's waiting.
- An operator gives only a role the person doesn't have.
- An existing end is changed only by Extend, and only to a later date.
- Nothing merges into access someone already has.

## 1 · Ask for access: every state of the one dialog (route-requests `ask`)

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

## 2 · One way for a dialog to report what it did (system-wide)

The same kind of action reports its result in three different ways:
- **Ask for access** turns its sheet into a result card, with an "Applied" badge.
- **Grant direct access** closes, and the person's page shows the result. The owner saw "the correct outcome but in a different form".
- **Rehearsed dialogs** (mappings, bulk, revoke) end on their own result step, listing who changed.

**Question.**
- Draw one pattern for "a dialog did its thing", and say when each variant applies: a single direct change, a request someone else will decide, and a rehearsed change to many people.
- Cover what the badge says ("Asked", "Given", "Waiting to be sent"), where the result lives (in the sheet or on the page), and what the one button reads.

## 3 · The role picker, asking and giving (route-requests, people)

The same choice is drawn two ways:
- **Ask for access:** a native Select labelled "What do you need to do there?", showing role names only.
- **Grant direct access:** a native Select labelled "Role", reading "Community · community" (name, then key). Below it are "2 people hold it", the role's description, and now "test user already has this, with no end."

Both now mark the roles the person already has the same way, and won't let them be picked.

**Questions.**
1. Should both use one picker? Should it be the role picker you drew for bundles (BUILD-LIST 3: one search, the catalogue's folds, the selection seat), or stay a native Select?
2. Should the key ("· community") show to an operator and not to a member, as now, or to neither?
3. Where do "2 people hold it" and the description belong for an operator, and does a member see either?
4. How does a role the person already has read in each? Code says "· you have this" to a member and "· has this" to an operator, and disables it. Should it instead stay pickable and explain itself when chosen?

## 4 · What the operator sees of the same request (route-requests queue, Home)

- A request for a role the person has come to hold since asking reads "Already has this, with no end. There is nothing to approve; decline it to close the request." Approve is off.
- On production, Syndra closed five such requests itself, with the note "They already have this, so there was nothing to approve."

**Question.** When a request becomes moot (granted directly meanwhile), should Syndra close it itself, as the clean-up did, and tell the member? Or leave it for an operator to decline, as now? If Syndra closes it, draw what the member reads on Your requests.

## Also fixed, no question

- In every dialog, the footer no longer covers the focus ring of the last field. The "Why?" box looked clipped while typing.
- Every dialog now opens clean, never on its last state: Ask for access, Grant direct access, Manage bundles and New bundle.
