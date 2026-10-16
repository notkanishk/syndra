# A-06 · From the build

Answer to `Q-06-from-the-build.md`, 17 Oct 2026. Every item is confirmed or adjusted below. Boards changed:
- `route-asking`: new frames keep-sent, keep-mine and op-keep-done
- `route-me`: "Ask to keep it" row action, "Revoked" in Recent changes, "ends" in the summary
- `route-users`: the bulk bar line
- `route-insights`: Revoked in Trends; Ending soon at 14 days

## 1 · The copy guide wins

"Revoke" is right, and both boards now match the build:
- People bulk bar: "Includes you · you are left out of anything that revokes access".
- My access, Recent changes: **Revoked** · "{role} was revoked."

Insights' Trends card follows the same rule: "Access given and revoked, by week", with the legend entry "Revoked".

## 2 · Copy written in code

- **Recent changes.**
  - Withdrawn: change it to **"You withdrew your request for {role}."** This matches the request row's "You withdrew this".
  - Given through a bundle: confirmed, "{role} was given to you, because you're in {bundle}." The bundle name stays plain text, because a member has no bundle page.
  - Given by a rule: name what it came with. "{role} came automatically, because you have {trigger role}." If the rule's trigger isn't a role, use "{role} was given automatically by a makerspace rule."
- **A search with no match.** "Nothing you can ask for matches "{q}". Ask for access, and say what you need in Why." Then the Ask for access button. It's "in Why" because Why is a field.
- **Two or more roles ending.** "{N} roles end, the first on {date}." With one role it stays "One ends on {date}". Never "run out".
- **The welcome bundle.** "Given when they joined" is confirmed.
- **A member, with Zitadel never read.** Use the **amber** Notice, not neutral. Neutral means the network is gone, and Zitadel not answering is amber for operators too. Same anatomy, with the breathing dot:
  - Title: "Syndra can't reach the sign-in system."
  - Sentence: "Nothing about your access has changed. This page tries again on its own."
  - Card headers show without counts, and there are no rows (N23).
- **A pending request's length.**
  - The member's row says the length only: "For a month, from when it's approved."
  - The operator's row keeps "For a month · until 17 Nov". Approving starts it now, so that date is what Approve will do.
- **A bundle opened from a bare link.**
  - Links inside Syndra already know the name from the name catalogue, so pass it and show it at once.
  - A pasted link shows a Skeleton the width of a name in the title seat, with the eyebrow "Bundles" in place. Never the word "Bundle".
  - If the read fails, the title stays a skeleton and the Notice says what couldn't load. Never an id.

## 3 · Ask to keep it (route-asking, route-me)

This was drawn in A-04 §8 as `ask-keep`. Three frames are added, and the entry point is corrected:
- **The entry.** On My access, an ending role's row gets an outline sm button **"Ask to keep it"** in its seat. It replaces "Ask to keep it longer →": it opens a dialog, so it's a button with no arrow (A-04 §7.5).
- **1 · The member's ask (`ask-keep`).**
  - The role is chosen. One sentence: "You have this until 18 Dec. Ask to keep it longer, and makerspace staff decide."
  - "How much longer?" (Another month | To the end of next term), with the hint "If it's approved, you keep it to 18 Jan instead of 18 Dec."
  - Why.
  - Footer: "Makerspace staff decide. Until they do, it still ends 18 Dec." Button: **Ask to keep it**.
- **Sent (`keep-sent`).** **Asked** · "To keep Laser cutter · Operator until 18 Jan." Faint under it: "Until makerspace staff decide, it still ends 18 Dec. The answer appears on Requests." Then Close.
- **2 · The operator's row (`op-own`).**
  - It reads "Keep Laser cutter · Operator", then "Another month · to 18 Jan instead of 18 Dec" and the quote.
  - The note under it says what Approve does: "Approving moves his end date to 18 Jan. Nothing else changes."
- **3 · Outcomes.**
  - Operator (`op-keep-done`), approved: **Extended** · "Now ends 18 Jan, not 18 Dec."
  - Operator, declined: **Declined** · "Nothing was sent · it still ends 20 Oct."
  - Member, on Requests (`keep-mine`): **Waiting** with "Until it's decided, it still ends 18 Dec."; **Kept** with "It now ends 12 Dec instead of 12 Nov."; **Not approved** with "It still ends 20 Oct."
  - Member, in Recent changes: Kept · "You keep {role} until {date}. Your request to keep it was approved." Not approved · "Your request to keep {role} was not approved. It still ends {date}."
