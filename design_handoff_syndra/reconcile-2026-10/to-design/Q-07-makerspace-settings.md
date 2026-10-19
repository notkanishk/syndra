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
