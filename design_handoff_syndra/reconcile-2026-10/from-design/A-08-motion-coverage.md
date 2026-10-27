# A-08 · Motion on every interaction

Answer to `Q-08-motion-coverage.md`, 24 Oct 2026. **Owner, 24 Oct: every recommendation here is accepted** (2a, and the rest). Board: `design/Motion coverage.dc.html` (Final, 24 Oct 2026), read with Motion v2. Nothing here adds a movement or a token. `FINAL.md` §4 now carries every answer below.

## 1 · A pressed row: `--tint-3` (redrawn)

- `--surface-2` was wrong, as Code found.
- **Drop `--hover-press`.** The step already exists: `--tint-3` is `rgba(243,245,239,.09)` in dark and `rgba(37,28,66,.08)` in light, the same values.
- `hover-tint`: hover `--hover`, pressed `--tint-3`. Nothing to add to the token table.

## 2 · Links on hover: 2a, accent with its underline (chosen)

- **Every link ends its hover in the same place: `--accent-text`, with its underline tinted in.**
  - A name link: ink to accent, underline tints in.
  - A violet link and `ArrowLink`: the colour stays, the underline tints in. ArrowLink loses its brightness lift.
- An underline tinting in from transparent is colour, so "hover is colour only" holds. S8 and the standing rules both draw it.
- Keep the guard test against `hover:underline` (switching the line on is a jump). Allow `text-decoration-line: underline` at rest with `text-decoration-color: transparent`, tinting to `currentColor` on tint (120ms). A resting underline stays; only its colour changes.
- Why not 2b (accent tints to ink, as built): two kinds of link swap colours, so a link's hover depends on which link it is, and a name link loses the underline the rules give it.

## 3 · Switching Basic and Advanced: Unfold (confirmed, one change)

- The switch's fill glides (S10).
- **Every Advanced part, in the rail and on the page, opens by Unfold:** height on settle (260ms), contents fading in 120ms behind.
- Back to Basic: contents fade first, then the gaps fold, quicker (200ms).
- The change: page regions use Unfold too, not a tint of their own. One switch moves everything one way.
- Nothing travels and nothing arrives. Rows already on screen hold still.
- **What you're reading stays put.** A part opening or closing above the view keeps the top visible row where it was.
- Still form: the parts appear and go at once.

## 4 · A row dropped by search or filter (confirmed, both)

- Dropped rows fade, and their gap settles straight away, pointer or not. The reader asked for it.
- Rows that still match hold still. Returning rows open their gap, then tint in.
- A row that comes back mid-fade (a backspace) reverses from where it is; it never restarts.
- The same for Filters, FilterPills and a FilterChip's ✕. A refresh, or someone else's change, still waits for the pointer to leave (S5).

## 5 · Small calls

- **StatCard that is a link: confirmed.** Press adds `--tint-3` as a second background layer over the card's own surface, never in place of it.
- **Read again: confirmed, one change.** The zero-height slot stays. Draw the pill no taller than the line plus 6px; on touch, the rest of the 44px target is an invisible hit area. Nothing visible reaches the cards 14px below.
- **Account sheet links: confirmed.** The fill bleeds 8px past the words on each side, as a nav item does; the words stay flush with the sheet's other text.
- **Seat size: redrawn.** 16 with a mouse, 24 on touch, as the standing rules say, in the selection bar too. The row is the hit area.
- **Token format editor, Discard: redrawn.** It's a change in place, so nothing arrives. Untouched rows hold still; edited values crossfade back in their seats (S4, no bloom); added rows fade, then their gap settles; removed rows open their gap, then tint in.
- **Button pending dot: confirmed.** The label and the width never change while busy. The dot breathes.
- **A Notice after first paint: confirmed, one change.** Still, no rise. Its space opens by Unfold and it tints in 120ms behind, so the content below settles rather than jumps; above the view, what you're reading stays put. It leaves by fading first, then folding. A Notice at first paint arrives with its page.