- **4 · Similar cases.**
  - **A role on hold.** A hold with a review date and no end has no end to move, so the role offers no Ask to keep it. If the hold has an end, it's offered like any role that ends.
  - **Asking again for a role a pending request covers.** Refused, as `ask-asked` is drawn: "You've already asked to keep this. It's waiting, and the answer appears on this page." Send is off.

## 4 · Small choices

- **Picker folds.**
  - Closed at rest.
  - A fold opens while searching, when it holds the chosen role, or when it's the place a link named. That one opens first, at the top (A-04 §1).
  - A locked or held role doesn't open its fold: the count says it instead ("1 you have", "3 in Safety basics"). Otherwise a member who holds a lot would open on a wall of grey ticks.
  - Turn 3's open folds were illustrative.
- **Chip ×.** Keep the 44px target, but don't make the chip taller.
  - On touch the chip is 36px and the × target extends invisibly to 44px.
  - The chips row has a 10px gap below desktop, so neighbouring targets don't overlap.
- **Plan rows.** No link, because nothing has been queued yet. But not the present-tense phrase either: a plan row says **"Will wait for you to send"**, with the amber dot and no link. It turns into the violet "Waiting for you to send →" in the result, once it's true.
- **Single choice.** The round seat with a dot is confirmed, as Radio in `Controls`.
- **Phone Filters footer.** Use ModalFooter's own order, as every dialog does: **Clear all** (ghost) first, then **Show 12 people** (accent) last, which is the thumb's side. Don't reorder it by hand.
- **Roles that wait.** Noted. The move card stays in code and on the board for other deployments.

## 5 · Motion from the build (folded into A-05)

- **Unfold.** Confirmed, with one retime: the contents fade in **120ms** (one tint) behind the height, not 140. Closing is as built. Present on first paint, it doesn't animate.
- **The selection seat.**
  - The fill changes on **tint**, not press. Press is the 3% gesture only (A-05 §2.3).
  - The tick, and Radio's dot, settle in from 60% scale, 40ms behind the fill (Motion v2 S15).
  - The seat presses 97%, and its row darkens one step. The row never scales.
- **Outcomes in a row's seat** fade in on tint, as 2a: rows don't travel. A dialog's result block lifts 6px and blooms (S6).
- **The picker's chips** tint in and rise 4px (S15).
- **The WaitsMark tooltip** tints only. It's read, not acted in.
- **Tabs.** The content swaps with no motion, as built. The underline glides to the new tab on settle (S10).

## 6 · Included bundles

The safer reading is confirmed, and it's what the board meant:
- **Update** writes only the including bundle's draft. Nobody's access moves when the child publishes.
- **Leave** keeps the version it includes now.
- Every including bundle needs an answer before the child can publish.

The including bundle's page then shows its own draft band (VersionBand): "v4 includes Safety basics v4 · Publish to move its 40 people". Its publish moves its people with its own rehearsal.

## 7 · Insights

- **Term dates.** Aug–Dec, Jan–May and Jun–Jul are right for the shape of the year. Make the dates a setting, though, not constants: terms move each year.
  - Put it in one place in the operator's settings, as one row per term (name, start, end).
  - Trends and "new this term" read from it.
  - The owner should confirm the names. Monsoon, Spring and Summer would match the university's own.
- **Badge taps.** Until a badge source is connected, the card is **Recent sign-ins**, sign-ins only, with no empty half. When a source connects, it becomes Recent use. Nothing on screen says "not connected": the Badge reader's own system page says that.
- **The what-if thread.** Yes, a light one, with no new drawing. Hovering or focusing a person or role named in the answer lights its line on the map (tint), as pins do today. Below tablet there is no map, so nothing changes there.
- **Held but not used.** The card is right without Last used. Its caption says what "not used" means in terms Syndra can show: "No sign-in to its app in 90 days." It's sorted by people, most first.
- **Recent use on the map.** Lighting only the roles held but not used is enough, and honest.
- **Ending soon's window.** Use **14 | 30 days, 14 first**, the same as Expiring access and Home (A-04 §4, 4a). One window across the product means the number on the map matches the list it links to. Drop 7 and 90.
- **Trends has no key.** Confirmed.
