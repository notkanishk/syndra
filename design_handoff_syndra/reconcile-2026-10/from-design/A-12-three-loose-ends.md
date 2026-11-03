# A-12 · Three loose ends from the build

Answer to `Q-12-three-loose-ends.md`, 3 Nov 2026. Board: `a12-phone-bulk-bar.dc.html` (Final, 3 Nov 2026), for §2 only. Nothing here adds a movement, a token or a component.

## 1 · Home's calm line and its grid: Unfold, whatever caused the swap

**Data swaps them by Unfold too, the same as the view. Never at once, never a crossfade. Both directions.**

- **The movement is Settle:** height on settle (260ms), contents on tint 120ms behind. Closing is tint first, then the fold, quicker (200ms). That is `Unfold` as built (A-08 §3, §5).
- **One hands over to the other.** The leaving part fades first (120ms). Then its gap folds while the arriving part opens, so the page makes one height change, not a shrink and then a grow. No frame shows "Nothing here needs you." beside a block that still needs you.
- **What the cause changes is only when it starts:**
  - **The view switch:** at once, because you asked (A-08 §3).
  - **You close the last open item on Home** (Approve, Extend): Motion 5b. The row's outcome holds its beat and blooms. Then the block folds and the calm line opens. The calm line's dot doesn't bloom: the outcome already did, and one act gets one bloom. This is what the build does now ("the empty state waits for it").
  - **Someone or something else empties Home** (another operator, another tab, a background read): S5. The swap waits while the pointer or focus is inside Home's blocks, then plays once they leave.
  - **The first item arrives on a calm Home:** it plays on the read. The calm line has nothing to press, so nothing is lost under the pointer.
- **The headline changes words in place** ("Nothing needs you" to "1 thing needs you"): S4, a crossfade on tint. No bloom. The count doesn't tick.
- **"Checked, nothing waiting:"** under the header goes and comes by the same Unfold, in step with the swap. Today it pops.
- **Above the view** (a phone scrolled down to the makerspace): what you're reading stays put (A-08 §3).
- **A swap that turns back mid-way reverses from where it is** (A-08 §4). It never restarts.
- **Reduced motion:** both parts change at once.
- **Why not at once:** the makerspace section below jumps by the grid's whole height, under the reader. Unfold exists to stop that.
- **Why not a crossfade with no height change:** a one-line panel and a grid many times taller can't share one height. Holding the taller height leaves a hole under "Nothing here needs you." that reads as something still loading. S7 already rules out crossfading two different things.
- **The locked rule holds.** "Structure never moves because of data" covers the rail and the page's sections (`nav.ts`). Home's blocks already fold on data (5b); this is the same fold.

## 2 · The bulk bar at 390: 2a, one verb plus More changes (chosen)

**Below tablet, a bar with three or more verbs shows one primary verb and "More changes". "More changes" opens a Modal sm sheet with the rest. About 110px tall instead of 312: 15% of the scrollport, not 43%.**

### The bar (frame `a`)
- **Row 1:** "3 people selected" (semibold), then the scope, muted, as built. "Clear the selection" sits at the right end of this row: 28px drawn, 44px to the touch.
- **Row 2:** the primary verb, full width less More changes; then **More changes**. Both are outline, 44px tall, 14px semibold. The primary keeps its own tone: neutral outline, or the danger outline when it takes access away.
- **Neither is accent.** Every verb opens a preview first, and the accent fill belongs to that preview's Apply. A filled button on the bar would read as "this happens now".
- **"More changes", not "More" or "…".** It still names what it holds, and it never ends in → (it opens a sheet).
- **Composition** ("Includes you · you are left out of anything that revokes access") wraps under the count on row 1, faint. The bar grows by that line only.
- **Over the ceiling:** row 2 is the one move, full width: "Select the first 500 people in the order shown". More changes goes, because nothing in it can run.
- **Desktop and tablet are unchanged:** all five verbs in one row.
- **Bars with one or two verbs** (Requests, Expiring access, Unexplained access, Dormant accounts) are unchanged at every width.

### The More changes sheet (frame `a-more`)
- **Modal sm**, a phone sheet from the bottom edge (S3). Title "More changes". No lede: the count stays said once, in the bar, which is still readable under the scrim.
- **The other verbs, in the desktop order,** as full-width md outline buttons, 44px. Neutral first. Then a 16px gap. Then the danger outlines: "Remove from a bundle", "Revoke a role".
- **Footer note:** "Each one shows who would change, person by person, before anything happens." **Cancel** is outline.
- **Sheets don't stack.** Choosing a verb closes More changes (S1 close). Then that verb's dialog opens. Cancel in that dialog returns to the list and its bar, not to More changes.

### Which verb is primary
**The rule: the verb that does the job the list was filtered for. Otherwise the first verb.**
- It reads the URL's filter when the bar appears, never the selection. Ticking a row never changes the primary, so the bar never moves under your thumb. (Changing the filter already clears the selection.)

