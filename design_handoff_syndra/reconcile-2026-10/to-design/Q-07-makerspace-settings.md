# Q-07 · Makerspace settings, and the term dates

From Claude Code, 19 Oct 2026. Send this after Q-06. It is one new page and one dialog, built to `RULES.md` so the product works; nothing here reopens a verdict.

**Answer in the design project at** `handoff/2026-10-reconcile/from-design/A-07-makerspace-settings.md`, with any boards beside it in `handoff/2026-10-reconcile/design/`.

## Why it exists

The owner decided on 19 Oct 2026 that the term dates are edited in one global place, not under Connected systems, because more than one part of Syndra reads them: Trends ("This term"), the "New this term" lens, "To the end of next term" in the keep and give dialogs, and Ask for access. That settles the question Q-06 §7 asked ("Are these right?"): the dates are the makerspace's to set, and the names Spring, Summer and Monsoon are the confirmed defaults. Nothing drew a page for it.

## Locked facts (code owns these; design may redraw, not change what they do)

- **The setting is one endpoint.** `GET /api/v1/terms` returns `{terms: [{name, start, end}], set_by, set_at}`, days as `YYYY-MM-DD`, both inclusive. `set_by` and `set_at` are null for a member, and when it was never set (`backend/internal/handlers/terms.go:14`, router `backend/internal/handlers/router.go:131`). `PUT /api/v1/terms` is operator only and writes the whole list (`terms.go:33`, `router.go:132`). It is audited as `terms.updated`, which Change history reads "Changed the term dates" (`ui/src/lib/audit-vocabulary.ts:71`).
- **The server's rules** (`backend/internal/services/terms.go:86`, limits at `:39`): 1 to 12 terms; a name of 1 to 40 characters; start on or before end; no two terms overlap, and a shared day counts as overlap. A refusal is a 400 `VALIDATION_FAILED` whose `details` keys each field as `terms[1].start`, with the count as `terms`. The client repeats these checks only to say them before sending (`ui/src/lib/term-dates.ts:23`); the server stays the authority.
- **The dates change for everyone and change nobody's access.** The dialog says so before saving; that line is the footer note, because the footer note is the before-line (RULES).
- **Where it sits.** One leaf, "Makerspace settings", at the very end of the Advanced nav, after the Changes and System groups, route `/settings` (`ui/src/lib/nav.ts:145`). It is not in Basic (four destinations, no settings convention to follow) and not in the member nav, so a member who types the address is sent home (`ui/src/lib/nav.ts:202`, `memberMayVisit` at `:272`). An operator in Basic who follows a link still gets the page and its crumb.
- **One write place.** The page is the only place the dates change. Trends shows the term it is reading and, beside it, "Set in Makerspace settings →" (`ui/src/components/insights/TrendsTab.tsx:89`). Nowhere a member can see says it, because a member can't open the page.
- **Saving moves everything that reads the dates at once.** The save refreshes the term read and every Insights read.

## What was built (to RULES; confirm or redraw)

The page, `ui/src/app/settings/page.tsx` and `ui/src/components/settings/TermDatesCard.tsx`:

