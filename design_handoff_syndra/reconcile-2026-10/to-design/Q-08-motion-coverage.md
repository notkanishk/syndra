# Q-08 · Motion on every interaction, and what that raised

From Claude Code, 24 Oct 2026. Send this after Q-07. Nothing here reopens a verdict.

After FINAL §4 shipped, we audited every interaction in the product against the patterns table. Most surfaces already matched. The gaps are now built to the table: arrows turn on settle, focus rings fade on tint, links tint, rows darken when pressed, copy confirmations use S18, "How long" is a Segmented, checkboxes and radios are Seats, folds Unfold, search and filter keep the rows that stay, Insights' view switch doesn't replay arrive, loading follows S7, Home has a first paint, and the phone navigation sheet leaves and drags like a dialog.

Building those raised the questions below. Each is already built one way so the product works. Confirm it, or redraw it.

**Answer in the design project at** `handoff/2026-10-reconcile/from-design/A-08-motion-coverage.md`, with any boards beside it in `handoff/2026-10-reconcile/design/`.

## 1 · A pressed row: which colour? (token)

FINAL §4 says a pressed row or card "darkens one step (`--surface-2`)". In light, `--surface-2` is `#ffffff`, so a press went *lighter* than hover. In dark it was barely different from hover.

Built: a new token beside `--hover`, used by `hover-tint` (`ui/src/app/globals.css:491-493, 562-563`):
- dark: `--hover` `rgba(243,245,239,.05)`, `--hover-press` `rgba(243,245,239,.09)`
- light: `--hover` `rgba(37,28,66,.04)`, `--hover-press` `rgba(37,28,66,.08)`

**Recommend:** keep `--hover-press` and add it to the design system's token table. Or name the step you want.

## 2 · Links on hover: colour only, to which colour?

"Hover is colour only", so every link's hover underline is gone. A guard test now refuses `hover:underline`. A link's resting underline, where it has one, stays.

Built: an accent link tints to `ink` on hover, on tint (120ms). `ArrowLink` keeps its brightness lift, now on tint too.

**Recommend:** `ink` for every accent link, `ArrowLink` included, so there is one hover for a link. The alternative is a darker accent step, which needs a token.

## 3 · Switching Basic and Advanced

The patterns table has no row for this. Today the Advanced rail groups and the Advanced-only page regions appear and disappear at once.

**Recommend:**
- Advanced rail groups open and close by Unfold: settle 260ms, with contents fading in 120ms behind.
- Advanced-only page regions tint in, and tint out before their gap folds.
- Nothing travels. It's the reader's own act, so it doesn't break "structure never moves in response to data".

Not built yet.

## 4 · A row dropped by search or filter

S5 says the gap of a leaving row waits while the pointer or focus is in the list. That's built for a refresh.

For a search or filter, the reader asked for the rows to go, so the dropped rows fade and their gap settles straight away, even with the pointer in the list (`ui/src/components/ui/LeavingRows.tsx`, `narrowedBy`). Rows that still match hold still, and returning rows tint in.

**Recommend:** confirm both behaviours.

## 5 · Small calls made while building (confirm or redraw)

- **StatCard that is a link:** hover stays the accent ring. A press fills the card with `--hover-press`. A translucent hover fill would show the page ground through the opaque card.
- **"Read again" appearing in a read line:** it tints in, in a zero-height slot, so the line doesn't grow. The button overflows the line by about 6px on desktop and 12px on touch. The alternative is reserving its height on every read line.
- **Account sheet links (My access, Network storage):** they now use the row hover, with the fill reaching 8px beyond the text on each side. Or keep them flush.
- **Seat size:** checkboxes and radios are now Seats, a 24px circle, as in the selection bar. The native ones were 16px on desktop.
- **Token format editor, Discard:** every claim row fades out and the saved rows arrive again. An alternative is a plain swap.
- **Button pending dot:** it sits in the button's left padding, so the label no longer shifts 16px when a button is busy.
- **A Notice that appears after first paint** tints in, still, with no rise. A Notice present at first paint arrives with its page.

## Where to look

- The rules: FINAL §4. What changed: `openspec/changes/reconcile-build/tasks.md` 6.102.
- Live on the dev box, which the owner can open.
