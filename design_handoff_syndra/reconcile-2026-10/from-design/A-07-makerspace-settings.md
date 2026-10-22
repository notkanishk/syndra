# A-07 · Makerspace settings, and what the board rebuild asked

Answer to `Q-07-makerspace-settings.md`, 23 Oct 2026. **Owner, 23 Oct: every recommendation here is accepted** (2b, 26b, 32a, and the rest). Every item is answered below; where a board and this file disagree, this file is later. `FINAL.md` beside this folder is the index of every final choice, board and motion. Boards:
- `route-settings` (new): the page (set, never set, loading, error), the dialog as 2a and 2b, 2b's states, phone
- `route-service-accounts` (new): 26a, 26b, a service account's page, Mark as a service account, Marked, phone
- `route-home`: §32–34 frames long-a (32a), long-b (32b), long-c (32c), long-phone; People tile reads "in 14 days"
- `route-requests`: renders again (§4)
- `route-users-id`: `?frame=person` opens `default` (§5)
- Degraded wording on every route board (§8) and the person and Home boards' buttons (§9)

## 1–3 · Makerspace settings

### The page (confirmed, two changes)
- PageHeader, title and lede as built. No eyebrow.
- **The card's lede stays in the card.** It explains these dates; the page lede explains the page.
- **Drop the header note "For the whole makerspace".** The page lede already says it, and the header then carries only the title and Edit term dates.
- **Each row says which term is this one and which is next**, faint, after the dates: "3 Aug to 18 Dec · This term", "11 Jan to 15 May 2027 · Next term". It lets the operator check what "to the end of next term" means today. Nothing for a term that is neither.
- The read line as built. "Kanishk Singh" links to his page.
- Never set: the usual dates show, because they are what Syndra uses. No warning: nothing is wrong.
- Loading: the card's caption stays (it isn't data); no rows, no read line, no Edit.
- Error: the red Notice takes the card's place: "Couldn't load the term dates." · "Syndra's server did not answer when this page asked for them. Nothing was changed." · Try again · reference.

### The dialog: 2b, one row per term (chosen)
- **Modal lg**, one row per term on desktop: Name · First day · Last day · Remove. The field names are said once, above the rows; each input's own label is "Spring, first day".
- **Why not a bordered group per term (2a):** a box means something is wrong, so three boxes read as three warnings; and rows let the operator read the year top to bottom and see a gap or an overlap before Save does.
- **"First day" and "Last day", not Start and End.** They say "both included" on their own. The lede stays as built.
- **Errors** appear on Save, under the row, across its width, in danger text, and the field takes the danger line. They go when that field is edited. Wording as built: "Overlaps Monsoon, which runs 3 Aug to 18 Dec. Terms can't share a day."
- **The server's refusal** with no field: one red Notice at the top of the form. "The term dates didn't save." · "Syndra's server didn't answer when you saved. Nothing was changed." · reference. The form keeps what was typed; Save is the retry.
- **Twelve:** the list scrolls inside the sheet; Add a term (ghost sm) is off, with "Twelve is the most." beside it.
- **Remove** is outline sm, off when one term is left. It isn't danger: nothing is lost until Save.
- **Footer** as built. **Cancel is outline**, as in every other dialog (route-asking), not ghost.
- **Saved:** healthy dot, **Saved**, "Trends and new this term now use these dates." Faint: "Asking and giving use them from now on. Access already given keeps the end it was given." Close. Reopened, it starts clean.

### Phone
- The page: a row wraps the dates under the name only when both don't fit.
- The dialog is the lg sheet. Each term stacks: Name full width, then First day and Last day **side by side** (a pair reads as a range, and each keeps about 160px), the error under the pair, then Remove at the right, 44px tall. A divider separates terms; no boxes.
- The dates stay native inputs: the phone's own date picker is the best one it has.

### Questions
1. **Just the term dates.**
   - The drift sound stays in Account: it's per browser.
   - **Remove the line on Roles that wait that points to it.** The sound is about drift, and a pointer on an unrelated page only raises the question.
   - Roles that wait stays in Changes. The welcome bundle stays on Bundles, its one write place. A usual request length has no setting today; if one is made, it lands here.
   - **Keep the name "Makerspace settings".** It's the owner's word, and the next makerspace-wide setting arrives without a rename.