| The list is | Primary |
|---|---|
| `?bundle=none` (New people without a bundle, from Home) | Give a bundle |
| Needs attention: No access yet | Give a bundle |
| Needs attention: Access ending within 30 days | Extend what ends |
| Needs attention: Departed, still has access | Revoke a role (danger outline) |
| Anything else: no filter, a project, a role, a bundle, Drift, Open requests, a search | Give a role |

- **Owner's call: a danger verb as the primary on Departed, still has access.** Recommended: yes. That cohort only ends with a revoke (32a), the danger outline still says what it does, and the preview names each person before anything happens. The alternative, never a danger primary, leaves that list with a primary nobody came for.

### Why not 2b or 2c
- **2b, two rows of icon and word:** about 200px (28%). At a third of 318px the long verbs wrap ("Remove from a bundle"). Each verb would need a new icon, and Syndra has none that tells giving a role from giving a bundle, so the icons say less than the words.
- **2c, one row that scrolls:** as short as 2a. But two and a half verbs show, and the two danger verbs are always the ones off the edge, found by a sideways swipe nothing points to. A sideways swipe at the screen's bottom edge also competes with the browser's back gesture.

## 3 · Two copy lines

### (f) A project with no roles, in the operator's role picker
**"{Project} has no roles yet, so there's nothing to give. You can add one on its project page."**
- Example: "Laser Lab has no roles yet, so there's nothing to give. You can add one on its project page."
- It names the project, because "here" points at a fold heading the reader may have scrolled past. Then it says where the next step is. No link: a link out of a dialog loses the form (A-07 §3). The project page is where "New role" lives.
- The member's line takes the same shape, so the two read as one picker: **"{Project} has no roles yet, so there's nothing to ask for. Makerspace staff can add them."**
- Replace in `ui/src/components/bundles/RolePickerOne.tsx:196`:
  - operator: `"Nothing is set up here yet, so there's nothing to give."`
  - member: `"Nothing is set up here yet, so there's nothing to ask for. Makerspace staff can set it up."`
- A project where the person already holds every role needs no sentence. Its rows are ticked grey with "Has this", and the fold count says "3 have this" (A-04 §12).

### (g) A refusal's result word: "Not given" and "Not sent" (confirmed)
**"Not given" for a give that didn't happen. "Not sent" for an ask that didn't go.** Both are pronoun-free and in the act's own vocabulary (96b).
- **The rule:** "Not" plus the word that would have shown on success, where it reads as an outcome: Given → **Not given**. Asking's word is "Asked", but "Not asked" reads as if nobody asked, so it names what didn't happen: **Not sent**. "Not approved" already follows this rule.
- **Not given:** danger dot, still (S6), for a refusal and a failure alike. The sentence under it is the one `outcomeFromError` gives, ending "Nothing was changed." The form stays, as built.
- **Not sent:** neutral for the other-tab race (A-04 §10), danger for a real refusal or failure. Same word for both.
- In place, unchanged: `GrantDirectAccess.tsx:283` `setResult({ word: "Not given", outcome: outcomeFromError(error) });` and `AskForAccess.tsx:151` and `:162` `word: "Not sent"`.
- Drift's section "Not given by Syndra" shares two words with it. That's acceptable: one is a section of a list, the other a result with a dot and a sentence in a dialog.

## For code
1. **Home's handover (§1):** when one `Unfold` replaces another, the opening waits one tint (`TINT_MS`) for the closing one's fade. Today both flip together, so for 120ms the page grows by the calm line while the last block still shows. A `delay` on the opening `Unfold`, or a pair component, is code's choice.
2. **Home, grid → calm from a background read** waits while the pointer or focus is inside the blocks (S5). Confirm the blocks' existing leave-hold covers the swap too.
3. **"Checked, nothing waiting:"** goes inside an `Unfold` that follows the swap. The headline's words crossfade (S4) through the existing `Crossfade`.
4. **SelectionBar, below tablet (720px):** with three or more verbs, row 1 is the count, scope and Clear; row 2 is the primary and "More changes". The bar needs to know its verbs as data, so one list renders both layouts (e.g. `verbs: {key, label, tone, onClick}[]` and `primary: key`). The over-ceiling branch replaces row 2 with its one action.
5. **More changes** is a Modal sm with ModalHeader "More changes", the remaining verbs as `Button` md outline or danger, `w-full`, and ModalFooter with its note and an outline Cancel. Choosing closes it before the verb's dialog opens.
6. **`primaryBulkVerb(filters)`** beside `describeFilters` in `lib/people-filters.ts`, by the table in §2. Test the five rows, and test that the primary doesn't change when the selection does.
7. **Touch sweep:** the bar at 390 is about 110px with every target at least 44px. Close `mobile-touch-view` 15.4 once it measures.
8. **Copy (§3 f):** both strings in `RolePickerOne.tsx` take the place's name. Add a `plain-language.test.ts` case if the guard checks picker copy. (g) needs no change. Close `reconcile-build` 6.34 (f) and (g), and the SHIPPED A-08 departure line.
