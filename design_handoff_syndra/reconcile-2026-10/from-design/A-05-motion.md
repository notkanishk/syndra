# A-05 · Motion, alive

Answer to `Q-05-motion.md`, 17 Oct 2026. Board: `design/Motion v2.dc.html`, beside the Motion board, which stays as it is. Every demo loops, and Tweaks slow them down or show the still forms. Ids S1–S12 refer to the board. The design system's card `guidelines/motion.html` is redrawn as six movements.

## The owner's direction

Code's reading is right: **more life inside the rules, not fewer rules.** The board reads the four words like this:
- **Organic.** Everything that moves uses one curve, `--ease-settle`: it leaves fast and lands softly. Light leads and shape follows 40ms behind. Groups stagger instead of all moving together.
- **Rich.** Depth comes from layers, not distance. A scrim tints before its sheet rises. A control's indicator glides to the choice. A dot that just went well blooms once. Rows, links, nav items and cards all answer the pointer, not only buttons.
- **Uplifting.** Success lifts 6px and blooms. Failure stays still and steady, so it reads as under control. Closing is quicker than opening.
- **Subtle.** Nothing in the page travels more than 10px. A press goes no deeper than 3%. Counts never animate. A healthy screen is the stillest thing on it.

Every Locked point holds.

## 1 · Code's reading

Confirmed in full: the six movements and their timings, the picks 1b, 2a, 3a, 4a, 5b, 6a and 7a, the settled 8a–8d, and the rule of thumb.

## 2 · Where the board disagreed with itself

1. **Six or three?** Six. The guideline card that said three is redrawn as six. Arrive and flash both stay.
2. **Easing.**
   - `--ease-arrive`: `cubic-bezier(0.16, 1, 0.3, 1)`, the same curve as settle.
   - `--ease-breathe`: `cubic-bezier(0.37, 0, 0.63, 1)`.
   - `--ease-flash`: `cubic-bezier(0.33, 1, 0.68, 1)`.
3. **Press is a gesture**, used only for the 3% scale.
   - Hover and focus use tint.
   - A popover opens on settle, rising 4px, and closes on tint.
4. **The 10px cap wins** and becomes a token, `--travel-cap: 10px`. 3a's bar rises 10px, and arrive stays at 8px. Two things fall outside the cap because they aren't page content: a phone sheet entering from the screen's edge, and a control's own indicator (Segmented's fill, Tabs' underline).
5. **One loop.** Inside the product, breathe is the only loop.
   - Insights loses its infinite halo and travelling lights. When a question is asked, the lights travel once along the answer, then rest. While the answer is being computed, its node breathes.
   - The login is outside the product, so its two ambient loops stay there. They stop with reduced motion.
6. **Sorting: the spec wins, and rows never travel.**
   - A re-sort is one tint: the list fades out, then back in its new order, 120ms each way. 8c's two keyframes go.
   - Filtering works the same way. Rows that stay don't replay arrive.
7. **Durations without a token.**
   - One new token, `--duration-glide: 520ms` on settle's curve, used for scrolling only. It replaces the 600, 520 and 900ms glides.
   - "Fade first, then close" waits one tint (120ms), so the 140ms delay goes.
   - A flash after a glide starts when the glide ends, so the 280ms delay goes.
   - "+40" is `--arrive-step`.
   - `proposed/NotOpenYet.md` now says 260ms.
8. **Reduced motion.** The board's token table gives every movement's still form.
   - Press survives: it's 3%, under the finger, and travels nowhere.
   - Glides become jumps.
   - Flash survives as colour only. The bloom goes.
   - Breathe rests at full strength.
   - In 7a, a drag doesn't move the sheet at all. The sentence answers with a tint.
9. **7a's resistance.** While busy, the sheet follows a drag at 15% of the finger's travel, capped at 10px, and settles back on release. It never closes, however far or fast it's dragged.
10. **Refresh, filter, and "under your eye".**
    - Key rows by id.
    - Arrive plays only on a list's first paint after you navigate to it, or when a fold opens.
    - Every later change (refresh, filter, sort, search) works the same way: rows that stay hold still, new rows tint in, and leaving rows tint out before their gap settles.
    - "Under your eye" means two things are true: the row changed because of something you did on this screen in the last 2 seconds, and it's on screen now. Background reads never flash.
11. **5b's beat stands.** It isn't on the critical path: the next row's buttons work during the beat, and only the fold waits. If the queue empties, the fold follows the flash.

## 3 · Surfaces (Motion v2)