2. **Yes.** One leaf at the very end of the rail, after System, is where an operator looks for settings. See §36 for the rail itself.
3. **No second link.** The keep and give dialogs name the term beside the date ("until 15 May 2027, the end of Spring"). That's enough to notice a wrong date, and a link out of a dialog would lose the form. Trends stays the one read-only place with "Set in Makerspace settings →".

## 4–6 · Boards

4. **Fixed at source.** The degraded note in `route-requests` now uses curly quotes, and the board renders.
5. **`person` was never a frame.** No board listed it, and `default` is the person's page. The board now opens `default` for `?frame=person`, so an old link still lands.
6. **Yes, and the rule holds: an undated board loses to any dated answer.** From now on, every board's header label says the date it was settled; the two new boards say 23 Oct 2026. For the four named (a person's page, Revocations, People's sort, Systems), the dated answers win wherever they disagree.

## 7–12 · Wording

7. **Consensus #35: the id shows in both views.** "Unknown account" or "Unknown project", with the id beside it in faint mono as a CopyableValue. The id is the point there: it's the only thing anyone can look up in Zitadel, and that's as true in Basic.
8. **A-01's words, confirmed.** "Paused" is only ever a person's choice. The one sentence, under the Notice title "Zitadel isn't answering. This is what it held at {time}.":
   - "Everything below is real, read {age} ago, and may have changed since. Anything you give or revoke now waits until Zitadel answers."
   - Drift's version ends: "Deciding drift waits until Zitadel answers, because a decision made on an old read can be wrong."
   - Every route board now says this. Bundles, Roles and an app's page name their own verbs: "Publishing, moving people and retiring wait until Zitadel answers."
9. **They don't pause; they say they will wait.**
   - Drift stays off, as N23 has it, because its decisions read Zitadel's state.
   - A person's page, Requests and Expiring stay on, because the change queues safely. The dialog's before-line says "Zitadel isn't answering, so this will wait for you to send it once it does." The result keeps "Waiting for you to send →".
   - The person and Home boards no longer disable their buttons when degraded.
10. **"Put on hold".** It says what happens to the access. The TrueNAS board now says it.
11. **"Check for differences".** It's plain language; "Reconciliation" is the system's word. The TrueNAS board now says it.
12. **"Revoked".** The copy guide, A-06 §1 and RULES all say it. Every board now does, including Insights' Trends.

## 13–20 · Layout