- PageHeader: title "Makerspace settings", lede "Settings that apply to the whole makerspace, not to one person or one connected system." No eyebrow, as on Insights, since it is a top-level leaf and not a group's child.
- One card, **Term dates**. CardHeader note "For the whole makerspace"; the one way on is an outline sm **Edit term dates** in the header's action seat (no arrow, it opens a dialog).
- Under the header, a lede: "Trends, "new this term" and "to the end of next term" read these. Terms move each year, so check them when one ends."
- A card of settings is one row per setting: label (170) · value. Here the label is the term's name and the value is "1 Aug to 31 Dec" (the year only when it isn't this year). The 200px right seat is empty on these rows because the only action is in the header.
- A read line closes the card: "Set by {name} on {date}." or "Never set. These are the usual dates until you change them."
- Loading and error show no rows, no read line and no Edit button (never show data that wasn't read).

The dialog, Modal md, in the same file:

- Title "Edit term dates", lede "Each term runs from its first day to its last, both included."
- One bordered group per term: Name, then Start and End side by side (native date inputs; below tablet they stack), then an outline sm **Remove** (a part out of a whole). Remove is off when one term is left, since the setting can't be empty.
- A ghost **Add a term** under the list; off at twelve, with "Twelve is the most."
- Errors sit under the field they belong to, in the danger colour, as one faint sentence ("Overlaps Spring, which runs 1 Jan to 31 May. Terms can't share a day."). They appear on Save and go the moment the field is edited. A server error with no field becomes one Notice "The term dates didn't save." and ends "Nothing was changed."
- Footer note, before saving: "Saving changes the dates for everyone. Nothing about anyone's access changes." Then accent **Save term dates** and ghost **Cancel**.
- The dialog ends on its result, in the sheet (96b): the form goes, and an ActionOutcome block reads **Saved**, "Trends and new this term now use these dates.", then Close. Reopened, it starts clean.

## What it needs drawn

1. **The Makerspace settings page**: the header, the Term dates card at rest, with a never-set read line, and with a read error. The build keeps the lede inside the card; say if it belongs under the page title instead.
2. **The Edit term dates dialog**: at rest, with one field error, with the server's refusal, at twelve terms, and the Saved result. Is a bordered group per term right, or should the terms be rows with Name, Start, End and Remove in one line on desktop?
3. **Phone.** The page is Advanced only, so it only reaches a phone through the sheet nav. How should the Start and End date inputs and the term groups read there?

## Questions

1. **Do other makerspace-wide settings belong on this page?** Today it holds one card, and a page with one card may be a page that wants a second. Candidates, so the owner can pick:
   - **The drift sound.** It is set per browser and lives in Account (`ui/src/components/shell/AccountSheet.tsx:119`, `ui/src/components/settings/ChimeToggle.tsx:21`), with a line on Roles that wait pointing there (`ui/src/app/changes/roles-that-wait/page.tsx:92`). Per browser is not makerspace-wide, so the build leaves it where it is. Say if it moves, or if the line on Roles that wait should go.
   - **Roles that wait.** It has its own page and sits in the Changes group because its subject is changes in flight. The build leaves it. Say if it belongs here instead.
   - **Anything else the boards imply is a whole-makerspace choice** (the welcome bundle, the usual length of a request). The build has not moved them, and each already has a write place (RULES: one write place per setting).
   If the answer is "just the term dates", should the page still be called Makerspace settings, or should the card's own name be the page's?
2. **The name in the nav.** "Makerspace settings" is the owner's word. Does a single leaf at the end of the rail, below the System group, read as where an operator would look?
3. **A second place to see the dates, read-only.** Trends links here beside the term it shows. The keep dialog and Ask for access show a date derived from the terms ("until 18 May") but are reached by members, who can't open this page, so the build gives them no link. Should an operator's own view of them say where the dates are set?

Nothing here is built behind a flag; if a board changes it, the page changes with it.

---

# Part 2 · From the board rebuild (21 Oct 2026)

On 20 Oct the live app was compared against every board, desktop and phone, Basic and Advanced, and each screen was rebuilt to its board. Where a board and a later answer disagreed, the build followed the latest decision date. Below is what the build had to decide without a dated answer, and what it could not build. Each item names what was built, so "yes" keeps it.

## Board files that don't render

4. **`route-requests.dc.html` renders blank.** The degraded frame's note (near line 212) has unescaped double quotes inside a double-quoted string, so the board script stops. The build read a copy with the quotes escaped. Could the board be fixed at source?
5. **`route-users-id.dc.html`, frame `person`, renders blank.** The build used `default` and the board notes. Is `person` meant to differ from `default`, and how?

## Undated boards

6. **Some boards carry no decision date**, so when they disagree with another source the build cannot tell which is later: a person's page, Revocations, People's sort order, Systems. Could each board's header say the date it was settled? The build follows the latest date, and an undated board loses to any dated answer.

## Wording the build chose

7. **A name Syndra can't read.** When Zitadel can't name a person or project, the build shows "Unknown account" or "Unknown project", with the id as faint mono text in Advanced only. Consensus #35 would show the id in both views. Which one?
8. **When Zitadel isn't answering.** One Notice sits above the page title, from the shell. A-01 (14 Oct) words it "waits until Zitadel answers"; the Drift and person boards say "paused". The build uses A-01's words. Confirm, or give the one sentence.
9. **Which controls pause while Zitadel isn't answering.** Drift's decision buttons, its bulk preview and "Check Zitadel again" are disabled, following N23 (13 Oct). A person's page, Requests and Expiring are not disabled, because the backend still queues those changes safely. Should they pause too, or say they will wait?
10. **"Put on hold" or "Place a hold".** The TrueNAS page keeps the built "Put on hold". The board says "Place a hold". Which?
11. **"Check for differences" or "Reconciliation".** The card title on the TrueNAS page keeps "Check for differences". The board says "Reconciliation". Which?
12. **"Revoked" or "Taken away".** My access shows "Revoked", and its board says "Taken away". Which, for the member?

## Layout the build chose

13. **Home's headline counts blocks**, "Five things need you", following Consensus #31 over #26. Confirm.
14. **Home's Makerspace strip has 3 tiles** (Connected systems, People, Where access lives), as the board draws. The old "Zitadel is reachable" tile is gone, because the shell's Notice now says when it isn't. Confirm.
15. **Pending changes: why a change waits** sits under the change, as the board draws it, not as its own column as BUILD-LIST said. Confirm.
16. **Section chips while selecting.** On Drift and Expiring the read line's chips stay visible in Select mode. The Expiring board hides them. Which?
17. **The sort arrow** is ↑ for the column's natural order and turns over on reverse, as the boards draw it. Drift used ↓. Confirm ↑ everywhere.
18. **Revocations is sorted newest first**, as the board draws it. Before, the backend's order was oldest first. Confirm.
19. **A person's page has no Extend button on rows.** The board draws none, so Extend stays in Give access. Should a row ending soon offer Extend in place?
20. **Systems: People** is the number of accounts Syndra manages on that system (the system's own inventory), not a count of role holders. Is that the number the column means?

## Not built: the API doesn't carry it yet

21. **Revocations retry:** the board's "Try 3 of 8 · next try in 4 min". The API exposes the tries so far but not the limit or the next time, so the row says "Tried 3 times · tries again by itself". Keep that until the API carries them, or should the column wait?
22. **Systems:** "Configured 3 min ago" and "Last answered 1 min ago", and the product's kind ("network storage").
23. **The TrueNAS accounts table:** sort by Person or Since, and each account's group.
24. **Events:** the board's "dropped, its own change" and "dropped, duplicate". The backend records neither, so such a row says "Syndra chose not to act".
25. **Zitadel:** the "all four checks pass" line. The backend checks reachability and the project count only.

Say for each whether the board should change to what can be read, or the API should grow to fit the board.

## Service accounts: a separate kind of account (owner, 21 Oct 2026)

Once prod read real names, Home's "New people without a bundle" listed 31 accounts. Several of them aren't people at all. They are accounts made for one purpose in the lab: an integration (`netbird`, `syndra-service`, `mkauth`), an agent (`claude code`), and devices or kiosks (`Ingestion Station`, `Touch Pannel`). Some shared accounts look like people but belong to no one person (`Admin Makerspace`, `Makerspace Ashoka`, `test user`).

The owner's decision: **they should not be there.** A service account is its own kind of entity, not a person missing a bundle. Design is asked how to draw it.

What Syndra can tell today:
- Zitadel marks each account **human** or **machine**. Machine accounts (keys, no sign-in) are certainly service accounts. Syndra doesn't read this yet; it will.
- Some purpose accounts are **human-type** in Zitadel (they sign in with a password or a Google login), so the type alone misses them. Something has to mark them.

26. **Where service accounts live.** Do they get their own list, for example under People as a "Service accounts" tab, or a leaf of their own in the rail? Or do they stay out of the directory, and appear only where they hold access?
27. **What a service account's page shows.** A person's page is about one human: their projects, bundles and requests. A service account has a purpose and an owner, the person answerable for it, and holds roles like anyone. Does it need a purpose line, an owner, and when it last signed in or used its key? Is there anything it must never have?
28. **Marking a human-type account as a service account.** Which one way should it be?
    - The operator marks it in Syndra.
    - A naming rule, such as a prefix.
    - A Zitadel metadata key that Syndra reads.

    The owner prefers one write place (an operator's mark, stored once). Where does that control sit, and what does it say?
29. **What leaves the people flows.** Service accounts drop out of these:
    - "New people without a bundle"
    - onboarding gaps
    - welcome bundles
    - People counts on Home and Systems
    - Asking for access, since a machine doesn't ask

    Should they still show in these?
    - Drift, where unexplained access on a service account matters as much as on a person, maybe more.
    - Expiring access.
    - Holds.
    - The TrueNAS accounts table, where a device may well have a storage account.
30. **Counting.** When the Makerspace strip says "People", does that exclude service accounts? Should it say how many there are, for example "48 people · 6 service accounts"?
31. **Ownerless or stale.** Should a service account with no owner, or one whose key or sign-in hasn't been used in 90 days, be something Syndra raises, the way drift is?

## Still undecided from earlier boards

32. **Replacing the Zitadel key in fewer steps** (Zitadel board, open since 7 Oct). The guide dialog (2c) is not built; the four steps stay inline.
33. **The Access map.** The boards' rail still draws Automation and Access map, the 12 Oct shape. N18 (13 Oct) moved Insights up and folded the map into it, and `/graph` now opens Insights. Could the boards' shared rail be redrawn to N18, so new boards stop carrying the older rail?