- **S1 · A dialog opening and closing.** Opening: the scrim tints first. 40ms later the sheet rises 8px from 98% on settle, its light 40ms ahead of its shape. Closing: the sheet fades and sinks 4px on tint, then the scrim follows 40ms behind. Still form: both appear and leave together.
- **S2 · A step pushed inside a sheet.** This is the one place that used to be a sheet over a sheet. The form fades first, then the sheet's height settles to the step. The sheet itself doesn't move. Keep it runs the reverse, and the form comes back exactly as it was.
- **S3 · A phone sheet.** It opens from the bottom edge on settle. Held, it follows the finger 1:1, and the scrim thins with it. Let go short of a third of its height, it settles back. Past a third, or on a quick flick, it leaves on settle's duration with tint's curve. While busy, it behaves as 7a.
- **S4 · A status changing in place.** The words crossfade on tint in a seat sized to the taller of the two, so the row doesn't shift. The dot changes colour where it sits. Only when something went well, one ring of the new colour opens 9px from the dot and fades (flash). A status that changed off screen doesn't bloom when you scroll to it. Note: the board's example reads "Sent", never "Applied".
- **S5 · A row leaving after an action elsewhere.** Changed by someone else: the row keeps its seat and its seat says what happened, e.g. "Cancelled by Sam · nothing will be sent" (Moment 2). On the next read it fades, then its gap closes on settle. If the pointer or focus is inside the list, the gap waits until it leaves.
- **S6 · Success and failure.** Success rises 6px on settle and its dot blooms once. Failure appears in place on tint. Its danger dot is steady: no shake, no bloom, no breathe. Both take focus, as `role="status"` for success and `role="alert"` for failure.
- **S7 · Loading becoming content.** The skeleton shows only if the read takes longer than 200ms. When the read lands, the skeleton fades first, then the rows arrive. They don't crossfade.
- **S8 · Hover and press.** Hover is colour only, everywhere: a row's background, a card's line, a link's ink to accent with its underline tinting in, a nav item's background. Controls press 3%: buttons, chips, nav items and segments. Rows and cards never scale. Pressed, they darken one step (`--surface-2`).
- **S9 · The focus ring.** It appears instantly, because a keyboard user is waiting on it, and fades out on tint.
- **S10 · Tabs, Segmented and Select.** Segmented's fill and Tabs' underline glide to the choice on settle, and the labels tint. A tab's content swaps in place, with no slide. Select stays native and doesn't animate.
- **S11 · A screen's first paint.** The shell never moves. The page header tints in, then the cards above the fold arrive. This happens only when you navigate to the page, never on back, a refresh or a filter. Route transitions stay banned.
- **S12 · The undoing flow.** The ask, the longer list and Keep it are S2. Going ahead is press, then the button's spinner, then Moment 1. The cancelled row leaving Pending changes is S5. Frame by frame in `flow-cancel-queued`.

## 3b · Six moments of delight (added by design, owner 17 Oct · Motion v2 S13–S18)

Each one uses only the six movements, travels 6px at most, has a still form and never loops. None plays for something the reader didn't do.
- **S13 · Everything sent.** When an operator's send clears Pending changes, the sent rows fade first and the list folds. One line rises 6px into its place: "Everything has reached Zitadel", with "3 changes sent at 11:20 · nothing is waiting for you" faint under it. Its lime dot blooms once, and the Send button fades out. A queue that empties because someone else sent it shows the line without the bloom.
- **S14 · Daylight.** The theme toggle crossfades every colour token over 260ms (settle's duration, tint's curve), and nothing moves. The toggle's thumb glides as in S10. Implement it with a short `transition` on `background-color, color, border-color` applied to `html` for the switch only, then removed.
- **S15 · Choosing in the role picker.** On press the row darkens. The seat fills on tint, and its dot grows in from the centre on settle, 40ms behind. The chip tints into its reserved seat, rising 4px, and the fold's count tints to "1 chosen". Removing runs it backwards, quicker.
- **S16 · Typing the name to confirm.** Each matching letter warms the field's border one step toward `--danger`, reaching 70% at the full match. A wrong letter returns it to neutral, with no shake. On the last letter the confirm tints from disabled to live.
- **S17 · Refreshed.** Refresh presses and shows its own busy state. The line keeps the old age until the new read lands, so it never shows a time it hasn't read. Then "just now" crossfades in and the freshness dot blooms once. Background reads never bloom.
- **S18 · Copied.** Copy presses, then crossfades in its own seat to a lime dot and "Copied". The dot blooms once, and the seat returns to Copy after 900ms (#85). The insecure-origin "Select" form doesn't animate.

The bloom is one shared effect: a 9px ring of the dot's own colour, on flash's duration and curve. It appears only in S4, S6, S13, S17 and S18, always for something that just went well because of the reader.

## 3c · From the build (A-06 §5)

Unfold's contents fade in 120ms behind the height, not 140. The selection seat's fill is tint, not press; its tick or dot settles in from 60%, 40ms behind. Outcomes in a row's seat tint in; a dialog's result lifts (S6). Chips tint in and rise 4px. Tooltips tint only. Tabs: content swaps still, the underline glides.

## 4 · Code's own fixes

Agreed, all five. S5, S7 and S11 show what the fixed versions should look like, and §2.10 is the rule for the arrive replay.

## New tokens, for /design-sync

`--ease-arrive`, `--ease-breathe`, `--ease-flash`, `--duration-glide: 520ms`, `--travel-cap: 10px`. The DS keeps `--arrive-cap: 5`: the sixth row and after arrive with the fifth.
