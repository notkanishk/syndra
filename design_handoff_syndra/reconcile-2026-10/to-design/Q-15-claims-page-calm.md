# Q-15 · The claims page, calm by default

From Claude Code, 3 Nov 2026, after the owner saw A-14 built and live. This revises A-14's page layout. A-14's behaviour, API and rulings all stay.

## The owner (3 Nov, verbatim)

> I thought the custom claims are going to be under an advance section enclosed away to not have so much clutter. Please redo the screen. It needs to be no bullshit, get work done, while also being enriching. Hardcore technical on purpose when needed. Elegant and clear straight away. Make the important and most common things easier to spot and access, the things that will be used 90% of the time. Cohesive, flowing, beautiful, welcoming user experience.

## What's live now (seen 3 Nov, netbird, Advanced view)

- **Left card, "What netbird is told":**
  - a template line ("From the Custom template. 5 changes since. See the template · Copy another app's shape");
  - the roles table (Role · netbird calls it · Included), plus the "Roles added later" switch;
  - **every claim row open as a form at once:** path segment, key, source and Remove inputs; a format pill set; and a `Sent as …` line under each. Five rows for the seeded v1 shape (`groups`, then `custom_claims` → groups, given_name, family_name, name).
  - "Add a claim";
  - **"Lands in"**, with a three-way pill set and a paragraph of token mechanics;
  - Save.
- **Right card, Preview:** a person picker, then the five checks, each as a dot with two lines of prose, then the JSON, then "Sent in … Zitadel adds sub, email, name and preferred_username itself", then "Copy preview".
- **Below:** a Debug mode card (one line plus "Turn on debug mode"), and History (a version table).
- **Header:** "OIDC · roles from Netbird → · No sign-in yet · client …", a lede, and "Look up sign-ins".

**The problem:** the page opens on the machinery. The 90% jobs aren't the first thing you see, and nothing says in one glance what this app gets.

## The 90% jobs (code's reading of real use; confirm or correct)

1. See, in one glance, what this app gets: which roles, under which word, plus the few details. The answer is a sentence and a compact token, not five rows of forms.
2. Change which of the project's roles the app sees, and what it calls them.
3. Start from the right template for a known app, or copy another app's shape.
4. Check one person: "what does Ada get in netbird?"
5. Save safely: the breaking warning, the summary, "I understand".

**The 10%:**
- custom claim rows (nested paths, if-role, bundles, role ends, attributes, fixed values);
- text formats;
- where claims land;
- debug mode;
- history.

These must be one deliberate step away, and fully technical when opened.

## Locked

- Every A-14 behaviour, the API (`openspec/changes/claims-shaping/design.md`) and the owner's rulings in A-14 and NEXT.md.
- Debug mode stays on this page only.
- One write place: the project default is on the project's page.
- Basic vs Advanced: Basic should never show custom claim rows at all.
- The design system has every part already built for A-14:
  - CheckTrail, TokenBlock (with raw mode), ClaimRow, RoleWordsTable, SignInRow;
  - plus Unfold, Notice, NotOpenYet, ChoiceRow, StatusSeat, ArrowLink, SortHead and the rest.
  
  Use them; propose a new part only if nothing fits.

## What I need

1. **The page's structure, top to bottom,** for Basic and for Advanced. What's visible on arrival, and what is one step away (a fold, a tab, a sheet). The first screen at 1440 should answer "what does this app get, and is it working?".
2. **The summary:** how "what this app gets" reads in one glance. A sentence, a compact token, or both, and how it relates to the preview.
3. **The roles table as the hero of editing:** density, and how renames and exclusions read at rest versus while editing.
4. **"Custom claims" folded:** its closed form (a count and a one-line summary such as "custom_claims · 4 details nested"), its open form (the ClaimRow forms, fully technical), and where formats and "Lands in" live.
5. **The preview and checks:** checks collapse to one line when everything passes ("All five checks pass · Checked 19:06"), and open on a failure, or on demand. Where the person picker sits.
6. **Debug mode and History:** they stay below, but how do they read as quiet, deliberate tools?
7. **Phone at 390.**
8. **Motion:** opening and closing the folds, per Motion v2 / A-08 (Unfold, nothing travels more than 10px).

Draw it with the real components, at true width, in both themes. Recommend one layout; draw a second only if you genuinely can't choose.

**Answer at** `handoff/2026-10-reconcile/from-design/A-15-claims-page-calm.md` on the canvas, with the board `a15-claims-page.dc.html`.
