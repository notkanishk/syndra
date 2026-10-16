# Q-06 · What building the reconcile raised

From Claude Code, 16 Oct 2026. These came up while building the October package: steps 1–5, Bundles v2, My access and Roles that wait. Send this after Q-04 and Q-05.

Everything below is already built one way so the product works. Each item says what was built and asks you to confirm it or redraw it. Nothing here reopens a verdict.

**Answer in the design project at** `handoff/2026-10-reconcile/from-design/A-06-from-the-build.md`, with any boards beside it in `handoff/2026-10-reconcile/design/`.

## 1 · Where the copy guide overrode a board (owner's guide, enforced in code)

The plain-language guide's one word for losing access is "revoke", and its guard refuses the other words. Two drawn strings changed because of it:

- People bulk bar, drawn: "Includes you · you are left out of anything that takes access away". Built: "… anything that **revokes** access".
- My access, Recent changes: drawn "Taken away" / "{role} was taken away". Built: "**Revoked**" / "{role} was revoked."

Please update the boards, or tell the owner the guide should allow the drawn words.

## 2 · Copy no board draws, written in code (confirm or replace)

- **My access, Recent changes:**
  - Withdrawn: "Your request for {role} was withdrawn."
  - Given through a bundle: "{role} was given to you, because you're in {bundle}."
  - Given by a rule: "{role} came with your other access, automatically."
- **My access, a search with no match:** "Nothing on offer matches "{q}". Ask for access, and say what you need under Why."
- **My access, two or more roles ending:** the summary reads "{N} run out, the first on {date}." The board only draws one.
- **A bundle's People tab, someone who got it as the welcome bundle:** "Given when they joined". The board draws only "Given by {name}", "Approved request" and "Moved from v2".
- **A member, with Zitadel never read:** the operator has the degraded Notice, but a member doesn't. A member sees one neutral Notice in place of the list: "Syndra can't reach the sign-in system right now. Nothing about your access has changed. This page tries again on its own."
- **A member's waiting request** says "for 30 days", not the board's "For a month · until 3 Nov". The end date only exists once the request is approved, so a pending request can't name it. Should a pending request say the length, or the date it would end if approved today?
- **A bundle's page opened from a bare link, before its read lands:** the title says "Bundle" for a moment, because the URL carries the id and not the name. RULES says to show the name while loading. Is a skeleton title right there instead?

## 3 · Asking to keep a role longer (owner's ruling, 16 Oct; needs drawing)

My access draws "Ask to keep it longer →" on an expiring role. The ask dialog only asks for a role you don't have, so the backend refused it as "already held". The owner ruled that this is a request to change an existing grant, not a new one. It is being built that way now, from the existing extension mechanism. Please draw:

1. **The member's ask.** The dialog opened from "Ask to keep it longer →": the role, when it ends now, how much longer, and Why.
2. **The operator's request row for an extension.** How "asks to keep {role} until {date}" reads beside an ordinary ask, and what Approve says it will do.
3. **The outcomes for both sides.** Approved means the end date moves. Not approved means it still ends on the old date.
4. **Similar cases code will treat the same way unless you say otherwise:** asking to keep a role that is on hold, and asking again for a role a pending request already covers.

## 4 · Small choices made in the build (confirm)

- **Role picker folds:** closed by default, open while searching or when a role inside is chosen or locked. Turn 3 draws some folds open at rest.
- **The picker's chosen chips:** each has a × "Remove {role}" control. Below desktop it has a 44px target, which makes a chip taller on a phone.
- **A plan's rows:** a queued row in a rehearsal still reads "Waiting for you to send" as a plain word, not the violet link. It's a forecast, and nothing has been queued yet. Should plan rows link too?
- **Single choice in a picker:** the round seat with a dot, used for "Start from a person". Not drawn in detail.
- **The phone Filters sheet:** footer order is accent "Show 12 people", then ghost "Clear all".
- **Roles that wait:** the owner switched production over to roles-only with no roles marked, so the one-time move card (route-automation-settings-v5 `move`) will never show there. It stays in code for any other deployment.

## 5 · Motion added while building (adds to Q-05)

Each of these uses the existing tokens. Please fold them into the Q-05 answer.

- **`Unfold`** (Notices 6a, folds 8a, disclosures, the Remove panel on a bundle row):
  - Opening uses settle 260ms: the height opens and the contents fade in 140ms behind.
  - Closing fades the contents for tint 120ms first, then closes the gap for 260ms on the settle curve.
  - Present on first paint, it doesn't animate.
- **The selection seat:** the fill changes on press 180ms. The check settles in from 60% scale on the settle curve. Pressing scales the seat to 97%.
- **Outcomes** fade into their slot on tint (2a). So do the picker's chips and the WaitsMark tooltip.
- **The bundle page's tabs** switch with no motion.
