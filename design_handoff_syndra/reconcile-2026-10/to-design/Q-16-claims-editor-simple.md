# Q-16 · The claims editor, simple for the common case

From Claude Code, 4 Nov 2026, after the owner used the A-15 page on prod (Syndra app, Advanced view). This revises A-15's shape card and A-14's claim rows. Behaviour, API and rulings stay.

## The owner (4 Nov, verbatim)

> This is such a mess. The buttons are not consistent with the app and design system. Make the roles-names section smaller somehow; it doesn't need to be consuming this much space. I want the 'groups' claim to be outside, or visible by default. The rest can be hidden until needed. I don't know, you tell me. It just feels still clunky and too much.

> [Add a claim] is so messy. Please design something, give me options, ask me questions. The idea is that most of the time I wouldn't need to change the name of the role; I would just need to add "groups" as claims. Be elegant. Best, cohesive, enriching user experience.

## What's live (seen 4 Nov)

**Summary lines:**
- `groups`: "Every role in Syndra, in Syndra's words".
- `custom_claims`: "An object holding groups, given_name, family_name and name".
- Faint line: "In the access token and userinfo. From the Custom template, 5 changes since." Then "See the template" and "Copy another app's shape" as bare text buttons, which don't match the app's button or link grammar.

**"Roles Syndra sees" table:**
- Columns: Role (name + key) · "Syndra calls it" (a large mono field, borderless at rest) · Included (round seat).
- Each row is about 116px tall. With two roles it already fills most of the card.
- A "Roles added later" row with a Left out / Included segmented switch.

**"Claim rows" fold, open.** Each row is:
- three big outlined inputs: the path segment, the key, and the source as a select-like field;
- "Remove" as bare text;
- a "Sent as" segmented switch (A list · Text, with commas · Text, with spaces);
- a "Sent as … for syndra-service" sentence;
- "Put it inside another claim" as bare text.

  Five such rows for the seeded v1 shape: `groups`, then `custom_claims` → groups, given_name, family_name, name.

**Add a claim (second screenshot):**
- a key input, a source field set to "A fixed value", Remove;
- a Text · Number · Yes/no · List segmented switch, and a value input;
- "Sent as "test" for everyone.";
- "Put it inside another claim", then "Add a claim", both bare text.

**Problems the owner names, or that are visible:**
- Inconsistent controls: bare text buttons, outsized inputs, a segmented switch per row.
- The roles table is too tall.
- The common job (send the roles as `groups`) is buried.
- Adding a claim is an unguided form.

## The 90% job (owner)

"Most of the time I wouldn't need to change the name of the role; I would just need to add `groups` as claims." So the common case is:
1. the app gets the project's roles, under the claim name the app expects (often `groups`), as a list;
2. maybe a couple of identity details (name, email);
3. renaming a role, leaving roles out, nesting, fixed values, conditional claims and text formats are rare.

## Locked

- Every A-14 behaviour and API (`openspec/changes/claims-shaping/design.md`); A-15's calm-page structure (summary and roles beside preview and verdict; tools card below).
- Claim keys mono. One write place. Breaking-save gating ("I understand"). Debug mode on this page only.
- The design system's real parts and the standing rules (RULES.md), including the new one: **lists of names are alphabetical, always**.
- **About to change in the backend** (being built now, 4 Nov): roles in the token come from Zitadel's live grant list, with Syndra's facts adding names and other details.

## What I need

1. **The shape card, redrawn:** what is visible by default, and what is folded. The owner wants the roles claim (`groups`) visible by default.
   - Is the roles claim the card's hero?
   - Does it read as one line, e.g. "`groups` · a list of every role this app's people hold"?
   - Where do its few settings live (name, format, which roles)?
2. **The roles table, smaller:** much more compact rows, or folded behind "Rename or leave out roles", since renaming is rare. Show how it reads when no role is renamed or left out (the common case: maybe just "All 2 roles, as they are") and when some are.
3. **Adding a claim:** options side by side (2–3, genuinely different), for example:
   - **(a)** a menu of ready-made claims (the person's name, email, username, their bundles, a fixed value, "yes if they hold a role"), each adding a finished, sensible row;
   - **(b)** an inline composer with smart defaults that grows only as needed;
   - **(c)** a small sheet with a live preview of the result.
   
   Recommend one. Nesting (`custom_claims.…`) and text formats are Advanced and rare: where do they live?
4. **Controls:** every button, link, switch and input must use the design system's parts and sizes, consistent with the rest of Syndra. Name the part for each control.
5. **Questions for the owner:** numbered, each with your recommendation, about what genuinely needs his preference.

Draw both themes for the main frame and a phone frame. **Answer at** `handoff/2026-10-reconcile/from-design/A-16-claims-editor-simple.md`, with the board `a16-claims-editor.dc.html`.