13. **Confirmed.** The headline counts blocks (#31).
14. **Confirmed.** Three tiles; the shell's Notice says when Zitadel isn't answering. The People tile reads "12 whose access ends in 14 days" (the one window, A-04 §4).
15. **Confirmed.** Why a change waits sits under the change.
16. **The chips stay visible in Select mode**, on Drift and Expiring alike. Moving between sections matters most while selecting across them. The Expiring board now does.
17. **Confirmed: ↑ everywhere** for the column's natural order, turning over on reverse. Drift's headings already draw it; the ↓ there is the section-jump chip, which points down the page. Insights' Ending soon now carries ↑ too.
18. **Confirmed.** Newest first, the default where nothing else settles it.
19. **Yes.** A row on a person's page that ends within 14 days offers **Extend** (outline sm) in its seat, the same as Expiring and Home. Other rows don't. Give access no longer has to be the way to an end date.
20. **People counts people.** The column counts people with an account on that system, not the system's whole inventory. Service accounts are counted on their own list (§30), and the number matches People.

## 21–25 · What the API carries

21. **Keep "Tried 3 times · tries again by itself".** When the API carries the next time, add "next try in 4 min". The limit isn't needed: nobody acts on it.
22. **Boards change to what can be read, with one exception: the API should grow "Last answered".** Freshness rests on it, and A-01's check pips already need the check history. Drop "Configured" and the product's kind until the add-on carries them.
23. **Sort by Person and Since in the client**, from the headings, if the table holds every account. That needs no API. Drop the group until the API carries it.
24. **Keep "Syndra chose not to act"** until the backend records why. The board changes.
25. **The board changes to the two checks:** "Answering, and sees 9 projects."

## 26–31 · Service accounts (`route-service-accounts`)

26. **26b (chosen): a leaf of its own, "Service accounts", in the System group after Connected systems** (Advanced). These accounts are how systems reach Zitadel, so they sit with the systems. A tab under People (26a) keeps them a kind of person, which the owner ruled out.
    - The list: Account (with kind under it) · Purpose · Owner · Last used · Roles (110, right). Sorted by name, with the arrow on Account.
    - It's a short list, so no search.
    - A search on People that matches a service account offers "netbird is a service account →".
27. **The page is a card of settings, then what it holds.**
    - This account: one row per setting.
      - Purpose: **Change purpose**.
      - Owner: the person, linked, and **Change owner**.
      - Kind: "Integration · a machine account in Zitadel", read-only "Set by Zitadel"; for a human-type account, **Mark as a person**.
      - Last used: "Its key, 5 min ago" or "Signed in 3 hours ago", read from Zitadel.
    - What it holds reads like a person's roles, with Revoke.
    - **It must never ask or be welcomed.** No Ask for access and no welcome bundle. Nothing is forbidden by kind otherwise: it holds what its purpose needs, and its owner answers for it.
28. **The operator's mark, stored once in Syndra.**
    - It lives in the **Kind row of the This account card** on the account's own page (Advanced). A person's page says "A person · Signs in with a password in Zitadel", with **Mark as a service account**.
    - The dialog, Modal md:
      - What it is, as round seats: Integration, Device, Agent, Shared login.
      - Purpose, with the hint "What it's for, in a few words. It shows under its name everywhere."
      - Owner, chosen like any person.
      - Before-line: "This finishes now. It leaves People, the people counts and the welcome bundle; nothing about its access changes."
    - The result: **Marked** · "Ingestion Station is a service account now, and Sai Khurana answers for it." Faint: "It's in Service accounts. If this was wrong, Mark as a person on its page puts it back."
    - Machine accounts are service accounts because Zitadel says so. Nobody marks them.
    - No naming rule and no metadata key.
29. **They leave every people flow listed.** They stay, as service accounts, in:
    - **Drift**: yes. Unexplained access on an integration matters at least as much.
    - **Expiring access**: yes, if something it holds ends.
    - **Holds**: yes.
    - **The TrueNAS accounts table**: yes.

    In each, the row's description says "Service account" where a person's would say their title, and the name links to its page.
30. **People never includes them.** No "48 people · 6 service accounts" on Home: Home is a queue. The count is at the top of their own list, and Systems counts people (§20).
31. **Yes, both.** No owner, or not used in 90 days (where Zitadel reports last use), is a dot and a word in the row's seat. Together they are one Home block in When you're ready, "Service accounts to look at", one row per account naming why. It isn't drift: nothing in Zitadel is wrong, but nobody is answering for the access.

## 32–34 · A long block on Home (`route-home`)

32. **32a (chosen): at most five rows, then one violet line to the rest.**
    - Newest (or soonest) first, then "and 26 more →".
    - **The same rule holds for every Home block**: Requests, Expiring, Drift, all of them.
    - The block's height has a ceiling, so an intake of 31 or 300 draws the same card. Fewer rows draw a shorter block. That's the queue emptying, not structure moving.
    - 32b changes the block's shape at a threshold and loses Give a bundle on Home. 32c needs a cap and a link per group, so it still grows.
33. **"and 26 more →" opens People filtered to no bundle**, not a page of its own.
    - The FilterChip is in force, sorted by Joined, newest first, with search and Select.
    - Select's verb is **Give a bundle**, the bulk flow the People board draws (`route-users?frame=bulk-a`).
    - Home's rows keep their own Give a bundle.
34. **The same rule on a phone, at three rows**, then "and 28 more →". Three is what a phone screen holds before the next block.

## 35–36 · Still open

35. **Bench the guide dialog (2c).** The four inline steps work, and replacing the key is rare. It goes to `benched-additions.md`.
36. **Done for the boards.** The rail on the boards is the design system's own Sidebar, built from `nav.ts` when the design system was published (the 12 Oct shape). The design project's copy of the bundle now carries a local patch with the current tree, so every board's rail shows it: N18, Service accounts (§26) and Makerspace settings. Republishing the design system from `ui`, with Service accounts added to `nav.ts`, replaces the patch.
