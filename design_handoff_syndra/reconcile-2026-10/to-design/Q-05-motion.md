# Q-05 · Motion: what code understood, what it couldn't, and the owner's new direction

From Claude Code, 16 Oct 2026. Follows Q-04 (Q-04 §14 point 9 asks for the undoing flow's motion and points here). Nothing here retires the Motion board. It asks design to confirm code's reading of it, settle where the board disagrees with itself, and draw the surfaces it doesn't cover.

**Answer in the design project at** `handoff/2026-10-reconcile/from-design/A-05-motion.md`, with a revised `Motion.dc.html` (or a new board) beside it in `handoff/2026-10-reconcile/design/`.

## The owner's direction (owner, 16 Oct · product choice)

> "Everything needs to have this posh, rich, uplifting, subtle animation that brings it to organic life and a joy to interact with."

The board today reads calm and restrained: "nothing travels that you did not ask to move", one loop, never on a number. Code reads the owner's words as **more life inside those rules, not fewer rules**: subtle and organic fit what's there, while rich and uplifting ask for more of the product to move, with more care. If design reads it differently, say so; the owner decides.

What stays Locked whatever the answer: reduced motion has a still form for everything; nothing moves under the reader while they read; a healthy screen is the stillest thing on it; counts don't animate; a result never travels to a toast; no motion delays an operator clearing a queue.

## 1 · Code's reading of the board (confirm or correct)

- **Six movements**, each a token in `ui/src/app/globals.css:130-165`: tint 120ms (colour and opacity only), press 180ms (3% scale on the pressed control), settle 260ms (pushed, never overlapped), arrive 420ms (rows rise 8px, 40ms apart, at most six), breathe 2400ms (the one loop), flash 900ms (one soft wash on a row that changed under your eye).
- **The picks:** 1b (outcome marks the plan rows), 2a (a row keeps its seat), 3a (the selection bar docks), 4a (filter popover), 5b (say the outcome, then fold), 6a (Notices), 7a (a busy sheet refuses a drag). And 8a–8d (fold, show more, sort, jump), which were settled, not offered as options.
- **Rule of thumb:** things that open settle; things that close fade first; leaving is quicker than arriving.

## 2 · Where the board disagrees with itself

1. **Six or three?** The board says six. The design system's guideline card (`guidelines/motion.html`) says "three movements" (tint, press, settle) plus breathe. Which is current? If three, what replaces arrive and flash?
2. **Easing for arrive, breathe and flash.** Only tint, press and settle have an easing token. Code borrows settle's curve for arrive and uses plain ease-in-out and ease-out for the other two. Name them, or confirm.
3. **Press is used for two things.** It's the 3% press, and also the timing the filter popover opens on and hover and focus ride on. Is press a gesture or a duration?
4. **The 10px cap.** Code and the spec cap travel at 10px. The board never states it, and 3a rises 12px. Which wins?
5. **One loop?** Route-insights draws an infinite halo and travelling lights, and the login has two ambient loops. Are those licensed exceptions, or should they go?
6. **Sorting.** 8c settles the list in from 35% opacity with two keyframes outside the vocabulary. The spec says sorting and filtering hold still. Which is it?
7. **Durations without a token:** 600, 520 and 900ms glides, 280ms and 140ms delays, "+40ms", and the 900ms beat in 5b. Should these become tokens or be retimed to existing ones? `proposed/NotOpenYet.md` says settle is 200ms; it's 260.
8. **Reduced motion** is written out only for 1a and 1b. For the rest, say whether press scale survives, whether glides become jumps, whether flash (colour, no travel) stays, and what answers a drag in 7a.
9. **7a's "heavy resistance"** has no curve or threshold.
10. **"Never on a refresh."** How should code tell a refresh from a filter change, and what does "under your eye" mean for flash?
11. **5b's beat** holds the outcome for 900ms, against "nothing on the critical path is slower than settle".

## 3 · Surfaces the board doesn't cover (draw these)

These are where the owner's "rich, uplifting" can go without breaking a rule above.

1. **A dialog opening and closing.** Code opens with a 10px rise from 97%, 40ms behind its scrim, and closes instantly. Opening comes from the spec, not a board; closing is drawn nowhere.
2. **A sheet over a sheet**, the one place it happens (Q-04 §13 question 4, §14).
3. **A phone sheet** opening, closing and being dragged away (only the busy refusal is drawn).
4. **A status changing in place**, e.g. "Waiting to be sent" to "Applied" while the reader watches.
5. **A row leaving a list** after an action elsewhere: a cancelled change leaving Pending changes, a request decided by someone else.
6. **Success and failure.** Today only colour and words tell them apart. Is there a gentle lift for success, and a still, steady form for failure?
7. **Loading to content.** The skeleton breathes, then content arrives. Is it a crossfade, or arrive?
8. **Hover and press on rows, cards, nav items and links.** Only Button presses today.
9. **The focus ring.** Code fades it in on press timing.
10. **Tabs, segmented controls and selects** changing.
11. **The first load of a screen.** Route transitions are banned, but is there an arrival for the page's own header and first cards?
12. **The undoing flow** in Q-04 §14: the ask arriving, the longer list arriving again, the outcome landing.

## 4 · Found in code, code will fix (no question)

These are build defects against the board, not design questions. They're logged in `openspec/changes/truenas-reconcile/tasks.md` (E13):
- Settle's push-down utility is used nowhere; disclosures open instantly.
- Dialogs, popovers and cards animate in but not out.
- Rows a filter change brings into a list replay the arrive entrance (read in code; not yet seen in a browser).
- A scroll in the view switcher glides even with reduced motion on.
- ActionOutcome, the selection bar and the offline and degraded banners don't move as 2a, 3a and 6a draw.
